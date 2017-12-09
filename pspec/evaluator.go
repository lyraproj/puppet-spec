package pspec

import (
	"reflect"

	. "github.com/puppetlabs/go-evaluator/eval"
	. "github.com/puppetlabs/go-evaluator/evaluator"
	. "github.com/puppetlabs/go-evaluator/pcore"
	. "github.com/puppetlabs/go-evaluator/types"
	. "github.com/puppetlabs/go-parser/issue"
	. "github.com/puppetlabs/go-parser/parser"
	. "github.com/puppetlabs/go-parser/validator"
	. "github.com/puppetlabs/go-pspec/testutils"
)

var pcore = NewPcore(NewStdLogger())

type (
	Assertions interface {
		AssertEquals(a interface{}, b interface{})

		Fail(message string)
	}

	Executable func(assertions Assertions)

	SpecEvaluator interface {
		Evaluator

		CreateTests(expression Expression, loader Loader) []Test
	}

	specEval struct {
		evaluator Evaluator
		nodes     []Node
		path      []Expression
	}

	SpecFunction func(s *specEval, semantic Expression, args []PValue) PValue

	Input interface {
		CreateTests(expected Result) []Executable
	}

	Node interface {
		Description() string
		CreateTest() Test
	}

	Result interface {
		CreateTest(actual interface{}) Executable

		setExample(example *Example)
	}

	ResultEntry interface {
		Match() PValue
	}

	Test interface {
		Name() string
	}

	Example struct {
		description string
		given       *Given
		result      Result
		scope       Scope
		evaluator   Evaluator
	}

	Examples struct {
		description string
		children    []Node
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

	ScopeInput struct {
		scope Scope
	}

	TestExecutable struct {
		name string
		test Executable
	}

	TestGroup struct {
		name  string
		tests []Test
	}
)

func NewSpecEvaluator(loader DefiningLoader) SpecEvaluator {
	specEval := &specEval{nodes: make([]Node, 0), path: make([]Expression, 0)}
	specEval.evaluator = NewOverriddenEvaluator(loader, NewStdLogger(), specEval)
	return specEval
}

func (s *specEval) AddDefinitions(expression Expression) {
	s.evaluator.AddDefinitions(expression)
}

func (s *specEval) Evaluate(expression Expression, scope Scope, loader Loader) (PValue, *ReportedIssue) {
	return s.evaluator.Evaluate(expression, scope, loader)
}

func (s *specEval) Logger() Logger {
	return s.evaluator.Logger()
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

func init() {
	NewGoConstructor(`Example`,
		func(d Dispatch) {
			d.Param(`String`)
			d.Param(`Runtime[go, '*pspec.Given']`)
			d.Param2(NewRuntimeType3(reflect.TypeOf([]Result{}).Elem()))
			d.Function(func(c EvalContext, args []PValue) PValue {
				result := args[2].(*RuntimeValue).Interface().(Result)
				given := args[1].(*RuntimeValue).Interface().(*Given)
				var scope Scope
				for _, input := range given.inputs {
					if si, ok := input.(*ScopeInput); ok {
						scope = si.Scope()
						break
					}
				}
				if scope == nil {
					scope = NewScope()
				}
				example := &Example{args[0].String(), given, result, scope, nil}
				result.setExample(example)
				return WrapRuntime(example)
			})
		})

	NewGoConstructor2(`Examples`,
		func(l LocalTypes) {
			l.Type2(`Node`, NewRuntimeType3(reflect.TypeOf([]Node{}).Elem()))
			l.Type(`Nodes`, `Variant[Node, Array[Nodes]]`)
		},
		func(d Dispatch) {
			d.Param(`String`)
			d.RepeatedParam(`Nodes`)
			d.Function(func(c EvalContext, args []PValue) PValue {
				return WrapRuntime(&Examples{args[0].String(), splatNodes(args[1:])})
			})
		})

	NewGoConstructor(`Evaluates_to`,
		func(d Dispatch) {
			d.Param(`Any`)
			d.Function(func(c EvalContext, args []PValue) PValue {
				return WrapRuntime(&EvaluationResult{nil, args[0]})
			})
		})

	NewGoConstructor(`Given`,
		func(d Dispatch) {
			d.RepeatedParam2(NewVariantType2(DefaultStringType(), NewRuntimeType3(reflect.TypeOf([]Input{}).Elem())))
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

	NewGoConstructor(`Parses_to`,
		func(d Dispatch) {
			d.Param(`String`)
			d.Function(func(c EvalContext, args []PValue) PValue {
				return WrapRuntime(&ParseResult{nil, args[0].String()})
			})
		})

	NewGoConstructor(`Scope`,
		func(d Dispatch) {
			d.Param(`Hash[Pattern[/\A[a-z_]\w*\z/],Any]`)
			d.Function(func(c EvalContext, args []PValue) PValue {
				return WrapRuntime(&ScopeInput{NewScope2(args[0].(*HashValue))})
			})
		})

	NewGoConstructor(`Source`,
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

	NewGoConstructor(`Unindent`,
		func(d Dispatch) {
			d.Param(`String`)
			d.Function(func(c EvalContext, args []PValue) PValue {
				return WrapString(Unindent(args[0].String()))
			})
		})
}

func (s *specEval) specError(issueCode IssueCode, semantic Expression, args ...interface{}) *ReportedIssue {
	return NewReportedIssue(issueCode, SEVERITY_ERROR, args, semantic)
}

func (s *specEval) CreateTests(expression Expression, loader Loader) []Test {
	s.AddDefinitions(expression)
	if _, err := s.Evaluate(expression, NewScope(), loader); err != nil {
		panic(err)
	}
	tests := make([]Test, len(s.nodes))
	for _, node := range s.nodes {
		tests = append(tests, node.CreateTest())
	}
	return tests
}

func (s *specEval) Eval(expression Expression, ctx EvalContext) PValue {
	switch expression.(type) {
	case *BlockExpression:
		return s.eval_BlockExpression(expression.(*BlockExpression), ctx)
	case *QualifiedReference:
		return s.eval_QualifiedReference(expression.(*QualifiedReference), ctx)
	default:
		return s.evaluator.Eval(expression, ctx)
	}
}

func (s *specEval) ResolveDefinitions() {
	s.evaluator.ResolveDefinitions()
}

func (s *specEval) addNode(n Node) {
	s.nodes = append(s.nodes, n)
}

func (s *specEval) eval_BlockExpression(expr *BlockExpression, ctx EvalContext) PValue {
	stmts := expr.Statements()
	result := PValue(UNDEF)
	oldPath := s.path

	p := make([]Expression, len(s.path), len(s.path)+1)
	copy(p, s.path)
	s.path = append(p, expr)

	defer func() {
		s.path = oldPath
	}()

	for _, stmt := range stmts {
		result = s.Eval(stmt, ctx)
		if len(oldPath) == 0 {
			if rt, ok := result.(*RuntimeValue); ok {
				var n Node
				if n, ok = rt.Interface().(Node); ok {
					s.addNode(n)
				}
			}
		}
	}
	return result
}

func (s *specEval) eval_QualifiedReference(qr *QualifiedReference, ctx EvalContext) PValue {
	if i, ok := IssueForCode2(IssueCode(qr.Name())); ok {
		return WrapRuntime(i)
	}
	return s.evaluator.Eval(qr, ctx)
}

func hasError(issues []*ReportedIssue) bool {
	for _, issue := range issues {
		if issue.Severity() == SEVERITY_ERROR {
			return true
		}
	}
	return false
}

func failOnError(assertions Assertions, issues []*ReportedIssue) {
	for _, issue := range issues {
		if issue.Severity() == SEVERITY_ERROR {
			assertions.Fail(issue.Error())
			return
		}
	}
}

func (e *EvaluationResult) CreateTest(actual interface{}) Executable {
	source := actual.(string)

	return func(assertions Assertions) {
		actual, issues := parseAndValidate(source, false)
		failOnError(assertions, issues)
		actualResult, evalIssues := evaluate(e.example.Evaluator(), actual, e.example.Scope())
		failOnError(assertions, evalIssues)
		assertions.AssertEquals(e.expected, actualResult)
	}
}

func (e *EvaluationResult) setExample(example *Example) {
	e.example = example
}

func (e *Example) CreateTest() Test {
	tests := make([]Executable, 0, 8)
	for _, input := range e.given.inputs {
		tests = append(tests, input.CreateTests(e.result)...)
	}
	test := func(assertions Assertions) {
		for _, test := range tests {
			test(assertions)
		}
	}
	return &TestExecutable{e.description, test}
}

func (e *Example) Description() string {
	return e.description
}

func (e *Example) Evaluator() Evaluator {
	if e.evaluator == nil {
		e.evaluator = NewEvaluator(NewParentedLoader(pcore.Loader()), NewArrayLogger())
	}
	return e.evaluator
}

func (e *Example) Scope() Scope {
	return e.scope
}

func (e *Examples) CreateTest() Test {
	tests := make([]Test, len(e.children))
	for idx, child := range e.children {
		tests[idx] = child.CreateTest()
	}
	return &TestGroup{e.description, tests}
}

func (e *Examples) Description() string {
	return e.description
}

func (p *ParseResult) CreateTest(actual interface{}) Executable {
	source := actual.(string)
	expectedPN := ParsePN(``, p.expected)

	return func(assertions Assertions) {
		actual, issues := parseAndValidate(source, true)
		failOnError(assertions, issues)
		actualPN := actual.ToPN()
		assertions.AssertEquals(expectedPN.String(), actualPN.String())
	}
}

func (p *ParseResult) setExample(example *Example) {
	p.example = example
}

func (i *ScopeInput) CreateTests(expected Result) []Executable {
	// Scope input does not create any tests
	return []Executable{}
}

func (i *ScopeInput) Scope() Scope {
	return i.scope
}

func (i *Source) CreateTests(expected Result) []Executable {
	result := make([]Executable, len(i.sources))
	for idx, source := range i.sources {
		result[idx] = expected.CreateTest(source)
	}
	return result
}

func (v *TestExecutable) Name() string {
	return v.name
}

func (v *TestExecutable) Executable() Executable {
	return v.test
}

func (v *TestGroup) Name() string {
	return v.name
}

func (v *TestGroup) Tests() []Test {
	return v.tests
}

func parseAndValidate(source string, singleExpression bool) (Expression, []*ReportedIssue) {
	expr, err := CreateParser().Parse(``, source, false, singleExpression)
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
