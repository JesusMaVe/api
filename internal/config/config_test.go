package config

import (
	"log/slog"
	"net/url"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

const secretValue = "p@ss:w/rd-que-no-debe-salir"

func validEnv(t *testing.T) map[string]string {
	t.Helper()
	dir := t.TempDir()
	pw := filepath.Join(dir, "pg_pw")
	key := filepath.Join(dir, "jwt_pub")
	if err := os.WriteFile(pw, []byte(secretValue+"\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(key, []byte("-----BEGIN PUBLIC KEY-----\nabc\n-----END PUBLIC KEY-----\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	return map[string]string{
		"API_PORT":               "8080",
		"LOG_LEVEL":              "info",
		"POSTGRES_HOST":          "postgres",
		"POSTGRES_PORT":          "5432",
		"POSTGRES_DB":            "api",
		"POSTGRES_USER":          "api",
		"POSTGRES_PASSWORD_FILE": pw,
		"POSTGRES_SSLMODE":       "disable",
		"JWT_PUBLIC_KEY_FILE":    key,
		"JWT_ISSUER":             "auth-svc",
		"JWT_AUDIENCE":           "auth-dashboard-api",
		"MAX_BODY_BYTES":         "65536",
		"ITEM_TITLE_MAX":         "120",
		"ITEM_DESCRIPTION_MAX":   "1000",
		"ITEMS_PAGE_LIMIT":       "100",
	}
}

func load(env map[string]string) (Config, error) {
	return Load(func(k string) string { return env[k] })
}

func TestLoadValid(t *testing.T) {
	c, err := load(validEnv(t))
	if err != nil {
		t.Fatalf("Load: %v", err)
	}
	if c.Port != "8080" || c.LogLevel != slog.LevelInfo || c.JWTIssuer != "auth-svc" || c.JWTAudience != "auth-dashboard-api" {
		t.Errorf("valores inesperados: %+v", c)
	}
	if c.MaxBodyBytes != 65536 || c.ItemTitleMax != 120 || c.ItemDescriptionMax != 1000 || c.ItemsPageLimit != 100 {
		t.Errorf("límites: %+v", c)
	}
	if !strings.HasPrefix(string(c.JWTPublicKeyPEM), "-----BEGIN PUBLIC KEY-----") {
		t.Error("JWTPublicKeyPEM no se leyó del archivo")
	}
	u, err := url.Parse(c.DatabaseURL)
	if err != nil {
		t.Fatalf("DatabaseURL no es una URL: %v", err)
	}
	pw, _ := u.User.Password()
	if u.Scheme != "postgres" || u.Host != "postgres:5432" || u.Path != "/api" || u.User.Username() != "api" || pw != secretValue {
		t.Errorf("DatabaseURL mal armada (host=%s path=%s user=%s)", u.Host, u.Path, u.User.Username())
	}
	if u.Query().Get("sslmode") != "disable" {
		t.Errorf("sslmode = %q", u.Query().Get("sslmode"))
	}
}

func TestLoadMissing(t *testing.T) {
	for key := range validEnv(t) {
		t.Run(key, func(t *testing.T) {
			env := validEnv(t)
			delete(env, key)
			_, err := load(env)
			if err == nil || !strings.Contains(err.Error(), key) {
				t.Fatalf("falta %s: se esperaba un error que la nombre, got %v", key, err)
			}
		})
	}
}

func TestLoadInvalid(t *testing.T) {
	cases := []struct{ key, value string }{
		{"API_PORT", "0"},
		{"POSTGRES_PORT", "abc"},
		{"POSTGRES_SSLMODE", "prefer"},
		{"LOG_LEVEL", "verbose"},
		{"MAX_BODY_BYTES", "0"},
		{"ITEM_TITLE_MAX", "-1"},
		{"ITEMS_PAGE_LIMIT", "muchos"},
		{"POSTGRES_PASSWORD_FILE", "/no/existe"},
		{"JWT_PUBLIC_KEY_FILE", "/no/existe"},
	}
	for _, tc := range cases {
		t.Run(tc.key+"="+tc.value, func(t *testing.T) {
			env := validEnv(t)
			env[tc.key] = tc.value
			_, err := load(env)
			if err == nil || !strings.Contains(err.Error(), tc.key) {
				t.Fatalf("se esperaba un error que nombre %s, got %v", tc.key, err)
			}
			if strings.Contains(err.Error(), secretValue) {
				t.Fatalf("el error muestra el valor de un secreto: %v", err)
			}
		})
	}
}

func TestLoadReportsAllErrors(t *testing.T) {
	_, err := load(map[string]string{})
	if err == nil {
		t.Fatal("se esperaba error")
	}
	for _, key := range []string{"API_PORT", "POSTGRES_HOST", "JWT_ISSUER"} {
		if !strings.Contains(err.Error(), key) {
			t.Errorf("el error debe nombrar %s: %v", key, err)
		}
	}
}
