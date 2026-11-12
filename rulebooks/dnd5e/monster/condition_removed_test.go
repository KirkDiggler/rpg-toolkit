// Copyright (C) 2026 Kirk Diggler
// SPDX-License-Identifier: GPL-3.0-or-later

package monster_test

import (
	"context"
	"encoding/json"
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
func TestWrongBaneSourceDoesNotRemoveOrDirtyMonster(t *testing.T) {
	ctx := context.Background()
	bus := events.NewEventBus()
	m := monster.New(monster.Config{ID: "skeleton-1", Name: "Skeleton", HP: 13, AC: 13})
	owner := dnd5eEvents.ConditionAddress{MemberID: "bard-a", ConditionRef: refs.Conditions.LinkedEffects().String(), SourceID: "application:bard-a"}
	config, err := json.Marshal(conditions.LinkedConditionConfig{ApplicationID: owner.SourceID, AddressSourceID: "bard-a", Role: conditions.LinkedRecipient,
		Bindings: &conditions.LinkedBindings{Name: "Bane", Owner: &owner, MemberKey: "skeleton-1"}})
	require.NoError(t, err)
	bane, err := conditions.CreateFromRef(&conditions.CreateFromRefInput{Ref: refs.Conditions.Baned().String(), MemberID: "skeleton-1",
		Origin: &dnd5eEvents.ConditionOrigin{Ref: *refs.Spells.Bane(), EntityID: "bard-a"}, Config: config})
	require.NoError(t, err)
	require.NoError(t, m.AddLoadedCondition(bane.Condition))
	require.NoError(t, m.SheetKeeper().Apply(ctx, bus))
	require.False(t, m.IsDirty())

	for _, sourceID := range []string{"bard-b", ""} {
		require.NoError(t, dnd5eEvents.ConditionRemovedTopic.On(bus).Publish(ctx,
			dnd5eEvents.ConditionRemovedEvent{
				MemberID: "skeleton-1", ConditionRef: refs.Conditions.Baned().String(), SourceID: sourceID,
			}))
		require.Len(t, m.GetConditions(), 1)
		require.False(t, m.IsDirty(), "mismatched and empty source identities are exact no-ops")
	}

	output, err := m.DescribeRollContributions(&dnd5eEvents.DescribeRollContributionsInput{
		Kind: dnd5eEvents.RollKindAttack,
	})
	require.NoError(t, err)
	require.Len(t, output.Contributions, 1)
	require.Equal(t, "bard-a", output.Contributions[0].Source.SourceID)
}

func TestAPrunedConditionIsUnsubscribedFromAMonster(t *testing.T) {
	ctx := context.Background()
	bus := events.NewEventBus()

	m, err := monster.Load(ctx, &monster.Data{
		ID: "skeleton-1", Name: "Skeleton", HitPoints: 13, MaxHitPoints: 13, ArmorClass: 13,
	})
	require.NoError(t, err)
	binding, err := conditions.BindApplications(&conditions.BindApplicationsInput{Participants: []conditions.ParticipantConditions{{MemberID: "skeleton-1"}}, Bus: bus})
	require.NoError(t, err)
	defer func() { require.NoError(t, binding.Close(ctx)) }()
	require.NoError(t, m.SheetKeeper().Apply(ctx, bus))
	hold, err := conditions.CreateFromRef(&conditions.CreateFromRefInput{Ref: refs.Conditions.LinkedEffects().String(), MemberID: "skeleton-1",
		Origin: &dnd5eEvents.ConditionOrigin{Ref: *refs.Spells.TrueStrike(), EntityID: "skeleton-1"},
		Config: json.RawMessage(`{"application_id":"application:skeleton-1","address_source_id":"application:skeleton-1","role":"coordinator","bindings":{"name":"True Strike","parts":[{"key":"hold","kind":"concentration","on_terminal":"application"}]}}`)})
	require.NoError(t, err)
	require.NoError(t, dnd5eEvents.ConditionAppliedTopic.On(bus).Publish(ctx,
		dnd5eEvents.ConditionAppliedEvent{Target: m, Condition: hold.Condition}))
	require.Len(t, m.GetConditions(), 1)

	before := &dnd5eEvents.DamageTakenEvent{MemberID: "skeleton-1", Amount: 12}
	require.NoError(t, dnd5eEvents.DamageTakenTopic.On(bus).Publish(ctx, before))
	require.Len(t, before.FollowUps, 1,
		"the fixture has to answer damage before removal, or this proves nothing")

	require.NoError(t, dnd5eEvents.ConditionRemovedTopic.On(bus).Publish(ctx,
		dnd5eEvents.ConditionRemovedEvent{
			MemberID:     "skeleton-1",
			ConditionRef: refs.Conditions.LinkedEffects().String(),
			SourceID:     "application:skeleton-1",
			Reason:       conditions.ConcentrationEndedRecast,
		}))

	require.Empty(t, m.GetConditions(), "dropped from the sheet")

	after := &dnd5eEvents.DamageTakenEvent{MemberID: "skeleton-1", Amount: 12}
	require.NoError(t, dnd5eEvents.DamageTakenTopic.On(bus).Publish(ctx, after))
	require.Empty(t, after.FollowUps,
		"AND off the bus — a condition that keeps answering events was never really removed")
}
