// Copyright (C) 2026 Kirk Diggler
// SPDX-License-Identifier: GPL-3.0-or-later

package resolution

import (
	"context"
	"encoding/json"
	"testing"

	"github.com/stretchr/testify/suite"

	coreResources "github.com/KirkDiggler/rpg-toolkit/core/resources"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/abilities"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/character"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/classes"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/races"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/resources"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/shared"
)

const shortResterID = "short-rester"

var (
	shortRestPool = coreResources.ResourceKey("short-rest-pool")
	longRestPool  = coreResources.ResourceKey("long-rest-pool")
)

// ShortRestTestSuite proves the record-in/record-out short rest rolls the hit
// dice through the roller it is handed, each die sourced to the resting
// character, and returns the record the root operation left.
type ShortRestTestSuite struct {
	suite.Suite

	ctx context.Context
}

func TestShortRestSuite(t *testing.T) {
	suite.Run(t, new(ShortRestTestSuite))
}

func (s *ShortRestTestSuite) SetupTest() {
	s.ctx = context.Background()
}

// rester is a level-2 fighter with CON 14 (+2), hurt, with two hit dice left
// and one pool of each reset kind spent.
func (s *ShortRestTestSuite) rester() *character.Data {
	return &character.Data{
		ID: shortResterID, PlayerID: "rest-player", Name: "Short Rester",
		Level: 2, ProficiencyBonus: 2, RaceID: races.Human, ClassID: classes.Fighter,
		Levels: syntheticLevels(classes.Fighter, 2, 12, 8),
		AbilityScores: shared.AbilityScores{
			abilities.STR: 16, abilities.DEX: 14, abilities.CON: 14,
			abilities.INT: 10, abilities.WIS: 12, abilities.CHA: 8,
		},
		HitPoints: 3, MaxHitPoints: 20,
		Resources: map[coreResources.ResourceKey]character.RecoverableResourceData{
			resources.HitDice: {Current: 2, Maximum: 2, ResetType: coreResources.ResetLongRest},
			shortRestPool:     {Current: 0, Maximum: 1, ResetType: coreResources.ResetShortRest},
			longRestPool:      {Current: 0, Maximum: 1, ResetType: coreResources.ResetLongRest},
		},
	}
}

// §3 done-when: the trace carries every hit die with the character as its
// source, and the returned record is what the root operation reports.
func (s *ShortRestTestSuite) TestTheTraceSourcesEveryDieToTheResterAndTheRecordMatches() {
	roller := &sequenceRoller{pair: []int{6, 3}}

	out, err := ShortRest(s.ctx, &ShortRestInput{Character: s.rester(), HitDice: 2, Roller: roller})
	s.Require().NoError(err)
	s.Require().NotNil(out.Character)

	trace := out.Result.Healing
	s.Require().NotNil(trace)
	var thrown []int
	for _, component := range trace.Components {
		s.Equal(shortResterID, component.Source.SourceID,
			"every component of the rest's roll names the resting character")
		if component.Dice != nil {
			thrown = append(thrown, component.Dice.FinalRolls...)
		}
	}
	s.Equal([]int{6, 3}, thrown, "every die the roller threw is on the trace, and only those")
	s.Equal(6+3+2*2, trace.Total, "two dice plus CON per die")

	s.Equal(2, out.Result.HitDiceSpent)
	s.Equal(0, out.Result.HitDiceRemaining)
	s.Equal(13, out.Result.Healed)

	got := out.Character
	s.Equal(3+out.Result.Healed, got.HitPoints, "the record healed what the result says")
	s.Equal(out.Result.HitDiceRemaining, got.Resources[resources.HitDice].Current,
		"the record spent what the result says")
	s.Equal(1, got.Resources[shortRestPool].Current, "a short-rest pool refilled")
	s.Equal(0, got.Resources[longRestPool].Current, "a long-rest pool did not")
}

// The healing is capped at maximum hit points on the record.
func (s *ShortRestTestSuite) TestTheRecordNeverHealsPastMaximum() {
	data := s.rester()
	data.HitPoints = 18

	out, err := ShortRest(s.ctx, &ShortRestInput{
		Character: data, HitDice: 1, Roller: &sequenceRoller{pair: []int{10}},
	})
	s.Require().NoError(err)
	s.Equal(20, out.Character.HitPoints)
	s.Equal(1, out.Character.Resources[resources.HitDice].Current)
}

// A zero-dice rest needs no roller and still refills what a short rest
// refills.
func (s *ShortRestTestSuite) TestAZeroDiceRestNeedsNoRoller() {
	out, err := ShortRest(s.ctx, &ShortRestInput{Character: s.rester()})
	s.Require().NoError(err)
	s.Nil(out.Result.Healing)
	s.Equal(3, out.Character.HitPoints)
	s.Equal(2, out.Character.Resources[resources.HitDice].Current)
	s.Equal(1, out.Character.Resources[shortRestPool].Current)
}

func (s *ShortRestTestSuite) TestRefusalsSpendNothingAndReturnNoRecord() {
	s.Run("nil input", func() {
		out, err := ShortRest(s.ctx, nil)
		s.Require().ErrorIs(err, ErrNilInput)
		s.Require().Nil(out)
	})

	s.Run("no character", func() {
		out, err := ShortRest(s.ctx, &ShortRestInput{})
		s.Require().ErrorIs(err, ErrBadParticipant)
		s.Require().Nil(out)
	})

	s.Run("dice with no roller", func() {
		out, err := ShortRest(s.ctx, &ShortRestInput{Character: s.rester(), HitDice: 1})
		s.Require().ErrorIs(err, ErrNoRoller)
		s.Require().Nil(out)
	})

	s.Run("more dice than remain", func() {
		input := s.rester()
		before, err := json.Marshal(input)
		s.Require().NoError(err)

		out, err := ShortRest(s.ctx, &ShortRestInput{
			Character: input, HitDice: 3, Roller: &sequenceRoller{pair: []int{1, 1, 1}},
		})
		s.Require().Error(err)
		s.Require().Nil(out)

		after, err := json.Marshal(input)
		s.Require().NoError(err)
		s.Require().JSONEq(string(before), string(after), "the caller's record must not move")
	})
}
