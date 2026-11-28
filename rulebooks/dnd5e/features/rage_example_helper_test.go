// Copyright (C) 2026 Kirk Diggler
// SPDX-License-Identifier: GPL-3.0-or-later

package features_test

import (
	"context"
	"fmt"

	coreCombat "github.com/KirkDiggler/rpg-toolkit/core/combat"
	coreResources "github.com/KirkDiggler/rpg-toolkit/core/resources"
	"github.com/KirkDiggler/rpg-toolkit/events"
	dnd5eCombat "github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/combat"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/conditions"
	dnd5eEvents "github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/events"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/features"
)

// mockLedger is the account the mock character pays a description's cost
// from: one bonus action for the turn, and the character's own resources.
type mockLedger struct {
	owner *mockCharacter
	bonus int
}

func (l *mockLedger) InCombat() bool { return true }

func (l *mockLedger) SlotsLeft(slot coreCombat.ActionType) int {
	if slot == coreCombat.ActionBonus {
		return l.bonus
	}
	return 0
}

func (l *mockLedger) CapacityLeft(_ dnd5eCombat.CapacityType) int { return 0 }

func (l *mockLedger) PoolLeft(key coreResources.ResourceKey) int { return l.owner.resources[key] }

func (l *mockLedger) SpendSlots(slot coreCombat.ActionType, n int) {
	if slot == coreCombat.ActionBonus {
		l.bonus -= n
	}
}

func (l *mockLedger) SpendCapacity(_ dnd5eCombat.CapacityType, _ int) {}

func (l *mockLedger) SpendPool(key coreResources.ResourceKey, n int) { _ = l.owner.UseResource(key, n) }

func (l *mockLedger) BankCapacity(_ dnd5eCombat.CapacityType, _ int) {}

func (l *mockLedger) GrantSlots(_ coreCombat.ActionType, _ int) {}

var _ dnd5eCombat.Ledger = (*mockLedger)(nil)

// rageFromItsDescription stands in for the resolver's effect machine
// (rulebooks/dnd5e/resolution/effect.go), which this module cannot import. It
// does what the door does, in the door's order, from the DESCRIPTION alone:
// the feature's description, the status built first from its effect, the cost
// paid at the gate, and the status published to its user as a feature's.
func rageFromItsDescription(
	ctx context.Context, feature features.Feature, owner *mockCharacter, bus events.EventBus,
) error {
	described, ok := feature.(features.Described)
	if !ok {
		return fmt.Errorf("%s carries no description", feature.GetID())
	}
	definition, err := described.Definition(owner)
	if err != nil {
		return err
	}
	status := definition.Effect.Condition
	built, err := conditions.CreateFromRef(&conditions.CreateFromRefInput{
		Ref:      status.Ref.String(),
		Config:   status.Parameters,
		MemberID: owner.GetID(),
		Origin:   &dnd5eEvents.ConditionOrigin{Ref: definition.Ref, EntityID: owner.GetID()},
	})
	if err != nil {
		return err
	}
	if err := dnd5eCombat.Pay(&mockLedger{owner: owner, bonus: 1}, definition.Cost); err != nil {
		return err
	}
	return dnd5eEvents.ConditionAppliedTopic.On(bus).Publish(ctx, dnd5eEvents.ConditionAppliedEvent{
		Target: owner, Source: dnd5eEvents.ConditionSourceFeature, Condition: built.Condition,
	})
}
