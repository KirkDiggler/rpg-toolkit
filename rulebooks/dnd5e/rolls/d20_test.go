// Copyright (C) 2026 Kirk Diggler
// SPDX-License-Identifier: GPL-3.0-or-later

package rolls_test

import (
	"context"
	"testing"

	"github.com/stretchr/testify/suite"
	"go.uber.org/mock/gomock"

	"github.com/KirkDiggler/rpg-toolkit/core"
	mock_dice "github.com/KirkDiggler/rpg-toolkit/dice/mock"
	dnd5eEvents "github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/events"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/refs"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/rolls"
)

// RollD20TestSuite pins the four shapes one d20 pool can take, and the
// refusals that keep a wrong record from ever being written. These are the
// facts every d20 machine used to answer its own way.
type RollD20TestSuite struct {
	suite.Suite
	ctrl   *gomock.Controller
	ctx    context.Context
	roller *mock_dice.MockRoller
}

func TestRollD20Suite(t *testing.T) {
	suite.Run(t, new(RollD20TestSuite))
}

func (s *RollD20TestSuite) SetupTest() {
	s.ctrl = gomock.NewController(s.T())
	s.ctx = context.Background()
	s.roller = mock_dice.NewMockRoller(s.ctrl)
}

func (s *RollD20TestSuite) TearDownTest() {
	s.ctrl.Finish()
}

func strikeSource() dnd5eEvents.RollSource {
	return dnd5eEvents.RollSource{Ref: refs.Actions.Strike(), Name: "Strike", SourceID: "hero"}
}

func ruleSource(name string, ref *core.Ref, entity string) dnd5eEvents.RollSource {
	return dnd5eEvents.RollSource{Ref: ref, Name: name, SourceID: entity}
}

// componentFor wraps a rolled pool the way its caller does, so the trace can be
// put through the validator that guards the seam.
func componentFor(source dnd5eEvents.RollSource, out *rolls.RollD20Output) *dnd5eEvents.RollCalculation {
	return dnd5eEvents.NewRollCalculation([]dnd5eEvents.RollComponent{{Source: source, Dice: out.Trace}})
}

// TestStraightRoll: nobody touched the pool, so the zero value tells the truth.
func (s *RollD20TestSuite) TestStraightRoll() {
	s.roller.EXPECT().Roll(s.ctx, 20).Return(11, nil)

	out, err := rolls.RollD20(s.ctx, &rolls.RollD20Input{Roller: s.roller, Source: strikeSource()})
	s.Require().NoError(err)

	s.Equal(11, out.Face)
	s.Equal("1d20", out.Trace.Notation)
	s.Equal([]int{11}, out.Trace.OriginalRolls)
	s.Equal([]int{11}, out.Trace.FinalRolls)
	s.Empty(out.Trace.KeptIndices)
	s.Equal(11, out.Trace.Subtotal)
	s.Nil(out.Trace.Keep)
	s.Require().NoError(dnd5eEvents.ValidateRollCalculation(componentFor(strikeSource(), out)))
}

// TestAdvantageKeepsTheHigherFaceAndSaysWho: two dice, one kept index on the
// max, and the record names the rule and the entity that brought it.
func (s *RollD20TestSuite) TestAdvantageKeepsTheHigherFaceAndSaysWho() {
	s.roller.EXPECT().RollN(s.ctx, 2, 20).Return([]int{7, 18}, nil)

	out, err := rolls.RollD20(s.ctx, &rolls.RollD20Input{
		Roller: s.roller, Source: strikeSource(),
		Granted: []dnd5eEvents.RollSource{
			ruleSource("Reckless Attack", refs.Conditions.RecklessAttack(), "hero"),
		},
	})
	s.Require().NoError(err)

	s.Equal(18, out.Face)
	s.Equal("2d20", out.Trace.Notation)
	s.Equal([]int{7, 18}, out.Trace.FinalRolls, "the discarded face is kept, not thrown away")
	s.Equal([]int{1}, out.Trace.KeptIndices)
	s.Equal(18, out.Trace.Subtotal)
	s.Require().NotNil(out.Trace.Keep)
	s.Equal(dnd5eEvents.KeepAdvantage, out.Trace.Keep.Rule)
	s.Require().Len(out.Trace.Keep.Granted, 1)
	s.Equal("Reckless Attack", out.Trace.Keep.Granted[0].Name)
	s.Equal("hero", out.Trace.Keep.Granted[0].SourceID)
	s.Empty(out.Trace.Keep.Imposed)
	s.Require().NoError(dnd5eEvents.ValidateRollCalculation(componentFor(strikeSource(), out)))
}

// TestAdvantageKeepsTheFirstFaceWhenItIsHigher pins which index is recorded
// when the pair is not in ascending order — the kept index must follow the
// face, not the position.
func (s *RollD20TestSuite) TestAdvantageKeepsTheFirstFaceWhenItIsHigher() {
	s.roller.EXPECT().RollN(s.ctx, 2, 20).Return([]int{18, 7}, nil)

	out, err := rolls.RollD20(s.ctx, &rolls.RollD20Input{
		Roller: s.roller, Source: strikeSource(),
		Granted: []dnd5eEvents.RollSource{
			ruleSource("Reckless Attack", refs.Conditions.RecklessAttack(), "hero"),
		},
	})
	s.Require().NoError(err)

	s.Equal(18, out.Face)
	s.Equal([]int{0}, out.Trace.KeptIndices)
}

