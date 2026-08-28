package core

import "time"

// Todo is the domain entity, a pure data value. Timestamps are carried as data
// (the shell/DB assigns them); core never calls the clock, so every function
// here stays deterministic.
//
// The entity lives in this shared vocabulary package (rather than in
// core/todos) because the StoreTodo effect carries a Todo, and effects must live
// with the sealed Effect interface. The todos *logic* lives in core/todos.
type Todo struct {
	ID        string
	Title     string
	Done      bool
	CreatedAt time.Time
	UpdatedAt time.Time
}
