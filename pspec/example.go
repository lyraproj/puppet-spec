package pspec

import (
	"fmt"

	"github.com/puppetlabs/go-evaluator/eval"
	"github.com/puppetlabs/go-evaluator/impl"
	"github.com/puppetlabs/go-evaluator/types"
	"github.com/puppetlabs/go-parser/issue"
	"github.com/puppetlabs/go-parser/parser"
	"github.com/puppetlabs/go-pspec/testutils"
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
		Match() eval.PValue
	}

	node struct {
		description string
		values      map[string]LazyValue
		given       *Given
	}

	Example struct {
		node
		results   []Result
		evaluator eval.Evaluator
	}

	Examples struct {
		node
		children []Node
	}

	Given struct {
		inputs []Input
	}

	ParseResult struct {
		example  *Example
		expected string
	}

	EvaluationResult struct {
		example  *Example
		expected eval.PValue
	}

	Source struct {
		sources []string
	}

	NamedSource struct {
		name   string
		source string
	}

	SettingsInput struct {
		settings eval.PValue
	}

	ScopeInput struct {
		scope eval.PValue
	}
)

func pathAndContent(source interface{}) (path, content string) {
	switch source.(type) {
	case string:
		return ``, source.(string)
	case *NamedSource:
		ns := source.(*NamedSource)
		return ns.name, ns.source
	default:
		panic(eval.Error(eval.EVAL_FAILURE, issue.H{`message`: fmt.Sprintf(`Unknown source type %T`, source)}))
	}
}