// TestDisadvantageKeepsTheLowerFaceAndSaysWho is the mirror, and it is the
// shape the check machine could not record at all.
func (s *RollD20TestSuite) TestDisadvantageKeepsTheLowerFaceAndSaysWho() {
	s.roller.EXPECT().RollN(s.ctx, 2, 20).Return([]int{7, 18}, nil)

	out, err := rolls.RollD20(s.ctx, &rolls.RollD20Input{
		Roller: s.roller, Source: strikeSource(),
		Imposed: []dnd5eEvents.RollSource{
			ruleSource("Untrained", refs.Rules.Untrained(), "hero"),
		},
	})
	s.Require().NoError(err)

	s.Equal(7, out.Face)
	s.Equal([]int{0}, out.Trace.KeptIndices)
	s.Equal(7, out.Trace.Subtotal)
	s.Require().NotNil(out.Trace.Keep)
	s.Equal(dnd5eEvents.KeepDisadvantage, out.Trace.Keep.Rule)
	s.Require().Len(out.Trace.Keep.Imposed, 1)
	s.Equal("Untrained", out.Trace.Keep.Imposed[0].Name)
	s.Empty(out.Trace.Keep.Granted)
	s.Require().NoError(dnd5eEvents.ValidateRollCalculation(componentFor(strikeSource(), out)))
}

// TestCancellationRollsOneDieAndRecordsBoth: RAW rolls one die, we roll one
// die, and the record is the only thing that tells the two cases apart.
func (s *RollD20TestSuite) TestCancellationRollsOneDieAndRecordsBoth() {
	s.roller.EXPECT().Roll(s.ctx, 20).Return(11, nil)

	out, err := rolls.RollD20(s.ctx, &rolls.RollD20Input{
		Roller: s.roller, Source: strikeSource(),
		Granted: []dnd5eEvents.RollSource{ruleSource("Help", refs.Conditions.Helped(), "alice")},
		Imposed: []dnd5eEvents.RollSource{ruleSource("Untrained", refs.Rules.Untrained(), "hero")},
	})
	s.Require().NoError(err)

	s.Equal(11, out.Face)
	s.Equal("1d20", out.Trace.Notation)
	s.Empty(out.Trace.KeptIndices)
	s.Require().NotNil(out.Trace.Keep)
	s.Equal(dnd5eEvents.KeepCancelled, out.Trace.Keep.Rule)
	s.Require().Len(out.Trace.Keep.Granted, 1)
	s.Equal("alice", out.Trace.Keep.Granted[0].SourceID, "advantage is attributed to the helper")
	s.Require().Len(out.Trace.Keep.Imposed, 1)
	s.Require().NoError(dnd5eEvents.ValidateRollCalculation(componentFor(strikeSource(), out)))
}

// TestKeepSourcesAreCopied pins that the record owns its refs: a caller that
// scribbles on the source it passed in cannot reach through and change what
// was recorded.
func (s *RollD20TestSuite) TestKeepSourcesAreCopied() {
	s.roller.EXPECT().RollN(s.ctx, 2, 20).Return([]int{7, 18}, nil)

	granted := ruleSource("Reckless Attack", refs.Conditions.RecklessAttack(), "hero")
	out, err := rolls.RollD20(s.ctx, &rolls.RollD20Input{
		Roller: s.roller, Source: strikeSource(),
		Granted: []dnd5eEvents.RollSource{granted},
	})
	s.Require().NoError(err)

	s.NotSame(granted.Ref, out.Trace.Keep.Granted[0].Ref)
	s.Equal(granted.Ref.String(), out.Trace.Keep.Granted[0].Ref.String())
}

// TestRefusesAnAnonymousPool is R7 at the roller: a d20 with no entity behind
// it is refused before the die is thrown, not recorded anonymously.
func (s *RollD20TestSuite) TestRefusesAnAnonymousPool() {
	source := strikeSource()
	source.SourceID = ""

	out, err := rolls.RollD20(s.ctx, &rolls.RollD20Input{Roller: s.roller, Source: source})

	s.Require().Error(err)
	s.Nil(out)
	s.Contains(err.Error(), "source id is required for a d20 roll")
}

func (s *RollD20TestSuite) TestRefusesSourcelessRules() {
	tests := []struct {
		name    string
		input   *rolls.RollD20Input
		message string
	}{
		{
			name:    "nil input",
			input:   nil,
			message: "roll d20 input is required",
		},
		{
			name:    "no roller",
			input:   &rolls.RollD20Input{Source: strikeSource()},
			message: "roll d20 roller is required",
		},
		{
			name: "granted rule with no ref",
			input: &rolls.RollD20Input{
				Roller:  mock_dice.NewMockRoller(s.ctrl),
				Source:  strikeSource(),
				Granted: []dnd5eEvents.RollSource{ruleSource("Mystery", nil, "hero")},
			},
			message: "source ref is required",
		},
		{
			name: "imposed rule with no entity",
			input: &rolls.RollD20Input{
				Roller:  mock_dice.NewMockRoller(s.ctrl),
				Source:  strikeSource(),
				Imposed: []dnd5eEvents.RollSource{ruleSource("Untrained", refs.Rules.Untrained(), "")},
			},
			message: "source id is required for a disadvantage source",
		},
	}

	for _, test := range tests {
		s.Run(test.name, func() {
			out, err := rolls.RollD20(s.ctx, test.input)

			s.Require().Error(err)
			s.Nil(out)
			s.Contains(err.Error(), test.message)
		})
	}
}
