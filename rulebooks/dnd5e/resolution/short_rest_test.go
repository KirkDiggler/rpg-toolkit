// Copyright (C) 2026 Kirk Diggler
// SPDX-License-Identifier: GPL-3.0-or-later

package resolution

import (
	"context"
	"encoding/json"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/stretchr/testify/suite"

	"github.com/KirkDiggler/rpg-toolkit/core"
	coreResources "github.com/KirkDiggler/rpg-toolkit/core/resources"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/abilities"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/character"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/classes"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/conditions"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/encounter"
	dnd5eEvents "github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/events"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/monster/monsters"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/races"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/refs"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/resources"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/shared"
)

const (
	shortResterID = "short-rester"
	restAllyID    = "rest-ally"
)

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
	s.Equal(13, out.Result.Requested)
	s.Equal(13, out.Result.Healed, "under the cap, everything requested landed")

	got := out.Character
	s.Equal(3+out.Result.Healed, got.HitPoints, "the record healed what the result says")
	s.Equal(out.Result.HitDiceRemaining, got.Resources[resources.HitDice].Current,
		"the record spent what the result says")
	s.Equal(1, got.Resources[shortRestPool].Current, "a short-rest pool refilled")
	s.Equal(0, got.Resources[longRestPool].Current, "a long-rest pool did not")

	var refilled []string
	for _, ref := range out.Result.Refilled {
		refilled = append(refilled, ref.String())
	}
	s.Contains(refilled, "dnd5e:resources:short-rest-pool", "the session can name what refilled")
	s.NotContains(refilled, "dnd5e:resources:long-rest-pool")
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
	s.Equal(2, out.Result.Healed, "what landed after the cap")
	s.Equal(18+out.Result.Healed, out.Character.HitPoints)
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

// blessHold is the rester concentrating on Bless (a one-minute spell) with
// turnEnds left, blessing itself and the ally.
func (s *ShortRestTestSuite) blessHold(turnEnds int) (rester, ally *character.Data) {
	blessed := func(memberID string) (dnd5eEvents.ConditionAddress, json.RawMessage) {
		condition, err := conditions.NewBlessedCondition(conditions.NewBlessedConditionInput{
			MemberID: memberID, SourceID: shortResterID, SourceRef: refs.Spells.Bless(),
		})
		s.Require().NoError(err)
		raw, err := condition.ToJSON()
		s.Require().NoError(err)
		return dnd5eEvents.ConditionAddress{
			MemberID: memberID, ConditionRef: refs.Conditions.Blessed().String(), SourceID: shortResterID,
		}, raw
	}
	selfAddress, selfBlessed := blessed(shortResterID)
	allyAddress, allyBlessed := blessed(restAllyID)

	hold := conditions.NewConcentratingConditionWithInput(conditions.NewConcentratingConditionInput{
		MemberID: shortResterID, SourceID: shortResterID, SpellRef: refs.Spells.Bless().String(),
		SpellName: "Bless", TurnEnds: turnEnds,
	})
	s.Require().NoError(hold.AddChild(s.ctx, selfAddress))
	s.Require().NoError(hold.AddChild(s.ctx, allyAddress))
	holdJSON, err := hold.ToJSON()
	s.Require().NoError(err)

	rester = s.rester()
	rester.Conditions = []json.RawMessage{holdJSON, selfBlessed}

	ally = s.rester()
	ally.ID, ally.Name = restAllyID, "Rest Ally"
	ally.Conditions = []json.RawMessage{allyBlessed}
	return rester, ally
}

func restConditionRefs(t *testing.T, data *character.Data) []string {
	t.Helper()
	var out []string
	for _, raw := range data.Conditions {
		var head struct {
			Ref *core.Ref `json:"ref"`
		}
		require.NoError(t, json.Unmarshal(raw, &head))
		require.NotNil(t, head.Ref)
		out = append(out, head.Ref.String())
	}
	return out
}

// A one-minute hold ends on the rest: the entry names it, strips its effect from the rester
// and from the ally passed in, and the ally comes back dirty.
func (s *ShortRestTestSuite) TestAOneMinuteHoldEndsAndIsNamed() {
	rester, ally := s.blessHold(10)

	out, err := ShortRest(s.ctx, &ShortRestInput{
		Character: rester, Others: []Participant{{Character: ally}},
	})
	s.Require().NoError(err)

	s.Require().Len(out.ConcentrationBreaks, 1, "one hold, one break")
	held := out.ConcentrationBreaks[0]
	s.Equal(encounter.MemberID(shortResterID), held.Caster)
	s.Equal(refs.Spells.Bless().String(), held.Spell.Ref)
	s.Equal("rest", held.Reason, "the rulebook ends a hold on any rest")
	var removedFrom []encounter.MemberID
	for _, removed := range held.Removed {
		s.Require().NotNil(removed.Address)
		s.Equal(refs.Conditions.Blessed().String(), removed.Address.ConditionRef)
		removedFrom = append(removedFrom, removed.Address.MemberID)
	}
	s.ElementsMatch([]encounter.MemberID{shortResterID, restAllyID}, removedFrom)

	s.NotContains(restConditionRefs(s.T(), out.Character), refs.Conditions.Concentrating().String())
	s.NotContains(restConditionRefs(s.T(), out.Character), refs.Conditions.Blessed().String())
	s.Empty(out.Ended, "the hold and its effects are the break's to report")

	s.Require().Len(out.DirtyCharacters, 1)
	s.Equal(restAllyID, out.DirtyCharacters[0].ID)
	s.NotContains(restConditionRefs(s.T(), out.DirtyCharacters[0]), refs.Conditions.Blessed().String())
	s.Equal(0, out.DirtyCharacters[0].Resources[shortRestPool].Current,
		"the ally did not rest: its short-rest pool stays spent")
}

