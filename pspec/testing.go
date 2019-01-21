package pspec

import (
	"github.com/lyraproj/puppet-evaluator/eval"
	"github.com/lyraproj/puppet-evaluator/impl"
	"github.com/lyraproj/puppet-evaluator/types"
	"github.com/lyraproj/issue/issue"
	"github.com/lyraproj/puppet-parser/parser"
	"github.com/lyraproj/puppet-parser/validator"
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
		accessedValues map[int64]eval.Value
		tearDowns      []Housekeeping
		scope          eval.Scope
		parserOptions  eval.OrderedMap
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

func (tc *TestContext) Get(l LazyComputedValue) eval.Value {
	if v, ok := tc.accessedValues[l.Id()]; ok {
		return v
	}

	v := l.Get(tc)
	tc.accessedValues[l.Id()] = v
	return v
}

func (tc *TestContext) DoWithContext(doer func(eval.Context)) {
	c := impl.NewContext(impl.NewEvaluator, eval.NewParentedLoader(eval.Puppet.EnvironmentLoader()), eval.NewArrayLogger())
	eval.DoWithContext(c, func(c eval.Context) {
		c.DoWithScope(tc.newLazyScope(), func() {
			doer(c)
		})
	})
}

func (tc *TestContext) ParserOptions() []parser.Option {
	o := []parser.Option{}
	if tc.parent != nil {
		o = append(o, tc.parent.ParserOptions()...)
	}
	if tc.parserOptions != nil {
		tc.parserOptions.EachPair(func(k, v eval.Value) {
			switch k.String() {
			case `tasks`:
				if b, ok := v.(eval.BooleanValue); ok && b.Bool() {
					o = append(o, parser.PARSER_TASKS_ENABLED)
				}
			case `hex_escapes`:
				if b, ok := v.(eval.BooleanValue); ok && b.Bool() {
					o = append(o, parser.PARSER_HANDLE_HEX_ESCAPES)
				}
			case `backtick_strings`:
				if b, ok := v.(eval.BooleanValue); ok && b.Bool() {
					o = append(o, parser.PARSER_HANDLE_BACKTICK_STRINGS)
				}
			}
		})
	}
	return o
}

func (tc *TestContext) newLazyScope() *LazyScope {
	return &LazyScope{*tc.Scope().(*impl.BasicScope), tc}
}

func (tc *TestContext) Scope() eval.Scope {
	if tc.scope == nil {
		tc.scope = impl.NewScope(false)
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

func (tc *TestContext) resolveLazyValue(v eval.Value) eval.Value {
	switch v.(type) {
	case *types.RuntimeValue:
		if lv, ok := v.(*types.RuntimeValue).Interface().(LazyComputedValue); ok {
			return tc.Get(lv)
		}
		if lg, ok := v.(*types.RuntimeValue).Interface().(*LazyValueGet); ok {
			return lg.Get(tc)
		}
		return v
	case *types.HashValue:
		oe := v.(*types.HashValue)
		ne := make([]*types.HashEntry, oe.Len())
		oe.EachWithIndex(func(v eval.Value, i int) {
			e := v.(*types.HashEntry)
			ne[i] = types.WrapHashEntry(tc.resolveLazyValue(e.Key()), tc.resolveLazyValue(e.Value()))
		})
		return types.WrapHash(ne)
	case *types.ArrayValue:
		return types.WrapValues(tc.resolveLazyValues(v.(*types.ArrayValue)))
	default:
		return v
	}
}

func (tc *TestContext) resolveLazyValues(values eval.List) []eval.Value {
	resolved := make([]eval.Value, values.Len())
	values.EachWithIndex(func(e eval.Value, i int) {
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
				eval.Puppet.Logger().Log(eval.ERR, types.WrapString(e.Error()))
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

func parseAndValidate(name, source string, singleExpression bool, o ...parser.Option) (parser.Expression, []issue.Reported) {
	expr, err := parser.CreateParser(o...).Parse(name, source, singleExpression)
	var issues []issue.Reported
	if err != nil {
		i, ok := err.(issue.Reported)
		if !ok {
			panic(err.Error())
		}
		issues = []issue.Reported{i}
	} else {
		issues = validator.ValidatePuppet(expr, validator.STRICT_ERROR).Issues()
	}
	return expr, issues
}

func evaluate(c eval.Context, expr parser.Expression) (eval.Value, []issue.Reported) {
	c.AddDefinitions(expr)
	result, i := eval.TopEvaluate(c, expr)
	issues := []issue.Reported{}
	if i != nil {
		issues = []issue.Reported{i}
	}
	return result, issues
}
