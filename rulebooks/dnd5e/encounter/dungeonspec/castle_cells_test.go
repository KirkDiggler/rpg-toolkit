// Copyright (C) 2026 Kirk Diggler
// SPDX-License-Identifier: GPL-3.0-or-later

package dungeonspec_test

// castle_cells_test.go is R12 ON THE ROOM IT WAS FOUND ON (rpg-project#569):
// the mad king's cells, rpg-api's shipped content with `opened: { fact:
// cells-opened }` on the cell door and an ending `freed` waiting on that
// fact. Before R12 that ending could not be written; here it compiles, and
// picking the lock ends the run freed with the guards still standing.

import (
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/encounter"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/encounter/dungeonspec"
	"github.com/KirkDiggler/rpg-toolkit/tools/spatial"
)

const (
	cellsFixture = "testdata/world-builder-v4-castle-cells.yaml"
	cellsDoor    = encounter.DoorID("the-mad-king-cells/16556f61-2915-4656-ab30-d2445dff0174")
	cellsPicker  = encounter.MemberID("picker")
)

// cellsPick is the cell door's one authored route, as the file writes it.
var cellsPick = encounter.CheckApproach{Ability: "dex", Tool: "dnd5e:item:thieves-tools", DC: 13}

// TestTheCellDoorCompilesItsFact: the attached door lowers with the fact,
// and the ending is the file's own.
func TestTheCellDoorCompilesItsFact(t *testing.T) {
	compiled := loadContent(t, cellsFixture)

	var door *encounter.DoorInput
	for i := range compiled.Field.Doors {
		if compiled.Field.Doors[i].ID == cellsDoor {
			door = &compiled.Field.Doors[i]
		}
	}
	require.NotNil(t, door, "the cell door is an opening's attached door")
	require.Equal(t, encounter.FactID("cells-opened"), door.Opened)
	require.Contains(t, compiled.Endings,
		encounter.EndingInput{Key: "freed", Trigger: encounter.TriggerFact{Fact: "cells-opened"}})
}

// TestPickingTheCellLockEndsTheRunFreed is the walk's first route as a
// scenario: a player at the cell door picks the lock, and the run ends
// `freed` with every guard still in the run.
//
// THE PICKER STANDS AT [-8, 6], between the two guards the file posts at
// [-10, 5] and [-7, 5] on the corridor side of the cell door — a cell from
// which the door is within reach. If the file moves its door, the Unlock
// below is refused out of range and says so.
func TestPickingTheCellLockEndsTheRunFreed(t *testing.T) {
	compiled := loadContent(t, cellsFixture)
	enc := cellsRun(t, compiled, spatial.Position{X: -8, Y: 6}, everyoneSeesTheWholeMap{}, nil)

	_, err := enc.Unlock(&encounter.UnlockInput{
		Door: cellsDoor, Actor: cellsPicker, Beaten: true, Total: 18, Applied: cellsPick,
	})
	require.NoError(t, err, "the picker's cell must be within reach of the cell door")

	requireFreed(t, enc)
	for _, m := range compiled.Monsters {
		if m.Faction == "garrison" {
			require.Contains(t, memberIDsOf(t, enc), encounter.MemberID(m.MemberID),
				"no guard left the run: the lock was picked, not fought through")
		}
	}
}

