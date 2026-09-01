package character_test

import (
	"context"
	"encoding/json"
	"fmt"
	"sync"
	"testing"

	"github.com/stretchr/testify/suite"

	"github.com/KirkDiggler/rpg-toolkit/events"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/abilities"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/backgrounds"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/character"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/character/choices"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/classes"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/fightingstyles"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/languages"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/races"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/shared"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/skills"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/weapons"
)

// CategoryBasedEquipmentTestSuite tests the fix for issue #346
type CategoryBasedEquipmentTestSuite struct {
	suite.Suite
	ctx   context.Context
	bus   events.EventBus
	draft *character.Draft
}

func (s *CategoryBasedEquipmentTestSuite) SetupTest() {
	s.ctx = context.Background()
	s.bus = events.NewEventBus()
	s.draft = character.LoadDraftFromData(&character.DraftData{
		ID:       "draft-category-test",
		PlayerID: "player-test",
	})

	// Set up minimal character
	err := s.draft.SetName(&character.SetNameInput{Name: "Test Barbarian"})
	s.Require().NoError(err)

	err = s.draft.SetAbilityScores(&character.SetAbilityScoresInput{
		Scores: shared.AbilityScores{
			abilities.STR: 16,
			abilities.DEX: 14,
			abilities.CON: 15,
			abilities.INT: 10,
			abilities.WIS: 12,
			abilities.CHA: 8,
		},
		Method: "standard",
	})
	s.Require().NoError(err)

	err = s.draft.SetRace(&character.SetRaceInput{
		RaceID:    races.Human,
		SubraceID: "",
		Choices: character.RaceChoices{
			Languages: []languages.Language{languages.Orc},
		},
	})
	s.Require().NoError(err)

	err = s.draft.SetBackground(&character.SetBackgroundInput{
		BackgroundID: backgrounds.Outlander,
	})
	s.Require().NoError(err)
}

// TestBarbarianSecondaryWeaponCategoryChoice tests the bug scenario from issue #346
// Barbarian secondary weapon option B has CategoryChoices for "any simple weapon"
func (s *CategoryBasedEquipmentTestSuite) TestBarbarianSecondaryWeaponCategoryChoice() {
	// Set class with secondary weapon option B (any simple weapon)
	err := s.draft.SetClass(&character.SetClassInput{
		ClassID: classes.Barbarian,
		Choices: character.ClassChoices{
			Skills: []skills.Skill{skills.Athletics, skills.Intimidation},
			Equipment: []character.EquipmentChoiceSelection{
				{
					ChoiceID: choices.BarbarianWeaponsPrimary,
					OptionID: choices.BarbarianWeaponGreataxe,
				},
				{
					ChoiceID:           choices.BarbarianWeaponsSecondary,
					OptionID:           choices.BarbarianSecondarySimple,    // Option B with CategoryChoices
					CategorySelections: []shared.EquipmentID{weapons.Spear}, // Player chooses a spear
				},
				{
					ChoiceID: choices.BarbarianPack,
					OptionID: choices.BarbarianPackExplorer,
				},
			},
		},
	})
	s.Require().NoError(err, "Setting Barbarian class with category weapon choice should succeed")

	// Verify choices were recorded
	draftChoices := s.draft.Choices()
	s.Require().NotEmpty(draftChoices, "Should have recorded choices")

	// Find the secondary weapon choice
	var secondaryWeaponChoice *choices.ChoiceData
	for _, choice := range draftChoices {
		if choice.ChoiceID == choices.BarbarianWeaponsSecondary {
			secondaryWeaponChoice = &choice
			break
		}
	}

	s.Require().NotNil(secondaryWeaponChoice, "Should have recorded secondary weapon choice")
	s.Assert().Equal(shared.ChoiceEquipment, secondaryWeaponChoice.Category)
	s.Assert().Equal(shared.SourceClass, secondaryWeaponChoice.Source)
	s.Assert().Equal(choices.BarbarianSecondarySimple, secondaryWeaponChoice.OptionID)
	s.Require().NotEmpty(secondaryWeaponChoice.EquipmentSelection, "Should have equipment selection")
	s.Assert().Contains(secondaryWeaponChoice.EquipmentSelection, weapons.Spear,
		"Should have the selected spear in equipment selection")
}

