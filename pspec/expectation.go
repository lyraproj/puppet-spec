package pspec

import (
	"bytes"
	"github.com/lyraproj/puppet-evaluator/pdsl"
	"regexp"
	"strings"

	"github.com/lyraproj/issue/issue"
	"github.com/lyraproj/pcore/hash"
	"github.com/lyraproj/pcore/px"
	"github.com/lyraproj/pcore/types"
	"github.com/lyraproj/pcore/utils"
	"github.com/lyraproj/puppet-parser/parser"
)

type (
	Expectation struct {
		levelExpectations []*LevelExpectation
	}

	LevelExpectation struct {
		level    px.LogLevel
		includes []*Include
		excludes []*Exclude
	}

	Match interface {
		MatchString(str string) bool
		MatchIssue(issue issue.Reported) bool
		String() string
	}

	Include struct {
		matchers []Match
	}

	Exclude struct {
		matchers []Match
	}

	IssueMatch struct {
		issue   issue.Issue
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

var expectOk = &Expectation{levelExpectations: []*LevelExpectation{}}

func (e *Expectation) MatchEntries(b *bytes.Buffer, log *px.ArrayLogger, allIssues []issue.Reported) {
	for _, level := range []px.LogLevel{px.NOTICE, px.WARNING, px.ERR} {
		entries := log.Entries(level)
		issues := issuesForLevel(allIssues, level)
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
			if re, ok := entry.(*px.ReportedEntry); ok {
				issues = append(issues, re.Issue())
			} else {
				texts = append(texts, entry.Message())
			}
		}
		matchEntries(b, level, includes, excludes, texts, issues)
	}
}

func issuesForLevel(issues []issue.Reported, level px.LogLevel) []issue.Reported {
	levelIssues := make([]issue.Reported, 0)
	severity := level.Severity()
	if severity != issue.SEVERITY_IGNORE {
		for _, i := range issues {
			if severity == i.Severity() {
				levelIssues = append(levelIssues, i)
			}
		}
	}
	return levelIssues
}

func matchEntries(b *bytes.Buffer, level px.LogLevel, includes []*Include, excludes []*Exclude, entries []string, issues []issue.Reported) {
nextStr:
	for _, str := range entries {
		for _, i := range includes {
			if i.matchEntry(str) {
				continue nextStr
			}
		}
		excluded := false
		for _, e := range excludes {
			if e.matchAppendEntry(b, level, str) {
				excluded = true
			}
		}
		if !excluded {
			utils.Fprintf(b, "Unexpected %s('%s')\n", level, str)
		}
	}

nextIssue:
	for _, is := range issues {
		for _, i := range includes {
			if i.matchIssue(is) {
				continue nextIssue
			}
		}
		excluded := false
		for _, e := range excludes {
			if e.matchAppendIssue(b, is) {
				excluded = true
			}
		}
		if !excluded {
			utils.Fprintf(b, "Unexpected %s: %s\n", is.Code(), is.String())
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

func (i *Include) matchIssue(issue issue.Reported) bool {
	for _, m := range i.matchers {
		if m.MatchIssue(issue) {
			return true
		}
	}
	return false
}

func (i *Include) matchExpectedIncludes(b *bytes.Buffer, level px.LogLevel, strings []string, issues []issue.Reported) {
nextMatch:
	for _, m := range i.matchers {
		for _, str := range strings {
			if m.MatchString(str) {
				continue nextMatch
			}
		}
		for _, is := range issues {
			if m.MatchIssue(is) {
				continue nextMatch
			}
		}
		utils.Fprintf(b, "Expected %s(%s) but it was not produced\n", level, m.String())
	}
}

func (e *Exclude) matchAppendEntry(b *bytes.Buffer, level px.LogLevel, str string) bool {
	for _, m := range e.matchers {
		if m.MatchString(str) {
			utils.Fprintf(b, "%s(%s) matches exclusion %s\n", level, str, m.String())
			return true
		}
	}
	return false
}

func (e *Exclude) matchAppendIssue(b *bytes.Buffer, issue issue.Reported) bool {
	excluded := false
	for _, m := range e.matchers {
		if m.MatchIssue(issue) {
			utils.Fprintf(b, "%s matches exclusion %s\n", issue.String(), m.String())
			excluded = true
		}
	}
	return excluded
}

func (im *IssueMatch) MatchString(str string) bool {
	return string(im.issue.Code()) == str
}

func (im *IssueMatch) MatchIssue(issue issue.Reported) bool {
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
			switch v := v.(type) {
			case string:
				if m.MatchString(v) {
					continue
				}
			case byte:
				if m.MatchString(string([]byte{v})) {
					continue
				}
			case rune:
				if m.MatchString(string([]rune{v})) {
					continue
				}
			}
			return false
		} else {
			if !px.Equals(a, px.Wrap(nil, v)) {
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

func (rm *RegexpMatch) MatchIssue(issue issue.Reported) bool {
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

func (sm *StringMatch) MatchIssue(issue issue.Reported) bool {
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

var matchType = types.NewGoRuntimeType((*Match)(nil))
var issueType = types.NewGoRuntimeType((*issue.Issue)(nil))
var includeType = types.NewGoRuntimeType(&Include{})
var excludeType = types.NewGoRuntimeType(&Exclude{})
var expectationType = types.NewGoRuntimeType(&Expectation{})
var matchArgType = types.NewVariantType(types.DefaultStringType(), types.DefaultRegexpType(), issueType)
var matchersType = types.NewVariantType(types.DefaultStringType(), types.DefaultRegexpType(), issueType, matchType)
var expectationsType = types.NewVariantType(types.DefaultStringType(), types.DefaultRegexpType(), issueType, matchType, includeType, excludeType)

func makeMatches(name string, args []px.Value) (result []Match) {
	result = make([]Match, len(args))
	for ix, arg := range args {
		switch arg := arg.(type) {
		case px.StringValue:
			result[ix] = &StringMatch{false, arg.String()}
			continue
		case *types.Regexp:
			result[ix] = &RegexpMatch{arg.Regexp()}
			continue
		case *types.RuntimeValue:
			x := arg.Interface()
			switch x.(type) {
			case issue.Issue:
				result[ix] = &IssueMatch{x.(issue.Issue), nil}
				continue
			case Match:
				result[ix] = x.(Match)
				continue
			}
		}
		panic(px.Error(px.IllegalArgumentType, issue.H{`function`: name, `index`: ix, `expected`: `Variant[String,Regexp,Issue,Match]`, `actual`: arg.PType()}))
	}
	return
}

func makeIssueArgMatch(arg px.Value) interface{} {
	switch arg := arg.(type) {
	case px.StringValue:
		return &StringMatch{false, arg.String()}
	case *types.Regexp:
		return &RegexpMatch{arg.Regexp()}
	case *types.RuntimeValue:
		return arg.Interface()
	}
	return arg
}

func makeExpectations(name string, level px.LogLevel, args []px.Value) (result []*LevelExpectation) {
	result = make([]*LevelExpectation, len(args))
	for ix, arg := range args {
		switch arg := arg.(type) {
		case px.StringValue:
			result[ix] = &LevelExpectation{level: level, includes: []*Include{{[]Match{&StringMatch{false, arg.String()}}}}}
			continue
		case *types.Regexp:
			result[ix] = &LevelExpectation{level: level, includes: []*Include{{[]Match{&RegexpMatch{arg.Regexp()}}}}}
			continue
		case *types.RuntimeValue:
			switch x := arg.Interface().(type) {
			case issue.Issue:
				result[ix] = &LevelExpectation{level: level, includes: []*Include{{[]Match{&IssueMatch{x, nil}}}}}
				continue
			case *Include:
				result[ix] = &LevelExpectation{level: level, includes: []*Include{x}}
				continue
			case *Exclude:
				result[ix] = &LevelExpectation{level: level, excludes: []*Exclude{x}}
				continue
			case Match:
				result[ix] = &LevelExpectation{level: level, includes: []*Include{{[]Match{x}}}}
				continue
			}
		}
		panic(px.Error(px.IllegalArgumentType, issue.H{`function`: name, `index`: ix, `expected`: `Variant[String,Regexp,Issue,Match,Include,Exclude]`, `actual`: arg.PType()}))
	}
	return
}

func (e *EvaluatesWith) CreateTest(actual interface{}) Executable {
	path, source, epp := pathContentAndEpp(actual)
	return func(tc *TestContext, assertions Assertions) {
		o := tc.ParserOptions()
		if epp {
			o = append(o, parser.PARSER_EPP_MODE)
		}
		actual, issues := parseAndValidate(path, tc.resolveLazyValue(source).String(), false, o...)
		tc.DoWithContext(func(c pdsl.EvaluationContext) {
			if !hasError(issues) {
				_, evalIssues := evaluate(c, actual)
				issues = append(issues, evalIssues...)
			}
			validateExpectations(assertions, e.expectations, issues, c.Logger().(*px.ArrayLogger))
		})
	}
}

func (e *EvaluatesWith) setExample(example *Example) {
	e.example = example
}

func (v *ValidatesWith) CreateTest(actual interface{}) Executable {
	path, source, epp := pathContentAndEpp(actual)
	return func(tc *TestContext, assertions Assertions) {
		o := tc.ParserOptions()
		if epp {
			o = append(o, parser.PARSER_EPP_MODE)
		}
		_, issues := parseAndValidate(path, tc.resolveLazyValue(source).String(), false, o...)
		validateExpectations(assertions, v.expectations, issues, px.NewArrayLogger())
	}
}

func (v *ValidatesWith) setExample(example *Example) {
	v.example = example
}

func validateExpectations(assertions Assertions, expectations []*Expectation, issues []issue.Reported, log *px.ArrayLogger) {
	bld := bytes.NewBufferString(``)
	for _, ex := range expectations {
		ex.MatchEntries(bld, log, issues)
	}
	if bld.Len() > 0 {
		assertions.Fail(bld.String())
	}
}

func init() {

	px.NewGoConstructor(`PSpec::Exclude`,
		func(d px.Dispatch) {
			d.RepeatedParam2(matchersType)
			d.Function(func(c px.Context, args []px.Value) px.Value {
				return types.WrapRuntime(&Exclude{makeMatches(`Exclude`, args)})
			})
		})

	px.NewGoConstructor(`PSpec::Include`,
		func(d px.Dispatch) {
			d.RepeatedParam2(matchersType)
			d.Function(func(c px.Context, args []px.Value) px.Value {
				return types.WrapRuntime(&Include{makeMatches(`Include`, args)})
			})
		})

	px.NewGoConstructor(`PSpec::Contain`,
		func(d px.Dispatch) {
			d.Param(`String`)
			d.Function(func(c px.Context, args []px.Value) px.Value {
				return types.WrapRuntime(&StringMatch{true, args[0].String()})
			})
		})

	px.NewGoConstructor(`PSpec::Issue`,
		func(d px.Dispatch) {
			d.Param2(types.NewGoRuntimeType((*issue.Issue)(nil)))
			d.OptionalParam(`Hash[String,Any]`)
			d.Function(func(c px.Context, args []px.Value) px.Value {
				var argsMap *hash.StringHash
				if len(args) > 1 {
					argsMap = hash.NewStringHash(5)
					args[1].(*types.Hash).EachPair(func(k, v px.Value) {
						argsMap.Put(k.String(), makeIssueArgMatch(v))
					})
				}
				return types.WrapRuntime(&IssueMatch{issue: args[0].(*types.RuntimeValue).Interface().(issue.Issue), argsMap: argsMap})
			})
		})

	px.NewGoConstructor(`PSpec::Match`,
		func(d px.Dispatch) {
			d.Param2(matchArgType)
			d.Function(func(c px.Context, args []px.Value) px.Value {
				return types.WrapRuntime(makeMatches(`Match`, args)[0])
			})
		})

	px.NewGoConstructor(`PSpec::Error`,
		func(d px.Dispatch) {
			d.RepeatedParam2(expectationsType)
			d.Function(func(c px.Context, args []px.Value) px.Value {
				return types.WrapRuntime(&Expectation{makeExpectations(`Error`, px.ERR, args)})
			})
		})

	px.NewGoConstructor(`PSpec::Notice`,
		func(d px.Dispatch) {
			d.RepeatedParam2(expectationsType)
			d.Function(func(c px.Context, args []px.Value) px.Value {
				return types.WrapRuntime(&Expectation{makeExpectations(`Notice`, px.NOTICE, args)})
			})
		})

	px.NewGoConstructor(`PSpec::Warning`,
		func(d px.Dispatch) {
			d.RepeatedParam2(expectationsType)
			d.Function(func(c px.Context, args []px.Value) px.Value {
				return types.WrapRuntime(&Expectation{makeExpectations(`Warning`, px.WARNING, args)})
			})
		})

	px.NewGoConstructor(`PSpec::Evaluates_ok`,
		func(d px.Dispatch) {
			d.Function(func(c px.Context, args []px.Value) px.Value {
				return types.WrapRuntime(&EvaluatesWith{nil, []*Expectation{expectOk}})
			})
		})

	px.NewGoConstructor(`PSpec::Evaluates_to`,
		func(d px.Dispatch) {
			d.Param(`Any`)
			d.Function(func(c px.Context, args []px.Value) px.Value {
				return types.WrapRuntime(&EvaluationResult{nil, args[0]})
			})
		})

	px.NewGoConstructor(`PSpec::Evaluates_with`,
		func(d px.Dispatch) {
			d.RepeatedParam2(expectationType)
			d.Function(func(c px.Context, args []px.Value) px.Value {
				argc := len(args)
				results := make([]*Expectation, argc)
				for idx := 0; idx < argc; idx++ {
					results[idx] = args[idx].(*types.RuntimeValue).Interface().(*Expectation)
				}
				return types.WrapRuntime(&EvaluatesWith{nil, results})
			})
		},

		func(d px.Dispatch) {
			d.RepeatedParam2(expectationsType)
			d.Function(func(c px.Context, args []px.Value) px.Value {
				return types.WrapRuntime(&EvaluatesWith{nil, []*Expectation{{makeExpectations(`Error`, px.ERR, args)}}})
			})
		})

	px.NewGoConstructor(`PSpec::Parses_to`,
		func(d px.Dispatch) {
			d.Param(`String`)
			d.Function(func(c px.Context, args []px.Value) px.Value {
				return types.WrapRuntime(&ParseResult{location: c.StackTop(), expected: args[0].String()})
			})
		})

	px.NewGoConstructor(`PSpec::Validates_ok`,
		func(d px.Dispatch) {
			d.Function(func(c px.Context, args []px.Value) px.Value {
				return types.WrapRuntime(&ValidatesWith{nil, []*Expectation{expectOk}})
			})
		})

	px.NewGoConstructor(`PSpec::Validates_with`,
		func(d px.Dispatch) {
			d.RepeatedParam2(expectationType)
			d.Function(func(c px.Context, args []px.Value) px.Value {
				argc := len(args)
				results := make([]*Expectation, argc)
				for idx := 0; idx < argc; idx++ {
					results[idx] = args[idx].(*types.RuntimeValue).Interface().(*Expectation)
				}
				return types.WrapRuntime(&ValidatesWith{nil, results})
			})
		},

		func(d px.Dispatch) {
			d.RepeatedParam2(expectationsType)
			d.Function(func(c px.Context, args []px.Value) px.Value {
				return types.WrapRuntime(&ValidatesWith{nil, []*Expectation{{makeExpectations(`Error`, px.ERR, args)}}})
			})
		})
}
