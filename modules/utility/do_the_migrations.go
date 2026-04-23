package utility

import (
	"fmt"
	"log"
	"net/url"
	"os"
	"strings"

	"github.com/golang-migrate/migrate/v4"
	_ "github.com/golang-migrate/migrate/v4/database/postgres"
	_ "github.com/golang-migrate/migrate/v4/source/file"
)

var logger = log.New(os.Stdout, "[Utility] ", log.LstdFlags|log.Lshortfile)

// ensureSSLMode ensures the database URL has an explicit sslmode parameter.
// golang-migrate defaults to sslmode=prefer which causes EOF errors when
// the server does not support SSL.
func ensureSSLMode(dbURL string) string {
	parsed, err := url.Parse(dbURL)
	if err != nil {
		return dbURL
	}
	q := parsed.Query()
	if q.Get("sslmode") == "" {
		q.Set("sslmode", "disable")
		parsed.RawQuery = q.Encode()
		return parsed.String()
	}
	return dbURL
}

// normalizeDBURL ensures the URL uses the postgres:// scheme that
// golang-migrate expects and has an explicit sslmode.
func normalizeDBURL(dbURL string) string {
	dbURL = strings.TrimSpace(dbURL)
	dbURL = ensureSSLMode(dbURL)
	return dbURL
}

func DoMigrations(dbURL string) error {
	fmt.Println("- Looking for migrations")
	dbURL = normalizeDBURL(dbURL)
	m, err := migrate.New(
		"file://db/migrations",
		dbURL,
	)
	if err != nil {
		return fmt.Errorf("error creating migration instance: %w", err)
	}

	if err := m.Up(); err != nil && err != migrate.ErrNoChange {
		return fmt.Errorf("error applying migrations: %w", err)
	}

	fmt.Println("- Migrations applied successfully")
	return nil
}

// RunMigrationsUp runs all pending migrations up
func RunMigrationsUp(dbURL string) error {
	logger.Println("Running migrations up...")
	dbURL = normalizeDBURL(dbURL)
	m, err := migrate.New(
		"file://db/migrations",
		dbURL,
	)
	if err != nil {
		return fmt.Errorf("error creating migration instance: %w", err)
	}

	err = m.Up()
	if err != nil && err != migrate.ErrNoChange {
		return fmt.Errorf("error applying migrations: %w", err)
	}

	if err == migrate.ErrNoChange {
		logger.Println("No pending migrations to apply")
	} else {
		logger.Println("Migrations applied successfully")
	}
	return nil
}

// RunMigrationsDown runs migrations down by specified steps (use -1 for all)
func RunMigrationsDown(dbURL string, steps int) error {
	logger.Printf("Running migrations down by %d steps...", steps)
	dbURL = normalizeDBURL(dbURL)
	m, err := migrate.New(
		"file://db/migrations",
		dbURL,
	)
	if err != nil {
		return fmt.Errorf("error creating migration instance: %w", err)
	}

	err = m.Steps(steps)
	if err != nil && err != migrate.ErrNoChange {
		return fmt.Errorf("error rolling back migrations: %w", err)
	}

	logger.Println("Migrations rolled back successfully")
	return nil
}

// GetMigrationVersion returns the current migration version
func GetMigrationVersion(dbURL string) (uint, bool, error) {
	dbURL = normalizeDBURL(dbURL)
	m, err := migrate.New(
		"file://db/migrations",
		dbURL,
	)
	if err != nil {
		return 0, false, fmt.Errorf("error creating migration instance: %w", err)
	}

	version, dirty, err := m.Version()
	return version, dirty, err
}

// CreateMigration creates a new migration file
func CreateMigration(name string) error {
	logger.Printf("Creating migration: %s", name)

	// For now, just provide instructions since we need to manually create migration files
	fmt.Printf("To create a new migration, manually create files in db/migrations/:\n")
	fmt.Printf("  %04d_%s.up.sql\n", getNextMigrationNumber(), name)
	fmt.Printf("  %04d_%s.down.sql\n", getNextMigrationNumber(), name)
	fmt.Printf("\nExample:\n")
	fmt.Printf("  000005_create_example_table.up.sql\n")
	fmt.Printf("  000005_create_example_table.down.sql\n")

	return nil
}

// getNextMigrationNumber calculates the next migration number
func getNextMigrationNumber() int {
	files, err := os.ReadDir("db/migrations")
	if err != nil {
		return 1
	}

	maxNum := 0
	for _, file := range files {
		if file.IsDir() {
			continue
		}

		var num int
		if _, err := fmt.Sscanf(file.Name(), "%04d_", &num); err == nil {
			if num > maxNum {
				maxNum = num
			}
		}
	}

	return maxNum + 1
}
