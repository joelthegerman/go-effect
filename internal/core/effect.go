// Package core is pure: it does no I/O. It turns input into Effects that
// DESCRIBE what should happen; package shell is what actually performs them.
// core_test.go enforces the purity boundary at build time.
package core

// Effect is a description of something that should happen in the real world.
type Effect interface{ isEffect() }

type Log struct{ Line string }
type SendEmail struct{ To, Body string }

func (Log) isEffect()       {}
func (SendEmail) isEffect() {}
