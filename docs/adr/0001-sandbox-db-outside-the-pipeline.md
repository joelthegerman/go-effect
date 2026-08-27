# 1. The sandbox database sits outside the effect pipeline

Date: 2026-08-25

## Status

Superseded by [0002](0002-todos-persisted-through-a-repository.md) (2026-08-27).
The app now persists todos in Postgres via a repository behind the effect gate,
exactly the "separate, deliberate change" the Consequences section anticipated.

## Context

This repo's central thesis is that **every side effect is data** flowing
`core → gate → shell`, and `shell` is the only place I/O happens. A database
write is, by that logic, a side effect: it "should" be modelled as an `Effect`
(e.g. `InsertUser`), planned in `core`, screened by the gate, and executed in a
`Run` case that owns the connection.

We wanted a real Postgres available in the sandbox — plus pgAdmin — to poke at
a live database. The question was whether to wire it through the pipeline or
stand it up beside the app.

## Decision

The sandbox Postgres + pgAdmin stack (`docker-compose.yml`, `db/`) is
**infra only**. It is intentionally **not** connected to the Go application:

- `internal/core` returns no persistence effects and does not know the DB exists.
- `internal/shell` opens no database connection; the gate screens no DB effects.
- `db/init.sql` seeds an illustrative `users` table that is **not** the app's
  schema. Nothing in the app reads or writes it.

## Consequences

- **A future reader will be surprised** — this ADR exists precisely because a
  Postgres the app ignores looks like a loose end. It is deliberate.
- The demonstration stays honest: the app's I/O story is still "everything goes
  through the gate," undiluted by a persistence layer we didn't design.
- The database is a scratchpad. Wiping it (`make db-reset`) affects nothing in
  the app.
- **If we later want signup to persist,** that is a separate, deliberate change:
  add an `InsertUser` effect, a `core` function that returns it, a gate policy
  if the write needs guarding, and a `Run` case holding the pool. That work
  would supersede this ADR.
