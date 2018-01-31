package pspec

import (
	"fmt"

	. "github.com/puppetlabs/go-evaluator/eval"
	. "github.com/puppetlabs/go-evaluator/evaluator"
	. "github.com/puppetlabs/go-evaluator/types"
	. "github.com/puppetlabs/go-parser/issue"
	. "github.com/puppetlabs/go-pspec/testutils"
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
		Match() PValue
	}

	node struct {
		description string
		values      map[string]LazyValue
		given       *Given
	}

	Example struct {
		node
		results   []Result
		evaluator Evaluator
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
		expected PValue
	}

	Source struct {
		sources []string
	}

	NamedSource struct {
		name   string
		source string
	}

	SettingsInput struct {
		settings PValue
	}

	ScopeInput struct {
		scope PValue
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
		panic(Error(EVAL_FAILURE, H{`message`: fmt.Sprintf(`Unknown source type %T`, source)}))
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

func (e *Example) Evaluator() Evaluator {
	if e.evaluator == nil {
		e.evaluator = NewEvaluator(NewParentedLoader(Puppet.EnvironmentLoader()), NewArrayLogger())
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
		actual, issues := parseAndValidate(path, source, true)
		failOnError(assertions, issues)
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
		settings, ok := tc.resolveLazyValue(s.settings).(*HashValue)
		if !ok {
			Error(PSPEC_VALUE_NOT_HASH, H{`type`: `Settings`})
		}
		p := Puppet
		for _, e := range settings.EntriesSlice() {
			p.Set(e.Key().String(), e.Value())
		}
	}}
}

func (s *ScopeInput) CreateTests(expected Result) []Executable {
	return []Executable{func(tc *TestContext, assertions Assertions) {
		scope, ok := tc.resolveLazyValue(s.scope).(*HashValue)
		if !ok {
			Error(PSPEC_VALUE_NOT_HASH, H{`type`: `Scope`})
		}
		tc.scope = NewScope2(scope)
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
	NewGoConstructor2(`PSpec::Example`,
		func(l LocalTypes) {
			l.Type2(`Given`, NewGoRuntimeType([]*Given{}))
			l.Type2(`Let`, NewGoRuntimeType([]*LazyValueLet{}))
			l.Type2(`Result`, NewGoRuntimeType([]Result{}))
		},
		func(d Dispatch) {
			d.Param(`String`)
			d.RepeatedParam(`Variant[Let,Given,Result]`)
			d.Function(func(c EvalContext, args []PValue) PValue {
				lets := make([]*LazyValueLet, 0)
				var given *Given
				results := make([]Result, 0)
				for _, arg := range args[1:] {
					if rt, ok := arg.(*RuntimeValue); ok {
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
				return WrapRuntime(example)
			})
		})

	NewGoConstructor2(`PSpec::Examples`,
		func(l LocalTypes) {
			l.Type2(`Given`, NewGoRuntimeType([]*Given{}))
			l.Type2(`Let`, NewGoRuntimeType([]*LazyValueLet{}))
			l.Type2(`Node`, NewGoRuntimeType([]Node{}))
			l.Type(`Nodes`, `Variant[Node, Array[Nodes]]`)
		},
		func(d Dispatch) {
			d.Param(`String`)
			d.RepeatedParam(`Variant[Nodes,Let,Given]`)
			d.Function(func(c EvalContext, args []PValue) PValue {
				lets := make([]*LazyValueLet, 0)
				var given *Given
				others := make([]PValue, 0)
				for _, arg := range args[1:] {
					if rt, ok := arg.(*RuntimeValue); ok {
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
				ex := newExamples(args[0].String(), given, splatNodes(others))
				ex.addLetDefs(lets)
				return WrapRuntime(ex)
			})
		})

	NewGoConstructor(`PSpec::Given`,
		func(d Dispatch) {
			d.RepeatedParam2(NewVariantType2(DefaultStringType(), NewGoRuntimeType([]Input{})))
			d.Function(func(c EvalContext, args []PValue) PValue {
				argc := len(args)
				inputs := make([]Input, argc)
				for idx := 0; idx < argc; idx++ {
					arg := args[idx]
					if str, ok := arg.(*StringValue); ok {
						inputs[idx] = &Source{[]string{str.String()}}
					} else {
						inputs[idx] = arg.(*RuntimeValue).Interface().(Input)
					}
				}
				return WrapRuntime(&Given{inputs})
			})
		})

	NewGoConstructor(`PSpec::Settings`,
		func(d Dispatch) {
			d.Param(`Any`)
			d.Function(func(c EvalContext, args []PValue) PValue {
				return WrapRuntime(&SettingsInput{args[0]})
			})
		})

	NewGoConstructor(`PSpec::Scope`,
		func(d Dispatch) {
			d.Param(`Hash[Pattern[/\A[a-z_]\w*\z/],Any]`)
			d.Function(func(c EvalContext, args []PValue) PValue {
				return WrapRuntime(&ScopeInput{args[0]})
			})
		})

	NewGoConstructor(`PSpec::Source`,
		func(d Dispatch) {
			d.RepeatedParam(`String`)
			d.Function(func(c EvalContext, args []PValue) PValue {
				argc := len(args)
				sources := make([]string, argc)
				for idx := 0; idx < argc; idx++ {
					sources[idx] = args[idx].String()
				}
				return WrapRuntime(&Source{sources})
			})
		})

	NewGoConstructor(`PSpec::NamedSource`,
		func(d Dispatch) {
			d.Param(`String`)
			d.Param(`String`)
			d.Function(func(c EvalContext, args []PValue) PValue {
				return WrapRuntime(&NamedSource{args[0].String(), args[1].String()})
			})
		})

	NewGoConstructor(`PSpec::Unindent`,
		func(d Dispatch) {
			d.Param(`String`)
			d.Function(func(c EvalContext, args []PValue) PValue {
				return WrapString(Unindent(args[0].String()))
			})
		})
}

func splatNodes(args []PValue) []Node {
	nodes := make([]Node, 0)
	for _, arg := range args {
		if rv, ok := arg.(*RuntimeValue); ok {
			nodes = append(nodes, rv.Interface().(Node))
		} else {
			nodes = append(nodes, splatNodes(arg.(*ArrayValue).Elements())...)
		}
	}
	return nodes
}
