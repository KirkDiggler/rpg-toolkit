// Copyright (C) 2026 Kirk Diggler
// SPDX-License-Identifier: GPL-3.0-or-later

package machine

// Vocabulary is the host's plug into the machine: the facts a guard may
// read and the beats a transition may be on. The machine knows no beat and
// no fact of its own; Compile checks a recipe against the vocabulary it is
// given and binds the recipe's names to the specs once.
type Vocabulary struct {
	Facts map[FactName]FactSpec
	Beats map[Beat]BeatSpec
}

// ScalarKind is the kind of a fact or a bag value.
type ScalarKind string

const (
	// KindBool is a bool scalar.
	KindBool ScalarKind = "bool"
	// KindInt is an int scalar.
	KindInt ScalarKind = "int"
	// KindString is a string scalar.
	KindString ScalarKind = "string"
)

// FactSpec declares one fact: its kind, and Read, which returns the fact's
// value from the env a host passes to Step and whether it is known. A fact
// Read reports unknown, or known with a value of another kind, makes its
// leaf answer Depends.
type FactSpec struct {
	Kind ScalarKind
	Read func(env any) (any, bool)
}

// BeatSpec declares one beat.
//
//   - Timed marks a beat the clock raises (time): the due heap answers who
//     is asked it, so it is never a listen, and asking it at or after a
//     blob's wake clears the wake.
//   - InScope answers whether the beat's event ev falls within the scope s
//     of the creature asking; nil means the beat has no scope, and Compile
//     refuses a scope leaf on it.
//   - Round returns the round ev carries and whether it carries one; nil
//     means the beat never does, and Compile refuses a rounds leaf on it.
type BeatSpec struct {
	Timed   bool
	InScope func(ev any, s ScopeLeaf) bool
	Round   func(ev any) (uint64, bool)
}

// kindOf returns the scalar kind of a bag or leaf value, and false for any
// other type.
func kindOf(v any) (ScalarKind, bool) {
	switch v.(type) {
	case bool:
		return KindBool, true
	case int:
		return KindInt, true
	case string:
		return KindString, true
	}
	return "", false
}
