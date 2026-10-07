// Copyright (C) 2026 Kirk Diggler
// SPDX-License-Identifier: GPL-3.0-or-later

package character

import (
	"context"
	"encoding/json"
	"strings"
	"testing"

	"github.com/stretchr/testify/suite"

	"github.com/KirkDiggler/rpg-toolkit/events"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/abilities"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/classes"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/combat"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/conditions"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/contributions"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/damage"
	dnd5eEvents "github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/events"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/features"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/refs"
)

// SheetFactsTestSuite pins that class-scaled numbers are asked of the sheet
// at the moment they are used: advancing a level rewrites nothing below the
// sheet, and the next use reads the new level.
type SheetFactsTestSuite struct {
	suite.Suite
	ctx context.Context
	bus events.EventBus
}

func TestSheetFactsSuite(t *testing.T) {
	suite.Run(t, new(SheetFactsTestSuite))
}

func (s *SheetFactsTestSuite) SetupTest() {
	s.ctx = context.Background()
	s.bus = events.NewEventBus()
}

// finalize finalizes a draft onto the suite's bus with the experience for any
// level these tests take.
func (s *SheetFactsTestSuite) finalize(draft *Draft, id string) *Character {
	char, err := draft.ToCharacter(s.ctx, id, s.bus)
	s.Require().NoError(err)
	char.experience = ExperienceThresholdForLevel(MaxCharacterLevel)
	return char
}

// advance takes count more levels in class.
func (s *SheetFactsTestSuite) advance(char *Character, class classes.Class, count int) {
	for range count {
		_, err := char.Advance(s.ctx, &AdvanceInput{ClassID: class, HitPointMethod: HitPointMethodAverage})
		s.Require().NoError(err)
	}
}

// daggerFrame is the frame resolution builds for char's finesse dagger attack
// with advantage on a goblin: its class levels asked of the sheet itself.
func daggerFrame(char *Character) contributions.Frame {
	return contributions.Frame{
		Actor:            char.GetID(),
		ActorClassLevels: char.ClassLevels(),
		Target:           contributions.Known("goblin"),
		Action: contributions.ActionFacts{
			Roll:         contributions.Known(contributions.RollKindAttack),
			Ability:      contributions.Known(abilities.DEX),
			Melee:        contributions.Known(true),
			WeaponPool:   contributions.Known(true),
			Advantage:    contributions.Known(true),
			Weapon:       contributions.Known(refs.Weapons.Dagger().String()),
			Finesse:      contributions.Known(true),
			RangedWeapon: contributions.Known(false),
		},
		Complete: true,
	}
}

// levelThreeRogue is a rogue finalized at level 1, advanced to 2, and given
// its third rogue level in the record. Advance refuses rogue 3 today — it
// requires a subclass choice advancement cannot apply — so the third entry is
// written to the saved record, which is exactly what a sheet advanced by a
// later Advance holds. The Sneak Attack blob is the one level 1 granted;
// rewrite may replace it before the sheet loads.
func (s *SheetFactsTestSuite) levelThreeRogue(rewrite func(raw json.RawMessage) json.RawMessage) *Character {
	char := s.finalize(newRogueDraft(s.T()), "advancing-rogue")
	s.advance(char, classes.Rogue, 1)
	data := mustToData(s.T(), char)
	s.Require().NoError(char.Cleanup(s.ctx))
	data.Levels = append(data.Levels, LevelEntry{Level: 3, ClassID: classes.Rogue, HitPointMethod: HitPointMethodAverage})
	data.Level = 3
	for i, raw := range data.Conditions {
		if strings.Contains(string(raw), refs.Features.SneakAttack().String()) && rewrite != nil {
			data.Conditions[i] = rewrite(raw)
		}
	}
	s.bus = events.NewEventBus()
	loaded, err := LoadFromData(s.ctx, data, s.bus)
	s.Require().NoError(err)
	s.Require().Equal(3, loaded.ClassLevel(classes.Rogue))
	return loaded
}

// sneakDice folds char's dagger hit and returns the Sneak Attack faces.
func (s *SheetFactsTestSuite) sneakDice(char *Character, frame contributions.Frame) []int {
	roller := &scriptedD10{face: 4}
	for _, condition := range char.GetConditions() {
		if binder, ok := condition.(conditions.RollerBinder); ok {
			binder.BindRoller(roller)
		}
	}
	event := &dnd5eEvents.DamageChainEvent{
		AttackerID: char.GetID(), TargetID: "goblin", Frame: frame,
		WeaponDamageType: damage.Piercing,
		Components: []dnd5eEvents.DamageComponent{{
			Source:     dnd5eEvents.DamageSourceWeapon,
			Properties: []damage.Property{damage.AddsAttackAbilityModifier},
			DamageType: damage.Piercing,
		}},
	}
	chain := events.NewStagedChain[*dnd5eEvents.DamageChainEvent](combat.ModifierStages)
	modified, err := dnd5eEvents.DamageChain.On(s.bus).PublishWithChain(s.ctx, event, chain)
	s.Require().NoError(err)
	folded, err := modified.Execute(s.ctx, event)
	s.Require().NoError(err)

	var faces []int
	for _, component := range folded.Components {
		if component.Roll.Source.Ref != nil && component.Roll.Source.Ref.Equals(refs.Features.SneakAttack()) {
			faces = component.Roll.Dice.FinalRolls
		}
	}
	return faces
}

