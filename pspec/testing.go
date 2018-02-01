package pspec

import (
	. "github.com/puppetlabs/go-evaluator/eval"
	. "github.com/puppetlabs/go-evaluator/impl"
	. "github.com/puppetlabs/go-evaluator/types"
	. "github.com/puppetlabs/go-parser/issue"
	. "github.com/puppetlabs/go-parser/parser"
	. "github.com/puppetlabs/go-parser/validator"
)

type (
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
		parent         *TestContext
		node           Node
		accessedValues map[int64]PValue
		tearDowns      []Housekeeping
		scope          Scope
		loader         Loader
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

func (tc *TestContext) newLazyScope() *LazyScope {
	return &LazyScope{*tc.scope.(*BasicScope), tc}
}

func (tc *TestContext) Scope() Scope {
	if tc.scope == nil {
		tc.scope = NewScope()
	}
	return tc.scope
}

func (tc *TestContext) getLazyValue(key string) (LazyValue, bool) {
	v, ok := tc.node.Get(key)
	if ok {
		return v, true
	}
	if tc.parent == nil {
		return nil, false
	}
	return tc.parent.getLazyValue(key)
}

func (tc *TestContext) registerTearDown(td Housekeeping) {
	tc.tearDowns = append(tc.tearDowns, td)
}

func (tc *TestContext) resolveLazyValue(v PValue) PValue {
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
		oe := v.(*HashValue)
		ne := make([]*HashEntry, oe.Len())
		oe.EachWithIndex(func(v PValue, i int) {
			e := v.(*HashEntry)
			ne[i] = WrapHashEntry(tc.resolveLazyValue(e.Key()), tc.resolveLazyValue(e.Value()))
		})
		return WrapHash(ne)
	case *ArrayValue:
		return WrapArray(tc.resolveLazyValues(v.(*ArrayValue)))
	default:
		return v
	}
}

func (tc *TestContext) resolveLazyValues(values IndexedValue) []PValue {
	resolved := make([]PValue, values.Len())
	values.EachWithIndex(func(e PValue, i int) {
		resolved[i] = tc.resolveLazyValue(e)
	})
	return resolved
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
	expr, err := CreateParser().Parse(name, source, singleExpression)
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
