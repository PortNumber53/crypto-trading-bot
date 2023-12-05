package coinbase

import (
	"database/sql"
	"encoding/json"
	"io/ioutil"
	"log"
	"net/http"
	"trading/modules/database"
)

// Currency represents an individual currency object
type Currency struct {
	ID       string `json:"id"`
	Name     string `json:"name"`
	MinSize  string `json:"min_size"`
}

// CurrenciesResponse represents the structure of the response from Coinbase API
type CurrenciesResponse struct {
	Data []Currency `json:"data"`
}

// FetchCurrencies fetches currencies from the Coinbase API
func FetchCurrencies() ([]Currency, error) {
	log.Println("- Fetching Currencies from Coinbase")
	url := "https://api.coinbase.com/v2/currencies"

	// Make a GET request to the Coinbase API
	resp, err := http.Get(url)
	if err != nil {
		log.Fatalf("Coinbase Error: %s", err)
		return nil, err
	}
	defer resp.Body.Close()

	// Read the response body
	body, err := ioutil.ReadAll(resp.Body)
	if err != nil {
		return nil, err
	}

	// Unmarshal the JSON response
	var currenciesResp CurrenciesResponse
	if err := json.Unmarshal(body, &currenciesResp); err != nil {
		log.Fatalf("Unmarshall Error: %s", err)
		return nil, err
	}

	return currenciesResp.Data, nil
}


func StoreCurrenciesInDatabase(currencies []Currency) error {
    log.Println("- Storing Currencies in Database")

    // Open a connection to the database
    db, err := database.OpenConnection()
    if err != nil {
        return err
    }
    defer database.CloseConnection(db)

    // Start a transaction
    tx, err := db.Begin()
    if err != nil {
        return err
    }
    defer func() {
        if p := recover(); p != nil {
            tx.Rollback()
            panic(p)
        } else if err != nil {
            tx.Rollback()
        } else {
            err = tx.Commit()
        }
    }()

    // Iterate over currencies and insert or update them in the database
    for _, currency := range currencies {
        // Check if the currency already exists in the database
        exists, err := currencyExists(tx, currency.ID)
        if err != nil {
            log.Printf("Error checking if currency %s exists: %s", currency.ID, err)
            // Rollback the transaction and return the error
            return err
        }

        if exists {
            // Currency already exists, you can choose to update or skip
            log.Printf("Currency %s already exists in the database, skipping insertion", currency.ID)
            continue
        }

        // Currency doesn't exist, insert it into the database
        _, err = tx.Exec("INSERT INTO currencies (id, name, min_size) VALUES ($1, $2, $3)",
            currency.ID, currency.Name, currency.MinSize)
        if err != nil {
            log.Printf("Error inserting currency %s into database: %s", currency.ID, err)
            // Rollback the transaction and return the error
            return err
        }
    }

    return nil
}



// currencyExists checks if a currency with the given ID already exists in the transaction
func currencyExists(tx *sql.Tx, currencyID string) (bool, error) {
	var count int
	err := tx.QueryRow("SELECT COUNT(*) FROM currencies WHERE id = $1", currencyID).Scan(&count)
	if err != nil {
		return false, err
	}
	return count > 0, nil
}