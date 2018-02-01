package pspec

import (
	"github.com/puppetlabs/go-parser/parser"
	"github.com/puppetlabs/go-parser/pn"
)

func ParsePN(file string, content string) pn.PN {
	lexer := parser.NewSimpleLexer(file, content)
	lexer.NextToken()
	return parseNext(lexer)
}

func parseNext(lexer parser.Lexer) pn.PN {
	switch lexer.CurrentToken() {
	case parser.TOKEN_LB, parser.TOKEN_LISTSTART:
		return parseArray(lexer)
	case parser.TOKEN_LC, parser.TOKEN_SELC:
		return parseMap(lexer)
	case parser.TOKEN_LP, parser.TOKEN_WSLP:
		return parseCall(lexer)
	case parser.TOKEN_STRING, parser.TOKEN_BOOLEAN, parser.TOKEN_INTEGER, parser.TOKEN_FLOAT, parser.TOKEN_UNDEF:
		return parseLiteral(lexer)
	case parser.TOKEN_IDENTIFIER:
		switch lexer.TokenValue() {
		case `null`:
			return pn.Literal(nil)
		default:
			lexer.SyntaxError()
		}
	case parser.TOKEN_SUBTRACT:
		switch lexer.NextToken() {
		case parser.TOKEN_FLOAT:
			return pn.Literal(-lexer.TokenValue().(float64))
		case parser.TOKEN_INTEGER:
			return pn.Literal(-lexer.TokenValue().(int64))
		default:
			lexer.SyntaxError()
		}
	default:
		lexer.SyntaxError()
	}
	return nil
}

func parseArray(lexer parser.Lexer) pn.PN {
	return pn.List(parseElements(lexer, parser.TOKEN_RB))
}

func parseMap(lexer parser.Lexer) pn.PN {
	entries := make([]pn.Entry, 0, 8)
	token := lexer.NextToken()
	for token != parser.TOKEN_RC && token != parser.TOKEN_END {
		lexer.AssertToken(parser.TOKEN_COLON)
		lexer.NextToken()
		key := parseIdentifier(lexer)
		entries = append(entries, parseNext(lexer).WithName(key))
		token = lexer.CurrentToken()
	}
	lexer.AssertToken(parser.TOKEN_RC)
	lexer.NextToken()
	return pn.Map(entries)
}

func parseCall(lexer parser.Lexer) pn.PN {
	lexer.NextToken()
	name := parseIdentifier(lexer)
	elements := parseElements(lexer, parser.TOKEN_RP)
	return pn.Call(name, elements...)
}

func parseLiteral(lexer parser.Lexer) pn.PN {
	pn := pn.Literal(lexer.TokenValue())
	lexer.NextToken()
	return pn
}

func parseIdentifier(lexer parser.Lexer) string {
	switch lexer.CurrentToken() {
	case parser.TOKEN_END,
		parser.TOKEN_LP, parser.TOKEN_WSLP, parser.TOKEN_RP,
		parser.TOKEN_LB, parser.TOKEN_LISTSTART, parser.TOKEN_RB,
		parser.TOKEN_LC, parser.TOKEN_SELC, parser.TOKEN_RC,
		parser.TOKEN_EPP_END, parser.TOKEN_EPP_END_TRIM, parser.TOKEN_RENDER_EXPR, parser.TOKEN_RENDER_STRING,
		parser.TOKEN_COMMA, parser.TOKEN_COLON, parser.TOKEN_SEMICOLON,
		parser.TOKEN_STRING, parser.TOKEN_INTEGER, parser.TOKEN_FLOAT, parser.TOKEN_CONCATENATED_STRING, parser.TOKEN_HEREDOC,
		parser.TOKEN_REGEXP:
		lexer.SyntaxError()
		return ``
	case parser.TOKEN_DEFAULT:
		lexer.NextToken()
		return `default`
	default:
		str := lexer.TokenString()
		lexer.NextToken()
		return str
	}
}

func parseElements(lexer parser.Lexer, endToken int) []pn.PN {
	elements := make([]pn.PN, 0, 8)
	token := lexer.CurrentToken()
	for token != endToken && token != parser.TOKEN_END {
		elements = append(elements, parseNext(lexer))
		token = lexer.CurrentToken()
	}
	lexer.AssertToken(endToken)
	lexer.NextToken()
	return elements
}
