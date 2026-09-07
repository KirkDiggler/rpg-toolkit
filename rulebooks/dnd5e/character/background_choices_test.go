package character_test

import (
	"context"
	"testing"

	"github.com/stretchr/testify/suite"

	"github.com/KirkDiggler/rpg-toolkit/events"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/abilities"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/backgrounds"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/character"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/character/choices"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/classes"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/currency"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/languages"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/proficiencies"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/races"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/shared"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/skills"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/tools"
)

// BackgroundChoicesSuite covers the seven real background choices
// (rpg-toolkit#1554 PR 3): Outlander, Noble/Knight, Criminal/Spy
// (proficiency-only), Entertainer/Folk Hero/Guild Artisan (equipment with
// derived proficiency), Soldier (both, independent), and Charlatan
// (equipment, no tied proficiency).
type BackgroundChoicesSuite struct {
	suite.Suite
	ctx context.Context
	bus events.EventBus
}

func TestBackgroundChoicesSuite(t *testing.T) {
	suite.Run(t, new(BackgroundChoicesSuite))
}

func (s *BackgroundChoicesSuite) SetupTest() {
	s.ctx = context.Background()
	s.bus = events.NewEventBus()
}

// newCompleteDraft returns a draft with name/race/class/ability scores
// set, ready for the test to set its own background and finalize.
func (s *BackgroundChoicesSuite) newCompleteDraft(id string) *character.Draft {
	draft := character.LoadDraftFromData(&character.DraftData{ID: id, PlayerID: "player-1"})
	s.Require().NoError(draft.SetName(&character.SetNameInput{Name: "Test"}))
	s.Require().NoError(draft.SetRace(&character.SetRaceInput{
		RaceID: races.Human,
		Choices: character.RaceChoices{
			Languages: []languages.Language{languages.Elvish},
		},
	}))
	s.Require().NoError(draft.SetClass(&character.SetClassInput{
		ClassID: classes.Fighter,
		Choices: character.ClassChoices{
			Skills:        []skills.Skill{skills.Athletics, skills.Perception},
			FightingStyle: "defense",
			Equipment: []character.EquipmentChoiceSelection{
				{ChoiceID: choices.FighterArmor, OptionID: choices.FighterArmorLeather},
				{
					ChoiceID:           choices.FighterWeaponsPrimary,
					OptionID:           choices.FighterWeaponMartialShield,
					CategorySelections: []shared.EquipmentID{"longsword"},
				},
				{ChoiceID: choices.FighterWeaponsSecondary, OptionID: choices.FighterRangedCrossbow},
				{ChoiceID: choices.FighterPack, OptionID: choices.FighterPackExplorer},
			},
		},
	}))
	s.Require().NoError(draft.SetAbilityScores(&character.SetAbilityScoresInput{
		Scores: shared.AbilityScores{
			abilities.STR: 15, abilities.DEX: 14, abilities.CON: 13,
			abilities.INT: 12, abilities.WIS: 10, abilities.CHA: 8,
		},
	}))
	return draft
}

// TestOutlanderInstrumentProficiency covers the proficiency-only shape:
// no physical item, just a Tools choice.
func (s *BackgroundChoicesSuite) TestOutlanderInstrumentProficiency() {
	draft := s.newCompleteDraft("outlander-choice")
	s.Require().NoError(draft.SetBackground(&character.SetBackgroundInput{
		BackgroundID: backgrounds.Outlander,
		Choices:      character.BackgroundChoices{Tools: []shared.SelectionID{"lute"}},
	}))

	char, err := draft.ToCharacter(s.ctx, "outlander-char", s.bus)
	s.Require().NoError(err)
	s.Contains(char.ToData().ToolProficiencies, proficiencies.ToolLute)
}

// TestNobleGamingSetProficiency covers Noble/Knight's proficiency-only choice.
func (s *BackgroundChoicesSuite) TestNobleGamingSetProficiency() {
	draft := s.newCompleteDraft("noble-choice")
	s.Require().NoError(draft.SetBackground(&character.SetBackgroundInput{
		BackgroundID: backgrounds.Noble,
		Choices:      character.BackgroundChoices{Tools: []shared.SelectionID{"dragonchess-set"}},
	}))

	char, err := draft.ToCharacter(s.ctx, "noble-char", s.bus)
	s.Require().NoError(err)
	s.Contains(char.ToData().ToolProficiencies, proficiencies.ToolDragonchessSet)
}

