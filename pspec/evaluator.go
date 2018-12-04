package pspec

import (
	"github.com/lyraproj/puppet-evaluator/eval"
	"github.com/lyraproj/puppet-evaluator/impl"
	"github.com/lyraproj/puppet-evaluator/types"
	"github.com/lyraproj/issue/issue"
	"github.com/lyraproj/puppet-parser/parser"
)

type (
	specEval struct {
		eval.Evaluator
		path []parser.Expression
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

const TEST_NODES = `testNodes`

func NewSpecEvaluator(c eval.Context) eval.Evaluator {
	return &specEval{Evaluator: impl.NewEvaluator(c), path: make([]parser.Expression, 0)}
}

func (s *specEval) specError(issueCode issue.Code, semantic parser.Expression, args issue.H) issue.Reported {
	return issue.NewReported(issueCode, issue.SEVERITY_ERROR, args, semantic)
}

func CreateTests(c eval.Context, expression parser.Expression) []Test {
	c.Set(TEST_NODES, make([]Node, 0))
	c.AddDefinitions(expression)
	_, err := eval.TopEvaluate(c, expression)
	if err != nil {
		panic(err)
	}
	ns, _ := c.Get(TEST_NODES)
	nodes := ns.([]Node)
	tests := make([]Test, len(nodes))
	for i, node := range nodes {
		tests[i] = node.CreateTest()
	}
	return tests
}

func (s *specEval) Eval(expression parser.Expression) eval.Value {
	switch expression.(type) {
	case *parser.BlockExpression:
		return s.eval_BlockExpression(expression.(*parser.BlockExpression))
	case *parser.QualifiedReference:
		return s.eval_QualifiedReference(expression.(*parser.QualifiedReference))
	case *parser.CallNamedFunctionExpression:
		return s.eval_CallNamedFunctionExpression(expression.(*parser.CallNamedFunctionExpression))
	default:
		return impl.BasicEval(s, expression)
	}
}

func addNode(c eval.Context, n Node) {
	nodes, _ := c.Get(TEST_NODES)
	c.Set(TEST_NODES, append(nodes.([]Node), n))
}

func (s *specEval) eval_BlockExpression(expr *parser.BlockExpression) eval.Value {
	stmts := expr.Statements()
	result := eval.Value(eval.UNDEF)
	oldPath := s.path

	p := make([]parser.Expression, len(s.path), len(s.path)+1)
	copy(p, s.path)
	s.path = append(p, expr)

	defer func() {
		s.path = oldPath
	}()

	for _, stmt := range stmts {
		result = s.Eval(stmt)
		if len(oldPath) == 0 {
			if rt, ok := result.(*types.RuntimeValue); ok {
				var n Node
				if n, ok = rt.Interface().(Node); ok {
					addNode(s, n)
				}
			}
		}
	}
	return result
}

func (s *specEval) eval_QualifiedReference(qr *parser.QualifiedReference) eval.Value {
	if i, ok := issue.IssueForCode2(issue.Code(qr.Name())); ok {
		return types.WrapRuntime(i)
	}
	if p, ok := PSPEC_QREFS[qr.Name()]; ok {
		qr = qr.WithName(p)
	}
	return impl.BasicEval(s, qr)
}

func (s *specEval) eval_CallNamedFunctionExpression(call *parser.CallNamedFunctionExpression) eval.Value {
	if qr, ok := call.Functor().(*parser.QualifiedReference); ok {
		if p, ok := PSPEC_QREFS[qr.Name()]; ok {
			call = call.WithFunctor(qr.WithName(p))
		}
	}
	return impl.BasicEval(s, call)
}

func hasError(issues []issue.Reported) bool {
	for _, i := range issues {
		if i.Severity() == issue.SEVERITY_ERROR {
			return true
		}
	}
	return false
}

func failOnError(assertions Assertions, issues []issue.Reported) {
	for _, i := range issues {
		if i.Severity() == issue.SEVERITY_ERROR {
			assertions.Fail(i.Error())
			return
		}
	}
}
