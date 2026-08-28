package kernel

import "agentic-sandbox/internal/core"

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
// Policies are GUARDRAILS imposed on the (untrusted) core, so each feature
// defines its own in its shell package and hands them to the gate at wiring
// time — the gate itself stays feature-agnostic and never grows a switch.
type Policy func(core.Effect) error

// Gate runs every policy against every effect. On the first failure nothing is
// vetted; on success it returns the Vetted token that Run requires.
func Gate(policies []Policy, effects []core.Effect) (Vetted, error) {
	for _, e := range effects {
		for _, p := range policies {
			if err := p(e); err != nil {
				return Vetted{}, err
			}
		}
	}
	return Vetted{effects: effects}, nil
}
