-- These are the ONLY queries against the todos table. sqlc reads this file and
-- generates type-safe Go in internal/shell/sqlcgen; repo.go wraps that code and
-- maps rows to/from core.Todo. To add or change a query, edit here and run
-- `make sqlc`.

-- name: GetTodo :one
SELECT id, title, done, created_at, updated_at
FROM todos
WHERE id = $1;

-- name: ListTodos :many
SELECT id, title, done, created_at, updated_at
FROM todos
WHERE (sqlc.narg('done')::boolean IS NULL OR done = sqlc.narg('done'))
ORDER BY created_at DESC, id
LIMIT sqlc.arg('limit') OFFSET sqlc.arg('offset');

-- name: UpsertTodo :exec
INSERT INTO todos (id, title, done)
VALUES ($1, $2, $3)
ON CONFLICT (id) DO UPDATE
SET title = EXCLUDED.title, done = EXCLUDED.done, updated_at = now();

-- name: DeleteTodo :execrows
DELETE FROM todos WHERE id = $1;
