package todos

import (
	"context"
	"errors"
	"io"
	"log/slog"
	"os"
	"testing"
	"time"

	"agentic-sandbox/internal/core"
	todo "agentic-sandbox/internal/core/todos"
	"agentic-sandbox/internal/kernel"
)

// testRepo connects to the database named by TEST_DATABASE_URL (falling back to
// DATABASE_URL, then the docker-compose default) and skips the test if nothing
// is reachable — so `go test ./...` stays green without a running Postgres.
func testRepo(t *testing.T) *Repo {
	t.Helper()
	dsn := os.Getenv("TEST_DATABASE_URL")
	if dsn == "" {
		dsn = os.Getenv("DATABASE_URL")
	}
	if dsn == "" {
		dsn = "postgres://postgres:postgres@localhost:5432/sandbox?sslmode=disable"
	}

	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()

	pool, err := kernel.Connect(ctx, dsn)
	if err != nil {
		t.Skipf("no database reachable (%v); skipping integration test", err)
	}
	t.Cleanup(pool.Close)

	if err := kernel.Migrate(ctx, pool, Migrations()); err != nil {
		t.Fatalf("migrate: %v", err)
	}
	return NewRepo(pool)
}

var testLogger = slog.New(slog.NewTextHandler(io.Discard, nil))

// gateRun gates and executes a batch of effects, failing the test on error.
// It's the test-side equivalent of an API write handler's tail, and it runs the
// real todos guardrails via Policies().
func gateRun(t *testing.T, r *Repo, effects []core.Effect) {
	t.Helper()
	vetted, err := kernel.Gate(Policies(), effects)
	if err != nil {
		t.Fatalf("gate: %v", err)
	}
	if err := kernel.Run(context.Background(), r, testLogger, vetted); err != nil {
		t.Fatalf("run: %v", err)
	}
}

func TestRepoLifecycle(t *testing.T) {
	r := testRepo(t)
	ctx := context.Background()
	id := kernel.NewID()

	// create
	created, err := todo.Create(id, todo.CreateInput{Title: "walk dog"})
	if err != nil {
		t.Fatal(err)
	}
	gateRun(t, r, created)

	got, err := r.Get(ctx, id)
	if err != nil || got.Title != "walk dog" || got.Done {
		t.Fatalf("after create: %#v, %v", got, err)
	}
	if got.CreatedAt.IsZero() {
		t.Fatal("created_at should be set by the database")
	}

	// update (toggle done, keep title)
	done := true
	updated, err := todo.Update(got, todo.Patch{Done: &done})
	if err != nil {
		t.Fatal(err)
	}
	gateRun(t, r, updated)

	got, err = r.Get(ctx, id)
	if err != nil || !got.Done || got.Title != "walk dog" {
		t.Fatalf("after update: %#v, %v", got, err)
	}

	// list finds it under done=true
	yes := true
	list, err := r.List(ctx, ListParams{Done: &yes, Limit: 100})
	if err != nil || !containsID(list, id) {
		t.Fatalf("list done=true missing %s: %v", id, err)
	}

	// delete
	del, err := todo.Delete(id)
	if err != nil {
		t.Fatal(err)
	}
	gateRun(t, r, del)

	if _, err := r.Get(ctx, id); !errors.Is(err, kernel.ErrNotFound) {
		t.Fatalf("after delete, Get should be ErrNotFound, got %v", err)
	}
}

func TestRepoDeleteMissingIsNotFound(t *testing.T) {
	r := testRepo(t)
	effects, _ := todo.Delete(kernel.NewID())
	vetted, _ := kernel.Gate(Policies(), effects)
	if err := kernel.Run(context.Background(), r, testLogger, vetted); !errors.Is(err, kernel.ErrNotFound) {
		t.Fatalf("deleting a missing todo should be ErrNotFound, got %v", err)
	}
}

func containsID(todos []core.Todo, id string) bool {
	for _, t := range todos {
		if t.ID == id {
			return true
		}
	}
	return false
}
