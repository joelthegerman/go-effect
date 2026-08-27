# Working in this repo

This project uses a **functional core / imperative shell** architecture with
**effects as data**. The safety properties are machine-enforced. Follow them.

## What this project is

A small HTTP service structured so that agent-written logic can be trusted by
construction: business logic is pure and cannot do I/O, every side effect goes
through one gate that cannot be bypassed, and effects are plain data you can
inspect. See `README.md` for the full picture.

## The one rule

**All logic goes in `internal/core`. Code there must be pure.**

A pure function returns the same output for the same input and performs no I/O
(no filesystem, network, database, clock, randomness, logging).

When core logic needs something to happen in the real world, it does **not**
do it. It returns an `Effect` value (`internal/core/effect.go`) describing what
should happen. Package `internal/shell` performs effects — and it is the *only*
code that may.

## The flow

The app is a **Todos API**. `internal/api` handlers wire three steps per write
and hold no business logic themselves:

1. **plan** — a `core` function (`Create`, `Update`, `Delete`) returns
   `[]Effect` (pure).
2. **gate** — `shell.Gate` runs every policy and returns a `Vetted` token, or
   refuses. Nothing has run yet.
3. **run** — `shell.Run` performs the effects (writing through the Postgres
   repository). It accepts only a `Vetted`, so the gate cannot be skipped.

**Reads** (get / list) are I/O, so they live in the shell's repository and are
handed *into* pure core as data (e.g. `core.Update(current, patch)`). Core never
reads the world.

## Where things go

- `internal/core/` — pure business logic. **You write here.** Never import
  `os`, `net`, `net/http`, `database/sql`, `syscall`, `log`, etc.
- `internal/shell/` — performs effects, owns the gate, the Postgres repository,
  and the embedded migrations. Trusted, audited.
- `internal/api/` — HTTP handlers, routing, JSON, middleware. Translates HTTP
  into the plan→gate→run flow; holds no business logic.
- `internal/config/` — env-driven configuration.
- `cmd/server/` — wiring only (connect, migrate, serve, graceful shutdown).

## Where policies go (validation vs. guardrails)

Ask: *does the code want this rule for its own correctness, or is it a limit
imposed on the code?*

- **Validation** the logic wants (valid email, positive amount) → **`core`**.
- **Guardrails** imposed on the logic (no external email, no prod deletes,
  quotas) → **`shell`**, so untrusted core can't weaken or bypass them.

Add a guardrail by adding a small `Policy` func to the `policies` slice in
`internal/shell/gate.go`. Do not grow it into a switch.

## Adding a new effect

1. Add a struct in `internal/core/effect.go` with an `isEffect()` method.
2. Return it from a core function.
3. Handle it in `internal/shell/run.go`'s `Run` switch (adding a repository
   method if it's a new kind of write).

`TestRunHandlesEveryEffect` fails the build if you skip step 3.

## How correctness is guaranteed

- `go test ./...` runs `TestCoreIsPure` (core imports nothing side-effecting)
  and `TestRunHandlesEveryEffect` (executor handles every effect). Always-on.
- `.golangci.yml` depguard enforces core purity in CI/editors too.
- The `Vetted` token (unexported field, minted only by `shell.Gate`) makes it a
  compile error to run ungated effects.

Run `make check` before considering work done.
