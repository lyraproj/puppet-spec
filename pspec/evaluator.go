package pspec

import (
	"github.com/lyraproj/issue/issue"
	"github.com/lyraproj/pcore/px"
	"github.com/lyraproj/pcore/types"
	"github.com/lyraproj/puppet-evaluator/evaluator"
	"github.com/lyraproj/puppet-evaluator/pdsl"
	"github.com/lyraproj/puppet-parser/parser"
)

type (
	specEval struct {
		pdsl.Evaluator
		path []parser.Expression
	}
)

var pspecQRefs = map[string]string{
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

const testNodes = `testNodes`

func NewSpecEvaluator(c pdsl.EvaluationContext) pdsl.Evaluator {
	return &specEval{Evaluator: evaluator.NewEvaluator(c), path: make([]parser.Expression, 0)}
}

func (s *specEval) specError(issueCode issue.Code, semantic parser.Expression, args issue.H) issue.Reported {
	return issue.NewReported(issueCode, issue.SEVERITY_ERROR, args, semantic)
}

func CreateTests(c pdsl.EvaluationContext, expression parser.Expression) []Test {
	c.Set(testNodes, make([]Node, 0))
	c.AddDefinitions(expression)
	pdsl.TopEvaluate(c, expression)
	ns, _ := c.Get(testNodes)
	nodes := ns.([]Node)
	tests := make([]Test, len(nodes))
	for i, node := range nodes {
		tests[i] = node.CreateTest()
	}
	return tests
}

func (s *specEval) Eval(expression parser.Expression) px.Value {
	switch expression.(type) {
	case *parser.BlockExpression:
		return s.evalBlockExpression(expression.(*parser.BlockExpression))
	case *parser.QualifiedReference:
		return s.evalQualifiedReference(expression.(*parser.QualifiedReference))
	case *parser.CallNamedFunctionExpression:
		return s.evalCallNamedFunctionExpression(expression.(*parser.CallNamedFunctionExpression))
	default:
		return evaluator.BasicEval(s, expression)
	}
}

func addNode(c px.Context, n Node) {
	nodes, _ := c.Get(testNodes)
	c.Set(testNodes, append(nodes.([]Node), n))
}

func (s *specEval) evalBlockExpression(expr *parser.BlockExpression) px.Value {
	stmts := expr.Statements()
	result := px.Value(px.Undef)
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

func (s *specEval) evalQualifiedReference(qr *parser.QualifiedReference) px.Value {
	if i, ok := issue.IssueForCode2(issue.Code(qr.Name())); ok {
		return types.WrapRuntime(i)
	}
	if p, ok := pspecQRefs[qr.Name()]; ok {
		qr = qr.WithName(p)
	}
	return evaluator.BasicEval(s, qr)
}

func (s *specEval) evalCallNamedFunctionExpression(call *parser.CallNamedFunctionExpression) px.Value {
	if qr, ok := call.Functor().(*parser.QualifiedReference); ok {
		if p, ok := pspecQRefs[qr.Name()]; ok {
			call = call.WithFunctor(qr.WithName(p))
		}
	}
	return evaluator.BasicEval(s, call)
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
