// Copyright (C) 2026 Kirk Diggler
// SPDX-License-Identifier: GPL-3.0-or-later

package session_test

import (
	"context"
	"encoding/json"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/encounter"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/refs"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/resolution"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/resources"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/session"
)

// barbarian_rage_door_test.go plays "a barbarian rages" (rpg-project#1131,
// slice rpg-project#1281): Rage is a described feature, so Manager.Activate
// pays for it at the door.
//
// WHAT SHOWS THE DOOR PAID. Nothing a player sees differs from the path Rage
// took before, so these tests pin behaviour that is meant to be the same
// before and after and may pass at the old pins. What shows the door is used
// is that at rules v0.251.0 the feature path refuses Rage (its own code is
// retired): a barbarian who rages at all at those pins went through the
// description's door.

// ragingStatus is the saved shape of the raging condition, the fields this
// slice reads: its ref and who put it there.
type ragingStatus struct {
	Ref    string `json:"ref"`
	Origin struct {
		Ref      string `json:"ref"`
		EntityID string `json:"entity_id"`
	} `json:"origin"`
}

// ragingStatusOf is the raging condition on a stored sheet, or nil.
func ragingStatusOf(t *testing.T, stored []json.RawMessage) *ragingStatus {
	t.Helper()
	for _, raw := range stored {
		var status ragingStatus
		require.NoError(t, json.Unmarshal(raw, &status))
		if status.Ref == refs.Conditions.Raging().String() {
			return &status
		}
	}
	return nil
}

// Test 1. She rages through Manager.Activate.
func TestABarbarianRagesThroughTheDoor(t *testing.T) {
	scene := newActivationEventScene(t, ragingBarbarian("alice", 2), armedFighter("bob"))
	ref := refs.Features.Rage().String()
	require.Equal(t, 2, storedSheet(t, scene.characters, "alice").Resources[resources.RageCharges].Current)

	out, err := scene.mgr.Activate(context.Background(), &session.ActivateInput{
		Session: "sess", Member: "alice", DeclarationID: activationSelector(t, scene.mgr, "alice", ref),
	})
	require.NoError(t, err)
	require.Equal(t, ref, out.Ability)

	stored := storedSheet(t, scene.characters, "alice")
	require.NotNil(t, stored.ActionEconomy, "in a fight the economy is kept")
	require.Zero(t, stored.ActionEconomy.BonusActionsRemaining, "the bonus action is spent")
	require.Equal(t, 1, stored.Resources[resources.RageCharges].Current, "one charge is spent")
	require.Equal(t, 2, stored.Resources[resources.RageCharges].Maximum)

	status := ragingStatusOf(t, stored.Conditions)
	require.NotNil(t, status, "her sheet holds the raging status")
	require.Equal(t, ref, status.Origin.Ref, "its origin is Rage")
	require.Equal(t, "alice", status.Origin.EntityID, "and her")

	// The beats are the ones TestRageAndDashActivationResultVariants expects:
	// one activation and its result, adjacent, the result a condition applied.
	events := activationEvents(t, scene.since(t, "bob"))
	require.Equal(t, encounter.BeatActivated, events[0].Beat)
	require.Equal(t, encounter.BeatActivationResult, events[1].Beat)
	result := resultOf(t, events[1])
	require.Equal(t, "alice", result.Actor)
	require.Equal(t, encounter.ResultConditionApplied, result.Kind)
	require.Equal(t, "alice", result.Target)
	require.Equal(t, refs.Conditions.Raging().String(), result.Ref)
	require.Equal(t, "Raging", result.Name)
}

