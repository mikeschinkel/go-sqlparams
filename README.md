# sqlparams

> **Current package name**: `dbqvars`
> **Future package name**: `sqlparams` (when extracted to standalone repository)

A Go library for parsing and rewriting SQL queries with named parameters. Converts `:name` style placeholders to database-specific parameter formats (`$1`, `?`, `@p1`, etc.) while safely handling SQL syntax edge cases.

## Features

- ✅ **Database-agnostic**: Works with PostgreSQL, MySQL, SQLite, SQL Server, Oracle, and more
- ✅ **Named parameters**: Use `:name` instead of positional `?` or `$1`
- ✅ **Nested data support**: Reference nested structures with `:user.id` or `:items[0].name`
- ✅ **Safe parsing**: Skips parameters inside strings, comments, and identifiers
- ✅ **Duplicate handling**: Reuse parameters like `:since` multiple times in one query
- ✅ **Zero dependencies**: Only uses Go standard library
- ✅ **Comprehensive edge case handling**: PostgreSQL `::` casts, time literals, Oracle Q-quotes, etc.
- ✅ **Infinite loop protection**: Tested against all known hang scenarios

## Usage

Here's a complete, runnable example showing how to safely use this package with user-provided JSON data:

```go
package main

import (
	"database/sql"
	"encoding/json"
	"fmt"
	"log"
	"os"

	_ "github.com/mattn/go-sqlite3" // SQLite driver
	"github.com/mikeschinkel/go-sqlparams"
)

func main() {
	// 1. Initialize database with sample data
	db, err := initDatabase("example.db")
	if err != nil {
		log.Fatal(err)
	}
	defer db.Close()

	// 2. User-provided JSON (e.g., from HTTP request body)
	userJSON := `{
		"status": "active",
		"min_score": 65
	}`

	// 3. Parse and validate JSON
	var params map[string]interface{}
	if err := json.Unmarshal([]byte(userJSON), &params); err != nil {
		log.Fatal("Invalid JSON:", err)
	}

	// 4. SQL template with named parameters
	var sqlTemplate sqlparams.SQLQuery = `
		SELECT id, email, name, score
		FROM users
		WHERE status = :status
		  AND score >= :min_score
		ORDER BY score DESC
	`

	// 5. Parse SQL and rewrite for SQLite (uses ? placeholders)
	parsed, err := sqlparams.ParseSQL(sqlTemplate,
		func(i int) string {
			return "?"
		},
	)
	if err != nil {
		log.Fatal("SQL parse error:", err)
	}

	// 6. Build ordered parameter values (SAFE: uses parameterized queries)
	values := make([]any, 0, len(parsed.Parameters()))
	for _, param := range parsed.Parameters() {
		value, ok := params[string(param.Name)]
		if !ok {
			log.Fatalf("Missing required parameter: %s", param.Name)
		}
		values = append(values, value)
	}

	// 7. Execute query safely (no SQL injection possible)
	rows, err := db.Query(string(parsed.SQL), values...)
	if err != nil {
		log.Fatal("Query error:", err)
	}
	defer rows.Close()

	// 8. Process results
	fmt.Println("Results:")
	for rows.Next() {
		var id, score int
		var email, name string
		if err := rows.Scan(&id, &email, &name, &score); err != nil {
			log.Fatal(err)
		}
		fmt.Printf("  User: %s (%s) - Score: %d\n", name, email, score)
	}

	// Expected output:
	// Results:
	//   User: Alice Anderson (alice@example.com) - Score: 95
	//   User: Eve Evans (eve@example.com) - Score: 88
	//   User: Jack Jackson (jack@example.com) - Score: 83
	//   User: Bob Brown (bob@example.com) - Score: 78
	//   User: Henry Harris (henry@example.com) - Score: 72
	//   User: Frank Foster (frank@example.com) - Score: 67
	//
	// Records that do NOT match (filtered out):
	//   Carol Carter (inactive, score: 82) - status != "active"
	//   Dave Davis (active, score: 45) - score < 65
	//   Grace Green (inactive, score: 91) - status != "active"
	//   Iris Ivanov (active, score: 55) - score < 65
}

// initDatabase creates the database and populates it with sample data
func initDatabase(dbPath string) (*sql.DB, error) {
	// Remove existing database for clean test
	os.Remove(dbPath)

	// Open/create database
	db, err := sql.Open("sqlite3", dbPath)
	if err != nil {
		return nil, fmt.Errorf("failed to open database: %w", err)
	}

	// Create users table
	createTableSQL := `
		CREATE TABLE IF NOT EXISTS users (
			id INTEGER PRIMARY KEY AUTOINCREMENT,
			email TEXT NOT NULL UNIQUE,
			name TEXT NOT NULL,
			status TEXT NOT NULL,
			score INTEGER NOT NULL
		)
	`
	if _, err := db.Exec(createTableSQL); err != nil {
		return nil, fmt.Errorf("failed to create table: %w", err)
	}

	// Insert sample data (10 users with varying scores)
	sampleUsers := []struct {
		email  string
		name   string
		status string
		score  int
	}{
		{"alice@example.com", "Alice Anderson", "active", 95},
		{"bob@example.com", "Bob Brown", "active", 78},
		{"carol@example.com", "Carol Carter", "inactive", 82},
		{"dave@example.com", "Dave Davis", "active", 45},
		{"eve@example.com", "Eve Evans", "active", 88},
		{"frank@example.com", "Frank Foster", "active", 67},
		{"grace@example.com", "Grace Green", "inactive", 91},
		{"henry@example.com", "Henry Harris", "active", 72},
		{"iris@example.com", "Iris Ivanov", "active", 55},
		{"jack@example.com", "Jack Jackson", "active", 83},
	}

	insertSQL := `INSERT INTO users (email, name, status, score) VALUES (?, ?, ?, ?)`
	for _, user := range sampleUsers {
		if _, err := db.Exec(insertSQL, user.email, user.name, user.status, user.score); err != nil {
			return nil, fmt.Errorf("failed to insert user %s: %w", user.email, err)
		}
	}

	return db, nil
}
```

