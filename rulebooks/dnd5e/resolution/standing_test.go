// Copyright (C) 2026 Kirk Diggler
// SPDX-License-Identifier: GPL-3.0-or-later

package resolution

import (
	"context"
	"encoding/json"
	"testing"

	"github.com/stretchr/testify/suite"

	"github.com/KirkDiggler/rpg-toolkit/events"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/abilities"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/character"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/classes"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/combat"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/conditions"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/monster"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/races"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/shared"
)

type StandingTestSuite struct {
	suite.Suite
	ctx context.Context
}

func (s *StandingTestSuite) SetupTest() { s.ctx = context.Background() }

func TestStandingSuite(t *testing.T) { suite.Run(t, new(StandingTestSuite)) }

func (s *StandingTestSuite) hero(id string, hp int, conds ...json.RawMessage) *character.Data {
	return &character.Data{
		ID: id, PlayerID: "player-1", Name: "Standre", Level: 1,
		ClassID: classes.Barbarian, RaceID: races.Human,
		AbilityScores: shared.AbilityScores{
			abilities.STR: 16, abilities.DEX: 14, abilities.CON: 16,
			abilities.INT: 10, abilities.WIS: 12, abilities.CHA: 8,
		},
		HitPoints: hp, MaxHitPoints: 14, ArmorClass: 10,
		ProficiencyBonus: 2, Conditions: conds,
	}
}

func (s *StandingTestSuite) wolf(id string, hp int) *monster.Data {
	return &monster.Data{ID: id, Name: "Wolf", HitPoints: hp, MaxHitPoints: 11, ArmorClass: 13}
}

// Nobody down is the ordinary answer, and an EMPTY list rather than a nil-ish
// silence: a caller reading the zero value proceeds, which is correct.
func (s *StandingTestSuite) TestEverybodyStanding() {
	out, err := Standing(s.ctx, &StandingInput{Participants: []Participant{
		{Character: s.hero("hero-1", 14)},
		{Monster: s.wolf("wolf-1", 11)},
	}})
	s.Require().NoError(err)

	s.Empty(out.Down, "nobody at zero, so nobody is down")
}

// Zero hit points is down, and BELOW zero counts too — a combatant reporting
// -3 is not standing, whatever the sheets happen to floor at today.
func (s *StandingTestSuite) TestZeroAndBelowAreBothDown() {
	out, err := Standing(s.ctx, &StandingInput{Participants: []Participant{
		{Character: s.hero("hero-1", 0)},
		{Character: s.hero("hero-2", -3)},
		{Character: s.hero("hero-3", 1)},
	}})
	s.Require().NoError(err)

	s.Equal([]string{"hero-1", "hero-2"}, out.Down,
		"one at zero, one below it, and the one on 1 hit point is still up")
}

// Both kinds are asked the same question. A monster is not a special case here:
// the seam this replaces had to know which store a sheet came from, and this
// one does not.
func (s *StandingTestSuite) TestAMonsterIsAskedTheSameQuestion() {
	out, err := Standing(s.ctx, &StandingInput{Participants: []Participant{
		{Character: s.hero("hero-1", 14)},
		{Monster: s.wolf("wolf-1", 0)},
	}})
	s.Require().NoError(err)

	s.Equal([]string{"wolf-1"}, out.Down)
}

// Cast order, not input order, so two calls over the same data answer the same
// way whatever order the caller happened to assemble them in.
func (s *StandingTestSuite) TestTheAnswerIsInCastOrder() {
	forwards, err := Standing(s.ctx, &StandingInput{Participants: []Participant{
		{Character: s.hero("aaa", 0)}, {Character: s.hero("zzz", 0)},
	}})
	s.Require().NoError(err)

	backwards, err := Standing(s.ctx, &StandingInput{Participants: []Participant{
		{Character: s.hero("zzz", 0)}, {Character: s.hero("aaa", 0)},
	}})
	s.Require().NoError(err)

	s.Equal([]string{"aaa", "zzz"}, forwards.Down)
	s.Equal(forwards.Down, backwards.Down, "the report does not depend on how the caller stacked them")
}

// A CHARACTER carrying a blob this build cannot parse still answers. The
// question is about hit points, and an unreadable condition has no bearing on
// them — refusing would abort a verb over something nobody asked about.
func (s *StandingTestSuite) TestAnUnreadableConditionDoesNotHideACharacter() {
	unreadable := json.RawMessage(`{"ref":"nonsense","x":`)

	out, err := Standing(s.ctx, &StandingInput{Participants: []Participant{
		{Character: s.hero("hero-1", 0, unreadable)},
	}})
	s.Require().NoError(err, "a read entry does not put an unreadable blob between a player and the game")

	s.Equal([]string{"hero-1"}, out.Down, "and the hit points still answered")
}

// A MONSTER carrying one refuses, and that is stated rather than hidden.
//
// DropUnreadable reaches the character branch and stops there, so this entry's
// leniency is a character policy. Pinned so the asymmetry is a fact on the
// record instead of a surprise: the path this replaces refuses the same record
// for the same reason, so nothing regressed, and the day monstertraits grows a
// lenient loader this test is what changes.
func (s *StandingTestSuite) TestAnUnreadableTraitStillRefusesAMonster() {
	broken := s.wolf("wolf-1", 11)
	broken.Conditions = []json.RawMessage{json.RawMessage(`{"ref":"nonsense","x":`)}

	_, err := Standing(s.ctx, &StandingInput{Participants: []Participant{{Monster: broken}}})

	s.Require().Error(err, "monstertraits has no lenient half; a caller asking to read leniently still gets a refusal")
}

// Nothing survives the call.
//
// Asserted on the BUS rather than on the teardown call, the shape the
// projection's own pin uses: it survives a change to which mechanism does the
// work. The hero carries Unarmored Defense precisely so there is something that
// COULD leak — it joins the AC chain on the way in, and afterwards that chain
// must answer nobody. A standing question that leaked would leave a sheet its
// caller has forgotten still folding.
func (s *StandingTestSuite) TestNothingIsLeftOnTheBus() {
	inner := events.NewEventBus()

	raw, err := (&conditions.UnarmoredDefenseCondition{
		CharacterID: "hero-1", Type: conditions.UnarmoredDefenseBarbarian,
	}).ToJSON()
	s.Require().NoError(err)

	out, err := standingOn(s.ctx, &StandingInput{Participants: []Participant{
		{Character: s.hero("hero-1", 14, raw)},
	}}, newSurface(inner))
	s.Require().NoError(err)
	s.Require().Empty(out.Down, "it answered before it tore down")

	event := &combat.ACChainEvent{
		CharacterID: "hero-1",
		Breakdown:   &combat.ACBreakdown{Total: 0, Components: []combat.ACComponent{}},
	}
	chain := events.NewStagedChain[*combat.ACChainEvent](combat.ModifierStages)

	modified, err := combat.ACChain.On(inner).PublishWithChain(s.ctx, event, chain)
	s.Require().NoError(err)

	folded, err := modified.Execute(s.ctx, event)
	s.Require().NoError(err)

	s.Require().Empty(folded.Breakdown.Components,
		"Unarmored Defense attached during the call and does not answer this chain")
}

func (s *StandingTestSuite) TestNilInputRefuses() {
	_, err := Standing(s.ctx, nil)
	s.Require().ErrorIs(err, ErrNilInput)
}
