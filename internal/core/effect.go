// Package core is pure: it does no I/O. It turns input into Effects that
// DESCRIBE what should happen; package shell is what actually performs them.
// core_test.go and the depguard linter enforce the purity boundary.
package core

// Effect is a description of something that should happen in the real world.
// Core produces Effects as plain data and performs none of them — the shell's
// executor is the only code that turns an Effect into an actual side effect.
type Effect interface{ isEffect() }

// Log is an audit line. The shell decides where it goes (stdout, a file, a log
// pipeline); core only says what to record.
type Log struct{ Line string }

// StoreTodo asks the shell to persist a todo (insert-or-update / upsert). Core
// builds the desired end state as data; the shell writes it.
type StoreTodo struct{ Todo Todo }

// DeleteTodo asks the shell to remove a todo by id.
type DeleteTodo struct{ ID string }

func (Log) isEffect()        {}
func (StoreTodo) isEffect()  {}
func (DeleteTodo) isEffect() {}
