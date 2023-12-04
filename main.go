// main.go
package main

import (
	"fmt"
	"log"
	"os"

	"trading/modules/utility"
)

	func main() {
		fmt.Println("Hello, Go!")

		dbURL := os.Getenv("DB_URL")
		if dbURL == "" {
			log.Fatal("DB_URL environment variable not set")
		}

		// Run migrations
		if err := utility.DoMigrations(dbURL); err != nil {
			log.Fatalf("Error: %s", err)
		}

	}
