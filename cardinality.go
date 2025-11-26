package sqlparams

import (
	"strings"
)

type Cardinality string

func (c Cardinality) EmptyOk() bool {
	return c[len(c)-1] == '?'
}

const (
	OneRow         Cardinality = "one"
	ManyRows       Cardinality = "many"
	OneRowOrNone   Cardinality = "one?"
	ManyRowsOrNone Cardinality = "many?"
)

func ParseCardinality(s string) (re Cardinality, err error) {
	if s == "" {
		re = DefaultCardinality
		goto end
	}
	re = Cardinality(strings.ToLower(s))
	switch re {
	case ManyRows, OneRow, ManyRowsOrNone, OneRowOrNone:

		// Nothing to do
	default:
		err = NewErr(ErrInvalidCardinalityType, "cardinality", s)
		re = ""
	}
end:
	return re, err
}
