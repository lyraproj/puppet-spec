package pspec

import (
	"bytes"
	. "fmt"
	"reflect"
	"regexp"
	"strings"

	. "github.com/puppetlabs/go-evaluator/evaluator"
	. "github.com/puppetlabs/go-evaluator/types"
	"github.com/puppetlabs/go-evaluator/utils"
	. "github.com/puppetlabs/go-parser/issue"
)

type (
	Expectation struct {
		levelExpectations []*LevelExpectation
	}

	LevelExpectation struct {
		level    LogLevel
		includes []*Include
		excludes []*Exclude
	}

	Match interface {
		MatchString(str string) bool
		MatchIssue(issue *ReportedIssue) bool
		String() string
	}

	Include struct {
		matchers []Match
	}

	Exclude struct {
		matchers []Match
	}

	IssueMatch struct {
		issue   *Issue
		argsMap map[string]string
	}

	StringMatch struct {
		text string
	}

	RegexpMatch struct {
		regexp *regexp.Regexp
	}

	EvaluatesWith struct {
		example      *Example
		expectations []*Expectation
	}

	ValidatesWith struct {
		example      *Example
		expectations []*Expectation
	}
)

func (e *Expectation) MatchEntries(b *bytes.Buffer, log *ArrayLogger, issues []*ReportedIssue) {
	for _, level := range []LogLevel{NOTICE, WARNING, ERR} {
		entries := log.Entries(level)
		issues := issuesForLevel(issues, level)
		includes := make([]*Include, 0)
		excludes := make([]*Exclude, 0)
		for _, le := range e.levelExpectations {
			if le.level == level {
				includes = append(includes, le.includes...)
				excludes = append(excludes, le.excludes...)
			}
		}
		matchEntries(b, level, includes, excludes, entries, issues)
	}
}

func issuesForLevel(issues []*ReportedIssue, level LogLevel) []*ReportedIssue {
	levelIssues := make([]*ReportedIssue, 0)
	severity := level.Severity()
	if severity != SEVERITY_IGNORE {
		for _, issue := range issues {
			if severity == issue.Severity() {
				levelIssues = append(levelIssues, issue)
			}
		}
	}
	return levelIssues
}

func matchEntries(b *bytes.Buffer, level LogLevel, includes []*Include, excludes []*Exclude, entries []string, issues []*ReportedIssue) {
nextStr:
	for _, str := range entries {
		for _, i := range includes {
			if i.matchEntry(str) {
				continue nextStr
			}
		}
		for _, e := range excludes {
			e.matchLogEntry(b, level, str)
		}
	}

nextIssue:
	for _, issue := range issues {
		for _, i := range includes {
			if i.matchIssue(issue) {
				continue nextIssue
			}
		}
		excluded := false
		for _, e := range excludes {
			if e.matchIssue(b, issue) {
				excluded = true
			}
		}
		if !excluded {
			Fprintf(b, "Unexpected %s\n", issue.String())
		}
	}

	for _, i := range includes {
		i.matchEntries(b, level, entries, issues)
	}
}

func (i *Include) matchEntry(str string) bool {
	for _, m := range i.matchers {
		if m.MatchString(str) {
			return true
		}
	}
	return false
}

func (i *Include) matchIssue(issue *ReportedIssue) bool {
	for _, m := range i.matchers {
		if m.MatchIssue(issue) {
			return true
		}
	}
	return false
}

func (i *Include) matchEntries(b *bytes.Buffer, level LogLevel, strings []string, issues []*ReportedIssue) {
nextMatch:
	for _, m := range i.matchers {
		for _, str := range strings {
			if m.MatchString(str) {
				continue nextMatch
			}
		}
		for _, issue := range issues {
			if m.MatchIssue(issue) {
				continue nextMatch
			}
		}
		Fprintf(b, "Expected %s(%s) but it was not produced\n", level, m.String())
	}
}

func (e *Exclude) matchLogEntry(b *bytes.Buffer, level LogLevel, str string) {
	for _, m := range e.matchers {
		if m.MatchString(str) {
			Fprintf(b, "%s(%s) matches exclusion %s\n", level, str, m.String())
		}
	}
}

func (e *Exclude) matchIssue(b *bytes.Buffer, issue *ReportedIssue) bool {
	excluded := false
	for _, m := range e.matchers {
		if m.MatchIssue(issue) {
			Fprintf(b, "%s matches exclusion %s\n", issue.String(), m.String())
			excluded = true
		}
	}
	return excluded
}

