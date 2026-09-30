// Package server arma el handler HTTP de la API: rutas y cadena de middlewares.
package server

import (
	"context"
	"log/slog"
	"net/http"
	"time"

	"github.com/JesusMaVe/api/internal/auth"
	"github.com/JesusMaVe/api/internal/httpx"
	"github.com/JesusMaVe/api/internal/items"
)

// healthzTimeout acota el ping a la base en /healthz (detalle interno, no configuración).
const healthzTimeout = 2 * time.Second

// itemsTimeout acota cada petición a /api/* (menor que el WriteTimeout del servidor): una base que
// acepta la conexión pero no responde da un 500 genérico en vez de colgar al cliente.
const itemsTimeout = 5 * time.Second

type Pinger interface {
	Ping(ctx context.Context) error
}

type Deps struct {
	DB           Pinger
	Log          *slog.Logger
	MaxBodyBytes int64
	Verifier     auth.TokenVerifier
	Items        items.Repository
	Limits       items.Limits
}

func New(d Deps) http.Handler {
	mux := http.NewServeMux()
	mux.HandleFunc("GET /healthz", func(w http.ResponseWriter, r *http.Request) {
		ctx, cancel := context.WithTimeout(r.Context(), healthzTimeout)
		defer cancel()
		if err := d.DB.Ping(ctx); err != nil {
			d.Log.Error("healthz: la base no responde", "err", err)
			httpx.JSON(w, http.StatusServiceUnavailable, map[string]string{"status": "unavailable"})
			return
		}
		httpx.JSON(w, http.StatusOK, map[string]string{"status": "ok"})
	})
	api := auth.RequireBearer(d.Verifier, items.NewHandler(d.Items, d.Limits, d.Log))
	mux.Handle("/api/", http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		ctx, cancel := context.WithTimeout(r.Context(), itemsTimeout)
		defer cancel()
		api.ServeHTTP(w, r.WithContext(ctx))
	}))
	return httpx.Chain(mux,
		httpx.RequestID,
		httpx.Logging(d.Log),
		httpx.Recover(d.Log),
		httpx.SecurityHeaders,
		httpx.MaxBytes(d.MaxBodyBytes),
	)
}
