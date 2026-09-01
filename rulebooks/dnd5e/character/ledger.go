// Copyright (C) 2026 Kirk Diggler
// SPDX-License-Identifier: GPL-3.0-or-later

package character

import (
	coreCombat "github.com/KirkDiggler/rpg-toolkit/core/combat"
	coreResources "github.com/KirkDiggler/rpg-toolkit/core/resources"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/combat"
)

// A sheet is a [combat.Ledger].
//
// The gate spends from characters and monsters through one surface and is not
// allowed to learn which it has — a monster's economy satisfies the same
// interface from the combat package's own side. What differs is entirely below
// this line: a monster's economy is built for a turn and discarded, while every
// write here has to reach storage, which means every write here has to MARK
// (#1087). A spend that does not dirty the sheet serializes perfectly and is
// then dropped by the write-back.
var _ combat.Ledger = (*Character)(nil)

// grantedKeyFor is the one place the gate's cost vocabulary meets the sheet's
// storage vocabulary.
//
// They are two vocabularies for a reason that is historical rather than good:
// [combat.CapacityType] is what an action declares it consumes, and
// [GrantedActionKey] is what the persisted economy files it under, and the two
// were named independently — "attack" against "attacks", "flurry_strike"
// against "flurry_strikes". The strings are in saved characters, so unifying
// them is a migration rather than a rename.
//
// What matters is that the translation is TOTAL. The census's finding about
// the older bridge (toToolkitActionEconomy) was that it carried three of five
// keys, and the two it dropped went nowhere and reported nothing. A key with
// no entry here is therefore not dropped: it is stored under its own name, so
// the worst a missing alias can do is file a spend in a differently-spelled
// drawer rather than lose it.
var grantedKeyFor = map[combat.CapacityType]GrantedActionKey{
	combat.CapacityAttack:                 GrantedAttacks,
	combat.CapacityOffHandAttack:          GrantedOffHandStrikes,
	combat.CapacityMartialArtsBonusAttack: GrantedMartialArtsBonus,
	combat.CapacityFlurryStrike:           GrantedFlurryStrikes,
}

// grantedKey translates a capacity into the key the sheet files it under.
func grantedKey(key combat.CapacityType) GrantedActionKey {
	if stored, ok := grantedKeyFor[key]; ok {
		return stored
	}

	return GrantedActionKey(key)
}

// SlotsLeft reports the per-turn slots remaining.
//
// Only the three slots the sheet keeps can answer. Free and movement report
// zero — free because a free action costs no slot and so can have no slot cost,
// movement because the gate spends it as keyed capacity.
//
// Implements [combat.Ledger].
func (c *Character) SlotsLeft(slot coreCombat.ActionType) int {
	if c.actionEconomy == nil {
		return 0
	}

	switch slot {
	case coreCombat.ActionStandard:
		return c.actionEconomy.ActionsRemaining
	case coreCombat.ActionBonus:
		return c.actionEconomy.BonusActionsRemaining
	case coreCombat.ActionReaction:
		return c.actionEconomy.ReactionsRemaining
	case coreCombat.ActionFree, coreCombat.ActionMovement:
		return 0
	default:
		return 0
	}
}

// CanReact reports whether this character has a reaction left to spend.
//
// The gate every reacting rule asks before it publishes its trigger, phrased
// as the question rather than as the arithmetic, so a rule does not need a
// ledger handle to ask it. Implements [combat.Member].
func (c *Character) CanReact() bool {
	return c.SlotsLeft(coreCombat.ActionReaction) > 0
}

// SpendSlots debits per-turn slots and marks the sheet.
//
// The gate checks affordability in full before calling this, which is what
// makes a multi-currency payment atomic — see [combat.Pay].
//
// Implements [combat.Ledger].
func (c *Character) SpendSlots(slot coreCombat.ActionType, n int) {
	if c.actionEconomy == nil {
		return
	}

	switch slot {
	case coreCombat.ActionStandard:
		c.actionEconomy.ActionsRemaining -= n
	case coreCombat.ActionBonus:
		c.actionEconomy.BonusActionsRemaining -= n
	case coreCombat.ActionReaction:
		c.actionEconomy.ReactionsRemaining -= n
	case coreCombat.ActionFree, coreCombat.ActionMovement:
		return
	default:
		return
	}

	c.economyChanged()
}

// CapacityLeft reports how much of a keyed capacity remains.
//
// Movement is the one capacity the persisted economy fields rather than keys,
// so it is read from MovementRemaining; everything else comes out of the
// Granted map. Both are the sheet's own state — this is a view over what is
// saved, never a copy of it.
//
// Implements [combat.Ledger].
func (c *Character) CapacityLeft(key combat.CapacityType) int {
	if c.actionEconomy == nil || key == combat.CapacityNone {
		return 0
	}
	if key == combat.CapacityMovement {
		return c.actionEconomy.MovementRemaining
	}

	return c.actionEconomy.Granted[grantedKey(key)]
}

// SpendCapacity debits keyed capacity and marks the sheet.
//
// Implements [combat.Ledger].
func (c *Character) SpendCapacity(key combat.CapacityType, n int) {
	c.BankCapacity(key, -n)
}

// BankCapacity adds keyed capacity and marks the sheet.
//
// Adding rather than assigning is what lets an action taken twice in one turn
// bank twice — the same reason [Character.GrantCapacity], which features spend
// through, adds. The two write the same state: what a feature banked, the gate
// can spend, and the other way round.
//
// Implements [combat.Ledger].
func (c *Character) BankCapacity(key combat.CapacityType, n int) {
	if c.actionEconomy == nil || key == combat.CapacityNone {
		return
	}

	if key == combat.CapacityMovement {
		c.actionEconomy.MovementRemaining += n
		c.economyChanged()

		return
	}

	if c.actionEconomy.Granted == nil {
		c.actionEconomy.Granted = make(map[GrantedActionKey]int)
	}
	c.actionEconomy.Granted[grantedKey(key)] += n
	c.economyChanged()
}

// PoolLeft reports the points remaining in a keyed pool.
//
// A pool the sheet does not have reports zero rather than erroring, because
// zero is the answer that REFUSES a cost in it. A character with no ki must not
// be able to pay for Flurry of Blows by not paying.
//
// Implements [combat.Ledger].
func (c *Character) PoolLeft(key coreResources.ResourceKey) int {
	return c.GetResource(key).Current()
}

// SpendPool debits a keyed pool.
//
// It spends through UseResource — the choke point every feature already spends
// through, and the one that marks the sheet — rather than reaching into the
// pool directly. UseResource can refuse, but not here: the gate checked
// [Character.PoolLeft] for every pool in the profile before anything moved, so
// by this point the points are known to be there. The same precedent sits in
// the monster's turn loop, which ignores the error from an economy it has
// already checked.
//
// Implements [combat.Ledger].
func (c *Character) SpendPool(key coreResources.ResourceKey, n int) {
	_ = c.UseResource(key, n)
}