func (im *IssueMatch) MatchString(str string) bool {
	return string(im.issue.Code()) == str
}

func (im *IssueMatch) MatchIssue(issue *ReportedIssue) bool {
	return im.issue.Code() == issue.Code()
}

func (im *IssueMatch) String() string {
	return string(im.issue.Code())
}

func (rm *RegexpMatch) MatchString(str string) bool {
	return rm.regexp.MatchString(str)
}

func (rm *RegexpMatch) MatchIssue(issue *ReportedIssue) bool {
	return rm.regexp.MatchString(issue.String())
}

func (rm *RegexpMatch) String() string {
	b := bytes.NewBufferString(``)
	utils.RegexpQuote(b, rm.regexp.String())
	return b.String()
}

func (sm *StringMatch) MatchString(str string) bool {
	return strings.Contains(str, sm.text)
}

func (sm *StringMatch) MatchIssue(issue *ReportedIssue) bool {
	return strings.Contains(issue.String(), sm.text)
}

func (sm *StringMatch) String() string {
	b := bytes.NewBufferString(``)
	utils.PuppetQuote(b, sm.text)
	return b.String()
}

var MATCH_TYPE = NewRuntimeType3(reflect.TypeOf([]Match{}).Elem())
var ISSUE_TYPE = NewRuntimeType3(reflect.TypeOf(&Issue{}))
var INCLUDE_TYPE = NewRuntimeType3(reflect.TypeOf(&Include{}))
var EXCLUDE_TYPE = NewRuntimeType3(reflect.TypeOf(&Exclude{}))
var EXPECTATION_TYPE = NewRuntimeType3(reflect.TypeOf(&Expectation{}))
var MATCH_ARG_TYPE = NewVariantType([]PType{DefaultStringType(), DefaultRegexpType(), ISSUE_TYPE})
var MATCHERS_TYPE = NewVariantType([]PType{DefaultStringType(), DefaultRegexpType(), ISSUE_TYPE, MATCH_TYPE})
var EXPECTATIONS_TYPE = NewVariantType([]PType{DefaultStringType(), DefaultRegexpType(), ISSUE_TYPE, MATCH_TYPE, INCLUDE_TYPE, EXCLUDE_TYPE})

func makeMatches(name string, args []PValue) (result []Match) {
	result = make([]Match, len(args))
	for ix, arg := range args {
		switch arg.(type) {
		case *StringValue:
			result[ix] = &StringMatch{arg.String()}
			continue
		case *RegexpValue:
			result[ix] = &RegexpMatch{arg.(*RegexpValue).Regexp()}
			continue
		case *RuntimeValue:
			x := arg.(*RuntimeValue).Interface()
			switch x.(type) {
			case *Issue:
				result[ix] = &IssueMatch{x.(*Issue), nil}
				continue
			case Match:
				result[ix] = x.(Match)
				continue
			}
		}
		panic(NewIllegalArgumentType2(name, ix, `Variant[String,Regexp,Issue,Match]`, arg))
	}
	return
}

func makeExpectations(name string, level LogLevel, args []PValue) (result []*LevelExpectation) {
	result = make([]*LevelExpectation, len(args))
	for ix, arg := range args {
		switch arg.(type) {
		case *StringValue:
			result[ix] = &LevelExpectation{level: level, includes: []*Include{{[]Match{&StringMatch{arg.String()}}}}}
			continue
		case *RegexpValue:
			result[ix] = &LevelExpectation{level: level, includes: []*Include{{[]Match{&RegexpMatch{arg.(*RegexpValue).Regexp()}}}}}
			continue
		case *RuntimeValue:
			x := arg.(*RuntimeValue).Interface()
			switch x.(type) {
			case *Issue:
				result[ix] = &LevelExpectation{level: level, includes: []*Include{{[]Match{&IssueMatch{x.(*Issue), nil}}}}}
				continue
			case *Include:
				result[ix] = &LevelExpectation{level: level, includes: []*Include{x.(*Include)}}
				continue
			case *Exclude:
				result[ix] = &LevelExpectation{level: level, excludes: []*Exclude{x.(*Exclude)}}
				continue
			case Match:
				result[ix] = &LevelExpectation{level: level, includes: []*Include{{[]Match{x.(Match)}}}}
				continue
			}
		}
		panic(NewIllegalArgumentType2(name, ix, `Variant[String,Regexp,Issue,Match,Include,Exclude]`, arg))
	}
	return
}

