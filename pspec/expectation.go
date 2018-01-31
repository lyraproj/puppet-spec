package pspec

import (
	"bytes"
	. "fmt"
	"regexp"

	"strings"

	. "github.com/puppetlabs/go-evaluator/evaluator"
	"github.com/puppetlabs/go-evaluator/hash"
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
		argsMap *hash.StringHash
	}

	StringMatch struct {
		partial bool
		text    string
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

var EXPECT_OK = &Expectation{levelExpectations: []*LevelExpectation{}}

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
		texts := make([]string, 0)
		for _, entry := range entries {
			if re, ok := entry.(*ReportedEntry); ok {
				issues = append(issues, re.Issue())
			} else {
				texts = append(texts, entry.Message())
			}
		}
		matchEntries(b, level, includes, excludes, texts, issues)
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
		excluded := false
		for _, e := range excludes {
			if e.matchLogEntry(b, level, str) {
				excluded = true
			}
		}
		if !excluded {
			Fprintf(b, "Unexpected %s('%s')\n", level, str)
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
			Fprintf(b, "Unexpected %s: %s\n", issue.Code(), issue.String())
		}
	}

	for _, i := range includes {
		i.matchExpectedIncludes(b, level, entries, issues)
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

func (i *Include) matchExpectedIncludes(b *bytes.Buffer, level LogLevel, strings []string, issues []*ReportedIssue) {
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

func (e *Exclude) matchLogEntry(b *bytes.Buffer, level LogLevel, str string) bool {
	for _, m := range e.matchers {
		if m.MatchString(str) {
			Fprintf(b, "%s(%s) matches exclusion %s\n", level, str, m.String())
			return true
		}
	}
	return false
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
	if im.issue.Code() != issue.Code() {
		return false
	}
	if im.argsMap == nil {
		return true
	}
	for _, k := range im.argsMap.Keys() {
		v := issue.Argument(k)
		if v == nil {
			return false
		}
		a := im.argsMap.Get(k, nil)
		if m, ok := a.(Match); ok {
			switch v.(type) {
			case string:
				if m.MatchString(v.(string)) {
					continue
				}
			case byte:
				if m.MatchString(string([]byte{v.(byte)})) {
					continue
				}
			case rune:
				if m.MatchString(string([]rune{v.(rune)})) {
					continue
				}
			}
			return false
		} else {
			if !Equals(a, WrapUnknown(v)) {
				return false
			}
		}
	}
	return true
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
	if sm.partial {
		return strings.Contains(str, sm.text)
	}
	return str == sm.text
}

func (sm *StringMatch) MatchIssue(issue *ReportedIssue) bool {
	if sm.partial {
		return strings.Contains(issue.String(), sm.text)
	}
	ie := issue.String()
	return ie == sm.text
}

func (sm *StringMatch) String() string {
	b := bytes.NewBufferString(``)
	utils.PuppetQuote(b, sm.text)
	return b.String()
}

var MATCH_TYPE = NewGoRuntimeType([]Match{})
var ISSUE_TYPE = NewGoRuntimeType([]*Issue{})
var INCLUDE_TYPE = NewGoRuntimeType([]*Include{})
var EXCLUDE_TYPE = NewGoRuntimeType([]*Exclude{})
var EXPECTATION_TYPE = NewGoRuntimeType([]*Expectation{})
var MATCH_ARG_TYPE = NewVariantType([]PType{DefaultStringType(), DefaultRegexpType(), ISSUE_TYPE})
var MATCHERS_TYPE = NewVariantType([]PType{DefaultStringType(), DefaultRegexpType(), ISSUE_TYPE, MATCH_TYPE})
var EXPECTATIONS_TYPE = NewVariantType([]PType{DefaultStringType(), DefaultRegexpType(), ISSUE_TYPE, MATCH_TYPE, INCLUDE_TYPE, EXCLUDE_TYPE})

func makeMatches(name string, args []PValue) (result []Match) {
	result = make([]Match, len(args))
	for ix, arg := range args {
		switch arg.(type) {
		case *StringValue:
			result[ix] = &StringMatch{false, arg.String()}
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

func makeIssueArgMatch(arg PValue) interface{} {
	switch arg.(type) {
	case *StringValue:
		return &StringMatch{false, arg.String()}
	case *RegexpValue:
		return &RegexpMatch{arg.(*RegexpValue).Regexp()}
	case *RuntimeValue:
		return arg.(*RuntimeValue).Interface()
	}
	return arg
}

func makeExpectations(name string, level LogLevel, args []PValue) (result []*LevelExpectation) {
	result = make([]*LevelExpectation, len(args))
	for ix, arg := range args {
		switch arg.(type) {
		case *StringValue:
			result[ix] = &LevelExpectation{level: level, includes: []*Include{{[]Match{&StringMatch{false, arg.String()}}}}}
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
	path, source := pathAndContent(actual)
	return func(tc *TestContext, assertions Assertions) {
		actual, issues := parseAndValidate(path, source, false)
		evaluator := e.example.Evaluator()
		if !hasError(issues) {
			_, evalIssues := evaluate(evaluator, actual, tc.Scope())
			issues = append(issues, evalIssues...)
		}
		validateExpectations(assertions, e.expectations, issues, evaluator.Logger().(*ArrayLogger))
	}
}

func (e *EvaluatesWith) setExample(example *Example) {
	e.example = example
}

func (v *ValidatesWith) CreateTest(actual interface{}) Executable {
	path, source := pathAndContent(actual)
	return func(tc *TestContext, assertions Assertions) {
		_, issues := parseAndValidate(path, source, true)
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

	NewGoConstructor(`PSpec::Exclude`,
		func(d Dispatch) {
			d.RepeatedParam2(MATCHERS_TYPE)
			d.Function(func(c EvalContext, args []PValue) PValue {
				return WrapRuntime(&Exclude{makeMatches(`Exclude`, args)})
			})
		})

	NewGoConstructor(`PSpec::Include`,
		func(d Dispatch) {
			d.RepeatedParam2(MATCHERS_TYPE)
			d.Function(func(c EvalContext, args []PValue) PValue {
				return WrapRuntime(&Include{makeMatches(`Include`, args)})
			})
		})

	NewGoConstructor(`PSpec::Contain`,
		func(d Dispatch) {
			d.Param(`String`)
			d.Function(func(c EvalContext, args []PValue) PValue {
				return WrapRuntime(&StringMatch{true, args[0].String()})
			})
		})

	NewGoConstructor(`PSpec::Issue`,
		func(d Dispatch) {
			d.Param2(NewGoRuntimeType([]*Issue{}))
			d.OptionalParam(`Hash[String,Any]`)
			d.Function(func(c EvalContext, args []PValue) PValue {
				var argsMap *hash.StringHash
				if len(args) > 1 {
					argsMap = hash.NewStringHash(5)
					args[1].(*HashValue).EachPair(func(k, v PValue) {
						argsMap.Put(k.String(), makeIssueArgMatch(v))
					})
				}
				return WrapRuntime(&IssueMatch{issue: args[0].(*RuntimeValue).Interface().(*Issue), argsMap: argsMap})
			})
		})

	NewGoConstructor(`PSpec::Match`,
		func(d Dispatch) {
			d.Param2(MATCH_ARG_TYPE)
			d.Function(func(c EvalContext, args []PValue) PValue {
				return WrapRuntime(makeMatches(`Match`, args)[0])
			})
		})

	NewGoConstructor(`PSpec::Error`,
		func(d Dispatch) {
			d.RepeatedParam2(EXPECTATIONS_TYPE)
			d.Function(func(c EvalContext, args []PValue) PValue {
				return WrapRuntime(&Expectation{makeExpectations(`Error`, ERR, args)})
			})
		})

	NewGoConstructor(`PSpec::Notice`,
		func(d Dispatch) {
			d.RepeatedParam2(EXPECTATIONS_TYPE)
			d.Function(func(c EvalContext, args []PValue) PValue {
				return WrapRuntime(&Expectation{makeExpectations(`Notice`, NOTICE, args)})
			})
		})

	NewGoConstructor(`PSpec::Warning`,
		func(d Dispatch) {
			d.RepeatedParam2(EXPECTATIONS_TYPE)
			d.Function(func(c EvalContext, args []PValue) PValue {
				return WrapRuntime(&Expectation{makeExpectations(`Warning`, WARNING, args)})
			})
		})

	NewGoConstructor(`PSpec::Evaluates_ok`,
		func(d Dispatch) {
			d.Function(func(c EvalContext, args []PValue) PValue {
				return WrapRuntime(&EvaluatesWith{nil, []*Expectation{EXPECT_OK}})
			})
		})

	NewGoConstructor(`PSpec::Evaluates_to`,
		func(d Dispatch) {
			d.Param(`Any`)
			d.Function(func(c EvalContext, args []PValue) PValue {
				return WrapRuntime(&EvaluationResult{nil, args[0]})
			})
		})

	NewGoConstructor(`PSpec::Evaluates_with`,
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
		},

		func(d Dispatch) {
			d.RepeatedParam2(EXPECTATIONS_TYPE)
			d.Function(func(c EvalContext, args []PValue) PValue {
				return WrapRuntime(&EvaluatesWith{nil, []*Expectation{&Expectation{makeExpectations(`Error`, ERR, args)}}})
			})
		})

	NewGoConstructor(`PSpec::Parses_to`,
		func(d Dispatch) {
			d.Param(`String`)
			d.Function(func(c EvalContext, args []PValue) PValue {
				return WrapRuntime(&ParseResult{nil, args[0].String()})
			})
		})

	NewGoConstructor(`PSpec::Validates_ok`,
		func(d Dispatch) {
			d.Function(func(c EvalContext, args []PValue) PValue {
				return WrapRuntime(&ValidatesWith{nil, []*Expectation{EXPECT_OK}})
			})
		})

	NewGoConstructor(`PSpec::Validates_with`,
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
		},

		func(d Dispatch) {
			d.RepeatedParam2(EXPECTATIONS_TYPE)
			d.Function(func(c EvalContext, args []PValue) PValue {
				return WrapRuntime(&ValidatesWith{nil, []*Expectation{&Expectation{makeExpectations(`Error`, ERR, args)}}})
			})
		})
}
