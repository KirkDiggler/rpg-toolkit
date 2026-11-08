// Copyright (C) 2026 Kirk Diggler
// SPDX-License-Identifier: GPL-3.0-or-later

package session_test

import (
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/encounter"
	"github.com/stretchr/testify/require"
)

const cookDefinition = `version: 5
behaviorDefinitions:
  cook-body:
    recipe:
      ref: example:behaviors:cook-body
      initial: posted
      states:
        posted: {transitions: [{on: heard, when: {scope: {within: 6}}, to: alert, do: [{anchor: true}]}]}
        alert: {transitions: [{on: time, when: {rounds: {op: gte, n: 3}}, to: posted}]}
`

func (s *LaunchTemplateSuite) member(id string) *encounter.MemberData {
	data, ok := s.encounters.byID[testSession]
	s.Require().True(ok)
	for i := range data.Members {
		if data.Members[i].ID == encounter.MemberID(id) {
			return &data.Members[i]
		}
	}
	s.Require().FailNowf("no member", "launch seated no %q", id)
	return nil
}

func (s *LaunchTemplateSuite) TestLaunch_PlacementBehaviorReachesTheSeatedMember() {
	dungeon := s.castle([2]string{"version: 5\n", cookDefinition}, [2]string{"cook-1: {faction: kitchen}", "cook-1: {faction: kitchen, behaviors: [{id: body, use: cook-body}]}"})
	compiled := s.placement(dungeon, "cook-1").Behaviors
	s.Require().NotNil(compiled)
	s.Require().NoError(s.launch(dungeon))
	stored := s.member("cook-1").Behaviors
	s.Require().NotNil(stored)
	s.Equal("posted", stored.Instances[0].Instance.Recipe.Initial)
	require.ElementsMatch(s.T(), []string{"posted", "alert"}, keysOf(stored.Instances[0].Instance.Recipe.States))
	s.Equal(compiled.Instances[0].Instance.Recipe, stored.Instances[0].Instance.Recipe, "the recipe crosses unchanged")
	s.Equal(compiled.Instances[0].Instance.ID, stored.Instances[0].Instance.ID)
	s.Empty(stored.Instances[0].Instance.Table, "no rows were invented for the body-only definition")
	s.Empty(stored.Instances[0].Completions)
	s.Nil(s.member("guard-1").Behaviors, "unauthored policy is not filled from a kind default")
}

func (s *LaunchTemplateSuite) TestLaunch_PlacementBlockingStatesReachTheSeatedMember() {
	dungeon := s.castle([2]string{"version: 5\n", cookDefinition}, [2]string{"cook-1: {faction: kitchen}", "cook-1: {faction: kitchen, behaviors: [{id: body, use: cook-body}], behaviorState: body, blockingStates: [posted]}"})
	s.Require().Equal([]string{"posted"}, s.placement(dungeon, "cook-1").BlockingStates)
	s.Require().NoError(s.launch(dungeon))
	s.Equal([]string{"posted"}, s.member("cook-1").BlockingStates)
	s.Equal("body", s.member("cook-1").Behaviors.BodyInstance)
	s.Empty(s.member("guard-1").BlockingStates)
}

func keysOf[V any](m map[string]V) []string {
	out := make([]string, 0, len(m))
	for k := range m {
		out = append(out, k)
	}
	return out
}
