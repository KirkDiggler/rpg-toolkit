package choices

import (
	"sync"
	"testing"

	"github.com/stretchr/testify/suite"

	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/armor"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/classes"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/items"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/shared"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/tools"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/weapons"
)

// CategoryChoiceOptionsTestSuite verifies that category-choice options are the
// same concrete items accepted by the category validator.
type CategoryChoiceOptionsTestSuite struct {
	suite.Suite
	validator *Validator
}

func TestCategoryChoiceOptionsSuite(t *testing.T) {
	suite.Run(t, new(CategoryChoiceOptionsTestSuite))
}

func (s *CategoryChoiceOptionsTestSuite) SetupTest() {
	s.validator = NewValidator()
}

func (s *CategoryChoiceOptionsTestSuite) TestExpansionMatchesValidatorForEveryClassCategoryChoice() {
	for _, classID := range implementedClasses {
		s.Run(string(classID), func() {
			for _, choice := range classCategoryChoices(GetClassRequirements(classID)) {
				expanded := equipmentItemIDs(choice.Options)
				expected := make(map[shared.EquipmentID]struct{})

				for _, id := range allCategoryCandidates() {
					if s.categorySelectionValid(choice, id) {
						expected[id] = struct{}{}
					}
				}

				s.Equal(expected, expanded,
					"category choice %q must enumerate exactly the IDs the validator accepts", choice.Label)
			}
		})
	}
}

func (s *CategoryChoiceOptionsTestSuite) TestWeaponChoicesIncludeBothMeleeAndRangedMembership() {
	for _, classID := range implementedClasses {
		for _, choice := range classCategoryChoices(GetClassRequirements(classID)) {
			if choice.Type != shared.EquipmentTypeWeapon {
				continue
			}

			categories := make(map[shared.EquipmentCategory]struct{}, len(choice.Categories))
			for _, category := range choice.Categories {
				categories[category] = struct{}{}
			}
			options := equipmentItemIDs(choice.Options)

			if _, ok := categories[weapons.CategorySimpleMelee]; ok {
				if _, ranged := categories[weapons.CategorySimpleRanged]; ranged {
					s.Contains(options, weapons.Club, "%s simple choice must include melee weapons", choice.Label)
					s.Contains(options, weapons.Dart, "%s simple choice must include ranged weapons", choice.Label)
				}
			}
			if _, ok := categories[weapons.CategoryMartialMelee]; ok {
				if _, ranged := categories[weapons.CategoryMartialRanged]; ranged {
					s.Contains(options, weapons.Longsword, "%s martial choice must include melee weapons", choice.Label)
					s.Contains(options, weapons.Longbow, "%s martial choice must include ranged weapons", choice.Label)
				}
			}
		}
	}
}

func (s *CategoryChoiceOptionsTestSuite) TestMonkSimpleWeaponOptionsIncludeRangedWeaponsAndKeepShortswordSeparate() {
	_, monkSimple := monkWeaponSimpleOption(s)
	s.Require().Len(monkSimple.CategoryChoices, 1)
	options := equipmentItemIDs(monkSimple.CategoryChoices[0].Options)

	s.Contains(options, weapons.Dart)
	s.Contains(options, weapons.LightCrossbow)
	s.Len(options, len(weapons.GetSimpleWeapons()),
		"Monk's any-simple-weapon option must track the complete equippable simple weapon registry")
	s.NotContains(options, weapons.Shortsword,
		"shortsword is the separate Monk starting-equipment alternative, not a simple-weapon option")

	reqs := GetClassRequirements(classes.Monk)
	var shortswordOption *EquipmentOption
	for _, req := range reqs.Equipment {
		if req.ID != MonkWeaponsPrimary {
			continue
		}
		for i := range req.Options {
			if req.Options[i].ID == MonkWeaponShortsword {
				shortswordOption = &req.Options[i]
			}
		}
	}
	s.Require().NotNil(shortswordOption)
	s.Require().Len(shortswordOption.Items, 1)
	s.Equal(weapons.Shortsword, shortswordOption.Items[0].ID)
}

func (s *CategoryChoiceOptionsTestSuite) TestSpecialWeaponsAreNeitherExpandedNorValid() {
	// The requirements builder populates options; this test uses a real Monk
	// choice so expansion and validation both exercise the production path.
	_, monkSimple := monkWeaponSimpleOption(s)
	choice := monkSimple.CategoryChoices[0]

	s.NotContains(equipmentItemIDs(choice.Options), weapons.UnarmedStrike)
	s.False(s.categorySelectionValid(choice, weapons.UnarmedStrike))
}

