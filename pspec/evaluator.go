package pspec

import (
	"github.com/puppetlabs/go-evaluator/eval"
	"github.com/puppetlabs/go-evaluator/impl"
	"github.com/puppetlabs/go-evaluator/types"
	"github.com/puppetlabs/go-parser/issue"
	"github.com/puppetlabs/go-parser/parser"
)

type (
	SpecEvaluator interface {
		eval.Evaluator

		CreateTests(c eval.Context, expression parser.Expression) []Test
	}

	specEval struct {
		evaluator eval.Evaluator
		nodes     []Node
		path      []parser.Expression
	}
)

var PSPEC_QREFS = map[string]string{
	`Contain`:        `PSpec::Contain`,
	`Directory`:      `PSpec::Directory`,
	`Epp_source`:     `PSpec::Epp_source`,
	`Error`:          `PSpec::Error`,
	`Evaluates_ok`:   `PSpec::Evaluates_ok`,
	`Evaluates_to`:   `PSpec::Evaluates_to`,
	`Evaluates_with`: `PSpec::Evaluates_with`,
	`Example`:        `PSpec::Example`,
	`Examples`:       `PSpec::Examples`,
	`Exclude`:        `PSpec::Exclude`,
	`File`:           `PSpec::File`,
	`Format`:         `PSpec::Format`,
	`Get`:            `PSpec::Get`,
	`Given`:          `PSpec::Given`,
	`Include`:        `PSpec::Include`,
	`Issue`:          `PSpec::Issue`,
	`Let`:            `PSpec::Let`,
	`Named_source`:   `PSpec::Named_source`,
	`Notice`:         `PSpec::Notice`,
	`Scope`:          `PSpec::Scope`,
	`Settings`:       `PSpec::Settings`,
	`Source`:         `PSpec::Source`,
	`Match`:          `PSpec::Match`,
	`Parser_options`: `PSpec::Parser_options`,
	`Parses_to`:      `PSpec::Parses_to`,
	`Validates_ok`:   `PSpec::Validates_ok`,
	`Validates_with`: `PSpec::Validates_with`,
	`Warning`:        `PSpec::Warning`,
	`Unindent`:       `PSpec::Unindent`,
}

func NewSpecEvaluator() SpecEvaluator {
	specEval := &specEval{nodes: make([]Node, 0), path: make([]parser.Expression, 0)}
	specEval.evaluator = impl.NewOverriddenEvaluator(eval.NewStdLogger(), specEval)
	return specEval
}

func (s *specEval) Evaluate(c eval.Context, expression parser.Expression) (eval.PValue, *issue.Reported) {
	return s.evaluator.Evaluate(c, expression)
}

func (s *specEval) Logger() eval.Logger {
	return s.evaluator.Logger()
}

func (s *specEval) specError(issueCode issue.Code, semantic parser.Expression, args issue.H) *issue.Reported {
	return issue.NewReported(issueCode, issue.SEVERITY_ERROR, args, semantic)
}

func (s *specEval) CreateTests(c eval.Context, expression parser.Expression) []Test {
	c.AddDefinitions(expression)
	if _, err := s.Evaluate(c, expression); err != nil {
		panic(err)
	}
	tests := make([]Test, len(s.nodes))
	for i, node := range s.nodes {
		tests[i] = node.CreateTest()
	}
	return tests
}

func (s *specEval) Eval(expression parser.Expression, ctx eval.Context) eval.PValue {
	switch expression.(type) {
	case *parser.BlockExpression:
		return s.eval_BlockExpression(expression.(*parser.BlockExpression), ctx)
	case *parser.QualifiedReference:
		return s.eval_QualifiedReference(expression.(*parser.QualifiedReference), ctx)
	case *parser.CallNamedFunctionExpression:
		return s.eval_CallNamedFunctionExpression(expression.(*parser.CallNamedFunctionExpression), ctx)
	default:
		return s.evaluator.Eval(expression, ctx)
	}
}

func (s *specEval) addNode(n Node) {
	s.nodes = append(s.nodes, n)
}

func (s *specEval) eval_BlockExpression(expr *parser.BlockExpression, ctx eval.Context) eval.PValue {
	stmts := expr.Statements()
	result := eval.PValue(eval.UNDEF)
	oldPath := s.path

	p := make([]parser.Expression, len(s.path), len(s.path)+1)
	copy(p, s.path)
	s.path = append(p, expr)

	defer func() {
		s.path = oldPath
	}()

	for _, stmt := range stmts {
		result = s.Eval(stmt, ctx)
		if len(oldPath) == 0 {
			if rt, ok := result.(*types.RuntimeValue); ok {
				var n Node
				if n, ok = rt.Interface().(Node); ok {
					s.addNode(n)
				}
			}
		}
	}
	return result
}

func (s *specEval) eval_QualifiedReference(qr *parser.QualifiedReference, ctx eval.Context) eval.PValue {
	if i, ok := issue.IssueForCode2(issue.Code(qr.Name())); ok {
		return types.WrapRuntime(i)
	}
	if p, ok := PSPEC_QREFS[qr.Name()]; ok {
		qr = qr.WithName(p)
	}
	return s.evaluator.Eval(qr, ctx)
}

func (s *specEval) eval_CallNamedFunctionExpression(call *parser.CallNamedFunctionExpression, c eval.Context) eval.PValue {
	if qr, ok := call.Functor().(*parser.QualifiedReference); ok {
		if p, ok := PSPEC_QREFS[qr.Name()]; ok {
			call = call.WithFunctor(qr.WithName(p))
		}
	}
	return s.evaluator.Eval(call, c)
}

func hasError(issues []*issue.Reported) bool {
	for _, i := range issues {
		if i.Severity() == issue.SEVERITY_ERROR {
			return true
		}
	}
	return false
}

func failOnError(assertions Assertions, issues []*issue.Reported) {
	for _, i := range issues {
		if i.Severity() == issue.SEVERITY_ERROR {
			assertions.Fail(i.Error())
			return
		}
	}
}
