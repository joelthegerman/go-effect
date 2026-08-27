// Package shell is the imperative shell: the ONLY package that performs I/O and
// the ONLY place effects are executed. It owns the gate (so gating is not
// optional — the executor structurally cannot run ungated effects), the
// Postgres-backed repository, and the migration runner.
package shell

import (
	"fmt"

	"agentic-sandbox/internal/core"
)

// Vetted is a batch of effects that has passed the gate.
//
// Its field is UNEXPORTED, so the only way to obtain a populated Vetted is to
// call Gate. Run requires a Vetted — therefore effects cannot be executed
// without being gated first, and the COMPILER enforces it.
type Vetted struct {
	effects []core.Effect
}

// GuardrailError is returned when a policy refuses an effect. It's a distinct
// type so the API layer can map it to an HTTP status without string-matching.
type GuardrailError struct{ Reason string }

func (e GuardrailError) Error() string { return e.Reason }

// Policy inspects one effect and blocks it by returning an error. Each policy
// cares only about the effect types it wants and returns nil for the rest.
//
// This is how the gate SCALES: a new guardrail is a small function added to
// `policies` — no existing code changes, and each policy is unit-tested in
// isolation. The gate never becomes one growing switch.
type Policy func(core.Effect) error

// maxTodoTitle is a GUARDRAIL: a limit imposed on the (untrusted) core, not a
// rule the logic wants for its own correctness. So it lives here in the gate
// where core can't weaken or bypass it — unlike the empty-title check, which is
// validation and lives in core.
const maxTodoTitle = 280

var policies = []Policy{
	noOversizeTodoTitle,
}

func noOversizeTodoTitle(e core.Effect) error {
	if st, ok := e.(core.StoreTodo); ok && len(st.Todo.Title) > maxTodoTitle {
		return GuardrailError{Reason: fmt.Sprintf("title exceeds %d characters", maxTodoTitle)}
	}
	return nil
}

// Gate runs every policy against every effect. On the first failure nothing is
// vetted; on success it returns the Vetted token that Run requires.
func Gate(effects []core.Effect) (Vetted, error) {
	for _, e := range effects {
		for _, p := range policies {
			if err := p(e); err != nil {
				return Vetted{}, err
			}
		}
	}
	return Vetted{effects: effects}, nil
}
