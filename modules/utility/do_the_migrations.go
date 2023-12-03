package utility

import (
	"log"

	"github.com/rubenv/sql-migrate"
)

func DoMigrations() {
    m, err := migrate.New(
        "file://db/migrations",
        "postgres://grimlock:1234@localhost:5432/dbcrypto?sslmode=disable",
    )
    if err != nil {
        log.Fatalf("Error creating migration instance: %s", err)
    }

    if err := m.Up(); err != nil && err != migrate.ErrNoChange {
        log.Fatalf("Error applying migrations: %s", err)
    } else if err == migrate.ErrNoChange {
        log.Println("No new migrations to apply")
    }
}