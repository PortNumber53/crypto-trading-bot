CREATE TABLE exchange_rates (
    base_currency VARCHAR(255) NOT NULL,
    target_currency VARCHAR(255) NOT NULL,
    rate DECIMAL(18,8) NOT NULL,
    PRIMARY KEY (base_currency, target_currency)
);