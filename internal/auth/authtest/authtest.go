// Package authtest genera claves y tokens para los tests. Solo lo importan tests.
package authtest

import (
	"crypto/ed25519"
	"crypto/rand"
	"crypto/x509"
	"encoding/pem"
	"testing"
	"time"

	"github.com/golang-jwt/jwt/v5"
)

const (
	Issuer   = "auth-svc"
	Audience = "auth-dashboard-api"
)

type Keys struct {
	PublicPEM []byte
	Private   ed25519.PrivateKey
}

func NewKeys(t *testing.T) Keys {
	t.Helper()
	pub, priv, err := ed25519.GenerateKey(rand.Reader)
	if err != nil {
		t.Fatal(err)
	}
	der, err := x509.MarshalPKIXPublicKey(pub)
	if err != nil {
		t.Fatal(err)
	}
	return Keys{PublicPEM: pem.EncodeToMemory(&pem.Block{Type: "PUBLIC KEY", Bytes: der}), Private: priv}
}

func ValidClaims(sub string) jwt.MapClaims {
	now := time.Now()
	return jwt.MapClaims{
		"sub": sub, "name": "Alice Example", "email": "alice@example.org",
		"iss": Issuer, "aud": Audience,
		"iat": now.Unix(), "nbf": now.Unix(), "exp": now.Add(time.Hour).Unix(),
	}
}

func Sign(t *testing.T, key ed25519.PrivateKey, claims jwt.MapClaims) string {
	t.Helper()
	tok, err := jwt.NewWithClaims(jwt.SigningMethodEdDSA, claims).SignedString(key)
	if err != nil {
		t.Fatal(err)
	}
	return tok
}
