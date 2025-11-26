# ADR-002: Infinite Loop Protection in SQL Parser

## Status

Accepted (2025-11-26)

## Context

SQL parsers that use state machines to tokenize input are vulnerable to infinite loops when:
1. Edge cases in the state transition logic don't advance the position correctly
2. Malformed input creates unexpected state combinations
3. Character sequences trigger handlers that fail to progress

Historical issues in this parser included:
- Oracle Q-quote handler (`consumeOracleQ()`) not advancing position when rejecting non-Q-quote 'q' characters
- Potential infinite loops in delimiter matching logic
- No systematic detection of hang scenarios during development

Without protection, infinite loops manifest as:
- Tests hanging indefinitely (requiring manual kill)
- Production servers freezing on malformed SQL
- No clear indication of which input caused the hang
- Difficult debugging (stack traces show loop location but not root cause)

## Decision

### 1. Mandatory Position Advancement

**Rule**: Every loop iteration in the parser MUST advance the position counter or explicitly break/return.

**Implementation**:
```go
// Every case in the main parser loop must either:
// 1. Advance position and continue
// 2. Call a consumer function that advances position
// 3. Break or goto end

for state.i < state.n {
	c := state.src[state.i]

	switch c {
	case 'q', 'Q':
		state.consumeOracleQ() // Must advance position internally
		continue
	case ':':
		// ... handle placeholder
		continue
	}

	state.i++ // Default: always advance
}
```

**Critical fix example** (from `consumeOracleQ()`):
```go
// BEFORE (infinite loop):
if s.i+1 >= s.n {
	goto end  // BUG: Didn't advance position!
}

// AFTER (fixed):
if s.i+1 >= s.n {
	s.i = start + 1  // MUST advance past 'q'/'Q'
	goto end
}
```

### 2. Comprehensive Hang Detection Tests

**Test structure**: Each test case runs in a goroutine with 100ms timeout:

```go
func TestParseSQL_NoInfiniteLoops(t *testing.T) {
	tests := []struct {
		name string
		sql  SQLQuery
	}{
		{name: "lowercase q in SQL keyword", sql: "SELECT seq FROM..."},
		{name: "malformed Oracle Q-quote", sql: "... q'<text"},
		// ... more cases
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			done := make(chan struct{})

			go func() {
				_, _ = ParseSQL(tt.sql, format)
				close(done)
			}()

			select {
			case <-done:
				// Success: completed in time
			case <-time.After(100 * time.Millisecond):
				t.Fatal("Parser hung (infinite loop detected)")
			}
		})
	}
}
```

**Timeout rationale**:
- Normal parsing: < 1ms for typical queries
- 100ms timeout: 100x safety margin
- Fast failure: Developers get immediate feedback
- Prevents CI/CD hangs: Tests fail quickly rather than timing out

### 3. Test Coverage of Problematic Patterns

All known hang-inducing patterns must have explicit test coverage:

**SQL Keywords with 'q'/'Q':**
- `UNIQUE`, `REQUIRE`, `EQUAL`, `SEQUENCE`, `queue`, etc.
- These trigger the Oracle Q-quote handler which must correctly reject them

**Edge Cases:**
- Lowercase 'q' followed by parenthesis: `SELECT seq FROM...`
- Uppercase 'Q' followed by space: `SELECT * FROM QUEUE WHERE...`
- 'q' at end of string: `SELECT * FROM freq`
- Multiple 'q' characters: `SELECT question, quantity FROM quiz`

**Malformed Input:**
- Unclosed Oracle Q-quotes: `q'<text` (missing closing `>'`)
- Missing opening quote: `q<text>` (no apostrophe after 'q')
- Consecutive delimiters: `'>>>'` (could confuse delimiter matching)

**Real-World Queries:**
- Complex queries from production: `LOWER(title) LIKE LOWER('%' || :q || '%')`
- Combined edge cases: Multiple 'q's, casts, string literals together

### 4. CI/CD Integration

**Test execution**:
```bash
go test -v -timeout=10s ./...
```

**Failure mode**:
- Hung test: Timeout after 10s (global)
- Infinite loop detection: Fail after 100ms (per test case)
- Clear error: "Parser hung (infinite loop detected) - took longer than 100ms"

**Benefits**:
- **Fast feedback**: Developers know immediately if they broke something
- **Precise diagnosis**: Error message identifies exact test case
- **CI safety**: Pipeline doesn't hang for 30+ minutes
- **Regression prevention**: Once fixed, stays fixed

## Examples

### Example 1: Detecting Unhandled 'q' Character

**Test case**:
```go
{
	name: "lowercase q in SQL keyword",
	sql:  "SELECT seq FROM sequences WHERE id = :id",
}
```

**Without protection**: Parser would hang when 'q' in `seq` triggered `consumeOracleQ()` but failed to advance position correctly.

**With protection**: Test fails in 100ms with clear error message.

---

### Example 2: Malformed Oracle Q-Quote

**Test case**:
```go
{
	name: "malformed Oracle Q-quote (no closing)",
	sql:  "SELECT * FROM users WHERE name = q'<text",
}
```

**Without protection**: Parser might loop forever searching for closing delimiter.

**With protection**: Parser reaches EOF, advances to `s.n`, exits loop. Test passes.

---

### Example 3: Real-World Query

**Test case**:
```go
{
	name: "complex query from test config",
	sql:  "SELECT t.id, IFNULL(u.email,'') AS email FROM tasks t WHERE (LOWER(t.title) LIKE LOWER('%' || :q || '%'))",
}
```

**Impact**: Ensures production-like queries don't cause hangs.

## Consequences

### Pros

