// Copyright (C) 2026 Kirk Diggler
// SPDX-License-Identifier: GPL-3.0-or-later

package combat

import (
	coreResources "github.com/KirkDiggler/rpg-toolkit/core/resources"
)

// PoolSource is a budget of pools that something other than the payer owns:
// an item's uses. The gate asks it; it spends its own.
type PoolSource interface {
	// HasPool reports whether this source owns the pool. A pool it does not
	// own is the payer's.
	HasPool(key coreResources.ResourceKey) bool
	// PoolLeft reports the points left in a pool this source owns.
	PoolLeft(key coreResources.ResourceKey) int
	// SpendPool debits a pool this source owns. Like Ledger's debits it
	// returns no error: the gate checked first.
	SpendPool(key coreResources.ResourceKey, n int)
}

// WithPools returns a ledger that pays the pools source owns from source and
// everything else from base. A pool both own is the source's. A nil source
// returns base itself; a nil base returns an untyped nil Ledger, which [Pay]
// refuses as it refuses any missing ledger.
func WithPools(base Ledger, source PoolSource) Ledger {
	if base == nil {
		return nil
	}
	if source == nil {
		return base
	}
	return &pooledLedger{Ledger: base, source: source}
}

// pooledLedger answers the two pool methods itself and leaves every other
// Ledger method to the embedded base.
type pooledLedger struct {
	Ledger
	source PoolSource
}

func (l *pooledLedger) PoolLeft(key coreResources.ResourceKey) int {
	if l.source.HasPool(key) {
		return l.source.PoolLeft(key)
	}
	return l.Ledger.PoolLeft(key)
}

func (l *pooledLedger) SpendPool(key coreResources.ResourceKey, n int) {
	if l.source.HasPool(key) {
		l.source.SpendPool(key, n)
		return
	}
	l.Ledger.SpendPool(key, n)
}
