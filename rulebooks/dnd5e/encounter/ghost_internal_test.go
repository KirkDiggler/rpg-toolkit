// Copyright (C) 2026 Kirk Diggler
// SPDX-License-Identifier: GPL-3.0-or-later

package encounter

import (
	"encoding/json"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/KirkDiggler/rpg-toolkit/mind/perception"
	"github.com/KirkDiggler/rpg-toolkit/tools/spatial"
)

// ghost_internal_test.go holds THE GHOST CORRECTION (rpg-project#961) to
// reading only the holder's own look at the cell: a hidden sneaker standing
// on their own ghost's cell gives nothing away.
//
// A PLAYER IS NEVER A STEALTH WATCHER in play ([Encounter.watchersIn] judges
// only creatures), and the correction is a player's. So the suppression is
// set down directly — the record a beaten roll leaves — to pin the property
// the rule must keep whatever the stealth rules come to judge: the
// correction never asks where the subject is.

// ghostHall is an open hall, everyone seeing it whole: the watcher at
// [1,2], the sneaker beside them at [2,2].
func ghostHall(t *testing.T) *Encounter {
	t.Helper()
	enc, err := NewEncounter(&SetupInput{
		Field: FieldInput{Canvas: openAir(), Regions: []RegionInput{rectRegion("hall", 0, 0, 10, 5)}},
		Members: []MemberInput{
			{ID: "watcher", Kind: KindPlayer, Position: spatial.Position{X: 1, Y: 2}},
			{ID: "sneaker", Kind: KindPlayer, Position: spatial.Position{X: 2, Y: 2}},
		},
		Endings:      []EndingInput{{Key: "withdrawn", Trigger: TriggerExternal{}}},
		Capabilities: escapeCapabilities(),
	})
	require.NoError(t, err)
	enc.members["sneaker"].Sneaking = true
	enc.sneakVerdicts = map[MemberID]map[MemberID]SneakVerdictData{"sneaker": {"watcher": {Beaten: true, InZone: true}}}
	return enc
}

// ghostWitness is what the watcher holds of the sneaker, and every beat the
// watcher's story carries from since on, as stored bytes.
func ghostWitness(t *testing.T, enc *Encounter, since int) ([]byte, []string) {
	t.Helper()
	held, err := enc.memberIntel("watcher")
	require.NoError(t, err)
	var sighting []byte
	for _, h := range held {
		if h.Channel == perception.Sight && h.Subject == "sneaker" {
			require.False(t, h.CurrentOn(perception.Sight))
			sighting = h.Payload
		}
	}
	story, err := enc.Story(&StoryInput{Audience: "watcher"})
	require.NoError(t, err)
	var beats []string
	for _, entry := range story[since:] {
		// THE MOVER'S OWN RECORD IS NOT SIGHT: a `moved` beat goes to the
		// mover's party whether or not anybody sees the step (a player
		// watcher is the sneaker's party here, the fixture's artefact). The
		// comparison is of what the watcher's look told them.
		var b struct{ Beat string }
		require.NoError(t, json.Unmarshal(entry.Payload, &b))
		if b.Beat == BeatMoved {
			continue
		}
		beats = append(beats, string(entry.Payload))
	}
	return sighting, beats
}

// TestASneakerOnTheirGhostGivesNothingAway: two runs alike, the sneaker
// unseen by the watcher in both. In one they stay on the cell the watcher
// last saw them on; in the other they walk away. The watcher holds the same
// unknown sighting and is told the same beats, byte for byte.
func TestASneakerOnTheirGhostGivesNothingAway(t *testing.T) {
	stays, gone := ghostHall(t), ghostHall(t)
	story, err := stays.Story(&StoryInput{Audience: "watcher"})
	require.NoError(t, err)
	since := len(story)

	_, _, err = stays.refreshSight(stays.rosterIDs())
	require.NoError(t, err)
	_, err = gone.Step(&StepInput{Member: "sneaker", To: gone.field.cellAt(spatial.Position{X: 8, Y: 2}), EndWalk: true})
	require.NoError(t, err)
	_, _, err = gone.refreshSight(gone.rosterIDs())
	require.NoError(t, err)

	staysSeen, staysBeats := ghostWitness(t, stays, since)
	goneSeen, goneBeats := ghostWitness(t, gone, since)
	testimony, ok := DecodeSightTestimony(staysSeen)
	require.True(t, ok)
	require.Equal(t, LocationUnknown, testimony.State, "the watcher sees the cell, and nobody on it")
	require.Equal(t, string(goneSeen), string(staysSeen), "the same holding, whether or not they stayed")
	want, _ := json.Marshal(goneBeats)
	got, _ := json.Marshal(staysBeats)
	require.Equal(t, string(want), string(got), "and the same story")
}
