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
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/gamectx"
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
	final, err := s.tryFoldAC(ctx, ud, characterID, baseTotal, false)
	s.Require().NoError(err)

	return final
}

// tryFoldAC is [foldAC] that hands back the fold's error instead of requiring
// none, for the refusals below.
func (s *UnarmoredDefenseCastSuite) tryFoldAC(
	ctx context.Context, ud *UnarmoredDefenseCondition, characterID string, baseTotal int, hasArmor bool,
) (*combat.ACChainEvent, error) {
	s.T().Helper()
	s.Require().NoError(ud.Apply(s.ctx, s.bus))

	event := &combat.ACChainEvent{
		CharacterID: characterID,
		Breakdown:   &combat.ACBreakdown{Total: baseTotal, Components: []combat.ACComponent{}},
		HasArmor:    hasArmor,
		HasShield:   false,
	}

	acChain := events.NewStagedChain[*combat.ACChainEvent](combat.ModifierStages)
	modified, err := combat.ACChain.On(s.bus).PublishWithChain(ctx, event, acChain)
	if err != nil {
		return nil, err
	}

	return modified.Execute(ctx, event)
}

// A barbarian reads CON off its own member. 10 + DEX(+2) + CON(+3) = 15.
//
// The same fixture the session-level pin uses, so the two are comparable by
// eye: a barbarian whose stored scalar says 11 and whose unattached fold says
// 12 must read 15 once the cast is installed.
func (s *UnarmoredDefenseCastSuite) TestABarbarianReadsCONOffTheCast() {
	ud := NewUnarmoredDefenseCondition(UnarmoredDefenseInput{
		MemberID: "barb-1",
		Type:     UnarmoredDefenseBarbarian,
		Source:   "dnd5e:classes:barbarian",
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
		MemberID: "monk-1",
		Type:     UnarmoredDefenseMonk,
		Source:   "dnd5e:classes:monk",
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

// NO CAST AT ALL: the fold REFUSES with gamectx.ErrNotInCast.
//
// This used to pin "the chain comes back untouched, and no error", on the
// theory that an erroring contributor discards every other contributor. That
// was true when EffectiveAC swallowed fold errors; it returns them now, and an
// untouched chain here is base armour for a monk who has Unarmored Defense —
// the number rpg-api saved on equip (rpg-toolkit#1965 tier 1 #2). A refusal is
// the only answer that is not a lie.
func (s *UnarmoredDefenseCastSuite) TestNoCastRefusesTheFold() {
	ud := NewUnarmoredDefenseCondition(UnarmoredDefenseInput{
		MemberID: "monk-1",
		Type:     UnarmoredDefenseMonk,
		Source:   "dnd5e:classes:monk",
	})

	_, err := s.tryFoldAC(context.Background(), ud, "monk-1", 13, false)

	s.Require().ErrorIs(err, gamectx.ErrNotInCast, "no cast, no WIS — and 13 would be a wrong AC")
}

// A cast that does not hold THIS character refuses the same way.
//
// Distinct from the case above rather than a duplicate of it: a cast is
// installed and answers questions, it simply cannot name this member — a
// roster the condition is genuinely absent from. Collapsing the two would let
// a lookup that ignored its own ID pass.
func (s *UnarmoredDefenseCastSuite) TestACastWithoutThisCharacterRefusesTheFold() {
	ud := NewUnarmoredDefenseCondition(UnarmoredDefenseInput{
		MemberID: "monk-1",
		Type:     UnarmoredDefenseMonk,
		Source:   "dnd5e:classes:monk",
	})

	ctx := castOf(s.ctx, &fakeConditionOwner{
		id: "somebody-else",
		scores: shared.AbilityScores{
			abilities.DEX: 16,
			abilities.WIS: 20, // +5 — would be unmistakable if it leaked in
		},
	})

	_, err := s.tryFoldAC(ctx, ud, "monk-1", 13, false)

	s.Require().ErrorIs(err, gamectx.ErrNotInCast, "another member's wisdom is not this monk's")
}

// Armoured, the condition contributes nothing and therefore needs nobody: no
// cast is no refusal. The refusal is for a contribution that would be missing,
// not a blanket "no cast, no AC".
func (s *UnarmoredDefenseCastSuite) TestAnArmouredHolderNeedsNoCast() {
	ud := NewUnarmoredDefenseCondition(UnarmoredDefenseInput{
		MemberID: "monk-1",
		Type:     UnarmoredDefenseMonk,
		Source:   "dnd5e:classes:monk",
	})

	final, err := s.tryFoldAC(context.Background(), ud, "monk-1", 14, true)

	s.Require().NoError(err)
	s.Equal(14, final.Breakdown.Total)
	s.Empty(final.Breakdown.Components, "Unarmored Defense does not apply in armour")
}