// TestCriminalGamingSetProficiency covers Criminal/Spy's proficiency-only
// choice — the one this wave corrected from a hardcoded, wrong fixed
// grant (ToolPlayingCardSet) to a real choice.
func (s *BackgroundChoicesSuite) TestCriminalGamingSetProficiency() {
	draft := s.newCompleteDraft("criminal-choice")
	s.Require().NoError(draft.SetBackground(&character.SetBackgroundInput{
		BackgroundID: backgrounds.Criminal,
		Choices:      character.BackgroundChoices{Tools: []shared.SelectionID{"three-dragon-ante-set"}},
	}))

	char, err := draft.ToCharacter(s.ctx, "criminal-char", s.bus)
	s.Require().NoError(err)
	s.Contains(char.ToData().ToolProficiencies, proficiencies.ToolThreeDragonAnte)
	s.Contains(char.ToData().ToolProficiencies, proficiencies.ToolThieves, "Criminal's fixed thieves' tools grant is untouched")
}

// TestEntertainerInstrumentDerivesProficiency covers the equipment+derived-
// proficiency shape: one choice produces both the owned item and the tied
// proficiency, with no separate Tools submission.
func (s *BackgroundChoicesSuite) TestEntertainerInstrumentDerivesProficiency() {
	draft := s.newCompleteDraft("entertainer-choice")
	s.Require().NoError(draft.SetBackground(&character.SetBackgroundInput{
		BackgroundID: backgrounds.Entertainer,
		Choices: character.BackgroundChoices{
			Equipment: []character.EquipmentChoiceSelection{
				{
					ChoiceID:           choices.EntertainerInstrument,
					OptionID:           choices.EntertainerInstrumentChoice,
					CategorySelections: []shared.EquipmentID{"flute"},
				},
			},
		},
	}))

	char, err := draft.ToCharacter(s.ctx, "entertainer-char", s.bus)
	s.Require().NoError(err)
	data := char.ToData()
	s.assertInventoryContainsID(data.Inventory, "flute")
	s.Contains(data.ToolProficiencies, proficiencies.ToolFlute,
		"proficiency must be derived from the equipment choice, not asked separately")
}

// TestFolkHeroArtisanToolsDerivesProficiency is the explicit derivation
// test the plan calls for: picking smith-tools for Folk Hero's equipment
// choice must produce ToolSmith with no separate Tools choice submitted.
func (s *BackgroundChoicesSuite) TestFolkHeroArtisanToolsDerivesProficiency() {
	draft := s.newCompleteDraft("folk-hero-choice")
	s.Require().NoError(draft.SetBackground(&character.SetBackgroundInput{
		BackgroundID: backgrounds.FolkHero,
		Choices: character.BackgroundChoices{
			Equipment: []character.EquipmentChoiceSelection{
				{
					ChoiceID:           choices.FolkHeroArtisanTools,
					OptionID:           choices.FolkHeroToolsChoice,
					CategorySelections: []shared.EquipmentID{"smith-tools"},
				},
			},
			// Deliberately no Tools choice submitted.
		},
	}))

	char, err := draft.ToCharacter(s.ctx, "folk-hero-char", s.bus)
	s.Require().NoError(err)
	data := char.ToData()
	s.assertInventoryContainsID(data.Inventory, "smith-tools")
	s.Contains(data.ToolProficiencies, proficiencies.ToolSmith,
		"proficiency must be derived from the equipment choice, not asked separately")
}

// TestGuildArtisanToolsDerivesProficiency covers the third derived-
// proficiency background.
func (s *BackgroundChoicesSuite) TestGuildArtisanToolsDerivesProficiency() {
	draft := s.newCompleteDraft("guild-artisan-choice")
	s.Require().NoError(draft.SetBackground(&character.SetBackgroundInput{
		BackgroundID: backgrounds.GuildArtisan,
		Choices: character.BackgroundChoices{
			Equipment: []character.EquipmentChoiceSelection{
				{
					ChoiceID:           choices.GuildArtisanTools,
					OptionID:           choices.GuildArtisanToolsChoice,
					CategorySelections: []shared.EquipmentID{"carpenter-tools"},
				},
			},
		},
	}))

	char, err := draft.ToCharacter(s.ctx, "guild-artisan-char", s.bus)
	s.Require().NoError(err)
	data := char.ToData()
	s.assertInventoryContainsID(data.Inventory, "carpenter-tools")
	s.Contains(data.ToolProficiencies, proficiencies.ToolCarpenter)
}