1. **Early detection**: Infinite loops caught during development, not production
2. **Fast debugging**: 100ms timeout + test name pinpoints exact problem
3. **Regression prevention**: Once fixed, comprehensive tests prevent recurrence
4. **CI/CD safety**: Tests don't hang for extended periods
5. **Developer confidence**: Can refactor parser logic without fear
6. **Documentation**: Test cases serve as examples of edge cases

### Cons

1. **Test maintenance**: Must add test cases for new edge cases
2. **Timeout tuning**: 100ms might be too aggressive for very slow systems (CI containers)
3. **False positives**: Extremely slow systems might trigger false failures

### Mitigation

- **Timeout configuration**: Could make timeout configurable via environment variable
- **Parallel execution**: Run tests in parallel to reduce overall test time
- **Selective testing**: Critical hang tests in separate suite with longer timeouts

## Implementation Checklist

When adding new parser functionality:

- [ ] Verify every loop advances position or exits explicitly
- [ ] Add test case for new syntax/edge cases
- [ ] Run infinite loop detection tests
- [ ] Document any new state machine transitions
- [ ] Consider malformed input variations

## Alternatives Considered

### 1. Iteration Count Limit

**Approach**: Limit loop iterations to 10x input length

```go
maxIterations := len(state.src) * 10
for iterations := 0; state.i < state.n && iterations < maxIterations; iterations++ {
	// ... parse
}
```

**Rejected because**:
- Hides bugs rather than fixing them
- Fails silently (returns incomplete parse)
- Arbitrary multiplier (why 10x?)
- Adds overhead to every parse

### 2. Global Timeout in Parser

**Approach**: Add timeout inside `ParseSQL()` function

```go
done := make(chan ParsedSQL)
go func() {
	// ... parse
	done <- result
}()

select {
case result := <-done:
	return result, nil
case <-time.After(1 * time.Second):
	return ParsedSQL{}, errors.New("parse timeout")
}
```

**Rejected because**:
- Adds goroutine overhead to every parse
- Timeout value is application-specific
- Doesn't help with debugging
- Masks underlying bugs

### 3. Manual Code Review Only

**Approach**: Rely on code review to catch infinite loop potential

**Rejected because**:
- Human error inevitable
- No systematic verification
- Refactoring can introduce regressions
- New contributors may not know all edge cases

## References

- **Oracle SQL**: Q-quote string literal syntax (`q'<text>'`, `q'{text}'`)
- **PostgreSQL**: Double-colon cast operator (`::`)
- **Testing best practices**: Fast failure, clear error messages
- **State machine design**: Guaranteed progress principles

## Fuzzing Implementation

**Implemented**: 2025-11-26

Native Go fuzzing has been added to automatically discover edge cases and verify parser robustness.

### Implementation Details

Three fuzz functions cover all major database parameter formats:

1. **FuzzParseSQL**: PostgreSQL format (`$1`, `$2`, ...)
2. **FuzzParseSQLWithMySQLFormat**: MySQL format (`?`)
3. **FuzzParseSQLWithSQLServerFormat**: SQL Server format (`@p1`, `@p2`, ...)

**Seed corpus** (60+ test cases):
- Basic named parameters (`:simple`, `:user_id`)
- Dotted paths (`:config.nested.value`)
- Array indices (`:items[0].name`, `:items[99].deeply.nested[5].value`)
- PostgreSQL `::` casts
- Oracle Q-quotes (valid and malformed)
- String literals with colons
- Comments (line, block, hash)
- Dollar quotes
- SQL keywords with 'q'/'Q' characters
- Malformed input (`:`, `:::`, `:123invalid`, `:[`)
- Long inputs (1KB, 10KB)
- Real-world queries from production

**Safety guarantees**:
```go
f.Fuzz(func(t *testing.T, sql string) {
    done := make(chan struct{})
    go func() {
        defer func() {
            if r := recover(); r != nil {
                t.Errorf("Parser panicked: %v", r)
            }
            close(done)
        }()
        result, err = dbqvars.ParseSQL(dbqvars.SQLQuery(sql), formatFunc)
    }()

    select {
    case <-done:
        // Verify output is sensible
    case <-time.After(100 * time.Millisecond):
        t.Fatalf("Parser hung on input: %q", sql)
    }
})
```

**Verification checks**:
- No panics on any input
- No infinite loops (100ms timeout)
- Non-empty SQL output for non-empty input
- Parameter count never exceeds input length
- All parameters have non-empty names

**Performance** (example run):
- PostgreSQL: 941,570 executions in 6s (145,546/sec), 219 interesting inputs
- MySQL: 579,047 executions in 3s (193,007/sec), 166 interesting inputs
- SQL Server: 465,963 executions in 3s (155,319/sec), 168 interesting inputs

**Running fuzzing**:
```bash
# Run for 30 seconds (default)
go test -fuzz=FuzzParseSQL -fuzztime=30s

# Run until failure or Ctrl+C
go test -fuzz=FuzzParseSQL

# Test all format variants
go test -fuzz=FuzzParseSQLWithMySQLFormat -fuzztime=30s
go test -fuzz=FuzzParseSQLWithSQLServerFormat -fuzztime=30s
```

**Crash artifacts**: Discovered failures are automatically saved to `testdata/fuzz/FuzzParseSQL/` for regression testing.

## Future Work

1. **Performance benchmarks**: Ensure hang detection doesn't slow normal parsing
2. **Timeout configuration**: Environment variable for CI/CD tuning
3. **Visualization**: Generate state transition diagrams for documentation
4. **Continuous fuzzing**: Run fuzzing in CI/CD for longer durations (5+ minutes)
