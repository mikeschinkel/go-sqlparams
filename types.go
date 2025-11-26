// Package sqlparams provides SQL query parsing and parameter extraction.
// It replaces :param placeholders with database-specific parameter syntax
// (e.g., $1, $2 for PostgreSQL; ? for MySQL).
//
// Placeholders use colon-prefixed syntax (:param) which:
//   - Works with IDE SQL syntax highlighting
//   - Matches standard SQL conventions
//   - Avoids conflicts with brace-based URL templates
package sqlparams

// Identifier is a word — typically lowercase — with first character being an
// alpha or underscore then alphanumeric or underscore, e.g. foo, bar123,
// foo_bar, or _baz.
type Identifier string

// Selector is a kind of super identifier consisting of identifiers and/or
// array indices, e.g. foo[0].bar indicates the 'bar' property of the first
// array element of the object type stored in foo. Used to index into JSON.
// Uses bracket notation for array indices: foo[0].bar
type Selector string

// SQLQuery is for the generic form of SQL query which can be for SQLite3,
// Postgres, MySQL, etc.
type SQLQuery string

// QueryString is for a generic query form which can represent a SQL Query or a
// query for a NoSQL database.
type QueryString string
