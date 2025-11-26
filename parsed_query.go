package sqlparams

type ParsedQuery interface {
	QueryString() QueryString
	Parameters() Parameters
	Occurrences() QueryTokens
}
