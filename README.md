# agentic-sandbox

A small Go HTTP service that demonstrates how to structure code you want to
**trust an AI agent to write** — using a **functional core / imperative shell**
architecture with **effects as data**, where the safety properties are
enforced by the compiler and `go test`, not by convention or review.

The premise: you don't *trust* agent-written code — you **confine and observe**
it. This repo turns that into three mechanical properties:

- **Confine** — business logic lives in a pure `core` that physically cannot do
  I/O (enforced at build time).
- **Gate** — every side effect passes one choke point that can refuse it, and
  the choke point cannot be bypassed (enforced by the type system).
- **Observe** — effects are plain data, so what the code intends to do is
  inspectable before it runs.

## Layout

```
cmd/server         wiring: plan -> gate -> run (holds no logic)
internal/core      PURE. Decides what should happen; returns []Effect. No I/O.
internal/shell     the ONLY place effects run; owns the gate and its policies.
```

## The flow

```go
effects, err := core.Signup(email)   // 1. PLAN  (pure)  — returns []Effect
vetted, err  := shell.Gate(effects)  // 2. GATE  (trusted) — returns a Vetted token, or refuses
shell.Run(vetted)                    // 3. RUN   (impure) — accepts only Vetted
```

An effect is just data describing an intended action:

```go
type Effect interface{ isEffect() }
type Log       struct{ Line string }
type SendEmail struct{ To, Body string }
```

## Run it

```sh
make run    # :8080 (override with PORT=9090 make run)

curl -X POST 'http://localhost:8080/signup?email=ada@example.com'    # 200, effects run
curl -X POST 'http://localhost:8080/signup?email=evil@attacker.com'  # 403, gate refuses, nothing runs
curl -X POST 'http://localhost:8080/signup?email=nope'               # 422, core rejects
```

The shell's effects print to the server's stdout. A blocked or invalid request
runs nothing.

## How the guarantees are enforced

| Property | Mechanism | Where |
|---|---|---|
| Core does no I/O | `TestCoreIsPure` parses core and fails on a forbidden import | `go test` |
| Core does no I/O (also) | `depguard` denies `os`/`net`/… in core | `golangci-lint` |
| Effects can't run ungated | `Run` accepts only `Vetted`; only `Gate` (trusted `shell`) can mint one, and its field is unexported so nothing can forge it | compiler |
| `Run` handles every effect | `TestRunHandlesEveryEffect` compares core's effect types to `Run`'s switch cases | `go test` |

Try any of them: add `import "os"` to a core file, or call `shell.Run(effects)`
with a raw slice, or add an effect without a `Run` case — each fails the build.

## Where policies go: validation vs. guardrails

Two kinds of "policy", placed by a single question — *does the code want this
rule for its own correctness, or is it a limit imposed on the code?*

- **Validation** the logic wants (valid email, positive amount) → **`core`**,
  close to the logic, pure and testable. (e.g. the `@` check in `core.Signup`.)
- **Guardrails** imposed on the logic (never email outsiders, never delete in
  prod, spend caps) → **trusted `shell`**, so untrusted core can't weaken or
  bypass them. (e.g. `noExternalEmail` in `shell`.)

The gate scales by adding a small `Policy` func to the `policies` slice in
`shell` — never a growing switch. The executor's switch stays one case per
effect, kept exhaustive by the test above.

## Commands

```sh
make run     # run the app
make test    # unit tests + purity guard + exhaustiveness guard
make lint    # golangci-lint (depguard); needs golangci-lint on PATH
make check   # test + lint
```

## Adding an effect

1. struct + `isEffect()` in `internal/core/effect.go`
2. return it from a core function
3. handle it in `internal/shell/shell.go`'s `Run` switch

The exhaustiveness test reminds you if you skip step 3.