func (s *CategoryBasedEquipmentTestSuite) TestFighterTwoMartialRejectsRepeatedIneligibleSelection() {
	err := s.draft.SetClass(fighterTwoMartialClassInput(weapons.Club, weapons.Club))

	s.Require().Error(err)
	s.ErrorContains(err, "Invalid equipment choice 'club'")
}

func (s *CategoryBasedEquipmentTestSuite) TestFighterTwoMartialRejectsWrongSelectionCount() {
	err := s.draft.SetClass(fighterTwoMartialClassInput(weapons.Longsword))

	s.Require().Error(err)
	s.ErrorContains(err, "option 'fighter-weapon-b' requires 2 category selections, got 1")
}

func (s *CategoryBasedEquipmentTestSuite) TestPersistedFighterRepeatedSelectionsSurviveJSONRoundTrip() {
	draft := s.loadPersistedFighterCategoryDraft(weapons.Longsword, weapons.Longsword)

	var persistedSelection []shared.SelectionID
	for _, choice := range draft.Choices() {
		if choice.ChoiceID == choices.FighterWeaponsPrimary {
			persistedSelection = choice.EquipmentSelection
			break
		}
	}
	// Exact equality kills a set/map-style persistence regression that collapses
	// repeated IDs before LoadDraftFromData can restore the draft.
	s.Require().Equal([]shared.SelectionID{weapons.Longsword, weapons.Longsword}, persistedSelection)

	char, err := draft.ToCharacter(s.ctx, "persisted-fighter-two-longswords", s.bus)
	s.Require().NoError(err)
	s.Require().NotNil(char)

	matching := make([]character.InventoryItemData, 0, 1)
	for _, item := range char.ToData().Inventory {
		if item.ID == weapons.Longsword {
			matching = append(matching, item)
		}
	}
	// Row count and quantity together reject both failure modes: leaving two
	// duplicate rows or silently dropping one persisted copy.
	s.Require().Len(matching, 1)
	s.Equal(2, matching[0].Quantity)
}

func (s *CategoryBasedEquipmentTestSuite) TestPersistedFighterTwoMartialRejectsWrongSelectionCount() {
	draft := s.loadPersistedFighterCategoryDraft(weapons.Longsword)

	// Exercising both entry points catches either removal of persisted count
	// validation or a finalization path that stops revalidating loaded drafts.
	err := draft.ValidateChoices()
	s.Require().Error(err)
	s.ErrorContains(err, "expected 2 items, got 1")

	char, err := draft.ToCharacter(s.ctx, "persisted-fighter-wrong-count", s.bus)
	s.Require().Error(err, "finalization must revalidate the persisted selection count")
	s.ErrorContains(err, "expected 2 items, got 1")
	s.Nil(char)
}

func (s *CategoryBasedEquipmentTestSuite) TestMonkCategoryChoiceRejectsEverySpecialWeaponViaSetClass() {
	requirements := choices.GetClassRequirements(classes.Monk)
	_, monkSimple := monkWeaponSimpleOptionFromRequirements(requirements)
	advertised := make(map[shared.EquipmentID]struct{}, len(monkSimple.CategoryChoices[0].Options))
	for _, item := range monkSimple.CategoryChoices[0].Options {
		advertised[item.ID] = struct{}{}
	}

	for specialID := range weapons.SpecialWeapons {
		s.Run(string(specialID), func() {
			_, offered := advertised[specialID]
			s.False(offered, "special weapons must never be advertised as category choices")

			draft := character.LoadDraftFromData(&character.DraftData{
				ID:       "draft-monk-special-" + string(specialID),
				PlayerID: "player-test",
			})
			err := draft.SetClass(monkClassInput(specialID))

			s.Require().Error(err)
			s.ErrorContains(err, "Invalid equipment choice '"+string(specialID)+"'")
			for _, choice := range draft.Choices() {
				s.NotContains(choice.EquipmentSelection, specialID,
					"rejected category equipment must not be recorded in the draft")
			}
		})
	}
}

