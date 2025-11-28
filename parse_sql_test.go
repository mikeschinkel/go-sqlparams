package sqlparams

import (
	"errors"
	"fmt"
	"testing"
	"time"
)

// noinspection SqlResolveForFile

func TestParseSQL(t *testing.T) {
	tests := []struct {
		name            string
		sql             SQLQuery
		formatParamFunc FormatParamFunc
		expected        ParsedSQL
		expectError     bool
		expectedError   error
	}{
		// Basic cases
		{
			name: "LIKE ':file_path'",
			sql:  "SELECT id FROM logs WHERE file_path LIKE :file_path || '%';",
			formatParamFunc: func(i int) string {
				return fmt.Sprintf("$%d", i)
			},
			expected: NewParsedSQL("SELECT id FROM logs WHERE file_path LIKE $1 || '%';", NewParameters("file_path")),
		},
		{
			name: "no placeholders",
			sql:  "SELECT * FROM users WHERE active = true",
			formatParamFunc: func(i int) string {
				return fmt.Sprintf("$%d", i)
			},
			expected: NewParsedSQL("SELECT * FROM users WHERE active = true", nil),
		},
		{
			name: "single placeholder",
			sql:  "SELECT * FROM users WHERE id = :id",
			formatParamFunc: func(i int) string {
				return fmt.Sprintf("$%d", i)
			},
			expected: NewParsedSQL("SELECT * FROM users WHERE id = $1", NewParameters("id")),
		},
		{
			name: "multiple unique placeholders",
			sql:  "SELECT * FROM orders WHERE account_id=:accountId AND created_at>=:since",
			formatParamFunc: func(i int) string {
				return fmt.Sprintf("$%d", i)
			},
			expected: NewParsedSQL("SELECT * FROM orders WHERE account_id=$1 AND created_at>=$2", NewParameters("accountId", "since")),
		},
		{
			name: "duplicate placeholders",
			sql:  "SELECT * FROM orders WHERE created_at >= :since AND updated_at >= :since",
			formatParamFunc: func(i int) string {
				return fmt.Sprintf("$%d", i)
			},
			expected: NewParsedSQL("SELECT * FROM orders WHERE created_at >= $1 AND updated_at >= $1", NewParameters("since")),
		},
		// Dotted path placeholders (ADR-008)
		{
			name: "dotted path placeholders",
			sql:  "INSERT INTO events (user_id, payload) VALUES (:user.id, :body.event)",
			formatParamFunc: func(i int) string {
				return fmt.Sprintf("$%d", i)
			},
			expected: NewParsedSQL("INSERT INTO events (user_id, payload) VALUES ($1, $2)", NewParameters("user.id", "body.event")),
		},
		{
			name: "array index placeholders",
			sql:  "SELECT * FROM products WHERE id = :items[0].id AND sku = :items[1].sku",
			formatParamFunc: func(i int) string {
				return fmt.Sprintf("$%d", i)
			},
			expected: NewParsedSQL("SELECT * FROM products WHERE id = $1 AND sku = $2", NewParameters("items[0].id", "items[1].sku")),
		},
		// Different database backends
		{
			name: "mysql format",
			sql:  "SELECT * FROM users WHERE id = :id",
			formatParamFunc: func(int) string {
				return "?"
			},
			expected: NewParsedSQL("SELECT * FROM users WHERE id = ?", NewParameters("id")),
		},
		{
			name: "sql server format",
			sql:  "SELECT * FROM users WHERE id = :id",
			formatParamFunc: func(i int) string {
				return fmt.Sprintf("@p%d", i)
			},
			expected: NewParsedSQL("SELECT * FROM users WHERE id = @p1", NewParameters("id")),
		},
		// String literal skipping
		{
			name: "placeholder in single quotes ignored",
			sql:  "SELECT * FROM users WHERE name = 'John :id Doe' AND id = :id",
			formatParamFunc: func(i int) string {
				return fmt.Sprintf("$%d", i)
			},
			expected: NewParsedSQL("SELECT * FROM users WHERE name = 'John :id Doe' AND id = $1", NewParameters("id")),
		},
		{
			name: "placeholder in double quotes ignored",
			sql:  `SELECT * FROM users WHERE name = "John :id Doe" AND id = :id`,
			formatParamFunc: func(i int) string {
				return fmt.Sprintf("$%d", i)
			},
			expected: NewParsedSQL(`SELECT * FROM users WHERE name = "John :id Doe" AND id = $1`, NewParameters("id")),
		},
		// Comment skipping
		{
			name: "placeholder in line comment ignored",
			sql:  "SELECT * FROM users -- WHERE id = :id\nWHERE active = true AND id = :id",
			formatParamFunc: func(i int) string {
				return fmt.Sprintf("$%d", i)
			},
			expected: NewParsedSQL("SELECT * FROM users -- WHERE id = :id\nWHERE active = true AND id = $1", NewParameters("id")),
		},
		{
			name: "placeholder in block comment ignored",
			sql:  "SELECT * FROM users /* WHERE id = :id */ WHERE active = true AND id = :id",
			formatParamFunc: func(i int) string {
				return fmt.Sprintf("$%d", i)
			},
			expected: NewParsedSQL("SELECT * FROM users /* WHERE id = :id */ WHERE active = true AND id = $1", NewParameters("id")),
		},
		// Error cases
		{
			name:            "nil format function",
			sql:             "SELECT * FROM users WHERE id = :id",
			formatParamFunc: nil,
			expectError:     true,
			expectedError:   ErrFormatParamFuncRequired,
		},
		{
			name: "invalid placeholder name starting with digit",
			sql:  "SELECT * FROM users WHERE id = :123invalid",
			formatParamFunc: func(i int) string {
				return fmt.Sprintf("$%d", i)
			},
			// This should NOT error - :123 is just treated as a colon followed by "123invalid"
			// The colon is skipped since next char (1) is not a valid identifier start
			expected: NewParsedSQL("SELECT * FROM users WHERE id = :123invalid", nil),
		},
		{
			name: "dot-digit notation rejected (use bracket notation instead)",
			sql:  "SELECT * FROM products WHERE id = :items.0.id",
			formatParamFunc: func(i int) string {
				return fmt.Sprintf("$%d", i)
			},
			expectError:   true,
			expectedError: ErrInvalidPlaceholderName,
		},
		// Complex realistic examples
		{
			name: "complex query with multiple features",
			sql: `SELECT u.*, p.title
FROM users u
LEFT JOIN posts p ON u.id = p.user_id
WHERE u.created_at >= :filters.since
  AND u.status IN ('active', 'pending')
  AND (u.name LIKE :search.name OR u.email LIKE :search.email)
  AND u.org_id = :auth.org_id
ORDER BY u.created_at DESC`,
			formatParamFunc: func(i int) string {
				return fmt.Sprintf("$%d", i)
			},
			expected: NewParsedSQL(
				`SELECT u.*, p.title
FROM users u
LEFT JOIN posts p ON u.id = p.user_id
WHERE u.created_at >= $1
  AND u.status IN ('active', 'pending')
  AND (u.name LIKE $2 OR u.email LIKE $3)
  AND u.org_id = $4
ORDER BY u.created_at DESC`,
				NewParameters("filters.since", "search.name", "search.email", "auth.org_id")),
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result, err := ParseSQL(tt.sql, tt.formatParamFunc)

			if tt.expectError {
				if err == nil {
					t.Fatalf("expected error but got none")
				}
				if tt.expectedError != nil && !errors.Is(err, tt.expectedError) {
					t.Errorf("expected error %v, got %v", tt.expectedError, err)
				}
				return
			}

			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}

			if result.SQL != tt.expected.SQL {
				t.Errorf("SQL mismatch:\nexpected: %q\nactual:   %q", tt.expected.SQL, result.SQL)
			}

			// Compare only the essential fields (Name and Index)
			if len(result.Parameters()) != len(tt.expected.Parameters()) {
				t.Errorf("Parameters length mismatch: expected %d, got %d", len(tt.expected.Parameters()), len(result.Parameters()))
			} else {
				for i, expectedParam := range tt.expected.Parameters() {
					actualParam := result.Parameters()[i]
					if actualParam != expectedParam {
						t.Errorf("Param[%d] Name mismatch: expected %q, got %q", i, expectedParam, actualParam)
					}
				}
			}
		})
	}
}

