package shell

import (
	"errors"
	"strings"
	"testing"

	"agentic-sandbox/internal/core"
)

func TestGateAllowsNormalTitle(t *testing.T) {
	e := core.StoreTodo{Todo: core.Todo{ID: "t1", Title: "buy milk"}}
	if _, err := Gate([]core.Effect{e}); err != nil {
		t.Fatalf("normal todo should pass: %v", err)
	}
}

func TestGateBlocksOversizeTitle(t *testing.T) {
	e := core.StoreTodo{Todo: core.Todo{ID: "t1", Title: strings.Repeat("x", maxTodoTitle+1)}}
	_, err := Gate([]core.Effect{e})
	var ge GuardrailError
	if !errors.As(err, &ge) {
		t.Fatalf("oversize title must be blocked as a GuardrailError, got %v", err)
	}
}

// A zero Vetted carries no effects, so nothing can be tricked into executing
// ungated effects by constructing Vetted{} directly.
func TestZeroVettedHasNoEffects(t *testing.T) {
	if len(Vetted{}.effects) != 0 {
		t.Fatal("zero Vetted must carry no effects")
	}
}
