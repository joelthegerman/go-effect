// Package todos is the pure business logic for the todos feature: it decides
// what should happen and returns core.Effect values, performing no I/O. It is
// the package you develop features in. Purity is enforced by TestCoreIsPure.
package todos

import (
	"strings"

	"agentic-sandbox/internal/core"
)

// CreateInput is the validated-elsewhere raw input for creating a todo. The API
// decodes JSON into this; core decides whether it's acceptable.
type CreateInput struct {
	Title string
}

// Patch is a partial update. Nil fields mean "leave unchanged" — that's why
// they're pointers: it lets core tell "set done to false" apart from "don't
// touch done".
type Patch struct {
	Title *string
	Done  *bool
}

// Create plans the creation of a todo. Pure: given an id and input it validates
// and returns the effects that SHOULD happen (persist + audit), performing none
// of them. The id is supplied by the shell so this function has no hidden inputs.
func Create(id string, in CreateInput) ([]core.Effect, error) {
	title, err := cleanTitle(in.Title)
	if err != nil {
		return nil, err
	}
	if id == "" {
		return nil, core.Invalid("id", "must not be empty")
	}
	todo := core.Todo{ID: id, Title: title, Done: false}
	return []core.Effect{
		core.StoreTodo{Todo: todo},
		core.Log{Line: "todo.create " + id},
	}, nil
}

// Update plans an update. This is the "read then decide" pattern: the shell
// loads the current todo and passes it in, so core can apply only the provided
// fields and preserve the rest — without doing any I/O itself.
func Update(cur core.Todo, p Patch) ([]core.Effect, error) {
	next := cur
	if p.Title != nil {
		title, err := cleanTitle(*p.Title)
		if err != nil {
			return nil, err
		}
		next.Title = title
	}
	if p.Done != nil {
		next.Done = *p.Done
	}
	return []core.Effect{
		core.StoreTodo{Todo: next},
		core.Log{Line: "todo.update " + cur.ID},
	}, nil
}

// Delete plans a deletion. Existence is checked by the shell (which owns the
// read) before this runs, so core only describes the intent.
func Delete(id string) ([]core.Effect, error) {
	if id == "" {
		return nil, core.Invalid("id", "must not be empty")
	}
	return []core.Effect{
		core.DeleteTodo{ID: id},
		core.Log{Line: "todo.delete " + id},
	}, nil
}

// cleanTitle is the validation the logic wants for its own correctness: a todo
// must have a non-empty title. The UPPER bound on length is a guardrail imposed
// on the code and lives in the shell's gate, not here.
func cleanTitle(raw string) (string, error) {
	title := strings.TrimSpace(raw)
	if title == "" {
		return "", core.Invalid("title", "must not be empty")
	}
	return title, nil
}