func TestParseSQL_EdgeCases(t *testing.T) {
	tests := []struct {
		name            string
		sql             SQLQuery
		formatParamFunc FormatParamFunc
		expected        ParsedSQL
	}{
		{
			name: "escaped single quotes",
			sql:  "SELECT * FROM users WHERE name = 'O''Brien :id' AND id = :id",
			formatParamFunc: func(i int) string {
				return fmt.Sprintf("$%d", i)
			},
			expected: NewParsedSQL("SELECT * FROM users WHERE name = 'O''Brien :id' AND id = $1", NewParameters("id")),
		},
		{
			name: "backtick identifiers",
			sql:  "SELECT * FROM `users` WHERE `user-id` = :id",
			formatParamFunc: func(i int) string {
				return fmt.Sprintf("$%d", i)
			},
			expected: NewParsedSQL("SELECT * FROM `users` WHERE `user-id` = $1", NewParameters("id")),
		},
		{
			name: "bracket identifiers",
			sql:  "SELECT * FROM [users] WHERE [user-id] = :id",
			formatParamFunc: func(i int) string {
				return fmt.Sprintf("$%d", i)
			},
			expected: NewParsedSQL("SELECT * FROM [users] WHERE [user-id] = $1", NewParameters("id")),
		},
		{
			name: "hash comment",
			sql:  "SELECT * FROM users # WHERE id = :id\nWHERE active = true AND id = :id",
			formatParamFunc: func(i int) string {
				return fmt.Sprintf("$%d", i)
			},
			expected: NewParsedSQL(
				"SELECT * FROM users # WHERE id = :id\nWHERE active = true AND id = $1",
				NewParameters("id"),
			),
		},
		{
			name: "postgresql dollar quoting",
			sql:  "SELECT * FROM users WHERE desc = $tag$:id not a placeholder$tag$ AND id = :id",
			formatParamFunc: func(i int) string {
				return fmt.Sprintf("$%d", i)
			},
			expected: NewParsedSQL(
				"SELECT * FROM users WHERE desc = $tag$:id not a placeholder$tag$ AND id = $1",
				NewParameters("id"),
			),
		},
		{
			name: "postgresql cast :: not a placeholder",
			sql:  "SELECT name::text, id::varchar FROM users WHERE id = :id",
			formatParamFunc: func(i int) string {
				return fmt.Sprintf("$%d", i)
			},
			expected: NewParsedSQL(
				"SELECT name::text, id::varchar FROM users WHERE id = $1",
				NewParameters("id"),
			),
		},
		{
			name: "time literal with colons not placeholders",
			sql:  "SELECT * FROM events WHERE time = '12:30:00' AND user_id = :userId",
			formatParamFunc: func(i int) string {
				return fmt.Sprintf("$%d", i)
			},
			expected: NewParsedSQL(
				"SELECT * FROM events WHERE time = '12:30:00' AND user_id = $1",
				NewParameters("userId"),
			),
		},
		{
			name: "colon in string literal ignored",
			sql:  "SELECT * FROM logs WHERE msg = 'Error: connection failed' AND user_id = :userId",
			formatParamFunc: func(i int) string {
				return fmt.Sprintf("$%d", i)
			},
			expected: NewParsedSQL(
				"SELECT * FROM logs WHERE msg = 'Error: connection failed' AND user_id = $1",
				NewParameters("userId"),
			),
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result, err := ParseSQL(tt.sql, tt.formatParamFunc)

			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}

			if result.SQL != tt.expected.SQL {
				t.Errorf("SQL mismatch:\nexpected: %q\nactual:   %q", tt.expected.SQL, result.SQL)
			}

			// Compare only the essential fields (Name and Index)
			if len(result.Parameters()) != len(tt.expected.Parameters()) {
				t.Errorf("Parameters length mismatch: expected %d, got %d", len(tt.expected.Parameters()), len(result.Parameters()))
			} else {
				for i, expectedParam := range tt.expected.Parameters() {
					actualParam := result.Parameters()[i]
					if actualParam != expectedParam {
						t.Errorf("Param[%d] Name mismatch: expected %q, got %q", i, expectedParam, actualParam)
					}
				}
			}
		})
	}
}

