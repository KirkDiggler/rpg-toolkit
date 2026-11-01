// Copyright (C) 2026 Kirk Diggler
// SPDX-License-Identifier: GPL-3.0-or-later

// Package machine is a state machine that listens: a state is the set of
// triggers it listens to.
//
// A machine is a current state and, for that state, the list of what it
// listens to and what it does when that fires: on a beat, when a guard
// holds, do acts, to a state. The list is data, a [Recipe], and a state
// with no timed transition is never asked what it does with time. That is
// the whole idea, and everything else here serves it: [Program.Listens]
// tells a host which beats the current state listens for and within what
// scope, [Program.DueAt] tells it the next round the state wants, and the
// two plain indexes [Due] and [Listeners] let the host wake only who is due
// or addressed instead of walking everyone when the clock moves.
//
// A guard posted at a gate, alert for three rounds after a near sound, and
// one asleep that wakes only to a sound at its ear:
//
//	initial: posted
//	posted:  on time                       do pick time
//	         on heard  when scope within 6  do anchor          to alert
//	alert:   on time   when rounds >= 3                         to posted
//	         on time                       do pick alert
//	asleep:  on heard  when scope within 2                      to posted
//
// The alert state's two transitions on time show the rule [Program.Step]
// keeps: transitions are asked in order and the first whose guard applies
// fires. A guard answers [Applies], [DoesNotApply] or [Depends]; a guard
// that depends does not fire. The asleep state has no transition on time,
// so DueAt never makes it due: it costs nothing until a sound lands within
// two hexes.
//
// The machine is rulebook-agnostic and bus-free. It imports only core. The
// host supplies the [Vocabulary]: the beats a transition may be on (with
// InScope and Round for the ones that carry a scope or a round) and the
// facts a guard may read. Step returns what happened as a [Stepped] value
// and never publishes; the host executes the acts that are its own (a Pick
// names a key of the creature's table) and the machine applies the rest
// (Set, Clear, Anchor, WakeIn) to the [Blob] it is handed. There are no
// goroutines and no timers: time is the round the host passes in.
//
// What persists is the Blob: the current state, the bag keys that differ
// from their declared zero, the wake and the anchor. The indexes are never
// persisted; a host rebuilds them from every blob's Listens and DueAt at
// load. Recipes and blobs round-trip through encoding/json/v2, which lands
// whole numbers in a bag as int.
//
// The shape is the one examples/effects/machine proved, written fresh
// (rpg-project#766, ruling R3: a fresh take with no baggage); nothing is
// shared with the example.
package machine
