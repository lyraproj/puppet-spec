package pspec

import (
	"fmt"

	"github.com/lyraproj/issue/issue"
	"github.com/lyraproj/pcore/pcore"
	"github.com/lyraproj/pcore/px"
	"github.com/lyraproj/pcore/types"
	"github.com/lyraproj/puppet-evaluator/evaluator"
	"github.com/lyraproj/puppet-evaluator/pdsl"
	"github.com/lyraproj/puppet-parser/parser"
)

type (
	Input interface {
		CreateTests(expected Result) []Executable
	}

	Node interface {
		Description() string
		Get(key string) (LazyValue, bool)
		CreateTest() Test
		collectInputs(context *TestContext, inputs []Input) []Input
	}

	Result interface {
		CreateTest(actual interface{}) Executable

		setExample(example *Example)
	}

	ResultEntry interface {
		Match() px.Value
	}

	node struct {
		description string
		values      map[string]LazyValue
		given       *Given
	}

	Example struct {
		node
		results []Result
	}

	Examples struct {
		node
		children []Node
	}

	Given struct {
		inputs []Input
	}

	ParseResult struct {
		// ParseResult needs a location so that it can provide that to the PN parser
		location issue.Location
		example  *Example
		expected string
	}

	EvaluationResult struct {
		example  *Example
		expected px.Value
	}

	source struct {
		code px.Value
		epp  bool
	}

	Source struct {
		sources []*source
	}

	NamedSource struct {
		source
		name string
	}

	ParserOptions struct {
		options px.OrderedMap
	}

	SettingsInput struct {
		settings px.Value
	}

	ScopeInput struct {
		scope px.Value
	}
)

func pathContentAndEpp(src interface{}) (path string, content px.Value, epp bool) {
	switch src := src.(type) {
	case *source:
		return ``, src.code, src.epp
	case *NamedSource:
		return src.name, src.code, src.epp
	default:
		panic(px.Error(px.Failure, issue.H{`message`: fmt.Sprintf(`Unknown source type %T`, src)}))
	}
}

func (e *EvaluationResult) CreateTest(actual interface{}) Executable {
	path, source, epp := pathContentAndEpp(actual)

	return func(context *TestContext, assertions Assertions) {
		o := context.ParserOptions()
		if epp {
			o = append(o, parser.EppMode)
		}
		actual, issues := parseAndValidate(path, context.resolveLazyValue(source).String(), false, o...)
		failOnError(assertions, issues)
		context.DoWithContext(func(c pdsl.EvaluationContext) {
			actualResult, evalIssues := evaluate(c, actual)
			failOnError(assertions, evalIssues)
			assertions.AssertEquals(context.resolveLazyValue(e.expected), actualResult)
		})
	}
}

func (e *EvaluationResult) setExample(example *Example) {
	e.example = example
}

func (n *node) initialize(description string, given *Given) {
	n.description = description
	n.given = given
	n.values = make(map[string]LazyValue, 8)
}

func (n *node) addLetDefs(lazyValueLets []*LazyValueLet) {
	for _, ll := range lazyValueLets {
		n.values[ll.valueName] = ll.value
	}
}

func newExample(description string, given *Given, results []Result) *Example {
	e := &Example{results: results}
	e.node.initialize(description, given)
	return e
}

func newExamples(description string, given *Given, children []Node) *Examples {
	e := &Examples{children: children}
	e.node.initialize(description, given)
	return e
}

func (n *node) collectInputs(context *TestContext, inputs []Input) []Input {
	pc := context.parent
	if pc != nil {
		inputs = pc.node.collectInputs(pc, inputs)
	}
	g := n.given
	if g != nil {
		inputs = append(inputs, g.inputs...)
	}
	return inputs
}

func (e *Example) CreateTest() Test {
	test := func(context *TestContext, assertions Assertions) {
		tests := make([]Executable, 0, 8)
		for _, input := range e.collectInputs(context, make([]Input, 0, 8)) {
			for _, result := range e.results {
				tests = append(tests, input.CreateTests(result)...)
			}
		}
		for _, test := range tests {
			test(context, assertions)
		}
	}
	return &TestExecutable{testNode{e}, test}
}

func (e *Examples) CreateTest() Test {
	tests := make([]Test, len(e.children))
	for idx, child := range e.children {
		tests[idx] = child.CreateTest()
	}
	return &TestGroup{testNode{e}, tests}
}

func (n *node) Description() string {
	return n.description
}

func (n *node) Get(key string) (v LazyValue, ok bool) {
	v, ok = n.values[key]
	return
}

