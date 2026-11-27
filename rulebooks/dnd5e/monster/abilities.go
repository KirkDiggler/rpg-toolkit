// Copyright (C) 2026 Kirk Diggler
// SPDX-License-Identifier: GPL-3.0-or-later

package monster

import (
	"maps"
	"slices"

	"github.com/KirkDiggler/rpg-toolkit/core"
	coreCombat "github.com/KirkDiggler/rpg-toolkit/core/combat"
	coreResources "github.com/KirkDiggler/rpg-toolkit/core/resources"
	"github.com/KirkDiggler/rpg-toolkit/rpgerr"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/combat"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/combat/actions"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/features"
)

// AbilitiesOfInput names the monster's stored sheet.
type AbilitiesOfInput struct {
	// Sheet is the monster's stored sheet. Required.
	Sheet *Data
}

// Ability is one thing a stored sheet lets its creature use.
type Ability struct {
	// Word is the ability as an author writes it, on a stat block's `lets`
	// and in a behaviour's row: "second wind".
	Word string
	// Grant is the name the sheet's kit files it under: "second-wind".
	Grant string
	// Name is how a story names it: "Second Wind".
	Name string
	// Payable is whether the sheet can pay the ability's whole price right
	// now: every slot and every count it costs. It is the gate's own answer
	// ([combat.CanPay]) for Definition.Cost.
	Payable bool
	// Definition is the description the gate runs, as [Monster.GrantedAbility]
	// gives it: its cost names the creature's own pool by [PoolKey].
	Definition *actions.Definition
}

// AbilitiesOfOutput is what the sheet lets its creature use, sorted by Grant.
// A sheet with no kit has none.
type AbilitiesOfOutput struct {
	Abilities []Ability
}

// AbilitiesOf reads what a stored sheet lets its creature use, with whether it
// can pay for each right now. It reads the sheet without loading it, needs no
// event bus and no context, and changes nothing: not the sheet, and not its
// absent account, which stays absent.
//
// Each description is built by the same code [Monster.GrantedAbility] uses, from
// the sheet's kit, name, ref and id. Payable is [combat.CanPay] over a read-only
// ledger that answers from the sheet by the code the loaded creature answers
// from, so the sheet and the creature cannot say different things about the
// same turn.
//
// Word is the one in [features.NamedAbilities] whose ref is the grant's. A grant
// whose ability has no word is refused by name: no stat block can compile one
// today, so a sheet that holds one was not made by the rules.
//
// Refuses a nil input or a nil Sheet with [rpgerr.CodeNil].
func AbilitiesOf(in *AbilitiesOfInput) (*AbilitiesOfOutput, error) {
	if in == nil {
		return nil, rpgerr.New(rpgerr.CodeNil, "no input to read a monster's abilities for")
	}
	if in.Sheet == nil {
		return nil, rpgerr.New(rpgerr.CodeNil, "no monster sheet to read abilities from")
	}
	sheet := in.Sheet
	if sheet.Kit == nil {
		return &AbilitiesOfOutput{}, nil
	}

	ledger := &sheetLedger{sheet: sheet}
	abilities := make([]Ability, 0, len(sheet.Kit.Grants))
	for _, grantName := range slices.Sorted(maps.Keys(sheet.Kit.Grants)) {
		grant := sheet.Kit.Grants[grantName]

		word, ok := wordOf(grant.Ability)
		if !ok {
			return nil, rpgerr.Newf(rpgerr.CodeInvalidArgument,
				"stat block %q grant %q names %s, which the rules have no word for",
				sheet.Name, grantName, grant.Ability.String())
		}
		definition, err := describeGrant(sheet.Name, sheet.Ref, sheet.ID, grantName, grant)
		if err != nil {
			return nil, err
		}

		abilities = append(abilities, Ability{
			Word:       word,
			Grant:      grantName,
			Name:       definition.Name,
			Payable:    combat.CanPay(ledger, definition.Cost),
			Definition: definition,
		})
	}

	return &AbilitiesOfOutput{Abilities: abilities}, nil
}

// wordOf is the rules' own word for an ability: the one name in
// [features.NamedAbilities] whose ref is this. It is never made from a grant's
// name.
func wordOf(ref core.Ref) (string, bool) {
	for _, word := range features.NamedAbilities() {
		if named, ok := features.NamedAbility(word); ok && named == ref {
			return word, true
		}
	}

	return "", false
}

// sheetLedger is a stored sheet read as a [combat.Ledger], for [combat.CanPay]
// and nothing else. Its reads are the loaded creature's own, through
// [accountOf] and [poolNamedIn]. Its spends do nothing: CanPay never calls
// them, and a sheet that is read here is never charged here.
type sheetLedger struct {
	sheet *Data
}

var _ combat.Ledger = (*sheetLedger)(nil)

// InCombat is always true, as a loaded creature's is.
func (l *sheetLedger) InCombat() bool { return true }

// SlotsLeft answers from the sheet's account, or a full turn when it has none.
func (l *sheetLedger) SlotsLeft(slot coreCombat.ActionType) int {
	return accountOf(l.sheet.Economy).SlotsLeft(slot)
}

// CapacityLeft answers from the sheet's account, or a full turn when it has none.
func (l *sheetLedger) CapacityLeft(key combat.CapacityType) int {
	return accountOf(l.sheet.Economy).CapacityLeft(key)
}

// PoolLeft answers what is left in the pool the key names, and zero for any
// other key.
func (l *sheetLedger) PoolLeft(key coreResources.ResourceKey) int {
	name, ok := poolNamedIn(l.sheet.Kit, key)
	if !ok {
		return 0
	}

	return l.sheet.Left[name]
}

// SpendSlots does nothing: this ledger is read.
func (l *sheetLedger) SpendSlots(coreCombat.ActionType, int) {}

// SpendCapacity does nothing: this ledger is read.
func (l *sheetLedger) SpendCapacity(combat.CapacityType, int) {}

// SpendPool does nothing: this ledger is read.
func (l *sheetLedger) SpendPool(coreResources.ResourceKey, int) {}

// BankCapacity does nothing: this ledger is read.
func (l *sheetLedger) BankCapacity(combat.CapacityType, int) {}
