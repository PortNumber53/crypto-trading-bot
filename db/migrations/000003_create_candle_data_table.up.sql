CREATE TABLE candle_data (
    id SERIAL PRIMARY KEY,
    product_id VARCHAR(50) NOT NULL,
    timestamp BIGINT NOT NULL,
    low DECIMAL(20,8) NOT NULL,
    high DECIMAL(20,8) NOT NULL,
    open DECIMAL(20,8) NOT NULL,
    close DECIMAL(20,8) NOT NULL,
    volume DECIMAL(20,8) NOT NULL,
    created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
    UNIQUE(product_id, timestamp)
);

CREATE INDEX idx_candle_data_product_timestamp ON candle_data(product_id, timestamp);
CREATE INDEX idx_candle_data_timestamp ON candle_data(timestamp);
