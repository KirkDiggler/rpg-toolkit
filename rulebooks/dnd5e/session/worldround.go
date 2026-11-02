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

// The world clock is a place to act (rpg-project#785, design
// ideas/free-roam-actions/design.md). Every verb the turn clock prices is
// offered there, and the world clock's price is one round of the world per
// action (R1), which the encounter already charges when the deed lands
// ([encounter.Encounter.RecordCast], the attack outcome, and the rest).
//
// # One round's economy, lent to one action and taken back
//
// The rulebook prices every declaring verb out of a turn's economy: Attack's
// swing buys capacity with an action, a spell pays its action and its slot
// through [character.Character.PaySpell], an ability spends its own bonus
// action from inside its activation. None of those can be asked of a sheet
// with no economy at all ([character.Character.InCombat] answers false in free
// roam and the gate refuses every payment there).
//
// So a world-clock action is readied with ONE FRESH ROUND'S ECONOMY, the same
// one a turn starts with, and every verb prices, pays and resolves through
// exactly the code it runs on the turn clock. What the world clock adds is
// that the economy is LENT: it is lit fresh for each action and put out
// before the actor's sheet is written ([Manager.saveDirty] reads
// [writeScope.worldActor]). So:
//
//   - The turn's three slots never run short, because each action gets a full
//     round. No slot shortfall can reach a world-clock row, which is R1's "no
//     slot and no shortfall" arrived at by the rule rather than by masking.
//   - What a sheet holds across rounds is still spent: a spell slot, a rage,
//     a Second Wind. Those are the sheet's own facts (the design's "availability
//     by the verb's own facts"), not the turn's, and a cleric out of slots in
//     free roam is out of slots.
//   - Nothing of the lent round survives the action. A sheet in free roam
//     stays exactly as out of combat as before, so a fight that later forms
//     lights its own economy from cold, which is the stale-economy bug
//     rpg-project#253 ruled out ([Manager.exitDissolvedCombatants]).
const worldRoundNumber = 0

// readyForClock readies an actor's sheet to price and pay on the clock it is
// on: the fight's round on the turn clock, or one fresh lent round on the
// world clock (see the note above).
//
// On the world clock the economy is put out FIRST, so whatever a sheet
// happens to carry is never what a world action is priced against: every
// world action starts from the same full round.
func readyForClock(ctx context.Context, sheet *character.Character, clock *encounter.ClockOfOutput) error {
	if ClockKind(clock.Kind) != ClockTurn {
		if _, err := sheet.ExitCombat(ctx, &character.ExitCombatInput{}); err != nil {
			return err
		}
		return resolution.ReadyForTurn(ctx, sheet, resolution.Turn{Number: worldRoundNumber, Speed: sheet.GetSpeed()})
	}
	return resolution.ReadyForTurn(ctx, sheet, resolution.Turn{Number: clock.Round, Speed: sheet.GetSpeed()})
}

// lendWorldRound marks this verb's actor as acting on the world clock, so the
// round its sheet was lent is taken back when the sheet is written. A no-op on
// the turn clock, whose economy is the fight's and persists across the turn.
func lendWorldRound(scope *writeScope, actor string, clock *encounter.ClockOfOutput) {
	if ClockKind(clock.Kind) != ClockTurn {
		scope.worldActor = actor
	}
}

// returnWorldRound puts out the lent economy on the world actor's sheet before
// it is written. Everything else the action did to that sheet (a slot spent,
// damage taken, a condition gained) is kept.
//
// ONLY WHILE THE ACTOR IS STILL ON THE WORLD CLOCK. A deed can form a fight in
// the same verb (R2, R3): the aggression law turns the pair, the boundary
// machine seats the actor and lights the fight's entrant economy, reaction
// included, and that landing is written through this same scope. The economy
// on the sheet is then the fight's, not the lent round, so it stands. Once a
// bubble holds the actor the lend is over: it is dropped for the rest of the
// verb, and the round it lent was already replaced by the entrant's.
func returnWorldRound(scope *writeScope, data *character.Data) error {
	if scope.worldActor == "" || data == nil || data.ID != scope.worldActor {
		return nil
	}
	clock, err := scope.enc.ClockOf(&encounter.ClockOfInput{Member: encounter.MemberID(data.ID)})
	if err != nil {
		return translate(err)
	}
	if ClockKind(clock.Kind) == ClockTurn {
		scope.worldActor = ""
		return nil
	}
	data.ActionEconomy = nil
	return nil
}

// worldPriced is a compiled row's projection on the world clock: no slot is
// drawn, because the world clock has no budget (R1). The selector the row
// carries is unchanged, so the door that regenerates the same offer selects
// it by the same id.
//
// A shortfall in one of the turn's own currencies would mean the lent round
// was not full, which [readyForClock] makes impossible; one reaching here is
// refused rather than shown, so the bar never tells a player in free roam
// that an action is spent.
func worldPriced(declaration Declaration) (Declaration, error) {
	declaration.Slot = SlotNone
	if why := declaration.Why; why != nil && why.Reason == ShortfallNoBudget {
		switch why.Currency {
		case CurrencyAction, CurrencyBonus, CurrencyReaction:
			return Declaration{}, fmt.Errorf("%w: %s row short of %s on the world clock",
				ErrInvalidWorld, declaration.Verb, why.Currency)
		}
	}
	if len(declaration.Cost) > 0 {
		kept := make([]CostComponent, 0, len(declaration.Cost))
		for _, component := range declaration.Cost {
			switch component.Currency {
			case CurrencyAction, CurrencyBonus, CurrencyReaction:
				continue
			}
			kept = append(kept, component)
		}
		declaration.Cost = kept
	}
	return declaration, nil
}
