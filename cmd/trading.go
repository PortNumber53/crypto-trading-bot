package main

import (
	"crypto/trading/modules/coinbase"
	"fmt"
	"log"

	"github.com/golang-migrate/migrate/v4"
	_ "github.com/golang-migrate/migrate/v4/database/postgres"
	_ "github.com/golang-migrate/migrate/v4/source/file"
)

func main() {
	fmt.Println("Hello, world!")

	utility.DoMigrations()

	currencies, err := coinbaseapi.FetchCurrencies()
	if err != nil {
		fmt.Println("Error fetching currencies:", err)
		return
	}

	// Print each currency's details
	for _, currency := range currencies {
		fmt.Printf("ID: %s, Name: %s, Min Size: %s\n", currency.ID, currency.Name, currency.MinSize)
	}

}
