package coinbase

import (
	"encoding/json"
	"fmt"
	"io/ioutil"
	"log"
	"net/http"
	"strconv"
	"strings"
	"time"
	"trading/modules/database"
)

// CandleData represents a single candle/bar data point
type CandleData struct {
	ProductID string  `json:"product_id"`
	Timestamp int64   `json:"timestamp"`
	Low       float64 `json:"low"`
	High      float64 `json:"high"`
	Open      float64 `json:"open"`
	Close     float64 `json:"close"`
	Volume    float64 `json:"volume"`
}

// CandleResponse represents the structure of the response from Coinbase candle API
type CandleResponse struct {
	// Coinbase returns candles as arrays: [timestamp, low, high, open, close, volume]
}

// FetchCandleData fetches candle data for a specific product and time range
func FetchCandleData(productID string, start, end time.Time, granularity int) ([]CandleData, error) {
	log.Printf("- Fetching candle data for %s from %s to %s with granularity %d seconds",
		productID, start.Format("2006-01-02"), end.Format("2006-01-02"), granularity)

	// Coinbase API endpoint for candles
	url := fmt.Sprintf("https://api.coinbase.com/v2/products/%s/candles", productID)

	// Build query parameters
	req, err := http.NewRequest("GET", url, nil)
	if err != nil {
		return nil, err
	}

	q := req.URL.Query()
	q.Add("start", start.Format("2006-01-02T15:04:05Z"))
	q.Add("end", end.Format("2006-01-02T15:04:05Z"))
	q.Add("granularity", strconv.Itoa(granularity))
	req.URL.RawQuery = q.Encode()

	// Make the request
	client := &http.Client{Timeout: 30 * time.Second}
	resp, err := client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("HTTP request failed: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		body, _ := ioutil.ReadAll(resp.Body)
		return nil, fmt.Errorf("API request failed with status %d: %s", resp.StatusCode, string(body))
	}

	// Read response body
	body, err := ioutil.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("failed to read response body: %w", err)
	}

	// Parse the JSON response - Coinbase returns an array of arrays
	var candleArrays [][]interface{}
	if err := json.Unmarshal(body, &candleArrays); err != nil {
		return nil, fmt.Errorf("failed to unmarshal candle data: %w", err)
	}

	// Convert to CandleData structs
	var candles []CandleData
	for _, candle := range candleArrays {
		if len(candle) < 6 {
			log.Printf("Invalid candle data format: %v", candle)
			continue
		}

		// Parse each field
		timestamp, ok := candle[0].(float64)
		if !ok {
			log.Printf("Invalid timestamp format: %v", candle[0])
			continue
		}

		low, err := strconv.ParseFloat(candle[1].(string), 64)
		if err != nil {
			log.Printf("Invalid low format: %v", candle[1])
			continue
		}

		high, err := strconv.ParseFloat(candle[2].(string), 64)
		if err != nil {
			log.Printf("Invalid high format: %v", candle[2])
			continue
		}

		open, err := strconv.ParseFloat(candle[3].(string), 64)
		if err != nil {
			log.Printf("Invalid open format: %v", candle[3])
			continue
		}

		close, err := strconv.ParseFloat(candle[4].(string), 64)
		if err != nil {
			log.Printf("Invalid close format: %v", candle[4])
			continue
		}

		volume, err := strconv.ParseFloat(candle[5].(string), 64)
		if err != nil {
			log.Printf("Invalid volume format: %v", candle[5])
			continue
		}

		candles = append(candles, CandleData{
			ProductID: productID,
			Timestamp: int64(timestamp),
			Low:       low,
			High:      high,
			Open:      open,
			Close:     close,
			Volume:    volume,
		})
	}

	log.Printf("Successfully fetched %d candle data points for %s", len(candles), productID)
	return candles, nil
}

