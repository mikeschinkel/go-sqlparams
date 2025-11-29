package sqlparams

import (
	"strings"
	"unicode"
)

type FormatParamFunc = func(int) string

type parseState struct {
	src     string
	n       int
	i       int
	edits   []editState
	order   []string
	indexOf map[string]int
	tokens  QueryTokens
}

func newParseState(sqlText SQLQuery) parseState {
	return parseState{
		src:     string(sqlText),
		n:       len(sqlText),
		i:       0,
		edits:   make([]editState, 0),
		order:   make([]string, 0),
		indexOf: make(map[string]int),
		tokens:  make([]QueryToken, 0),
	}
}

type editState struct {
	start, end int
	repl       string
}

func (s *parseState) getIndex(name string) (idx int) {
	var ok bool
	idx, ok = s.indexOf[name]
	if ok {
		goto end
	}
	s.order = append(s.order, name)
	idx = len(s.order)
	s.indexOf[name] = idx
end:
	return idx
}

func (s *parseState) peek(k int) (b byte) {
	if s.i+k < s.n {
		b = s.src[s.i+k]
	}
	return b
}

func (s *parseState) consumeSingleQuoted() {
	s.i++
	for s.i < s.n {
		c := s.src[s.i]
		s.i++
		if c == '\'' {
			if s.i <= s.n {
				goto end
			}
			if s.src[s.i] != '\'' {
				goto end
			}
			s.i++
		}
	}
end:
	return
}

func (s *parseState) consumeDoubleQuoted() {
	s.i++
	for s.i < s.n {
		c := s.src[s.i]
		s.i++
		if c == '"' {
			goto end
		}
	}
end:
	return
}

func (s *parseState) consumeBacktick() {
	s.i++
	for s.i < s.n {
		c := s.src[s.i]
		s.i++
		if c == '`' {
			goto end
		}
	}
end:
	return
}

func (s *parseState) consumeBracketIdent() {
	s.i++
	for s.i < s.n {
		c := s.src[s.i]
		s.i++
		if c == ']' {
			goto end
		}
	}
end:
	return
}

func (s *parseState) consumeDashDash() {
	for s.i < s.n && s.src[s.i] != '\n' {
		s.i++
	}
}

func (s *parseState) consumeHashComment() {
	s.i++
	for s.i < s.n && s.src[s.i] != '\n' {
		s.i++
	}
}

func (s *parseState) consumeBlockComment() {
	s.i += 2
	for s.i < s.n-1 {
		if s.src[s.i] != '*' {
			s.i++
			continue
		}
		if s.src[s.i+1] != '/' {
			s.i++
			continue
		}
		s.i += 2
		goto end
	}
	for s.i < s.n-1 {
		if s.src[s.i] == '*' && s.src[s.i+1] == '/' {
			s.i += 2
			goto end
		}
		s.i++
	}
	s.i = s.n
end:
	return
}

func (s *parseState) consumeDollarQuoted() {
	var tag string
	var idx int
	start := s.i
	s.i++
	for s.i < s.n {
		c := s.src[s.i]
		if c == '$' {
			s.i++
			break
		}
		if c != '_' && !unicode.IsLetter(rune(c)) && !unicode.IsDigit(rune(c)) {
			s.i = start + 1
			goto end
		}
		s.i++
	}
	if s.i > s.n {
		s.i = s.n
		goto end
	}
	tag = s.src[start:s.i]
	idx = strings.Index(s.src[s.i:], tag)
	if idx < 0 {
		s.i = s.n
		goto end
	}
	s.i += idx + len(tag)
end:
	return
}

func (s *parseState) consumeOracleQ() {
	var delim, closeDelim byte
	start := s.i
	if s.i+1 >= s.n {
		s.i = start + 1 // Advance past 'q'/'Q' if not Oracle Q-quote
		goto end
	}
	if s.src[s.i+1] != '\'' && s.src[s.i+1] != ' ' {
		s.i = start + 1 // Advance past 'q'/'Q' if not Oracle Q-quote
		goto end
	}
	s.i++
	for s.i < s.n && unicode.IsSpace(rune(s.src[s.i])) {
		s.i++
	}
	if s.i >= s.n || s.src[s.i] != '\'' {
		s.i = start + 1
		goto end
	}
	s.i++
	if s.i >= s.n {
		s.i = s.n
		goto end
	}

	delim = s.src[s.i]
	switch delim {
	case '<':
		closeDelim = '>'
	case '(':
		closeDelim = ')'
	case '[':
		closeDelim = ']'
	case '{':
		closeDelim = '}'
	default:
		closeDelim = delim
	}
	s.i++

	for s.i < s.n {
		if s.src[s.i] == closeDelim {
			if s.i+1 >= s.n {
				break // EOF without closing quote
			}
			if s.src[s.i+1] != '\'' {
				s.i++ // Not the end, keep scanning
				continue
			}
			s.i += 2
			goto end
		}
		s.i++
	}
	s.i = s.n
end:
	return
}

func (s *parseState) consumePlaceholder(formatFunc FormatParamFunc) (err error) {
	var idx int
	var rawName string

	start := s.i // Points to ':'
	s.i++        // Move past ':'

	// Scan identifier: [a-zA-Z_][a-zA-Z0-9_.]*
	j := s.i
	if j >= s.n || !isValidIdentifierStart(s.src[j]) {
		// Not a placeholder, just a standalone colon
		goto end
	}

	// Scan the rest of the identifier
	for j < s.n && isValidIdentifierChar(s.src[j]) {
		j++
	}

	rawName = s.src[s.i:j]
	if !isValidName(rawName) {
		err = NewErr(
			ErrInvalidPlaceholderName,
			"name", rawName,
			"offset", start,
		)
		goto end
	}

	idx = s.getIndex(rawName)
	s.tokens = append(s.tokens, QueryToken{
		Name:  Selector(rawName),
		Index: idx,
		Start: start,
		End:   j,
		Raw:   s.src[start:j],
	})
	s.edits = append(s.edits, editState{
		start: start,
		end:   j,
		repl:  formatFunc(idx),
	})
	s.i = j
end:
	return err
}

func (s *parseState) buildSQL() SQLQuery {
	var b strings.Builder
	var last int

	last = 0
	for _, e := range s.edits {
		if e.start > last {
			b.WriteString(s.src[last:e.start])
		}
		b.WriteString(e.repl)
		last = e.end
	}
	if last < len(s.src) {
		b.WriteString(s.src[last:])
	}
	return SQLQuery(b.String())
}

func (s *parseState) orderedTokens() QueryTokens {
	ordered := make(QueryTokens, len(s.order))
	for _, p := range s.tokens {
		if p.Index < 1 {
			continue
		}
		if p.Index > len(s.order) {
			continue
		}
		if s.order[p.Index-1] != string(p.Name) {
			continue
		}
		ordered[p.Index-1] = p
	}
	return ordered
}
