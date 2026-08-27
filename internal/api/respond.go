package api

import (
	"encoding/json"
	"errors"
	"log/slog"
	"net/http"

	"agentic-sandbox/internal/core"
	"agentic-sandbox/internal/shell"
)

// errorBody is the single JSON error shape the API returns.
type errorBody struct {
	Error errorDetail `json:"error"`
}

type errorDetail struct {
	Code    string `json:"code"`
	Message string `json:"message"`
}

func writeJSON(w http.ResponseWriter, status int, v any) {
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	w.WriteHeader(status)
	if v != nil {
		_ = json.NewEncoder(w).Encode(v)
	}
}

func writeError(w http.ResponseWriter, status int, code, msg string) {
	writeJSON(w, status, errorBody{Error: errorDetail{Code: code, Message: msg}})
}

// respondError maps a domain/infra error to the right HTTP status. This is the
// one place error->status translation lives, so handlers stay a single line.
func (s *Server) respondError(w http.ResponseWriter, r *http.Request, err error) {
	var ve core.ValidationError
	var ge shell.GuardrailError
	switch {
	case errors.As(err, &ve):
		writeError(w, http.StatusUnprocessableEntity, "validation_error", ve.Error())
	case errors.As(err, &ge):
		writeError(w, http.StatusUnprocessableEntity, "guardrail_violation", ge.Error())
	case errors.Is(err, shell.ErrNotFound):
		writeError(w, http.StatusNotFound, "not_found", "todo not found")
	default:
		// Unexpected: log the detail, tell the client nothing internal.
		s.log.ErrorContext(r.Context(), "request failed", slog.Any("err", err))
		writeError(w, http.StatusInternalServerError, "internal", "internal server error")
	}
}
