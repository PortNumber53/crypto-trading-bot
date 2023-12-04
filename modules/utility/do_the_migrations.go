package utility

import (
	"fmt"
	"log"
	"os"

	"github.com/golang-migrate/migrate/v4"
	_ "github.com/golang-migrate/migrate/v4/database/postgres"
	_ "github.com/golang-migrate/migrate/v4/source/file"
)

var logger = log.New(os.Stdout, "[Utility] ", log.LstdFlags|log.Lshortfile)

func DoMigrations(dbURL string) error {
	fmt.Println("- Looking for migrations")
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
