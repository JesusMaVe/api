package server

import (
	"context"
	"errors"
	"io"
	"log/slog"
	"net/http/httptest"
	"strings"
	"testing"
)

type fakeDB struct{ err error }

func (f fakeDB) Ping(context.Context) error { return f.err }

func discard() *slog.Logger { return slog.New(slog.NewTextHandler(io.Discard, nil)) }

func get(t *testing.T, d Deps, path string) *httptest.ResponseRecorder {
	t.Helper()
	rec := httptest.NewRecorder()
	New(d).ServeHTTP(rec, httptest.NewRequest("GET", path, nil))
	return rec
}

func TestHealthzOK(t *testing.T) {
	rec := get(t, Deps{DB: fakeDB{}, Log: discard()}, "/healthz")
	if rec.Code != 200 || !strings.Contains(rec.Body.String(), `"status":"ok"`) {
		t.Fatalf("got %d %s", rec.Code, rec.Body)
	}
}

func TestHealthzDBDown(t *testing.T) {
	rec := get(t, Deps{DB: fakeDB{err: errors.New("dial tcp: refused")}, Log: discard()}, "/healthz")
	if rec.Code != 503 || strings.Contains(rec.Body.String(), "refused") {
		t.Fatalf("se esperaba 503 genérico, got %d %s", rec.Code, rec.Body)
	}
}
