package auth

import (
	"errors"
	"testing"
	"time"

	"github.com/golang-jwt/jwt/v5"

	"github.com/JesusMaVe/api/internal/auth/authtest"
)

func newVerifier(t *testing.T, keys authtest.Keys) *Verifier {
	t.Helper()
	v, err := NewVerifier(keys.PublicPEM, authtest.Issuer, authtest.Audience)
	if err != nil {
		t.Fatalf("NewVerifier: %v", err)
	}
	return v
}

func TestVerifyValid(t *testing.T) {
	keys := authtest.NewKeys(t)
	u, err := newVerifier(t, keys).Verify(authtest.Sign(t, keys.Private, authtest.ValidClaims("alice")))
	if err != nil {
		t.Fatalf("Verify: %v", err)
	}
	if u != (User{Subject: "alice", Name: "Alice Example", Email: "alice@example.org"}) {
		t.Fatalf("usuario: %+v", u)
	}
}

func TestVerifyRejects(t *testing.T) {
	keys := authtest.NewKeys(t)
	other := authtest.NewKeys(t)
	v := newVerifier(t, keys)
	with := func(k string, val any) jwt.MapClaims {
		c := authtest.ValidClaims("alice")
		if val == nil {
			delete(c, k)
		} else {
			c[k] = val
		}
		return c
	}
	none, err := jwt.NewWithClaims(jwt.SigningMethodNone, authtest.ValidClaims("alice")).SignedString(jwt.UnsafeAllowNoneSignatureType)
	if err != nil {
		t.Fatal(err)
	}
	// "alg confusion": HS256 usando como secreto el PEM de la clave pública.
	hs, err := jwt.NewWithClaims(jwt.SigningMethodHS256, authtest.ValidClaims("alice")).SignedString(keys.PublicPEM)
	if err != nil {
		t.Fatal(err)
	}
	cases := map[string]string{
		"alg none":             none,
		"HS256 con la pública": hs,
		"otra clave":           authtest.Sign(t, other.Private, authtest.ValidClaims("alice")),
		"expirado":             authtest.Sign(t, keys.Private, with("exp", time.Now().Add(-time.Minute).Unix())),
		"sin exp":              authtest.Sign(t, keys.Private, with("exp", nil)),
		"iss distinto":         authtest.Sign(t, keys.Private, with("iss", "otro")),
		"aud distinta":         authtest.Sign(t, keys.Private, with("aud", "otra-api")),
		"sin sub":              authtest.Sign(t, keys.Private, with("sub", nil)),
		"sin iat":              authtest.Sign(t, keys.Private, with("iat", nil)),
		"malformado":           "no.es.un-jwt",
		"vacío":                "",
	}
	for name, tok := range cases {
		t.Run(name, func(t *testing.T) {
			if _, err := v.Verify(tok); !errors.Is(err, ErrInvalidToken) {
				t.Fatalf("se esperaba ErrInvalidToken, got %v", err)
			}
		})
	}
}

func TestNewVerifierRejectsBadKey(t *testing.T) {
	if _, err := NewVerifier([]byte("basura"), "i", "a"); err == nil {
		t.Fatal("se esperaba error")
	}
}
