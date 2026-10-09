// Copyright (C) 2026 Kirk Diggler
// SPDX-License-Identifier: GPL-3.0-or-later

package session

import (
	"context"
	"fmt"

	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/character"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/encounter"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/resolution"
)

// The economy at this seam: what a swing costs, and whose turn it is bought in.
//
// The ruling this implements is docs/ideas/session-sdk/economy-gate.md. The
// price is DATA compiled above resolution, the door charges it before the
// machine runs, and no machine ever touches a ledger. What this file adds is the
// only part that was left: somebody has to compile the price and say which turn
// it is being paid in, and for a character swinging in a fight that somebody is
// this package.

// swingPrice is what one swing costs and the sheet that will be charged for it.
//
// The sheet travels WITH the price because the two are made together and must
// not be separated. Readying a turn is a mutation — a cold sheet is lit, a stale
// bank is refilled — and the price is compiled from the state that mutation
// leaves behind. Handing the door the price while handing the cast the sheet as
// it was stored would charge a bank nobody looked at.
//
// Both fields are nil in free roam, which is a price of nothing rather than a
// missing one. See [Manager.priceSwing].
type swingPrice struct {
	// cost is what the door charges, or nil to charge nothing.
	cost *resolution.Cost

	// payer is the attacker's sheet with its turn readied, or nil when nothing
	// readied it. It replaces the stored sheet in the cast.
	payer *character.Data
}

// priceSwing works out what this swing costs its attacker, and readies the
// sheet the door will charge for it.
//
// # Free roam charges nothing, and that is a ruling rather than a gap
//
// The action economy is a FIGHT's economy. That is not this package's opinion:
// [combat.Ledger] opens with InCombat and refuses every payment from a holder
// who is not in one, and a character on the world clock has no turn to spend a
// turn's slots from. So a swing off the turn clock is passed no cost at all,
// which is the same thing the verb did before this file existed.
//
// The alternative was tried on paper and is worse in a way worth recording: a
// cost handed over in free roam is refused by the gate with "not in combat", so
// every free-roam swing in the package — the duel every other test in this
// suite is built on — would stop working. Charging nothing where there is no
// economy is the honest reading of the same fact.
//
// # The turn number is the round, and it is READ rather than invented
//
// [resolution.Turn] declines to derive a turn number and says why: the world
// carries per-bubble rounds and turning those into a turn number would be
// resolution deciding what a turn is. THIS package is allowed to decide that,
// because it is the one that projects the composition's clock to hosts already
// ([Manager.Turn]).
//
// A member in a bubble acts exactly once per round — the order advances one
// name per EndTurn and the round wraps when it comes back around — so the
// round IS that member's turn number. Nothing is guessed.
//
// # Speed is the honest half-measure, and it stayed one once movement started reading it
//
// [character.RefreshForTurnInput] wants the speed a turn seeds AFTER conditions
// have had their say, and this package has no such number: it can read the
// sheet's base walking speed and nothing else. So the seeded MovementRemaining
// is wrong on any hasted or slowed character.
//
// It was shipped anyway, deliberately, when nothing read it: movement cost
// nothing (the ruling's fork (d)), so no profile named CapacityMovement and no
// path spent it, and this line was named as the one that would have to grow a
// real answer the day that changed. That day is rpg-toolkit#1169: turn-clock
// [Manager.Move] now spends exactly the MovementRemaining this call seeds,
// through the identical [combat.Pay] gate a swing pays through. The
// haste/slow gap is unchanged and still real — a speed computed where
// conditions can be asked is still above this seam — but it is no longer
// theoretical, and the day it is closed this comment is where to start.
//
// It takes the encounter rather than a *writeScope so a read verb can call it
// too ([Manager.Afford] does): pricing a swing commits nothing on its own, and
// a signature that could only be handed a write scope would say otherwise.
//
// Returns ErrNoMember, ErrNoCharacter, ErrBadCharacter, ErrBadRepository, or
// ErrBadCost.
func (m *Manager) priceSwing(
	ctx context.Context, enc *encounter.Encounter, attacker string, sheet *character.Character,
) (*swingPrice, error) {
	clock, err := enc.ClockOf(&encounter.ClockOfInput{Member: encounter.MemberID(attacker)})
	if err != nil {
		return nil, translate(err)
	}
	if ClockKind(clock.Kind) != ClockTurn {
		return &swingPrice{}, nil
	}

	if err := resolution.ReadyForTurn(ctx, sheet, resolution.Turn{Number: clock.Round, Speed: sheet.GetSpeed()}); err != nil {
		return nil, fmt.Errorf("attacker %q: %w: %v", attacker, ErrBadCost, err)
	}

	profile, err := character.CostOfSwing(sheet)
	if err != nil {
		return nil, fmt.Errorf("attacker %q: %w: %v", attacker, ErrBadCost, err)
	}

	ready, err := sheet.ToData()
	if err != nil {
		return nil, fmt.Errorf("attacker %q: %w: %v", attacker, ErrBadCharacter, err)
	}
	return &swingPrice{
		cost: &resolution.Cost{
			PayerID: attacker,
			Profile: profile,
			// Handed over even though resolution.ReadyForTurn has already done it on the
			// sheet below, and the redundancy is deliberate rather than
			// forgotten. The door's refresh is the contract E2 built and this is
			// its caller honouring it; finding the economy already filed under
			// this turn, it does nothing. So a mutation that passes nil here
			// changes no observable behaviour and no test can catch it — which
			// is worth saying out loud, because the next reader to notice will
			// otherwise conclude the field is dead and remove it. What it is is
			// the backstop for the day this seam hands over a sheet it did not
			// ready.
			Turn: &resolution.Turn{
				Number: clock.Round,
				Speed:  sheet.GetSpeed(),
			},
		},
		payer: ready,
	}, nil
}
