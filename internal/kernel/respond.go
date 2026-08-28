package kernel

import (
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"net/http"

	"agentic-sandbox/internal/core"
)

// maxBodyBytes caps request bodies the API will read (1 MiB).
const maxBodyBytes = 1 << 20

// DecodeJSON reads a single JSON object from the request body into dst,
// rejecting unknown fields, oversized bodies, and trailing data. Feature
// handlers use it so decoding rules stay uniform across the API.
func DecodeJSON(w http.ResponseWriter, r *http.Request, dst any) error {
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

// errorBody is the single JSON error shape the API returns.
type errorBody struct {
	Error errorDetail `json:"error"`
}

type errorDetail struct {
	Code    string `json:"code"`
	Message string `json:"message"`
}

// WriteJSON encodes v as a JSON response with the given status.
func WriteJSON(w http.ResponseWriter, status int, v any) {
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	w.WriteHeader(status)
	if v != nil {
		_ = json.NewEncoder(w).Encode(v)
	}
}

// WriteError writes a structured JSON error with the given status.
func WriteError(w http.ResponseWriter, status int, code, msg string) {
	WriteJSON(w, status, errorBody{Error: errorDetail{Code: code, Message: msg}})
}

// RespondError maps a domain/framework error to the right HTTP status. This is
// the one place error->status translation lives, so feature handlers stay a
// single line. It knows only shared types (core validation, the gate's
// guardrail error, and ErrNotFound) — never a specific feature's internals.
func RespondError(w http.ResponseWriter, r *http.Request, logger *slog.Logger, err error) {
	var ve core.ValidationError
	var ge GuardrailError
	switch {
	case errors.As(err, &ve):
		WriteError(w, http.StatusUnprocessableEntity, "validation_error", ve.Error())
	case errors.As(err, &ge):
		WriteError(w, http.StatusUnprocessableEntity, "guardrail_violation", ge.Error())
	case errors.Is(err, ErrNotFound):
		WriteError(w, http.StatusNotFound, "not_found", "resource not found")
	default:
		// Unexpected: log the detail, tell the client nothing internal.
		logger.ErrorContext(r.Context(), "request failed", slog.Any("err", err))
		WriteError(w, http.StatusInternalServerError, "internal", "internal server error")
	}
}