// TestParseSQL_NoInfiniteLoops tests patterns that could potentially cause infinite loops
// in the parser, particularly in consumeOracleQ() and other stateful parsing functions.
// Each test has a 100ms timeout to catch hangs quickly.
func TestParseSQL_NoInfiniteLoops(t *testing.T) {
	tests := []struct {
		name string
		sql  SQLQuery
	}{
		{
			name: "lowercase q in SQL keyword",
			sql:  "SELECT id FROM logs WHERE file_path LIKE :path || '%'",
		},
		{
			name: "uppercase Q in SQL function",
			sql:  "SELECT IFNULL(email, '') FROM users WHERE id = :id",
		},
		{
			name: "q followed by parenthesis",
			sql:  "SELECT seq FROM sequences WHERE id = :id",
		},
		{
			name: "q followed by space (not Oracle Q-quote)",
			sql:  "SELECT * FROM queue WHERE status = :status",
		},
		{
			name: "multiple q characters",
			sql:  "SELECT question, quantity FROM quiz WHERE quality > :threshold",
		},
		{
			name: "q at end of string",
			sql:  "SELECT * FROM freq",
		},
		{
			name: "malformed Oracle Q-quote (no closing)",
			sql:  "SELECT * FROM users WHERE name = q'<text",
		},
		{
			name: "malformed Oracle Q-quote (missing quote)",
			sql:  "SELECT * FROM users WHERE name = q<text>",
		},
		{
			name: "consecutive delimiters",
			sql:  "SELECT * FROM users WHERE name = '>>>' AND id = :id",
		},
		{
			name: "query with LOWER and q parameter",
			sql:  "SELECT * FROM tasks WHERE LOWER(title) LIKE LOWER('%' || :q || '%')",
		},
		{
			name: "complex query from test config",
			sql:  "SELECT t.id, IFNULL(u.email,'') AS email FROM tasks t LEFT JOIN users u ON u.id = t.assignee_id WHERE (LOWER(t.title) LIKE LOWER('%' || :q || '%'))",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Use a channel and goroutine with timeout to detect hangs
			done := make(chan struct{})
			var result ParsedSQL
			var err error

			go func() {
				result, err = ParseSQL(tt.sql, func(i int) string {
					return "?"
				})
				close(done)
			}()

			select {
			case <-done:
				// Test completed successfully (error or not)
				// We don't care about the result, just that it didn't hang
				//if err != nil {
				//	 t.Logf("Parse returned error (expected for malformed input): %v", err)
				//} else {
				//	t.Logf("Parse succeeded: %d parameters found", len(result.Parameters()))
				//}
			case <-time.After(100 * time.Millisecond):
				t.Fatal("Parser hung (infinite loop detected) - took longer than 100ms")
			}
		})
	}
}
