// main.go
package main

import (
	"bufio"
	"database/sql"
	"flag"
	"fmt"
	"log"
	"net/http"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	"trading/modules/coinbase"
	"trading/modules/utility"

	"github.com/gin-gonic/gin"
	_ "github.com/lib/pq"
)

// parseCandleQueryParams parses query parameters for candle data requests
func parseCandleQueryParams(c *gin.Context) (*time.Time, *time.Time, int) {
	var startTime, endTime *time.Time
	limit := 0

	// Parse start time
	if startStr := c.Query("start"); startStr != "" {
		if t, err := time.Parse("2006-01-02T15:04:05Z", startStr); err == nil {
			startTime = &t
		} else if t, err := time.Parse("2006-01-02", startStr); err == nil {
			startTime = &t
		}
	}

	// Parse end time
	if endStr := c.Query("end"); endStr != "" {
		if t, err := time.Parse("2006-01-02T15:04:05Z", endStr); err == nil {
			endTime = &t
		} else if t, err := time.Parse("2006-01-02", endStr); err == nil {
			endTime = &t
		}
	}

	// Parse limit
	if limitStr := c.Query("limit"); limitStr != "" {
		if l, err := strconv.Atoi(limitStr); err == nil && l > 0 {
			limit = l
		}
	}

	return startTime, endTime, limit
}

// loadEnvFile loads environment variables from .env file
func loadEnvFile() error {
	envFile := filepath.Join(".", ".env")

	// Check if .env file exists
	if _, err := os.Stat(envFile); os.IsNotExist(err) {
		// .env file doesn't exist, skip loading
		return nil
	}

	file, err := os.Open(envFile)
	if err != nil {
		return fmt.Errorf("error opening .env file: %w", err)
	}
	defer file.Close()

	scanner := bufio.NewScanner(file)
	lineNum := 0

	for scanner.Scan() {
		lineNum++
		line := strings.TrimSpace(scanner.Text())

		// Skip empty lines and comments
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}

		// Parse key=value format
		parts := strings.SplitN(line, "=", 2)
		if len(parts) != 2 {
			log.Printf("Warning: Invalid line %d in .env file: %s", lineNum, line)
			continue
		}

		key := strings.TrimSpace(parts[0])
		value := strings.TrimSpace(parts[1])

		// Remove surrounding quotes if present
		if len(value) >= 2 && ((value[0] == '"' && value[len(value)-1] == '"') ||
			(value[0] == '\'' && value[len(value)-1] == '\'')) {
			value = value[1 : len(value)-1]
		}

		// Set environment variable (don't override if already set)
		if os.Getenv(key) == "" {
			os.Setenv(key, value)
		}
	}

	if err := scanner.Err(); err != nil {
		return fmt.Errorf("error reading .env file: %w", err)
	}

	log.Printf("Loaded environment variables from .env file")
	return nil
}

// printHelp displays usage information
func printHelp() {
	fmt.Println(`Coinbase Historical Candle Data Platform

USAGE:
    go run main.go <command> [subcommand] [options]
    go run main.go                    Start the HTTP server (default)

COMMANDS:
    help, --help, -h                  Show this help message
    coinbase <subcommand>             Coinbase data operations
    migrate <subcommand>              Database migration commands

ENVIRONMENT VARIABLES:
    DATABASE_URL                      PostgreSQL connection URL (required)
                                      e.g. postgres://user:pass@host:5432/db?sslmode=disable

    COINBASE_CLOUD_API_KEY_NAME       Coinbase Cloud API key name (required for v3 API)
    COINBASE_CLOUD_API_SECRET         Coinbase Cloud API secret (required for v3 API)

    COINBASE_API_KEY                  Coinbase legacy API key (not currently used)
    COINBASE_API_SECRET               Coinbase legacy API secret (not currently used)
    COINBASE_API_VERSION              Coinbase API version (not currently used)
    WEBHOOK_COINBASE                  Coinbase webhook secret (not currently used)

EXAMPLES:
    # Start the server
    export DATABASE_URL="postgres://user:pass@localhost:5432/cryptodb?sslmode=disable"
    go run main.go

    # Database migrations
    go run main.go migrate up
    go run main.go migrate down 1

    # Fetch data from Coinbase
    go run main.go coinbase products
    go run main.go coinbase fetch products
    go run main.go coinbase fetch candles -product BTC-USD -start-date 2024-01-01 -end-date 2024-01-31

    # Show command-specific help
    go run main.go coinbase help
    go run main.go migrate help

API ENDPOINTS:
    GET  /currencies                  List all available currencies
    GET  /exchange-rates              Get exchange rates for BTC
    GET  /products                    List available trading products

    GET  /candles                     List products with candle data
    GET  /candles/:product            Get candle data for a product
    POST /candles/:product/fetch      Fetch & store historical candle data

    GET  /candles/:product/paginated  Get paginated candle data
    GET  /candles/:product/latest     Get latest candle data
    GET  /candles/:product/info       Get data range information

SERVER:
    The server runs on port 8080 by default.
    Access the API at http://localhost:8080

DATABASE:
    The application automatically runs database migrations on startup.
    PostgreSQL is required; tables are created via migrations in db/migrations/.`)
}

