// Copyright (C) 2026 Kirk Diggler
// SPDX-License-Identifier: GPL-3.0-or-later

package monster

import (
	coreCombat "github.com/KirkDiggler/rpg-toolkit/core/combat"
	coreResources "github.com/KirkDiggler/rpg-toolkit/core/resources"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/combat"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/kits"
)

// A sheet is a [combat.Ledger].
//
// The gate spends from characters and monsters through one surface and is not
// allowed to learn which it has. A monster's account is the same
// [combat.ActionEconomy] a character's is built from, held on the sheet as
// [Data.Economy]: nil is a full turn (one action, one bonus action, one
// reaction), so a monster that has spent nothing keeps nothing. Reads never
// create the account and never mark the sheet, so [combat.CanPay] stays a
// pure read; the first write creates it and marks the sheet, because a spend
// that does not dirty the sheet is dropped by the write-back (#1087).
//
// Added 2026-11-26 (rpg-project#1131): nothing charges a monster an action or a
// bonus action yet; this is the account they will be charged to.
var _ combat.Ledger = (*Monster)(nil)

// InCombat is always true: a monster's account is always there, and an absent
// one is a full one. Unlike a character, whose sheet outside a fight carries no
// economy and refuses every payment, a monster has no state in which it is not
// holding a turn.
//
// Implements [combat.Ledger].
func (m *Monster) InCombat() bool {
	return true
}

// account is the economy a read answers from: the sheet's own, or a full turn
// when it holds none. It never stores what it builds.
func (m *Monster) account() *combat.ActionEconomy {
	return accountOf(m.economy)
}

// accountOf is the one place a read learns what a sheet's economy says: the
// economy itself, or a full turn when the sheet holds none. The loaded creature
// and a stored sheet's read-only ledger both answer from it, so they cannot
// disagree about what a nil account means. It never stores what it builds.
func accountOf(economy *combat.ActionEconomy) *combat.ActionEconomy {
	if economy == nil {
		return combat.NewActionEconomy()
	}

	return economy
}

// writable is the account a write changes: it creates a full turn on the first
// write and marks the sheet dirty.
func (m *Monster) writable() *combat.ActionEconomy {
	if m.economy == nil {
		m.economy = combat.NewActionEconomy()
	}
	m.dirty = true

	return m.economy
}

// SlotsLeft reports the per-turn slots remaining, from the account or from a
// full turn when there is none. It creates nothing and marks nothing.
//
// Implements [combat.Ledger].
func (m *Monster) SlotsLeft(slot coreCombat.ActionType) int {
	return m.account().SlotsLeft(slot)
}

// SpendSlots debits per-turn slots, creating the account on first write and
// marking the sheet. A reaction is FLOORED at zero, for the reason
// [character.Character.SpendSlots] gives: the meter itself is the guard against
// a duplicate bill, and a sheet driven to -1 would read as owing a reaction.
//
// Implements [combat.Ledger].
func (m *Monster) SpendSlots(slot coreCombat.ActionType, n int) {
	economy := m.writable()

	if slot == coreCombat.ActionReaction {
		economy.ReactionsRemaining = max(0, economy.ReactionsRemaining-n)

		return
	}

	economy.SpendSlots(slot, n)
}

// CapacityLeft reports how much of a keyed capacity remains, from the account
// or from a full turn when there is none. It creates nothing and marks nothing.
//
// Implements [combat.Ledger].
func (m *Monster) CapacityLeft(key combat.CapacityType) int {
	return m.account().CapacityLeft(key)
}

// SpendCapacity debits keyed capacity, creating the account on first write and
// marking the sheet.
//
// Implements [combat.Ledger].
func (m *Monster) SpendCapacity(key combat.CapacityType, n int) {
	m.writable().SpendCapacity(key, n)
}

// BankCapacity adds keyed capacity, creating the account on first write and
// marking the sheet.
//
// Implements [combat.Ledger].
func (m *Monster) BankCapacity(key combat.CapacityType, n int) {
	m.writable().BankCapacity(key, n)
}

// PoolKey is the key of one of a creature's own pools, as a cost names it and
// as this sheet's ledger answers it. It is compared whole, never parsed back.
func PoolKey(pool string) coreResources.ResourceKey {
	return coreResources.ResourceKey("kit:" + pool)
}

// poolNamed finds the pool a key names: the pool's name, when key is
// [PoolKey] of a pool the kit declares. ok is false for any other key.
func (m *Monster) poolNamed(key coreResources.ResourceKey) (name string, ok bool) {
	return poolNamedIn(m.kit, key)
}

// poolNamedIn is [Monster.poolNamed] over a kit alone, so a stored sheet finds
// the pool a key names by the same code the loaded creature does.
func poolNamedIn(kit *kits.Kit, key coreResources.ResourceKey) (name string, ok bool) {
	if kit == nil {
		return "", false
	}
	for pool := range kit.Pools {
		if PoolKey(pool) == key {
			return pool, true
		}
	}

	return "", false
}

// PoolLeft answers what is left in the creature's own pool that key names, and
// zero for any other key: a cost that names a pool this creature does not keep
// is refused whole rather than charged to nothing. It creates nothing and marks
// nothing.
//
// Implements [combat.Ledger].
func (m *Monster) PoolLeft(key coreResources.ResourceKey) int {
	name, ok := m.poolNamed(key)
	if !ok {
		return 0
	}

	return m.left[name]
}

// SpendPool debits the creature's own pool that key names, floored at zero, and
// marks the sheet. For any other key it does nothing: the gate never reaches it,
// because [Monster.PoolLeft] answers zero and the cost is refused first.
//
// Implements [combat.Ledger].
func (m *Monster) SpendPool(key coreResources.ResourceKey, n int) {
	name, ok := m.poolNamed(key)
	if !ok {
		return
	}
	m.left[name] = max(0, m.left[name]-n)
	m.dirty = true
}

// copyEconomy returns a copy of an account, nil for nil. The live sheet and a
// [Data] never share the pointer: whichever side changes it, the other keeps
// what it was given.
func copyEconomy(economy *combat.ActionEconomy) *combat.ActionEconomy {
	if economy == nil {
		return nil
	}

	copied := *economy

	return &copied
}

// cloneKit returns a copy of a kit, nil for nil. The live sheet and a [Data]
// never share a map, slice or byte of it.
func cloneKit(kit *kits.Kit) *kits.Kit {
	if kit == nil {
		return nil
	}

	copied := kit.Clone()

	return &copied
}

// GrantSlots adds per-turn slots, creating the account on first write and
// marking the sheet.
//
// Implements [combat.Ledger].
func (m *Monster) GrantSlots(slot coreCombat.ActionType, n int) {
	m.writable().GrantSlots(slot, n)
}
