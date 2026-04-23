package coinbase

import (
	"crypto/ecdsa"
	"crypto/rand"
	"crypto/x509"
	"encoding/pem"
	"fmt"
	"math"
	"math/big"
	"os"
	"strings"
	"time"

	"github.com/golang-jwt/jwt/v5"
)

const coinbaseAPIHost = "api.coinbase.com"

// generateJWT creates a JWT token for Coinbase Advanced Trade API authentication.
// It uses COINBASE_CLOUD_API_KEY_NAME and COINBASE_CLOUD_API_SECRET from env.
// Format follows: https://docs.cdp.coinbase.com/coinbase-app/authentication-authorization/api-key-authentication
func generateJWT(requestMethod, requestPath string) (string, error) {
	keyName := os.Getenv("COINBASE_CLOUD_API_KEY_NAME")
	keySecret := os.Getenv("COINBASE_CLOUD_API_SECRET")

	if keyName == "" || keySecret == "" {
		return "", fmt.Errorf("COINBASE_CLOUD_API_KEY_NAME and COINBASE_CLOUD_API_SECRET must be set")
	}

	// URI format per Coinbase docs: "GET api.coinbase.com/api/v3/brokerage/products"
	uri := fmt.Sprintf("%s %s%s", requestMethod, coinbaseAPIHost, requestPath)

	// Replace literal \n with actual newlines for PEM parsing
	pemData := strings.ReplaceAll(keySecret, "\\n", "\n")

	// Parse the EC private key
	block, _ := pem.Decode([]byte(pemData))
	if block == nil {
		return "", fmt.Errorf("failed to decode PEM block from COINBASE_CLOUD_API_SECRET (length: %d)", len(pemData))
	}

	var key *ecdsa.PrivateKey

	parsedKey, err := x509.ParseECPrivateKey(block.Bytes)
	if err != nil {
		// Try PKCS8 format
		pkcs8Key, err2 := x509.ParsePKCS8PrivateKey(block.Bytes)
		if err2 != nil {
			return "", fmt.Errorf("failed to parse private key (EC: %v, PKCS8: %v)", err, err2)
		}
		var ok bool
		key, ok = pkcs8Key.(*ecdsa.PrivateKey)
		if !ok {
			return "", fmt.Errorf("PKCS8 key is not an EC key")
		}
	} else {
		key = parsedKey
	}

	// Generate a random nonce
	nonce, err := generateNonce()
	if err != nil {
		return "", fmt.Errorf("failed to generate nonce: %w", err)
	}

	now := time.Now()

	// JWT claims per Coinbase docs - no "aud" field
	claims := jwt.MapClaims{
		"sub": keyName,
		"iss": "cdp",
		"nbf": now.Unix(),
		"exp": now.Add(2 * time.Minute).Unix(),
		"uri": uri,
	}

	token := jwt.NewWithClaims(jwt.SigningMethodES256, claims)
	token.Header["kid"] = keyName
	token.Header["nonce"] = nonce
	token.Header["typ"] = "JWT"

	signedToken, err := token.SignedString(key)
	if err != nil {
		return "", fmt.Errorf("failed to sign JWT: %w", err)
	}

	return signedToken, nil
}

// generateNonce creates a cryptographically random hex string
func generateNonce() (string, error) {
	n, err := rand.Int(rand.Reader, big.NewInt(math.MaxInt64))
	if err != nil {
		return "", err
	}
	return fmt.Sprintf("%x", n), nil
}
