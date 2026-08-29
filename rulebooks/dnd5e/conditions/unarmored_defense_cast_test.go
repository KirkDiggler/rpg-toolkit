// Copyright (C) 2026 Kirk Diggler
// SPDX-License-Identifier: GPL-3.0-or-later

package conditions

import (
	"context"
	"testing"

	"github.com/stretchr/testify/suite"

	"github.com/KirkDiggler/rpg-toolkit/events"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/abilities"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/combat"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/shared"
)

// UnarmoredDefenseCastSuite pins what the condition contributes now that it
// reads itself off the cast, BY VALUE and in both class variants.
//
// By value rather than by round trip, because the two variants differ only in
// which ability they read: a swap of CON for WIS is invisible to a test that
// only checks "some feature component appeared", and the barbarian and monk
// fixtures below are built so the wrong ability produces a DIFFERENT number
// rather than the same one by luck.
type UnarmoredDefenseCastSuite struct {
	suite.Suite
	ctx context.Context
	bus events.EventBus
}

func (s *UnarmoredDefenseCastSuite) SetupTest() {
	s.ctx = context.Background()
	s.bus = events.NewEventBus()
}

func TestUnarmoredDefenseCastSuite(t *testing.T) { suite.Run(t, new(UnarmoredDefenseCastSuite)) }

// foldAC applies the condition, folds an unarmored AC chain over the given
// context, and reports the finished event.
//
// baseTotal is what the sheet's own arithmetic already put on the breakdown
// before any condition contributes — 10 + DEX — so the assertions below read as
// "the condition added N", which is the only thing this condition does.
func (s *UnarmoredDefenseCastSuite) foldAC(
	ctx context.Context, ud *UnarmoredDefenseCondition, characterID string, baseTotal int,
) *combat.ACChainEvent {
	s.T().Helper()
	s.Require().NoError(ud.Apply(s.ctx, s.bus))

	event := &combat.ACChainEvent{
		CharacterID: characterID,
		Breakdown:   &combat.ACBreakdown{Total: baseTotal, Components: []combat.ACComponent{}},
		HasArmor:    false,
		HasShield:   false,
	}

	acChain := events.NewStagedChain[*combat.ACChainEvent](combat.ModifierStages)
	modified, err := combat.ACChain.On(s.bus).PublishWithChain(ctx, event, acChain)
	s.Require().NoError(err)

	final, err := modified.Execute(ctx, event)
	s.Require().NoError(err,
		"a condition that cannot answer must leave the chain untouched, never error: "+
			"an erroring contributor discards every other AC contributor with it")

	return final
}

// A barbarian reads CON off its own member. 10 + DEX(+2) + CON(+3) = 15.
//
// The same fixture the session-level pin uses, so the two are comparable by
// eye: a barbarian whose stored scalar says 11 and whose unattached fold says
// 12 must read 15 once the cast is installed.
func (s *UnarmoredDefenseCastSuite) TestABarbarianReadsCONOffTheCast() {
	ud := NewUnarmoredDefenseCondition(UnarmoredDefenseInput{
		CharacterID: "barb-1",
		Type:        UnarmoredDefenseBarbarian,
		Source:      "dnd5e:classes:barbarian",
	})

	ctx := castOf(s.ctx, &fakeConditionOwner{
		id: "barb-1",
		scores: shared.AbilityScores{
			abilities.STR: 16,
			abilities.DEX: 14, // +2, already in the base below
			abilities.CON: 16, // +3, what this condition adds
			abilities.INT: 10,
			abilities.WIS: 8, // -1: reading WIS instead of CON gives 11, not 15
			abilities.CHA: 10,
		},
	})

	final := s.foldAC(ctx, ud, "barb-1", 12)

	s.Equal(15, final.Breakdown.Total, "12 base (10 + DEX) + CON(+3) = 15")
	s.Require().Len(final.Breakdown.Components, 1)
	s.Equal(combat.ACSourceFeature, final.Breakdown.Components[0].Type)
	s.Equal(3, final.Breakdown.Components[0].Value, "CON, not WIS: WIS here is -1")
}

// A monk reads WIS off the same seam. Mirror fixture: reading CON instead
// would give 18, so the number names which ability was read.
func (s *UnarmoredDefenseCastSuite) TestAMonkReadsWISOffTheCast() {
	ud := NewUnarmoredDefenseCondition(UnarmoredDefenseInput{
		CharacterID: "monk-1",
		Type:        UnarmoredDefenseMonk,
		Source:      "dnd5e:classes:monk",
	})

	ctx := castOf(s.ctx, &fakeConditionOwner{
		id: "monk-1",
		scores: shared.AbilityScores{
			abilities.STR: 10,
			abilities.DEX: 16, // +3, already in the base below
			abilities.CON: 18, // +4: reading CON instead of WIS gives 17, not 15
			abilities.INT: 10,
			abilities.WIS: 14, // +2, what this condition adds
			abilities.CHA: 10,
		},
	})

	final := s.foldAC(ctx, ud, "monk-1", 13)

	s.Equal(15, final.Breakdown.Total, "13 base (10 + DEX) + WIS(+2) = 15")
	s.Require().Len(final.Breakdown.Components, 1)
	s.Equal(2, final.Breakdown.Components[0].Value, "WIS, not CON: CON here is +4")
}

// NO CAST AT ALL: the chain comes back untouched, and no error.
//
// The pair of assertions is the point. "Left alone" and "did not blow up" are
// separate promises, and the second is the one that matters most: an errored
// fold discards every OTHER contributor to that armour class, which is how a
// barbarian ended up at base AC with nothing logged.
func (s *UnarmoredDefenseCastSuite) TestNoCastLeavesTheChainUntouched() {
	ud := NewUnarmoredDefenseCondition(UnarmoredDefenseInput{
		CharacterID: "monk-1",
		Type:        UnarmoredDefenseMonk,
		Source:      "dnd5e:classes:monk",
	})

	final := s.foldAC(context.Background(), ud, "monk-1", 13)

	s.Equal(13, final.Breakdown.Total, "no cast, no contribution — and no damage to the rest")
	s.Empty(final.Breakdown.Components, "nothing may be attributed that was not read")
}

// A cast that does not hold THIS character is the same answer as no cast.
//
// Distinct from the case above rather than a duplicate of it: a cast is
// installed and answers questions, it simply cannot name this member — a
// roster the condition is genuinely absent from. Collapsing the two would let
// a lookup that ignored its own ID pass.
func (s *UnarmoredDefenseCastSuite) TestACastWithoutThisCharacterLeavesTheChainUntouched() {
	ud := NewUnarmoredDefenseCondition(UnarmoredDefenseInput{
		CharacterID: "monk-1",
		Type:        UnarmoredDefenseMonk,
		Source:      "dnd5e:classes:monk",
	})

	ctx := castOf(s.ctx, &fakeConditionOwner{
		id: "somebody-else",
		scores: shared.AbilityScores{
			abilities.DEX: 16,
			abilities.WIS: 20, // +5 — would be unmistakable if it leaked in
		},
	})

	final := s.foldAC(ctx, ud, "monk-1", 13)

	s.Equal(13, final.Breakdown.Total, "another member's wisdom is not this monk's")
	s.Empty(final.Breakdown.Components)
}