// TestSoldierItemAndProficiencyAreIndependent is the one case RAW forces
// to stay independent: the physical-item choice (bone dice) and the
// proficiency choice (dragonchess set) deliberately disagree, proving
// they're two real, separate choices rather than one derived from the
// other.
func (s *BackgroundChoicesSuite) TestSoldierItemAndProficiencyAreIndependent() {
	draft := s.newCompleteDraft("soldier-choice")
	s.Require().NoError(draft.SetBackground(&character.SetBackgroundInput{
		BackgroundID: backgrounds.Soldier,
		Choices: character.BackgroundChoices{
			Tools: []shared.SelectionID{"dragonchess-set"},
			Equipment: []character.EquipmentChoiceSelection{
				{ChoiceID: choices.SoldierGamingSetItem, OptionID: choices.SoldierGamingSetDice},
			},
		},
	}))

	char, err := draft.ToCharacter(s.ctx, "soldier-char", s.bus)
	s.Require().NoError(err)
	data := char.ToData()
	s.assertInventoryContainsID(data.Inventory, string(tools.DiceSet))
	s.Contains(data.ToolProficiencies, proficiencies.ToolDragonchessSet,
		"proficiency choice is independent of which physical item was picked")
	s.NotContains(data.ToolProficiencies, proficiencies.ToolDiceSet,
		"the physical item chosen must not itself imply a matching proficiency for Soldier")
}

// TestCharlatanToolsOfTheCon covers the plain 4-option (reduced to 3
// modeled options — see GetBackgroundRequirements' doc comment) Equipment
// requirement with no tied proficiency.
func (s *BackgroundChoicesSuite) TestCharlatanToolsOfTheCon() {
	draft := s.newCompleteDraft("charlatan-choice")
	s.Require().NoError(draft.SetBackground(&character.SetBackgroundInput{
		BackgroundID: backgrounds.Charlatan,
		Choices: character.BackgroundChoices{
			Equipment: []character.EquipmentChoiceSelection{
				{ChoiceID: choices.CharlatanToolsOfTheCon, OptionID: choices.CharlatanConSignetRing},
			},
		},
	}))

	char, err := draft.ToCharacter(s.ctx, "charlatan-char", s.bus)
	s.Require().NoError(err)
	data := char.ToData()
	s.assertInventoryContainsID(data.Inventory, "signet-ring")
}

// TestIsBackgroundCompleteFalseUntilChoiceMade confirms background
// completeness is a real, structural gate now (rpg-toolkit#1555's
// per-source validation), not the old stub that returned true once
// background != "".
func (s *BackgroundChoicesSuite) TestIsBackgroundCompleteFalseUntilChoiceMade() {
	draft := s.newCompleteDraft("completeness-check")
	s.Require().NoError(draft.SetBackground(&character.SetBackgroundInput{
		BackgroundID: backgrounds.Outlander,
	}))
	s.False(draft.IsBackgroundComplete(), "Outlander's instrument choice hasn't been made yet")

	s.Require().NoError(draft.SetBackground(&character.SetBackgroundInput{
		BackgroundID: backgrounds.Outlander,
		Choices:      character.BackgroundChoices{Tools: []shared.SelectionID{"lute"}},
	}))
	s.True(draft.IsBackgroundComplete(), "the choice has now been made")
}

// TestUnknownBackgroundIDRejected covers SetBackground's own
// long-deferred validation fix.
func (s *BackgroundChoicesSuite) TestUnknownBackgroundIDRejected() {
	draft := s.newCompleteDraft("unknown-background")
	err := draft.SetBackground(&character.SetBackgroundInput{
		BackgroundID: backgrounds.Background("not-a-real-background"),
	})
	s.Error(err)
}

// TestMissingBackgroundChoiceFailsFinalization confirms a background
// requiring a choice cannot be finalized without one — background
// completeness structurally cannot gate finalization was rpg-toolkit#1555's
// bug; this is the background-choice-specific proof it's fixed.
func (s *BackgroundChoicesSuite) TestMissingBackgroundChoiceFailsFinalization() {
	draft := s.newCompleteDraft("missing-choice")
	s.Require().NoError(draft.SetBackground(&character.SetBackgroundInput{
		BackgroundID: backgrounds.Charlatan,
	}))

	_, err := draft.ToCharacter(s.ctx, "missing-choice-char", s.bus)
	s.Error(err, "Charlatan's tools-of-the-con choice was never made")
}

// TestInvalidToolIDRejected confirms a tool ID outside the requirement's
// options is rejected, not silently accepted.
func (s *BackgroundChoicesSuite) TestInvalidToolIDRejected() {
	draft := s.newCompleteDraft("invalid-tool")
	err := draft.SetBackground(&character.SetBackgroundInput{
		BackgroundID: backgrounds.Outlander,
		Choices:      character.BackgroundChoices{Tools: []shared.SelectionID{"not-a-real-instrument"}},
	})
	s.Require().NoError(err, "SetBackground itself succeeds; validity is checked at finalization")

	_, err = draft.ToCharacter(s.ctx, "invalid-tool-char", s.bus)
	s.Error(err)
}

