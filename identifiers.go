package sqlparams

import (
	"strings"
)

type Parameter struct {
	Name  Selector
	Index int
}

func (p Parameter) IsIdentifier() bool {
	return !strings.Contains(string(p.Name), ".")
} //Absolute or Relative

type Parameters []Parameter

func NewParameters(names ...Selector) (ps Parameters) {
	ps = make(Parameters, len(names))
	for i, name := range names {
		ps[i] = NewParameter(name, i+1)
	}
	return ps
}
func NewParameter(name Selector, index int) Parameter {
	return Parameter{
		Name:  name,
		Index: index,
	}
}

// Identifiers extracts slice of Identifier from a Parameters value (a
// slice of []Parameter)
func (ps Parameters) Identifiers() (ids []Identifier) {
	ids = make([]Identifier, 0, len(ps))
	for _, p := range ps {
		if !p.IsIdentifier() {
			continue
		}
		ids = append(ids, Identifier(p.Name))
	}
	return ids
}

func (ps Parameters) DottedSelectors() (selectors []Selector) {
	selectors = make([]Selector, 0, len(ps))
	for _, p := range ps {
		if p.IsIdentifier() {
			continue
		}
		selectors = append(selectors, Selector(p.Name))
	}
	return selectors
}