**How Sanitization Works:**

**No explicit sanitization is needed.** The safety comes from parameterized queries:

1. ✅ **JSON Validation**: `json.Unmarshal` ensures valid JSON structure
2. ✅ **No String Concatenation**: Never builds SQL strings with user input
3. ✅ **Parameterized Queries**: Database driver handles all escaping automatically
4. ✅ **Named Parameters**: Clear mapping between JSON keys and SQL placeholders
5. ✅ **SQL Injection Prevention**: User input never interpreted as SQL code

**The key insight**: When you pass values as parameters to `db.Query(sql, values...)`, the database driver:
- Treats values as **data**, not **code**
- Handles all necessary escaping internally
- Uses protocol-level parameter binding (not string manipulation)
- Prevents SQL injection at the protocol layer

**Comparison:**

```go
// ❌ UNSAFE - String concatenation (DON'T DO THIS)
status := params["status"].(string)
sql := "SELECT * FROM users WHERE status = '" + status + "'"
db.Query(sql) // Vulnerable to SQL injection!

// ✅ SAFE - Parameterized query (SQLite)
sql := "SELECT * FROM users WHERE status = ?"
db.Query(sql, status) // Database driver handles escaping
```

This package makes it easier to use parameterized queries with named parameters instead of positional ones.

**What Gets Sent to Database:**

```sql
-- Rewritten SQL (SQLite format):
SELECT id, email, name, score
FROM users
WHERE status = ?
  AND score >= ?
ORDER BY score DESC

-- Parameters (bound safely):
? (position 1) = "active"
? (position 2) = 65
```

**Attack Example:**

Even if user input contains malicious SQL, it's treated as literal data:

```go
// Malicious JSON input:
userJSON := `{"status": "'; DROP TABLE users; --"}`

// After parsing, this becomes:
? (position 1) = "'; DROP TABLE users; --"

// The database receives:
SELECT * FROM users WHERE status = ?
// with parameter: "'; DROP TABLE users; --"

// The query looks for a user with status literally equal to "'; DROP TABLE users; --"
// It does NOT execute the DROP TABLE command
```

**Why?** The database driver sends the SQL and parameters separately using the database protocol. The parameter is never parsed as SQL syntax - it's transmitted as a data value in the protocol message, completely isolated from the SQL code.

