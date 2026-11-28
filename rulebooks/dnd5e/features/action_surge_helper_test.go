// Copyright (C) 2026 Kirk Diggler
// SPDX-License-Identifier: GPL-3.0-or-later

package features

import (
	"testing"

	"github.com/stretchr/testify/require"

	coreCombat "github.com/KirkDiggler/rpg-toolkit/core/combat"
	coreResources "github.com/KirkDiggler/rpg-toolkit/core/resources"
	dnd5eCombat "github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/combat"
)

// surgeLedger is the account a stand-in owner pays Action Surge's cost from: a
// turn that starts with one action, and the owner's own pool. It is the twin of
// the helper in package character, for owners that are not a Character.
type surgeLedger struct {
	owner   *StubEntity
	actions int
}

func (l *surgeLedger) InCombat() bool { return true }

func (l *surgeLedger) SlotsLeft(slot coreCombat.ActionType) int {
	if slot == coreCombat.ActionStandard {
		return l.actions
	}
	return 0
}

func (l *surgeLedger) CapacityLeft(_ dnd5eCombat.CapacityType) int { return 0 }

func (l *surgeLedger) PoolLeft(key coreResources.ResourceKey) int { return l.owner.resources[key] }

func (l *surgeLedger) SpendSlots(slot coreCombat.ActionType, n int) {
	if slot == coreCombat.ActionStandard {
		l.actions -= n
	}
}

func (l *surgeLedger) SpendCapacity(_ dnd5eCombat.CapacityType, _ int) {}

func (l *surgeLedger) SpendPool(key coreResources.ResourceKey, n int) {
	_ = l.owner.UseResource(key, n)
}

func (l *surgeLedger) BankCapacity(_ dnd5eCombat.CapacityType, _ int) {}

func (l *surgeLedger) GrantSlots(slot coreCombat.ActionType, n int) {
	if slot == coreCombat.ActionStandard {
		l.actions += n
	}
}

var _ dnd5eCombat.Ledger = (*surgeLedger)(nil)

// useActionSurgeFromItsDescription uses Action Surge the one way left: it builds
// the description from the feature and pays its cost at the gate. It returns
// the payment's error.
func useActionSurgeFromItsDescription(t *testing.T, as *ActionSurge, ledger *surgeLedger) error {
	t.Helper()
	definition, err := as.Definition(ledger.owner)
	require.NoError(t, err)
	return dnd5eCombat.Pay(ledger, definition.Cost)
}
