// Package auth verifica el JWT de auth-svc y protege las rutas con Authorization: Bearer.
package auth

import (
	"crypto/ed25519"
	"errors"
	"fmt"

	"github.com/golang-jwt/jwt/v5"
)

// ErrInvalidToken cubre cualquier motivo de rechazo; el detalle no se devuelve al cliente.
var ErrInvalidToken = errors.New("token inválido")

type User struct {
	Subject string
	Name    string
	Email   string
}

type Verifier struct {
	key    ed25519.PublicKey
	parser *jwt.Parser
}

type claims struct {
	Name  string `json:"name"`
	Email string `json:"email"`
	jwt.RegisteredClaims
}

func NewVerifier(publicKeyPEM []byte, issuer, audience string) (*Verifier, error) {
	k, err := jwt.ParseEdPublicKeyFromPEM(publicKeyPEM)
	if err != nil {
		return nil, fmt.Errorf("auth: clave pública inválida: %w", err)
	}
	pub, ok := k.(ed25519.PublicKey)
	if !ok {
		return nil, errors.New("auth: la clave pública no es Ed25519")
	}
	return &Verifier{
		key: pub,
		parser: jwt.NewParser(
			jwt.WithValidMethods([]string{jwt.SigningMethodEdDSA.Alg()}),
			jwt.WithIssuer(issuer),
			jwt.WithAudience(audience),
			jwt.WithExpirationRequired(),
			jwt.WithIssuedAt(),
		),
	}, nil
}

func (v *Verifier) Verify(token string) (User, error) {
	var c claims
	if _, err := v.parser.ParseWithClaims(token, &c, func(*jwt.Token) (any, error) { return v.key, nil }); err != nil {
		return User{}, fmt.Errorf("%w: %w", ErrInvalidToken, err)
	}
	if c.Subject == "" {
		return User{}, fmt.Errorf("%w: sin sub", ErrInvalidToken)
	}
	// WithIssuedAt solo valida iat si viene; la spec lo exige siempre.
	if c.IssuedAt == nil {
		return User{}, fmt.Errorf("%w: sin iat", ErrInvalidToken)
	}
	return User{Subject: c.Subject, Name: c.Name, Email: c.Email}, nil
}
