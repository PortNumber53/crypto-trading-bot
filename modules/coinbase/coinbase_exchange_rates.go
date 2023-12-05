// modules/coinbase/coinbase_exchange_rates.go
package coinbase

import (
	"encoding/json"
	"fmt"
	"io/ioutil"
	"log"
	"net/http"
)

// ExchangeRatesResponse represents the structure of the response from Coinbase API for exchange rates
type ExchangeRatesResponse struct {
	Data struct {
		Currency string            `json:"currency"`
		Rates    map[string]string `json:"rates"`
	} `json:"data"`
}

// FetchExchangeRates fetches exchange rates from the Coinbase API
func FetchExchangeRates(baseCurrency string) (map[string]string, error) {
	log.Printf("- Fetching Exchange Rates from Coinbase for %s\n", baseCurrency)
	url := fmt.Sprintf("https://api.coinbase.com/v2/exchange-rates?currency=%s", baseCurrency)

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
	var ratesResp ExchangeRatesResponse
	if err := json.Unmarshal(body, &ratesResp); err != nil {
		log.Fatalf("Unmarshall Error: %s", err)
		return nil, err
	}

	return ratesResp.Data.Rates, nil
}