// handleCommandLine processes command line arguments
func handleCommandLine() {
	if len(os.Args) < 2 {
		printHelp()
		return
	}

	command := os.Args[1]

	switch command {
	case "help", "--help", "-h":
		printHelp()
	case "coinbase":
		handleCoinbaseCommand()
	case "migrate":
		handleMigrateCommand()
	default:
		fmt.Printf("Unknown command: %s\n\n", command)
		// Suggest correction for common mistakes
		if command == "fetch" {
			fmt.Println("Did you mean: go run main.go coinbase fetch ...")
			fmt.Println()
		}
		printHelp()
	}
}

// handleMigrateCommand processes migration-related commands
func handleMigrateCommand() {
	if len(os.Args) < 3 {
		fmt.Println("Usage: go run main.go migrate <subcommand> [options]")
		fmt.Println("Subcommands:")
		fmt.Println("  up           Run all pending migrations")
		fmt.Println("  down <steps> Rollback migrations by specified steps")
		fmt.Println("  version      Show current migration version")
		fmt.Println("  create <name> Create a new migration file")
		fmt.Println("  help         Show migration help")
		return
	}

	subcommand := os.Args[2]

	// Check DATABASE_URL for commands that need database access
	if subcommand != "create" && subcommand != "help" {
		if os.Getenv("DATABASE_URL") == "" {
			fmt.Println("Error: DATABASE_URL environment variable not set")
			return
		}
	}

	switch subcommand {
	case "up":
		if err := utility.RunMigrationsUp(os.Getenv("DATABASE_URL")); err != nil {
			fmt.Printf("Error running migrations up: %v\n", err)
			return
		}
		fmt.Println("Migrations completed successfully")

	case "down":
		if len(os.Args) < 4 {
			fmt.Println("Usage: go run main.go migrate down <steps>")
			fmt.Println("Example: go run main.go migrate down 1")
			fmt.Println("         go run main.go migrate down -1  (rollback all)")
			return
		}

		steps, err := strconv.Atoi(os.Args[3])
		if err != nil {
			fmt.Printf("Error: Invalid steps value '%s'. Must be a number.\n", os.Args[3])
			return
		}

		if err := utility.RunMigrationsDown(os.Getenv("DATABASE_URL"), steps); err != nil {
			fmt.Printf("Error rolling back migrations: %v\n", err)
			return
		}
		fmt.Println("Migration rollback completed successfully")

	case "version":
		version, dirty, err := utility.GetMigrationVersion(os.Getenv("DATABASE_URL"))
		if err != nil {
			fmt.Printf("Error getting migration version: %v\n", err)
			return
		}

		fmt.Printf("Current migration version: %d", version)
		if dirty {
			fmt.Printf(" (dirty - migration state inconsistent)")
		}
		fmt.Println()

	case "create":
		if len(os.Args) < 4 {
			fmt.Println("Usage: go run main.go migrate create <migration_name>")
			fmt.Println("Example: go run main.go migrate create create_users_table")
			return
		}

		name := os.Args[3]
		if err := utility.CreateMigration(name); err != nil {
			fmt.Printf("Error creating migration: %v\n", err)
			return
		}

	case "help":
		printMigrateHelp()

	default:
		fmt.Printf("Unknown migrate subcommand: %s\n", subcommand)
		printMigrateHelp()
	}
}