func (p *ParseResult) CreateTest(actual interface{}) Executable {
	path, source, epp := pathContentAndEpp(actual)
	expectedPN := ParsePN(p.location, p.expected)

	return func(context *TestContext, assertions Assertions) {
		o := context.ParserOptions()
		if epp {
			o = append(o, parser.EppMode)
		}
		actual, issues := parseAndValidate(path, context.resolveLazyValue(source).String(), false, o...)
		failOnError(assertions, issues)

		// Automatically strip off blocks that contain one statement
		if pr, ok := actual.(*parser.Program); ok {
			actual = pr.Body()
		}
		if be, ok := actual.(*parser.BlockExpression); ok {
			s := be.Statements()
			if len(s) == 1 {
				actual = s[0]
			}
		}
		actualPN := actual.ToPN()
		assertions.AssertEquals(expectedPN.String(), actualPN.String())
	}
}

func (p *ParseResult) setExample(example *Example) {
	p.example = example
}

func (s *SettingsInput) CreateTests(expected Result) []Executable {
	// Settings input does not create any tests
	return []Executable{func(tc *TestContext, assertions Assertions) {
		settings, ok := tc.resolveLazyValue(s.settings).(*types.Hash)
		if !ok {
			panic(px.Error(ValueNotHash, issue.H{`type`: `Settings`}))
		}
		settings.EachPair(func(key, value px.Value) {
			pcore.Set(key.String(), value)
		})
	}}
}

func (s *ScopeInput) CreateTests(expected Result) []Executable {
	return []Executable{func(tc *TestContext, assertions Assertions) {
		scope, ok := tc.resolveLazyValue(s.scope).(*types.Hash)
		if !ok {
			panic(px.Error(ValueNotHash, issue.H{`type`: `Scope`}))
		}
		tc.scope = evaluator.NewScope2(scope, false)
	}}
}

func (i *Source) CreateTests(expected Result) []Executable {
	result := make([]Executable, len(i.sources))
	for idx, source := range i.sources {
		result[idx] = expected.CreateTest(source)
	}
	return result
}

func (i *Source) AsInput() Input {
	return i
}

func (ns *NamedSource) CreateTests(expected Result) []Executable {
	return []Executable{expected.CreateTest(ns)}
}

func (ps *ParserOptions) CreateTests(expected Result) []Executable {
	return []Executable{func(tc *TestContext, assertions Assertions) {
		if tc.parserOptions == nil {
			tc.parserOptions = ps.options
		} else {
			tc.parserOptions = tc.parserOptions.Merge(ps.options)
		}
	}}
}

