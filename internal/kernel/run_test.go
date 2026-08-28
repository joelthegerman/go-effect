package kernel

import (
	"context"
	"errors"
	"io"
	"log/slog"
	"testing"

	"agentic-sandbox/internal/core"
)

// testLogger discards audit output during tests.
var testLogger = slog.New(slog.NewTextHandler(io.Discard, nil))

// errMissing is a stand-in for whatever a real writer returns on a failed
// write; the atomicity test only cares that some effect fails, not which error.
var errMissing = errors.New("missing")

// memStore is an in-memory UnitOfWork used to test Run without a database. Its
// WithTx models a real transaction: fn writes into a staging buffer that is
// merged into the committed map only if fn succeeds, and discarded (rolled
// back) otherwise — which is exactly the atomicity property Run relies on.
type memStore struct {
	committed  map[string]core.Todo
	failDelete map[string]bool // ids whose Delete should fail
}

func newMemStore() *memStore {
	return &memStore{committed: map[string]core.Todo{}, failDelete: map[string]bool{}}
}

func (m *memStore) WithTx(_ context.Context, fn func(TodoWriter) error) error {
	st := &staged{base: m, puts: map[string]core.Todo{}, dels: map[string]bool{}}
	if err := fn(st); err != nil {
		return err // rollback: nothing is merged into committed
	}
	for id, t := range st.puts {
		m.committed[id] = t
	}
	for id := range st.dels {
		delete(m.committed, id)
	}
	return nil
}

// staged buffers a transaction's writes until WithTx commits them.
type staged struct {
	base *memStore
	puts map[string]core.Todo
	dels map[string]bool
}

func (s *staged) Upsert(_ context.Context, t core.Todo) error {
	s.puts[t.ID] = t
	delete(s.dels, t.ID)
	return nil
}

func (s *staged) Delete(_ context.Context, id string) error {
	if s.base.failDelete[id] {
		return errMissing
	}
	_, inBase := s.base.committed[id]
	_, inStaged := s.puts[id]
	if !inBase && !inStaged {
		return errMissing
	}
	s.dels[id] = true
	delete(s.puts, id)
	return nil
}

// TestRunIsAtomic is the guarantee the transactional executor adds: if any
// effect in a batch fails, earlier writes in the same batch do not persist.
func TestRunIsAtomic(t *testing.T) {
	m := newMemStore()
	m.failDelete["missing"] = true

	id := "11111111-1111-4111-8111-111111111111"
	effects := []core.Effect{
		core.StoreTodo{Todo: core.Todo{ID: id, Title: "should not persist"}},
		core.DeleteTodo{ID: "missing"}, // fails -> whole batch rolls back
	}
	v, err := Gate(nil, effects)
	if err != nil {
		t.Fatalf("gate: %v", err)
	}
	if err := Run(context.Background(), m, testLogger, v); !errors.Is(err, errMissing) {
		t.Fatalf("Run err = %v, want errMissing", err)
	}
	if _, ok := m.committed[id]; ok {
		t.Fatal("StoreTodo was committed even though a later effect failed — Run is not atomic")
	}
}

// TestRunCommitsWholeBatch is the happy path: with no failure, every write in
// the batch is committed together.
func TestRunCommitsWholeBatch(t *testing.T) {
	m := newMemStore()
	id := "22222222-2222-4222-8222-222222222222"
	effects := []core.Effect{
		core.StoreTodo{Todo: core.Todo{ID: id, Title: "keep"}},
		core.Log{Line: "todo.create " + id},
	}
	v, err := Gate(nil, effects)
	if err != nil {
		t.Fatalf("gate: %v", err)
	}
	if err := Run(context.Background(), m, testLogger, v); err != nil {
		t.Fatalf("run: %v", err)
	}
	if got, ok := m.committed[id]; !ok || got.Title != "keep" {
		t.Fatalf("batch did not commit as expected: %#v", m.committed)
	}
}