// printMigrateHelp shows migration-specific help
func printMigrateHelp() {
	fmt.Println(`MIGRATION COMMANDS:

USAGE:
    go run main.go migrate <subcommand> [options]

SUBCOMMANDS:
    up              Run all pending migrations
    down <steps>    Rollback migrations by specified steps (use -1 for all)
    version         Show current migration version
    create <name>   Create a new migration file (template only)
    help            Show this help message

EXAMPLES:
    # Run all pending migrations
    go run main.go migrate up

    # Rollback last migration
    go run main.go migrate down 1

    # Rollback all migrations
    go run main.go migrate down -1

    # Show current migration version
    go run main.go migrate version

    # Create a new migration file
    go run main.go migrate create add_user_preferences_table

ENVIRONMENT VARIABLES:
    DATABASE_URL          PostgreSQL database connection URL (required for most commands)

NOTE:
    The 'create' command only shows instructions for creating migration files.
    You need to manually create the .up.sql and .down.sql files in db/migrations/.`)
}

// handleCoinbaseCommand processes Coinbase-related commands
func handleCoinbaseCommand() {
	if len(os.Args) < 3 {
		fmt.Println("Usage: go run main.go coinbase <subcommand> [options]")
		fmt.Println("Subcommands:")
		fmt.Println("  fetch candles - Fetch historical candle data")
		fmt.Println("  products     - List available products")
		fmt.Println("  help         - Show Coinbase command help")
		return
	}

	subcommand := os.Args[2]

	switch subcommand {
	case "fetch":
		if len(os.Args) >= 4 && os.Args[3] == "candles" {
			handleFetchCandles()
		} else if len(os.Args) >= 4 && os.Args[3] == "products" {
			handleFetchProducts()
		} else {
			fmt.Println("Usage: go run main.go coinbase fetch <type>")
			fmt.Println("Types: candles, products")
		}
	case "products":
		handleListProducts()
	case "help":
		printCoinbaseHelp()
	default:
		fmt.Printf("Unknown Coinbase subcommand: %s\n", subcommand)
		printCoinbaseHelp()
	}
}