// TestChangingCompleteBackgroundClearsOldChoice confirms a previously-
// complete background's choice doesn't leak into a newly-selected
// background that has no such requirement.
func (s *BackgroundChoicesSuite) TestChangingCompleteBackgroundClearsOldChoice() {
	draft := s.newCompleteDraft("background-change")
	s.Require().NoError(draft.SetBackground(&character.SetBackgroundInput{
		BackgroundID: backgrounds.Outlander,
		Choices:      character.BackgroundChoices{Tools: []shared.SelectionID{"lute"}},
	}))
	s.True(draft.IsBackgroundComplete())

	s.Require().NoError(draft.SetBackground(&character.SetBackgroundInput{
		BackgroundID: backgrounds.Sage,
	}))
	s.True(draft.IsBackgroundComplete(), "Sage requires no choice at all")

	char, err := draft.ToCharacter(s.ctx, "background-change-char", s.bus)
	s.Require().NoError(err)
	s.NotContains(char.ToData().ToolProficiencies, proficiencies.ToolLute,
		"Outlander's old choice must not survive the background change")
}

// TestFullPublicPathRoundTrip exercises the whole path the plan's
// acceptance criteria call for: select race/class/background, serialize
// and reload the draft, finalize, serialize and reload the character, and
// verify skills, expertise, tools, inventory quantities, and wallet all
// survive.
func (s *BackgroundChoicesSuite) TestFullPublicPathRoundTrip() {
	draft := character.LoadDraftFromData(&character.DraftData{ID: "full-path", PlayerID: "player-1"})
	s.Require().NoError(draft.SetName(&character.SetNameInput{Name: "Full Path"}))
	s.Require().NoError(draft.SetRace(&character.SetRaceInput{
		RaceID: races.HalfElf,
		Choices: character.RaceChoices{
			Languages: []languages.Language{languages.Elvish},
			Skills:    []skills.Skill{skills.Insight, skills.Perception},
		},
	}))
	s.Require().NoError(draft.SetClass(&character.SetClassInput{
		ClassID: classes.Rogue,
		Choices: character.ClassChoices{
			Skills: []skills.Skill{skills.Stealth, skills.SleightOfHand, skills.Acrobatics, skills.Deception},
			Expertise: []skills.Skill{
				skills.Stealth,
				skills.Insight, // proficient only via the Half-Elf race choice above
			},
			Equipment: []character.EquipmentChoiceSelection{
				{ChoiceID: choices.RogueWeaponsPrimary, OptionID: choices.RogueWeaponRapier},
				{ChoiceID: choices.RogueWeaponsSecondary, OptionID: choices.RogueSecondaryShortbow},
				{ChoiceID: choices.RoguePack, OptionID: choices.RoguePackBurglar},
			},
		},
	}))
	s.Require().NoError(draft.SetBackground(&character.SetBackgroundInput{
		BackgroundID: backgrounds.FolkHero,
		Choices: character.BackgroundChoices{
			Equipment: []character.EquipmentChoiceSelection{
				{
					ChoiceID:           choices.FolkHeroArtisanTools,
					OptionID:           choices.FolkHeroToolsChoice,
					CategorySelections: []shared.EquipmentID{"weaver-tools"},
				},
			},
		},
	}))
	s.Require().NoError(draft.SetAbilityScores(&character.SetAbilityScoresInput{
		Scores: shared.AbilityScores{
			abilities.STR: 10, abilities.DEX: 16, abilities.CON: 13,
			abilities.INT: 12, abilities.WIS: 14, abilities.CHA: 10,
		},
	}))

	// Serialize and reload the draft before finalizing.
	reloadedDraft := character.LoadDraftFromData(draft.ToData())

	char, err := reloadedDraft.ToCharacter(s.ctx, "full-path-char", s.bus)
	s.Require().NoError(err)

	// Serialize and reload the character.
	data := char.ToData()
	reloaded, err := character.Load(s.ctx, data)
	s.Require().NoError(err)
	reloadedData := reloaded.ToData()

	s.Equal(shared.Expert, reloadedData.Skills[skills.Stealth], "class expertise survives")
	s.Equal(shared.Expert, reloadedData.Skills[skills.Insight], "race-granted expertise survives")
	s.Contains(reloadedData.ToolProficiencies, proficiencies.ToolWeaver, "derived background proficiency survives")
	s.assertInventoryContainsID(reloadedData.Inventory, "weaver-tools")
	s.Equal(currency.FromGold(10), reloadedData.Wallet, "Folk Hero's starting gold survives")
}

// assertInventoryContainsID is a small local helper — this suite lives in
// package character_test and doesn't have access to DraftTestSuite's
// unexported assertInventoryStack.
func (s *BackgroundChoicesSuite) assertInventoryContainsID(inventory []character.InventoryItemData, id string) {
	for _, item := range inventory {
		if item.ID == id {
			return
		}
	}
	s.Failf("item not found in inventory", "expected %q in inventory", id)
}
