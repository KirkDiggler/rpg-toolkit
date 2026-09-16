// Copyright (C) 2026 Kirk Diggler
// SPDX-License-Identifier: GPL-3.0-or-later

package choices

import (
	"testing"

	"github.com/stretchr/testify/suite"

	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/classes"
)

// LevelRequirementsTestSuite covers the difference between what a level-N
// character must have chosen in total and what level N itself asks for.
type LevelRequirementsTestSuite struct {
	suite.Suite
}

func TestLevelRequirementsSuite(t *testing.T) {
	suite.Run(t, new(LevelRequirementsTestSuite))
}

func (s *LevelRequirementsTestSuite) TestLevelOneGainsEveryChoiceTheClassAsksFor() {
	gained := GetClassChoiceIDsGainedAtLevel(classes.Fighter, 1)

	s.Equal(GetClassRequirementsAtLevel(classes.Fighter, 1).ChoiceIDs(), gained,
		"there is no level before level 1, so every choice is gained there")
	s.Contains(gained, FighterSkills)
	s.Contains(gained, FighterFightingStyle)
}

func (s *LevelRequirementsTestSuite) TestAFighterSecondLevelAsksForNothingNew() {
	gained := GetClassChoiceIDsGainedAtLevel(classes.Fighter, 2)

	s.Empty(gained,
		"every choice a level-2 fighter must have made, it made at level 1")
}

func (s *LevelRequirementsTestSuite) TestAFighterThirdLevelGainsTheSubclassChoice() {
	gained := GetClassChoiceIDsGainedAtLevel(classes.Fighter, 3)

	subclass := GetClassRequirementsAtLevel(classes.Fighter, 3).Subclass
	s.Require().NotNil(subclass, "the fighter picks a Martial Archetype at 3")
	s.Equal([]ChoiceID{subclass.ID}, gained,
		"the archetype is the only thing level 3 newly asks for")
}

func (s *LevelRequirementsTestSuite) TestAClericGainsItsDomainAtLevelOneAndNothingAtTwo() {
	atOne := GetClassChoiceIDsGainedAtLevel(classes.Cleric, 1)
	atTwo := GetClassChoiceIDsGainedAtLevel(classes.Cleric, 2)

	domain := GetClassRequirementsAtLevel(classes.Cleric, 1).Subclass
	s.Require().NotNil(domain, "a cleric picks its domain at level 1")
	s.Contains(atOne, domain.ID)
	s.Empty(atTwo, "and is asked for nothing new at level 2")
}

func (s *LevelRequirementsTestSuite) TestAWizardGainsItsSchoolAtLevelTwo() {
	gained := GetClassChoiceIDsGainedAtLevel(classes.Wizard, 2)

	school := GetClassRequirementsAtLevel(classes.Wizard, 2).Subclass
	s.Require().NotNil(school, "a wizard picks its Arcane Tradition at level 2")
	s.Equal([]ChoiceID{school.ID}, gained)
}

func (s *LevelRequirementsTestSuite) TestALevelBelowOneGainsNothing() {
	s.Nil(GetClassChoiceIDsGainedAtLevel(classes.Fighter, 0))
}

func (s *LevelRequirementsTestSuite) TestChoiceIDsNamesEveryKindOfRequirement() {
	reqs := GetClassRequirementsAtLevel(classes.Rogue, 3)
	ids := reqs.ChoiceIDs()

	s.Contains(ids, reqs.Skills.ID)
	s.Contains(ids, reqs.Expertise.ID)
	s.Require().NotNil(reqs.Subclass)
	s.Contains(ids, reqs.Subclass.ID)
	for _, equipment := range reqs.Equipment {
		s.Contains(ids, equipment.ID)
	}
}

func (s *LevelRequirementsTestSuite) TestChoiceIDsOfNilRequirementsIsNil() {
	var reqs *Requirements
	s.Nil(reqs.ChoiceIDs())
}
