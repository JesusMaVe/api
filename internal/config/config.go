// Package config lee y valida la configuración de la API desde variables de entorno.
// Falla al arrancar si falta algo; los mensajes nombran la variable, nunca su valor.
package config

import (
	"bytes"
	"errors"
	"fmt"
	"log/slog"
	"net"
	"net/url"
	"os"
	"slices"
	"strconv"
	"strings"
)

type Config struct {
	Port               string
	LogLevel           slog.Level
	DatabaseURL        string
	JWTPublicKeyPEM    []byte
	JWTIssuer          string
	JWTAudience        string
	MaxBodyBytes       int64
	ItemTitleMax       int
	ItemDescriptionMax int
	ItemsPageLimit     int
}

// Load construye la Config con getenv (os.Getenv en producción) y devuelve todos los errores juntos.
func Load(getenv func(string) string) (Config, error) {
	r := reader{getenv: getenv}
	pgHost := r.str("POSTGRES_HOST")
	pgPort := r.port("POSTGRES_PORT")
	pgDB := r.str("POSTGRES_DB")
	pgUser := r.str("POSTGRES_USER")
	pgPassword := r.file("POSTGRES_PASSWORD_FILE")
	pgSSLMode := r.oneOf("POSTGRES_SSLMODE", "disable", "require", "verify-full")
	c := Config{
		Port:     r.port("API_PORT"),
		LogLevel: r.logLevel("LOG_LEVEL"),
		DatabaseURL: (&url.URL{
			Scheme:   "postgres",
			User:     url.UserPassword(pgUser, string(pgPassword)),
			Host:     net.JoinHostPort(pgHost, pgPort),
			Path:     "/" + pgDB,
			RawQuery: url.Values{"sslmode": {pgSSLMode}, "connect_timeout": {"5"}}.Encode(),
		}).String(),
		JWTPublicKeyPEM:    r.file("JWT_PUBLIC_KEY_FILE"),
		JWTIssuer:          r.str("JWT_ISSUER"),
		JWTAudience:        r.str("JWT_AUDIENCE"),
		MaxBodyBytes:       int64(r.positiveInt("MAX_BODY_BYTES")),
		ItemTitleMax:       r.positiveInt("ITEM_TITLE_MAX"),
		ItemDescriptionMax: r.positiveInt("ITEM_DESCRIPTION_MAX"),
		ItemsPageLimit:     r.positiveInt("ITEMS_PAGE_LIMIT"),
	}
	if err := errors.Join(r.errs...); err != nil {
		return Config{}, fmt.Errorf("config:\n%w", err)
	}
	return c, nil
}

type reader struct {
	getenv func(string) string
	errs   []error
}

func (r *reader) invalid(key, want string) {
	r.errs = append(r.errs, fmt.Errorf("%s inválida: %s", key, want))
}

func (r *reader) str(key string) string {
	v := strings.TrimSpace(r.getenv(key))
	if v == "" {
		r.errs = append(r.errs, fmt.Errorf("falta %s", key))
	}
	return v
}

func (r *reader) port(key string) string {
	v := r.str(key)
	if v == "" {
		return ""
	}
	if n, err := strconv.Atoi(v); err != nil || n < 1 || n > 65535 {
		r.invalid(key, "debe ser un puerto entre 1 y 65535")
	}
	return v
}

func (r *reader) positiveInt(key string) int {
	v := r.str(key)
	if v == "" {
		return 0
	}
	n, err := strconv.Atoi(v)
	if err != nil || n < 1 {
		r.invalid(key, "debe ser un entero positivo")
		return 0
	}
	return n
}

func (r *reader) oneOf(key string, allowed ...string) string {
	v := r.str(key)
	if v != "" && !slices.Contains(allowed, v) {
		r.invalid(key, "debe ser uno de: "+strings.Join(allowed, ", "))
	}
	return v
}

func (r *reader) logLevel(key string) slog.Level {
	v := r.str(key)
	var l slog.Level
	if v != "" && l.UnmarshalText([]byte(v)) != nil {
		r.invalid(key, "debe ser debug, info, warn o error")
	}
	return l
}

// file lee un secreto montado como archivo (Docker secret) y quita el salto de línea final.
func (r *reader) file(key string) []byte {
	path := r.str(key)
	if path == "" {
		return nil
	}
	b, err := os.ReadFile(path) // #nosec G304 -- la ruta la define el operador (Docker secret)
	if err != nil {
		r.invalid(key, "no se puede leer el archivo")
		return nil
	}
	b = bytes.TrimRight(b, "\r\n")
	if len(b) == 0 {
		r.invalid(key, "el archivo está vacío")
	}
	return b
}
