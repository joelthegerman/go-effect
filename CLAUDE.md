# Working in this repo

This project uses a **functional core / imperative shell** architecture with
**effects as data**, organized into **feature slices** over a small **framework
kernel**. The safety properties are machine-enforced. Follow them.

## What this project is

A small HTTP service structured so that agent-written logic can be trusted by
construction: business logic is pure and cannot do I/O, every side effect goes
through one gate that cannot be bypassed, and effects are plain data you can
inspect. See `README.md` for the full picture.

## The one rule

**All business logic goes in `internal/core`. Code there must be pure.**

A pure function returns the same output for the same input and performs no I/O
(no filesystem, network, database, clock, randomness, logging).

When core logic needs something to happen in the real world, it does **not** do
it. It returns an `Effect` value (`internal/core/effect.go`) describing what
should happen. The `internal/kernel` executor performs effects — and, with the
feature repositories it drives, is the *only* code that may.

## Layout: framework vs. features

Two kinds of code, kept apart so you rarely touch the framework while building
features:

- **`internal/kernel/`** — the framework. Setup and plumbing you don't normally
  read while developing: the gate, the executor, config, the database
  connect/migrate, HTTP middleware, and response helpers. Feature-agnostic.
- **feature slices** — each feature spans three thin, same-named packages:
  - `internal/core/<feature>/` — pure logic (**you develop here**).
  - `internal/shell/<feature>/` — infrastructure: repository, guardrail
    policies, SQL + migrations.
  - `internal/api/<feature>/` — HTTP handlers + DTOs.

`internal/core/` itself holds the shared, sealed **vocabulary** every feature
speaks: the `Effect` interface and effect structs (`effect.go`), the entity
types (`todo.go`), and `ValidationError` (`errors.go`). The entity types live
here (not in `core/<feature>/`) because effects carry them and effects must sit
with the sealed interface — see `docs/adding-a-feature.md`.

`cmd/server/` is the composition root: the one place that knows every feature
and wires each feature's shell + handlers to the kernel.

## The flow

The app is a **Todos API**. A feature's HTTP handler wires three steps per write
and holds no business logic itself:

1. **plan** — a `core/<feature>` function (`Create`, `Update`, `Delete`) returns
   `[]core.Effect` (pure).
2. **gate** — `kernel.Gate` runs the feature's policies and returns a `Vetted`
   token, or refuses. Nothing has run yet. (`kernel.Pipeline.Apply` bundles the
   gate + run for handlers.)
3. **run** — `kernel.Run` performs the effects in a single database transaction,
   so a multi-effect plan is applied all-or-nothing. It accepts only a `Vetted`,
   so the gate cannot be skipped.

**Reads** (get / list) are I/O, so they live in the feature's repository and are
handed *into* pure core as data (e.g. `todos.Update(current, patch)`). Core never
reads the world.

**New here?** `docs/adding-a-feature.md` is a task-oriented "where does my code
go" guide, including how to add a whole new feature (e.g. projects).

## Where policies go (validation vs. guardrails)

Ask: *does the code want this rule for its own correctness, or is it a limit
imposed on the code?*

- **Validation** the logic wants (valid email, positive amount) → **`core/<feature>`**.
- **Guardrails** imposed on the logic (no external email, no prod deletes,
  quotas) → the feature's **`shell/<feature>/policy.go`**, so untrusted core
  can't weaken or bypass them.

Add a guardrail by adding a small `kernel.Policy` func to the feature's
`Policies()` slice. The gate stays feature-agnostic — never grow it into a switch.

## Adding a new effect

1. Add a struct in `internal/core/effect.go` with an `isEffect()` method.
2. Return it from a `core/<feature>` function.
3. Handle it in `internal/kernel/run.go`'s `Run` switch (for a new kind of write,
   add a query to the feature's `shell/<feature>/queries.sql`, run `make sqlc`,
   and add a repository/`txWriter` method there).

`TestRunHandlesEveryEffect` fails the build if you skip step 3.

## How correctness is guaranteed

- `go test ./...` runs `TestCoreIsPure` (core and every feature's core import
  nothing side-effecting, checked transitively) and `TestRunHandlesEveryEffect`
  (the executor handles every effect). Always-on.
- `.golangci.yml` depguard enforces core purity in CI/editors too.
- The `Vetted` token (unexported field, minted only by `kernel.Gate`) makes it a
  compile error to run ungated effects.

Run `make check` before considering work done.
