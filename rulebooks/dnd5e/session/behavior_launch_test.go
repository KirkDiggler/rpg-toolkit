// Copyright (C) 2026 Kirk Diggler
// SPDX-License-Identifier: GPL-3.0-or-later

package session_test

import (
	"context"
	"fmt"
	"os"

	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/encounter"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/encounter/dungeonspec"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/session"
	"github.com/KirkDiggler/rpg-toolkit/tools/spatial"
)

type behaviorLaunchDice struct{ calls int }

func (d *behaviorLaunchDice) Roll(_ context.Context, sides int) (int, error) {
	if sides != 100 || d.calls >= 4 {
		return 0, fmt.Errorf("unexpected d%d after %d choices", sides, d.calls)
	}
	d.calls++
	return 1, nil
}

func (s *LaunchSuite) TestLaunchCarriesBehaviorConfigurationThroughNormalVerbs() {
	raw, err := os.ReadFile("testdata/behavior-patrol-consumer.yaml")
	s.Require().NoError(err)
	compiled, err := dungeonspec.Load(raw)
	s.Require().NoError(err)
	die := &behaviorLaunchDice{}
	s.mgr, err = session.NewManager(&session.Config{Record: session.DiscardRecord{}, Acts: s.stream, Seats: s.seats,
		CharacterConfig: newFakeCharacterConfig(), PresentationIDs: testPresentationIDs{}, Dice: die, TurnDriver: session.Driver(),
		Sessions: s.sessions, Encounters: s.encounters, Characters: s.characters})
	s.Require().NoError(err)
	_, err = s.mgr.Launch(context.Background(), &session.LaunchInput{Session: "run", DungeonKey: compiled.Key, Dungeon: &compiled, Party: []string{"alice"}})
	s.Require().NoError(err)
	stored, err := s.encounters.GetEncounter(context.Background(), "run")
	s.Require().NoError(err)
	var original *encounter.BehaviorData
	for _, m := range stored.Members {
		if m.ID == "west" {
			original = m.Behaviors
		}
	}
	s.Require().NotNil(original)
	s.Require().Len(original.Instances, 2)
	s.Equal("duty", original.Instances[1].Instance.ID)
	s.Zero(die.calls, "launch and reads do not select actions")
	for round, path := range [][]spatial.Position{
		{{X: 1, Y: 2}, {X: 2, Y: 2}, {X: 3, Y: 2}, {X: 4, Y: 2}, {X: 5, Y: 2}, {X: 6, Y: 2}},
		{{X: 5, Y: 2}, {X: 4, Y: 2}, {X: 3, Y: 2}, {X: 2, Y: 2}, {X: 1, Y: 2}, {X: 0, Y: 2}},
	} {
		_, err = s.mgr.Move(context.Background(), &session.MoveInput{Session: "run", Member: "alice", Path: path})
		s.Require().NoError(err)
		s.Equal((round+1)*2, die.calls, "exactly one singleton choice for each guard per world opportunity")
		stored, err = s.encounters.GetEncounter(context.Background(), "run")
		s.Require().NoError(err)
		for _, m := range stored.Members {
			if m.ID == "west" {
				s.Require().NotNil(m.Behaviors)
				s.Equal(original.Definition, m.Behaviors.Definition)
			}
		}
	}
	where, err := s.mgr.Where(context.Background(), &session.WhereInput{Session: "run", Member: "west"})
	s.Require().NoError(err)
	s.Equal(spatial.Position{X: 2}, where.Position)
	// A read is a normal reload too; it does not resample or expose a brain object.
	_, err = s.mgr.View(context.Background(), &session.ViewInput{Session: "run", Member: "alice"})
	s.Require().NoError(err)
	s.Equal(4, die.calls)
}
