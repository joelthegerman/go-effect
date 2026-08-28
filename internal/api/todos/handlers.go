// Package todos is the HTTP layer for the todos feature: routing, request
// decoding, and response encoding. It translates HTTP into the plan -> gate ->
// run flow (plan via core/todos, gate+run via kernel.Pipeline) and holds no
// business logic of its own.
package todos

import (
	"context"
	"fmt"
	"log/slog"
	"net/http"
	"strconv"

	"agentic-sandbox/internal/core"
	todo "agentic-sandbox/internal/core/todos"
	"agentic-sandbox/internal/kernel"
	store "agentic-sandbox/internal/shell/todos"
)

const (
	defaultLimit = 50
	maxLimit     = 200
)

// Reader is the read surface these handlers need. Writes never go through here —
// they go through the gate via the pipeline — which is why this exposes only
// reads. The todos repository satisfies it.
type Reader interface {
	List(ctx context.Context, p store.ListParams) ([]core.Todo, error)
	Get(ctx context.Context, id string) (core.Todo, error)
}

// Handlers serves the todos HTTP routes.
type Handlers struct {
	reader Reader
	write  *kernel.Pipeline
	log    *slog.Logger
}

// NewHandlers wires the todos HTTP layer to its repository (reads) and the
// shared write pipeline (gated, atomic writes).
func NewHandlers(reader Reader, write *kernel.Pipeline, log *slog.Logger) *Handlers {
	return &Handlers{reader: reader, write: write, log: log}
}

// Register mounts the todos routes on the given mux.
func (h *Handlers) Register(mux *http.ServeMux) {
	mux.HandleFunc("GET /todos", h.list)
	mux.HandleFunc("POST /todos", h.create)
	mux.HandleFunc("GET /todos/{id}", h.get)
	mux.HandleFunc("PATCH /todos/{id}", h.update)
	mux.HandleFunc("DELETE /todos/{id}", h.delete)
}

func (h *Handlers) list(w http.ResponseWriter, r *http.Request) {
	params, err := listParams(r)
	if err != nil {
		kernel.WriteError(w, http.StatusBadRequest, "bad_request", err.Error())
		return
	}
	todos, err := h.reader.List(r.Context(), params)
	if err != nil {
		kernel.RespondError(w, r, h.log, err)
		return
	}
	kernel.WriteJSON(w, http.StatusOK, listResponse{
		Items:  toResponses(todos),
		Limit:  params.Limit,
		Offset: params.Offset,
	})
}

func (h *Handlers) create(w http.ResponseWriter, r *http.Request) {
	var body createRequest
	if err := kernel.DecodeJSON(w, r, &body); err != nil {
		kernel.WriteError(w, http.StatusBadRequest, "bad_request", err.Error())
		return
	}

	id := kernel.NewID()
	effects, err := todo.Create(id, todo.CreateInput{Title: body.Title}) // 1. PLAN
	if err != nil {
		kernel.RespondError(w, r, h.log, err)
		return
	}
	if err := h.write.Apply(r.Context(), effects); err != nil { // 2. GATE + 3. RUN
		kernel.RespondError(w, r, h.log, err)
		return
	}

	created, err := h.reader.Get(r.Context(), id)
	if err != nil {
		kernel.RespondError(w, r, h.log, err)
		return
	}
	w.Header().Set("Location", "/todos/"+id)
	kernel.WriteJSON(w, http.StatusCreated, toResponse(created))
}

func (h *Handlers) get(w http.ResponseWriter, r *http.Request) {
	t, err := h.reader.Get(r.Context(), r.PathValue("id"))
	if err != nil {
		kernel.RespondError(w, r, h.log, err)
		return
	}
	kernel.WriteJSON(w, http.StatusOK, toResponse(t))
}

func (h *Handlers) update(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")

	var body patchRequest
	if err := kernel.DecodeJSON(w, r, &body); err != nil {
		kernel.WriteError(w, http.StatusBadRequest, "bad_request", err.Error())
		return
	}

	current, err := h.reader.Get(r.Context(), id) // read -> 404 if missing
	if err != nil {
		kernel.RespondError(w, r, h.log, err)
		return
	}
	effects, err := todo.Update(current, todo.Patch{Title: body.Title, Done: body.Done}) // PLAN
	if err != nil {
		kernel.RespondError(w, r, h.log, err)
		return
	}
	if err := h.write.Apply(r.Context(), effects); err != nil {
		kernel.RespondError(w, r, h.log, err)
		return
	}

	updated, err := h.reader.Get(r.Context(), id)
	if err != nil {
		kernel.RespondError(w, r, h.log, err)
		return
	}
	kernel.WriteJSON(w, http.StatusOK, toResponse(updated))
}

func (h *Handlers) delete(w http.ResponseWriter, r *http.Request) {
	effects, err := todo.Delete(r.PathValue("id")) // PLAN
	if err != nil {
		kernel.RespondError(w, r, h.log, err)
		return
	}
	// The executor's delete returns kernel.ErrNotFound when no row matched, which
	// RespondError maps to 404 — so a missing id is handled without a pre-read.
	if err := h.write.Apply(r.Context(), effects); err != nil {
		kernel.RespondError(w, r, h.log, err)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

func listParams(r *http.Request) (store.ListParams, error) {
	p := store.ListParams{Limit: defaultLimit}
	q := r.URL.Query()

	if v := q.Get("limit"); v != "" {
		n, err := strconv.Atoi(v)
		if err != nil || n < 1 {
			return p, fmt.Errorf("limit must be a positive integer")
		}
		p.Limit = min(n, maxLimit)
	}
	if v := q.Get("offset"); v != "" {
		n, err := strconv.Atoi(v)
		if err != nil || n < 0 {
			return p, fmt.Errorf("offset must be a non-negative integer")
		}
		p.Offset = n
	}
	if v := q.Get("done"); v != "" {
		b, err := strconv.ParseBool(v)
		if err != nil {
			return p, fmt.Errorf("done must be true or false")
		}
		p.Done = &b
	}
	return p, nil
}
