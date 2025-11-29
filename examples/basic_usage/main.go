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
