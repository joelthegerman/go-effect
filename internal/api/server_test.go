package api

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
	"agentic-sandbox/internal/shell"
)

// fakeStore is an in-memory Store so handler tests need no database. Writes in
// the real system flow through shell.Run; here the test server's runner applies
// the same effects to this map, exercising the gate for real.
type fakeStore struct {
	items map[string]core.Todo
}

func newFakeStore() *fakeStore { return &fakeStore{items: map[string]core.Todo{}} }

func (f *fakeStore) Ping(context.Context) error { return nil }

func (f *fakeStore) Get(_ context.Context, id string) (core.Todo, error) {
	t, ok := f.items[id]
	if !ok {
		return core.Todo{}, shell.ErrNotFound
	}
	return t, nil
}

func (f *fakeStore) List(_ context.Context, p shell.ListParams) ([]core.Todo, error) {
	var out []core.Todo
	for _, t := range f.items {
		if p.Done != nil && t.Done != *p.Done {
			continue
		}
		out = append(out, t)
	}
	return out, nil
}

// fakeStore also implements shell.TodoWriter so the real shell.Run drives it,
// exercising the gate and executor without a database.
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
		return shell.ErrNotFound
	}
	delete(f.items, id)
	return nil
}

func applyToFake(store *fakeStore, v shell.Vetted) error {
	return shell.Run(context.Background(), store, slog.New(slog.NewTextHandler(io.Discard, nil)), v)
}

// testServer builds a Server whose runner applies gated effects to the fake
// store, so the real Gate runs but no Postgres is involved.
func testServer(t *testing.T) (*Server, *fakeStore) {
	t.Helper()
	store := newFakeStore()
	s := &Server{
		store: store,
		log:   slog.New(slog.NewTextHandler(io.Discard, nil)),
	}
	s.run = func(_ context.Context, v shell.Vetted) error {
		return applyToFake(store, v)
	}
	return s, store
}

func do(t *testing.T, s *Server, method, target, body string) *httptest.ResponseRecorder {
	t.Helper()
	var r *http.Request
	if body == "" {
		r = httptest.NewRequest(method, target, nil)
	} else {
		r = httptest.NewRequest(method, target, strings.NewReader(body))
	}
	rec := httptest.NewRecorder()
	s.Handler().ServeHTTP(rec, r)
	return rec
}

func TestCreateGetUpdateDelete(t *testing.T) {
	s, _ := testServer(t)

	// create
	rec := do(t, s, http.MethodPost, "/todos", `{"title":"buy milk"}`)
	if rec.Code != http.StatusCreated {
		t.Fatalf("create status = %d, body=%s", rec.Code, rec.Body)
	}
	var created todoResponse
	mustJSON(t, rec.Body.Bytes(), &created)
	if created.ID == "" || created.Title != "buy milk" || created.Done {
		t.Fatalf("bad created todo: %#v", created)
	}

	// get
	rec = do(t, s, http.MethodGet, "/todos/"+created.ID, "")
	if rec.Code != http.StatusOK {
		t.Fatalf("get status = %d", rec.Code)
	}

	// update
	rec = do(t, s, http.MethodPatch, "/todos/"+created.ID, `{"done":true}`)
	if rec.Code != http.StatusOK {
		t.Fatalf("update status = %d, body=%s", rec.Code, rec.Body)
	}
	var updated todoResponse
	mustJSON(t, rec.Body.Bytes(), &updated)
	if !updated.Done || updated.Title != "buy milk" {
		t.Fatalf("update did not preserve/patch correctly: %#v", updated)
	}

	// delete
	rec = do(t, s, http.MethodDelete, "/todos/"+created.ID, "")
	if rec.Code != http.StatusNoContent {
		t.Fatalf("delete status = %d", rec.Code)
	}
	rec = do(t, s, http.MethodGet, "/todos/"+created.ID, "")
	if rec.Code != http.StatusNotFound {
		t.Fatalf("get after delete = %d, want 404", rec.Code)
	}
}

func TestCreateEmptyTitleIs422(t *testing.T) {
	s, _ := testServer(t)
	rec := do(t, s, http.MethodPost, "/todos", `{"title":"   "}`)
	if rec.Code != http.StatusUnprocessableEntity {
		t.Fatalf("empty title status = %d, want 422", rec.Code)
	}
}

func TestOversizeTitleIs422Guardrail(t *testing.T) {
	s, _ := testServer(t)
	big := `{"title":"` + strings.Repeat("x", 500) + `"}`
	rec := do(t, s, http.MethodPost, "/todos", big)
	if rec.Code != http.StatusUnprocessableEntity {
		t.Fatalf("oversize title status = %d, want 422 from the gate", rec.Code)
	}
}

func TestUnknownFieldIs400(t *testing.T) {
	s, _ := testServer(t)
	rec := do(t, s, http.MethodPost, "/todos", `{"titel":"typo"}`)
	if rec.Code != http.StatusBadRequest {
		t.Fatalf("unknown field status = %d, want 400", rec.Code)
	}
}

func TestGetMissingIs404(t *testing.T) {
	s, _ := testServer(t)
	rec := do(t, s, http.MethodGet, "/todos/does-not-exist", "")
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