// TestAGuardOpeningTheCellsOutOfThePartysSightEndsTheRunFreed is the walk's
// second route, as ruled on the plan (rpg-project PR #570, 5611748): the
// guard who opens the cell door is its actor, so he learns `cells-opened`,
// and a `{ fact }` ending fires on anyone learning its fact (the truth grain,
// design R5) — the party need not have seen it. The party stands at the
// file's own start, blind, so nothing it could see is what fires the ending.
func TestAGuardOpeningTheCellsOutOfThePartysSightEndsTheRunFreed(t *testing.T) {
	compiled := loadContent(t, cellsFixture)
	const opener = encounter.MemberID("44f4aa72-bd03-46b4-b8f4-d5dc2e895cff")
	enc := cellsRun(t, compiled, compiled.PartyStart[0].At, partyIsBlind{},
		map[encounter.MemberID]spatial.Position{opener: {X: -8, Y: 6}})

	_, err := enc.Unlock(&encounter.UnlockInput{
		Door: cellsDoor, Actor: opener, Beaten: true, Total: 18, Applied: cellsPick,
	})
	require.NoError(t, err)

	requireFreed(t, enc)
	learners := map[string]bool{}
	for _, f := range enc.ToData().Journal.Facts {
		if f.Kind == "known:fact:cells-opened" {
			learners[f.Subject] = true
		}
	}
	require.True(t, learners[string(opener)], "the guard who opened it learned it")
	require.True(t, learners["7b8b5d1c-1b8f-4ea1-a681-5abab9420d0a"], "and so did the guard who watched him do it")
	require.False(t, learners[string(cellsPicker)], "and the party, which saw nothing, did not")
}

// partyIsBlind is a sight model in which the party's one player sees nothing
// and every creature sees the whole map.
type partyIsBlind struct{}

func (partyIsBlind) Sight(members []encounter.MemberID) (map[encounter.MemberID]int, error) {
	out, err := everyoneSeesTheWholeMap{}.Sight(members)
	if _, asked := out[cellsPicker]; asked {
		out[cellsPicker] = 0
	}

	return out, err
}

// requireFreed asserts the run is over and ended `freed`.
func requireFreed(t *testing.T, enc *encounter.Encounter) {
	t.Helper()
	status, err := enc.Status()
	require.NoError(t, err)
	require.False(t, status.Open)
	require.NotNil(t, status.Outcome)
	require.Equal(t, "freed", status.Outcome.Ending)
}

// cellsRun opens the cells with the party's one player at a cell, the
// file's creatures where it posts them (or where moved puts one), and the
// sight model given.
func cellsRun(
	t *testing.T, compiled dungeonspec.Compiled, picker spatial.Position, sight encounter.Sight,
	moved map[encounter.MemberID]spatial.Position,
) *encounter.Encounter {
	t.Helper()
	members := []encounter.MemberInput{{ID: cellsPicker, Kind: encounter.KindPlayer, Position: picker}}
	for _, m := range compiled.Monsters {
		at := m.At
		if cell, ok := moved[encounter.MemberID(m.MemberID)]; ok {
			at = cell
		}
		members = append(members, encounter.MemberInput{
			ID: encounter.MemberID(m.MemberID), Kind: encounter.KindMonster, Position: at,
			Faction: encounter.FactionID(m.Faction),
		})
	}
	enc, err := encounter.NewEncounter(&encounter.SetupInput{
		Field:   compiled.Field,
		Members: members,
		Endings: append([]encounter.EndingInput{{Key: "withdrawn", Trigger: encounter.TriggerExternal{}}}, compiled.Endings...),
		Capabilities: encounter.Capabilities{
			Sight:         sight,
			Equipment:     encounter.UnobservedEquipment{},
			Conditions:    encounter.UnobservedEquipment{},
			Sheets:        zeroSheets{},
			Participation: everyoneStanding{},
			Initiative:    orderAsGiven{},
			Driver:        passDriver{},
			CheckResolver: nothingIsEverFound{},
			Witness:       nobodyPerceivesAnything{},
			Actors: encounter.Actors{
				Striker:   noAttacksExpected{},
				Mover:     quietMover{},
				Announcer: quietAnnouncer{},
			},
		},
	})
	require.NoError(t, err)

	return enc
}

// memberIDsOf is every member still in the run.
func memberIDsOf(t *testing.T, enc *encounter.Encounter) []encounter.MemberID {
	t.Helper()
	members, err := enc.Members()
	require.NoError(t, err)
	var out []encounter.MemberID
	for _, m := range members {
		out = append(out, m.ID)
	}

	return out
}
