// Copyright (C) 2026 Kirk Diggler
// SPDX-License-Identifier: GPL-3.0-or-later

package encounter

// TurnDriver decides what a member with no player does when a fight's clock
// lands on their turn.
//
// # Why this is not Decider
//
// Decider already answers "what does an unplayed member do" — but only in
// free roam. Pump skips a monster caught in a bubble entirely: "a monster in
// a fight is not consulted... its decider is skipped until the fight
// dissolves." That boundary is load-bearing rather than an oversight —
// Decider's Intent vocabulary (IntentMoveTo, IntentHold) answers WHERE TO BE,
// and a turn asks a different question, ATTACKING, TARGETING, AND THE ACTION
// ECONOMY, which nothing in this module can even express yet. Reusing
// Decider for a turn would blur the exact clock-kind boundary ClockKind
// exists to keep sharp: the world thinks on the tick, the fight thinks in
// turns, and now each has its own driver.
//
// # Required, never defaulted — and why that differs from Decider
//
// Decider is optional per member; a monster with none simply holds forever in
// free roam, which is locally inert — nothing else in the encounter depends
// on that monster ever moving. TurnDriver has no such safe default: a member
// the clock lands on with no driver stalls the ENTIRE bubble, forever, for
// every member in it — the exact defect rpg-toolkit#1162 exists to close. So
// it is required at construction like Standing, Sight and Initiative, and for
// the identical reason: a nil answer here would be this module guessing a
// rule instead of asking for one. See ADR-0043.
//
// # One capability, not one per member
//
// Unlike Decider, TurnDriver is asked with a member's ID rather than supplied
// per member: v1's answer (Pass) is identical for every unplayed member, so
// there is nothing yet to vary per monster. When a real driver needs to
// answer differently per monster, that is the driver's own business — it can
// key off the ID it is handed, the same way any Decider already would — and
// does not require a second capability shape at this seam.
type TurnDriver interface {
	// Act decides what member does on their turn, or returns an error that
	// aborts the caller's whole verb — see EndTurn and form, both of which
	// consult this synchronously and persist nothing until the caller's own
	// commit, so a driver failure here costs nothing but the retry.
	Act(member MemberID) (TurnOutcome, error)
}

// TurnOutcome is a sealed vocabulary (unexported marker method, the same
// shape Intent already uses for Decider).
//
// ONE CASE TODAY. A driver that could decide to attack or move needs
// resolution-level machinery (weapons, the action economy) this module does
// not have and is not allowed to import — so a second TurnOutcome case is
// real vocabulary growth, arriving with the caller that needs it, the same
// road Intent travelled from IntentMoveTo to IntentHold. Following this
// repo's practice for sealed vocabularies at this seam (ADR-0038's rule for
// Gather | Pose | Request | Done), a second case should probably earn its own
// ADR rather than being added quietly.
type TurnOutcome interface {
	isTurnOutcome()
}

// Pass is the only outcome a v1 driver may return: the member's turn ends
// with no other effect than the clock advancing and a beat being recorded.
type Pass struct{}

// isTurnOutcome marks Pass as a TurnOutcome.
func (Pass) isTurnOutcome() {}
