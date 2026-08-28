package todos

import (
	"time"

	"agentic-sandbox/internal/core"
)

// createRequest is the POST /todos body.
type createRequest struct {
	Title string `json:"title"`
}

// patchRequest is the PATCH /todos/{id} body. Pointers distinguish an omitted
// field from a zero value, matching the core Patch semantics.
type patchRequest struct {
	Title *string `json:"title"`
	Done  *bool   `json:"done"`
}

// todoResponse is the wire representation of a todo.
type todoResponse struct {
	ID        string    `json:"id"`
	Title     string    `json:"title"`
	Done      bool      `json:"done"`
	CreatedAt time.Time `json:"created_at"`
	UpdatedAt time.Time `json:"updated_at"`
}

type listResponse struct {
	Items  []todoResponse `json:"items"`
	Limit  int            `json:"limit"`
	Offset int            `json:"offset"`
}

func toResponse(t core.Todo) todoResponse {
	return todoResponse{
		ID:        t.ID,
		Title:     t.Title,
		Done:      t.Done,
		CreatedAt: t.CreatedAt,
		UpdatedAt: t.UpdatedAt,
	}
}

func toResponses(ts []core.Todo) []todoResponse {
	out := make([]todoResponse, 0, len(ts))
	for _, t := range ts {
		out = append(out, toResponse(t))
	}
	return out
}
