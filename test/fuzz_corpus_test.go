package test

import (
	"bufio"
	"fmt"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/mikeschinkel/go-sqlparams"
)

// TestFuzzCorpus reads each fuzz corpus file and tests it with timeout detection
func TestFuzzCorpus(t *testing.T) {
	corpusDir := "testdata/fuzz/FuzzParseSQL"
	entries, err := os.ReadDir(corpusDir)
	if err != nil {
		t.Fatalf("Failed to read corpus directory: %v", err)
	}

	formatFunc := func(i int) string { return fmt.Sprintf("$%d", i) }

	infiniteLoops := []string{}
	parseErrors := []string{}
	successes := []string{}

	for _, entry := range entries {
		if entry.IsDir() {
			continue
		}

		// Read the fuzz corpus file
		path := filepath.Join(corpusDir, entry.Name())
		f, err := os.Open(path)
		if err != nil {
			t.Logf("Failed to open %s: %v", entry.Name(), err)
			continue
		}

		scanner := bufio.NewScanner(f)
		lineNum := 0
		var input string
		for scanner.Scan() {
			lineNum++
			if lineNum == 2 { // Second line contains string("...")
				line := scanner.Text()
				if strings.HasPrefix(line, "string(") && strings.HasSuffix(line, ")") {
					strLiteral := line[7 : len(line)-1] // Remove "string(" and ")"
					unquoted, err := strconv.Unquote(strLiteral)
					if err != nil {
						t.Logf("Error unquoting %s: %v", entry.Name(), err)
						break
					}
					input = unquoted
				}
				break
			}
		}
		err = f.Close()
		if err != nil {
			t.Error(err.Error())
		}

		if input == "" {
			continue
		}

		// Test this input with timeout
		done := make(chan struct{})
		var result sqlparams.ParsedSQL
		var parseErr error

		go func() {
			defer func() {
				if r := recover(); r != nil {
					parseErr = fmt.Errorf("PANIC: %v", r)
				}
				close(done)
			}()

			result, parseErr = sqlparams.ParseSQL(sqlparams.SQLQuery(input), formatFunc)
		}()

		select {
		case <-done:
			// Parse completed
			if parseErr != nil {
				parseErrors = append(parseErrors, entry.Name())
				t.Logf("%-20s ERROR: %v", entry.Name(), parseErr)
			} else {
				successes = append(successes, entry.Name())
				t.Logf("%-20s OK: %d params", entry.Name(), len(result.Parameters()))
			}
		case <-time.After(10 * time.Second):
			infiniteLoops = append(infiniteLoops, entry.Name())
			t.Errorf("%-20s INFINITE LOOP: %q", entry.Name(), input)
		}
	}

	// Summary
	t.Logf("\n=== SUMMARY ===")
	t.Logf("Total files: %d", len(entries))
	t.Logf("Infinite loops: %d", len(infiniteLoops))
	t.Logf("Parse errors: %d", len(parseErrors))
	t.Logf("Successes: %d", len(successes))

	if len(infiniteLoops) > 0 {
		t.Logf("\nFiles causing infinite loops:")
		for _, name := range infiniteLoops {
			t.Logf("  - %s", name)
		}
		t.Fatalf("Found %d infinite loop(s)", len(infiniteLoops))
	}
}
