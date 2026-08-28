package kernel

import (
	"context"
	"fmt"
	"log/slog"

	"agentic-sandbox/internal/core"
)

// TodoWriter is the write surface the executor drives, bound to a single
// transaction by UnitOfWork.WithTx. A feature's repository implements it; tests
// supply an in-memory one. Because Run is the ONLY caller of these methods and
// Run requires a Vetted, every write is gated.
//
// As more features add write effects, this grows (or gains sibling writer
// interfaces) alongside the switch in Run.
type TodoWriter interface {
	Upsert(ctx context.Context, t core.Todo) error
	Delete(ctx context.Context, id string) error
}

// UnitOfWork runs a function inside one atomic transaction, giving it a
// TodoWriter scoped to that transaction. A feature repository implements it over
// Postgres; tests implement it in memory. It is the seam that lets Run execute a
// whole batch of effects all-or-nothing: if any effect fails, none persist.
type UnitOfWork interface {
	WithTx(ctx context.Context, fn func(TodoWriter) error) error
}

// Run performs a vetted batch of effects atomically: all writes execute inside
// one transaction, and audit Log effects are emitted only AFTER that
// transaction commits — so a rolled-back write never leaves a misleading audit
// line behind, and a partially-applied batch can never be observed.
//
// Adding an effect type means adding a case here — TestRunHandlesEveryEffect
// fails the build if you forget, and the default panic is the runtime backstop.
func Run(ctx context.Context, uow UnitOfWork, logger *slog.Logger, v Vetted) error {
	var audit []string
	err := uow.WithTx(ctx, func(w TodoWriter) error {
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
				panic(fmt.Sprintf("kernel.Run: unhandled effect %T", e))
			}
		}
		return nil
	})
	if err != nil {
		return err
	}
	for _, line := range audit {
		logger.InfoContext(ctx, "audit", "event", line)
	}
	return nil
}
