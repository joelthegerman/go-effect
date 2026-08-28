# Adding a feature

The codebase is a small **framework kernel** plus **feature slices**. You spend
almost all your time in a feature slice; you rarely touch the kernel.

Every feature is three thin, same-named packages plus a wiring block:

| Layer | Package | What lives there |
|-------|---------|------------------|
| pure logic | `internal/core/<feature>/` | decisions: validate input, compute next state, return `[]core.Effect` |
| infrastructure | `internal/shell/<feature>/` | repository (sqlc), guardrail `Policies()`, `queries.sql`, `migrations/` |
| HTTP | `internal/api/<feature>/` | handlers + DTOs |
| wiring | `cmd/server/main.go` | one block: repo → pipeline → handlers → routes |

The shared, sealed vocabulary all features speak lives in **`internal/core`**:
the `Effect` interface + effect structs (`effect.go`), entity types
(`todo.go`), and `ValidationError` (`errors.go`).

## The one organizing question

> **Is this a decision (what should happen) or an effect (making it happen)?**

- A **decision** — validating, computing the next state, choosing effects — is
  pure. It goes in **`core/<feature>`**.
- An **effect** — a DB write, a log line — is performed only by the kernel
  executor, and only after passing the gate. Feature code just *returns* effects.

When in doubt, start in `core/<feature>`; if you reach for I/O, you're in the
wrong package — return an `Effect` instead.

## Where does my code go?

| I want to…                                    | Edit                                                       |
|-----------------------------------------------|------------------------------------------------------------|
| Reject bad input the logic itself won't allow | `core/<feature>` (validation) → maps to 422 |
| Impose a limit *on* the logic (quota, cap)    | `shell/<feature>/policy.go` — a `kernel.Policy` in `Policies()` |
| Change what state a write produces            | `core/<feature>` (`Create`/`Update`/`Delete`) |
| Add a new kind of side effect                 | `core/effect.go` **+** `internal/kernel/run.go` (see below) |
| Add or change a SQL query                     | `shell/<feature>/queries.sql` → `make sqlc` |
| Add a column / change the schema              | new `shell/<feature>/migrations/NNNN_*.sql` |
| Add an HTTP route or change JSON shape        | `api/<feature>` (`handlers.go`, `dto.go`) |

## Worked example: add a `priority` field to todos

1. **Schema** — new migration
   `internal/shell/todos/migrations/0002_todo_priority.sql`:
   ```sql
   ALTER TABLE todos ADD COLUMN priority INT NOT NULL DEFAULT 0;
   ```
2. **Queries** — add `priority` to the SELECT/INSERT columns in
   `internal/shell/todos/queries.sql`, then `make sqlc`. `sqlcgen.Todo` now has
   a `Priority` field.
3. **Entity** — add `Priority int` to `core.Todo` (shared vocabulary), and map
   it in `toCoreTodo` / `UpsertTodoParams` in `internal/shell/todos/repo.go`.
4. **Decision** — accept and validate it in `core/todos` `Create`/`Update`
   (e.g. clamp to 0–3). Validation lives in core.
5. **Guardrail?** — "priority may never exceed 3, even from buggy core" is a
   `kernel.Policy` in `internal/shell/todos/policy.go`, not a core check.
6. **API** — expose it in `internal/api/todos/dto.go`.

No handler or kernel wiring changes.

## Worked example: add a whole new feature (`projects`)

1. `internal/core/projects/projects.go` — `package projects`: the pure logic
   (`Create`/`Update`/…) returning `[]core.Effect`.
2. `internal/core/` — add the entity (`Project`) and its effect structs
   (`StoreProject`, `DeleteProject`) to the shared vocabulary. Effect structs go
   *here*, not in `core/projects`, because they carry the entity and must sit
   with the sealed `Effect` interface (a cross-package unexported method is
   impossible in Go).
3. `internal/kernel/run.go` — add `case core.StoreProject:` / `DeleteProject` to
   the `Run` switch (and the writer method it calls). `TestRunHandlesEveryEffect`
   fails the build until you do.
4. `internal/shell/projects/` — repository (`queries.sql` + `make sqlc`),
   `migrations/`, and `Policies()` / `Migrations()`.
5. `internal/api/projects/` — handlers + DTOs.
6. `cmd/server/main.go` — a wiring block mirroring todos: `NewRepo` →
   `kernel.NewPipeline` → `NewHandlers` → `Register`, plus `kernel.Migrate` with
   the feature's `Migrations()`.

## Adding a new effect (mechanics)

1. **Declare** it as data in `internal/core/effect.go`:
   ```go
   type RecordAudit struct{ Action, TodoID string }
   func (RecordAudit) isEffect() {}
   ```
2. **Emit** it from a `core/<feature>` function.
3. **Interpret** it in `internal/kernel/run.go`'s switch (add a repository
   method + query for a new kind of write).

`TestRunHandlesEveryEffect` fails the build if you skip step 3.

## Before you're done

Run `make check` (tests + lint). The always-on architecture tests:

- `TestCoreIsPure` — core and every `core/<feature>` may import only an
  allowlist of pure packages and may not transitively reach any first-party
  package that does I/O.
- `TestRunHandlesEveryEffect` — every `core.Effect` has a case in `kernel.Run`.
