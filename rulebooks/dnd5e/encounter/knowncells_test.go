// Copyright (C) 2026 Kirk Diggler
// SPDX-License-Identifier: GPL-3.0-or-later

package encounter_test

// knowncells_test.go is KNOWLEDGE WRITTEN ONCE, COMPACTLY (rpg-project#753,
// S4): a member's `known:cells` facts are row runs, a run saved before that
// still folds from its one-cell-per-entry facts, and a seat's revealed-cells
// beat carries only what the seat did not already know.

import (
	"encoding/json"
	"strings"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/encounter"
	"github.com/KirkDiggler/rpg-toolkit/tools/spatial"
)

// savedData is the run as the session stores it: marshalled and read back.
func savedData(t testing.TB, enc *encounter.Encounter) (encounter.EncounterData, int) {
	t.Helper()
	raw, err := json.Marshal(enc.ToData())
	require.NoError(t, err)
	var data encounter.EncounterData
	require.NoError(t, json.Unmarshal(raw, &data))
	return data, len(raw)
}

// legacySpelled is data with every `known:cells` fact rewritten in the
// one-cell-per-entry form a run saved before S4 carries.
func legacySpelled(t *testing.T, data encounter.EncounterData) encounter.EncounterData {
	t.Helper()
	raw, err := json.Marshal(data)
	require.NoError(t, err)
	var out encounter.EncounterData
	require.NoError(t, json.Unmarshal(raw, &out))
	require.NotNil(t, out.Journal, "the run has a journal")
	rewritten := 0
	for i, f := range out.Journal.Facts {
		if f.Kind != encounter.ExportedCellsKnownKind {
			continue
		}
		cells, err := encounter.ExportedDecodeCellsKnown(f.Detail)
		require.NoError(t, err)
		legacy, err := json.Marshal(map[string][]spatial.Position{"cells": cells})
		require.NoError(t, err)
		out.Journal.Facts[i].Detail = string(legacy)
		rewritten++
	}
	require.Positive(t, rewritten, "precondition: the run has known-cells facts to respell")
	return out
}

func reloadKnowledge(t *testing.T, data encounter.EncounterData) *encounter.Encounter {
	t.Helper()
	enc, err := encounter.LoadEncounter(&encounter.LoadEncounterInput{
		Data: data, Capabilities: knowledgeCapabilities(&sightList{fallback: 20}),
	})
	require.NoError(t, err)
	return enc
}

// knowledgeOf is every ever-member's folded known set.
func knowledgeOf(enc *encounter.Encounter, members []encounter.MemberID) map[encounter.MemberID]map[spatial.Position]bool {
	out := make(map[encounter.MemberID]map[spatial.Position]bool, len(members))
	for _, m := range members {
		out[m] = enc.ExportedKnownCells(m)
	}
	return out
}

// TestKnownCellsFact_RunsRoundTrip: the same run saved with its facts as row
// runs and as one cell per entry folds to the same known set for every
// member, and so to the same atlas; the legacy run, played on, writes runs
// and still agrees with the run that never was legacy.
func TestKnownCellsFact_RunsRoundTrip(t *testing.T) {
	enc := monsterRooms(t)
	_, err := enc.OpenDoor(&encounter.OpenDoorInput{Door: "far-door"})
	require.NoError(t, err)
	_, err = enc.OpenDoor(&encounter.OpenDoorInput{Door: "entry-door"})
	require.NoError(t, err)
	data, _ := savedData(t, enc)
	for _, f := range data.Journal.Facts {
		if f.Kind == encounter.ExportedCellsKnownKind {
			require.True(t, strings.HasPrefix(f.Detail, `{"rows":`), "this module writes row runs: %s", f.Detail)
		}
	}
	members := data.EverMembers
	want := knowledgeOf(enc, members)
	for _, m := range members {
		require.NotEmpty(t, want[m], "precondition: %s knows some floor", m)
	}

	runs := reloadKnowledge(t, data)
	legacy := reloadKnowledge(t, legacySpelled(t, data))
	require.Equal(t, want, knowledgeOf(runs, members), "row runs fold to what was learned")
	require.Equal(t, want, knowledgeOf(legacy, members), "the legacy spelling folds to the same")
	require.Equal(t, knownTo(t, runs, sweepHero), knownTo(t, legacy, sweepHero), "and draws the same atlas")

	// Played on: the hero walks into the room on both, and the legacy run's
	// new facts are runs folded beside its old ones.
	for _, played := range []*encounter.Encounter{runs, legacy} {
		_, err := played.Step(&encounter.StepInput{Member: sweepHero, To: cellAt(3, 1)})
		require.NoError(t, err)
	}
	require.Equal(t, knowledgeOf(runs, members), knowledgeOf(legacy, members), "a legacy run played on agrees")
	mixed, _ := savedData(t, legacy)
	forms := map[bool]int{}
	for _, f := range mixed.Journal.Facts {
		if f.Kind == encounter.ExportedCellsKnownKind {
			forms[strings.HasPrefix(f.Detail, `{"rows":`)]++
		}
	}
	require.Positive(t, forms[true], "the legacy run wrote runs once played on")
	require.Positive(t, forms[false], "and kept its legacy facts as they were")
	require.Equal(t, knowledgeOf(legacy, members), knowledgeOf(reloadKnowledge(t, mixed), members),
		"and the mixed journal reloads to the same known sets")
}

