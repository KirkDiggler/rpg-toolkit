// Copyright (C) 2026 Kirk Diggler
// SPDX-License-Identifier: GPL-3.0-or-later

package monster

import (
	"fmt"

	coreCombat "github.com/KirkDiggler/rpg-toolkit/core/combat"
	"github.com/KirkDiggler/rpg-toolkit/rpgerr"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/combat"
	combatActions "github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/combat/actions"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/conditions"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/refs"
)

// The cost compilers for a monster: what its turn's actions are priced at.
//
// A player's swing is priced by the rules at the moment it is made, not by a
// cost stored on the weapon, and a monster's listed action is priced the same
// way. These compilers sit beside the sheet for the reason the character's do
// (see character.CostOfSwing): a compiler reads the SHEET, and the gate that
// charges the price cannot see one.
//
// The conventions follow the character compilers'. A concrete sheet in, a
// neutral [combat.SpendProfile] out, nil refused by name, and only STATIC
// facts compiled: nothing here asks what the monster is standing next to.

// CostOfActionInput names a monster's sheet and one of the actions it lists.
type CostOfActionInput struct {
	// Sheet is the monster's stored sheet. It is required, though it is not
	// read today.
	Sheet *Data
	// Action is the listed action to price.
	Action combatActions.Definition
}

// CostOfActionOutput is the price, for the gate to charge.
type CostOfActionOutput struct {
	// Cost is the price of the action. It is always a copy the caller owns.
	Cost *combat.SpendProfile
}

// CostOfAction compiles what one of a monster's listed actions costs.
//
// An action that states its own cost costs that: a copy of it, never the
// definition's own pointer, so charging or editing the price cannot reach the
// definition. An action that states none costs one action. A multiattack is one
// listed action and costs one action; the swings it runs are the sequence's and
// are not priced here.
//
// It does not look the action up on the sheet and does not judge whether the
// sheet can pay: that is the gate's.
//
// The sheet is not read today. It is taken anyway, because the price of an
// action is a sheet question the moment anything makes it one, and a compiler
// that has to grow a parameter is a compiler every caller has to be found for.
//
// Refuses a nil input or a nil Sheet with [rpgerr.CodeNil].
func CostOfAction(in *CostOfActionInput) (*CostOfActionOutput, error) {
	if in == nil {
		return nil, rpgerr.New(rpgerr.CodeNil, "no input to price a monster's action for")
	}
	if in.Sheet == nil {
		return nil, rpgerr.New(rpgerr.CodeNil, "no monster sheet to price an action for")
	}

	if in.Action.Cost != nil {
		return &CostOfActionOutput{Cost: combatActions.CloneSpendProfile(in.Action.Cost)}, nil
	}

	return &CostOfActionOutput{Cost: oneSlot(coreCombat.ActionStandard)}, nil
}

// CostOfDisengageInput names the monster's sheet.
type CostOfDisengageInput struct {
	// Sheet is the monster's stored sheet, whose conditions are read for the
	// Nimble Escape trait. It is required.
	Sheet *Data
}

// CostOfDisengageOutput is the price, for the gate to charge.
type CostOfDisengageOutput struct {
	// Cost is the price of the Disengage. It is always a fresh profile.
	Cost *combat.SpendProfile
}

// CostOfDisengage compiles what a monster's Disengage costs.
//
// A sheet that holds the Nimble Escape trait pays one bonus action. A sheet
// without it pays one action: Disengage is an action any creature can take.
// Nothing asks for that second arm today, since the encounter disengages only
// a creature with the trait; it is written because a price function must
// answer for every sheet, and the wrong answer there would be "free".
//
// The trait is read with [conditions.HoldsRef] off the stored sheet, loading
// nothing.
//
// Refuses a nil input or a nil Sheet with [rpgerr.CodeNil], and a sheet whose
// stored conditions cannot be read with an error wrapping the read's own.
func CostOfDisengage(in *CostOfDisengageInput) (*CostOfDisengageOutput, error) {
	if in == nil {
		return nil, rpgerr.New(rpgerr.CodeNil, "no input to price a monster's Disengage for")
	}
	if in.Sheet == nil {
		return nil, rpgerr.New(rpgerr.CodeNil, "no monster sheet to price a Disengage for")
	}

	held, err := conditions.HoldsRef(in.Sheet.Conditions, refs.MonsterTraits.NimbleEscape())
	if err != nil {
		return nil, fmt.Errorf("monster %q: reading its traits to price Disengage: %w", in.Sheet.ID, err)
	}
	if held {
		return &CostOfDisengageOutput{Cost: oneSlot(coreCombat.ActionBonus)}, nil
	}

	return &CostOfDisengageOutput{Cost: oneSlot(coreCombat.ActionStandard)}, nil
}

// CostOfDashInput names the monster's sheet.
type CostOfDashInput struct {
	// Sheet is the monster's stored sheet. It is required, though it is not
	// read today.
	Sheet *Data
}

// CostOfDashOutput is the price, for the gate to charge.
type CostOfDashOutput struct {
	// Cost is the price of the Dash. It is always a fresh profile.
	Cost *combat.SpendProfile
}

// CostOfDash compiles what a monster's Dash costs: one action, for every
// sheet, and nothing banked.
//
// It grants nothing. A player's Dash banks movement on the player's sheet
// (see combatabilities.Dash, which adds the character's speed to its movement
// remaining). An NPC's movement is not on its sheet: the encounter walks it
// one cell at a time and adds a run's speed to its own count, ruled on
// 2026-11-27 (https://github.com/KirkDiggler/rpg-project/issues/1131#issuecomment-7200191665).
// So this profile has no Grants. The day an NPC's movement is on its sheet,
// this profile banks it.
//
// The sheet is not read today: no trait makes an NPC's Dash a bonus action.
// It is taken anyway, for the reason [CostOfAction] gives.
//
// Refuses a nil input or a nil Sheet with [rpgerr.CodeNil].
func CostOfDash(in *CostOfDashInput) (*CostOfDashOutput, error) {
	if in == nil {
		return nil, rpgerr.New(rpgerr.CodeNil, "no input to price a monster's Dash for")
	}
	if in.Sheet == nil {
		return nil, rpgerr.New(rpgerr.CodeNil, "no monster sheet to price a Dash for")
	}

	return &CostOfDashOutput{Cost: oneSlot(coreCombat.ActionStandard)}, nil
}

// oneSlot is a profile of exactly one of the given slot and nothing else.
func oneSlot(slot coreCombat.ActionType) *combat.SpendProfile {
	return &combat.SpendProfile{Slots: map[coreCombat.ActionType]int{slot: 1}}
}