func init() {
	px.NewGoConstructor2(`PSpec::Example`,
		func(l px.LocalTypes) {
			l.Type2(`Given`, types.NewGoRuntimeType(&Given{}))
			l.Type2(`Let`, types.NewGoRuntimeType(&LazyValueLet{}))
			l.Type2(`SpecResult`, types.NewGoRuntimeType((*Result)(nil)))
		},
		func(d px.Dispatch) {
			d.Param(`String`)
			d.RepeatedParam(`Variant[Let,Given,SpecResult]`)
			d.Function(func(c px.Context, args []px.Value) px.Value {
				lets := make([]*LazyValueLet, 0)
				var given *Given
				results := make([]Result, 0)
				for _, arg := range args[1:] {
					if rt, ok := arg.(*types.RuntimeValue); ok {
						i := rt.Interface()
						switch i.(type) {
						case *LazyValueLet:
							lets = append(lets, i.(*LazyValueLet))
						case *Given:
							if given != nil {

							}
							given = i.(*Given)
						case Result:
							results = append(results, i.(Result))
						}
					}
				}
				example := newExample(args[0].String(), given, results)
				example.addLetDefs(lets)
				for _, result := range results {
					result.setExample(example)
				}
				return types.WrapRuntime(example)
			})
		})

	px.NewGoConstructor2(`PSpec::Examples`,
		func(l px.LocalTypes) {
			l.Type2(`Given`, types.NewGoRuntimeType(&Given{}))
			l.Type2(`Let`, types.NewGoRuntimeType(&LazyValueLet{}))
			l.Type2(`ExampleNode`, types.NewGoRuntimeType((*Node)(nil)))
			l.Type(`Nodes`, `Variant[ExampleNode, Array[Nodes]]`)
		},
		func(d px.Dispatch) {
			d.Param(`String`)
			d.RepeatedParam(`Variant[Nodes,Let,Given]`)
			d.Function(func(c px.Context, args []px.Value) px.Value {
				lets := make([]*LazyValueLet, 0)
				var given *Given
				others := make([]px.Value, 0)
				for _, arg := range args[1:] {
					if rt, ok := arg.(*types.RuntimeValue); ok {
						if l, ok := rt.Interface().(*LazyValueLet); ok {
							lets = append(lets, l)
							continue
						}
						if g, ok := rt.Interface().(*Given); ok {
							given = g
							continue
						}
					}
					others = append(others, arg)
				}
				ex := newExamples(args[0].String(), given, splatNodes(types.WrapValues(others)))
				ex.addLetDefs(lets)
				return types.WrapRuntime(ex)
			})
		})

	px.NewGoConstructor(`PSpec::Given`,
		func(d px.Dispatch) {
			d.RepeatedParam2(types.NewVariantType(types.DefaultStringType(), types.NewGoRuntimeType((*Input)(nil)), types.NewGoRuntimeType((*LazyValue)(nil))))
			d.Function(func(c px.Context, args []px.Value) px.Value {
				argc := len(args)
				inputs := make([]Input, argc)
				for idx := 0; idx < argc; idx++ {
					arg := args[idx]
					switch arg.(type) {
					case px.StringValue:
						inputs[idx] = &Source{[]*source{{arg, false}}}
					default:
						v := arg.(*types.RuntimeValue).Interface()
						switch v.(type) {
						case Input:
							inputs[idx] = v.(Input)
						default:
							inputs[idx] = &Source{[]*source{{arg, false}}}
						}
					}
				}
				return types.WrapRuntime(&Given{inputs})
			})
		})

	px.NewGoConstructor(`PSpec::Settings`,
		func(d px.Dispatch) {
			d.Param(`Any`)
			d.Function(func(c px.Context, args []px.Value) px.Value {
				return types.WrapRuntime(&SettingsInput{args[0]})
			})
		})

	px.NewGoConstructor(`PSpec::Scope`,
		func(d px.Dispatch) {
			d.Param(`Hash[Pattern[/\A[a-z_]\w*\z/],Any]`)
			d.Function(func(c px.Context, args []px.Value) px.Value {
				return types.WrapRuntime(&ScopeInput{args[0]})
			})
		})

	px.NewGoConstructor(`PSpec::Source`,
		func(d px.Dispatch) {
			d.RepeatedParam2(types.NewVariantType(types.DefaultStringType(), types.NewGoRuntimeType((*LazyValue)(nil))))
			d.Function(func(c px.Context, args []px.Value) px.Value {
				argc := len(args)
				sources := make([]*source, argc)
				for idx := 0; idx < argc; idx++ {
					sources[idx] = &source{args[idx], false}
				}
				return types.WrapRuntime(&Source{sources})
			})
		})

	px.NewGoConstructor(`PSpec::Epp_source`,
		func(d px.Dispatch) {
			d.RepeatedParam2(types.NewVariantType(types.DefaultStringType(), types.NewGoRuntimeType((*LazyValue)(nil))))
			d.Function(func(c px.Context, args []px.Value) px.Value {
				argc := len(args)
				sources := make([]*source, argc)
				for idx := 0; idx < argc; idx++ {
					sources[idx] = &source{args[idx], true}
				}
				return types.WrapRuntime(&Source{sources})
			})
		})

	px.NewGoConstructor(`PSpec::Named_source`,
		func(d px.Dispatch) {
			d.Param(`String`)
			d.Param2(types.NewVariantType(types.DefaultStringType(), types.NewGoRuntimeType((*LazyValue)(nil))))
			d.Function(func(c px.Context, args []px.Value) px.Value {
				return types.WrapRuntime(&NamedSource{source{args[1], false}, args[0].String()})
			})
		})

	px.NewGoConstructor(`PSpec::Unindent`,
		func(d px.Dispatch) {
			d.Param(`String`)
			d.Function(func(c px.Context, args []px.Value) px.Value {
				return types.WrapString(issue.Unindent(args[0].String()))
			})
		})

	px.NewGoConstructor(`PSpec::Parser_options`,
		func(d px.Dispatch) {
			d.Param(`Hash[Pattern[/[a-z_]*/],Data]`)
			d.Function(func(c px.Context, args []px.Value) px.Value {
				return types.WrapRuntime(&ParserOptions{args[0].(*types.Hash)})
			})
		})
}

func splatNodes(args px.List) []Node {
	nodes := make([]Node, 0)
	args.Each(func(arg px.Value) {
		if rv, ok := arg.(*types.RuntimeValue); ok {
			nodes = append(nodes, rv.Interface().(Node))
		} else {
			nodes = append(nodes, splatNodes(arg.(*types.Array))...)
		}
	})
	return nodes
}