// handleFetchCandles handles the fetch candles command
func handleFetchCandles() {
	// Set up flags for fetch candles command
	var (
		productID   string
		startDate   string
		endDate     string
		granularity string
		help        bool
	)

	// Create a new flag set for this subcommand
	fetchCmd := flag.NewFlagSet("fetch candles", flag.ExitOnError)
	fetchCmd.StringVar(&productID, "product", "", "Trading product ID (e.g., BTC-USD)")
	fetchCmd.StringVar(&startDate, "start-date", "", "Start date in YYYY-MM-DD format")
	fetchCmd.StringVar(&endDate, "end-date", "", "End date in YYYY-MM-DD format")
	fetchCmd.StringVar(&granularity, "granularity", "ONE_HOUR", "Granularity: ONE_MINUTE, FIVE_MINUTE, FIFTEEN_MINUTE, ONE_HOUR, SIX_HOURS, ONE_DAY")
	fetchCmd.BoolVar(&help, "help", false, "Show help for fetch candles command")

	// Parse flags after the subcommand
	fetchCmd.Parse(os.Args[4:])

	if help {
		fmt.Println(`USAGE:
    go run main.go coinbase fetch candles [OPTIONS]

OPTIONS:
    -product string        Trading product ID (e.g., BTC-USD, ETH-USD) [required]
    -start-date string     Start date in YYYY-MM-DD format [required]
    -end-date string       End date in YYYY-MM-DD format [required]
    -granularity string    Time granularity (default: ONE_HOUR)
                           Available: ONE_MINUTE, FIVE_MINUTE, FIFTEEN_MINUTE, 
                                     ONE_HOUR, SIX_HOURS, ONE_DAY
    -help                  Show this help message

EXAMPLES:
    # Fetch BTC-USD candles for January 2024 with 1-hour granularity
    go run main.go coinbase fetch candles -product BTC-USD -start-date 2024-01-01 -end-date 2024-01-31 -granularity ONE_HOUR

    # Fetch ETH-USD candles for today with 15-minute granularity
    go run main.go coinbase fetch candles -product ETH-USD -start-date 2024-02-06 -end-date 2024-02-06 -granularity FIFTEEN_MINUTE`)
		return
	}

	// Validate required parameters
	if productID == "" {
		fmt.Println("Error: -product is required")
		fetchCmd.Usage()
		return
	}

	if startDate == "" {
		fmt.Println("Error: -start-date is required")
		fetchCmd.Usage()
		return
	}

	if endDate == "" {
		fmt.Println("Error: -end-date is required")
		fetchCmd.Usage()
		return
	}

	// Convert granularity string to seconds
	granularitySeconds, err := parseGranularity(granularity)
	if err != nil {
		fmt.Printf("Error: %v\n", err)
		return
	}

	// Parse dates
	start, err := time.Parse("2006-01-02", startDate)
	if err != nil {
		fmt.Printf("Error parsing start date: %v\n", err)
		return
	}

	end, err := time.Parse("2006-01-02", endDate)
	if err != nil {
		fmt.Printf("Error parsing end date: %v\n", err)
		return
	}

	// Validate date range
	if end.Before(start) {
		fmt.Println("Error: End date must be after or equal to start date")
		return
	}

	// Check DATABASE_URL
	dbURL := os.Getenv("DATABASE_URL")
	if dbURL == "" {
		fmt.Println("Error: DATABASE_URL environment variable not set")
		return
	}

	// Run migrations
	fmt.Println("Running database migrations...")
	if err := utility.DoMigrations(dbURL); err != nil {
		fmt.Printf("Error running migrations: %v\n", err)
		return
	}

	// Fetch candle data
	fmt.Printf("Fetching candle data for %s from %s to %s with %s granularity...\n",
		productID, startDate, endDate, granularity)

	candles, err := coinbase.FetchCandleData(productID, start, end, granularitySeconds)
	if err != nil {
		fmt.Printf("Error fetching candle data: %v\n", err)
		return
	}

	if len(candles) == 0 {
		fmt.Println("No candle data found for the specified parameters")
		return
	}

	// Store in database
	fmt.Printf("Storing %d candle data points in database...\n", len(candles))
	err = coinbase.StoreCandleData(candles)
	if err != nil {
		fmt.Printf("Error storing candle data: %v\n", err)
		return
	}

	fmt.Printf("Successfully fetched and stored %d candle data points for %s\n", len(candles), productID)

	// Show summary
	if len(candles) > 0 {
		firstCandle := candles[0]
		lastCandle := candles[len(candles)-1]
		fmt.Printf("Date range: %s to %s\n",
			time.Unix(firstCandle.Timestamp, 0).Format("2006-01-02 15:04:05"),
			time.Unix(lastCandle.Timestamp, 0).Format("2006-01-02 15:04:05"))
		fmt.Printf("Price range: $%.2f - $%.2f\n", firstCandle.Low, lastCandle.High)
	}
}

// handleFetchProducts handles the fetch products command
func handleFetchProducts() {
	fmt.Println("Fetching products from Coinbase and storing in database...")

	// Check DATABASE_URL
	dbURL := os.Getenv("DATABASE_URL")
	if dbURL == "" {
		fmt.Println("Error: DATABASE_URL environment variable not set")
		return
	}

	// Debug: Show the DATABASE_URL (masked for security)
	if len(dbURL) > 50 {
		fmt.Printf("Debug: DATABASE_URL loaded (length: %d, starts with: %s...)\n", len(dbURL), dbURL[:20])
	} else {
		fmt.Printf("Debug: DATABASE_URL loaded: %s\n", dbURL)
	}

	// Run migrations to ensure products table exists
	fmt.Println("Ensuring database schema is up to date...")
	fmt.Printf("Using DATABASE_URL: %s\n", dbURL[:50]+"...")

	if err := utility.DoMigrations(dbURL); err != nil {
		fmt.Printf("Error running migrations: %v\n", err)

		// Try to connect to database manually to debug
		fmt.Println("Attempting manual database connection for debugging...")
		db, connErr := sql.Open("postgres", dbURL)
		if connErr != nil {
			fmt.Printf("Manual connection failed: %v\n", connErr)
		} else {
			pingErr := db.Ping()
			if pingErr != nil {
				fmt.Printf("Database ping failed: %v\n", pingErr)
			} else {
				fmt.Println("Manual database connection successful!")
				var version string
				queryErr := db.QueryRow("SELECT version()").Scan(&version)
				if queryErr != nil {
					fmt.Printf("Version query failed: %v\n", queryErr)
				} else {
					fmt.Printf("PostgreSQL version: %s\n", version)
				}
			}
			db.Close()
		}

		fmt.Println("\nTroubleshooting tips:")
		fmt.Println("1. Check your DATABASE_URL format. Should be: postgres://user:password@host:port/database?sslmode=disable")
		fmt.Println("2. Try using sslmode=require if your database requires SSL")
		fmt.Println("3. Check if golang-migrate library is compatible with your PostgreSQL version")
		fmt.Println("4. Try using a simpler password without special characters")
		return
	}

	// Fetch and store products
	err := coinbase.FetchAndStoreProducts()
	if err != nil {
		fmt.Printf("Error fetching and storing products: %v\n", err)
		return
	}

	// Get count of products stored
	count, err := coinbase.GetProductsCount()
	if err != nil {
		fmt.Printf("Error getting products count: %v\n", err)
		return
	}

	fmt.Printf("Successfully imported %d products from Coinbase to local database\n", count)

	// Show a few examples
	products, err := coinbase.GetProductsFromDatabase("", "", "", 5)
	if err != nil {
		fmt.Printf("Error retrieving sample products: %v\n", err)
		return
	}

	if len(products) > 0 {
		fmt.Println("\nSample products imported:")
		for _, product := range products {
			fmt.Printf("  %s (%s/%s) - %s\n",
				product.ID, product.BaseCurrency, product.QuoteCurrency, product.Status)
		}
	}
}

