// Copyright (C) 2026 Kirk Diggler
// SPDX-License-Identifier: GPL-3.0-or-later

package resolution

import (
	"fmt"

	"github.com/KirkDiggler/rpg-toolkit/core"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/encounter"
	"github.com/KirkDiggler/rpg-toolkit/tools/spatial"
)

// placedMember is a member as the spatial room needs it: an ID at a position.
// The room stores entities, and nothing here reads anything else off them.
type placedMember struct {
	id string
}

func (m placedMember) GetID() string            { return m.id }
func (m placedMember) GetType() core.EntityType { return "member" }

// interactionRoom builds the room an interaction's predicates read positions
// from, out of the world the caller handed in.
//
// The positions are already here. [Input.World] is the persisted world —
// EncounterData.Members carries every member's room and position, and
// Field.Rooms carries each room's grid, size and origin — so this reconstructs
// the room from the same bytes the encounter itself loads from, rather than
// asking the encounter for a runtime object it does not expose. There is no
// C2 question in that: C2 governs what a *decider* may see during a pump, and
// resolution is not a decider. It is handed every sheet by construction (R3)
// and is the one place that holds the whole world.
//
// One room, not all of them. An interaction happens where its participants
// are, and if they are not all in one room this returns nil — a machine that
// needs positions then refuses, rather than measuring a distance between two
// rooms' local coordinate systems, which would be a number with no meaning.
// (Members' positions are room-local: encounter's ToData reads them out of
// each room, so two members of the same room are directly comparable and two
// members of different rooms are not.)
//
// Positions are as-persisted. Nothing moves during an interaction — movement
// is the walk machine's, and it is a different interaction — so a room built
// once at the start stays true for the whole call. A live-truth projection,
// for when an interaction does move somebody, is #964's question.
func interactionRoom(world encounter.EncounterData, participants []Participant) (spatial.Room, error) {
	wanted := make(map[string]struct{}, len(participants))
	for _, p := range participants {
		if id := p.ID(); id != "" {
			wanted[id] = struct{}{}
		}
	}

	roomID := ""
	placed := make([]encounter.MemberData, 0, len(wanted))
	for _, member := range world.Members {
		if _, ok := wanted[string(member.ID)]; !ok {
			continue
		}

		if roomID == "" {
			roomID = member.Room
		} else if member.Room != roomID {
			// Participants span rooms: no single room describes this
			// interaction, so none is installed.
			return nil, nil
		}

		placed = append(placed, member)
	}

	if roomID == "" {
		// No participant is a member of this world — legal (a saving throw
		// needs no geometry) and simply means no room to install.
		return nil, nil
	}

	data, ok := roomByID(world, roomID)
	if !ok {
		return nil, fmt.Errorf("%w: member room %q is not in the field", ErrBadWorld, roomID)
	}

	room := spatial.NewBasicRoom(spatial.BasicRoomConfig{
		ID:   data.ID,
		Type: "encounter",
		Grid: buildRoomGrid(data),
	})

	for _, member := range placed {
		position := spatial.Position{X: member.Position.X, Y: member.Position.Y}
		if err := room.PlaceEntity(placedMember{id: string(member.ID)}, position); err != nil {
			return nil, fmt.Errorf("%w: place %q: %w", ErrBadWorld, member.ID, err)
		}
	}

	return room, nil
}

// roomByID finds a room's persisted description.
func roomByID(world encounter.EncounterData, id string) (encounter.RoomData, bool) {
	for _, room := range world.Field.Rooms {
		if room.ID == id {
			return room, true
		}
	}

	return encounter.RoomData{}, false
}

// buildRoomGrid mirrors the encounter's own grid construction, deliberately.
//
// A room built with a different grid than the encounter loaded would measure
// different distances from identical data — the prone predicate would answer
// "within five feet" where the encounter says otherwise — so the mapping here
// tracks encounter.buildRoomGrid rather than choosing for itself: hex rooms
// are axial hex with the size as its span, everything else is a square grid.
func buildRoomGrid(data encounter.RoomData) spatial.Grid {
	if data.Grid == spatial.GridTypeHex {
		return spatial.NewAxialHexGrid(spatial.AxialHexGridConfig{
			SpanWidth:  float64(data.Width),
			SpanHeight: float64(data.Height),
		})
	}

	return spatial.NewSquareGrid(spatial.SquareGridConfig{
		Width:  float64(data.Width),
		Height: float64(data.Height),
	})
}
