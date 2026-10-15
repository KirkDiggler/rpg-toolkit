// Copyright (C) 2026 Kirk Diggler
// SPDX-License-Identifier: GPL-3.0-or-later

package dungeonspec_test

// castle_cells_test.go is R12 ON THE ROOM IT WAS FOUND ON (rpg-project#569):
// the mad king's cells, rpg-api's shipped content with `opened: { fact:
// cells-opened }` on the cell door and an ending `freed` waiting on that
// fact. Before R12 that ending could not be written; here it compiles, and
// picking the lock ends the run freed with the guards still standing.

import (
	"errors"
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

	enc, ok := cellsRunWithPickerAt(t, compiled, spatial.Position{X: -8, Y: 6})
	require.True(t, ok, "the picker's cell must be standable and within reach of the cell door")

	status, err := enc.Status()
	require.NoError(t, err)
	require.False(t, status.Open)
	require.NotNil(t, status.Outcome)
	require.Equal(t, "freed", status.Outcome.Ending)
	for _, m := range compiled.Monsters {
		if m.Faction == "garrison" {
			require.Contains(t, memberIDsOf(t, enc), encounter.MemberID(m.MemberID),
				"no guard left the run: the lock was picked, not fought through")
		}
	}
}

// cellsRunWithPickerAt opens the cells with the picker standing at one cell
// and has them pick the cell lock, reporting false when that cell is not one
// the picker can stand on or reach the door from.
func cellsRunWithPickerAt(t *testing.T, compiled dungeonspec.Compiled, cell spatial.Position) (*encounter.Encounter, bool) {
	t.Helper()
	members := []encounter.MemberInput{{ID: cellsPicker, Kind: encounter.KindPlayer, Position: cell}}
	for _, m := range compiled.Monsters {
		members = append(members, encounter.MemberInput{
			ID: encounter.MemberID(m.MemberID), Kind: encounter.KindMonster, Position: m.At,
			Faction: encounter.FactionID(m.Faction),
		})
	}
	enc, err := encounter.NewEncounter(&encounter.SetupInput{
		Field:   compiled.Field,
		Members: members,
		Endings: append([]encounter.EndingInput{{Key: "withdrawn", Trigger: encounter.TriggerExternal{}}}, compiled.Endings...),
		Capabilities: encounter.Capabilities{
			Sight:         everyoneSeesTheWholeMap{},
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
	if errors.Is(err, encounter.ErrBadPlacement) {
		return nil, false
	}
	require.NoError(t, err)

	_, err = enc.Unlock(&encounter.UnlockInput{
		Door: cellsDoor, Actor: cellsPicker, Beaten: true, Total: 18, Applied: cellsPick,
	})
	if errors.Is(err, encounter.ErrOutOfRange) {
		return nil, false
	}
	require.NoError(t, err)

	return enc, true
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
