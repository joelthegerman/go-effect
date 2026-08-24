// Package shell performs effects. This is the ONLY package that does I/O, and
// the ONLY place effects can be executed. It also owns the gate, so gating is
// not optional: the executor structurally cannot run ungated effects.
package shell

import (
	"fmt"
	"strings"

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

// Policy inspects one effect and blocks it by returning an error. Each policy
// cares only about the effect types it wants and returns nil for the rest.
//
// This is how the gate SCALES: a new rule is a small function added to
// `policies` below — no existing code changes, and each policy is unit-tested
// in isolation. The gate never becomes one growing switch.
type Policy func(core.Effect) error

var policies = []Policy{
	noExternalEmail,
	noEmptyEmailBody,
}

func noExternalEmail(e core.Effect) error {
	if se, ok := e.(core.SendEmail); ok && !strings.HasSuffix(se.To, "@example.com") {
		return fmt.Errorf("blocked external email: %s", se.To)
	}
	return nil
}

func noEmptyEmailBody(e core.Effect) error {
	if se, ok := e.(core.SendEmail); ok && strings.TrimSpace(se.Body) == "" {
		return fmt.Errorf("blocked email with empty body to %s", se.To)
	}
	return nil
}

// Gate runs every policy against every effect. On the first failure, nothing
// is vetted. On success it returns the Vetted token that Run requires.
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

// Run performs vetted effects. Adding an effect type means adding a case here;
// TestRunHandlesEveryEffect fails the build if you forget, and the default
// panic is the runtime backstop.
func Run(v Vetted) {
	for _, e := range v.effects {
		switch ef := e.(type) {
		case core.Log:
			fmt.Println("[log]", ef.Line)
		case core.SendEmail:
			fmt.Printf("[email] to=%s: %s\n", ef.To, ef.Body)
		default:
			panic(fmt.Sprintf("shell.Run: unhandled effect %T", e))
		}
	}
}
