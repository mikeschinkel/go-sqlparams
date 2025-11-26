package sqlparams

const (
	// DefaultCardinality specifies the default expected row count for database queries.
	DefaultCardinality = ManyRowsOrNone

	// DefaultRowType specifies the default format for returning database query results.
	DefaultRowType = ColumnsRowType

	// DefaultDBDataType specifies the default data type for database columns when none is specified.
	DefaultDBDataType = StringDBDataType
)