// FetchAllHistoricalCandles fetches all available historical candle data for a product
func FetchAllHistoricalCandles(productID string, granularity int) ([]CandleData, error) {
	log.Printf("- Fetching ALL historical candle data for %s", productID)

	var allCandles []CandleData

	// Coinbase API typically limits to 300 candles per request
	// We'll fetch data in chunks starting from the most recent
	maxCandlesPerRequest := 300

	// Start from current time and go backwards
	endTime := time.Now()

	for {
		// Calculate start time (go back by maxCandlesPerRequest * granularity seconds)
		duration := time.Duration(maxCandlesPerRequest*granularity) * time.Second
		startTime := endTime.Add(-duration)

		// Fetch candle data for this time range
		candles, err := FetchCandleData(productID, startTime, endTime, granularity)
		if err != nil {
			log.Printf("Error fetching candle data: %v", err)
			break
		}

		if len(candles) == 0 {
			log.Printf("No more candle data available for %s", productID)
			break
		}

		// Add to our collection
		allCandles = append(allCandles, candles...)

		// Update endTime to be the timestamp of the earliest candle we just fetched
		earliestTimestamp := candles[0].Timestamp
		endTime = time.Unix(earliestTimestamp, 0)

		log.Printf("Fetched %d candles, going back to %s", len(candles), endTime.Format("2006-01-02 15:04:05"))

		// If we got fewer than max candles, we've probably reached the beginning
		if len(candles) < maxCandlesPerRequest {
			break
		}

		// Add a small delay to avoid rate limiting
		time.Sleep(100 * time.Millisecond)
	}

	log.Printf("Total historical candles fetched for %s: %d", productID, len(allCandles))
	return allCandles, nil
}

