# agentic-sandbox

A small Go **Todos HTTP API** structured so that code you want to **trust an AI
agent to write** can be trusted by construction — using a **functional core /
imperative shell** architecture with **effects as data**, where the safety
properties are enforced by the compiler and `go test`, not by convention.

The premise: you don't *trust* agent-written code — you **confine and observe**
it:

- **Confine** — business logic lives in a pure `core` that physically cannot do
  I/O (enforced at build time by a test and a linter).
- **Gate** — every side effect passes one choke point that can refuse it, and
  the choke point cannot be bypassed (enforced by the type system).
- **Observe** — effects are plain data, so what the code intends to do is
  inspectable before it runs.

Persistence is real: todos live in Postgres, reached through a repository that
sits **behind** the gate. See
[docs/adr/0002](docs/adr/0002-todos-persisted-through-a-repository.md) for how a
database fits the model without eroding it.

## Layout

```
cmd/server         wiring: config -> connect -> migrate -> serve (graceful shutdown)
internal/core      PURE. Decides what should happen; returns []Effect. No I/O.
internal/shell     the ONLY place effects run; owns the gate, the Postgres repo,
                   and the embedded migrations.
internal/api       HTTP layer: routing, JSON, middleware. No business logic.
internal/config    env-driven configuration.
```

## The flow

Every write follows the same three steps:

```go
effects, err := core.Create(id, input)   // 1. PLAN  (pure)   — returns []Effect
vetted,  err := shell.Gate(effects)       // 2. GATE  (trusted)— returns a Vetted, or refuses
err          := shell.Run(ctx, repo, log, // 3. RUN   (impure) — accepts only a Vetted
                          vetted)
```

An effect is just data describing an intended action:

```go
type Effect interface{ isEffect() }
type Log        struct{ Line string }
type StoreTodo  struct{ Todo Todo }   // upsert
type DeleteTodo struct{ ID string }
```

**Reads** (get / list) don't fit the write pipeline — loading a row is I/O. So
the shell's repository reads, and hands the result *into* pure core as plain
data (e.g. `core.Update(current, patch)`). Core stays deterministic.

## API

| Method & path        | Purpose                        | Success |
|----------------------|--------------------------------|---------|
| `GET /healthz`       | liveness + DB ping             | 200 |
| `GET /todos`         | list (`?done=`, `?limit=`, `?offset=`) | 200 |
| `POST /todos`        | create `{"title":"…"}`         | 201 |
| `GET /todos/{id}`    | fetch one                      | 200 |
| `PATCH /todos/{id}`  | partial update `{"title"?,"done"?}` | 200 |
| `DELETE /todos/{id}` | delete                         | 204 |

Errors return `{"error":{"code":"…","message":"…"}}`:
`422` validation/guardrail, `400` malformed request, `404` not found, `503` DB
down. Every response carries an `X-Request-ID`; the server logs one structured
line per request.

## Run it

```sh
make db-up          # Postgres on :5432, pgAdmin on :8081
make run            # migrations run on startup, then serves :8080

curl -s localhost:8080/todos -d '{"title":"buy milk"}'          # 201, created
curl -s localhost:8080/todos                                    # list
curl -s -X PATCH localhost:8080/todos/<id> -d '{"done":true}'   # 200, updated
curl -s -X DELETE localhost:8080/todos/<id>                     # 204

curl -s localhost:8080/todos -d '{"title":"  "}'   # 422, core rejects (validation)
curl -s localhost:8080/todos -d '{"nope":1}'       # 400, unknown field
```

`DATABASE_URL` overrides the connection (defaults to the docker-compose Postgres);
`PORT` overrides `:8080`.

## How the guarantees are enforced

| Property | Mechanism | Where |
|---|---|---|
| Core does no I/O | `TestCoreIsPure` parses core and fails on a forbidden import | `go test` |
| Core does no I/O (also) | `depguard` denies `os`/`net`/`database/sql`/… in core | `golangci-lint` |
| Writes can't run ungated | `Run` accepts only `Vetted`; only `Gate` can mint one, and its field is unexported so nothing can forge it | compiler |
| `Run` handles every effect | `TestRunHandlesEveryEffect` compares core's effect types to `Run`'s switch cases | `go test` |

Try any of them: add `import "database/sql"` to a core file, or call
`shell.Run` with a raw slice, or add an effect without a `Run` case — each fails
the build.

## Where policies go: validation vs. guardrails

One question places every rule — *does the code want this rule for its own
correctness, or is it a limit imposed on the code?*

- **Validation** the logic wants (title not empty) → **`core`**, close to the
  logic, pure and testable. Maps to `422`.
- **Guardrails** imposed on the logic (title length cap, and — in general — no
  prod deletes, quotas) → **trusted `shell`**, so untrusted core can't weaken or
  bypass them. Added as a small `Policy` func in `internal/shell/gate.go`.

## Database & migrations

Postgres is now the app's real store, reached only through
`internal/shell/repo.go`. The schema is owned by the app: migrations in
`internal/shell/migrations/*.sql` are embedded and applied on startup (tracked
in `schema_migrations`), idempotently.

```sh
make db-up      # start Postgres + pgAdmin
make migrate    # apply migrations and exit (also runs automatically on `make run`)
make db-reset   # wipe volumes; migrations rebuild the schema on next start
```

`db/init.sql` still seeds a throwaway `users` table so pgAdmin has something to
show on first boot — it is a demo, not the app's schema. pgAdmin is at
**http://localhost:8081** (`admin@example.com` / `admin`). All credentials are
hardcoded dev-only values.

## Commands

```sh
make run       # run the API (needs Postgres; migrations run on startup)
make migrate   # apply migrations and exit
make test      # unit tests + purity guard + exhaustiveness guard (DB tests skip if none)
make test-int  # integration tests against a live database
make lint      # golangci-lint (depguard)
make check     # test + lint
```

## Adding an effect

1. struct + `isEffect()` in `internal/core/effect.go`
2. return it from a core function
3. handle it in `internal/shell/run.go`'s `Run` switch

The exhaustiveness test reminds you if you skip step 3.