func (s *CategoryBasedEquipmentTestSuite) TestMonkCategoryChoiceAppliesExactlyAdvertisedWeaponOptions() {
	requirements := choices.GetClassRequirements(classes.Monk)
	_, monkSimple := monkWeaponSimpleOptionFromRequirements(requirements)
	advertised := make(map[shared.EquipmentID]struct{}, len(monkSimple.CategoryChoices[0].Options))
	for _, item := range monkSimple.CategoryChoices[0].Options {
		advertised[item.ID] = struct{}{}
	}

	for weaponID := range weapons.All {
		s.Run(string(weaponID), func() {
			_, offered := advertised[weaponID]
			draft := character.LoadDraftFromData(&character.DraftData{
				ID:       "draft-monk-" + string(weaponID),
				PlayerID: "player-test",
			})
			err := draft.SetClass(monkClassInput(weaponID))

			if offered {
				s.Require().NoError(err, "advertised equipment must be accepted through Draft.SetClass")
				return
			}
			s.Require().Error(err, "unadvertised equipment must be rejected through Draft.SetClass")
		})
	}
}

func (s *CategoryBasedEquipmentTestSuite) TestMonkCategoryChoiceAppliesAdvertisedOptionsConcurrently() {
	_, monkSimple := monkWeaponSimpleOptionFromRequirements(choices.GetClassRequirements(classes.Monk))
	options := monkSimple.CategoryChoices[0].Options
	const workers = 64

	errors := make(chan string, workers)
	var waitGroup sync.WaitGroup
	for worker := range workers {
		waitGroup.Add(1)
		go func() {
			defer waitGroup.Done()
			item := options[worker%len(options)]
			draft := character.LoadDraftFromData(&character.DraftData{
				ID:       fmt.Sprintf("draft-monk-concurrent-%d", worker),
				PlayerID: "player-test",
			})
			if err := draft.SetClass(monkClassInput(item.ID)); err != nil {
				errors <- fmt.Sprintf("%s: %v", item.ID, err)
			}
		}()
	}
	waitGroup.Wait()
	close(errors)
	for err := range errors {
		s.Fail(err)
	}
}

func (s *CategoryBasedEquipmentTestSuite) TestPersistedMonkCategoryChoiceRevalidatesNestedSelection() {
	invalidDraft := s.loadPersistedMonkCategoryDraft(weapons.UnarmedStrike)

	err := invalidDraft.ValidateChoices()
	s.Error(err, "persisted unarmed strike must fail final choice validation")
	s.ErrorContains(err, "monk-weapons-primary")
	s.ErrorContains(err, "monk-weapon-b")
	s.ErrorContains(err, "unarmed-strike")

	char, err := invalidDraft.ToCharacter(s.ctx, "persisted-monk-invalid", s.bus)
	s.Error(err, "persisted unarmed strike must not finalize into a character")
	s.Nil(char)

	for _, test := range []struct {
		name     string
		weaponID shared.EquipmentID
	}{
		{name: "valid simple melee club", weaponID: weapons.Club},
		{name: "valid simple ranged shortbow", weaponID: weapons.Shortbow},
	} {
		s.Run(test.name, func() {
			draft := s.loadPersistedMonkCategoryDraft(test.weaponID)

			s.Require().NoError(draft.ValidateChoices())
			char, err := draft.ToCharacter(s.ctx, "persisted-monk-"+string(test.weaponID), s.bus)
			s.Require().NoError(err)
			s.Require().NotNil(char)

			found := false
			for _, item := range char.ToData().Inventory {
				if item.ID == test.weaponID {
					found = true
					break
				}
			}
			s.True(found, "finalized inventory must retain the valid persisted choice %q", test.weaponID)
		})
	}
}

// loadPersistedMonkCategoryDraft round-trips the exact serialized shape that
// origin/main accepted for a Monk's nested category selection before this PR.
func (s *CategoryBasedEquipmentTestSuite) loadPersistedMonkCategoryDraft(weaponID shared.EquipmentID) *character.Draft {
	const baseCreatedMonkDraftJSON = `{
		"id": "base-persisted-monk",
		"player_id": "player",
		"name": "Persisted Monk",
		"race": "human",
		"class": "monk",
		"background": "hermit",
		"base_ability_scores": {
			"cha": 12,
			"con": 13,
			"dex": 14,
			"int": 8,
			"str": 10,
			"wis": 15
		},
		"choices": [
			{"category": "name", "source": "player", "name": "Persisted Monk"},
			{"category": "ability_scores", "source": "player", "ability_scores": {"cha": 12, "con": 13, "dex": 14, "int": 8, "str": 10, "wis": 15}},
			{"category": "languages", "source": "race", "choice_id": "human-language", "languages": ["elvish"]},
			{"category": "skills", "source": "class", "choice_id": "monk-skills", "skills": ["acrobatics", "stealth"]},
			{"category": "tool_proficiency", "source": "class", "choice_id": "monk-tools", "tools": ["brewer-supplies"]},
			{"category": "equipment", "source": "class", "choice_id": "monk-weapons-primary", "option_id": "monk-weapon-b", "equipment": ["unarmed-strike"]},
			{"category": "equipment", "source": "class", "choice_id": "monk-pack", "option_id": "monk-pack-b", "equipment": ["explorer-pack"]}
		],
		"progress": 31
	}`

	var data character.DraftData
	s.Require().NoError(json.Unmarshal([]byte(baseCreatedMonkDraftJSON), &data))
	for i := range data.Choices {
		choice := &data.Choices[i]
		if choice.ChoiceID == choices.MonkWeaponsPrimary {
			choice.EquipmentSelection = []shared.SelectionID{weaponID}
			break
		}
	}

	serialized, err := json.Marshal(data)
	s.Require().NoError(err)
	s.Require().NoError(json.Unmarshal(serialized, &data))
	return character.LoadDraftFromData(&data)
}

