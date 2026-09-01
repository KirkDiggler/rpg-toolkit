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
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/encounter"
	dnd5eEvents "github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/events"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/races"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/refs"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/shared"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/skills"
)

const seekerID = "seeker"

// The authored vocabulary the entry speaks: routes as the composition hands
// them over. DCs are chosen so the arithmetic in each test names itself.
func route(ability string, dc int) encounter.CheckApproach {
	return encounter.CheckApproach{Ability: ability, DC: dc}
}

type CheckTestSuite struct {
	suite.Suite

	ctx    context.Context
	roller *scriptedRoller
}

func (s *CheckTestSuite) SetupTest() {
	s.ctx = context.Background()
	s.roller = &scriptedRoller{single: straightRoll, pair: []int{straightRoll, advantageRoll}}
}

// seeker is the checker every test loads: STR 16 (+3), DEX 14 (+2),
// WIS 12 (+1), proficiency +2, proficient in Athletics (+5) and
// Perception (+3). The spreads are deliberate — every selection test below
// resolves to a different route depending on whether skill arithmetic,
// raw-ability arithmetic, or DC pricing is applied, so a wrong dispatch
// cannot produce a right answer.
func (s *CheckTestSuite) seeker(conds ...json.RawMessage) *character.Data {
	return &character.Data{
		ID:       seekerID,
		PlayerID: "player-1",
		Name:     "Vex",
		Level:    1,
		ClassID:  classes.Barbarian,
		RaceID:   races.Human,
		AbilityScores: shared.AbilityScores{
			abilities.STR: 16,
			abilities.DEX: 14,
			abilities.CON: 14,
			abilities.INT: 10,
			abilities.WIS: 12,
			abilities.CHA: 8,
		},
		HitPoints:        14,
		MaxHitPoints:     14,
		ProficiencyBonus: 2,
		Skills: map[skills.Skill]shared.ProficiencyLevel{
			skills.Athletics:  shared.Proficient,
			skills.Perception: shared.Proficient,
		},
		Conditions: conds,
	}
}

func (s *CheckTestSuite) raging() json.RawMessage {
	raw, err := (&conditions.RagingCondition{
		CharacterID: seekerID,
		DamageBonus: 2,
		Level:       1,
		Source:      "rage",
	}).ToJSON()
	s.Require().NoError(err)

	return raw
}

func (s *CheckTestSuite) check(data *character.Data, approaches ...encounter.CheckApproach) (*CheckOutput, error) {
	return MakeCheck(s.ctx, &CheckInput{
		Character:  data,
		Approaches: approaches,
		Roller:     s.roller,
	})
}

// THE HEADLINE, and the chain's first live production audience. Nobody
// attached anything and nobody computed a modifier at the seam: the caller
// passed a record with a persisted Raging condition and a route list, and
// Raging's own predicate — advantage on Strength-based checks — reached the
// d20 through resolution's bus.
func (s *CheckTestSuite) TestARagingCheckerGetsAdvantageOnAnAthleticsCheck() {
	out, err := s.check(s.seeker(s.raging()), route(string(skills.Athletics), 12))
	s.Require().NoError(err)

	s.Require().Equal(advantageRoll, out.Result.Roll,
		"only a rolled-twice-take-higher can produce this die")
	s.Require().Equal(advantageRoll+5, out.Result.Total,
		"athletics is STR +3 with proficiency +2")
	s.Require().True(out.Result.Success)

	s.Require().Len(out.Result.AdvantageSources, 1)
	s.Require().Equal(refs.Conditions.Raging(), out.Result.AdvantageSources[0].SourceRef,
		"and it is Raging that says so, by name, not the wiring")
	s.Require().Equal("Raging", out.Result.AdvantageSources[0].Name)
}

// The control that makes the headline mean something: the same checker, the
// same route, no condition — a straight roll.
func (s *CheckTestSuite) TestTheSameCheckerWithoutRageRollsStraight() {
	out, err := s.check(s.seeker(), route(string(skills.Athletics), 12))
	s.Require().NoError(err)

	s.Require().Equal(straightRoll, out.Result.Roll)
	s.Require().Empty(out.Result.AdvantageSources)
	s.Require().Nil(out.DirtyCharacter,
		"a plain check leaves the sheet untouched, and nil says so")
}

// Applicability stays the effect's own predicate: Raging is attached for a
// Perception check exactly as for an Athletics one, and declines — WIS is not
// its business.
func (s *CheckTestSuite) TestRagingDeclinesAPerceptionCheckOnItsOwn() {
	out, err := s.check(s.seeker(s.raging()), route(string(skills.Perception), 12))
	s.Require().NoError(err)

	s.Require().Equal(straightRoll, out.Result.Roll)
	s.Require().Empty(out.Result.AdvantageSources)
}

// Routes are priced separately (rpg-project#350): best cannot mean best
// modifier alone. STR is the seeker's best number (+3 raw) but its route
// costs DC 15 (net -12); Perception is +3 against DC 10 (net -7) and wins.
func (s *CheckTestSuite) TestBestApproachPricesRoutesSeparately() {
	out, err := s.check(s.seeker(),
		route(string(abilities.STR), 15),
		route(string(skills.Perception), 10),
	)
	s.Require().NoError(err)

	s.Require().Equal(string(skills.Perception), out.Applied.Ability)
	s.Require().Equal(10, out.Applied.DC, "the verdict names the DC actually faced")
	s.Require().Equal(straightRoll+3, out.Result.Total, "perception is WIS +1 with proficiency +2")
}

