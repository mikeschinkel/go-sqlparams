package test

import (
	"fmt"
	"testing"
	"time"

	"github.com/mikeschinkel/go-sqlparams"
)

// FuzzParseSQL uses Go's built-in fuzzing to discover edge cases in the SQL parser.
//
// Run with:
//
//	go test -fuzz=FuzzParseSQL -fuzztime=30s
//
// The fuzzer will generate random SQL strings and test that the parser:
// 1. Never panics
// 2. Never hangs (completes within timeout)
// 3. Always returns valid output or error
//
// Discovered crashes are saved to testdata/fuzz/FuzzParseSQL/
func FuzzParseSQL(f *testing.F) {
	// Seed corpus: Known good inputs that cover different parser paths
	seeds := []string{
		// Basic cases
		"SELECT * FROM users WHERE id = :id",
		"SELECT * FROM users",
		"",

		// Named parameters
		":simple",
		":user_id",
		":config.nested.value",
		":items[0].name",
		":items[99].deeply.nested[5].value",

		// Edge cases - colons
		"SELECT '12:30:00' as time",
		"SELECT name::text FROM users WHERE id = :id",
		"SELECT * FROM users WHERE created > '2024-01-01 10:30:00'",
		":::",
		": : :",

		// Edge cases - q/Q characters
		"SELECT seq FROM sequences",
		"SELECT question, quantity FROM quiz WHERE quality > :threshold",
		"SELECT * FROM queue",
		"q",
		"Q",
		"qqq",
		"QQQ",

		// Oracle Q-quotes
		"SELECT * FROM users WHERE name = q'<text>'",
		"SELECT * FROM users WHERE name = q'{text}'",
		"SELECT * FROM users WHERE name = q'[text]'",
		"SELECT * FROM users WHERE name = q'(text)'",
		"q'",
		"q'<",
		"q'<>",
		"q'<>'",

		// Comments
		"-- comment\nSELECT :id",
		"/* comment */ SELECT :id",
		"# comment\nSELECT :id",

		// String literals
		"'string with :fake'",
		"\"string with :fake\"",
		"`identifier with :fake`",
		"[identifier with :fake]",

		// PostgreSQL dollar quotes
		"SELECT $tag$text with :fake$tag$",
		"$$text$$",

		// Duplicates
		"SELECT * FROM orders WHERE created >= :since AND updated >= :since",

		// Complex real-world query
		"SELECT t.id, IFNULL(u.email,'') AS email FROM tasks t LEFT JOIN users u ON u.id = t.assignee_id WHERE (LOWER(t.title) LIKE LOWER('%' || :q || '%'))",

		// Edge cases - special characters
		":name_with_underscores",
		":name123",
		":_private",
		":::multiple:::colons:::",

		// Malformed
		":",
		":123invalid",
		":.",
		":[]",
		":[",
		":]",

		// Long inputs (test performance)
		string(make([]byte, 1000)),
		string(make([]byte, 10000)),
	}

	for _, seed := range seeds {
		f.Add(seed)
	}

	// Format function for testing
	postgresFormat := func(i int) string {
		return fmt.Sprintf("$%d", i)
	}

	f.Fuzz(func(t *testing.T, sql string) {
		// Use timeout to catch infinite loops
		done := make(chan struct{})
		var result sqlparams.ParsedSQL
		var err error

		go func() {
			defer func() {
				// Catch panics
				if r := recover(); r != nil {
					t.Errorf("Parser panicked on input %q: %v", sql, r)
				}
				close(done)
			}()

			result, err = sqlparams.ParseSQL(sqlparams.SQLQuery(sql), postgresFormat)
		}()

		select {
		case <-done:
			// Parse completed successfully or returned error
			// Both are acceptable outcomes

			if err != nil {
				// Error is acceptable - just verify it's a known error type
				t.Logf("Parse error (acceptable): %v", err)
				return
			}

			// Verify output is sensible
			if result.SQL == "" && sql != "" {
				t.Errorf("Parser returned empty SQL for non-empty input: %q", sql)
			}

			// Verify parameter count is reasonable (shouldn't exceed input length)
			if len(result.Parameters()) > len(sql) {
				t.Errorf("Parser returned more parameters (%d) than input length (%d) for: %q",
					len(result.Parameters()), len(sql), sql)
			}

			// Verify all parameters have valid names (if any)
			for _, param := range result.Parameters() {
				if len(string(param.Name)) == 0 {
					t.Errorf("Parser returned empty parameter name for input: %q", sql)
				}
			}

		case <-time.After(10 * time.Second):
			t.Fatalf("Parser hung (infinite loop detected) on input: %q", sql)
		}
	})
}

// FuzzParseSQLWithMySQLFormat fuzzes with MySQL format (?) to test different code paths
func FuzzParseSQLWithMySQLFormat(f *testing.F) {
	// Seed with some basic cases
	f.Add("SELECT * FROM users WHERE id = :id")
	f.Add(":param")
	f.Add("")

	mysqlFormat := func(int) string { return "?" }

	f.Fuzz(func(t *testing.T, sql string) {
		done := make(chan struct{})

		go func() {
			defer func() {
				if r := recover(); r != nil {
					t.Errorf("Parser panicked: %v", r)
				}
				close(done)
			}()

			_, _ = sqlparams.ParseSQL(sqlparams.SQLQuery(sql), mysqlFormat)
		}()

		select {
		case <-done:
			// Success
		case <-time.After(1 * time.Second):
			t.Fatalf("Parser hung on input: %q", sql)
		}
	})
}

// FuzzParseSQLWithSQLServerFormat fuzzes with SQL Server format (@p1) to test different code paths
func FuzzParseSQLWithSQLServerFormat(f *testing.F) {
	// Seed with some basic cases
	f.Add("SELECT * FROM users WHERE id = :id")
	f.Add(":param")
	f.Add("")

	sqlServerFormat := func(i int) string {
		return fmt.Sprintf("@p%d", i)
	}

	f.Fuzz(func(t *testing.T, sql string) {
		done := make(chan struct{})

		go func() {
			defer func() {
				if r := recover(); r != nil {
					t.Errorf("Parser panicked: %v", r)
				}
				close(done)
			}()

			_, _ = sqlparams.ParseSQL(sqlparams.SQLQuery(sql), sqlServerFormat)
		}()

		select {
		case <-done:
			// Success
		case <-time.After(1 * time.Second):
			t.Fatalf("Parser hung on input: %q", sql)
		}
	})
}
