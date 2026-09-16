// Copyright (C) 2026 Kirk Diggler
// SPDX-License-Identifier: GPL-3.0-or-later

package character

import (
	"context"
	"testing"

	"github.com/stretchr/testify/suite"

	"github.com/KirkDiggler/rpg-toolkit/events"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/abilities"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/backgrounds"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/character/choices"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/classes"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/fightingstyles"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/languages"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/races"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/refs"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/shared"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/skills"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/weapons"
)

// FightingStyleCreationSuite pins that a class's fighting style is answered
// under that class's own requirement id.
//
// The walk finding: the submission builder completeness reads carried
// `ChoiceID: choices.FighterFightingStyle` for EVERY class, under the comment
// "Would need mapping for other classes". A ranger's requirement is
// "ranger-fighting-style", so its answered requirement was never seen,
// IsClassComplete was false forever, and FinalizeDraft refused with
// "missing: [class selection or class choices] (progress: 80%)". A ranger could
// not be created by any client with any choices.
//
// Table-driven over every class with a level-1 fighting style rather than
// ranger alone, so the next class given one is covered the day its row lands.
type FightingStyleCreationSuite struct {
	suite.Suite

	ctx context.Context
}

func (s *FightingStyleCreationSuite) SetupTest() { s.ctx = context.Background() }

func TestFightingStyleCreationSuite(t *testing.T) {
	suite.Run(t, new(FightingStyleCreationSuite))
}

// classesWithALevelOneFightingStyle asks the requirement rows rather than
// naming classes, so this cannot go stale against the table.
func (s *FightingStyleCreationSuite) classesWithALevelOneFightingStyle() []classes.Class {
	found := make([]classes.Class, 0)
	for classID := range classes.ClassData {
		if choices.GetClassRequirements(classID).FightingStyle != nil {
			found = append(found, classID)
		}
	}
	return found
}

func (s *FightingStyleCreationSuite) TestEveryClassWithAFightingStyleCanBeCreated() {
	found := s.classesWithALevelOneFightingStyle()
	s.Require().ElementsMatch([]classes.Class{classes.Fighter, classes.Ranger}, found,
		"fighter and ranger are the two classes that pick a style at level 1")

	for _, classID := range found {
		s.Run(string(classID), func() {
			draft := s.draftFor(classID)

			s.Require().True(draft.IsClassComplete(),
				"%s answered its fighting style and must read as complete", classID)
			s.Require().NoError(draft.ValidateChoices())

			char, err := draft.ToCharacter(s.ctx, string(classID)+"-1", events.NewEventBus())
			s.Require().NoError(err)

			// Archery for both, so the assertion is about the style reaching
			// the sheet rather than about which style each class picked.
			s.True(hasCondition(char, refs.Conditions.FightingStyleArchery().ID),
				"%s carries its chosen fighting style", classID)
		})
	}
}

// TestTheSubmissionCarriesTheClasssOwnRequirementID is the claim underneath the
// one above, stated directly: a ranger's answer is filed under
// "ranger-fighting-style", never under the fighter's constant.
func (s *FightingStyleCreationSuite) TestTheSubmissionCarriesTheClasssOwnRequirementID() {
	draft := s.draftFor(classes.Ranger)

	subs := draft.getClassSubmissions()

	styles := subs.GetByCategory(shared.ChoiceFightingStyle)
	s.Require().Len(styles, 1)
	s.Equal(choices.RangerFightingStyle, styles[0].ChoiceID)
	s.NotEqual(choices.FighterFightingStyle, styles[0].ChoiceID)
	s.Equal(choices.GetClassRequirements(classes.Ranger).FightingStyle.ID, styles[0].ChoiceID,
		"the id belongs to the requirement, and the answer quotes it back")
}

// TestAStoredChoiceWithNoIDFallsBackToTheClassRow — a draft persisted before
// the fighting style carried its own id still completes, from the class's row
// rather than from a per-class map.
func (s *FightingStyleCreationSuite) TestAStoredChoiceWithNoIDFallsBackToTheClassRow() {
	draft := s.draftFor(classes.Ranger)
	for i := range draft.choices {
		if draft.choices[i].Category == shared.ChoiceFightingStyle {
			draft.choices[i].ChoiceID = ""
		}
	}

	styles := draft.getClassSubmissions().GetByCategory(shared.ChoiceFightingStyle)
	s.Require().Len(styles, 1)
	s.Equal(choices.RangerFightingStyle, styles[0].ChoiceID)
	s.True(draft.IsClassComplete())
}

// draftFor builds a complete level-1 draft for a class that picks a fighting
// style, with Archery chosen.
func (s *FightingStyleCreationSuite) draftFor(classID classes.Class) *Draft {
	if classID == classes.Fighter {
		draft := newFighterDraft(s.T())
		// The shared fixture takes Defense; this suite is about Archery
		// reaching the sheet for both classes through one assertion.
		s.Require().NoError(draft.SetClass(&SetClassInput{
			ClassID: classes.Fighter,
			Choices: ClassChoices{
				Skills: []skills.Skill{skills.Athletics, skills.History},
				Equipment: []EquipmentChoiceSelection{
					{ChoiceID: choices.FighterArmor, OptionID: choices.FighterArmorChainMail},
					{
						ChoiceID:           choices.FighterWeaponsPrimary,
						OptionID:           choices.FighterWeaponMartialShield,
						CategorySelections: []shared.EquipmentID{weapons.Longsword},
					},
					{ChoiceID: choices.FighterWeaponsSecondary, OptionID: choices.FighterRangedCrossbow},
					{ChoiceID: choices.FighterPack, OptionID: choices.FighterPackDungeoneer},
				},
				FightingStyle: fightingstyles.Archery,
			},
		}))
		return draft
	}

	draft, err := NewDraft(&DraftConfig{ID: string(classID) + "-draft", PlayerID: "player-1"})
	s.Require().NoError(err)
	s.Require().NoError(draft.SetName(&SetNameInput{Name: "Vex"}))
	s.Require().NoError(draft.SetRace(&SetRaceInput{
		RaceID:  races.Human,
		Choices: RaceChoices{Languages: []languages.Language{languages.Elvish}},
	}))
	s.Require().NoError(draft.SetClass(&SetClassInput{
		ClassID: classes.Ranger,
		Choices: ClassChoices{
			Skills: []skills.Skill{skills.AnimalHandling, skills.Athletics, skills.Insight},
			Equipment: []EquipmentChoiceSelection{
				{ChoiceID: choices.RangerArmor, OptionID: choices.RangerArmorScale},
				{ChoiceID: choices.RangerWeaponsPrimary, OptionID: choices.RangerWeaponShortswords},
				{ChoiceID: choices.RangerPack, OptionID: choices.RangerPackExplorer},
			},
			FightingStyle: fightingstyles.Archery,
		},
	}))
	s.Require().NoError(draft.SetBackground(&SetBackgroundInput{BackgroundID: backgrounds.Hermit}))
	s.Require().NoError(draft.SetAbilityScores(&SetAbilityScoresInput{
		Scores: shared.AbilityScores{
			abilities.STR: 12, abilities.DEX: 15, abilities.CON: 13,
			abilities.INT: 10, abilities.WIS: 14, abilities.CHA: 8,
		},
		Method: "standard-array",
	}))
	return draft
}

// hasCondition reports whether the sheet carries a condition with this ref id.
func hasCondition(char *Character, id string) bool {
	for _, condition := range char.conditions {
		if ref := condition.Ref(); ref != nil && ref.ID == id {
			return true
		}
	}
	return false
}
