package items

import (
	"encoding/json"
	"errors"
	"log/slog"
	"net/http"
	"strings"

	"github.com/JesusMaVe/api/internal/auth"
	"github.com/JesusMaVe/api/internal/httpx"
)

type handler struct {
	repo   Repository
	limits Limits
	log    *slog.Logger
}

// NewHandler expone GET y POST /api/items. Va detrás de auth.RequireBearer.
func NewHandler(repo Repository, l Limits, log *slog.Logger) http.Handler {
	h := &handler{repo: repo, limits: l, log: log}
	mux := http.NewServeMux()
	mux.HandleFunc("GET /api/items", h.list)
	mux.HandleFunc("POST /api/items", h.create)
	return mux
}

func (h *handler) list(w http.ResponseWriter, r *http.Request) {
	if _, ok := auth.UserFrom(r.Context()); !ok {
		httpx.Error(w, http.StatusUnauthorized, "unauthorized")
		return
	}
	list, err := h.repo.List(r.Context(), h.limits.PageLimit)
	if err != nil {
		h.log.Error("listar items", "err", err)
		httpx.Error(w, http.StatusInternalServerError, "internal error")
		return
	}
	httpx.JSON(w, http.StatusOK, map[string][]Item{"items": list})
}

type createRequest struct {
	Title       string `json:"title"`
	Description string `json:"description"`
}

func (h *handler) create(w http.ResponseWriter, r *http.Request) {
	user, ok := auth.UserFrom(r.Context())
	if !ok {
		httpx.Error(w, http.StatusUnauthorized, "unauthorized")
		return
	}
	var req createRequest
	dec := json.NewDecoder(r.Body)
	dec.DisallowUnknownFields()
	if err := dec.Decode(&req); err != nil {
		var tooLarge *http.MaxBytesError
		if errors.As(err, &tooLarge) {
			httpx.Error(w, http.StatusRequestEntityTooLarge, "request too large")
			return
		}
		httpx.Error(w, http.StatusBadRequest, "invalid request")
		return
	}
	if fields := Validate(req.Title, req.Description, h.limits); fields != nil {
		httpx.JSON(w, http.StatusBadRequest, map[string]any{"error": "validation failed", "fields": fields})
		return
	}
	it, err := h.repo.Create(r.Context(), NewItem{
		Title:       strings.TrimSpace(req.Title),
		Description: strings.TrimSpace(req.Description),
		CreatedBy:   user.Subject,
	})
	if err != nil {
		h.log.Error("crear item", "err", err)
		httpx.Error(w, http.StatusInternalServerError, "internal error")
		return
	}
	h.log.Info("item creado", "user", user.Subject, "id", it.ID)
	httpx.JSON(w, http.StatusCreated, it)
}
