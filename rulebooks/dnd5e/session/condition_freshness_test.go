// Copyright (C) 2026 Kirk Diggler
// SPDX-License-Identifier: GPL-3.0-or-later

package session_test

import (
	"context"
	"slices"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/session"
)

// Condition freshness (rpg-project#520, R19): a change to a member's
// conditions refreshes the sightings of that member at commit, the way an
// equipment change does, so a held row never outlives the condition it
// describes. Conditions ride no delivered payload, so these tests watch the
// beat that tells a client to look again, and the held rows the refreshed
// testimony yields (held_effects_test.go).

const dodging = "dnd5e:conditions:dodging"

// dodge has alice take the Dodge action.
func dodge(t *testing.T, mgr *session.Manager) {
	t.Helper()
	id := activationSelector(t, mgr, "alice", "dnd5e:combat_abilities:dodge")
	_, err := mgr.Activate(context.Background(), &session.ActivateInput{Session: "sess", Member: "alice", DeclarationID: id})
	require.NoError(t, err)
}

// TestConditionEndingRefreshesWatchers: Dodging ends when alice's next turn
// starts — a change no step makes visible — and the skeleton is told to look
// at her again: a sighting "changed" beat naming her, the client's cue.
func TestConditionEndingRefreshesWatchers(t *testing.T) {
	mgr, _, _, chars := aFight(t, ragingBarbarian("alice", 2), []int{1, 1})
	dodge(t, mgr)
	before := sightedNaming(t, mgr, "skeleton", "alice")

	_, err := mgr.EndTurn(context.Background(), &session.EndTurnInput{
		Session: "sess", Member: "alice", DeclarationID: currentEndTurnID(t, mgr, "sess", "alice"),
	})
	require.NoError(t, err)
	turn, err := mgr.Turn(context.Background(), &session.TurnInput{Session: "sess", Member: "alice"})
	require.NoError(t, err)
	require.Equal(t, "alice", turn.Active, "precondition: the round came back to alice")
	require.NotContains(t, storedConditionRefs(t, chars, "alice"), dodging, "precondition: the dodge ended")

	require.Greater(t, sightedNaming(t, mgr, "skeleton", "alice"), before,
		"the watcher is told to look at alice again when her dodge ends")
}

// TestUnchangedConditionsDoNotRecheck: a write that changes no condition asks
// for no re-look of anyone whose conditions it left alone. The host's own
// Recheck of the skeleton commits like any verb; it names the skeleton, and
// alice — whose dodge is already seen — draws no beat.
func TestUnchangedConditionsDoNotRecheck(t *testing.T) {
	mgr, _, _, _ := aFight(t, ragingBarbarian("alice", 2), []int{1, 1})
	dodge(t, mgr)
	before := sightedNaming(t, mgr, "skeleton", "alice")

	_, err := mgr.Recheck(context.Background(), &session.RecheckInput{Session: "sess", Members: []string{"skeleton"}})
	require.NoError(t, err)

	require.Equal(t, before, sightedNaming(t, mgr, "skeleton", "alice"),
		"no beat names alice: her conditions did not change")
}

// sightedNaming counts the sighting beats in watcher's story that name
// subject as changed.
func sightedNaming(t *testing.T, mgr *session.Manager, watcher, subject string) int {
	t.Helper()
	story, err := mgr.Story(context.Background(), &session.StoryInput{Session: "sess", Member: watcher})
	require.NoError(t, err)
	count := 0
	for _, event := range story {
		if body, ok := event.Body.(session.SightedBody); ok && slices.Contains(body.Changed, subject) {
			count++
		}
	}
	return count
}
