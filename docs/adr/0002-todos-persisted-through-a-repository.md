# 2. Todos are persisted in Postgres through a repository behind the gate

Date: 2026-08-27

## Status

Accepted. Supersedes [0001](0001-sandbox-db-outside-the-pipeline.md).

## Context

ADR 0001 kept the sandbox Postgres beside the app so the "every side effect goes
through the gate" story stayed undiluted, and explicitly said that persisting
data would be a separate, deliberate change. We now want a real, production-shaped
Todos API backed by that database. This ADR makes the change and records how it
preserves the architecture instead of eroding it.

The tension a database introduces: the pipeline models side effects as data
planned in the **pure** `core` and executed in the `shell`. Writes fit that
directly. **Reads** do not — `core` cannot load a row, because loading is I/O.

## Decision

Persistence lives entirely in `internal/shell`, behind a repository, and reaches
the database only through the existing gate.

**Writes stay effects-as-data.** `core` returns `StoreTodo` / `DeleteTodo`
effects (plain data). Only `shell.Run` executes them, and `Run` requires a
`Vetted` — so every write still passes the gate. `Run` drives a small
`TodoWriter` interface (`Upsert` / `Delete`), which the Postgres `Repo`
implements and tests fake.

**Reads happen in the shell, then flow *into* pure core as data.** The repository
exposes `List` / `Get`. For an edit, the API handler loads the current todo via
the repo and passes it to `core.Update`, which decides against it (exists?
preserve `Done`?) without doing any I/O. `core` stays deterministic: same input,
same output.

**Impure inputs are injected, not reached for.** IDs come from `shell.NewID`
(a UUIDv4); timestamps are assigned by the database. `core` never calls the
clock or a random source.

**Schema is owned by the app.** Migrations in `internal/shell/migrations` are
embedded and applied on startup, tracked in `schema_migrations`. This is separate
from `db/init.sql`, which remains a throwaway pgAdmin demo (the `users` table),
not the app's schema.

**The validation/guardrail split is unchanged.** "Title must not be empty" is
validation the logic wants → `core` (HTTP 422). "Title must be ≤ N characters"
is a guardrail imposed on the code → a gate policy in `shell` (HTTP 422 via
`GuardrailError`).

## Consequences

- The gate remains mandatory for writes: `Run` takes only a `Vetted`, whose
  effect slice is unexported, so no layer can execute an ungated write.
- Swapping Postgres for another store, or adding caching, touches only the
  repository — `core` and the API are unaffected.
- Reads are outside the effect log by design. What the app *changes* is still
  fully described by effects; what it *observes* is a plain repository call.
- Two extra reads exist on create/update (a `Get` after the write) to return the
  database's canonical row, including its timestamps. Acceptable for this shape;
  a `RETURNING`-based fast path is a future optimization.
- Multi-write atomicity is not needed today (every todo batch is a single write
  plus an audit log). If a future batch spans multiple writes, `Run` should wrap
  them in one transaction; the `TodoWriter` seam leaves room for that.
- `db/init.sql`'s demo table now coexists with the real `todos` table in the
  same database. Wiping volumes (`make db-reset`) drops both; migrations rebuild
  the app's schema on next start.
