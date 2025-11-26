# ADR-001: Named SQL Parameter Placeholder Syntax

## Status

Accepted (2025-11-26)

## Context

When building systems that generate SQL queries dynamically from user input or configuration, a key requirement is a **safe, consistent way to represent query parameters in SQL templates**:

* Parameters are **named** (not positional) because source data structures (JSON, maps, structs) do not have a guaranteed positional order
* The system must bind parameters safely (avoiding SQL injection) and rewrite placeholders to the **native syntax of the target backend**
* The syntax must be unambiguous, easy to learn, and resistant to accidental collisions with SQL syntax
* The parser must work across multiple SQL dialects without requiring database-specific parsing logic

Several styles exist in the SQL ecosystem:

* `?` positional parameters (JDBC, MySQL, SQLite)
* `$1, $2, …` positional parameters (PostgreSQL)
* `:name` named parameters (PDO, Oracle, ActiveRecord)
* `@name` named parameters (SQL Server, ADO.NET)

## Decision

### Canonical Placeholder Syntax

Use **colon-prefixed named placeholders** (`:name`):

```sql
:name
:user_id
:config.setting
:items[0].sku
```

**Rationale:**
* Standard SQL tooling recognizes `:name` as parameter syntax
* IDE syntax highlighting works correctly
* Widely used convention (Oracle, PDO, SQLAlchemy, ActiveRecord)
* Natural word boundaries for parameter extraction

### Placeholder Structure

Placeholders may include:
* **Simple names**: `:user_id`, `:email`
* **Dot-separated paths**: `:config.database.host` (for nested data structures)
* **Array indices**: `:items[0].name`, `:tags[2]` (using bracket notation)


### Parser Design

* **Tokenizer-based approach**: Not a full SQL parser
* **Superset scanning**: Recognizes and skips all major SQL literal/identifier/comment forms:
  - Single quotes: `'...'`
  - Double quotes: `"..."`
  - Backticks: `` `...` ``
  - Bracket identifiers: `[...]`
  - Line comments: `-- ...`, `# ...`
  - Block comments: `/* ... */`
  - PostgreSQL dollar-quotes: `$tag$...$tag$`
  - Oracle Q-quotes: `q'<...>'`, `q'{...}'`, etc.

* **Edge case handling**:
  - PostgreSQL `::` type casts explicitly detected and skipped
  - Time literals in strings (`'12:30:00'`) handled by string skip logic
  - Colons in comments and identifiers already handled by superset scanning

### Backend Rewriting

The parser is database-agnostic. Backend-specific placeholder generation is delegated to a `FormatParamFunc`:

```go
type FormatParamFunc func(paramIndex int) string
```

**Examples:**
* PostgreSQL: `func(i int) string { return fmt.Sprintf("$%d", i) }`
* MySQL/SQLite: `func(int) string { return "?" }`
* SQL Server: `func(i int) string { return fmt.Sprintf("@p%d", i) }`

### API

```go
ParseSQL(sql SQLQuery, formatFunc FormatParamFunc) (ParsedSQL, error)
```

* **Input**: SQL template with `:name` placeholders + backend formatter
* **Output**: `ParsedSQL` containing:
  - Rewritten SQL with backend-native placeholders
  - Ordered list of parameter names (for binding)
  - All parameter occurrences (including duplicates)

### Duplicate Placeholders

The same `:name` may appear multiple times in SQL. The parser:
* Returns deduplicated parameter list (first occurrence wins)
* Tracks all occurrences for validation
* Substitutes consistently across all occurrences

### Scope

The SQL parser is responsible **only** for:
* Detecting `:name` placeholders outside of string literals, identifiers, and comments
* Rewriting placeholders into backend-native parameter syntax
* Producing an ordered parameter list for binding

The parser does **not** implement:
* Parameter validation (type checking, constraints)
* Optional parameters or default values
* Array expansion for `IN` clauses

These features should be handled by upstream systems before SQL generation.

## Examples

### Example 1: Simple Equality

**Template:**
```sql
SELECT * FROM users WHERE id = :id;
```

**PostgreSQL (rewritten):**
```sql
SELECT * FROM users WHERE id = $1;
```

**MySQL/SQLite (rewritten):**
```sql
SELECT * FROM users WHERE id = ?;
```

**SQL Server (rewritten):**
```sql
SELECT * FROM users WHERE id = @p1;
```

---

### Example 2: Multiple Parameters with Reuse

