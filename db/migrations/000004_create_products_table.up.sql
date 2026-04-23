CREATE TABLE products (
    id SERIAL PRIMARY KEY,
    product_id VARCHAR(50) UNIQUE NOT NULL,
    base_currency VARCHAR(10) NOT NULL,
    quote_currency VARCHAR(10) NOT NULL,
    price DECIMAL(20,8),
    price_percentage_change_24h DECIMAL(10,4),
    volume_24h DECIMAL(20,8),
    volume_percentage_change_24h DECIMAL(10,4),
    base_increment VARCHAR(50),
    quote_increment VARCHAR(50),
    min_market_size DECIMAL(20,8),
    max_market_size DECIMAL(20,8),
    status VARCHAR(20) NOT NULL,
    quote_currency_id VARCHAR(10),
    base_currency_id VARCHAR(10),
    display_name VARCHAR(100),
    created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
    updated_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP
);

CREATE INDEX idx_products_product_id ON products(product_id);
CREATE INDEX idx_products_base_currency ON products(base_currency);
CREATE INDEX idx_products_quote_currency ON products(quote_currency);
CREATE INDEX idx_products_status ON products(status);