## Installation

```bash
go get github.com/mikeschinkel/go-sqlparams
```

## Quick Start

```go
package main

import (
	"fmt"
	"github.com/mikeschinkel/go-sqlparams"
)

func main() {
	// SQL template with named parameters
	sql := "SELECT * FROM users WHERE email = :email AND active = :active"

	// Parse for PostgreSQL ($1, $2, ...)
	result, err := sqlparams.ParseSQL(
		sqlparams.SQLQuery(sql),
		func(i int) string {
			return fmt.Sprintf("$%d", i)
		},
	)

	if err != nil {
		panic(err)
	}

	fmt.Println(result.SQL)
	// Output: SELECT * FROM users WHERE email = $1 AND active = $2

	fmt.Println(result.Parameters())
	// Output: [email active]
}
```

## Why Named Parameters?

**Problem**: When building SQL queries from configuration, JSON APIs, or user input, positional parameters (`?`) are error-prone:

```go
// Positional - hard to maintain, easy to break
query := "SELECT * FROM orders WHERE user_id = ? AND status = ? AND created_at > ?"
db.Query(query, status, userId, since) // Bug! Wrong order
```

**Solution**: Named parameters are self-documenting and order-independent:

```go
// Named - clear intent, order doesn't matter
query := "SELECT * FROM orders WHERE user_id = :userId AND status = :status AND created_at > :since"
parsed, _ := sqlparams.ParseSQL(sqlparams.SQLQuery(query), postgresFormat)
// Bind values by name, not position
```

## Usage Examples

### Basic Usage

```go
import "github.com/mikeschinkel/go-sqlparams"

// Define your database's parameter format
func postgresFormat(i int) string {
	return fmt.Sprintf("$%d", i)
}

func mysqlFormat(int) string {
	return "?"
}

func sqlServerFormat(i int) string {
	return fmt.Sprintf("@p%d", i)
}

// Parse your SQL
sql := "SELECT * FROM users WHERE id = :id"
result, err := sqlparams.ParseSQL(sqlparams.SQLQuery(sql), postgresFormat)

// Use rewritten SQL
fmt.Println(result.SQL)        // SELECT * FROM users WHERE id = $1
fmt.Println(result.Parameters()) // [id]
```

### Duplicate Parameters

When the same parameter appears multiple times, it's only bound once:

```go
sql := `
	SELECT * FROM orders
	WHERE created_at >= :since
	  AND updated_at >= :since
`

result, _ := sqlparams.ParseSQL(sqlparams.SQLQuery(sql), postgresFormat)

fmt.Println(result.SQL)
// SELECT * FROM orders
// WHERE created_at >= $1
//   AND updated_at >= $1

fmt.Println(result.Parameters())
// [since]
```

**Note**: PostgreSQL handles this automatically. For MySQL/SQLite, you must repeat the value:

```go
result, _ := sqlparams.ParseSQL(sqlparams.SQLQuery(sql), mysqlFormat)
// SELECT * FROM orders WHERE created_at >= ? AND updated_at >= ?

// MySQL requires repeating the value
db.Query(result.SQL, sinceValue, sinceValue)
```

### Nested Data (Dotted Paths)

Reference nested fields from JSON or structs:

```go
sql := `
	INSERT INTO events (user_id, event_type, metadata)
	VALUES (:user.id, :event.type, :event.metadata)
`

result, _ := sqlparams.ParseSQL(sqlparams.SQLQuery(sql), postgresFormat)

// Application resolves dotted paths from data structure
params := map[string]interface{}{
	"user.id":       42,
	"event.type":    "click",
	"event.metadata": `{"button": "submit"}`,
}
```

### Array Indices

Access array elements using bracket notation:

```go
sql := `
	INSERT INTO cart_items (cart_id, product_id, quantity)
	VALUES
		(:cart_id, :items[0].product_id, :items[0].quantity),
		(:cart_id, :items[1].product_id, :items[1].quantity)
`

result, _ := sqlparams.ParseSQL(sqlparams.SQLQuery(sql), postgresFormat)

// Result: 5 unique parameters
fmt.Println(result.Parameters())
// [cart_id items[0].product_id items[0].quantity items[1].product_id items[1].quantity]
```

