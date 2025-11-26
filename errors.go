// Package sqlparams/errors defines error values used throughout the sqlparams package.
// These sentinel errors provide specific error types for different failure modes
// during SQL parsing and placeholder processing.
package sqlparams

import (
	"errors"
)

// Sentinel errors for various sqlparams operations.
var (
	// ErrFormatParamFuncRequired indicates that ParseSQLArgs.FormatParamFunc is nil.
	ErrFormatParamFuncRequired = errors.New("ParseSQLArgs.GetFormatParamFunc is required")

	// ErrInvalidPlaceholderName indicates that a placeholder name is invalid or malformed.
	// Valid placeholders are :name where name follows identifier rules (letters, digits, underscores, dots).
	ErrInvalidPlaceholderName = errors.New("invalid placeholder name")

	ErrInvalidCardinalityType = errors.New("invalid cardinality type")

	ErrInvalidResultsColumnDataType = errors.New("invalid results column data type")

	ErrInvalidRowType = errors.New("invalid row type")

	ErrInvalidDataType = errors.New("invalid data type")
)
