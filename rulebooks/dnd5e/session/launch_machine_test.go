// Copyright (C) 2026 Kirk Diggler
// SPDX-License-Identifier: GPL-3.0-or-later

package session_test

// launch_machine_test.go is the session's half of rpg-project#766 S4: the
// states an author writes on a binding compile to a machine recipe on the
// placement, and Launch carries that recipe through the door to the seated
// member. The castle kitchen is launched as an author wrote it, with states
// added to one binding, so what reaches the encounter came from the file.

import (
	"github.com/stretchr/testify/require"

	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/encounter"
)

// cookStates gives the cook a two-state recipe: posted until it hears
// something, then alert.
const cookStates = `cook-1:
        faction: kitchen
        states:
          initial: posted
          posted:
            transitions:
              - { on: heard, within: 6, to: alert }
          alert:
            transitions:
              - { on: time, rounds: 3, to: posted }`

// member is the seated member with id in the encounter the launch saved.
func (s *LaunchTemplateSuite) member(id string) *encounter.MemberData {
	s.T().Helper()
	data, ok := s.encounters.byID[testSession]
	s.Require().True(ok, "the launch saved no encounter")
	for i := range data.Members {
		if data.Members[i].ID == encounter.MemberID(id) {
			return &data.Members[i]
		}
	}
	s.Require().Failf("no member", "the launch seated no member %q", id)
	return nil
}

// TestLaunch_PlacementMachineReachesTheSeatedMember: the binding's states
// arrive on the seated cook as its recipe, and the guard nobody wrote states
// for runs the default (no recipe recorded).
func (s *LaunchTemplateSuite) TestLaunch_PlacementMachineReachesTheSeatedMember() {
	dungeon := s.castle([2]string{"cook-1: {faction: kitchen}", cookStates})
	compiled := s.placement(dungeon, "cook-1").Machine
	s.Require().NotNil(compiled, "precondition: the compile made the cook a recipe")

	s.Require().NoError(s.launch(dungeon))

	cook := s.member("cook-1")
	s.Require().NotNil(cook.Recipe, "the cook's recipe crossed the door")
	s.Equal("posted", cook.Recipe.Initial)
	require.ElementsMatch(s.T(), []string{"posted", "alert"}, keysOf(cook.Recipe.States))
	s.Equal(*compiled, *cook.Recipe, "the door carries the compiled recipe unchanged")

	s.Nil(s.member("guard-1").Recipe, "a binding without states runs the default recipe")
}

func keysOf[V any](m map[string]V) []string {
	out := make([]string, 0, len(m))
	for k := range m {
		out = append(out, k)
	}
	return out
}

// postedCookStates is cookStates with the post blocking (rpg-project#836):
// posted, the cook's body holds its cell; alert, it does not.
const postedCookStates = `cook-1:
        faction: kitchen
        states:
          initial: posted
          posted:
            blocks: true
            transitions:
              - { on: heard, within: 6, to: alert }
          alert:
            transitions:
              - { on: time, rounds: 3, to: posted }`

// TestLaunch_PlacementBlockingStatesReachTheSeatedMember (rpg-project#836):
// the states an author marked `blocks` cross the door beside the recipe, so
// the seated cook blocks by its state. A launch that carried the recipe and
// dropped the binding would seat a body that never blocks.
func (s *LaunchTemplateSuite) TestLaunch_PlacementBlockingStatesReachTheSeatedMember() {
	dungeon := s.castle([2]string{"cook-1: {faction: kitchen}", postedCookStates})
	compiled := s.placement(dungeon, "cook-1").BlockingStates
	s.Require().Equal([]string{"posted"}, compiled, "precondition: the compile bound the post")

	s.Require().NoError(s.launch(dungeon))

	s.Equal(compiled, s.member("cook-1").BlockingStates, "the door carries the blocking states unchanged")
	s.Empty(s.member("guard-1").BlockingStates, "a binding without states blocks as it is declared")
}