// Test 2. Spent, it is refused and nothing is spent; raging, it is refused
// again and nothing is charged.
func TestRageSpentOrAlreadyRagingIsRefusedAndNothingIsSpent(t *testing.T) {
	ctx := context.Background()
	ref := refs.Features.Rage().String()

	t.Run("no charges left", func(t *testing.T) {
		scene := newActivationEventScene(t, ragingBarbarian("alice", 2), armedFighter("bob"))
		id := activationSelector(t, scene.mgr, "alice", ref)
		spendRage(scene.characters, "alice")
		before := storedSheet(t, scene.characters, "alice")
		marks := storyLengths(t, scene.mgr, "alice", "bob", "skeleton")

		out, err := scene.mgr.Activate(ctx, &session.ActivateInput{Session: "sess", Member: "alice", DeclarationID: id})
		require.Nil(t, out)
		require.Error(t, err)
		t.Logf("rage with no charges is refused with: %v", err)
		require.ErrorIs(t, err, session.ErrStaleDeclaration)

		after := storedSheet(t, scene.characters, "alice")
		require.Equal(t, before.ActionEconomy, after.ActionEconomy, "her bonus action is untouched")
		require.Equal(t, before, after, "her sheet is unchanged")
		require.Equal(t, marks, storyLengths(t, scene.mgr, "alice", "bob", "skeleton"), "no beat is told")
	})

	t.Run("already raging", func(t *testing.T) {
		scene := newActivationEventScene(t, ragingBarbarian("alice", 2), armedFighter("bob"))
		id := activationSelector(t, scene.mgr, "alice", ref)
		_, err := scene.mgr.Activate(ctx, &session.ActivateInput{Session: "sess", Member: "alice", DeclarationID: id})
		require.NoError(t, err)
		before := storedSheet(t, scene.characters, "alice")
		require.Equal(t, 1, before.Resources[resources.RageCharges].Current)
		marks := storyLengths(t, scene.mgr, "alice", "bob", "skeleton")

		out, err := scene.mgr.Activate(ctx, &session.ActivateInput{Session: "sess", Member: "alice", DeclarationID: id})
		require.Nil(t, out)
		require.Error(t, err)
		t.Logf("rage while raging is refused with: %v", err)

		after := storedSheet(t, scene.characters, "alice")
		require.Equal(t, 1, after.Resources[resources.RageCharges].Current, "nothing more is charged")
		require.Equal(t, before, after, "her sheet is unchanged")
		require.Equal(t, marks, storyLengths(t, scene.mgr, "alice", "bob", "skeleton"), "no beat is told")
	})
}

// Test 3. A described ability named with a target it cannot take is a
// malformed activation, the same for every described ability: Second Wind and
// the fighter's Action Surge land on their user and name nobody else.
func TestADescribedAbilityNamedWithATargetIsAMalformedActivation(t *testing.T) {
	ctx := context.Background()
	requireMalformed := func(t *testing.T, err error) {
		t.Helper()
		require.ErrorIs(t, err, session.ErrBadActivation)
		require.NotErrorIs(t, err, session.ErrCannotActivate)
		require.NotErrorIs(t, err, resolution.ErrBadAction)
		require.NotErrorIs(t, err, resolution.ErrBadActivation)
	}

	t.Run("second wind", func(t *testing.T) {
		scene := newActivationEventScene(t, secondWindFighter(t, "alice", 10, 30), armedFighter("bob"))
		scene.characters.byID["alice"].HitPoints = 10
		id := activationSelector(t, scene.mgr, "alice", refs.Features.SecondWind().String())
		before := storedSheet(t, scene.characters, "alice")
		marks := storyLengths(t, scene.mgr, "alice", "bob", "skeleton")

		out, err := scene.mgr.Activate(ctx, &session.ActivateInput{
			Session: "sess", Member: "alice", DeclarationID: id, Target: "bob",
		})
		require.Nil(t, out)
		require.Error(t, err)
		t.Logf("second wind with a target answered: %v", err)
		requireMalformed(t, err)
		require.Equal(t, before, storedSheet(t, scene.characters, "alice"), "nothing is spent")
		require.Equal(t, marks, storyLengths(t, scene.mgr, "alice", "bob", "skeleton"), "no story is appended")
	})

	t.Run("action surge", func(t *testing.T) {
		s := newSurgeScene(t, levelTwoFighter(t), true)
		before := s.stored()
		marks := storyLengths(t, s.mgr, "alice", "bob")

		out, err := s.mgr.Activate(ctx, &session.ActivateInput{
			Session: "sess", Member: "alice", DeclarationID: s.row().ID, Target: "bob",
		})
		require.Nil(t, out)
		require.Error(t, err)
		t.Logf("action surge with a target answered: %v", err)
		requireMalformed(t, err)
		require.Equal(t, before, s.stored(), "nothing is spent")
		require.Equal(t, marks, storyLengths(t, s.mgr, "alice", "bob"), "no story is appended")
	})
}