// StoreCandleData stores candle data in the database
func StoreCandleData(candles []CandleData) error {
	log.Printf("- Storing %d candle data points in database", len(candles))

	// Open database connection
	db, err := database.OpenConnection()
	if err != nil {
		return fmt.Errorf("failed to open database connection: %w", err)
	}
	defer database.CloseConnection(db)

	// Start transaction
	tx, err := db.Begin()
	if err != nil {
		return fmt.Errorf("failed to begin transaction: %w", err)
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

	// Prepare insert statement
	stmt, err := tx.Prepare(`
		INSERT INTO candle_data (product_id, timestamp, low, high, open, close, volume) 
		VALUES ($1, $2, $3, $4, $5, $6, $7)
		ON CONFLICT (product_id, timestamp) DO UPDATE SET
			low = EXCLUDED.low,
			high = EXCLUDED.high,
			open = EXCLUDED.open,
			close = EXCLUDED.close,
			volume = EXCLUDED.volume
	`)
	if err != nil {
		return fmt.Errorf("failed to prepare statement: %w", err)
	}
	defer stmt.Close()

	// Insert each candle
	insertedCount := 0
	skippedCount := 0

	for _, candle := range candles {
		_, err = stmt.Exec(
			candle.ProductID,
			candle.Timestamp,
			candle.Low,
			candle.High,
			candle.Open,
			candle.Close,
			candle.Volume,
		)

		if err != nil {
			log.Printf("Error inserting candle data for %s at %d: %v",
				candle.ProductID, candle.Timestamp, err)
			skippedCount++
			continue
		}
		insertedCount++
	}

	log.Printf("Successfully stored %d candle data points, skipped %d", insertedCount, skippedCount)
	return nil
}

// GetCandleData retrieves candle data from database with optional filtering
func GetCandleData(productID string, startTime, endTime *time.Time, limit int) ([]CandleData, error) {
	log.Printf("- Retrieving candle data for %s", productID)

	db, err := database.OpenConnection()
	if err != nil {
		return nil, fmt.Errorf("failed to open database connection: %w", err)
	}
	defer database.CloseConnection(db)

	// Build query
	query := "SELECT product_id, timestamp, low, high, open, close, volume FROM candle_data WHERE product_id = $1"
	args := []interface{}{productID}
	argIndex := 2

	if startTime != nil {
		query += fmt.Sprintf(" AND timestamp >= $%d", argIndex)
		args = append(args, startTime.Unix())
		argIndex++
	}

	if endTime != nil {
		query += fmt.Sprintf(" AND timestamp <= $%d", argIndex)
		args = append(args, endTime.Unix())
		argIndex++
	}

	query += " ORDER BY timestamp ASC"

	if limit > 0 {
		query += fmt.Sprintf(" LIMIT $%d", argIndex)
		args = append(args, limit)
	}

	rows, err := db.Query(query, args...)
	if err != nil {
		return nil, fmt.Errorf("failed to execute query: %w", err)
	}
	defer rows.Close()

	var candles []CandleData
	for rows.Next() {
		var candle CandleData
		err := rows.Scan(
			&candle.ProductID,
			&candle.Timestamp,
			&candle.Low,
			&candle.High,
			&candle.Open,
			&candle.Close,
			&candle.Volume,
		)
		if err != nil {
			return nil, fmt.Errorf("failed to scan row: %w", err)
		}
		candles = append(candles, candle)
	}

	if err = rows.Err(); err != nil {
		return nil, fmt.Errorf("row iteration error: %w", err)
	}

	log.Printf("Retrieved %d candle data points for %s", len(candles), productID)
	return candles, nil
}

// GetProductsInDatabase returns list of products that have candle data in the database
func GetProductsInDatabase() ([]string, error) {
	log.Println("- Getting products with candle data from database")

	db, err := database.OpenConnection()
	if err != nil {
		return nil, fmt.Errorf("failed to open database connection: %w", err)
	}
	defer database.CloseConnection(db)

	rows, err := db.Query("SELECT DISTINCT product_id FROM candle_data ORDER BY product_id")
	if err != nil {
		return nil, fmt.Errorf("failed to query products: %w", err)
	}
	defer rows.Close()

	var products []string
	for rows.Next() {
		var product string
		if err := rows.Scan(&product); err != nil {
			return nil, fmt.Errorf("failed to scan product: %w", err)
		}
		products = append(products, product)
	}

	if err = rows.Err(); err != nil {
		return nil, fmt.Errorf("row iteration error: %w", err)
	}

	log.Printf("Found %d products with candle data in database", len(products))
	return products, nil
}

// FetchAndStoreHistoricalCandles fetches and stores all historical candle data for a product
func FetchAndStoreHistoricalCandles(productID string, granularity int) error {
	log.Printf("- Fetching and storing historical candles for %s", productID)

	// Fetch all historical candles
	candles, err := FetchAllHistoricalCandles(productID, granularity)
	if err != nil {
		return fmt.Errorf("failed to fetch historical candles: %w", err)
	}

	if len(candles) == 0 {
		log.Printf("No candle data found for %s", productID)
		return nil
	}

	// Store in database
	err = StoreCandleData(candles)
	if err != nil {
		return fmt.Errorf("failed to store candle data: %w", err)
	}

	log.Printf("Successfully fetched and stored %d candle data points for %s", len(candles), productID)
	return nil
}

// GetAvailableProducts fetches all available trading products from Coinbase
func GetAvailableProducts() ([]string, error) {
	log.Println("- Fetching available products from Coinbase")

	// Try the Advanced Trade API endpoint first
	url := "https://api.coinbase.com/v2/brokerage/products"

	req, err := http.NewRequest("GET", url, nil)
	if err != nil {
		return nil, fmt.Errorf("failed to create request: %w", err)
	}

	// Set user agent to avoid 403/404 errors
	req.Header.Set("User-Agent", "crypto-trading-bot/1.0")
	req.Header.Set("Accept", "application/json")

	client := &http.Client{Timeout: 30 * time.Second}
	resp, err := client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("failed to fetch products: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		// Fallback to the old API endpoint
		log.Printf("Advanced Trade API failed with status %d, trying legacy API...", resp.StatusCode)
		return getLegacyProducts()
	}

	body, err := ioutil.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("failed to read response: %w", err)
	}

	var response struct {
		Products []struct {
			ProductID                 string  `json:"product_id"`
			Price                     float64 `json:"price"`
			PricePercentageChange24h  float64 `json:"price_percentage_change_24h"`
			Volume24h                 float64 `json:"volume_24h"`
			VolumePercentageChange24h float64 `json:"volume_percentage_change_24h"`
			BaseIncrement             string  `json:"base_increment"`
			QuoteIncrement            string  `json:"quote_increment"`
			MinMarketSize             float64 `json:"min_market_size"`
			MaxMarketSize             float64 `json:"max_market_size"`
			Status                    string  `json:"status"`
			QuoteCurrencyID           string  `json:"quote_currency_id"`
			BaseCurrencyID            string  `json:"base_currency_id"`
		} `json:"products"`
		NumProducts int `json:"num_products"`
	}

	if err := json.Unmarshal(body, &response); err != nil {
		return nil, fmt.Errorf("failed to unmarshal products: %w", err)
	}

	var products []string
	for _, product := range response.Products {
		// Only include active products
		if product.Status == "online" {
			products = append(products, product.ProductID)
		}
	}

	log.Printf("Found %d available trading products", len(products))
	return products, nil
}

