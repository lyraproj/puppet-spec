package pspec

import (
	"github.com/lyraproj/issue/issue"
	"github.com/lyraproj/pcore/pcore"
	"github.com/lyraproj/pcore/px"
	"github.com/lyraproj/pcore/types"
	"github.com/lyraproj/puppet-evaluator/evaluator"
	"github.com/lyraproj/puppet-evaluator/pdsl"
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
		accessedValues map[int64]px.Value
		tearDowns      []Housekeeping
		scope          pdsl.Scope
		parserOptions  px.OrderedMap
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

func (tc *TestContext) Get(l LazyComputedValue) px.Value {
	if v, ok := tc.accessedValues[l.Id()]; ok {
		return v
	}

	v := l.Get(tc)
	tc.accessedValues[l.Id()] = v
	return v
}

func (tc *TestContext) DoWithContext(doer func(pdsl.EvaluationContext)) {
	c := evaluator.NewContext(evaluator.NewEvaluator, px.NewParentedLoader(pcore.EnvironmentLoader()), px.NewArrayLogger())
	px.DoWithContext(c, func(c px.Context) {
		ec := c.(pdsl.EvaluationContext)
		ec.DoWithScope(tc.newLazyScope(), func() {
			doer(ec)
		})
	})
}

func (tc *TestContext) ParserOptions() []parser.Option {
	o := make([]parser.Option, 0)
	if tc.parent != nil {
		o = append(o, tc.parent.ParserOptions()...)
	}
	if tc.parserOptions != nil {
		tc.parserOptions.EachPair(func(k, v px.Value) {
			switch k.String() {
			case `tasks`:
				if b, ok := v.(px.Boolean); ok && b.Bool() {
					o = append(o, parser.PARSER_TASKS_ENABLED)
				}
			case `hex_escapes`:
				if b, ok := v.(px.Boolean); ok && b.Bool() {
					o = append(o, parser.PARSER_HANDLE_HEX_ESCAPES)
				}
			case `backtick_strings`:
				if b, ok := v.(px.Boolean); ok && b.Bool() {
					o = append(o, parser.PARSER_HANDLE_BACKTICK_STRINGS)
				}
			}
		})
	}
	return o
}

func (tc *TestContext) newLazyScope() *LazyScope {
	return &LazyScope{*tc.Scope().(*evaluator.BasicScope), tc}
}

func (tc *TestContext) Scope() pdsl.Scope {
	if tc.scope == nil {
		tc.scope = evaluator.NewScope(false)
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

func (tc *TestContext) resolveLazyValue(v px.Value) px.Value {
	switch v.(type) {
	case *types.RuntimeValue:
		if lv, ok := v.(*types.RuntimeValue).Interface().(LazyComputedValue); ok {
			return tc.Get(lv)
		}
		if lg, ok := v.(*types.RuntimeValue).Interface().(*LazyValueGet); ok {
			return lg.Get(tc)
		}
		return v
	case *types.Hash:
		oe := v.(*types.Hash)
		ne := make([]*types.HashEntry, oe.Len())
		oe.EachWithIndex(func(v px.Value, i int) {
			e := v.(*types.HashEntry)
			ne[i] = types.WrapHashEntry(tc.resolveLazyValue(e.Key()), tc.resolveLazyValue(e.Value()))
		})
		return types.WrapHash(ne)
	case *types.Array:
		return types.WrapValues(tc.resolveLazyValues(v.(*types.Array)))
	default:
		return v
	}
}

func (tc *TestContext) resolveLazyValues(values px.List) []px.Value {
	resolved := make([]px.Value, values.Len())
	values.EachWithIndex(func(e px.Value, i int) {
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
	pcore.Reset()
	v.test(ctx, assertions)
	for i := len(ctx.tearDowns) - 1; i >= 0; i-- {
		safeHousekeeping(ctx.tearDowns[i])
	}
}

func safeHousekeeping(h Housekeeping) {
	defer func() {
		if err := recover(); err != nil {
			if e, ok := err.(error); ok {
				pcore.Logger().Log(px.ERR, types.WrapString(e.Error()))
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

func evaluate(c pdsl.EvaluationContext, expr parser.Expression) (result px.Value, issues []issue.Reported) {
	defer func() {
		if r := recover(); r != nil {
			if err, ok := r.(issue.Reported); ok {
				issues = []issue.Reported{err}
			} else {
				panic(r)
			}
		}
	}()

	issues = []issue.Reported{}
	c.AddDefinitions(expr)
	result = pdsl.TopEvaluate(c, expr)
	return
}
