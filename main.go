// main.go
package main

import (
	"fmt"
	"log"
	"net/http"
	"os"

	"trading/modules/coinbase"
	"trading/modules/utility"

	"github.com/gin-gonic/gin"
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


	// Fetch currencies from Coinbase API
	currencies, err := coinbase.FetchCurrencies()
	if err != nil {
		log.Fatalf("Error fetching currencies: %s", err)
	}

	// Print the list of currencies
	fmt.Println("List of Currencies:")
	for _, currency := range currencies {
		fmt.Printf("ID: %s, Name: %s, MinSize: %s\n", currency.ID, currency.Name, currency.MinSize)
	}
	err = coinbase.StoreCurrenciesInDatabase(currencies)
	if err != nil {
		log.Fatalf("Error storing currencies: %s", err)
	}



    // Initialize GIN router
    r := gin.Default()

    // Define endpoints
    r.GET("/currencies", func(c *gin.Context) {
        // Return the list of currencies as JSON
        c.JSON(http.StatusOK, currencies)
    })

    r.GET("/exchange-rates", func(c *gin.Context) {
        // Fetch exchange rates from Coinbase API for BTC
        baseCurrency := "BTC"
        rates, err := coinbase.FetchExchangeRates(baseCurrency)
        if err != nil {
            c.JSON(http.StatusInternalServerError, gin.H{"error": "Error fetching exchange rates"})
            return
        }

        // Return the exchange rates as JSON
        c.JSON(http.StatusOK, rates)
    })

    // Run the web server
	log.Printf("Starting web werver...")
    err = r.Run(":8080")
    if err != nil {
        log.Fatalf("Error starting web server: %s", err)
    }
}
