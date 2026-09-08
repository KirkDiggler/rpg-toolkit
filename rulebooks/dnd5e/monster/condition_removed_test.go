// Copyright (C) 2026 Kirk Diggler
// SPDX-License-Identifier: GPL-3.0-or-later

package monster_test

import (
	"context"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/KirkDiggler/rpg-toolkit/events"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/conditions"
	dnd5eEvents "github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/events"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/monster"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/refs"
)

// The monster keeper's half of rpg-project#407's hop 2, and it fails on the
// code it shipped with for the same reason the character's does: dropping a
// condition from the sheet's slice without calling Remove leaves it subscribed,
// still answering events from a list it is no longer in.
//
// A monster does not cast today (R12), so the condition here is a stand-in for
// a child a caster's concentration will strip off a monster's sheet. The fix is
// about the removal FACT, not about casting, which is why both keepers get it.
func TestAPrunedConditionIsUnsubscribedFromAMonster(t *testing.T) {
	ctx := context.Background()
	bus := events.NewEventBus()

	m, err := monster.Load(ctx, &monster.Data{
		ID: "skeleton-1", Name: "Skeleton", HitPoints: 13, MaxHitPoints: 13, ArmorClass: 13,
	})
	require.NoError(t, err)
	require.NoError(t, m.SheetKeeper().Apply(ctx, bus))

	hold := conditions.NewConcentratingCondition(
		"skeleton-1", refs.Spells.TrueStrike().String(), conditions.TrueStrikeName, 2)
	require.NoError(t, dnd5eEvents.ConditionAppliedTopic.On(bus).Publish(ctx,
		dnd5eEvents.ConditionAppliedEvent{Target: m, Condition: hold}))
	require.Len(t, m.GetConditions(), 1)

	before := &dnd5eEvents.DamageTakenEvent{MemberID: "skeleton-1", Amount: 12}
	require.NoError(t, dnd5eEvents.DamageTakenTopic.On(bus).Publish(ctx, before))
	require.Len(t, before.FollowUps, 1,
		"the fixture has to answer damage before removal, or this proves nothing")

	require.NoError(t, dnd5eEvents.ConditionRemovedTopic.On(bus).Publish(ctx,
		dnd5eEvents.ConditionRemovedEvent{
			MemberID:     "skeleton-1",
			ConditionRef: refs.Conditions.Concentrating().String(),
			Reason:       conditions.ConcentrationEndedRecast,
		}))

	require.Empty(t, m.GetConditions(), "dropped from the sheet")

	after := &dnd5eEvents.DamageTakenEvent{MemberID: "skeleton-1", Amount: 12}
	require.NoError(t, dnd5eEvents.DamageTakenTopic.On(bus).Publish(ctx, after))
	require.Empty(t, after.FollowUps,
		"AND off the bus — a condition that keeps answering events was never really removed")
}
