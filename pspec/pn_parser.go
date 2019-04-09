package pspec

import (
	"bytes"
	"fmt"
	"strconv"
	"unicode/utf8"

	"github.com/lyraproj/issue/issue"
	"github.com/lyraproj/puppet-parser/pn"
)

type (
	token rune

	pnParser struct {
		location   issue.Location
		text       string
		pos        int
		token      token
		tokenValue interface{}
	}
)

const (
	tokenEnd        = token(0)
	tokenBool       = token('?')
	tokenNil        = token('_')
	tokenInt        = token('0')
	tokenFloat      = token('.')
	tokenString     = token('"')
	tokenLp         = token('(')
	tokenRp         = token(')')
	tokenLb         = token('[')
	tokenRb         = token(']')
	tokenLc         = token('{')
	tokenRc         = token('}')
	tokenIdentifier = token('a')
	tokenKey        = token(':')
)

func ParsePN(location issue.Location, content string) pn.PN {
	p := &pnParser{location: location, text: content}
	p.nextToken()
	return p.parseNext()
}

func (p *pnParser) parseNext() pn.PN {
	switch p.token {
	case tokenLb:
		return p.parseArray()
	case tokenLc:
		return p.parseMap()
	case tokenLp:
		return p.parseCall()
	case tokenBool, tokenInt, tokenFloat, tokenString, tokenNil:
		return p.parseLiteral()
	case tokenEnd:
		panic(p.error(`unexpected end of input`))
	default:
		panic(p.error(fmt.Sprintf(`unexpected '%v'`, p.tokenValue)))
	}
}

func (p *pnParser) parseLiteral() pn.PN {
	pv := pn.Literal(p.tokenValue)
	p.nextToken()
	return pv
}

func (p *pnParser) parseArray() pn.PN {
	p.nextToken()
	return pn.List(p.parseElements(tokenRb))
}

func (p *pnParser) parseMap() pn.PN {
	entries := make([]pn.Entry, 0, 8)
	p.nextToken()
	for p.token != tokenRc && p.token != tokenEnd {
		if p.token != tokenKey {
			panic(p.error(`map key expected`))
		}
		key := p.tokenValue.(string)
		p.nextToken()
		entries = append(entries, p.parseNext().WithName(key))
	}
	if p.token != tokenRc {
		panic(p.error(`missing '}' to end map`))
	}
	p.nextToken()
	return pn.Map(entries)
}

func (p *pnParser) parseCall() pn.PN {
	p.nextToken()
	if p.token != tokenIdentifier {
		panic(p.error(`expected identifier to follow '('`))
	}
	name := p.tokenValue.(string)
	p.nextToken()
	return pn.Call(name, p.parseElements(tokenRp)...)
}

func (p *pnParser) parseElements(endToken token) []pn.PN {
	elements := make([]pn.PN, 0, 8)
	for p.token != endToken && p.token != tokenEnd {
		elements = append(elements, p.parseNext())
	}
	if p.token != endToken {
		panic(p.error(fmt.Sprintf(`missing '%c' to end list`, endToken)))
	}
	p.nextToken()
	return elements
}

func (p *pnParser) nextToken() {
	p.skipWhite()
	s := p.pos
	c := p.next()

	if '0' <= c && c <= '9' {
		p.skipDecimalDigits()
		c, _ = p.peek()
		if c == '.' {
			p.pos++
			p.consumeFloat(s, '.')
		} else {
			v, _ := strconv.ParseInt(p.from(s), 10, 64)
			p.setTokenValue(tokenInt, v)
		}
		return
	}

	switch c {
	case 0:
		p.setTokenValue(tokenEnd, nil)
	case '-':
		// Unary minus if preceding a digit, otherwise identifier
		c, _ = p.peek()
		if '0' <= c && c <= '9' {
			p.nextToken()
			if p.token == tokenFloat {
				p.tokenValue = -p.tokenValue.(float64)
			} else {
				p.tokenValue = -p.tokenValue.(int64)
			}
		} else {
			p.consumeIdentifier(s)
		}
	case '(', ')', '[', ']', '{', '}':
		p.setTokenValue(token(c), c)
	case ':':
		c, _ = p.peek()
		if 'a' <= c && c <= 'z' || 'A' <= c && c <= 'Z' {
			p.nextToken()
			p.setTokenValue(tokenKey, p.tokenValue)
		} else {
			panic(p.error(`expected identifier after ':'`))
		}
	case '"':
		p.consumeString()
	default:
		p.consumeIdentifier(s)
	}
}

