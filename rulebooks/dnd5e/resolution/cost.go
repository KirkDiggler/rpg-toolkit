// Copyright (C) 2026 Kirk Diggler
// SPDX-License-Identifier: GPL-3.0-or-later

package resolution

import (
	"context"
	"fmt"

	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/character"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/combat"
)

// Cost is what an interaction costs the actor who declared it, compiled.
//
// It is DATA, and under R2 that is what lets it exist at all. A gate resolution
// consulted would be the first capability this package reads rather than
// carries; a price it is handed is a struct field, and the difference is the
// whole reason the economy needed no vocabulary change to arrive
// (docs/ideas/session-sdk/economy-gate.md).
//
// The price is compiled ABOVE this package, by whoever knows what a fighter is
// — character.CostOfAttack and character.CostOfStrike today. By the time a
// profile reaches the door the class table is gone, and a level-5 fighter's
// Attack action is indistinguishable from a level-1 fighter's except in the
// number it banks.
//
// A nil *Cost is a free action: nothing is looked up, nothing is refreshed, and
// nothing is charged. That is the shipped semantics of the gate one layer down
// ("a machine maybe doesn't have a cost and that's okay"), and it is what every
// Resolve in this package did before the door existed.
type Cost struct {
	// PayerID names the participant whose sheet is charged. REQUIRED whenever a
	// cost is present — a price nobody pays is a free action wearing a cost's
	// clothes, and nothing downstream would ever say so.
	//
	// It must name a CHARACTER in the cast. A monster is refused by name rather
	// than charged: monster.Monster keeps no economy, it is handed one for the
	// duration of a turn and it is thrown away after, so there is nothing on
	// that sheet to debit. Monsters take no gated action in v1 — the session
	// refuses them at Attack — and this refusal is what keeps the day they do
	// from being a silent free swing.
	PayerID string

	// Profile is the price, compiled. Nil charges nothing.
	Profile *combat.SpendProfile

	// Turn is the turn the payer is acting in, so a bank left over from an
	// earlier one can be refilled before it is charged. Nil refreshes nothing
	// and charges the bank exactly as it was stored.
	//
	// Nil is a legal statement rather than a missing capability, and the
	// difference from rpg-toolkit#1033's roller is that nil INVENTS nothing.
	// THE FAILURE DIRECTION IS THE WHOLE ARGUMENT FOR ALLOWING IT: forgetting
	// this field yields refusals, never free actions. A caller who omits it
	// gets the bank exactly as stored, and once that bank is empty every ask is
	// refused — loudly, and pointing at the missing wiring. #1033's nil failed
	// the other way, quietly becoming real randomness that looked like a
	// working capability.
	//
	// What this package will not do is guess a turn — see [Turn] for why
	// neither half of one is derivable here.
	Turn *Turn
}

// Turn is which turn the payer is acting in, and what a fresh one grants them.
//
// It is not initiative order and not the encounter's clock: it is the marker
// character.RefreshForTurn compares against the economy stored on the sheet.
// An economy filed under any other number is stale and gets replaced with a
// full one; filed under this one it is left exactly as it is, so a second swing
// cannot refill what the first spent. That is what makes the refresh safe to
// ask for at every door rather than exactly once at a turn boundary nobody
// announces.
//
// BOTH HALVES ARE SUPPLIED BECAUSE NEITHER IS DERIVABLE HERE, and the
// derivations that look available are wrong:
//
//   - The world carries no per-character turn number. [Input.World] holds
//     per-bubble rounds (clock.TurnData's Order, ActiveIdx and Round), and a
//     member on the world clock has no bubble at all. Turning that into a turn
//     number would be this package deciding what a turn is.
//   - character.Character's own speed accessor answers the BASE walking speed,
//     before conditions have their say, while the refresh wants the speed a
//     turn actually seeds. Reading it here would put a right-looking wrong
//     number on every hasted, slowed or unarmored-moving sheet — the shape of
//     failure rpg-toolkit#1033 exists to remember.
type Turn struct {
	// Number is the turn being acted in. An economy stored under any other
	// number is stale.
	Number int

	// Speed is the movement a fresh turn seeds, in feet — the payer's speed
	// after whatever conditions have to say about it, which the caller computes.
	Speed int
}

