package shell

import (
	"context"
	"fmt"
	"log/slog"

	"agentic-sandbox/internal/core"
)

// TodoWriter is the write surface the executor drives. Postgres (*Repo) is the
// production implementation; tests supply an in-memory one. Because Run is the
// ONLY caller of these methods and Run requires a Vetted, every write is gated.
type TodoWriter interface {
	Upsert(ctx context.Context, t core.Todo) error
	Delete(ctx context.Context, id string) error
}

// Run performs a vetted batch of effects: writes go to the TodoWriter, and
// audit Log effects are emitted only AFTER the writes succeed, so a failed
// write never produces a misleading audit line.
//
// Adding an effect type means adding a case here — TestRunHandlesEveryEffect
// fails the build if you forget, and the default panic is the runtime backstop.
func Run(ctx context.Context, w TodoWriter, logger *slog.Logger, v Vetted) error {
	var audit []string
	for _, e := range v.effects {
		switch ef := e.(type) {
		case core.Log:
			audit = append(audit, ef.Line)
		case core.StoreTodo:
			if err := w.Upsert(ctx, ef.Todo); err != nil {
				return err
			}
		case core.DeleteTodo:
			if err := w.Delete(ctx, ef.ID); err != nil {
				return err
			}
		default:
			panic(fmt.Sprintf("shell.Run: unhandled effect %T", e))
		}
	}
	for _, line := range audit {
		logger.InfoContext(ctx, "audit", "event", line)
	}
	return nil
}
