// modules/database/database.go
package database

import (
	"database/sql"
	"fmt"
	"log"
	"os"

	_ "github.com/lib/pq"
)

// OpenConnection opens a connection to the PostgreSQL database
func OpenConnection() (*sql.DB, error) {
	var connStr string

	// Prefer DATABASE_URL if set
	if dbURL := os.Getenv("DATABASE_URL"); dbURL != "" {
		connStr = dbURL
		// Ensure sslmode is set
		if !contains(connStr, "sslmode") {
			if contains(connStr, "?") {
				connStr += "&sslmode=disable"
			} else {
				connStr += "?sslmode=disable"
			}
		}
	} else {
		host := os.Getenv("DB_HOST")
		port := os.Getenv("DB_PORT")
		user := os.Getenv("DB_USER")
		password := os.Getenv("DB_PASSWORD")
		dbname := os.Getenv("DB_NAME")

		connStr = fmt.Sprintf("host=%s port=%s user=%s password=%s dbname=%s sslmode=disable",
			host, port, user, password, dbname)
	}

	// Open a connection to the database
	db, err := sql.Open("postgres", connStr)
	if err != nil {
		log.Fatalf("Error opening database connection: %s", err)
		return nil, err
	}

	// Check if the connection is successful
	if err := db.Ping(); err != nil {
		log.Fatalf("Error pinging database: %s", err)
		db.Close()
		return nil, err
	}

	log.Println("- Database connection established")
	return db, nil
}

// contains checks if a string contains a substring
func contains(s, substr string) bool {
	return len(s) >= len(substr) && (s == substr || len(s) > 0 && containsSubstring(s, substr))
}

func containsSubstring(s, substr string) bool {
	for i := 0; i <= len(s)-len(substr); i++ {
		if s[i:i+len(substr)] == substr {
			return true
		}
	}
	return false
}

// CloseConnection closes the connection to the PostgreSQL database
func CloseConnection(db *sql.DB) {
	if db != nil {
		if err := db.Close(); err != nil {
			log.Printf("Error closing database connection: %s", err)
		} else {
			log.Println("- Database connection closed")
		}
	}
}
