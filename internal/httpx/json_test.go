package httpx

import (
	"net/http/httptest"
	"testing"
)

func TestJSON(t *testing.T) {
	rec := httptest.NewRecorder()
	JSON(rec, 201, map[string]int{"n": 1})
	if rec.Code != 201 || rec.Header().Get("Content-Type") != "application/json" || rec.Body.String() != "{\"n\":1}\n" {
		t.Fatalf("got %d %q %q", rec.Code, rec.Header().Get("Content-Type"), rec.Body)
	}
}

func TestError(t *testing.T) {
	rec := httptest.NewRecorder()
	Error(rec, 404, "not found")
	if rec.Code != 404 || rec.Body.String() != "{\"error\":\"not found\"}\n" {
		t.Fatalf("got %d %q", rec.Code, rec.Body)
	}
}