**Template:**
```sql
SELECT * FROM orders
WHERE account_id = :accountId
  AND created_at >= :since
  AND updated_at >= :since;
```

**PostgreSQL:**
```sql
SELECT * FROM orders
WHERE account_id = $1
  AND created_at >= $2
  AND updated_at >= $2;
```

Parameters: `["accountId", "since"]`
Binding: `[acct123, "2024-01-01T00:00:00Z"]`

**MySQL/SQLite:**
```sql
SELECT * FROM orders
WHERE account_id = ?
  AND created_at >= ?
  AND updated_at >= ?;
```

Note: MySQL/SQLite require the same value repeated for duplicate placeholders.

---

### Example 3: Nested Data (Dotted Paths)

**Template:**
```sql
INSERT INTO events (user_id, event_type, payload)
VALUES (:user.id, :event.type, :event.data);
```

**PostgreSQL:**
```sql
INSERT INTO events (user_id, event_type, payload)
VALUES ($1, $2, $3);
```

Parameters: `["user.id", "event.type", "event.data"]`

The application resolves these dotted paths from its data structure (e.g., extracting `user.id` from a nested JSON object or struct field).

---

### Example 4: Array Indices

**Template:**
```sql
SELECT * FROM products
WHERE id IN (:items[0].id, :items[1].id, :items[2].id);
```

**PostgreSQL:**
```sql
SELECT * FROM products
WHERE id IN ($1, $2, $3);
```

Parameters: `["items[0].id", "items[1].id", "items[2].id"]`

---

### Example 5: PostgreSQL Type Casts (No Conflict)

**Template:**
```sql
SELECT name::text, created_at::date
FROM users
WHERE id = :userId
  AND status::int = :status;
```

**PostgreSQL:**
```sql
SELECT name::text, created_at::date
FROM users
WHERE id = $1
  AND status::int = $2;
```

The `::` cast operator is explicitly handled and does not interfere with `:name` placeholders.

## Consequences

### Pros

* **IDE-friendly**: Standard SQL tooling recognizes `:name` syntax
* **Ecosystem alignment**: Matches conventions in Oracle, PDO, SQLAlchemy, ActiveRecord
* **Database-agnostic**: Works across SQLite, PostgreSQL, MySQL, MariaDB, SQL Server, DuckDB, Oracle, DB2
* **Simple implementation**: Tokenizer rather than full SQL parser
* **Extensible**: Supports nested data (dotted paths) and array indices
* **Safe**: Superset scanning avoids false matches in strings, comments, identifiers
* **Explicit edge case handling**: PostgreSQL `::`, time literals, etc.

### Cons

* **Custom syntax for paths**: `:user.id` and `:items[0]` are not standard SQL (but neither are any placeholder formats)
* **Requires tokenizer**: More complex than naive regex replacement
* **Backend-specific binding**: Applications must handle duplicate placeholders differently for MySQL vs PostgreSQL

### Trade-offs

* **Named vs Positional**: Named parameters simplify application logic but require parameter name tracking
* **Single pass parsing**: Cannot look ahead for validation (must be handled externally)

## Alternatives Considered

### 1. Positional Placeholders Only (`?`, `$1`, etc.)

**Rejected because:**
* Source data structures (maps, JSON objects) have no stable positional ordering
* Maintaining positional order across query modifications is error-prone
* Parameter reuse requires explicit repetition in source code

### 2. Use Standard `:name` Without Special Handling

**Initially considered, later adopted with edge case handling:**
* Original concern: Ambiguity with PostgreSQL `::` casts and time literals
* Solution: Explicit tokenization handles these cases correctly

### 3. Brace-Delimited Syntax (`{name}`)

**Rejected because:**
* Not recognized by SQL tooling or IDEs
* Creates visual noise and syntax errors in editors
* No precedent in SQL ecosystem

### 4. Prefix Operators (`:name:`, `{{name}}`)

**Rejected because:**
* Adds unnecessary verbosity
* No precedent in SQL ecosystem
* Harder to read and type

## Future Work

* **Array expansion**: Define syntax and behavior for `IN (:ids...)` expansion
* **Type annotations**: Consider optional inline type hints (e.g., `:id:uuid`)
* **Performance optimization**: Benchmark and optimize for large SQL templates

## References

* Oracle SQL: Named bind variables (`:name`)
* Python DB-API 2.0: Named parameter styles
* PDO (PHP): Named parameters
* SQLAlchemy: Bindparam mechanism
* ActiveRecord: Named placeholders in `where` clauses
