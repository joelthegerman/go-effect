package kernel

import (
	"errors"
	"testing"

	"agentic-sandbox/internal/core"
)

// blockAll is a test policy that refuses every effect, standing in for any
// feature guardrail (the real ones live in each feature's shell package).
func blockAll(core.Effect) error { return GuardrailError{Reason: "blocked"} }

func TestGateAllowsWhenNoPolicyBlocks(t *testing.T) {
	e := core.StoreTodo{Todo: core.Todo{ID: "t1", Title: "buy milk"}}
	if _, err := Gate(nil, []core.Effect{e}); err != nil {
		t.Fatalf("with no policies, effects should pass: %v", err)
	}
}

func TestGateBlocksWhenPolicyRefuses(t *testing.T) {
	e := core.StoreTodo{Todo: core.Todo{ID: "t1", Title: "buy milk"}}
	_, err := Gate([]Policy{blockAll}, []core.Effect{e})
	var ge GuardrailError
	if !errors.As(err, &ge) {
		t.Fatalf("a refusing policy must surface as a GuardrailError, got %v", err)
	}
}

// A zero Vetted carries no effects, so nothing can be tricked into executing
// ungated effects by constructing Vetted{} directly.
func TestZeroVettedHasNoEffects(t *testing.T) {
	if len(Vetted{}.effects) != 0 {
		t.Fatal("zero Vetted must carry no effects")
	}
}