// TestKnownCellsFact_LoadRefusesMalformedRuns: the load boundary decodes runs
// as the fold does and refuses a fact it could not have written.
func TestKnownCellsFact_LoadRefusesMalformedRuns(t *testing.T) {
	enc := monsterRooms(t)
	data, _ := savedData(t, enc)
	at := -1
	for i, f := range data.Journal.Facts {
		if f.Kind == encounter.ExportedCellsKnownKind && f.Actor == string(sweepHero) {
			at = i
		}
	}
	require.NotEqual(t, -1, at, "precondition: the hero learned at first light")
	floor, err := encounter.ExportedDecodeCellsKnown(data.Journal.Facts[at].Detail)
	require.NoError(t, err)
	q, r := int(floor[0].X), int(floor[0].Y)
	cell := func(x, y int) string { return `{"x":` + itoa(x) + `,"y":` + itoa(y) + `}` }
	cases := map[string]string{
		"rows out of order":        `{"rows":[{"r":` + itoa(r) + `,"q":[[` + itoa(q) + `,` + itoa(q) + `]]},{"r":` + itoa(r) + `,"q":[[` + itoa(q) + `,` + itoa(q) + `]]}]}`,
		"spans that touch":         `{"rows":[{"r":` + itoa(r) + `,"q":[[` + itoa(q) + `,` + itoa(q) + `],[` + itoa(q+1) + `,` + itoa(q+1) + `]]}]}`,
		"a descending span":        `{"rows":[{"r":` + itoa(r) + `,"q":[[` + itoa(q+1) + `,` + itoa(q) + `]]}]}`,
		"a row with no span":       `{"rows":[{"r":` + itoa(r) + `,"q":[]}]}`,
		"a span off the floor":     `{"rows":[{"r":` + itoa(r) + `,"q":[[` + itoa(q) + `,` + itoa(q+500) + `]]}]}`,
		"a span of a billion":      `{"rows":[{"r":` + itoa(r) + `,"q":[[-1000000000,1000000000]]}]}`,
		"both spellings":           `{"cells":[` + cell(q, r) + `],"rows":[{"r":` + itoa(r) + `,"q":[[` + itoa(q) + `,` + itoa(q) + `]]}]}`,
		"neither spelling":         `{}`,
		"a legacy cell off floor":  `{"cells":[` + cell(q+500, r) + `]}`,
		"no rows and no cells set": `{"rows":[]}`,
	}
	for name, detail := range cases {
		edited := data
		edited.Journal = &encounter.JournalData{Facts: append([]encounter.FactData(nil), data.Journal.Facts...)}
		edited.Journal.Facts[at].Detail = detail
		_, err := encounter.LoadEncounter(&encounter.LoadEncounterInput{
			Data: edited, Capabilities: knowledgeCapabilities(&sightList{fallback: 20}),
		})
		require.ErrorIs(t, err, encounter.ErrInvalidData, name)
	}
}

func itoa(v int) string {
	raw, _ := json.Marshal(v)
	return string(raw)
}

// revealedCells is every cell one cells-revealed beat lists — its regions'
// cells and its scenery — and the ids of the placements it introduces.
type revealed struct {
	cells  []spatial.Position
	placed []string
}

