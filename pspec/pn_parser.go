package pspec

import (
	"strings"

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
		case `nil`:
			lexer.NextToken()
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
	lexer.NextToken()
	return pn.List(parseElements(lexer, parser.TOKEN_RB))
}

func parseMap(lexer parser.Lexer) pn.PN {
	entries := make([]pn.Entry, 0, 8)
	token := lexer.NextToken()
	for token != parser.TOKEN_RC && token != parser.TOKEN_END {
		lexer.AssertToken(parser.TOKEN_COLON)
		lexer.NextToken()
		key, ok := parseIdentifier(lexer)
		if !ok {
			lexer.SyntaxError()
		}
		entries = append(entries, parseNext(lexer).WithName(key))
		token = lexer.CurrentToken()
	}
	lexer.AssertToken(parser.TOKEN_RC)
	lexer.NextToken()
	return pn.Map(entries)
}

func parseCall(lexer parser.Lexer) pn.PN {
	lexer.NextToken()
	name, ok := parseIdentifier(lexer)
	if !ok {
		lexer.SyntaxError()
	}
	elements := parseElements(lexer, parser.TOKEN_RP)
	return pn.Call(name, elements...)
}

func parseLiteral(lexer parser.Lexer) pn.PN {
	pn := pn.Literal(lexer.TokenValue())
	lexer.NextToken()
	return pn
}

func parseIdentifier(lexer parser.Lexer) (string, bool) {
	switch lexer.CurrentToken() {
	case parser.TOKEN_END,
		parser.TOKEN_LP, parser.TOKEN_WSLP, parser.TOKEN_RP,
		parser.TOKEN_LB, parser.TOKEN_LISTSTART, parser.TOKEN_RB,
		parser.TOKEN_LC, parser.TOKEN_SELC, parser.TOKEN_RC,
		parser.TOKEN_EPP_END, parser.TOKEN_EPP_END_TRIM, parser.TOKEN_RENDER_EXPR, parser.TOKEN_RENDER_STRING,
		parser.TOKEN_COMMA, parser.TOKEN_COLON, parser.TOKEN_SEMICOLON,
		parser.TOKEN_STRING, parser.TOKEN_INTEGER, parser.TOKEN_FLOAT, parser.TOKEN_CONCATENATED_STRING, parser.TOKEN_HEREDOC,
		parser.TOKEN_REGEXP:
		return ``, false
	case parser.TOKEN_DEFAULT:
		lexer.NextToken()
		return `default`, true
	default:
		str := lexer.TokenString()
		pos := lexer.TokenStartPos()
		lexer.NextToken()
		if lexer.CurrentToken() == parser.TOKEN_SUBTRACT {
			revertTo := lexer.TokenStartPos()
			sr := lexer.(parser.StringReader)
			lexer.NextToken()
			str2Pos := lexer.TokenStartPos()
			if str2, ok := parseIdentifier(lexer); ok {
				// If the two identifiers are bound together with a '-' and no whitespace
				// then accept this as one identifer
				sr := lexer.(parser.StringReader)
				if strings.IndexAny(sr.Text()[pos:str2Pos], " \t\n") < 0 {
					return str + `-` + str2, true
				}
			}
			sr.SetPos(revertTo)
		}
		return str, true
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
