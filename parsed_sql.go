package sqlparams

import (
	"unicode"
)

var _ ParsedQuery = (*ParsedSQL)(nil)

type ParsedSQL struct {
	SQL         SQLQuery
	parameters  []Parameter  // ordered by first appearance, deduped by Name
	occurrences []QueryToken // all parameter occurrences including duplicates
}

func NewParsedSQL(SQL SQLQuery, parameters []Parameter) ParsedSQL {
	if parameters == nil {
		parameters = make([]Parameter, 0)
	}
	return ParsedSQL{
		SQL:        SQL,
		parameters: parameters,
	}
}

func NewParsedSQLWithOccurrences(SQL SQLQuery, parameters []Parameter, occurrences []QueryToken) ParsedSQL {
	if parameters == nil {
		parameters = make([]Parameter, 0)
	}
	if occurrences == nil {
		occurrences = make([]QueryToken, 0)
	}
	return ParsedSQL{
		SQL:         SQL,
		parameters:  parameters,
		occurrences: occurrences,
	}
}

//func (ps ParsedSQL) GetValues(paramsMap map[Identifier]any, bodyJSON []byte) (values []any, err error) {
//	return QueryTokens(ps.Parameters).GetValues(paramsMap, bodyJSON)
//}

func (ps ParsedSQL) QueryString() QueryString {
	return QueryString(ps.SQL)
}

func (ps ParsedSQL) Parameters() (names Parameters) {
	return ps.parameters
}

func (ps ParsedSQL) Occurrences() (tokens QueryTokens) {
	return ps.occurrences
}

type ParseSQLArgs struct{}

// ParseSQL finds :name placeholders OUTSIDE of strings/identifiers/comments,
// rewrites them via FormatParamFunc, and returns the rewritten SQL & ordered tokens.
// Supports dotted paths like :user.id and array indices like :items[0].id.
// Does NOT match PostgreSQL :: casts or standalone : characters.
//
// FormatParamFunc examples:
//
//	Postgres: func(i int) string { return fmt.Sprintf("$%d", i) }
//	MySQL/SQLite: func(int) string { return "?" }
//	SQL Server: func(i int) string { return fmt.Sprintf("@p%d", i) }
func ParseSQL(sqlText SQLQuery, formatFunc FormatParamFunc) (ps ParsedSQL, err error) {
	var state parseState

	if formatFunc == nil {
		err = ErrFormatParamFuncRequired
		goto end
	}

	state = newParseState(sqlText)

	for state.i < state.n {
		c := state.src[state.i]

		switch c {
		case '-':
			if state.peek(1) == '-' {
				state.i += 2
				state.consumeDashDash()
				continue
			}
		case '#':
			state.consumeHashComment()
			continue
		case '/':
			if state.peek(1) == '*' {
				state.consumeBlockComment()
				continue
			}
		case '\'':
			state.consumeSingleQuoted()
			continue
		case '"':
			state.consumeDoubleQuoted()
			continue
		case '`':
			state.consumeBacktick()
			continue
		case '[':
			state.consumeBracketIdent()
			continue
		case '$':
			state.consumeDollarQuoted()
			continue
		case 'q', 'Q':
			state.consumeOracleQ()
			continue
		case ':':
			// Skip PostgreSQL :: cast operator
			if state.peek(1) == ':' {
				state.i += 2
				continue
			}
			// Only consume if next char is valid identifier start
			if state.i+1 < state.n && isValidIdentifierStart(state.src[state.i+1]) {
				err = state.consumePlaceholder(formatFunc)
				if err != nil {
					goto end
				}
				continue
			}
		}

		state.i++
	}

	if len(state.edits) == 0 {
		ps = NewParsedSQLWithOccurrences(
			SQLQuery(state.src),
			state.tokens.Parameters(),
			state.tokens,
		)
		goto end
	}

	ps = NewParsedSQLWithOccurrences(
		state.buildSQL(),
		state.orderedTokens().Parameters(),
		state.tokens,
	)

end:
	return ps, err
}

func isValidName(s string) (is bool) {
	var i int
	if s == "" {
		goto end
	}
	i = 0
	if !readIdent(s, &i) {
		goto end
	}
	for i < len(s) {
		switch s[i] {
		case '.':
			// Dot must be followed by identifier (not digit)
			// Use bracket notation for array indices: foo[0].bar
			i++
			if !readIdent(s, &i) {
				goto end
			}
		case '[':
			// Array index notation: [0], [123], etc.
			i++
			if !readDigits(s, &i) {
				goto end
			}
			if i >= len(s) || s[i] != ']' {
				goto end
			}
			i++
		default:
			goto end
		}
	}
	is = true
end:
	return is
}

func readIdent(s string, i *int) (ok bool) {
	var r rune
	var w int

	if *i >= len(s) {
		goto end
	}

	r, w = utf8At(s, *i)
	if !isLetterOrUnderscore(r) {
		goto end
	}

	*i += w
	for *i < len(s) {
		r, w := utf8At(s, *i)
		if isLetterDigitOrUnderscore(r) {
			*i += w
			continue
		}
		break
	}

	ok = true

end:
	return ok
}

func readDigits(s string, i *int) (ok bool) {
	start := *i

	if *i >= len(s) {
		goto end
	}

	for *i < len(s) {
		if s[*i] < '0' || s[*i] > '9' {
			break
		}
		*i++
	}

	if *i == start {
		goto end
	}

	ok = true

end:
	return ok
}

// utf8At returns the rune at byte offset i and its width.
func utf8At(s string, i int) (r rune, w int) {
	var rr []rune
	// Fast path: ASCII
	b := s[i]
	if b < 0x80 {
		r, w = rune(b), 1
		goto end
	}
	// Minimal safe decode for a single rune
	rr = []rune(s[i:])
	r, w = rr[0], len(string(rr[:1]))
end:
	return r, w
}

func isLetterOrUnderscore(r rune) (is bool) {
	if r == '_' {
		is = true
		goto end
	}
	if unicode.IsLetter(r) {
		is = true
		goto end
	}
end:
	return is
}

func isLetterDigitOrUnderscore(r rune) (is bool) {
	if r == '_' {
		is = true
		goto end
	}
	if unicode.IsLetter(r) {
		is = true
		goto end
	}
	if unicode.IsDigit(r) {
		is = true
		goto end
	}
end:
	return is
}

// isValidIdentifierStart checks if a byte is a valid start of an identifier (letter or underscore)
func isValidIdentifierStart(b byte) bool {
	return (b >= 'a' && b <= 'z') ||
		(b >= 'A' && b <= 'Z') ||
		b == '_'
}

// isValidIdentifierChar checks if a byte is a valid identifier character (letter, digit, underscore, dot, or brackets)
func isValidIdentifierChar(b byte) bool {
	return isValidIdentifierStart(b) ||
		(b >= '0' && b <= '9') ||
		b == '.' ||
		b == '[' || b == ']'
}