// handleListProducts lists available Coinbase products
func handleListProducts() {
	fmt.Println("Fetching available products from Coinbase...")

	products, err := coinbase.GetAvailableProducts()
	if err != nil {
		fmt.Printf("Error fetching products: %v\n", err)
		return
	}

	fmt.Printf("Found %d available products:\n\n", len(products))

	// Group products by base currency
	baseGroups := make(map[string][]string)
	for _, product := range products {
		parts := strings.Split(product, "-")
		if len(parts) == 2 {
			base := parts[0]
			baseGroups[base] = append(baseGroups[base], product)
		}
	}

	// Print grouped products
	for base, products := range baseGroups {
		fmt.Printf("%s:\n", base)
		for _, product := range products {
			fmt.Printf("  %s\n", product)
		}
		fmt.Println()
	}
}

// parseGranularity converts granularity string to seconds
func parseGranularity(granularity string) (int, error) {
	switch strings.ToUpper(granularity) {
	case "ONE_MINUTE":
		return 60, nil
	case "FIVE_MINUTE":
		return 300, nil
	case "FIFTEEN_MINUTE":
		return 900, nil
	case "ONE_HOUR":
		return 3600, nil
	case "SIX_HOURS":
		return 21600, nil
	case "ONE_DAY":
		return 86400, nil
	default:
		return 0, fmt.Errorf("invalid granularity: %s. Available: ONE_MINUTE, FIVE_MINUTE, FIFTEEN_MINUTE, ONE_HOUR, SIX_HOURS, ONE_DAY", granularity)
	}
}

// printCoinbaseHelp shows Coinbase-specific help
func printCoinbaseHelp() {
	fmt.Println(`COINBASE COMMANDS:

USAGE:
    go run main.go coinbase <subcommand> [options]

SUBCOMMANDS:
    fetch candles    Fetch historical candle data
    fetch products   Fetch and store all trading products in database
    products         List available trading products
    help             Show this help message

FETCH CANDLES OPTIONS:
    -product string        Trading product ID (e.g., BTC-USD) [required]
    -start-date string     Start date in YYYY-MM-DD format [required]
    -end-date string       End date in YYYY-MM-DD format [required]
    -granularity string    Time granularity (default: ONE_HOUR)
                           Available: ONE_MINUTE, FIVE_MINUTE, FIFTEEN_MINUTE, 
                                     ONE_HOUR, SIX_HOURS, ONE_DAY

EXAMPLES:
    # List available products
    go run main.go coinbase products

    # Fetch BTC-USD data for January 2024
    go run main.go coinbase fetch candles -product BTC-USD -start-date 2024-01-01 -end-date 2024-01-31

    # Import all products to local database
    go run main.go coinbase fetch products

    # Fetch ETH-USD data for today with 15-minute granularity
    go run main.go coinbase fetch candles -product ETH-USD -start-date 2024-02-06 -end-date 2024-02-06 -granularity FIFTEEN_MINUTE`)
}

