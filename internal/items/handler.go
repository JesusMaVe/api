package items

import (
	"encoding/json"
	"errors"
	"log/slog"
	"net/http"
	"strconv"
	"strings"

	"github.com/JesusMaVe/api/internal/auth"
	"github.com/JesusMaVe/api/internal/httpx"
)

type handler struct {
	repo   Repository
	limits Limits
	log    *slog.Logger
}

// NewHandler expone GET y POST /api/items, y PUT y DELETE /api/items/{id} (solo el dueño).
// Va detrás de auth.RequireBearer.
func NewHandler(repo Repository, l Limits, log *slog.Logger) http.Handler {
	h := &handler{repo: repo, limits: l, log: log}
	mux := http.NewServeMux()
	mux.HandleFunc("GET /api/items", h.list)
	mux.HandleFunc("POST /api/items", h.create)
	mux.HandleFunc("PUT /api/items/{id}", h.update)
	mux.HandleFunc("DELETE /api/items/{id}", h.delete)
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
	c, ok := h.decode(w, r)
	if !ok {
		return
	}
	it, err := h.repo.Create(r.Context(), NewItem{Title: c.Title, Description: c.Description, CreatedBy: user.Subject})
	if err != nil {
		h.log.Error("crear item", "err", err)
		httpx.Error(w, http.StatusInternalServerError, "internal error")
		return
	}
	h.log.Info("item creado", "user", user.Subject, "id", it.ID)
	httpx.JSON(w, http.StatusCreated, it)
}

// decode lee y valida {title, description}; si falla ya respondió (400/413) y devuelve false.
func (h *handler) decode(w http.ResponseWriter, r *http.Request) (Changes, bool) {
	var req createRequest
	dec := json.NewDecoder(r.Body)
	dec.DisallowUnknownFields()
	if err := dec.Decode(&req); err != nil {
		var tooLarge *http.MaxBytesError
		if errors.As(err, &tooLarge) {
			httpx.Error(w, http.StatusRequestEntityTooLarge, "request too large")
			return Changes{}, false
		}
		httpx.Error(w, http.StatusBadRequest, "invalid request")
		return Changes{}, false
	}
	if fields := Validate(req.Title, req.Description, h.limits); fields != nil {
		httpx.JSON(w, http.StatusBadRequest, map[string]any{"error": "validation failed", "fields": fields})
		return Changes{}, false
	}
	return Changes{Title: strings.TrimSpace(req.Title), Description: strings.TrimSpace(req.Description)}, true
}

// itemID lee {id} de la ruta; un id que no es un entero positivo no puede existir: 404.
func itemID(w http.ResponseWriter, r *http.Request) (int64, bool) {
	id, err := strconv.ParseInt(r.PathValue("id"), 10, 64)
	if err != nil || id < 1 {
		httpx.Error(w, http.StatusNotFound, "not found")
		return 0, false
	}
	return id, true
}

// repoError traduce los errores del repositorio; el detalle interno solo va al log.
func (h *handler) repoError(w http.ResponseWriter, err error, action string) {
	switch {
	case errors.Is(err, ErrNotFound):
		httpx.Error(w, http.StatusNotFound, "not found")
	case errors.Is(err, ErrForbidden):
		httpx.Error(w, http.StatusForbidden, "forbidden")
	default:
		h.log.Error(action, "err", err)
		httpx.Error(w, http.StatusInternalServerError, "internal error")
	}
}

func (h *handler) update(w http.ResponseWriter, r *http.Request) {
	user, ok := auth.UserFrom(r.Context())
	if !ok {
		httpx.Error(w, http.StatusUnauthorized, "unauthorized")
		return
	}
	id, ok := itemID(w, r)
	if !ok {
		return
	}
	c, ok := h.decode(w, r)
	if !ok {
		return
	}
	it, err := h.repo.Update(r.Context(), id, user.Subject, c)
	if err != nil {
		h.repoError(w, err, "editar item")
		return
	}
	h.log.Info("item editado", "user", user.Subject, "id", it.ID)
	httpx.JSON(w, http.StatusOK, it)
}

func (h *handler) delete(w http.ResponseWriter, r *http.Request) {
	user, ok := auth.UserFrom(r.Context())
	if !ok {
		httpx.Error(w, http.StatusUnauthorized, "unauthorized")
		return
	}
	id, ok := itemID(w, r)
	if !ok {
		return
	}
	if err := h.repo.Delete(r.Context(), id, user.Subject); err != nil {
		h.repoError(w, err, "borrar item")
		return
	}
	h.log.Info("item borrado", "user", user.Subject, "id", id)
	w.WriteHeader(http.StatusNoContent)
}
