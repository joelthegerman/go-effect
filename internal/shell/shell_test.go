package shell

import (
	"testing"

	"agentic-sandbox/internal/core"
)

func TestGateAllowsValidEmail(t *testing.T) {
	if _, err := Gate([]core.Effect{core.SendEmail{To: "ada@example.com", Body: "hi"}}); err != nil {
		t.Fatalf("valid email should pass: %v", err)
	}
}

func TestGateBlocksExternalEmail(t *testing.T) {
	if _, err := Gate([]core.Effect{core.SendEmail{To: "evil@attacker.com", Body: "hi"}}); err == nil {
		t.Fatal("external email must be blocked")
	}
}

func TestGateBlocksEmptyBody(t *testing.T) {
	if _, err := Gate([]core.Effect{core.SendEmail{To: "ada@example.com", Body: "  "}}); err == nil {
		t.Fatal("email with empty body must be blocked")
	}
}

// A zero Vetted carries no effects, so Run can't be tricked into executing
// ungated effects by constructing Vetted{} directly.
func TestZeroVettedRunsNothing(t *testing.T) {
	Run(Vetted{})
}
