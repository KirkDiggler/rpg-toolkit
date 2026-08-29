// Copyright (C) 2026 Kirk Diggler
// SPDX-License-Identifier: GPL-3.0-or-later

package resolution

import (
	"encoding/json"

	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/refs"
)

// The static half of the projection's answer: the numbers a caller needs about
// a character it is only allowed to hold a RECORD of.
//
// These hang off ProjectionTestSuite rather than a suite of their own, so they
// share its fixtures without re-running its tests under a second name.

// The facts come off the reconstituted sheet, and SPEED is the one that proves
// it happened.
//
// Every other field could have been echoed straight out of the record the
// caller already holds. Speed could not: it is not stored anywhere, it is
// derived from race when asked, and a Human's 30 is a number only a loaded
// sheet can produce. If this entry ever starts copying the record instead of
// reading the sheet, this is the assertion that notices.
func (s *ProjectionTestSuite) TestTheFactsComeOffTheLoadedSheet() {
	out, err := ProjectCharacter(s.ctx, &ProjectCharacterInput{Character: s.barbarian()})
	s.Require().NoError(err)

	s.Equal(projectedHeroID, out.Sheet.ID)
	s.Equal("Standre", out.Sheet.Name)
	s.Equal(1, out.Sheet.Level)
	s.Equal(14, out.Sheet.HitPoints)
	s.Equal(14, out.Sheet.MaxHitPoints)
	s.Equal(2, out.Sheet.ProficiencyBonus)

	s.Equal(30, out.Sheet.SpeedFeet,
		"a Human walks 30 feet, and that number is on no record — only a loaded sheet knows it")
}

// AN EMPTY HAND IS AN UNARMED STRIKE, not an absent attack.
//
// This fixture equips nothing at all, and the main hand still compiles: the
// rules say a creature with empty hands can punch, so character.AssembleAttack
// falls back to the unarmed strike rather than refusing. The projection reports
// what the rules say rather than what the inventory looks like.
//
// Worth pinning because the shape of the output was nearly wrong here. Planning
// this entry assumed there would be a "no main-hand attack" case for nil to
// mean; there is not, for any character the loader will produce. The pointer
// survives only for the reason the next test states.
func (s *ProjectionTestSuite) TestAnEmptyHandProjectsAnUnarmedStrike() {
	out, err := ProjectCharacter(s.ctx, &ProjectCharacterInput{Character: s.barbarian()})
	s.Require().NoError(err)

	s.Require().NotNil(out.MainHand, "empty hands still punch")
	s.Equal(refs.Weapons.UnarmedStrike().ID, out.MainHand.Ref.ID)
	s.Equal(AttackKindMelee, out.MainHand.Kind, "a punch is made in reach")
	s.Equal(5, out.MainHand.RangeFeet, "and reach is 5 feet")
}

// Kind is never left empty on an attack that compiled, which is what makes the
// empty string safe to read as "nothing here".
//
// The zero value is load-bearing in the other direction: a bool would have to
// call one of melee/ranged false, so an unset field would read as a real
// answer. Nothing asserts this by staring at the type, so it is asserted here.
func (s *ProjectionTestSuite) TestACompiledAttackAlwaysNamesItsKind() {
	out, err := ProjectCharacter(s.ctx, &ProjectCharacterInput{Character: s.barbarian()})
	s.Require().NoError(err)
	s.Require().NotNil(out.MainHand)

	s.Contains([]string{AttackKindMelee, AttackKindRanged}, out.MainHand.Kind,
		"an attack that compiled has a kind; empty is reserved for no attack at all")
}

// The facts survive the same lenient load the armour class does: a record
// carrying a condition this build cannot parse still projects, and the numbers
// come back whole.
func (s *ProjectionTestSuite) TestTheFactsSurviveAnUnreadableCondition() {
	record := s.barbarian(json.RawMessage(`{"ref":"nonsense","x":`))

	out, err := ProjectCharacter(s.ctx, &ProjectCharacterInput{Character: record})
	s.Require().NoError(err)

	s.Equal(30, out.Sheet.SpeedFeet, "the rest of the character still loaded")
	s.Require().NotNil(out.MainHand)
}