// validate reports whether this cost names a price somebody could be charged.
//
// It runs with the rest of [Input.Validate], before the world is loaded and
// before a sheet is attached, because a malformed price is a caller or a
// content defect rather than an actor who ran out — and the two must not reach
// a client as the same refusal.
//
// What it deliberately does not require is a price at all. A nil cost reaches
// this method and leaves it legal, and so does a nil profile: both are free
// actions, which is the gate's shipped semantics one layer down rather than a
// default invented at this seam.
func (c *Cost) validate() error {
	if c == nil {
		return nil
	}

	if c.PayerID == "" {
		return fmt.Errorf("%w: no payer named", ErrBadCost)
	}

	if err := c.Profile.Validate(); err != nil {
		// Both are wrapped: this package's sentinel is what a caller matches on,
		// and the gate's own structured error is what says which key was wrong.
		return fmt.Errorf("%w: %w", ErrBadCost, err)
	}

	return nil
}

// payAtTheDoor charges the actor after pure machine preflight and before the
// first yielded step runs.
//
// This is the runner's enforcement point. Machine.Start has already validated
// the interaction without publishing, rolling, spending, or mutating; every
// executable step runs only after this payment and is never told whether the
// swing cost an action or nothing. That preserves the ignorance the ruling asks
// for because price was answered by whoever compiled the profile one layer up.
//
// The order inside is not arbitrary:
//
//   - The payer is found FIRST. A cost naming somebody who cannot be charged is
//     a caller defect, and it should not refill anybody's bank on its way to
//     being refused.
//   - The refresh comes NEXT, because a bank left over from an earlier turn has
//     to be full before it is asked to pay. It is safe to ask at every door: an
//     economy already filed under this turn is left exactly as it is, so a
//     second swing cannot refill what the first one spent.
//   - The charge comes LAST, and it is all-or-none by the gate's own
//     construction — a bonus strike that needs a bonus action and a ki point
//     cannot take the action and then discover there is no ki.
func payAtTheDoor(ctx context.Context, cost *Cost, cast *Participants) error {
	if cost == nil {
		return nil
	}

	payer, err := ledgerFor(cast, cost.PayerID)
	if err != nil {
		return err
	}

	if cost.Turn != nil {
		if _, refreshErr := payer.RefreshForTurn(ctx, &character.RefreshForTurnInput{
			TurnNumber: cost.Turn.Number,
			Speed:      cost.Turn.Speed,
		}); refreshErr != nil {
			return fmt.Errorf("resolution: refresh %q for turn %d: %w",
				cost.PayerID, cost.Turn.Number, refreshErr)
		}
	}

	if err := combat.Pay(payer, cost.Profile); err != nil {
		return fmt.Errorf("%w: %q: %w", ErrCannotPay, cost.PayerID, err)
	}

	return nil
}

// ledgerFor finds the sheet a cost is charged to.
//
// The concrete character comes back rather than a combat.Ledger, and both
// reasons matter: the refresh is a character's own verb, and a nil *Character
// inside a non-nil interface would sail past the gate's nil check as a ledger
// that exists and answers nothing.
func ledgerFor(cast *Participants, id string) (*character.Character, error) {
	if ch, ok := cast.Character(id); ok {
		return ch, nil
	}

	if _, ok := cast.Monster(id); ok {
		// Named rather than dereferenced, and named separately from "was not
		// passed in", because the two want different fixes. A monster keeps no
		// economy: one is handed to it for the duration of a turn by whoever
		// runs that turn, and it is thrown away after. Monsters take no gated
		// action in v1, and this is what keeps the day they do from arriving as
		// a silently free one.
		return nil, fmt.Errorf(
			"%w: %q is a monster, whose economy belongs to whoever runs its turn rather than to its sheet",
			ErrNoPayer, id)
	}

	return nil, fmt.Errorf("%w: %q was not passed in", ErrNoPayer, id)
}
