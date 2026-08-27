package shell

import (
	"context"
	"errors"
	"fmt"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"

	"agentic-sandbox/internal/core"
)

// ErrNotFound is returned by reads (and by Delete) when a todo does not exist.
// The API layer maps it to HTTP 404.
var ErrNotFound = errors.New("todo not found")

// Repo is the shell's Postgres-backed persistence: the ONLY thing that touches
// the database. Reads are plain methods; the write methods (Upsert/Delete)
// implement TodoWriter and are called ONLY by Run, so the gate stays mandatory.
type Repo struct {
	pool *pgxpool.Pool
}

func NewRepo(pool *pgxpool.Pool) *Repo { return &Repo{pool: pool} }

// Ping reports whether the database is reachable (used by /healthz).
func (r *Repo) Ping(ctx context.Context) error { return r.pool.Ping(ctx) }

// ListParams controls a List query. Done nil means "any".
type ListParams struct {
	Done   *bool
	Limit  int
	Offset int
}

const selectColumns = `id, title, done, created_at, updated_at`

// List returns todos newest-first. This is the READ half of the workflow; it
// lives in the shell because reading is I/O, and the API hands the result
// straight back to the client (or into pure core) as plain data.
func (r *Repo) List(ctx context.Context, p ListParams) ([]core.Todo, error) {
	rows, err := r.pool.Query(ctx, `
		SELECT `+selectColumns+`
		FROM todos
		WHERE ($1::bool IS NULL OR done = $1)
		ORDER BY created_at DESC, id
		LIMIT $2 OFFSET $3`, p.Done, p.Limit, p.Offset)
	if err != nil {
		return nil, fmt.Errorf("list todos: %w", err)
	}
	defer rows.Close()

	todos := make([]core.Todo, 0)
	for rows.Next() {
		t, err := scanTodo(rows)
		if err != nil {
			return nil, err
		}
		todos = append(todos, t)
	}
	return todos, rows.Err()
}

// Get returns one todo or ErrNotFound.
func (r *Repo) Get(ctx context.Context, id string) (core.Todo, error) {
	row := r.pool.QueryRow(ctx, `SELECT `+selectColumns+` FROM todos WHERE id = $1`, id)
	t, err := scanTodo(row)
	if errors.Is(err, pgx.ErrNoRows) {
		return core.Todo{}, ErrNotFound
	}
	if err != nil {
		return core.Todo{}, fmt.Errorf("get todo: %w", err)
	}
	return t, nil
}

// --- write side: implements TodoWriter; only Run calls these ----------------

// Upsert inserts a new todo or updates an existing one by id. created_at is set
// by the database on insert and preserved on update; updated_at is refreshed.
func (r *Repo) Upsert(ctx context.Context, t core.Todo) error {
	_, err := r.pool.Exec(ctx, `
		INSERT INTO todos (id, title, done)
		VALUES ($1, $2, $3)
		ON CONFLICT (id) DO UPDATE
		SET title = EXCLUDED.title, done = EXCLUDED.done, updated_at = now()`,
		t.ID, t.Title, t.Done)
	if err != nil {
		return fmt.Errorf("upsert todo %s: %w", t.ID, err)
	}
	return nil
}

// Delete removes a todo, returning ErrNotFound if no row matched.
func (r *Repo) Delete(ctx context.Context, id string) error {
	tag, err := r.pool.Exec(ctx, `DELETE FROM todos WHERE id = $1`, id)
	if err != nil {
		return fmt.Errorf("delete todo %s: %w", id, err)
	}
	if tag.RowsAffected() == 0 {
		return ErrNotFound
	}
	return nil
}

type rowScanner interface{ Scan(dest ...any) error }

func scanTodo(s rowScanner) (core.Todo, error) {
	var t core.Todo
	err := s.Scan(&t.ID, &t.Title, &t.Done, &t.CreatedAt, &t.UpdatedAt)
	return t, err
}
