package coinbase

import (
	"database/sql"
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

// Product represents a Coinbase trading product
type Product struct {
	ID                        string    `json:"product_id" db:"product_id"`
	BaseCurrency              string    `json:"base_currency_id" db:"base_currency_id"`
	QuoteCurrency             string    `json:"quote_currency_id" db:"quote_currency_id"`
	Price                     float64   `json:"price,string" db:"price"`
	PricePercentageChange24h  float64   `json:"price_percentage_change_24h,string" db:"price_percentage_change_24h"`
	Volume24h                 float64   `json:"volume_24h,string" db:"volume_24h"`
	VolumePercentageChange24h float64   `json:"volume_percentage_change_24h,string" db:"volume_percentage_change_24h"`
	BaseIncrement             string    `json:"base_increment" db:"base_increment"`
	QuoteIncrement            string    `json:"quote_increment" db:"quote_increment"`
	MinMarketSize             float64   `json:"min_market_size,string" db:"min_market_size"`
	MaxMarketSize             float64   `json:"max_market_size,string" db:"max_market_size"`
	Status                    string    `json:"status" db:"status"`
	DisplayName               string    `json:"display_name" db:"display_name"`
	CreatedAt                 time.Time `json:"created_at" db:"created_at"`
	UpdatedAt                 time.Time `json:"updated_at" db:"updated_at"`
}

// FetchProductsFromAPI fetches all products from Coinbase Advanced Trade API
func FetchProductsFromAPI() ([]Product, error) {
	log.Println("- Fetching products from Coinbase Advanced Trade API")

	requestPath := "/api/v3/brokerage/products"
	apiURL := "https://api.coinbase.com" + requestPath

	// Generate JWT for authentication
	jwtToken, err := generateJWT("GET", requestPath)
	if err != nil {
		return nil, fmt.Errorf("failed to generate JWT: %w", err)
	}

	var allProducts []Product
	cursor := ""

	for {
		reqURL := apiURL
		if cursor != "" {
			reqURL += "?cursor=" + cursor
		}

		req, err := http.NewRequest("GET", reqURL, nil)
		if err != nil {
			return nil, fmt.Errorf("failed to create request: %w", err)
		}

		req.Header.Set("Authorization", "Bearer "+jwtToken)
		req.Header.Set("Content-Type", "application/json")

		client := &http.Client{Timeout: 30 * time.Second}
		resp, err := client.Do(req)
		if err != nil {
			return nil, fmt.Errorf("failed to fetch products: %w", err)
		}
		defer resp.Body.Close()

		body, err := ioutil.ReadAll(resp.Body)
		if err != nil {
			return nil, fmt.Errorf("failed to read response: %w", err)
		}

		if resp.StatusCode != http.StatusOK {
			return nil, fmt.Errorf("API request failed with status %d: %s", resp.StatusCode, string(body))
		}

		var response struct {
			Products    []json.RawMessage `json:"products"`
			NumProducts int               `json:"num_products"`
			Pagination  struct {
				NextCursor string `json:"next_cursor"`
				HasNext    bool   `json:"has_next"`
			} `json:"pagination"`
		}

		if err := json.Unmarshal(body, &response); err != nil {
			return nil, fmt.Errorf("failed to unmarshal response: %w", err)
		}

		// Parse each product individually to handle string-encoded numbers
		for _, raw := range response.Products {
			product, err := parseProductJSON(raw)
			if err != nil {
				log.Printf("Warning: failed to parse product: %v", err)
				continue
			}
			allProducts = append(allProducts, product)
		}

		log.Printf("Fetched %d products (total so far: %d)", len(response.Products), len(allProducts))

		if !response.Pagination.HasNext || response.Pagination.NextCursor == "" {
			break
		}
		cursor = response.Pagination.NextCursor

		// Regenerate JWT for next page (in case of expiry)
		jwtToken, err = generateJWT("GET", requestPath)
		if err != nil {
			return nil, fmt.Errorf("failed to generate JWT for next page: %w", err)
		}
	}

	log.Printf("Fetched %d total products from Advanced Trade API", len(allProducts))
	return allProducts, nil
}

// parseProductJSON parses a single product from raw JSON, handling string-encoded numbers
func parseProductJSON(raw json.RawMessage) (Product, error) {
	var p struct {
		ProductID                 string `json:"product_id"`
		BaseCurrencyID            string `json:"base_currency_id"`
		QuoteCurrencyID           string `json:"quote_currency_id"`
		Price                     string `json:"price"`
		PricePercentageChange24h  string `json:"price_percentage_change_24h"`
		Volume24h                 string `json:"volume_24h"`
		VolumePercentageChange24h string `json:"volume_percentage_change_24h"`
		BaseIncrement             string `json:"base_increment"`
		QuoteIncrement            string `json:"quote_increment"`
		BaseMinSize               string `json:"base_min_size"`
		BaseMaxSize               string `json:"base_max_size"`
		Status                    string `json:"status"`
		DisplayName               string `json:"display_name"`
	}

	if err := json.Unmarshal(raw, &p); err != nil {
		return Product{}, err
	}

	now := time.Now()
	product := Product{
		ID:             p.ProductID,
		BaseCurrency:   p.BaseCurrencyID,
		QuoteCurrency:  p.QuoteCurrencyID,
		BaseIncrement:  p.BaseIncrement,
		QuoteIncrement: p.QuoteIncrement,
		Status:         p.Status,
		DisplayName:    p.DisplayName,
		CreatedAt:      now,
		UpdatedAt:      now,
	}

	// Parse numeric fields safely (they come as strings from the API)
	product.Price, _ = parseFloatSafe(strings.TrimSuffix(p.Price, "%"))
	product.PricePercentageChange24h, _ = parseFloatSafe(strings.TrimSuffix(p.PricePercentageChange24h, "%"))
	product.Volume24h, _ = parseFloatSafe(p.Volume24h)
	product.VolumePercentageChange24h, _ = parseFloatSafe(strings.TrimSuffix(p.VolumePercentageChange24h, "%"))
	product.MinMarketSize, _ = parseFloatSafe(p.BaseMinSize)
	product.MaxMarketSize, _ = parseFloatSafe(p.BaseMaxSize)

	return product, nil
}

// parseFloatSafe safely parses string to float64
func parseFloatSafe(s string) (float64, error) {
	if s == "" {
		return 0, nil
	}
	return strconv.ParseFloat(s, 64)
}

// StoreProductsInDatabase stores products in the database
func StoreProductsInDatabase(products []Product) (err error) {
	log.Printf("- Storing %d products in database", len(products))

	db, err := database.OpenConnection()
	if err != nil {
		return fmt.Errorf("failed to open database connection: %w", err)
	}
	defer database.CloseConnection(db)

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

	// Prepare upsert statement
	stmt, err := tx.Prepare(`
		INSERT INTO products (
			product_id, base_currency, quote_currency, price, price_percentage_change_24h,
			volume_24h, volume_percentage_change_24h, base_increment, quote_increment,
			min_market_size, max_market_size, status, display_name, created_at, updated_at
		) VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11, $12, $13, $14, $15)
		ON CONFLICT (product_id) DO UPDATE SET
			base_currency = EXCLUDED.base_currency,
			quote_currency = EXCLUDED.quote_currency,
			price = EXCLUDED.price,
			price_percentage_change_24h = EXCLUDED.price_percentage_change_24h,
			volume_24h = EXCLUDED.volume_24h,
			volume_percentage_change_24h = EXCLUDED.volume_percentage_change_24h,
			base_increment = EXCLUDED.base_increment,
			quote_increment = EXCLUDED.quote_increment,
			min_market_size = EXCLUDED.min_market_size,
			max_market_size = EXCLUDED.max_market_size,
			status = EXCLUDED.status,
			display_name = EXCLUDED.display_name,
			updated_at = EXCLUDED.updated_at
	`)
	if err != nil {
		return fmt.Errorf("failed to prepare statement: %w", err)
	}
	defer stmt.Close()

	successCount := 0

	for _, product := range products {
		_, err = stmt.Exec(
			product.ID,
			product.BaseCurrency,
			product.QuoteCurrency,
			product.Price,
			product.PricePercentageChange24h,
			product.Volume24h,
			product.VolumePercentageChange24h,
			product.BaseIncrement,
			product.QuoteIncrement,
			product.MinMarketSize,
			product.MaxMarketSize,
			product.Status,
			product.DisplayName,
			product.CreatedAt,
			product.UpdatedAt,
		)

		if err != nil {
			log.Printf("Error storing product %s: %v", product.ID, err)
			continue
		}

		successCount++
	}

	log.Printf("Successfully stored %d products", successCount)
	return
}

// GetProductsFromDatabase retrieves products from database with optional filtering
func GetProductsFromDatabase(baseCurrency, quoteCurrency, status string, limit int) ([]Product, error) {
	log.Printf("- Retrieving products from database (base: %s, quote: %s, status: %s)", baseCurrency, quoteCurrency, status)

	db, err := database.OpenConnection()
	if err != nil {
		return nil, fmt.Errorf("failed to open database connection: %w", err)
	}
	defer database.CloseConnection(db)

	query := `SELECT product_id, base_currency, quote_currency, price, price_percentage_change_24h,
			volume_24h, volume_percentage_change_24h, base_increment, quote_increment,
			min_market_size, max_market_size, status, display_name, created_at, updated_at
			FROM products WHERE 1=1`
	args := []interface{}{}
	argIndex := 1

	if baseCurrency != "" {
		query += fmt.Sprintf(" AND base_currency = $%d", argIndex)
		args = append(args, baseCurrency)
		argIndex++
	}

	if quoteCurrency != "" {
		query += fmt.Sprintf(" AND quote_currency = $%d", argIndex)
		args = append(args, quoteCurrency)
		argIndex++
	}

	if status != "" {
		query += fmt.Sprintf(" AND status = $%d", argIndex)
		args = append(args, status)
		argIndex++
	}

	query += " ORDER BY product_id"

	if limit > 0 {
		query += fmt.Sprintf(" LIMIT $%d", argIndex)
		args = append(args, limit)
	}

	rows, err := db.Query(query, args...)
	if err != nil {
		return nil, fmt.Errorf("failed to execute query: %w", err)
	}
	defer rows.Close()

	var products []Product
	for rows.Next() {
		var product Product
		err := rows.Scan(
			&product.ID,
			&product.BaseCurrency,
			&product.QuoteCurrency,
			&product.Price,
			&product.PricePercentageChange24h,
			&product.Volume24h,
			&product.VolumePercentageChange24h,
			&product.BaseIncrement,
			&product.QuoteIncrement,
			&product.MinMarketSize,
			&product.MaxMarketSize,
			&product.Status,
			&product.DisplayName,
			&product.CreatedAt,
			&product.UpdatedAt,
		)
		if err != nil {
			return nil, fmt.Errorf("failed to scan product: %w", err)
		}
		products = append(products, product)
	}

	if err = rows.Err(); err != nil {
		return nil, fmt.Errorf("row iteration error: %w", err)
	}

	log.Printf("Retrieved %d products from database", len(products))
	return products, nil
}

// GetProductByID retrieves a specific product by ID
func GetProductByID(productID string) (*Product, error) {
	log.Printf("- Retrieving product %s from database", productID)

	db, err := database.OpenConnection()
	if err != nil {
		return nil, fmt.Errorf("failed to open database connection: %w", err)
	}
	defer database.CloseConnection(db)

	var product Product
	err = db.QueryRow(`
		SELECT product_id, base_currency, quote_currency, price, price_percentage_change_24h,
			volume_24h, volume_percentage_change_24h, base_increment, quote_increment,
			min_market_size, max_market_size, status, display_name, created_at, updated_at
		FROM products WHERE product_id = $1
	`, productID).Scan(
		&product.ID,
		&product.BaseCurrency,
		&product.QuoteCurrency,
		&product.Price,
		&product.PricePercentageChange24h,
		&product.Volume24h,
		&product.VolumePercentageChange24h,
		&product.BaseIncrement,
		&product.QuoteIncrement,
		&product.MinMarketSize,
		&product.MaxMarketSize,
		&product.Status,
		&product.DisplayName,
		&product.CreatedAt,
		&product.UpdatedAt,
	)

	if err != nil {
		if err == sql.ErrNoRows {
			return nil, fmt.Errorf("product %s not found", productID)
		}
		return nil, fmt.Errorf("failed to retrieve product: %w", err)
	}

	return &product, nil
}

// GetProductsCount returns the total number of products in database
func GetProductsCount() (int, error) {
	db, err := database.OpenConnection()
	if err != nil {
		return 0, fmt.Errorf("failed to open database connection: %w", err)
	}
	defer database.CloseConnection(db)

	var count int
	err = db.QueryRow("SELECT COUNT(*) FROM products").Scan(&count)
	if err != nil {
		return 0, fmt.Errorf("failed to get products count: %w", err)
	}

	return count, nil
}

// FetchAndStoreProducts fetches products from API and stores them in database
func FetchAndStoreProducts() error {
	log.Println("- Fetching and storing products from Coinbase")

	// Fetch from API
	products, err := FetchProductsFromAPI()
	if err != nil {
		return fmt.Errorf("failed to fetch products from API: %w", err)
	}

	if len(products) == 0 {
		log.Println("No products found to store")
		return nil
	}

	// Store in database
	err = StoreProductsInDatabase(products)
	if err != nil {
		return fmt.Errorf("failed to store products in database: %w", err)
	}

	log.Printf("Successfully fetched and stored %d products", len(products))
	return nil
}
