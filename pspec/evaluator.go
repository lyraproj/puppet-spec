package pspec

import (
	. "github.com/puppetlabs/go-evaluator/eval"
	. "github.com/puppetlabs/go-evaluator/evaluator"
	. "github.com/puppetlabs/go-evaluator/types"
	. "github.com/puppetlabs/go-parser/issue"
	. "github.com/puppetlabs/go-parser/parser"
)

type (
	SpecEvaluator interface {
		Evaluator

		CreateTests(expression Expression, loader Loader) []Test
	}

	specEval struct {
		evaluator Evaluator
		nodes     []Node
		path      []Expression
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

func (s *specEval) specError(issueCode IssueCode, semantic Expression, args H) *ReportedIssue {
	return NewReportedIssue(issueCode, SEVERITY_ERROR, args, semantic)
}

func (s *specEval) CreateTests(expression Expression, loader Loader) []Test {
	s.AddDefinitions(expression)
	if _, err := s.Evaluate(expression, NewScope(), loader); err != nil {
		panic(err)
	}
	tests := make([]Test, len(s.nodes))
	for i, node := range s.nodes {
		tests[i] = node.CreateTest()
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

func (s *specEval) ResolveDefinitions(c EvalContext) {
	s.evaluator.ResolveDefinitions(c)
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
