package pspec

import (
	"bytes"
	"fmt"
	"regexp"
	"strings"

	"github.com/puppetlabs/go-evaluator/eval"
	"github.com/puppetlabs/go-evaluator/hash"
	"github.com/puppetlabs/go-evaluator/types"
	"github.com/puppetlabs/go-evaluator/utils"
	"github.com/puppetlabs/go-parser/issue"
)

type (
	Expectation struct {
		levelExpectations []*LevelExpectation
	}

	LevelExpectation struct {
		level    eval.LogLevel
		includes []*Include
		excludes []*Exclude
	}

	Match interface {
		MatchString(str string) bool
		MatchIssue(issue *issue.Reported) bool
		String() string
	}

	Include struct {
		matchers []Match
	}

	Exclude struct {
		matchers []Match
	}

	IssueMatch struct {
		issue   *issue.Issue
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

func (e *Expectation) MatchEntries(b *bytes.Buffer, log *eval.ArrayLogger, issues []*issue.Reported) {
	for _, level := range []eval.LogLevel{eval.NOTICE, eval.WARNING, eval.ERR} {
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
			if re, ok := entry.(*eval.ReportedEntry); ok {
				issues = append(issues, re.Issue())
			} else {
				texts = append(texts, entry.Message())
			}
		}
		matchEntries(b, level, includes, excludes, texts, issues)
	}
}

func issuesForLevel(issues []*issue.Reported, level eval.LogLevel) []*issue.Reported {
	levelIssues := make([]*issue.Reported, 0)
	severity := level.Severity()
	if severity != issue.SEVERITY_IGNORE {
		for _, issue := range issues {
			if severity == issue.Severity() {
				levelIssues = append(levelIssues, issue)
			}
		}
	}
	return levelIssues
}

func matchEntries(b *bytes.Buffer, level eval.LogLevel, includes []*Include, excludes []*Exclude, entries []string, issues []*issue.Reported) {
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
			fmt.Fprintf(b, "Unexpected %s('%s')\n", level, str)
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
			fmt.Fprintf(b, "Unexpected %s: %s\n", issue.Code(), issue.String())
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

func (i *Include) matchIssue(issue *issue.Reported) bool {
	for _, m := range i.matchers {
		if m.MatchIssue(issue) {
			return true
		}
	}
	return false
}

func (i *Include) matchExpectedIncludes(b *bytes.Buffer, level eval.LogLevel, strings []string, issues []*issue.Reported) {
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
		fmt.Fprintf(b, "Expected %s(%s) but it was not produced\n", level, m.String())
	}
}

func (e *Exclude) matchLogEntry(b *bytes.Buffer, level eval.LogLevel, str string) bool {
	for _, m := range e.matchers {
		if m.MatchString(str) {
			fmt.Fprintf(b, "%s(%s) matches exclusion %s\n", level, str, m.String())
			return true
		}
	}
	return false
}

func (e *Exclude) matchIssue(b *bytes.Buffer, issue *issue.Reported) bool {
	excluded := false
	for _, m := range e.matchers {
		if m.MatchIssue(issue) {
			fmt.Fprintf(b, "%s matches exclusion %s\n", issue.String(), m.String())
			excluded = true
		}
	}
	return excluded
}

func (im *IssueMatch) MatchString(str string) bool {
	return string(im.issue.Code()) == str
}

func (im *IssueMatch) MatchIssue(issue *issue.Reported) bool {
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
			if !eval.Equals(a, eval.WrapUnknown(v)) {
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

func (rm *RegexpMatch) MatchIssue(issue *issue.Reported) bool {
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

func (sm *StringMatch) MatchIssue(issue *issue.Reported) bool {
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

var MATCH_TYPE = types.NewGoRuntimeType([]Match{})
var ISSUE_TYPE = types.NewGoRuntimeType([]*issue.Issue{})
var INCLUDE_TYPE = types.NewGoRuntimeType([]*Include{})
var EXCLUDE_TYPE = types.NewGoRuntimeType([]*Exclude{})
var EXPECTATION_TYPE = types.NewGoRuntimeType([]*Expectation{})
var MATCH_ARG_TYPE = types.NewVariantType([]eval.PType{types.DefaultStringType(), types.DefaultRegexpType(), ISSUE_TYPE})
var MATCHERS_TYPE = types.NewVariantType([]eval.PType{types.DefaultStringType(), types.DefaultRegexpType(), ISSUE_TYPE, MATCH_TYPE})
var EXPECTATIONS_TYPE = types.NewVariantType([]eval.PType{types.DefaultStringType(), types.DefaultRegexpType(), ISSUE_TYPE, MATCH_TYPE, INCLUDE_TYPE, EXCLUDE_TYPE})

func makeMatches(name string, args []eval.PValue) (result []Match) {
	result = make([]Match, len(args))
	for ix, arg := range args {
		switch arg.(type) {
		case *types.StringValue:
			result[ix] = &StringMatch{false, arg.String()}
			continue
		case *types.RegexpValue:
			result[ix] = &RegexpMatch{arg.(*types.RegexpValue).Regexp()}
			continue
		case *types.RuntimeValue:
			x := arg.(*types.RuntimeValue).Interface()
			switch x.(type) {
			case *issue.Issue:
				result[ix] = &IssueMatch{x.(*issue.Issue), nil}
				continue
			case Match:
				result[ix] = x.(Match)
				continue
			}
		}
		panic(types.NewIllegalArgumentType2(name, ix, `Variant[String,Regexp,Issue,Match]`, arg))
	}
	return
}

func makeIssueArgMatch(arg eval.PValue) interface{} {
	switch arg.(type) {
	case *types.StringValue:
		return &StringMatch{false, arg.String()}
	case *types.RegexpValue:
		return &RegexpMatch{arg.(*types.RegexpValue).Regexp()}
	case *types.RuntimeValue:
		return arg.(*types.RuntimeValue).Interface()
	}
	return arg
}

func makeExpectations(name string, level eval.LogLevel, args []eval.PValue) (result []*LevelExpectation) {
	result = make([]*LevelExpectation, len(args))
	for ix, arg := range args {
		switch arg.(type) {
		case *types.StringValue:
			result[ix] = &LevelExpectation{level: level, includes: []*Include{{[]Match{&StringMatch{false, arg.String()}}}}}
			continue
		case *types.RegexpValue:
			result[ix] = &LevelExpectation{level: level, includes: []*Include{{[]Match{&RegexpMatch{arg.(*types.RegexpValue).Regexp()}}}}}
			continue
		case *types.RuntimeValue:
			x := arg.(*types.RuntimeValue).Interface()
			switch x.(type) {
			case *issue.Issue:
				result[ix] = &LevelExpectation{level: level, includes: []*Include{{[]Match{&IssueMatch{x.(*issue.Issue), nil}}}}}
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
		panic(types.NewIllegalArgumentType2(name, ix, `Variant[String,Regexp,Issue,Match,Include,Exclude]`, arg))
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
		validateExpectations(assertions, e.expectations, issues, evaluator.Logger().(*eval.ArrayLogger))
	}
}

func (e *EvaluatesWith) setExample(example *Example) {
	e.example = example
}

func (v *ValidatesWith) CreateTest(actual interface{}) Executable {
	path, source := pathAndContent(actual)
	return func(tc *TestContext, assertions Assertions) {
		_, issues := parseAndValidate(path, source, true)
		validateExpectations(assertions, v.expectations, issues, eval.NewArrayLogger())
	}
}

func (v *ValidatesWith) setExample(example *Example) {
	v.example = example
}

func validateExpectations(assertions Assertions, expectations []*Expectation, issues []*issue.Reported, log *eval.ArrayLogger) {
	bld := bytes.NewBufferString(``)
	for _, ex := range expectations {
		ex.MatchEntries(bld, log, issues)
	}
	if bld.Len() > 0 {
		assertions.Fail(bld.String())
	}
}

func init() {

	eval.NewGoConstructor(`PSpec::Exclude`,
		func(d eval.Dispatch) {
			d.RepeatedParam2(MATCHERS_TYPE)
			d.Function(func(c eval.EvalContext, args []eval.PValue) eval.PValue {
				return types.WrapRuntime(&Exclude{makeMatches(`Exclude`, args)})
			})
		})

	eval.NewGoConstructor(`PSpec::Include`,
		func(d eval.Dispatch) {
			d.RepeatedParam2(MATCHERS_TYPE)
			d.Function(func(c eval.EvalContext, args []eval.PValue) eval.PValue {
				return types.WrapRuntime(&Include{makeMatches(`Include`, args)})
			})
		})

	eval.NewGoConstructor(`PSpec::Contain`,
		func(d eval.Dispatch) {
			d.Param(`String`)
			d.Function(func(c eval.EvalContext, args []eval.PValue) eval.PValue {
				return types.WrapRuntime(&StringMatch{true, args[0].String()})
			})
		})

	eval.NewGoConstructor(`PSpec::Issue`,
		func(d eval.Dispatch) {
			d.Param2(types.NewGoRuntimeType([]*issue.Issue{}))
			d.OptionalParam(`Hash[String,Any]`)
			d.Function(func(c eval.EvalContext, args []eval.PValue) eval.PValue {
				var argsMap *hash.StringHash
				if len(args) > 1 {
					argsMap = hash.NewStringHash(5)
					args[1].(*types.HashValue).EachPair(func(k, v eval.PValue) {
						argsMap.Put(k.String(), makeIssueArgMatch(v))
					})
				}
				return types.WrapRuntime(&IssueMatch{issue: args[0].(*types.RuntimeValue).Interface().(*issue.Issue), argsMap: argsMap})
			})
		})

	eval.NewGoConstructor(`PSpec::Match`,
		func(d eval.Dispatch) {
			d.Param2(MATCH_ARG_TYPE)
			d.Function(func(c eval.EvalContext, args []eval.PValue) eval.PValue {
				return types.WrapRuntime(makeMatches(`Match`, args)[0])
			})
		})

	eval.NewGoConstructor(`PSpec::Error`,
		func(d eval.Dispatch) {
			d.RepeatedParam2(EXPECTATIONS_TYPE)
			d.Function(func(c eval.EvalContext, args []eval.PValue) eval.PValue {
				return types.WrapRuntime(&Expectation{makeExpectations(`Error`, eval.ERR, args)})
			})
		})

	eval.NewGoConstructor(`PSpec::Notice`,
		func(d eval.Dispatch) {
			d.RepeatedParam2(EXPECTATIONS_TYPE)
			d.Function(func(c eval.EvalContext, args []eval.PValue) eval.PValue {
				return types.WrapRuntime(&Expectation{makeExpectations(`Notice`, eval.NOTICE, args)})
			})
		})

	eval.NewGoConstructor(`PSpec::Warning`,
		func(d eval.Dispatch) {
			d.RepeatedParam2(EXPECTATIONS_TYPE)
			d.Function(func(c eval.EvalContext, args []eval.PValue) eval.PValue {
				return types.WrapRuntime(&Expectation{makeExpectations(`Warning`, eval.WARNING, args)})
			})
		})

	eval.NewGoConstructor(`PSpec::Evaluates_ok`,
		func(d eval.Dispatch) {
			d.Function(func(c eval.EvalContext, args []eval.PValue) eval.PValue {
				return types.WrapRuntime(&EvaluatesWith{nil, []*Expectation{EXPECT_OK}})
			})
		})

	eval.NewGoConstructor(`PSpec::Evaluates_to`,
		func(d eval.Dispatch) {
			d.Param(`Any`)
			d.Function(func(c eval.EvalContext, args []eval.PValue) eval.PValue {
				return types.WrapRuntime(&EvaluationResult{nil, args[0]})
			})
		})

	eval.NewGoConstructor(`PSpec::Evaluates_with`,
		func(d eval.Dispatch) {
			d.RepeatedParam2(EXPECTATION_TYPE)
			d.Function(func(c eval.EvalContext, args []eval.PValue) eval.PValue {
				argc := len(args)
				results := make([]*Expectation, argc)
				for idx := 0; idx < argc; idx++ {
					results[idx] = args[idx].(*types.RuntimeValue).Interface().(*Expectation)
				}
				return types.WrapRuntime(&EvaluatesWith{nil, results})
			})
		},

		func(d eval.Dispatch) {
			d.RepeatedParam2(EXPECTATIONS_TYPE)
			d.Function(func(c eval.EvalContext, args []eval.PValue) eval.PValue {
				return types.WrapRuntime(&EvaluatesWith{nil, []*Expectation{&Expectation{makeExpectations(`Error`, eval.ERR, args)}}})
			})
		})

	eval.NewGoConstructor(`PSpec::Parses_to`,
		func(d eval.Dispatch) {
			d.Param(`String`)
			d.Function(func(c eval.EvalContext, args []eval.PValue) eval.PValue {
				return types.WrapRuntime(&ParseResult{location: c.StackTop(), expected: args[0].String()})
			})
		})

	eval.NewGoConstructor(`PSpec::Validates_ok`,
		func(d eval.Dispatch) {
			d.Function(func(c eval.EvalContext, args []eval.PValue) eval.PValue {
				return types.WrapRuntime(&ValidatesWith{nil, []*Expectation{EXPECT_OK}})
			})
		})

	eval.NewGoConstructor(`PSpec::Validates_with`,
		func(d eval.Dispatch) {
			d.RepeatedParam2(EXPECTATION_TYPE)
			d.Function(func(c eval.EvalContext, args []eval.PValue) eval.PValue {
				argc := len(args)
				results := make([]*Expectation, argc)
				for idx := 0; idx < argc; idx++ {
					results[idx] = args[idx].(*types.RuntimeValue).Interface().(*Expectation)
				}
				return types.WrapRuntime(&ValidatesWith{nil, results})
			})
		},

		func(d eval.Dispatch) {
			d.RepeatedParam2(EXPECTATIONS_TYPE)
			d.Function(func(c eval.EvalContext, args []eval.PValue) eval.PValue {
				return types.WrapRuntime(&ValidatesWith{nil, []*Expectation{&Expectation{makeExpectations(`Error`, eval.ERR, args)}}})
			})
		})
}