// A hold whose effect sits on a member left out of Others refuses before
// anything ends: the ally could never be written, so the strip would be a lie.
func (s *ShortRestTestSuite) TestAHoldReachingAMemberNotPassedInRefuses() {
	rester, _ := s.blessHold(10)
	before, err := json.Marshal(rester)
	s.Require().NoError(err)

	out, err := ShortRest(s.ctx, &ShortRestInput{Character: rester})
	s.Require().ErrorIs(err, ErrBadParticipant)
	s.Require().ErrorContains(err, restAllyID)
	s.Require().Nil(out)

	after, err := json.Marshal(rester)
	s.Require().NoError(err)
	s.Require().JSONEq(string(before), string(after), "the caller's record must not move")
}

// A hold's effect on a monster comes off with it: Bane on a goblin, the goblin
// passed in, comes back dirty without Baned.
func (s *ShortRestTestSuite) TestAHoldStripsItsEffectFromAMonster() {
	const goblinID = "rest-goblin"
	goblin := monsters.NewGoblin(goblinID).ToData()
	goblin.Conditions = append(goblin.Conditions, baneConditionJSON(s.T(), goblinID, shortResterID))
	rester := s.rester()
	rester.Conditions = []json.RawMessage{baneOwnerJSON(s.T(), shortResterID, 10, dnd5eEvents.ConditionAddress{
		MemberID: goblinID, ConditionRef: refs.Conditions.Baned().String(), SourceID: shortResterID,
	})}

	out, err := ShortRest(s.ctx, &ShortRestInput{
		Character: rester, Others: []Participant{{Monster: goblin}},
	})
	s.Require().NoError(err)

	s.Require().Len(out.ConcentrationBreaks, 1)
	s.Equal(refs.Spells.Bane().String(), out.ConcentrationBreaks[0].Spell.Ref)
	s.Empty(out.DirtyCharacters)
	s.Require().Len(out.DirtyMonsters, 1)
	s.Equal(goblinID, out.DirtyMonsters[0].ID)
	for _, raw := range out.DirtyMonsters[0].Conditions {
		var head struct {
			Ref *core.Ref `json:"ref"`
		}
		s.Require().NoError(json.Unmarshal(raw, &head))
		if head.Ref != nil {
			s.NotEqual(refs.Conditions.Baned().String(), head.Ref.String(), "Baned came off the goblin")
		}
	}
}

// A short rest ends every hold, whatever its clock: the rulebook ends a
// combat-scoped condition on any rest.
func (s *ShortRestTestSuite) TestAHoldLongerThanAnHourEndsToo() {
	rester, ally := s.blessHold(encounter.RoundsPerHour + 1)

	out, err := ShortRest(s.ctx, &ShortRestInput{
		Character: rester, Others: []Participant{{Character: ally}},
	})
	s.Require().NoError(err)
	s.Require().Len(out.ConcentrationBreaks, 1)
	s.NotContains(restConditionRefs(s.T(), out.Character), refs.Conditions.Concentrating().String())
	s.Require().Len(out.DirtyCharacters, 1)
}

// A combat-scoped condition ends on a short rest and is named.
func (s *ShortRestTestSuite) TestACombatScopedConditionEndsAndIsNamed() {
	prone, err := conditions.NewProneCondition(shortResterID).ToJSON()
	s.Require().NoError(err)
	rester := s.rester()
	rester.Conditions = []json.RawMessage{prone}

	out, err := ShortRest(s.ctx, &ShortRestInput{Character: rester})
	s.Require().NoError(err)

	s.Require().Len(out.Ended, 1)
	s.Equal(refs.Conditions.Prone().String(), out.Ended[0].Address.ConditionRef)
	s.NotContains(restConditionRefs(s.T(), out.Character), refs.Conditions.Prone().String())
}

// An effect that ends on any rest ends itself on the short rest's event, and
// the entry names it.
func (s *ShortRestTestSuite) TestAnUntilRestEffectEndsAndIsNamed() {
	inspired, err := conditions.NewInspiredCondition(shortResterID, restAllyID, "").ToJSON()
	s.Require().NoError(err)
	rester := s.rester()
	rester.Conditions = []json.RawMessage{inspired}

	out, err := ShortRest(s.ctx, &ShortRestInput{Character: rester})
	s.Require().NoError(err)

	s.Require().Len(out.Ended, 1)
	s.Equal(encounter.ResultConditionRemoved, out.Ended[0].Kind)
	s.Equal(refs.Conditions.Inspired().String(), out.Ended[0].Address.ConditionRef)
	s.Empty(out.ConcentrationBreaks)
	s.NotContains(restConditionRefs(s.T(), out.Character), refs.Conditions.Inspired().String())
}

// A refused rest ends no hold that reaches a write: no record at all.
func (s *ShortRestTestSuite) TestARefusedRestReturnsNoBrokenHold() {
	rester, ally := s.blessHold(10)

	out, err := ShortRest(s.ctx, &ShortRestInput{
		Character: rester, HitDice: 3, Roller: &sequenceRoller{pair: []int{1, 1, 1}},
		Others: []Participant{{Character: ally}},
	})
	s.Require().Error(err)
	s.Require().Nil(out)
}

func (s *ShortRestTestSuite) TestTheResterAmongTheOthersIsRefused() {
	out, err := ShortRest(s.ctx, &ShortRestInput{
		Character: s.rester(), Others: []Participant{{Character: s.rester()}},
	})
	s.Require().ErrorIs(err, ErrBadParticipant)
	s.Require().Nil(out)
}
