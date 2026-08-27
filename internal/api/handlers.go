package api

import (
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"strconv"

	"agentic-sandbox/internal/core"
	"agentic-sandbox/internal/shell"
)

const (
	defaultLimit = 50
	maxLimit     = 200
	maxBodyBytes = 1 << 20 // 1 MiB
)

func (s *Server) handleHealth(w http.ResponseWriter, r *http.Request) {
	if err := s.store.Ping(r.Context()); err != nil {
		writeError(w, http.StatusServiceUnavailable, "unavailable", "database unreachable")
		return
	}
	writeJSON(w, http.StatusOK, map[string]string{"status": "ok"})
}

func (s *Server) handleList(w http.ResponseWriter, r *http.Request) {
	params, err := listParams(r)
	if err != nil {
		writeError(w, http.StatusBadRequest, "bad_request", err.Error())
		return
	}
	todos, err := s.store.List(r.Context(), params)
	if err != nil {
		s.respondError(w, r, err)
		return
	}
	writeJSON(w, http.StatusOK, listResponse{
		Items:  toResponses(todos),
		Limit:  params.Limit,
		Offset: params.Offset,
	})
}

func (s *Server) handleCreate(w http.ResponseWriter, r *http.Request) {
	var body createRequest
	if err := decodeJSON(w, r, &body); err != nil {
		writeError(w, http.StatusBadRequest, "bad_request", err.Error())
		return
	}

	id := shell.NewID()
	effects, err := core.Create(id, core.CreateInput{Title: body.Title}) // 1. PLAN
	if err != nil {
		s.respondError(w, r, err)
		return
	}
	if err := s.applyEffects(r.Context(), effects); err != nil { // 2. GATE + 3. RUN
		s.respondError(w, r, err)
		return
	}

	created, err := s.store.Get(r.Context(), id)
	if err != nil {
		s.respondError(w, r, err)
		return
	}
	w.Header().Set("Location", "/todos/"+id)
	writeJSON(w, http.StatusCreated, toResponse(created))
}

func (s *Server) handleGet(w http.ResponseWriter, r *http.Request) {
	todo, err := s.store.Get(r.Context(), r.PathValue("id"))
	if err != nil {
		s.respondError(w, r, err)
		return
	}
	writeJSON(w, http.StatusOK, toResponse(todo))
}

func (s *Server) handleUpdate(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")

	var body patchRequest
	if err := decodeJSON(w, r, &body); err != nil {
		writeError(w, http.StatusBadRequest, "bad_request", err.Error())
		return
	}

	current, err := s.store.Get(r.Context(), id) // read -> 404 if missing
	if err != nil {
		s.respondError(w, r, err)
		return
	}
	effects, err := core.Update(current, core.Patch{Title: body.Title, Done: body.Done}) // PLAN
	if err != nil {
		s.respondError(w, r, err)
		return
	}
	if err := s.applyEffects(r.Context(), effects); err != nil {
		s.respondError(w, r, err)
		return
	}

	updated, err := s.store.Get(r.Context(), id)
	if err != nil {
		s.respondError(w, r, err)
		return
	}
	writeJSON(w, http.StatusOK, toResponse(updated))
}

func (s *Server) handleDelete(w http.ResponseWriter, r *http.Request) {
	effects, err := core.Delete(r.PathValue("id")) // PLAN
	if err != nil {
		s.respondError(w, r, err)
		return
	}
	// The executor's delete returns shell.ErrNotFound when no row matched, which
	// respondError maps to 404 — so a missing id is handled without a pre-read.
	if err := s.applyEffects(r.Context(), effects); err != nil {
		s.respondError(w, r, err)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

// --- request parsing helpers ------------------------------------------------

func listParams(r *http.Request) (shell.ListParams, error) {
	p := shell.ListParams{Limit: defaultLimit}
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

func decodeJSON(w http.ResponseWriter, r *http.Request, dst any) error {
	r.Body = http.MaxBytesReader(w, r.Body, maxBodyBytes)
	dec := json.NewDecoder(r.Body)
	dec.DisallowUnknownFields()
	if err := dec.Decode(dst); err != nil {
		if errors.Is(err, io.EOF) {
			return fmt.Errorf("request body is empty")
		}
		return fmt.Errorf("invalid JSON body: %w", err)
	}
	if dec.More() {
		return fmt.Errorf("request body must contain a single JSON object")
	}
	return nil
}