func main() {
	// Load environment variables from .env file
	if err := loadEnvFile(); err != nil {
		log.Printf("Warning: Could not load .env file: %v", err)
	}

	// Parse command line arguments
	if len(os.Args) > 1 {
		handleCommandLine()
		return
	}

	fmt.Println("Starting Coinbase Candle Data Platform...")

	dbURL := os.Getenv("DATABASE_URL")
	if dbURL == "" {
		log.Fatal("DATABASE_URL environment variable not set")
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

	// Candle data endpoints
	r.GET("/candles/:product", func(c *gin.Context) {
		productID := c.Param("product")

		// Parse query parameters
		startTime, endTime, limit := parseCandleQueryParams(c)

		// Get candle data from database
		candles, err := coinbase.GetCandleData(productID, startTime, endTime, limit)
		if err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": "Error fetching candle data"})
			return
		}

		c.JSON(http.StatusOK, gin.H{
			"product_id": productID,
			"count":      len(candles),
			"data":       candles,
		})
	})

	r.GET("/candles", func(c *gin.Context) {
		// Get all products with candle data
		products, err := coinbase.GetProductsInDatabase()
		if err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": "Error fetching products"})
			return
		}

		c.JSON(http.StatusOK, gin.H{
			"products": products,
			"count":    len(products),
		})
	})

	r.POST("/candles/:product/fetch", func(c *gin.Context) {
		productID := c.Param("product")

		// Parse granularity from query parameter (default to 1 hour)
		granularity := 3600 // 1 hour default
		if g := c.Query("granularity"); g != "" {
			if parsed, err := strconv.Atoi(g); err == nil {
				granularity = parsed
			}
		}

		// Fetch and store historical candle data
		err := coinbase.FetchAndStoreHistoricalCandles(productID, granularity)
		if err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": fmt.Sprintf("Error fetching historical candles: %v", err)})
			return
		}

		c.JSON(http.StatusOK, gin.H{
			"message":     fmt.Sprintf("Successfully fetched and stored historical candle data for %s", productID),
			"product_id":  productID,
			"granularity": granularity,
		})
	})

	r.GET("/products", func(c *gin.Context) {
		// Get available products from Coinbase
		products, err := coinbase.GetAvailableProducts()
		if err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": "Error fetching available products"})
			return
		}

		c.JSON(http.StatusOK, gin.H{
			"products": products,
			"count":    len(products),
		})
	})

	// Paginated candle data endpoints
	r.GET("/candles/:product/paginated", func(c *gin.Context) {
		productID := c.Param("product")

		// Parse pagination parameters
		page := 1
		perPage := 100

		if p := c.Query("page"); p != "" {
			if parsed, err := strconv.Atoi(p); err == nil && parsed > 0 {
				page = parsed
			}
		}

		if pp := c.Query("per_page"); pp != "" {
			if parsed, err := strconv.Atoi(pp); err == nil && parsed > 0 && parsed <= 1000 {
				perPage = parsed
			}
		}

		// Parse date range parameters
		startDate := c.Query("start_date")
		endDate := c.Query("end_date")

		// Get paginated candle data
		result, err := coinbase.GetCandleDataByDateRange(productID, startDate, endDate, page, perPage)
		if err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": fmt.Sprintf("Error fetching paginated candle data: %v", err)})
			return
		}

		c.JSON(http.StatusOK, result)
	})

	r.GET("/candles/:product/latest", func(c *gin.Context) {
		productID := c.Param("product")

		// Parse limit parameter
		limit := 100
		if l := c.Query("limit"); l != "" {
			if parsed, err := strconv.Atoi(l); err == nil && parsed > 0 && parsed <= 1000 {
				limit = parsed
			}
		}

		// Get latest candle data
		candles, err := coinbase.GetLatestCandleData(productID, limit)
		if err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": "Error fetching latest candle data"})
			return
		}

		c.JSON(http.StatusOK, gin.H{
			"product_id": productID,
			"count":      len(candles),
			"data":       candles,
		})
	})

	r.GET("/candles/:product/info", func(c *gin.Context) {
		productID := c.Param("product")

		// Get data range info
		info, err := coinbase.GetDataRangeInfo(productID)
		if err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": "Error fetching data range info"})
			return
		}

		c.JSON(http.StatusOK, info)
	})

	// Run the web server
	log.Printf("Starting web server...")
	err = r.Run(":8080")
	if err != nil {
		log.Fatalf("Error starting web server: %s", err)
	}
}
