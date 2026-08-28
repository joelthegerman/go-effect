package todos

import (
	"fmt"

	"agentic-sandbox/internal/core"
	"agentic-sandbox/internal/kernel"
)

// maxTitle is a GUARDRAIL: a limit imposed on the (untrusted) core, not a rule
// the logic wants for its own correctness. So it lives here in the feature's
// shell — where core can't weaken or bypass it — unlike the empty-title check,
// which is validation and lives in core/todos.
const maxTitle = 280

// Policies are the todos guardrails. Wiring hands them to the gate, so the gate
// stays feature-agnostic and each guardrail is a small, unit-testable function.
func Policies() []kernel.Policy {
	return []kernel.Policy{noOversizeTitle}
}

func noOversizeTitle(e core.Effect) error {
	if st, ok := e.(core.StoreTodo); ok && len(st.Todo.Title) > maxTitle {
		return kernel.GuardrailError{Reason: fmt.Sprintf("title exceeds %d characters", maxTitle)}
	}
	return nil
}
