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

// A goblin's WIS 8 is a -1 modifier, so its passive Insight is 9 — the number
// ideas/shenanigans/intimidate.md names as the goblin's derived DC.
func (s *PassiveInsightTestSuite) TestAGoblinsWisdomAnswersNine() {
	s.Equal(9, s.blob(8, 2).PassiveInsight())
}

// A thug's WIS 10 is a +0 modifier: passive Insight 10.
func (s *PassiveInsightTestSuite) TestAThugsWisdomAnswersTen() {
	s.Equal(10, s.blob(10, 2).PassiveInsight())
}

// Listing Insight among the proficiencies adds the creature's proficiency
// bonus. The entry's own Bonus is deliberately not the number used: the
// proficiency list is read here as a membership test.
func (s *PassiveInsightTestSuite) TestInsightProficiencyAddsTheProficiencyBonus() {
	plain := s.blob(14, 3)
	proficient := s.blob(14, 3, ProficiencyData{Skill: string(skills.Insight), Bonus: 99})

	s.Equal(12, plain.PassiveInsight())
	s.Equal(15, proficient.PassiveInsight(),
		"10 + WIS 14's +2 + the creature's own +3, never the entry's authored Bonus")
}

// A proficiency in something else is not a proficiency in Insight.
func (s *PassiveInsightTestSuite) TestAnotherSkillsProficiencyChangesNothing() {
	s.Equal(12, s.blob(14, 3, ProficiencyData{Skill: string(skills.Stealth), Bonus: 5}).PassiveInsight())
}

// An absent proficiency bonus means 2, the same rule the loader applies, so a
// blob and the sheet loaded from it cannot disagree.
func (s *PassiveInsightTestSuite) TestAnAbsentProficiencyBonusMeansTwo() {
	s.Equal(12, s.blob(10, 0, ProficiencyData{Skill: string(skills.Insight)}).PassiveInsight())
}

// The loaded sheet answers what the blob it came from answers — for a
// proficient creature and a plain one alike.
func (s *PassiveInsightTestSuite) TestTheSheetAnswersWhatItsBlobAnswers() {
	for _, blob := range []*Data{
		s.blob(8, 2),
		s.blob(14, 3, ProficiencyData{Skill: string(skills.Insight), Bonus: 99}),
		s.blob(10, 0, ProficiencyData{Skill: string(skills.Insight)}),
	} {
		loaded, err := Load(context.Background(), blob)
		s.Require().NoError(err)
		s.Equal(blob.PassiveInsight(), loaded.PassiveInsight())
	}
}

// Nothing stores the number. A blob written back out carries no passive
// Insight to go stale, which is what "passive is derived, never stored" means
// in practice.
func (s *PassiveInsightTestSuite) TestPassiveInsightIsNotSerialized() {
	loaded, err := Load(context.Background(), s.blob(14, 3, ProficiencyData{Skill: string(skills.Insight)}))
	s.Require().NoError(err)
	s.Equal(15, loaded.PassiveInsight())

	written := loaded.ToData()
	s.Equal(SensesData{}, written.Senses,
		"no senses field carries passive Insight; raising Wisdom must move the answer")

	written.AbilityScores[abilities.WIS] = 18
	s.Equal(17, written.PassiveInsight())
}
