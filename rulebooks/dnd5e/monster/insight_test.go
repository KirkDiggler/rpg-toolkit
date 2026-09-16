// Copyright (C) 2026 Kirk Diggler
// SPDX-License-Identifier: GPL-3.0-or-later

package monster

import (
	"context"
	"testing"

	"github.com/stretchr/testify/suite"

	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/abilities"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/shared"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/skills"
)

// PassiveInsightTestSuite pins the derived default DC a shenanigan beats when
// the placement authored none (rpg-project#454).
type PassiveInsightTestSuite struct {
	suite.Suite
}

func TestPassiveInsightSuite(t *testing.T) {
	suite.Run(t, new(PassiveInsightTestSuite))
}

func (s *PassiveInsightTestSuite) blob(wis int, profBonus int, proficiencies ...ProficiencyData) *Data {
	return &Data{
		ID:   "sheet-1",
		Name: "Sheet",
		AbilityScores: shared.AbilityScores{
			abilities.STR: 10,
			abilities.DEX: 10,
			abilities.CON: 10,
			abilities.INT: 10,
			abilities.WIS: wis,
			abilities.CHA: 10,
		},
		ProficiencyBonus: profBonus,
		Proficiencies:    proficiencies,
	}
}

// A goblin's WIS 8 is a -1 modifier and its stat block lists no Insight, so
// its passive Insight is 9 — the number ideas/shenanigans/intimidate.md names
// as the goblin's derived DC.
func (s *PassiveInsightTestSuite) TestAnUnlistedInsightFallsBackToWisdom() {
	s.Equal(9, s.blob(8, 2).PassiveInsight(), "a goblin")
	s.Equal(10, s.blob(10, 2).PassiveInsight(), "a thug: WIS 10 is +0")
}

// THE LISTED NUMBER IS THE WHOLE MODIFIER. This fixture's +7 is deliberately
// NOT what Wisdom and proficiency would produce — WIS 14's +2 with a +3
// proficiency bonus is +5 — because an SRD stat block's listed skill already
// includes whatever else the creature has going for it (the goblin's Stealth
// +6 is DEX +2 and Nimble Escape, not DEX plus its +2 proficiency).
//
// So the answer is 17, and the two numbers the old reading would have
// produced are both wrong and both nearby: 15 if the bonus were treated as a
// proficiency flag, 20 if the listed total had the proficiency bonus added on
// top of it.
func (s *PassiveInsightTestSuite) TestAListedInsightIsTheWholeModifier() {
	listed := s.blob(14, 3, ProficiencyData{Skill: string(skills.Insight), Bonus: 7})

	s.Equal(17, listed.PassiveInsight(), "10 + the printed +7, and nothing else")
	s.NotEqual(15, listed.PassiveInsight(), "not 10 + WIS + the CR proficiency bonus")
	s.NotEqual(20, listed.PassiveInsight(), "and never the listed total PLUS that bonus")
}

// The creature's own proficiency bonus is never added to a listed number, at
// any value it takes — including the absent one the loader reads as 2.
func (s *PassiveInsightTestSuite) TestTheProficiencyBonusNeverJoinsAListedNumber() {
	for _, profBonus := range []int{0, 2, 3, 6} {
		listed := s.blob(14, profBonus, ProficiencyData{Skill: string(skills.Insight), Bonus: 7})
		s.Equal(17, listed.PassiveInsight(), "proficiency bonus %d changes nothing", profBonus)
	}
}

// A listed +0 is a real answer and reads as one: the list says whether the
// skill is there at all, separately from what it is worth. Wisdom does not
// get to override it.
func (s *PassiveInsightTestSuite) TestAListedZeroIsAnAnswer() {
	s.Equal(10, s.blob(18, 3, ProficiencyData{Skill: string(skills.Insight)}).PassiveInsight(),
		"listed at +0 despite WIS 18, which is what the stat block said")
}

// A listed skill that is not Insight is not an Insight number.
func (s *PassiveInsightTestSuite) TestAnotherListedSkillChangesNothing() {
	s.Equal(12, s.blob(14, 3, ProficiencyData{Skill: string(skills.Stealth), Bonus: 6}).PassiveInsight())
}

// The loaded sheet answers what the blob it came from answers, listed or not.
func (s *PassiveInsightTestSuite) TestTheSheetAnswersWhatItsBlobAnswers() {
	for _, blob := range []*Data{
		s.blob(8, 2),
		s.blob(14, 3, ProficiencyData{Skill: string(skills.Insight), Bonus: 7}),
		s.blob(18, 3, ProficiencyData{Skill: string(skills.Insight)}),
		s.blob(14, 3, ProficiencyData{Skill: string(skills.Stealth), Bonus: 6}),
	} {
		loaded, err := Load(context.Background(), blob)
		s.Require().NoError(err)
		s.Equal(blob.PassiveInsight(), loaded.PassiveInsight())
	}
}

// Nothing stores the number. A blob written back out carries no passive
// Insight to go stale, which is what "passive is derived, never stored" means
// in practice: move the stat block and the answer moves.
func (s *PassiveInsightTestSuite) TestPassiveInsightIsNotSerialized() {
	loaded, err := Load(context.Background(), s.blob(14, 3))
	s.Require().NoError(err)
	s.Equal(12, loaded.PassiveInsight())

	written := loaded.ToData()
	s.Equal(SensesData{}, written.Senses,
		"no senses field carries passive Insight; raising Wisdom must move the answer")

	written.AbilityScores[abilities.WIS] = 18
	s.Equal(14, written.PassiveInsight())

	written.Proficiencies = append(written.Proficiencies,
		ProficiencyData{Skill: string(skills.Insight), Bonus: 7})
	s.Equal(17, written.PassiveInsight(), "and listing the skill moves it again")
}
