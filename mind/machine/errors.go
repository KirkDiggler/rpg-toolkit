// Copyright (C) 2026 Kirk Diggler
// SPDX-License-Identifier: GPL-3.0-or-later

package machine

import "errors"

// The refusals. Compile joins every problem it finds into one error, each
// wrapping one of these and naming where in the recipe it is, so a host
// tests a refusal with errors.Is and shows an author the message.
var (
	// ErrUnknownBeat refuses a transition on a beat the vocabulary does not
	// supply, and a Step on one.
	ErrUnknownBeat = errors.New("machine: unknown beat")
	// ErrUnknownFact refuses a fact leaf naming a fact the vocabulary does
	// not supply.
	ErrUnknownFact = errors.New("machine: unknown fact")
	// ErrUnknownState refuses a transition, an initial state or a blob
	// naming a state the recipe lacks.
	ErrUnknownState = errors.New("machine: unknown state")
	// ErrNoInitial refuses a recipe with no initial state.
	ErrNoInitial = errors.New("machine: no initial state")
	// ErrBadAct refuses an act that sets other than exactly one field, a
	// wake_in at or below zero, or a set or clear of a key the bag does not
	// declare with that kind.
	ErrBadAct = errors.New("machine: bad act")
	// ErrNoScope refuses a scope leaf on a beat whose spec has no InScope.
	ErrNoScope = errors.New("machine: beat has no scope")
	// ErrNoRound refuses a rounds leaf on a beat whose spec has no Round.
	ErrNoRound = errors.New("machine: beat carries no round")
	// ErrBadGuard refuses a malformed guard node: other than exactly one
	// field set, an unknown op, a value of the wrong kind, a flag that is
	// not a declared bool, a scope with no measure.
	ErrBadGuard = errors.New("machine: bad guard")
	// ErrBadBag refuses a declared bag value that is not a bool, int or
	// string.
	ErrBadBag = errors.New("machine: bad bag")
	// ErrBadState refuses a terminal state that carries transitions.
	ErrBadState = errors.New("machine: bad state")
	// ErrNilBlob refuses a Step on no blob.
	ErrNilBlob = errors.New("machine: nil blob")
)