func (e *EvaluationResult) CreateTest(actual interface{}) Executable {
	path, source := pathAndContent(actual)

	return func(context *TestContext, assertions Assertions) {
		actual, issues := parseAndValidate(path, source, false)
		failOnError(assertions, issues)
		actualResult, evalIssues := evaluate(e.example.Evaluator(), actual, context.Scope())
		failOnError(assertions, evalIssues)
		assertions.AssertEquals(context.resolveLazyValue(e.expected), actualResult)
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

func (e *node) collectInputs(context *TestContext, inputs []Input) []Input {
	pc := context.parent
	if pc != nil {
		inputs = pc.node.collectInputs(pc, inputs)
	}
	g := e.given
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

func (e *Example) Evaluator() eval.Evaluator {
	if e.evaluator == nil {
		e.evaluator = impl.NewEvaluator(eval.NewParentedLoader(eval.Puppet.EnvironmentLoader()), eval.NewArrayLogger())
	}
	return e.evaluator
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
	path, source := pathAndContent(actual)
	expectedPN := ParsePN(``, p.expected)

	return func(context *TestContext, assertions Assertions) {
		actual, issues := parseAndValidate(path, source, false)
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
		settings, ok := tc.resolveLazyValue(s.settings).(*types.HashValue)
		if !ok {
			eval.Error(PSPEC_VALUE_NOT_HASH, issue.H{`type`: `Settings`})
		}
		p := eval.Puppet
		settings.EachPair(func(key, value eval.PValue) {
			p.Set(key.String(), value)
		})
	}}
}

func (s *ScopeInput) CreateTests(expected Result) []Executable {
	return []Executable{func(tc *TestContext, assertions Assertions) {
		scope, ok := tc.resolveLazyValue(s.scope).(*types.HashValue)
		if !ok {
			eval.Error(PSPEC_VALUE_NOT_HASH, issue.H{`type`: `Scope`})
		}
		tc.scope = impl.NewScope2(scope)
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

func init() {
	eval.NewGoConstructor2(`PSpec::Example`,
		func(l eval.LocalTypes) {
			l.Type2(`Given`, types.NewGoRuntimeType([]*Given{}))
			l.Type2(`Let`, types.NewGoRuntimeType([]*LazyValueLet{}))
			l.Type2(`Result`, types.NewGoRuntimeType([]Result{}))
		},
		func(d eval.Dispatch) {
			d.Param(`String`)
			d.RepeatedParam(`Variant[Let,Given,Result]`)
			d.Function(func(c eval.EvalContext, args []eval.PValue) eval.PValue {
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

	eval.NewGoConstructor2(`PSpec::Examples`,
		func(l eval.LocalTypes) {
			l.Type2(`Given`, types.NewGoRuntimeType([]*Given{}))
			l.Type2(`Let`, types.NewGoRuntimeType([]*LazyValueLet{}))
			l.Type2(`Node`, types.NewGoRuntimeType([]Node{}))
			l.Type(`Nodes`, `Variant[Node, Array[Nodes]]`)
		},
		func(d eval.Dispatch) {
			d.Param(`String`)
			d.RepeatedParam(`Variant[Nodes,Let,Given]`)
			d.Function(func(c eval.EvalContext, args []eval.PValue) eval.PValue {
				lets := make([]*LazyValueLet, 0)
				var given *Given
				others := make([]eval.PValue, 0)
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
				ex := newExamples(args[0].String(), given, splatNodes(types.WrapArray(others)))
				ex.addLetDefs(lets)
				return types.WrapRuntime(ex)
			})
		})

	eval.NewGoConstructor(`PSpec::Given`,
		func(d eval.Dispatch) {
			d.RepeatedParam2(types.NewVariantType2(types.DefaultStringType(), types.NewGoRuntimeType([]Input{})))
			d.Function(func(c eval.EvalContext, args []eval.PValue) eval.PValue {
				argc := len(args)
				inputs := make([]Input, argc)
				for idx := 0; idx < argc; idx++ {
					arg := args[idx]
					if str, ok := arg.(*types.StringValue); ok {
						inputs[idx] = &Source{[]string{str.String()}}
					} else {
						inputs[idx] = arg.(*types.RuntimeValue).Interface().(Input)
					}
				}
				return types.WrapRuntime(&Given{inputs})
			})
		})

	eval.NewGoConstructor(`PSpec::Settings`,
		func(d eval.Dispatch) {
			d.Param(`Any`)
			d.Function(func(c eval.EvalContext, args []eval.PValue) eval.PValue {
				return types.WrapRuntime(&SettingsInput{args[0]})
			})
		})

	eval.NewGoConstructor(`PSpec::Scope`,
		func(d eval.Dispatch) {
			d.Param(`Hash[Pattern[/\A[a-z_]\w*\z/],Any]`)
			d.Function(func(c eval.EvalContext, args []eval.PValue) eval.PValue {
				return types.WrapRuntime(&ScopeInput{args[0]})
			})
		})

	eval.NewGoConstructor(`PSpec::Source`,
		func(d eval.Dispatch) {
			d.RepeatedParam(`String`)
			d.Function(func(c eval.EvalContext, args []eval.PValue) eval.PValue {
				argc := len(args)
				sources := make([]string, argc)
				for idx := 0; idx < argc; idx++ {
					sources[idx] = args[idx].String()
				}
				return types.WrapRuntime(&Source{sources})
			})
		})

	eval.NewGoConstructor(`PSpec::NamedSource`,
		func(d eval.Dispatch) {
			d.Param(`String`)
			d.Param(`String`)
			d.Function(func(c eval.EvalContext, args []eval.PValue) eval.PValue {
				return types.WrapRuntime(&NamedSource{args[0].String(), args[1].String()})
			})
		})

	eval.NewGoConstructor(`PSpec::Unindent`,
		func(d eval.Dispatch) {
			d.Param(`String`)
			d.Function(func(c eval.EvalContext, args []eval.PValue) eval.PValue {
				return types.WrapString(testutils.Unindent(args[0].String()))
			})
		})
}

func splatNodes(args eval.IndexedValue) []Node {
	nodes := make([]Node, 0)
	args.Each(func(arg eval.PValue) {
		if rv, ok := arg.(*types.RuntimeValue); ok {
			nodes = append(nodes, rv.Interface().(Node))
		} else {
			nodes = append(nodes, splatNodes(arg.(*types.ArrayValue))...)
		}
	})
	return nodes
}
