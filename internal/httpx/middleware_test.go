package httpx

import (
	"bytes"
	"errors"
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"regexp"
	"strings"
	"testing"
)

func serve(h http.Handler, req *http.Request) *httptest.ResponseRecorder {
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, req)
	return rec
}

func TestSecurityHeaders(t *testing.T) {
	rec := serve(SecurityHeaders(http.NotFoundHandler()), httptest.NewRequest("GET", "/", nil))
	want := map[string]string{
		"Content-Security-Policy": "default-src 'none'; frame-ancestors 'none'",
		"X-Content-Type-Options":  "nosniff",
		"Referrer-Policy":         "no-referrer",
		"Cache-Control":           "no-store",
	}
	for k, v := range want {
		if got := rec.Header().Get(k); got != v {
			t.Errorf("%s = %q, se esperaba %q", k, got, v)
		}
	}
}

func TestMaxBytes(t *testing.T) {
	var readErr error
	h := MaxBytes(10)(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, readErr = io.ReadAll(r.Body)
	}))
	serve(h, httptest.NewRequest("POST", "/", strings.NewReader(strings.Repeat("a", 10))))
	if readErr != nil {
		t.Fatalf("10 bytes deben leerse: %v", readErr)
	}
	serve(h, httptest.NewRequest("POST", "/", strings.NewReader(strings.Repeat("a", 11))))
	var tooLarge *http.MaxBytesError
	if !errors.As(readErr, &tooLarge) {
		t.Fatalf("11 bytes deben dar MaxBytesError, got %v", readErr)
	}
}

func TestRequestIDAlwaysNew(t *testing.T) {
	var fromCtx string
	h := RequestID(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		fromCtx = RequestIDFrom(r.Context())
	}))
	req := httptest.NewRequest("GET", "/", nil)
	req.Header.Set("X-Request-Id", "puesto-por-el-cliente")
	rec := serve(h, req)
	id := rec.Header().Get("X-Request-Id")
	if !regexp.MustCompile(`^[0-9a-f]{16}$`).MatchString(id) {
		t.Fatalf("X-Request-Id = %q", id)
	}
	if fromCtx != id {
		t.Fatalf("el contexto tiene %q y el header %q", fromCtx, id)
	}
}

func TestLoggingNeverLogsCredentials(t *testing.T) {
	var logs bytes.Buffer
	log := slog.New(slog.NewJSONHandler(&logs, nil))
	h := RequestID(Logging(log)(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusTeapot)
	})))
	req := httptest.NewRequest("GET", "/api/items?token=secreto-en-query", nil)
	req.Header.Set("Authorization", "Bearer secreto-en-header")
	serve(h, req)
	out := logs.String()
	for _, want := range []string{`"path":"/api/items"`, `"status":418`, `"method":"GET"`, `"request_id":"`} {
		if !strings.Contains(out, want) {
			t.Errorf("el log debe contener %s: %s", want, out)
		}
	}
	if strings.Contains(out, "secreto-en-header") || strings.Contains(out, "secreto-en-query") {
		t.Fatalf("el log contiene credenciales: %s", out)
	}
}

func TestRecover(t *testing.T) {
	var logs bytes.Buffer
	log := slog.New(slog.NewJSONHandler(&logs, nil))
	h := Recover(log)(http.HandlerFunc(func(http.ResponseWriter, *http.Request) { panic("boom") }))
	rec := serve(h, httptest.NewRequest("GET", "/", nil))
	if rec.Code != 500 || rec.Body.String() != "{\"error\":\"internal error\"}\n" {
		t.Fatalf("got %d %q", rec.Code, rec.Body)
	}
	if !strings.Contains(logs.String(), "boom") {
		t.Fatalf("el panic debe quedar en el log: %s", logs.String())
	}
}

func TestChainOrder(t *testing.T) {
	var order []string
	mw := func(name string) Middleware {
		return func(next http.Handler) http.Handler {
			return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				order = append(order, name)
				next.ServeHTTP(w, r)
			})
		}
	}
	serve(Chain(http.NotFoundHandler(), mw("a"), mw("b")), httptest.NewRequest("GET", "/", nil))
	if strings.Join(order, ",") != "a,b" {
		t.Fatalf("orden = %v, el primero debe ser el más externo", order)
	}
}