func revealsSince(t *testing.T, enc *encounter.Encounter, member encounter.MemberID, after int) ([]revealed, int) {
	t.Helper()
	story, err := enc.Story(&encounter.StoryInput{Audience: member})
	require.NoError(t, err)
	var out []revealed
	for _, entry := range story[after:] {
		var beat struct {
			Beat    string `json:"beat"`
			Regions []struct {
				Cells []spatial.Position `json:"cells"`
			} `json:"regions"`
			Scenery []spatial.Position `json:"scenery"`
			Placed  []struct {
				ID string `json:"id"`
			} `json:"placed"`
		}
		require.NoError(t, json.Unmarshal(entry.Payload, &beat))
		if beat.Beat != encounter.BeatCellsRevealed {
			continue
		}
		r := revealed{cells: append([]spatial.Position(nil), beat.Scenery...)}
		for _, region := range beat.Regions {
			r.cells = append(r.cells, region.Cells...)
		}
		for _, p := range beat.Placed {
			r.placed = append(r.placed, p.ID)
		}
		out = append(out, r)
	}
	return out, len(story)
}

// TestRevealBeat_DeltaOnly: each learning moment's beat lists only cells the
// seat did not know and placements its map did not hold, every one of which
// it knows afterwards; the map at the scene's opening plus the beats is the
// map now, and a reload from the saved run draws that same map from the facts
// alone.
func TestRevealBeat_DeltaOnly(t *testing.T) {
	enc := monsterRooms(t)
	_, read := revealsSince(t, enc, sweepHero, 0)
	start, err := enc.AtlasFor(sweepHero)
	require.NoError(t, err)
	drawn := cellSet(start.Cells)
	moments := []func() error{
		func() error { _, err := enc.OpenDoor(&encounter.OpenDoorInput{Door: "entry-door"}); return err },
		func() error { _, err := enc.OpenDoor(&encounter.OpenDoorInput{Door: "far-door"}); return err },
		func() error {
			_, err := enc.Step(&encounter.StepInput{Member: sweepHero, To: cellAt(3, 1)})
			return err
		},
	}
	beats := 0
	for i, moment := range moments {
		knew := enc.ExportedKnownCells(sweepHero)
		before, err := enc.AtlasFor(sweepHero)
		require.NoError(t, err)
		hadPlaced := map[string]bool{}
		for _, p := range before.Placed {
			hadPlaced[string(p.ID)] = true
		}
		require.NoError(t, moment(), "moment %d", i)
		var got []revealed
		got, read = revealsSince(t, enc, sweepHero, read)
		knows := enc.ExportedKnownCells(sweepHero)
		for _, beat := range got {
			beats++
			for _, cell := range beat.cells {
				require.False(t, knew[cell], "moment %d: %v was already known and is not sent again", i, cell)
				require.False(t, drawn[cell], "moment %d: %v was already drawn and is not sent again", i, cell)
				require.True(t, knows[cell], "moment %d: %v is sent because it is now known", i, cell)
				drawn[cell] = true
			}
			for _, id := range beat.placed {
				require.False(t, hadPlaced[id], "moment %d: placement %s was already on the map", i, id)
			}
		}
	}
	require.Positive(t, beats, "precondition: the hero learned after the scene opened")
	now, err := enc.AtlasFor(sweepHero)
	require.NoError(t, err)
	require.Equal(t, cellSet(now.Cells), drawn, "the opening map plus the beats is the map now")

	data, _ := savedData(t, enc)
	reloaded := reloadKnowledge(t, data)
	require.Equal(t, enc.ExportedKnownCells(sweepHero), reloaded.ExportedKnownCells(sweepHero),
		"the reload folds the same known set from the facts alone")
	require.Equal(t, cellSet(now.Cells), knownTo(t, reloaded, sweepHero), "and draws the same map")
}

// TestBlobSize_Keep10k: the 10k keep's saved run after the session's launch
// — every seat and every monster boarded and looking — is under a megabyte,
// and reloading it folds every member's known set unchanged. Before S4 the
// same run saved 2.16 MB, 1.37 MB of it the known-cells facts written one
// cell per entry. It reads the launch [TestMain] starts beside the serial
// tests, and changes nothing in it.
func TestBlobSize_Keep10k(t *testing.T) {
	enc := launchedKeep10k(t)
	data, size := savedData(t, enc)
	require.Less(t, size, 1_000_000, "the launched keep's saved run is under 1 MB")

	members := data.EverMembers
	require.Greater(t, len(members), len(keepPlayers), "precondition: the garrison boarded")
	reloaded, err := encounter.LoadEncounter(&encounter.LoadEncounterInput{
		Data: data, Capabilities: knowledgeCapabilities(&sightList{fallback: 24}),
	})
	require.NoError(t, err)
	want := knowledgeOf(enc, members)
	for _, p := range keepPlayers {
		require.NotEmpty(t, want[p], "precondition: seat %s knows floor", p)
	}
	require.Equal(t, want, knowledgeOf(reloaded, members), "every member's known set survives the save")
}
