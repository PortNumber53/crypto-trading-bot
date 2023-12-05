CREATE TABLE currencies (
    id VARCHAR(3) PRIMARY KEY,
    name VARCHAR(255) NOT NULL,
    min_size DECIMAL(18,8) NOT NULL
);
