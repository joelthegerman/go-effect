package todos

import (
	"context"
	"encoding/json"
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"agentic-sandbox/internal/core"
	"agentic-sandbox/internal/kernel"
	store "agentic-sandbox/internal/shell/todos"
)

// fakeStore is an in-memory Reader + kernel.UnitOfWork so handler tests need no
// database. Writes flow through the real kernel.Pipeline (gate + executor) into
// this map, exercising the gate and the todos guardrails for real.
type fakeStore struct {
	items map[string]core.Todo
}

func newFakeStore() *fakeStore { return &fakeStore{items: map[string]core.Todo{}} }

func (f *fakeStore) Get(_ context.Context, id string) (core.Todo, error) {
	t, ok := f.items[id]
	if !ok {
		return core.Todo{}, kernel.ErrNotFound
	}
	return t, nil
}

func (f *fakeStore) List(_ context.Context, p store.ListParams) ([]core.Todo, error) {
	var out []core.Todo
	for _, t := range f.items {
		if p.Done != nil && t.Done != *p.Done {
			continue
		}
		out = append(out, t)
	}
	return out, nil
}

// WithTx makes fakeStore a kernel.UnitOfWork; there is no real transaction in
// memory, so it just runs fn against itself.
func (f *fakeStore) WithTx(_ context.Context, fn func(kernel.TodoWriter) error) error {
	return fn(f)
}

func (f *fakeStore) Upsert(_ context.Context, t core.Todo) error {
	if t.CreatedAt.IsZero() {
		t.CreatedAt = time.Now()
	}
	t.UpdatedAt = time.Now()
	f.items[t.ID] = t
	return nil
}

func (f *fakeStore) Delete(_ context.Context, id string) error {
	if _, ok := f.items[id]; !ok {
		return kernel.ErrNotFound
	}
	delete(f.items, id)
	return nil
}

// testServer builds the todos handlers wired to the fake store through the real
// pipeline (real gate, real guardrails), with no Postgres involved.
func testServer(t *testing.T) (http.Handler, *fakeStore) {
	t.Helper()
	fake := newFakeStore()
	log := slog.New(slog.NewTextHandler(io.Discard, nil))
	pipe := kernel.NewPipeline(fake, log, store.Policies()...)
	h := NewHandlers(fake, pipe, log)
	mux := http.NewServeMux()
	h.Register(mux)
	return kernel.Middleware(log)(mux), fake
}

func do(t *testing.T, h http.Handler, method, target, body string) *httptest.ResponseRecorder {
	t.Helper()
	var r *http.Request
	if body == "" {
		r = httptest.NewRequest(method, target, nil)
	} else {
		r = httptest.NewRequest(method, target, strings.NewReader(body))
	}
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, r)
	return rec
}

func TestCreateGetUpdateDelete(t *testing.T) {
	h, _ := testServer(t)

	rec := do(t, h, http.MethodPost, "/todos", `{"title":"buy milk"}`)
	if rec.Code != http.StatusCreated {
		t.Fatalf("create status = %d, body=%s", rec.Code, rec.Body)
	}
	var created todoResponse
	mustJSON(t, rec.Body.Bytes(), &created)
	if created.ID == "" || created.Title != "buy milk" || created.Done {
		t.Fatalf("bad created todo: %#v", created)
	}

	rec = do(t, h, http.MethodGet, "/todos/"+created.ID, "")
	if rec.Code != http.StatusOK {
		t.Fatalf("get status = %d", rec.Code)
	}

	rec = do(t, h, http.MethodPatch, "/todos/"+created.ID, `{"done":true}`)
	if rec.Code != http.StatusOK {
		t.Fatalf("update status = %d, body=%s", rec.Code, rec.Body)
	}
	var updated todoResponse
	mustJSON(t, rec.Body.Bytes(), &updated)
	if !updated.Done || updated.Title != "buy milk" {
		t.Fatalf("update did not preserve/patch correctly: %#v", updated)
	}

	rec = do(t, h, http.MethodDelete, "/todos/"+created.ID, "")
	if rec.Code != http.StatusNoContent {
		t.Fatalf("delete status = %d", rec.Code)
	}
	rec = do(t, h, http.MethodGet, "/todos/"+created.ID, "")
	if rec.Code != http.StatusNotFound {
		t.Fatalf("get after delete = %d, want 404", rec.Code)
	}
}

func TestCreateEmptyTitleIs422(t *testing.T) {
	h, _ := testServer(t)
	rec := do(t, h, http.MethodPost, "/todos", `{"title":"   "}`)
	if rec.Code != http.StatusUnprocessableEntity {
		t.Fatalf("empty title status = %d, want 422", rec.Code)
	}
}

func TestOversizeTitleIs422Guardrail(t *testing.T) {
	h, _ := testServer(t)
	big := `{"title":"` + strings.Repeat("x", 500) + `"}`
	rec := do(t, h, http.MethodPost, "/todos", big)
	if rec.Code != http.StatusUnprocessableEntity {
		t.Fatalf("oversize title status = %d, want 422 from the gate", rec.Code)
	}
}

func TestUnknownFieldIs400(t *testing.T) {
	h, _ := testServer(t)
	rec := do(t, h, http.MethodPost, "/todos", `{"titel":"typo"}`)
	if rec.Code != http.StatusBadRequest {
		t.Fatalf("unknown field status = %d, want 400", rec.Code)
	}
}

func TestGetMissingIs404(t *testing.T) {
	h, _ := testServer(t)
	rec := do(t, h, http.MethodGet, "/todos/does-not-exist", "")
	if rec.Code != http.StatusNotFound {
		t.Fatalf("missing get = %d, want 404", rec.Code)
	}
}

func mustJSON(t *testing.T, data []byte, v any) {
	t.Helper()
	if err := json.Unmarshal(data, v); err != nil {
		t.Fatalf("unmarshal: %v (body=%s)", err, data)
	}
}
