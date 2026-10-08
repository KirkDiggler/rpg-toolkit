// Copyright (C) 2026 Kirk Diggler
// SPDX-License-Identifier: GPL-3.0-or-later

package session_test

import (
	"context"
	"encoding/json"

	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/races"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/session"
	"github.com/KirkDiggler/rpg-toolkit/tools/spatial"
)

// The structural world keeps its learned layout, but never supplies sheet
// answers from its saved roster. Every SDK verb reloads the world and reads
// the character repository; even a missing sheet must remain a refusal.
func (s *StructuralSessionSuite) TestStructuralWalkReadsLiveSheetsWithoutRewritingKnowledge() {
	ctx := context.Background()
	characters := newFakeCharacters(armedFighter("alice"))
	worlds := newFakeEncounters()
	mgr, err := session.NewManager(&session.Config{
		Sessions: newFakeSessions(), Encounters: worlds, Characters: characters,
		Events: session.DiscardEvents{}, Dice: testDice{}, TurnDriver: session.Pass{}, PresentationIDs: testPresentationIDs{},
	})
	s.Require().NoError(err)
	_, err = mgr.StartSession(ctx, &session.StartSessionInput{Session: "sess", Encounter: "world", World: structuralRoomWorld(s.T())})
	s.Require().NoError(err)
	_, err = mgr.OpenDoor(ctx, &session.OpenDoorInput{Session: "sess", Member: "alice", Door: "gate"})
	s.Require().NoError(err)
	input := &session.KnowledgeInput{Session: "sess", Member: "alice", Player: "player-alice"}
	known, err := mgr.Knowledge(ctx, input)
	s.Require().NoError(err)
	s.Require().NotEmpty(known.Atlas.StructuralWalls)
	baseline := worlds.byID["world"].Clock.HighWater
	path := make([]spatial.Position, 0, 6)
	for x := 6; x <= 11; x++ {
		path = append(path, spatial.Position{X: float64(x)})
	}
	moved, err := mgr.Move(ctx, &session.MoveInput{Session: "sess", Member: "alice", Path: path})
	s.Require().NoError(err)
	s.Require().Len(moved.Steps, 6)
	s.Equal(baseline+1, worlds.byID["world"].Clock.HighWater, "human sheet: six cells pace one round")

	characters.byID["alice"].RaceID = races.Dwarf
	path = nil
	for x := 10; x >= 6; x-- {
		path = append(path, spatial.Position{X: float64(x)})
	}
	moved, err = mgr.Move(ctx, &session.MoveInput{Session: "sess", Member: "alice", Path: path})
	s.Require().NoError(err)
	s.Require().Len(moved.Steps, 5)
	s.Equal(baseline+2, worlds.byID["world"].Clock.HighWater, "changed sheet: five cells pace one round without rejoin")
	after, err := mgr.Knowledge(ctx, input)
	s.Require().NoError(err)
	s.Equal(known.Atlas.StructuralWalls, after.Atlas.StructuralWalls)
	s.Equal(known.Atlas.StructuralDoors, after.Atlas.StructuralDoors)
	s.Equal(known.Atlas.Cells, after.Atlas.Cells)

	beforeMissing, err := json.Marshal(worlds.byID["world"])
	s.Require().NoError(err)
	delete(characters.byID, "alice")
	_, err = mgr.Move(ctx, &session.MoveInput{Session: "sess", Member: "alice", Path: []spatial.Position{{X: 5}}})
	s.ErrorIs(err, session.ErrNoCharacter, "missing sheet is not a zero-speed/default sheet")
	afterMissing, err := json.Marshal(worlds.byID["world"])
	s.Require().NoError(err)
	s.Equal(beforeMissing, afterMissing, "refused verb cannot mutate encounter knowledge or placement")
}
