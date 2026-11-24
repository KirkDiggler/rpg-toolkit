// Copyright (C) 2026 Kirk Diggler
// SPDX-License-Identifier: GPL-3.0-or-later

package items

import (
	coreResources "github.com/KirkDiggler/rpg-toolkit/core/resources"
)

// Budget answers for an item's own pools at a gate, for one holder. The gate
// asks; the item spends its own. It has the shape of the D&D rulebook's
// combat.PoolSource without this package importing a rulebook.
type Budget struct {
	item   *Item
	holder string
}

// Budget returns this item's pools as a budget for holderID.
func (i *Item) Budget(holderID string) *Budget {
	return &Budget{item: i, holder: holderID}
}

// HasPool reports whether the item's definition declares the pool.
func (b *Budget) HasPool(key coreResources.ResourceKey) bool {
	_, ok := b.item.def.Pools[string(key)]
	return ok
}

// held reports whether the item is attached to the holder this budget asks for.
func (b *Budget) held() bool {
	return b.item.composed != nil && b.holder == b.item.holder
}

// PoolLeft reports what is left in a pool the item declares. It is zero when
// the item is detached or attached to a different holder: an item someone
// else holds cannot pay.
func (b *Budget) PoolLeft(key coreResources.ResourceKey) int {
	if !b.held() {
		return 0
	}
	pool, ok := b.item.pools[string(key)]
	if !ok {
		return 0
	}
	return pool.Current
}

// SpendPool debits the item's own pool. It does nothing when the item is not
// attached to this holder, and never takes a pool below zero.
func (b *Budget) SpendPool(key coreResources.ResourceKey, n int) {
	if !b.held() {
		return
	}
	if pool, ok := b.item.pools[string(key)]; ok {
		_ = pool.Use(n) // refuses, and changes nothing, when n exceeds what is left
	}
}
