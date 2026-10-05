// Copyright (C) 2026 Kirk Diggler
// SPDX-License-Identifier: GPL-3.0-or-later

package session_test

import (
	"context"
	"encoding/json"
	"slices"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/session"
)

// Condition freshness (rpg-project#520, R19): a change to a member's
// conditions refreshes the sightings of that member at commit, the way an
// equipment change does, so a held row never outlives the condition it
// describes. These tests watch alice through the skeleton's own sighting of
// her — testimony, read through View — with nobody moving.

const dodging = "dnd5e:conditions:dodging"

// seenConditionsOf is what watcher's current sighting of subject says the
// subject holds: the refs in the sighting's "conditions", or nil when the
// watcher holds no sighting of them.
func seenConditionsOf(t *testing.T, mgr *session.Manager, watcher, subject string) []string {
	t.Helper()
	seen, err := mgr.View(context.Background(), &session.ViewInput{Session: "sess", Member: watcher})
	require.NoError(t, err)
	for _, sighting := range seen.Sightings {
		if sighting.Subject != subject {
			continue
		}
		var payload struct {
			Conditions []struct {
				Ref string `json:"ref"`
			} `json:"conditions"`
		}
		require.NoError(t, json.Unmarshal(sighting.Payload, &payload))
		refs := make([]string, 0, len(payload.Conditions))
		for _, held := range payload.Conditions {
			refs = append(refs, held.Ref)
		}
		return refs
	}
	return nil
}

// dodge has alice take the Dodge action.
func dodge(t *testing.T, mgr *session.Manager) {
	t.Helper()
	id := activationSelector(t, mgr, "alice", "dnd5e:combat_abilities:dodge")
	_, err := mgr.Activate(context.Background(), &session.ActivateInput{Session: "sess", Member: "alice", DeclarationID: id})
	require.NoError(t, err)
}

// TestConditionChangeRechecksWatchers: alice dodges, standing still, and the
// skeleton's sighting of her carries Dodging when the verb commits.
func TestConditionChangeRechecksWatchers(t *testing.T) {
	mgr, _, _, chars := aFight(t, ragingBarbarian("alice", 2), []int{1, 1})
	require.NotContains(t, seenConditionsOf(t, mgr, "skeleton", "alice"), dodging, "precondition: nobody has seen a dodge")

	dodge(t, mgr)

	require.Contains(t, storedConditionRefs(t, chars, "alice"), dodging, "precondition: the sheet holds it")
	require.Contains(t, seenConditionsOf(t, mgr, "skeleton", "alice"), dodging,
		"the watcher's sighting carries the new condition with nobody moving")
}

// TestConditionEndingRemovesItFromSightings: Dodging ends when alice's next
// turn starts, and the skeleton's sighting of her stops carrying it.
func TestConditionEndingRemovesItFromSightings(t *testing.T) {
	mgr, _, _, chars := aFight(t, ragingBarbarian("alice", 2), []int{1, 1})
	dodge(t, mgr)
	require.Contains(t, seenConditionsOf(t, mgr, "skeleton", "alice"), dodging, "precondition")

	_, err := mgr.EndTurn(context.Background(), &session.EndTurnInput{
		Session: "sess", Member: "alice", DeclarationID: currentEndTurnID(t, mgr, "sess", "alice"),
	})
	require.NoError(t, err)
	turn, err := mgr.Turn(context.Background(), &session.TurnInput{Session: "sess", Member: "alice"})
	require.NoError(t, err)
	require.Equal(t, "alice", turn.Active, "precondition: the round came back to alice")
	require.NotContains(t, storedConditionRefs(t, chars, "alice"), dodging, "precondition: the dodge ended")

	require.NotContains(t, seenConditionsOf(t, mgr, "skeleton", "alice"), dodging,
		"the watcher's sighting no longer carries an ended condition")
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
