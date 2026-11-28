// Copyright (C) 2026 Kirk Diggler
// SPDX-License-Identifier: GPL-3.0-or-later

package integration

import (
	"context"
	"testing"

	"github.com/stretchr/testify/require"

	coreCombat "github.com/KirkDiggler/rpg-toolkit/core/combat"
	coreResources "github.com/KirkDiggler/rpg-toolkit/core/resources"
	"github.com/KirkDiggler/rpg-toolkit/dice"
	dnd5eCombat "github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/combat"
	dnd5eEvents "github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/events"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/features"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/healing"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/refs"
)

// descriptionLedger is the account a stand-in owner pays Second Wind's cost
// from: one bonus action for the turn, and the owner's own pool. It is the twin
// of the helper in package character, for owners that are not a Character.
type descriptionLedger struct {
	owner *mockFighterCharacter
	bonus int
}

func (l *descriptionLedger) InCombat() bool { return true }

func (l *descriptionLedger) SlotsLeft(slot coreCombat.ActionType) int {
	if slot == coreCombat.ActionBonus {
		return l.bonus
	}
	return 0
}

func (l *descriptionLedger) CapacityLeft(_ dnd5eCombat.CapacityType) int { return 0 }

func (l *descriptionLedger) PoolLeft(key coreResources.ResourceKey) int { return l.owner.pool[key] }

func (l *descriptionLedger) SpendSlots(slot coreCombat.ActionType, n int) {
	if slot == coreCombat.ActionBonus {
		l.bonus -= n
	}
}

func (l *descriptionLedger) SpendCapacity(_ dnd5eCombat.CapacityType, _ int) {}

func (l *descriptionLedger) SpendPool(key coreResources.ResourceKey, n int) {
	_ = l.owner.UseResource(key, n)
}

func (l *descriptionLedger) BankCapacity(_ dnd5eCombat.CapacityType, _ int) {}

func (l *descriptionLedger) GrantSlots(_ coreCombat.ActionType, _ int) {}

var _ dnd5eCombat.Ledger = (*descriptionLedger)(nil)

// useSecondWindFromItsDescription uses Second Wind the one way left: it builds
// the description from the feature, pays its cost at the gate (a fresh turn's
// one bonus action, then the owner's pool), and resolves the healing
// declaration with the roller. It returns the payment's error, or the resolved
// calculation. Applying the heal to hit points is the resolver's, tested there
// (rpg-project#1146).
func useSecondWindFromItsDescription(
	ctx context.Context, t *testing.T, sw *features.SecondWind, owner *mockFighterCharacter, roller dice.Roller,
) (*dnd5eEvents.RollCalculation, error) {
	t.Helper()
	definition, err := sw.Definition(owner)
	require.NoError(t, err)
	if err := dnd5eCombat.Pay(&descriptionLedger{owner: owner, bonus: 1}, definition.Cost); err != nil {
		return nil, err
	}
	resolved, err := healing.Resolve(ctx, *definition.Effect.Healing,
		dnd5eEvents.RollSource{Ref: refs.Features.SecondWind(), Name: "Second Wind", SourceID: owner.GetID()}, roller)
	require.NoError(t, err)
	return resolved, nil
}

// fixedD10Roller always rolls the same face.
type fixedD10Roller struct{ face int }

func (r *fixedD10Roller) Roll(_ context.Context, _ int) (int, error) { return r.face, nil }

func (r *fixedD10Roller) RollN(_ context.Context, count, _ int) ([]int, error) {
	faces := make([]int, count)
	for i := range faces {
		faces[i] = r.face
	}
	return faces, nil
}

var _ dice.Roller = (*fixedD10Roller)(nil)
