package pspec

import (
	. "github.com/puppetlabs/go-evaluator/evaluator"
	. "github.com/puppetlabs/go-parser/parser"
	. "github.com/puppetlabs/go-parser/issue"
	. "github.com/puppetlabs/go-parser/validator"
	. "github.com/puppetlabs/go-evaluator/pcore"
	. "github.com/puppetlabs/go-evaluator/types"
	. "github.com/puppetlabs/go-evaluator/eval"
)

type(
	Assertions interface {
		AssertEquals(a interface{}, b interface{})

		Fail(message string)
	}

	Executable func(context *TestContext, assertions Assertions)

	Housekeeping func()

	Test interface {
		Name() string
		Node() Node
	}

	TestContext struct {
		parent *TestContext
		node Node
		accessedValues map[int64]PValue
		tearDowns []Housekeeping
		scope Scope
	}

	testNode struct {
		node Node
	}

	TestExecutable struct {
		testNode
		test Executable
	}

	TestGroup struct {
		testNode
		tests []Test
	}
)

func (tc *TestContext) Get(l LazyValue) PValue {
	if v, ok := tc.accessedValues[l.Id()]; ok {
		return v
	}

	v := l.Get(tc)
	tc.accessedValues[l.Id()] = v
	return v
}

func (tc *TestContext) Scope() Scope {
	if tc.scope == nil {
		tc.scope = NewScope()
	}
	return tc.scope
}

func (tc *TestContext) getLazyValue(key string) LazyValue {
	v, ok := tc.node.Get(key)
	if ok {
		return v
	}
	if tc.parent == nil {
		panic(Error(PSPEC_GET_OF_UNKNOWN_VARIABLE, H{`name`: key}))
	}
	return tc.parent.getLazyValue(key)
}

func (tc* TestContext) registerTearDown(td Housekeeping) {
	tc.tearDowns = append(tc.tearDowns, td)
}

func (tc *TestContext) resolveLazyValues(v PValue) PValue {
	switch v.(type) {
	case *RuntimeValue:
		if lv, ok := v.(*RuntimeValue).Interface().(LazyValue); ok {
			return tc.Get(lv)
		}
		if lg, ok := v.(*RuntimeValue).Interface().(*LazyValueGet); ok {
			return lg.Get(tc)
		}
		return v
	case *HashValue:
		oe := v.(*HashValue).EntriesSlice()
		ne := make([]*HashEntry, len(oe))
		for i, e := range oe {
			ne[i] = WrapHashEntry(tc.resolveLazyValues(e.Key()), tc.resolveLazyValues(e.Value()))
		}
		return WrapHash(ne)
	case *ArrayValue:
		oe := v.(*ArrayValue).Elements()
		ne := make([]PValue, len(oe))
		for i, e := range oe {
			ne[i] = tc.resolveLazyValues(e)
		}
		return WrapArray(ne)
	default:
		return v
	}
}

func (v *testNode) Name() string {
	return v.node.Description()
}

func (v *testNode) Node() Node {
	return v.node
}

func (v *TestExecutable) Executable() Executable {
	return v.test
}

func (v *TestExecutable) Run(ctx *TestContext, assertions Assertions) {
	Puppet.Reset()
	v.test(ctx, assertions)
	for i := len(ctx.tearDowns) - 1; i >= 0; i-- {
		safeHousekeeping(ctx.tearDowns[i])
	}
}

func safeHousekeeping(h Housekeeping) {
	defer func() {
		if err := recover(); err != nil {
			if e, ok := err.(error); ok {
				CurrentContext().Logger().Log(ERR, WrapString(e.Error()))
			} else {
				panic(err)
			}
		}
	}()
	h()
}

func (v *TestGroup) Tests() []Test {
	return v.tests
}

func parseAndValidate(name, source string, singleExpression bool) (Expression, []*ReportedIssue) {
	expr, err := CreateParser().Parse(name, source, false, singleExpression)
	var issues []*ReportedIssue
	if err != nil {
		issue, ok := err.(*ReportedIssue)
		if !ok {
			panic(err.Error())
		}
		issues = []*ReportedIssue{issue}
	} else {
		checker := NewChecker(STRICT_ERROR)
		checker.Validate(expr)
		issues = checker.Issues()
	}
	return expr, issues
}

func evaluate(evaluator Evaluator, expr Expression, scope Scope) (PValue, []*ReportedIssue) {
	evaluator.AddDefinitions(expr)
	result, issue := evaluator.Evaluate(expr, scope, nil)
	issues := []*ReportedIssue{}
	if issue != nil {
		issues = []*ReportedIssue{issue}
	}
	return result, issues
}