func (p *pnParser) consumeIdentifier(s int) {
	for {
		c, n := p.peek()
		switch c {
		case 0, ' ', '\t', '\r', '\n', '(', ')', '[', ']', '{', '}', ':', '"':
			v := p.from(s)
			switch v {
			case `false`:
				p.setTokenValue(tokenBool, false)
			case `true`:
				p.setTokenValue(tokenBool, true)
			case `nil`:
				p.setTokenValue(tokenNil, nil)
			default:
				p.setTokenValue(tokenIdentifier, v)
			}
			return
		default:
			p.pos += n
		}
	}
}

func (p *pnParser) consumeString() {
	s := p.pos
	b := bytes.NewBufferString(``)
	for {
		c := p.next()
		switch c {
		case 0:
			p.pos = s - 1
			panic(p.error(`unterminated quote`))
		case '"':
			p.setTokenValue(tokenString, b.String())
			return
		case '\\':
			c := p.next()
			switch c {
			case 't':
				b.WriteByte('\t')
			case 'r':
				b.WriteByte('\r')
			case 'n':
				b.WriteByte('\n')
			case 'o':
				// Expect exactly 3 octal digits
				c = 0
				for i := 0; i < 3; i++ {
					n := p.next()
					if '0' <= n && n <= '7' {
						c *= 8
						c += n - '0'
					} else {
						panic(p.error(`malformed octal quote`))
					}
				}
				b.WriteRune(c)
			case '"':
				b.WriteByte('"')
			case '\\':
				b.WriteByte('\\')
			default:
				b.WriteByte('\\')
				b.WriteRune(c)
			}
			continue
		default:
			b.WriteRune(c)
		}
	}
}

func (p *pnParser) consumeFloat(s int, d rune) {
	if p.skipDecimalDigits() == 0 {
		panic(p.error(`digit expected`))
	}
	c, n := p.peek()
	if d == '.' {
		// Check for 'e'
		if c == 'e' || c == 'E' {
			p.pos += n
			if p.skipDecimalDigits() == 0 {
				panic(p.error(`digit expected`))
			}
			c, n = p.peek()
		}
	}

	switch c {
	case 0, ' ', '\t', '\r', '\n', '(', ')', '[', ']', '{', '}', ':', '"':
	default:
		panic(p.error(`digit expected`))
	}

	v, _ := strconv.ParseFloat(p.from(s), 64)
	p.setTokenValue(tokenFloat, v)
}

func (p *pnParser) error(message string) error {
	loc := p.location
	line := loc.Line()
	pos := loc.Pos() + 11 // Provided location is the Parses_to("") call.
	max := len(p.text) - 1
	if max > p.pos {
		max = p.pos
	}
	for i := 0; i < max; i++ {
		if p.text[i] == '\n' {
			line++
			pos = 0 // Assuming that it's not heredoc in which case the margin is lost
		} else {
			pos++
		}
	}
	return issue.NewReported(PnParseError, issue.SeverityError, issue.H{`detail`: message}, issue.NewLocation(loc.File(), line, pos))
}

func (p *pnParser) from(s int) string {
	return p.text[s:p.pos]
}

func (p *pnParser) setTokenValue(t token, v interface{}) {
	p.token = t
	p.tokenValue = v
}

func (p *pnParser) next() rune {
	c, n := p.peek()
	if c != 0 {
		p.pos += n
	}
	return c
}

func (p *pnParser) peek() (c rune, sz int) {
	start := p.pos
	if start >= len(p.text) {
		return 0, 0
	}
	c = rune(p.text[start])
	if c < utf8.RuneSelf {
		sz = 1
	} else {
		c, sz = utf8.DecodeRuneInString(p.text[start:])
		if c == utf8.RuneError {
			panic(p.error(`invalid unicode character`))
		}
	}
	return
}

func (p *pnParser) skipDecimalDigits() int {
	digitCount := 0
	c, n := p.peek()
	if c == '-' || c == '+' {
		p.pos += n
		c, n = p.peek()
	}
	for '0' <= c && c <= '9' {
		p.pos += n
		c, n = p.peek()
		digitCount++
	}
	return digitCount
}

func (p *pnParser) skipWhite() {
	for {
		c, n := p.peek()
		switch c {
		case ' ', '\r', '\t', '\n':
			p.pos += n
		default:
			return
		}
	}
}
