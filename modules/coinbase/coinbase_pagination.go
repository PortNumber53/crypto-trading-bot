package coinbase

import (
	"fmt"
	"log"
	"strconv"
	"time"
	"trading/modules/database"
)

// parseDateString parses various date string formats and returns Unix timestamp
func parseDateString(dateStr string) (int64, error) {
	// Try different date formats
	formats := []string{
		"2006-01-02T15:04:05Z",
		"2006-01-02T15:04:05",
		"2006-01-02",
		"2006/01/02",
		"01/02/2006",
		"02/01/2006",
	}

	for _, format := range formats {
		if t, err := time.Parse(format, dateStr); err == nil {
			return t.Unix(), nil
		}
	}

	// Try to parse as Unix timestamp
	if timestamp, err := strconv.ParseInt(dateStr, 10, 64); err == nil {
		return timestamp, nil
	}

	return 0, fmt.Errorf("unable to parse date string: %s", dateStr)
}

// CandleDataPagination represents paginated candle data response
type CandleDataPagination struct {
	ProductID  string       `json:"product_id"`
	Data       []CandleData `json:"data"`
	Page       int          `json:"page"`
	PerPage    int          `json:"per_page"`
	Total      int64        `json:"total"`
	TotalPages int          `json:"total_pages"`
}

// GetCandleDataPaginated retrieves candle data with pagination
func GetCandleDataPaginated(productID string, page, perPage int, startTime, endTime *int64) (*CandleDataPagination, error) {
	log.Printf("- Retrieving paginated candle data for %s, page %d, per_page %d", productID, page, perPage)

	db, err := database.OpenConnection()
	if err != nil {
		return nil, fmt.Errorf("failed to open database connection: %w", err)
	}
	defer database.CloseConnection(db)

	// First, get total count
	countQuery := "SELECT COUNT(*) FROM candle_data WHERE product_id = $1"
	countArgs := []interface{}{productID}
	argIndex := 2

	if startTime != nil {
		countQuery += fmt.Sprintf(" AND timestamp >= $%d", argIndex)
		countArgs = append(countArgs, *startTime)
		argIndex++
	}

	if endTime != nil {
		countQuery += fmt.Sprintf(" AND timestamp <= $%d", argIndex)
		countArgs = append(countArgs, *endTime)
		argIndex++
	}

	var total int64
	err = db.QueryRow(countQuery, countArgs...).Scan(&total)
	if err != nil {
		return nil, fmt.Errorf("failed to get total count: %w", err)
	}

	// Calculate pagination
	offset := (page - 1) * perPage
	totalPages := int((total + int64(perPage) - 1) / int64(perPage))

	// Build main query
	query := "SELECT product_id, timestamp, low, high, open, close, volume FROM candle_data WHERE product_id = $1"
	args := []interface{}{productID}
	argIndex = 2

	if startTime != nil {
		query += fmt.Sprintf(" AND timestamp >= $%d", argIndex)
		args = append(args, *startTime)
		argIndex++
	}

	if endTime != nil {
		query += fmt.Sprintf(" AND timestamp <= $%d", argIndex)
		args = append(args, *endTime)
		argIndex++
	}

	query += " ORDER BY timestamp ASC LIMIT $%d OFFSET $%d"
	args = append(args, perPage, offset)

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

	result := &CandleDataPagination{
		ProductID:  productID,
		Data:       candles,
		Page:       page,
		PerPage:    perPage,
		Total:      total,
		TotalPages: totalPages,
	}

	log.Printf("Retrieved %d candle data points for %s (page %d of %d, total: %d)",
		len(candles), productID, page, totalPages, total)

	return result, nil
}

// GetCandleDataByDateRange retrieves candle data for a specific date range with pagination
func GetCandleDataByDateRange(productID string, startDate, endDate string, page, perPage int) (*CandleDataPagination, error) {
	log.Printf("- Retrieving candle data for %s from %s to %s", productID, startDate, endDate)

	// Parse date strings to timestamps
	var startTime, endTime *int64

	if startDate != "" {
		if parsed, err := parseDateString(startDate); err == nil {
			startTime = &parsed
		} else {
			return nil, fmt.Errorf("invalid start date format: %w", err)
		}
	}

	if endDate != "" {
		if parsed, err := parseDateString(endDate); err == nil {
			endTime = &parsed
		} else {
			return nil, fmt.Errorf("invalid end date format: %w", err)
		}
	}

	return GetCandleDataPaginated(productID, page, perPage, startTime, endTime)
}

// GetLatestCandleData retrieves the most recent candle data for a product
func GetLatestCandleData(productID string, limit int) ([]CandleData, error) {
	log.Printf("- Retrieving latest %d candle data points for %s", limit, productID)

	db, err := database.OpenConnection()
	if err != nil {
		return nil, fmt.Errorf("failed to open database connection: %w", err)
	}
	defer database.CloseConnection(db)

	query := "SELECT product_id, timestamp, low, high, open, close, volume FROM candle_data WHERE product_id = $1 ORDER BY timestamp DESC LIMIT $2"

	rows, err := db.Query(query, productID, limit)
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

	log.Printf("Retrieved %d latest candle data points for %s", len(candles), productID)
	return candles, nil
}

// GetOldestCandleData retrieves the oldest candle data for a product
func GetOldestCandleData(productID string, limit int) ([]CandleData, error) {
	log.Printf("- Retrieving oldest %d candle data points for %s", limit, productID)

	db, err := database.OpenConnection()
	if err != nil {
		return nil, fmt.Errorf("failed to open database connection: %w", err)
	}
	defer database.CloseConnection(db)

	query := "SELECT product_id, timestamp, low, high, open, close, volume FROM candle_data WHERE product_id = $1 ORDER BY timestamp ASC LIMIT $2"

	rows, err := db.Query(query, productID, limit)
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

	log.Printf("Retrieved %d oldest candle data points for %s", len(candles), productID)
	return candles, nil
}

// GetDataRangeInfo gets information about the date range of candle data for a product
func GetDataRangeInfo(productID string) (map[string]interface{}, error) {
	log.Printf("- Getting data range info for %s", productID)

	db, err := database.OpenConnection()
	if err != nil {
		return nil, fmt.Errorf("failed to open database connection: %w", err)
	}
	defer database.CloseConnection(db)

	var count int64
	var minTimestamp, maxTimestamp int64

	// Get count and date range
	err = db.QueryRow(`
		SELECT 
			COUNT(*) as count,
			MIN(timestamp) as min_timestamp,
			MAX(timestamp) as max_timestamp
		FROM candle_data 
		WHERE product_id = $1
	`, productID).Scan(&count, &minTimestamp, &maxTimestamp)

	if err != nil {
		return nil, fmt.Errorf("failed to get data range info: %w", err)
	}

	result := map[string]interface{}{
		"product_id":       productID,
		"total_records":    count,
		"oldest_timestamp": minTimestamp,
		"newest_timestamp": maxTimestamp,
	}

	if count > 0 {
		result["has_data"] = true
	} else {
		result["has_data"] = false
	}

	log.Printf("Data range info for %s: %d records from %d to %d",
		productID, count, minTimestamp, maxTimestamp)

	return result, nil
}