func (e *EvaluatesWith) CreateTest(actual interface{}) Executable {
	source := actual.(string)
	return func(assertions Assertions) {
		actual, issues := parseAndValidate(source, false)
		evaluator := e.example.Evaluator()
		if !hasError(issues) {
			_, evalIssues := evaluate(evaluator, actual, e.example.Scope())
			issues = append(issues, evalIssues...)
		}
		validateExpectations(assertions, e.expectations, issues, evaluator.Logger().(*ArrayLogger))
	}
}

func (e *EvaluatesWith) setExample(example *Example) {
	e.example = example
}

func (v *ValidatesWith) CreateTest(actual interface{}) Executable {
	source := actual.(string)
	return func(assertions Assertions) {
		_, issues := parseAndValidate(source, true)
		validateExpectations(assertions, v.expectations, issues, NewArrayLogger())
	}
}

func (v *ValidatesWith) setExample(example *Example) {
	v.example = example
}

func validateExpectations(assertions Assertions, expectations []*Expectation, issues []*ReportedIssue, log *ArrayLogger) {
	bld := bytes.NewBufferString(``)
	for _, ex := range expectations {
		ex.MatchEntries(bld, log, issues)
	}
	if bld.Len() > 0 {
		assertions.Fail(bld.String())
	}
}

func init() {

	NewGoConstructor(`Exclude`,
		func(d Dispatch) {
			d.RepeatedParam2(MATCHERS_TYPE)
			d.Function(func(c EvalContext, args []PValue) PValue {
				return WrapRuntime(&Exclude{makeMatches(`Exclude`, args)})
			})
		})

	NewGoConstructor(`Include`,
		func(d Dispatch) {
			d.RepeatedParam2(MATCHERS_TYPE)
			d.Function(func(c EvalContext, args []PValue) PValue {
				return WrapRuntime(&Include{makeMatches(`Include`, args)})
			})
		})

	NewGoConstructor(`Match`,
		func(d Dispatch) {
			d.Param2(MATCH_ARG_TYPE)
			d.Function(func(c EvalContext, args []PValue) PValue {
				return WrapRuntime(makeMatches(`Match`, args)[0])
			})
		})

	NewGoConstructor(`Error`,
		func(d Dispatch) {
			d.RepeatedParam2(EXPECTATIONS_TYPE)
			d.Function(func(c EvalContext, args []PValue) PValue {
				return WrapRuntime(&Expectation{makeExpectations(`Error`, ERR, args)})
			})
		})

	NewGoConstructor(`Warning`,
		func(d Dispatch) {
			d.RepeatedParam2(EXPECTATIONS_TYPE)
			d.Function(func(c EvalContext, args []PValue) PValue {
				return WrapRuntime(&Expectation{makeExpectations(`Warning`, WARNING, args)})
			})
		})

	NewGoConstructor(`Notice`,
		func(d Dispatch) {
			d.RepeatedParam2(EXPECTATIONS_TYPE)
			d.Function(func(c EvalContext, args []PValue) PValue {
				return WrapRuntime(&Expectation{makeExpectations(`Notice`, NOTICE, args)})
			})
		})

	NewGoConstructor(`Evaluates_with`,
		func(d Dispatch) {
			d.RepeatedParam2(EXPECTATION_TYPE)
			d.Function(func(c EvalContext, args []PValue) PValue {
				argc := len(args)
				results := make([]*Expectation, argc)
				for idx := 0; idx < argc; idx++ {
					results[idx] = args[idx].(*RuntimeValue).Interface().(*Expectation)
				}
				return WrapRuntime(&EvaluatesWith{nil, results})
			})
		})

	NewGoConstructor(`Validates_with`,
		func(d Dispatch) {
			d.RepeatedParam2(EXPECTATION_TYPE)
			d.Function(func(c EvalContext, args []PValue) PValue {
				argc := len(args)
				results := make([]*Expectation, argc)
				for idx := 0; idx < argc; idx++ {
					results[idx] = args[idx].(*RuntimeValue).Interface().(*Expectation)
				}
				return WrapRuntime(&ValidatesWith{nil, results})
			})
		})
}