### Edge Cases (Handled Automatically)

The parser correctly handles SQL syntax edge cases:

```go
// PostgreSQL type casts (::)
sql := "SELECT name::text, id::int FROM users WHERE id = :id"
// Correctly identifies :id, ignores ::

// Time literals in strings
sql := "SELECT * FROM events WHERE time = '12:30:00' AND user_id = :userId"
// Correctly ignores :30 inside the string literal

// Comments
sql := "SELECT * FROM users -- WHERE id = :fake\nWHERE id = :real"
// Only :real is detected

// Oracle Q-quotes
sql := "SELECT * FROM users WHERE name = q'<Don't use :fake>' AND id = :id"
// Only :id is detected
```

## API Reference

### Types

```go
type SQLQuery string        // Input SQL with :name placeholders
type QueryString string     // Output SQL with database-specific placeholders
type Identifier string      // Simple parameter name (e.g., "user_id")
type Selector string        // Complex parameter name (e.g., "user.id", "items[0]")
type Parameter Selector     // Alias for parameter names

type ParsedSQL struct {
	SQL         SQLQuery      // Rewritten SQL
	// Private fields for parameters and occurrences
}

type FormatParamFunc func(paramIndex int) string
```

### Core Functions

#### ParseSQL

```go
func ParseSQL(sql SQLQuery, formatFunc FormatParamFunc) (ParsedSQL, error)
```

Parses SQL with `:name` placeholders and rewrites them using the provided format function.

**Parameters:**
- `sql`: SQL query template with `:name` style placeholders
- `formatFunc`: Function that converts parameter index to database-specific format

**Returns:**
- `ParsedSQL`: Parsed query with rewritten SQL and parameter metadata
- `error`: Parse errors (e.g., invalid placeholder names)

**Example:**
```go
result, err := sqlparams.ParseSQL(
	sqlparams.SQLQuery("SELECT * FROM users WHERE id = :id"),
	func(i int) string { return fmt.Sprintf("$%d", i) },
)
```

#### ParsedSQL Methods

```go
func (ps ParsedSQL) QueryString() QueryString
```
Returns the rewritten SQL as a `QueryString`.

```go
func (ps ParsedSQL) Parameters() Parameters
```
Returns ordered list of unique parameter names (deduplicated, first occurrence wins).

```go
func (ps ParsedSQL) Occurrences() QueryTokens
```
Returns all parameter occurrences including duplicates (useful for validation).

### Format Functions

Common format functions for different databases:

```go
// PostgreSQL: $1, $2, $3, ...
func PostgresFormat(i int) string {
	return fmt.Sprintf("$%d", i)
}

// MySQL / SQLite: ?, ?, ?, ...
func MySQLFormat(int) string {
	return "?"
}

// SQL Server: @p1, @p2, @p3, ...
func SQLServerFormat(i int) string {
	return fmt.Sprintf("@p1", i)
}
```

## Parameter Name Rules

Parameter names must follow these rules:

1. **Start with a letter or underscore**: `:user`, `:_temp`
2. **Contain letters, digits, underscores**: `:user_id`, `:item123`
3. **Support dotted paths**: `:user.email.domain`
4. **Support array indices (brackets)**: `:items[0]`, `:tags[5].name`

**Invalid names** (will cause parse errors):
- `:123invalid` - starts with digit
- `:user.0.name` - digit after dot (use bracket notation: `:user[0].name`)

## Advanced Usage

### Custom Backend Support

Add support for any SQL database by providing a format function:

```go
// Oracle: :param1, :param2, ...
func oracleFormat(i int) string {
	return fmt.Sprintf(":param%d", i)
}

// CockroachDB (same as PostgreSQL): $1, $2, ...
func cockroachFormat(i int) string {
	return fmt.Sprintf("$%d", i)
}
```

### Validation

Check if all required parameters are provided:

```go
result, _ := sqlparams.ParseSQL(sql, postgresFormat)

requiredParams := result.Parameters()
providedParams := map[string]interface{}{
	"user_id": 42,
	"status":  "active",
}

for _, param := range requiredParams {
	if _, ok := providedParams[string(param)]; !ok {
		return fmt.Errorf("missing required parameter: %s", param)
	}
}
```