// Two routes at the same net price: the first listed wins, so the answer
// cannot move between calls. DEX +2 against DC 12 and STR +3 against DC 13
// are both net -10.
func (s *CheckTestSuite) TestTiesGoToAuthoredOrder() {
	out, err := s.check(s.seeker(),
		route(string(abilities.DEX), 12),
		route(string(abilities.STR), 13),
	)
	s.Require().NoError(err)

	s.Require().Equal(string(abilities.DEX), out.Applied.Ability)
}

// The lock payload: a bare ability ref rolls the raw ability — STR +3, not
// Athletics +5, even though the seeker is proficient — and the tool ref rides
// the route unread, echoed back on Applied for whoever shelved tool
// proficiency to pick up later.
func (s *CheckTestSuite) TestABareAbilityRouteRollsTheRawAbility() {
	picks := encounter.CheckApproach{
		Ability: string(abilities.STR),
		Tool:    "dnd5e:item:thieves-tools",
		DC:      12,
	}

	out, err := s.check(s.seeker(), picks)
	s.Require().NoError(err)

	s.Require().Equal(straightRoll+3, out.Result.Total,
		"raw STR: proficiency belongs to skills, not to bare abilities")
	s.Require().Equal(picks, out.Applied, "the tool ref survives the trip, unread")
}

// Refusals, by name. Each row is a different author being told a different
// thing: a nil input and a nil roller are wiring, an empty or malformed route
// list is content, a broken sheet is data.
func (s *CheckTestSuite) TestRefusalsByName() {
	valid := []encounter.CheckApproach{route(string(skills.Perception), 10)}

	s.Run("nil input is refused", func() {
		_, err := MakeCheck(s.ctx, nil)
		s.Require().ErrorIs(err, ErrNilInput)
	})

	s.Run("a check without dice is refused", func() {
		_, err := MakeCheck(s.ctx, &CheckInput{Character: s.seeker(), Approaches: valid})
		s.Require().ErrorIs(err, ErrNoRoller)
	})

	s.Run("a check with no routes is refused", func() {
		_, err := s.check(s.seeker())
		s.Require().ErrorIs(err, ErrBadCheck)
	})

	s.Run("a route with no difficulty is refused even when another would win", func() {
		_, err := s.check(s.seeker(), valid[0], route(string(abilities.STR), 0))
		s.Require().ErrorIs(err, ErrBadCheck,
			"broken content is broken today, not on the day it becomes best")
	})

	s.Run("a route naming no rulebook skill or ability is refused", func() {
		_, err := s.check(s.seeker(), valid[0], route("lockpicking", 10))
		s.Require().ErrorIs(err, ErrBadCheck, "never a silent zero from an unknown key")
	})

	s.Run("a checker with no sheet is refused", func() {
		_, err := s.check(nil, valid[0])
		s.Require().ErrorIs(err, ErrBadParticipant)
	})
}

// The strictness contrast with the projection, pinned side by side the way
// TestTheProjectionReadsWhatResolveRefuses pins it against Resolve: the same
// record projects (read-only, drop and warn) and is REFUSED here, because a
// check rolled past a condition that could not load is exactly the unaided
// check the ruling forbids.
func (s *CheckTestSuite) TestTheCheckRefusesWhatTheProjectionReads() {
	unreadable := json.RawMessage(`{"ref":"nonsense","x":`)

	_, err := ProjectCharacter(s.ctx, &ProjectCharacterInput{
		Character: s.seeker(unreadable),
	})
	s.Require().NoError(err, "the read entry tolerates what it cannot parse")

	_, err = s.check(s.seeker(unreadable), route(string(skills.Perception), 10))
	s.Require().Error(err,
		"a rules verdict must not be computed past a condition nobody could load")
}

// The teardown half, asserted on the bus rather than on the teardown call so
// the pin survives a change to which mechanism does the work: Raging joins
// the check chain on the way in, and after the entry returns that chain
// answers nobody.
func (s *CheckTestSuite) TestTheCheckLeavesNothingOnTheBus() {
	inner := events.NewEventBus()

	out, err := makeCheckOn(s.ctx, &CheckInput{
		Character:  s.seeker(s.raging()),
		Approaches: []encounter.CheckApproach{route(string(skills.Athletics), 12)},
		Roller:     s.roller,
	}, newSurface(inner))
	s.Require().NoError(err)
	s.Require().Equal(advantageRoll, out.Result.Roll, "it folded before it tore down")

	event := &dnd5eEvents.AbilityCheckChainEvent{
		CheckerID: seekerID,
		Skill:     skills.Athletics,
		DC:        12,
	}
	chain := events.NewStagedChain[*dnd5eEvents.AbilityCheckChainEvent](combat.ModifierStages)

	modified, err := dnd5eEvents.AbilityCheckChain.On(inner).PublishWithChain(s.ctx, event, chain)
	s.Require().NoError(err)

	folded, err := modified.Execute(s.ctx, event)
	s.Require().NoError(err)

	s.Require().Empty(folded.AdvantageSources,
		"Raging attached during the check and does not answer this chain afterwards")
}

func TestCheckSuite(t *testing.T) {
	suite.Run(t, new(CheckTestSuite))
}
