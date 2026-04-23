# Coinbase Historical Candle Data Platform

A comprehensive Go-based platform for fetching and storing historical candle data from Coinbase API.

## Features

- **Historical Data Fetching**: Automatically fetch all available historical candle data for any Coinbase product
- **Database Storage**: Store candle data in PostgreSQL with efficient indexing
- **REST API**: Complete REST API for accessing candle data with pagination
- **Flexible Time Ranges**: Support for various time granularities and date ranges
- **Rate Limiting**: Built-in rate limiting to avoid API restrictions

## API Endpoints

### Core Endpoints

- `GET /currencies` - List all available currencies
- `GET /exchange-rates` - Get exchange rates for a base currency
- `GET /products` - List all available trading products from Coinbase

### Candle Data Endpoints

- `GET /candles` - List all products that have candle data in the database
- `GET /candles/:product` - Get candle data for a specific product (with optional filtering)
- `POST /candles/:product/fetch` - Fetch and store all historical candle data for a product

### Advanced Endpoints

- `GET /candles/:product/paginated` - Get paginated candle data with date range filtering
- `GET /candles/:product/latest` - Get the most recent candle data
- `GET /candles/:product/info` - Get information about the data range for a product

## Query Parameters

### For `/candles/:product`
- `start` - Start time (ISO 8601 format or YYYY-MM-DD)
- `end` - End time (ISO 8601 format or YYYY-MM-DD)
- `limit` - Maximum number of records to return

### For `/candles/:product/paginated`
- `page` - Page number (default: 1)
- `per_page` - Records per page (default: 100, max: 1000)
- `start_date` - Start date (various formats supported)
- `end_date` - End date (various formats supported)

### For `/candles/:product/latest`
- `limit` - Number of recent records to return (default: 100, max: 1000)

### For `/candles/:product/fetch`
- `granularity` - Time granularity in seconds (default: 3600 = 1 hour)
  - Common values: 60 (1 min), 300 (5 min), 900 (15 min), 3600 (1 hour), 86400 (1 day)

## Usage Examples

### Fetch all historical data for BTC-USD
```bash
curl -X POST "http://localhost:8080/candles/BTC-USD/fetch?granularity=3600"
```

### Get recent candle data for ETH-USD
```bash
curl "http://localhost:8080/candles/ETH-USD/latest?limit=50"
```

### Get paginated data for a specific date range
```bash
curl "http://localhost:8080/candles/BTC-USD/paginated?page=1&per_page=100&start_date=2024-01-01&end_date=2024-01-31"
```

### Get data range information
```bash
curl "http://localhost:8080/candles/BTC-USD/info"
```

## Database Schema

### candle_data table
- `id` - Primary key
- `product_id` - Trading pair (e.g., "BTC-USD")
- `timestamp` - Unix timestamp
- `low` - Lowest price in the period
- `high` - Highest price in the period
- `open` - Opening price
- `close` - Closing price
- `volume` - Trading volume
- `created_at` - Record creation timestamp

### Indexes
- Composite index on `(product_id, timestamp)` for fast queries
- Index on `timestamp` for time-based queries

## Setup

1. Set up PostgreSQL database
2. Configure `DATABASE_URL` environment variable
3. Run migrations: Database migrations are automatically run on startup
4. Start the server: `go run main.go`

## Rate Limiting

The platform includes built-in rate limiting to avoid hitting Coinbase API limits:
- 100ms delay between batch requests
- Maximum 300 candles per API request
- Automatic retry logic for failed requests

## Data Formats

### Candle Data Response
```json
{
  "product_id": "BTC-USD",
  "count": 100,
  "data": [
    {
      "product_id": "BTC-USD",
      "timestamp": 1640995200,
      "low": 46000.50,
      "high": 47500.25,
      "open": 46200.00,
      "close": 47100.75,
      "volume": 1234.56
    }
  ]
}
```

### Paginated Response
```json
{
  "product_id": "BTC-USD",
  "data": [...],
  "page": 1,
  "per_page": 100,
  "total": 15000,
  "total_pages": 150
}
```

## Supported Granularities

- 60 seconds (1 minute)
- 300 seconds (5 minutes)
- 900 seconds (15 minutes)
- 3600 seconds (1 hour)
- 21600 seconds (6 hours)
- 86400 seconds (1 day)

## Error Handling

The API returns appropriate HTTP status codes and error messages:
- 400: Bad request (invalid parameters)
- 404: Not found (product doesn't exist)
- 500: Internal server error (database/API issues)

## Performance Considerations

- Database is optimized with proper indexing
- Pagination prevents memory issues with large datasets
- Batch processing for efficient data storage
- Connection pooling for database operations