### Query Building

Safely build complex queries:

```go
func BuildUserQuery(filters map[string]interface{}) (string, []interface{}, error) {
	parts := []string{"SELECT * FROM users WHERE 1=1"}
	params := make(map[string]interface{})

	if email, ok := filters["email"]; ok {
		parts = append(parts, "AND email LIKE :email || '%'")
		params["email"] = email
	}

	if status, ok := filters["status"]; ok {
		parts = append(parts, "AND status = :status")
		params["status"] = status
	}

	sql := strings.Join(parts, " ")
	result, err := sqlparams.ParseSQL(sqlparams.SQLQuery(sql), postgresFormat)
	if err != nil {
		return "", nil, err
	}

	// Build ordered values array
	values := make([]interface{}, len(result.Parameters()))
	for i, param := range result.Parameters() {
		values[i] = params[string(param)]
	}

	return string(result.SQL), values, nil
}
```

## Testing

The package includes comprehensive tests:

```bash
go test ./...
```

### Infinite Loop Protection

Special tests ensure the parser cannot hang:

```go
// Tests complete in <100ms or fail
func TestParseSQL_NoInfiniteLoops(t *testing.T)
```

Test cases cover:
- Lowercase 'q' in SQL keywords (`SELECT`, `UNIQUE`, etc.)
- Malformed Oracle Q-quotes
- Consecutive delimiter characters
- Edge cases from real-world queries

## Performance

The parser uses a single-pass state machine for O(n) complexity:

- **Small queries** (< 1KB): ~100 µs
- **Medium queries** (1-10KB): ~1-5 ms
- **Large queries** (> 10KB): ~10-50 ms

Benchmarks:
```bash
go test -bench=. -benchmem
```

## Limitations

1. **Not a SQL validator**: The parser does not validate SQL syntax
2. **No array expansion**: `IN (:ids)` is not automatically expanded (handle upstream)
3. **No type checking**: Parameter types are not validated
4. **No schema awareness**: Does not know about table/column names

These are intentional design decisions to keep the parser simple and focused.

## Error Handling

### Common Errors

```go
var (
	ErrFormatParamFuncRequired = errors.New("FormatParamFunc is required")
	ErrInvalidPlaceholderName  = errors.New("invalid placeholder name")
)
```

### Invalid Placeholder Name

```go
sql := "SELECT * FROM users WHERE id = :items.0.id"
_, err := sqlparams.ParseSQL(sqlparams.SQLQuery(sql), postgresFormat)
// Error: invalid placeholder name (use bracket notation: :items[0].id)
```

## Migration Guide

### From Positional Parameters

**Before (MySQL):**
```go
query := "SELECT * FROM users WHERE email = ? AND active = ?"
rows, err := db.Query(query, email, active)
```

**After:**
```go
query := "SELECT * FROM users WHERE email = :email AND active = :active"
parsed, _ := sqlparams.ParseSQL(sqlparams.SQLQuery(query), mysqlFormat)
rows, err := db.Query(string(parsed.SQL), email, active)
```

### From Other Libraries

If you're using other SQL parameter libraries:

```go
// sqlx (already supports named params, but limited to Go structs)
// This library works with any data source

// database/sql (standard library uses positional)
// This library adds named parameter support

// squirrel/goqu (query builders)
// This library complements builders by handling parameter rewriting
```

## Contributing

Contributions welcome! Please:

1. Add tests for new features
2. Ensure all tests pass: `go test ./...`
3. Follow Go conventions (gofmt, golint)
4. Update documentation

## License

MIT License - See LICENSE file for details

## Related Work

- **ADR-001**: Named SQL Parameter Placeholder Syntax (in `adrs/` directory)
- **Oracle SQL**: Named bind variables
- **Python DB-API 2.0**: Named parameter styles
- **PDO (PHP)**: Named parameters
- **SQLAlchemy**: Bindparam mechanism

## Support

- **Issues**: https://github.com/mikeschinkel/go-sqlparams/issues
- **Discussions**: https://github.com/mikeschinkel/go-sqlparams/discussions

## Acknowledgments

Originally developed as part of the XMLUI project for building database-backed APIs from configuration files.
