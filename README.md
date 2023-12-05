# Crypto trading bot


1. Install migrate go tool



```
migrate create -ext sql -dir db/migrations -seq create_currencies_table

migrate create -ext sql -dir db/migrations -seq create_exchange_rates_table


```


Add an env var like this
```
DB_URL='postgres://grimlock:1234@localhost:5432/dbcrypto?sslmode=disable'
```
