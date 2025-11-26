package sqlparams

import (
	"sort"
)

type QueryToken struct {
	Name  Selector // logical name: e.g., "path.accountId" or "body.items.0.id"
	Index int      // assigned parameter index (1-based)
	Start int      // byte offset start in original SQL
	End   int      // byte offset end (exclusive)
	Raw   string   // full token, e.g. "{user.id}"
}

type QueryTokens []QueryToken

func (qts QueryTokens) Parameters() (names []Parameter) {
	names = make([]Parameter, len(qts))
	// Ensure the parameters are ordered by Index
	sort.Slice(qts, func(i, j int) bool {
		return qts[i].Index < qts[j].Index
	})
	for i, sp := range qts {
		names[i] = NewParameter(sp.Name, sp.Index)
	}
	return names
}