func (s *CategoryChoiceOptionsTestSuite) TestSubclassOptionsAreIndependentAcrossConcurrentRequirements() {
	const readers = 64
	var waitGroup sync.WaitGroup
	errors := make(chan string, readers)

	for range readers {
		waitGroup.Add(1)
		go func() {
			defer waitGroup.Done()
			requirements := GetClassRequirementsWithSubclass(classes.Cleric, 1, classes.WarDomain)
			for _, choice := range classCategoryChoices(requirements) {
				if choice.Label != "Choose a martial weapon" {
					continue
				}
				if len(choice.Options) == 0 {
					errors <- "War Domain martial options were not enriched"
				}
			}
		}()
	}
	waitGroup.Wait()
	close(errors)
	for err := range errors {
		s.Fail(err)
	}
}

func (s *CategoryChoiceOptionsTestSuite) TestOptionsAreDetailedDeterministicAndDuplicateFree() {
	for _, classID := range implementedClasses {
		first := classCategoryChoices(GetClassRequirements(classID))
		second := classCategoryChoices(GetClassRequirements(classID))
		s.Require().Len(second, len(first))

		for i, choice := range first {
			ids := make(map[shared.EquipmentID]struct{}, len(choice.Options))
			for _, item := range choice.Options {
				s.NotEmpty(item.ID)
				s.Equal(1, item.Quantity, "each category selection grants one item")
				s.NotNil(item.Detail, "option %q must have resolved detail", item.ID)
				if item.Detail != nil {
					s.NotEmpty(item.Detail.Name)
					if choice.Type == shared.EquipmentTypeWeapon {
						s.NotNil(item.Detail.Weapon, "weapon option %q must include weapon stats", item.ID)
						if item.Detail.Weapon != nil {
							s.NotEmpty(item.Detail.Weapon.Damage)
						}
					}
				}
				_, duplicate := ids[item.ID]
				s.False(duplicate, "option %q appears more than once", item.ID)
				ids[item.ID] = struct{}{}
			}
			s.Equal(choice.Options, second[i].Options, "options must be deterministic")
		}
	}
}

func (s *CategoryChoiceOptionsTestSuite) categorySelectionValid(choice EquipmentCategoryChoice, id shared.EquipmentID) bool {
	requirements := &Requirements{EquipmentCategories: []*EquipmentCategoryRequirement{{
		ID:         "category-choice-options-test",
		Choose:     1,
		Type:       choice.Type,
		Categories: choice.Categories,
	}}}
	submissions := NewSubmissions()
	submissions.Add(Submission{
		Category: shared.ChoiceEquipment,
		ChoiceID: "category-choice-options-test",
		Values:   []shared.SelectionID{id},
	})
	return s.validator.Validate(requirements, submissions).Valid
}

var implementedClasses = []classes.Class{
	classes.Barbarian,
	classes.Bard,
	classes.Cleric,
	classes.Druid,
	classes.Fighter,
	classes.Monk,
	classes.Paladin,
	classes.Ranger,
	classes.Rogue,
	classes.Sorcerer,
	classes.Warlock,
	classes.Wizard,
}

func monkWeaponSimpleOption(s *CategoryChoiceOptionsTestSuite) (*EquipmentRequirement, *EquipmentOption) {
	requirements := GetClassRequirements(classes.Monk)
	for _, requirement := range requirements.Equipment {
		if requirement.ID != MonkWeaponsPrimary {
			continue
		}
		for i := range requirement.Options {
			if requirement.Options[i].ID == MonkWeaponSimple {
				return requirement, &requirement.Options[i]
			}
		}
	}
	s.FailNow("MonkWeaponSimple should exist")
	return nil, nil
}

func classCategoryChoices(requirements *Requirements) []EquipmentCategoryChoice {
	var result []EquipmentCategoryChoice
	for _, requirement := range requirements.Equipment {
		for _, option := range requirement.Options {
			result = append(result, option.CategoryChoices...)
		}
	}
	return result
}

func allCategoryCandidates() []shared.EquipmentID {
	result := make([]shared.EquipmentID, 0, len(weapons.All)+len(armor.All)+len(tools.All)+len(items.All))
	for id := range weapons.All {
		result = append(result, id)
	}
	for id := range armor.All {
		result = append(result, id)
	}
	for id := range tools.All {
		result = append(result, id)
	}
	for id := range items.All {
		result = append(result, id)
	}
	return result
}

func equipmentItemIDs(items []EquipmentItem) map[shared.EquipmentID]struct{} {
	result := make(map[shared.EquipmentID]struct{}, len(items))
	for _, item := range items {
		result[item.ID] = struct{}{}
	}
	return result
}
