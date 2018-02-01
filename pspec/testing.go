package pspec

import (
	"github.com/puppetlabs/go-evaluator/eval"
	"github.com/puppetlabs/go-evaluator/impl"
	"github.com/puppetlabs/go-evaluator/types"
	"github.com/puppetlabs/go-parser/issue"
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
		accessedValues map[int64]eval.PValue
		tearDowns      []Housekeeping
		scope          eval.Scope
		loader         eval.Loader
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

func (tc *TestContext) Get(l LazyValue) eval.PValue {
	if v, ok := tc.accessedValues[l.Id()]; ok {
		return v
	}

	v := l.Get(tc)
	tc.accessedValues[l.Id()] = v
	return v
}

func (tc *TestContext) newLazyScope() *LazyScope {
	return &LazyScope{*tc.scope.(*impl.BasicScope), tc}
}

func (tc *TestContext) Scope() eval.Scope {
	if tc.scope == nil {
		tc.scope = impl.NewScope()
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

func (tc *TestContext) resolveLazyValue(v eval.PValue) eval.PValue {
	switch v.(type) {
	case *types.RuntimeValue:
		if lv, ok := v.(*types.RuntimeValue).Interface().(LazyValue); ok {
			return tc.Get(lv)
		}
		if lg, ok := v.(*types.RuntimeValue).Interface().(*LazyValueGet); ok {
			return lg.Get(tc)
		}
		return v
	case *types.HashValue:
		oe := v.(*types.HashValue)
		ne := make([]*types.HashEntry, oe.Len())
		oe.EachWithIndex(func(v eval.PValue, i int) {
			e := v.(*types.HashEntry)
			ne[i] = types.WrapHashEntry(tc.resolveLazyValue(e.Key()), tc.resolveLazyValue(e.Value()))
		})
		return types.WrapHash(ne)
	case *types.ArrayValue:
		return types.WrapArray(tc.resolveLazyValues(v.(*types.ArrayValue)))
	default:
		return v
	}
}

func (tc *TestContext) resolveLazyValues(values eval.IndexedValue) []eval.PValue {
	resolved := make([]eval.PValue, values.Len())
	values.EachWithIndex(func(e eval.PValue, i int) {
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
	eval.Puppet.Reset()
	v.test(ctx, assertions)
	for i := len(ctx.tearDowns) - 1; i >= 0; i-- {
		safeHousekeeping(ctx.tearDowns[i])
	}
}

func safeHousekeeping(h Housekeeping) {
	defer func() {
		if err := recover(); err != nil {
			if e, ok := err.(error); ok {
				eval.CurrentContext().Logger().Log(eval.ERR, types.WrapString(e.Error()))
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

func parseAndValidate(name, source string, singleExpression bool) (Expression, []*issue.Reported) {
	expr, err := CreateParser().Parse(name, source, singleExpression)
	var issues []*issue.Reported
	if err != nil {
		i, ok := err.(*issue.Reported)
		if !ok {
			panic(err.Error())
		}
		issues = []*issue.Reported{i}
	} else {
		checker := NewChecker(STRICT_ERROR)
		checker.Validate(expr)
		issues = checker.Issues()
	}
	return expr, issues
}

func evaluate(evaluator eval.Evaluator, expr Expression, scope eval.Scope) (eval.PValue, []*issue.Reported) {
	evaluator.AddDefinitions(expr)
	result, i := evaluator.Evaluate(expr, scope, nil)
	issues := []*issue.Reported{}
	if i != nil {
		issues = []*issue.Reported{i}
	}
	return result, issues
}