// TestARogueAtLevelThreeRollsTwoDice: the Sneak Attack granted at level 1 is
// not rescaled when levels are taken. At level 3 the information row reads
// +2d6 and the fold rolls 2d6, both from the one rule reading the sheet's
// rogue levels.
func (s *SheetFactsTestSuite) TestARogueAtLevelThreeRollsTwoDice() {
	char := s.levelThreeRogue(nil)

	frame := daggerFrame(char)
	listed, err := conditions.AssessActionEffects(&conditions.AssessActionEffectsInput{
		Conditions: char.GetConditions(), Frame: frame,
	})
	s.Require().NoError(err)
	var row *contributions.Effect
	for i := range listed.Effects {
		if listed.Effects[i].ID == refs.Features.SneakAttack().String() {
			row = &listed.Effects[i]
		}
	}
	s.Require().NotNil(row, "the rogue's Sneak Attack lists a row")
	s.Equal(contributions.StateApplies, row.State)
	s.Equal("+2d6 damage", row.Benefit)

	s.Equal([]int{4, 4}, s.sneakDice(char, frame), "a level-3 rogue rolls 2d6")
}

// TestALevelThreeRogueSavedWithOneDieRollsTwo: a level-3 rogue sheet saved
// when Sneak Attack stored a level and a one-die count loads; the copy is
// ignored and the rogue rolls 2d6 from its level record.
func (s *SheetFactsTestSuite) TestALevelThreeRogueSavedWithOneDieRollsTwo() {
	char := s.levelThreeRogue(func(raw json.RawMessage) json.RawMessage {
		var fields map[string]json.RawMessage
		s.Require().NoError(json.Unmarshal(raw, &fields))
		fields["level"] = json.RawMessage(`1`)
		fields["damage_dice"] = json.RawMessage(`1`)
		old, err := json.Marshal(fields)
		s.Require().NoError(err)
		return old
	})

	s.Equal([]int{4, 4}, s.sneakDice(char, daggerFrame(char)), "the saved one die is ignored")
}

// TestAFighterAdvancedToTwoHealsOneD10PlusTwo: Second Wind, granted at level
// 1, asks its owner's fighter level when it is used, so after advancing it
// heals 1d10 + 2 with nothing rewritten.
func (s *SheetFactsTestSuite) TestAFighterAdvancedToTwoHealsOneD10PlusTwo() {
	char := s.finalize(newFighterDraft(s.T()), "advancing-fighter")
	var healed []int
	_, err := dnd5eEvents.HealingReceivedTopic.On(s.bus).Subscribe(s.ctx,
		func(_ context.Context, event dnd5eEvents.HealingReceivedEvent) error {
			healed = append(healed, event.Amount)
			return nil
		})
	s.Require().NoError(err)

	var secondWind features.Feature
	for _, feature := range char.GetFeatures() {
		if feature.Ref().Equals(refs.Features.SecondWind()) {
			secondWind = feature
		}
	}
	s.Require().NotNil(secondWind)

	s.advance(char, classes.Fighter, 1)
	s.Require().NoError(secondWind.Activate(s.ctx, char, features.FeatureInput{Bus: s.bus, Roller: &scriptedD10{face: 6}}))

	s.Equal([]int{6 + 2}, healed, "the level-1 grant heals with the level-2 fighter's level")
}

// TestNoWrittenEffectCarriesALevel: no condition or feature JSON a finalized
// and advanced sheet writes carries a class level or a number derived from
// one — a dice count, a damage bonus, a stored ability modifier.
func (s *SheetFactsTestSuite) TestNoWrittenEffectCarriesALevel() {
	for name, sheet := range map[string]struct {
		draft *Draft
		class classes.Class
	}{
		"barbarian": {newBarbarianDraft(s.T()), classes.Barbarian},
		"fighter":   {newFighterDraft(s.T()), classes.Fighter},
		"monk":      {newMonkDraft(s.T()), classes.Monk},
		"rogue":     {newRogueDraft(s.T()), classes.Rogue},
	} {
		s.Run(name, func() {
			s.SetupTest()
			char := s.finalize(sheet.draft, name)
			s.advance(char, sheet.class, 1)
			data := mustToData(s.T(), char)
			s.Require().NotEmpty(append(data.Conditions, data.Features...), "the sheet writes effects to check")
			for _, raw := range append(data.Conditions, data.Features...) {
				var fields map[string]json.RawMessage
				s.Require().NoError(json.Unmarshal(raw, &fields))
				for key := range fields {
					for _, copied := range []string{"level", "dice", "bonus", "modifier"} {
						s.False(strings.Contains(key, copied), "%s writes %q in %s", name, key, raw)
					}
				}
			}
		})
	}
}

// TestClassLevelsAnswersFromTheLevelRecord: the frame fact a character
// answers is one entry per class taken, counted from the record, and known.
func (s *SheetFactsTestSuite) TestClassLevelsAnswersFromTheLevelRecord() {
	char := s.levelThreeRogue(nil)

	levels, known := char.ClassLevels().Get()

	s.True(known)
	s.Equal([]contributions.ClassLevel{{Class: classes.Rogue, Levels: 3}}, levels)
	s.NoError(char.ClassLevels().Validate())
}

// TestACharacterWithNoStatedRangeSeesTheStatedDefault: no race table states a
// sight range, so a character answers the rulebook's stated 120 feet — even a
// dwarf, whose darkvision is a dim-light rule awaiting the light model, not a
// range.
func (s *SheetFactsTestSuite) TestACharacterWithNoStatedRangeSeesTheStatedDefault() {
	char := s.finalize(newRogueDraft(s.T()), "sighted-rogue")

	s.Equal(120, char.SightFeet())
}