// loadPersistedFighterCategoryDraft creates a valid Fighter draft, substitutes
// the requested persisted selection shape, and exercises the JSON storage seam.
func (s *CategoryBasedEquipmentTestSuite) loadPersistedFighterCategoryDraft(
	weaponIDs ...shared.EquipmentID,
) *character.Draft {
	s.Require().NoError(s.draft.SetClass(fighterTwoMartialClassInput(
		weapons.Longsword,
		weapons.Greatsword,
	)))

	data := s.draft.ToData()
	found := false
	for i := range data.Choices {
		choice := &data.Choices[i]
		if choice.ChoiceID != choices.FighterWeaponsPrimary {
			continue
		}

		choice.EquipmentSelection = make([]shared.SelectionID, len(weaponIDs))
		copy(choice.EquipmentSelection, weaponIDs)
		found = true
		break
	}
	s.Require().True(found, "fixture must contain the Fighter primary weapon choice")

	serialized, err := json.Marshal(data)
	s.Require().NoError(err)
	var persisted character.DraftData
	s.Require().NoError(json.Unmarshal(serialized, &persisted))
	return character.LoadDraftFromData(&persisted)
}

func fighterTwoMartialClassInput(weaponIDs ...shared.EquipmentID) *character.SetClassInput {
	return &character.SetClassInput{
		ClassID: classes.Fighter,
		Choices: character.ClassChoices{
			Skills: []skills.Skill{skills.Athletics, skills.Intimidation},
			Equipment: []character.EquipmentChoiceSelection{
				{ChoiceID: choices.FighterArmor, OptionID: choices.FighterArmorChainMail},
				{
					ChoiceID:           choices.FighterWeaponsPrimary,
					OptionID:           choices.FighterWeaponTwoMartial,
					CategorySelections: weaponIDs,
				},
				{ChoiceID: choices.FighterWeaponsSecondary, OptionID: choices.FighterRangedCrossbow},
				{ChoiceID: choices.FighterPack, OptionID: choices.FighterPackDungeoneer},
			},
			FightingStyle: fightingstyles.Defense,
		},
	}
}

func monkClassInput(weaponID shared.EquipmentID) *character.SetClassInput {
	return &character.SetClassInput{
		ClassID: classes.Monk,
		Choices: character.ClassChoices{
			Skills: []skills.Skill{skills.Acrobatics, skills.Athletics},
			Equipment: []character.EquipmentChoiceSelection{
				{
					ChoiceID:           choices.MonkWeaponsPrimary,
					OptionID:           choices.MonkWeaponSimple,
					CategorySelections: []shared.EquipmentID{weaponID},
				},
				{
					ChoiceID: choices.MonkPack,
					OptionID: choices.MonkPackExplorer,
				},
			},
		},
	}
}

func monkWeaponSimpleOptionFromRequirements(requirements *choices.Requirements) (*choices.EquipmentRequirement, *choices.EquipmentOption) {
	for _, requirement := range requirements.Equipment {
		if requirement.ID != choices.MonkWeaponsPrimary {
			continue
		}
		for i := range requirement.Options {
			if requirement.Options[i].ID == choices.MonkWeaponSimple {
				return requirement, &requirement.Options[i]
			}
		}
	}
	panic("MonkWeaponSimple should exist")
}

func TestCategoryBasedEquipmentTestSuite(t *testing.T) {
	suite.Run(t, new(CategoryBasedEquipmentTestSuite))
}
