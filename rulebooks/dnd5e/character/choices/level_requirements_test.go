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

	s.Equal(GetClassRequirementsGainedAtLevel(classes.Fighter, 1).ChoiceIDs(), gained,
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

	subclass := GetClassRequirementsGainedAtLevel(classes.Fighter, 3).Subclass
	s.Require().NotNil(subclass, "the fighter picks a Martial Archetype at 3")
	s.Equal([]ChoiceID{subclass.ID}, gained,
		"the archetype is the only thing level 3 newly asks for")
}

func (s *LevelRequirementsTestSuite) TestAClericGainsItsDomainAtOneAndAnotherPreparationAtTwo() {
	atOne := GetClassChoiceIDsGainedAtLevel(classes.Cleric, 1)
	atTwo := GetClassChoiceIDsGainedAtLevel(classes.Cleric, 2)

	domain := GetClassRequirementsGainedAtLevel(classes.Cleric, 1).Subclass
	s.Require().NotNil(domain, "a cleric picks its domain at level 1")
	s.Contains(atOne, domain.ID)
	s.Equal([]ChoiceID{SpellChoiceID(classes.Cleric, 2)}, atTwo)
}

func (s *LevelRequirementsTestSuite) TestAWizardGainsItsSchoolAndTwoSpellbookSpellsAtLevelTwo() {
	gained := GetClassChoiceIDsGainedAtLevel(classes.Wizard, 2)

	school := GetClassRequirementsGainedAtLevel(classes.Wizard, 2).Subclass
	s.Require().NotNil(school, "a wizard picks its Arcane Tradition at level 2")
	s.Contains(gained, school.ID)

	// The spellbook question is the progression table speaking: six spells at
	// level 1, eight at level 2, and the difference is the choice. Before the
	// table existed the school was the only thing any class gained above 1.
	spellbook := GetClassRequirementsGainedAtLevel(classes.Wizard, 2).Spellbook
	s.Require().NotNil(spellbook, "a wizard adds two spells to its spellbook with every level")
	s.Equal(ChoiceID("wizard-spells-2"), spellbook.ID)
	s.Equal(2, spellbook.Count)
	s.Contains(gained, spellbook.ID)
}

func (s *LevelRequirementsTestSuite) TestALevelBelowOneGainsNothing() {
	s.Nil(GetClassChoiceIDsGainedAtLevel(classes.Fighter, 0))
}

func (s *LevelRequirementsTestSuite) TestChoiceIDsNamesEveryKindOfRequirement() {
	// Built by hand rather than taken from a class, because the claim is about
	// the method and no single class level carries all eleven kinds. A kind
	// this method forgets is a choice a level-up would never ask for.
	reqs := &Requirements{
		Skills:              &SkillRequirement{ID: "a-skills"},
		AdditionalSkills:    []*SkillRequirement{{ID: "a-additional-skills"}},
		Equipment:           []*EquipmentRequirement{{ID: "a-equipment"}},
		EquipmentCategories: []*EquipmentCategoryRequirement{{ID: "a-equipment-categories"}},
		Languages:           []*LanguageRequirement{{ID: "a-languages"}},
		Tools:               &ToolRequirement{ID: "a-tools"},
		FightingStyle:       &FightingStyleRequirement{ID: "a-fighting-style"},
		Expertise:           &ExpertiseRequirement{ID: "a-expertise"},
		Subclass:            &SubclassRequirement{ID: "a-subclass"},
		Cantrips:            &CantripRequirement{ID: "a-cantrips"},
		Spellbook:           &SpellbookRequirement{ID: "a-spellbook"},
	}

	s.ElementsMatch([]ChoiceID{
		"a-skills", "a-additional-skills", "a-equipment", "a-equipment-categories",
		"a-languages", "a-tools", "a-fighting-style", "a-expertise", "a-subclass",
		"a-cantrips", "a-spellbook",
	}, reqs.ChoiceIDs())
}

func (s *LevelRequirementsTestSuite) TestARogueGainsItsArchetypeAtThreeAndNothingElse() {
	reqs := GetClassRequirementsGainedAtLevel(classes.Rogue, 3)

	s.Require().NotNil(reqs.Subclass, "a rogue picks its Roguish Archetype at 3")
	s.Equal([]ChoiceID{reqs.Subclass.ID}, reqs.ChoiceIDs(),
		"level 3 asks for the archetype and nothing level 1 already asked")
	s.Nil(reqs.Skills, "the skills were chosen at level 1 and are not asked again")
	s.Empty(reqs.Equipment, "nor is the starting equipment")
}

func (s *LevelRequirementsTestSuite) TestChoiceIDsOfNilRequirementsIsNil() {
	var reqs *Requirements
	s.Nil(reqs.ChoiceIDs())
}