// getLegacyProducts fetches products from the legacy Coinbase API
func getLegacyProducts() ([]string, error) {
	log.Println("- Trying legacy Coinbase API")

	url := "https://api.coinbase.com/v2/products"

	req, err := http.NewRequest("GET", url, nil)
	if err != nil {
		return nil, fmt.Errorf("failed to create request: %w", err)
	}

	req.Header.Set("User-Agent", "crypto-trading-bot/1.0")
	req.Header.Set("Accept", "application/json")

	client := &http.Client{Timeout: 30 * time.Second}
	resp, err := client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("failed to fetch products: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		body, _ := ioutil.ReadAll(resp.Body)
		log.Printf("Legacy API also failed with status %d: %s", resp.StatusCode, string(body))
		// Return fallback products for testing
		log.Println("- Using fallback product list for testing")
		return getFallbackProducts(), nil
	}

	body, err := ioutil.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("failed to read response: %w", err)
	}

	var response struct {
		Data []struct {
			ID             string `json:"id"`
			BaseCurrency   string `json:"base_currency"`
			QuoteCurrency  string `json:"quote_currency"`
			BaseMinSize    string `json:"base_min_size"`
			BaseMaxSize    string `json:"base_max_size"`
			QuoteIncrement string `json:"quote_increment"`
			DisplayName    string `json:"display_name"`
			Status         string `json:"status"`
			MinMarketSize  string `json:"min_market_size"`
			MaxMarketSize  string `json:"max_market_size"`
		} `json:"data"`
	}

	if err := json.Unmarshal(body, &response); err != nil {
		return nil, fmt.Errorf("failed to unmarshal products: %w", err)
	}

	var products []string
	for _, product := range response.Data {
		// Only include active crypto pairs (BTC-USD, ETH-USD, etc.)
		if strings.Contains(product.ID, "-") && product.Status == "online" {
			products = append(products, product.ID)
		}
	}

	log.Printf("Found %d available trading products from legacy API", len(products))
	return products, nil
}

// getFallbackProducts returns a list of common trading pairs for testing
func getFallbackProducts() []string {
	return []string{
		"BTC-USD",
		"ETH-USD",
		"BTC-EUR",
		"ETH-EUR",
		"BTC-GBP",
		"ETH-GBP",
		"BTC-USDT",
		"ETH-USDT",
		"LTC-USD",
		"BCH-USD",
		"LINK-USD",
		"ADA-USD",
		"DOT-USD",
		"SOL-USD",
		"MATIC-USD",
		"UNI-USD",
		"AAVE-USD",
		"COMP-USD",
		"YFI-USD",
		"SNX-USD",
	}
}
