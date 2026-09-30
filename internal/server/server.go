// Package server arma el handler HTTP de la API: rutas y cadena de middlewares.
package server

import (
	"context"
	"log/slog"
	"net/http"
	"time"

	"github.com/JesusMaVe/api/internal/httpx"
)

// healthzTimeout acota el ping a la base en /healthz (detalle interno, no configuración).
const healthzTimeout = 2 * time.Second

type Pinger interface {
	Ping(ctx context.Context) error
}

type Deps struct {
	DB  Pinger
	Log *slog.Logger
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
	return mux
}
