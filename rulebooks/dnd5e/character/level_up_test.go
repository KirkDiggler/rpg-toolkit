// Copyright (C) 2026 Kirk Diggler
// SPDX-License-Identifier: GPL-3.0-or-later

package character

import (
	"context"
	"testing"

	"github.com/stretchr/testify/suite"

	"github.com/KirkDiggler/rpg-toolkit/core"
	coreResources "github.com/KirkDiggler/rpg-toolkit/core/resources"
	"github.com/KirkDiggler/rpg-toolkit/events"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/abilities"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/backgrounds"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/character/choices"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/classes"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/combat"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/languages"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/races"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/refs"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/resources"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/shared"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/skills"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/spells"
)

// LevelUpSuite is the level-up system's done-when (rpg-project level-up design
// §7): a class's per-level requirements and progression are data, a character
// holds experience and takes the level it has earned, and a level's choices
// reach the sheet rather than only the record.
type LevelUpSuite struct {
	suite.Suite

	ctx context.Context
	bus events.EventBus
}

func (s *LevelUpSuite) SetupTest() {
	s.ctx = context.Background()
	s.bus = events.NewEventBus()
}

func TestLevelUpSuite(t *testing.T) {
	suite.Run(t, new(LevelUpSuite))
}

// --- fixtures -------------------------------------------------------------

// bard finalizes a level-1 bard holding exactly the experience for level 2.
func (s *LevelUpSuite) bard() *Character {
	draft, err := NewDraft(&DraftConfig{ID: "bard-draft", PlayerID: "player-1"})
	s.Require().NoError(err)
	s.Require().NoError(draft.SetName(&SetNameInput{Name: "Scanlan"}))
	s.Require().NoError(draft.SetRace(&SetRaceInput{
		RaceID:  races.Human,
		Choices: RaceChoices{Languages: []languages.Language{languages.Dwarvish}},
	}))
	s.Require().NoError(draft.SetClass(&SetClassInput{
		ClassID: classes.Bard,
		Choices: ClassChoices{
			Skills:   []skills.Skill{skills.Performance, skills.Persuasion, skills.Deception},
			Tools:    []shared.SelectionID{"lute", "flute", "drum"},
			Cantrips: []shared.SelectionID{spells.TrueStrike, spells.ViciousMockery},
			Spells: []spells.Spell{
				spells.Bane, spells.Thunderwave, spells.DissonantWhispers, spells.Command,
			},
			Equipment: []EquipmentChoiceSelection{
				{ChoiceID: choices.BardWeaponsPrimary, OptionID: choices.BardWeaponRapier},
				{ChoiceID: choices.BardPack, OptionID: choices.BardPackDiplomat},
				{ChoiceID: choices.BardInstrument, OptionID: choices.BardInstrumentLute},
			},
		},
	}))
	s.Require().NoError(draft.SetBackground(&SetBackgroundInput{BackgroundID: backgrounds.Hermit}))
	s.Require().NoError(draft.SetAbilityScores(&SetAbilityScoresInput{
		Scores: shared.AbilityScores{
			abilities.STR: 8, abilities.DEX: 14, abilities.CON: 13,
			abilities.INT: 10, abilities.WIS: 12, abilities.CHA: 16,
		},
		Method: "standard-array",
	}))

	char, err := draft.ToCharacter(s.ctx, "levelling-bard", s.bus)
	s.Require().NoError(err)
	char.experience = ExperienceThresholdForLevel(2)
	return char
}

// cleric finalizes a level-1 Life Domain cleric with the experience for 2.
func (s *LevelUpSuite) cleric() *Character {
	draft, err := NewDraft(&DraftConfig{ID: "cleric-draft", PlayerID: "player-1"})
	s.Require().NoError(err)
	s.Require().NoError(draft.SetName(&SetNameInput{Name: "Mercy"}))
	s.Require().NoError(draft.SetRace(&SetRaceInput{
		RaceID:  races.Human,
		Choices: RaceChoices{Languages: []languages.Language{languages.Dwarvish}},
	}))
	s.Require().NoError(draft.SetClass(&SetClassInput{
		ClassID: classes.Cleric, SubclassID: classes.LifeDomain,
		Choices: ClassChoices{
			Skills:   []skills.Skill{skills.Medicine, skills.Religion},
			Cantrips: []spells.Spell{spells.SacredFlame, spells.Guidance, spells.Light},
			Spells: []spells.Spell{
				spells.Bane, spells.Bless, spells.Command, spells.CureWounds, spells.HealingWord, spells.Sanctuary,
			},
			Equipment: []EquipmentChoiceSelection{
				{ChoiceID: choices.ClericWeapons, OptionID: choices.ClericWeaponMace},
				{ChoiceID: choices.ClericArmor, OptionID: choices.ClericArmorChainMail},
				{ChoiceID: choices.ClericSecondaryWeapon, OptionID: choices.ClericSecondaryShortbow},
				{ChoiceID: choices.ClericPack, OptionID: choices.ClericPackExplorer},
				{ChoiceID: choices.ClericHolySymbol, OptionID: choices.ClericHolyAmulet},
			},
		},
	}))
	s.Require().NoError(draft.SetBackground(&SetBackgroundInput{BackgroundID: backgrounds.Hermit}))
	s.Require().NoError(draft.SetAbilityScores(&SetAbilityScoresInput{
		Scores: shared.AbilityScores{
			abilities.STR: 14, abilities.DEX: 10, abilities.CON: 13,
			abilities.INT: 8, abilities.WIS: 15, abilities.CHA: 12,
		},
		Method: "standard-array",
	}))

	char, err := draft.ToCharacter(s.ctx, "levelling-cleric", s.bus)
	s.Require().NoError(err)
	char.experience = ExperienceThresholdForLevel(2)
	return char
}

// wizard finalizes a level-1 wizard with the experience for 2.
func (s *LevelUpSuite) wizard() *Character {
	draft, err := NewDraft(&DraftConfig{ID: "wizard-draft", PlayerID: "player-1"})
	s.Require().NoError(err)
	s.Require().NoError(draft.SetName(&SetNameInput{Name: "Nott"}))
	s.Require().NoError(draft.SetRace(&SetRaceInput{
		RaceID:  races.Human,
		Choices: RaceChoices{Languages: []languages.Language{languages.Dwarvish}},
	}))
	s.Require().NoError(draft.SetClass(&SetClassInput{
		ClassID: classes.Wizard,
		Choices: ClassChoices{
			Skills:   []skills.Skill{skills.Arcana, skills.History},
			Cantrips: []spells.Spell{spells.FireBolt, spells.MageHand, spells.Light},
			Spells: []spells.Spell{
				spells.MagicMissile, spells.BurningHands, spells.ChromaticOrb,
				spells.Shield, spells.Sleep, spells.CharmPerson,
			},
			Equipment: []EquipmentChoiceSelection{
				{ChoiceID: choices.WizardWeaponsPrimary, OptionID: choices.WizardWeaponQuarterstaff},
				{ChoiceID: choices.WizardFocus, OptionID: choices.WizardFocusComponent},
				{ChoiceID: choices.WizardPack, OptionID: choices.WizardPackScholar},
			},
		},
	}))
	s.Require().NoError(draft.SetBackground(&SetBackgroundInput{BackgroundID: backgrounds.Hermit}))
	s.Require().NoError(draft.SetAbilityScores(&SetAbilityScoresInput{
		Scores: shared.AbilityScores{
			abilities.STR: 8, abilities.DEX: 14, abilities.CON: 13,
			abilities.INT: 16, abilities.WIS: 12, abilities.CHA: 10,
		},
		Method: "standard-array",
	}))

	char, err := draft.ToCharacter(s.ctx, "levelling-wizard", s.bus)
	s.Require().NoError(err)
	char.experience = ExperienceThresholdForLevel(2)
	return char
}

// spellChoice is a level's answer to a derived spell question.
func spellChoice(id choices.ChoiceID, chosen ...spells.Spell) choices.ChoiceData {
	return choices.ChoiceData{
		Category:       shared.ChoiceSpells,
		Source:         shared.SourceClass,
		ChoiceID:       id,
		SpellSelection: chosen,
	}
}

// knownSpellIDs names this character's known spells in order.
func knownSpellIDs(char *Character) []string {
	out := make([]string, 0, len(char.KnownSpells()))
	for _, ref := range char.KnownSpells() {
		out = append(out, ref.ID)
	}
	return out
}

// resourceChange finds one pool's reported movement.
func resourceChange(out *AdvanceOutput, key coreResources.ResourceKey) (ResourceChange, bool) {
	for _, change := range out.Gained.Resources {
		if change.Key == key {
			return change, true
		}
	}
	return ResourceChange{}, false
}

// --- experience and entitlement (R4.8 - R4.12a) ---------------------------

func (s *LevelUpSuite) TestAFreshCharacterHoldsNoExperienceAndIsEntitledToOne() {
	char := s.bard()
	char.experience = 0

	s.Equal(0, char.Experience())
	s.Equal(1, char.EntitledLevel(), "existing entitles a character to its first level")
	s.Equal(300, char.NextLevelThreshold())
}

func (s *LevelUpSuite) TestEntitlementIsTheHighestLevelTheTotalHasReached() {
	for _, tc := range []struct {
		experience int
		entitled   int
		next       int
	}{
		{experience: 0, entitled: 1, next: 300},
		{experience: 299, entitled: 1, next: 300},
		{experience: 300, entitled: 2, next: 900},
		{experience: 899, entitled: 2, next: 900},
		{experience: 900, entitled: 3, next: 2700},
		{experience: 354999, entitled: 19, next: 355000},
		{experience: 355000, entitled: 20, next: 0},
		{experience: 900000, entitled: 20, next: 0},
	} {
		s.Run("", func() {
			s.Equal(tc.entitled, EntitledLevelForExperience(tc.experience),
				"%d experience", tc.experience)
			s.Equal(tc.next, NextExperienceThreshold(tc.experience),
				"%d experience", tc.experience)
		})
	}
}

func (s *LevelUpSuite) TestAdvanceRefusesALevelTheCharacterHasNotEarned() {
	char := s.bard()
	char.experience = 299

	out, err := char.Advance(s.ctx, &AdvanceInput{
		ClassID:        classes.Bard,
		HitPointMethod: HitPointMethodAverage,
		Choices:        []choices.ChoiceData{spellChoice("bard-spells-2", spells.HealingWord)},
	})

	s.Require().Error(err)
	s.Nil(out)
	s.ErrorContains(err, "299 experience")
	s.ErrorContains(err, "300", "the refusal names the threshold the level needs")
	s.Equal(1, char.GetLevel(), "the character is still level 1")
}

func (s *LevelUpSuite) TestExperienceSurvivesTheRecordRoundTrip() {
	char := s.bard()
	char.experience = 1234

	data := char.ToData()
	s.Equal(1234, data.Experience, "ToData is the field-by-field surface")

	reloaded, err := Load(s.ctx, data)
	s.Require().NoError(err)
	s.Equal(1234, reloaded.Experience())
	s.Equal(3, reloaded.EntitledLevel())
}

// TestLoadDoesNotRecheckEntitlement pins R4.12a. Advance enforces the rule at
// the moment a level is taken; a later change to the threshold table must not
// refuse every stored sheet that was legal when it was written.
func (s *LevelUpSuite) TestLoadDoesNotRecheckEntitlement() {
	char := s.bard()
	out, err := char.Advance(s.ctx, &AdvanceInput{
		ClassID:        classes.Bard,
		HitPointMethod: HitPointMethodAverage,
		Choices:        []choices.ChoiceData{spellChoice("bard-spells-2", spells.HealingWord)},
	})
	s.Require().NoError(err)
	s.Require().Equal(2, out.Gained.CharacterLevel)

	data := char.ToData()
	data.Experience = 0

	reloaded, err := Load(s.ctx, data)
	s.Require().NoError(err, "a level-2 sheet with no experience still loads")
	s.Equal(2, reloaded.GetLevel())
	s.Equal(1, reloaded.EntitledLevel(), "and reads as entitled to less than it holds")
}

// --- the bard proof case (R4.4c, R4.6, R4.7) ------------------------------

func (s *LevelUpSuite) TestBardLevelTwoAsksForExactlyOneSpell() {
	reqs := choices.GetClassRequirementsGainedAtLevel(classes.Bard, 2)

	s.Require().NotNil(reqs.Spellbook, "spells known goes four to five at level 2")
	s.Equal(choices.ChoiceID("bard-spells-2"), reqs.Spellbook.ID)
	s.Equal(1, reqs.Spellbook.Count, "five known minus four known")
	s.Equal(1, reqs.Spellbook.SpellLevel, "the highest slot level a level-2 bard has")
	s.Equal(spells.Castable(reqs.Spellbook.Options), reqs.Spellbook.Options,
		"every option must compile to a cast")

	s.Nil(reqs.Cantrips, "a bard's cantrips known does not move until level 4")
	s.Equal([]choices.ChoiceID{"bard-spells-2"}, reqs.ChoiceIDs(),
		"the spell is the only thing level 2 asks a bard for")
}

func (s *LevelUpSuite) TestABardTakesLevelTwoAndKnowsTheSpellItChose() {
	char := s.bard()
	s.Require().Len(char.KnownSpells(), 4)
	s.Require().Equal(2, char.GetResource(resources.SpellSlotLevel1).Maximum())

	out, err := char.Advance(s.ctx, &AdvanceInput{
		ClassID:        classes.Bard,
		HitPointMethod: HitPointMethodAverage,
		Choices:        []choices.ChoiceData{spellChoice("bard-spells-2", spells.HealingWord)},
	})
	s.Require().NoError(err)

	// The spell is ON THE SHEET, not only in the record. Before this it reached
	// the append-only entry and nowhere else.
	known := make([]string, 0, len(char.KnownSpells()))
	for _, ref := range char.KnownSpells() {
		known = append(known, ref.ID)
	}
	s.Contains(known, "healing-word")
	s.Len(char.KnownSpells(), 5, "four chosen at creation and one at level 2")

	// The slots the table gives a level-2 bard, applied without asking.
	s.Equal(3, char.GetResource(resources.SpellSlotLevel1).Maximum())

	// The record holds both levels and the second one holds its input.
	s.Require().Len(char.Levels(), 2)
	s.Equal([]choices.ChoiceData{spellChoice("bard-spells-2", spells.HealingWord)},
		char.Levels()[1].Choices)

	// And the response says what moved, so a confirmation screen can (R4.7).
	slots, ok := resourceChange(out, resources.SpellSlotLevel1)
	s.Require().True(ok, "the slot pool's movement is reported")
	s.Equal(2, slots.From)
	s.Equal(3, slots.To)

	dice, ok := resourceChange(out, resources.HitDice)
	s.Require().True(ok)
	s.Equal(1, dice.From)
	s.Equal(2, dice.To)
}

func (s *LevelUpSuite) TestABardIsRefusedTwoSpellsWhereOneWasAsked() {
	char := s.bard()

	out, err := char.Advance(s.ctx, &AdvanceInput{
		ClassID:        classes.Bard,
		HitPointMethod: HitPointMethodAverage,
		Choices: []choices.ChoiceData{
			spellChoice("bard-spells-2", spells.HealingWord, spells.Bless),
		},
	})

	s.Require().Error(err)
	s.Nil(out)
	s.Equal(1, char.GetLevel())
	s.Len(char.KnownSpells(), 4, "nothing reached the sheet")
}

func (s *LevelUpSuite) TestABardIsRefusedASpellThatIsNotOnItsList() {
	char := s.bard()

	// Sacred Flame has a cast profile and is not a bard spell.
	out, err := char.Advance(s.ctx, &AdvanceInput{
		ClassID:        classes.Bard,
		HitPointMethod: HitPointMethodAverage,
		Choices:        []choices.ChoiceData{spellChoice("bard-spells-2", spells.SacredFlame)},
	})

	s.Require().Error(err)
	s.Nil(out)
	s.ErrorContains(err, "sacred-flame")
	s.Equal(1, char.GetLevel())
	s.Len(char.KnownSpells(), 4)
}

// TestABardIsRefusedTheRightIdInTheWrongCategory is why presence is not
// validation: the id matches the requirement exactly and the answer is a skill.
func (s *LevelUpSuite) TestABardIsRefusedTheRightIdInTheWrongCategory() {
	char := s.bard()

	out, err := char.Advance(s.ctx, &AdvanceInput{
		ClassID:        classes.Bard,
		HitPointMethod: HitPointMethodAverage,
		Choices: []choices.ChoiceData{{
			Category:       shared.ChoiceSkills,
			Source:         shared.SourceClass,
			ChoiceID:       "bard-spells-2",
			SkillSelection: []skills.Skill{skills.Stealth},
		}},
	})

	s.Require().Error(err)
	s.Nil(out)
	s.Equal(1, char.GetLevel())
}

func (s *LevelUpSuite) TestABardIsRefusedAChoiceTheLevelDidNotAskFor() {
	char := s.bard()

	out, err := char.Advance(s.ctx, &AdvanceInput{
		ClassID:        classes.Bard,
		HitPointMethod: HitPointMethodAverage,
		Choices: []choices.ChoiceData{
			spellChoice("bard-spells-2", spells.HealingWord),
			spellChoice("bard-spells-9", spells.Bless),
		},
	})

	s.Require().Error(err)
	s.Nil(out)
	s.ErrorContains(err, "bard-spells-9")
	s.Equal(1, char.GetLevel())
}

// --- a level never teaches what is already known (walk finding) -----------

// TestABardIsOfferedOnlyTheSpellItDoesNotKnow is the walk finding. Scanlan is
// created with four of the five spells his level-2 row offers, and the class
// function cannot know that — it answers for a class at a level. A screen
// driven by the class answer would offer him Bane, which he already has.
func (s *LevelUpSuite) TestABardIsOfferedOnlyTheSpellItDoesNotKnow() {
	char := s.bard()

	classAnswer := choices.GetClassRequirementsGainedAtLevel(classes.Bard, 2)
	s.Require().NotNil(classAnswer.Spellbook)
	s.Len(classAnswer.Spellbook.Options, 5,
		"the class row offers all five, because a class has no character")

	offered := char.NextLevelRequirements()
	s.Require().NotNil(offered.Spellbook)
	s.Equal([]spells.Spell{spells.HealingWord}, offered.Spellbook.Options,
		"the one spell this bard does not already know")
	s.Equal(1, offered.Spellbook.Count, "the count is the class's rule, not this bard's")
	s.Equal(choices.ChoiceID("bard-spells-2"), offered.Spellbook.ID)
}

// TestTheClassRowIsUnchangedByTheCharactersView — creation reads the class
// function, and a filtered row leaking back into it would quietly shrink what
// a new bard is offered at level 1.
func (s *LevelUpSuite) TestTheClassRowIsUnchangedByTheCharactersView() {
	char := s.bard()
	_ = char.NextLevelRequirements()

	again := choices.GetClassRequirementsGainedAtLevel(classes.Bard, 2)
	s.Require().NotNil(again.Spellbook)
	s.Len(again.Spellbook.Options, 5)
}

// TestACantripAlreadyKnownIsNotOffered — the rule covers both lists. A bard
// gains its third cantrip at level 4, and this one already has two of the four
// this build can cast.
func (s *LevelUpSuite) TestACantripAlreadyKnownIsNotOffered() {
	char := &Character{
		id:      "bard-at-three",
		classID: classes.Bard,
		levels: []LevelEntry{
			{Level: 1, ClassID: classes.Bard, HitPointMethod: HitPointMethodMax},
			{Level: 2, ClassID: classes.Bard, HitPointMethod: HitPointMethodAverage},
			{Level: 3, ClassID: classes.Bard, HitPointMethod: HitPointMethodAverage},
		},
		knownCantrips: []*core.Ref{refs.Spells.TrueStrike(), refs.Spells.ViciousMockery()},
	}

	offered := char.NextLevelRequirements()

	s.Require().NotNil(offered.Cantrips, "a bard's third cantrip arrives at level 4")
	s.Equal(1, offered.Cantrips.Count)
	s.NotContains(offered.Cantrips.Options, spells.TrueStrike)
	s.NotContains(offered.Cantrips.Options, spells.ViciousMockery)
	s.NotEmpty(offered.Cantrips.Options, "and the ones it does not know are still there")
}

// TestABardIsRefusedASpellItAlreadyKnows is the reproduction, exactly as it was
// found: a bard knowing [bane thunderwave dissonant-whispers command] takes
// level 2 and answers bard-spells-2 with bane.
//
// It used to return NO ERROR and leave the known list as
// [bane thunderwave dissonant-whispers command bane], with the duplicate choice
// written into a record that can never be corrected. The screen offering the
// spell was the visible half; this was the half that reached the sheet.
func (s *LevelUpSuite) TestABardIsRefusedASpellItAlreadyKnows() {
	char := s.bard()
	s.Require().Equal([]string{"bane", "thunderwave", "dissonant-whispers", "command"},
		knownSpellIDs(char), "the sheet the reproduction starts from")

	out, err := char.Advance(s.ctx, &AdvanceInput{
		ClassID:        classes.Bard,
		HitPointMethod: HitPointMethodAverage,
		Choices:        []choices.ChoiceData{spellChoice("bard-spells-2", spells.Bane)},
	})

	s.Require().Error(err)
	s.Nil(out)
	s.ErrorContains(err, "already knows")
	s.ErrorContains(err, "bane")

	// Nothing moved: not the list, not the level, not the append-only record.
	s.Equal([]string{"bane", "thunderwave", "dissonant-whispers", "command"},
		knownSpellIDs(char))
	s.Equal(1, char.GetLevel())
	s.Len(char.Levels(), 1, "no entry was appended")
}

// TestAStoredSheetHoldingASpellTwiceIsRefused — a duplicate on a persisted
// sheet is a corrupted record, and load refuses it rather than repairing it.
//
// Silently collapsing the list to a set would leave a sheet that reads correct
// beside a record that produced a wrong one, and the record is append-only so
// there is nothing to repair it to.
func (s *LevelUpSuite) TestAStoredSheetHoldingASpellTwiceIsRefused() {
	char := s.bard()
	data := char.ToData()
	s.Require().NoError(func() error { _, err := Load(s.ctx, data); return err }(),
		"the sheet loads before it is corrupted")

	for _, tc := range []struct {
		name    string
		corrupt func(*Data)
	}{
		{"a known spell twice", func(d *Data) {
			d.KnownSpells = append(d.KnownSpells, d.KnownSpells[0])
		}},
		{"a known cantrip twice", func(d *Data) {
			d.KnownCantrips = append(d.KnownCantrips, d.KnownCantrips[0])
		}},
	} {
		s.Run(tc.name, func() {
			corrupted := char.ToData()
			tc.corrupt(corrupted)

			loaded, err := Load(s.ctx, corrupted)

			s.Require().Error(err)
			s.Nil(loaded)
			s.ErrorContains(err, "appears twice")
		})
	}
}

// TestTheKnownListHoldsNoDuplicatesAfterALevel — the positive half: the one
// spell this bard did not know lands, and the list stays five DISTINCT spells.
func (s *LevelUpSuite) TestTheKnownListHoldsNoDuplicatesAfterALevel() {
	char := s.bard()

	_, err := char.Advance(s.ctx, &AdvanceInput{
		ClassID:        classes.Bard,
		HitPointMethod: HitPointMethodAverage,
		Choices:        []choices.ChoiceData{spellChoice("bard-spells-2", spells.HealingWord)},
	})
	s.Require().NoError(err)

	seen := make(map[string]int)
	for _, ref := range char.KnownSpells() {
		seen[ref.ID]++
	}
	s.Len(seen, 5, "five distinct spells")
	for id, count := range seen {
		s.Equal(1, count, "%s appears once", id)
	}
	s.Contains(seen, "healing-word")
}

// --- the classes that ask nothing (R4.14) ---------------------------------

func (s *LevelUpSuite) TestTheMartialClassesTakeLevelTwoWithNoQuestion() {
	for _, tc := range []struct {
		name  string
		class classes.Class
		draft func(*testing.T) *Draft
	}{
		{"fighter", classes.Fighter, newFighterDraft},
		{"barbarian", classes.Barbarian, newBarbarianDraft},
		{"monk", classes.Monk, newMonkDraft},
		{"rogue", classes.Rogue, newRogueDraft},
	} {
		s.Run(tc.name, func() {
			s.Empty(choices.GetClassChoiceIDsGainedAtLevel(tc.class, 2),
				"%s level 2 asks for nothing", tc.name)

			char, err := tc.draft(s.T()).ToCharacter(s.ctx, tc.name+"-2", events.NewEventBus())
			s.Require().NoError(err)
			char.experience = ExperienceThresholdForLevel(2)

			out, err := char.Advance(s.ctx, &AdvanceInput{
				ClassID:        tc.class,
				HitPointMethod: HitPointMethodAverage,
			})
			s.Require().NoError(err)
			s.Equal(2, out.Gained.CharacterLevel)

			// Handed a choice it did not ask for, it refuses.
			refused, err := char.Advance(s.ctx, &AdvanceInput{
				ClassID:        tc.class,
				HitPointMethod: HitPointMethodAverage,
				Choices:        []choices.ChoiceData{spellChoice("made-up", spells.Bless)},
			})
			s.Require().Error(err)
			s.Nil(refused)
		})
	}
}

// --- the prepared caster and the spellbook (R4.6, R4.6a) ------------------

func (s *LevelUpSuite) TestAClericTakesLevelTwoWithNoQuestionAndGainsASlot() {
	char := s.cleric()
	s.Require().Equal(2, char.GetResource(resources.SpellSlotLevel1).Maximum())
	s.Empty(choices.GetClassChoiceIDsGainedAtLevel(classes.Cleric, 2),
		"a cleric prepares from its list; level 2 asks nothing")

	out, err := char.Advance(s.ctx, &AdvanceInput{
		ClassID:        classes.Cleric,
		HitPointMethod: HitPointMethodAverage,
	})
	s.Require().NoError(err)

	s.Equal(3, char.GetResource(resources.SpellSlotLevel1).Maximum())
	slots, ok := resourceChange(out, resources.SpellSlotLevel1)
	s.Require().True(ok)
	s.Equal(2, slots.From)
	s.Equal(3, slots.To)
}

// TestAWizardHasTheSlotPoolItNeverHad is R4.6a's stated consequence: a wizard
// had slot data and no pool, so casting was silently impossible for it.
func (s *LevelUpSuite) TestAWizardHasTheSlotPoolItNeverHad() {
	char := s.wizard()

	pool := char.GetResource(resources.SpellSlotLevel1)
	s.Require().NotNil(pool, "a level-1 wizard has two 1st-level slots")
	s.Equal(2, pool.Maximum())
}

func (s *LevelUpSuite) TestAWizardAddsTwoSpellsToItsSpellbookAtLevelTwo() {
	char := s.wizard()
	s.Require().Len(char.KnownSpells(), 6)

	reqs := choices.GetClassRequirementsGainedAtLevel(classes.Wizard, 2)
	s.Require().NotNil(reqs.Spellbook)
	s.Equal(choices.ChoiceID("wizard-spells-2"), reqs.Spellbook.ID)
	s.Equal(2, reqs.Spellbook.Count, "the spellbook column goes six to eight")

	// A wizard also picks its Arcane Tradition at 2, and a subclass is not
	// something a ChoiceData can express (rpg-toolkit#1767), so the level is
	// refused rather than half-taken.
	out, err := char.Advance(s.ctx, &AdvanceInput{
		ClassID:        classes.Wizard,
		HitPointMethod: HitPointMethodAverage,
		Choices: []choices.ChoiceData{
			spellChoice("wizard-spells-2", spells.DetectMagic, spells.Identify),
		},
	})
	s.Require().Error(err)
	s.Nil(out)
	s.ErrorContains(err, "subclass")
}

// TestSlotsFollowTheClassLevelNotTheCharacterLevel is the mutation this file
// exists to kill: a bard with one bard level among five has a level-1 BARD's
// slots, not a level-5 caster's. The two numbers are equal for every
// single-class character, which is exactly why a swap between them is
// invisible until multiclassing, and why it is pinned now.
func (s *LevelUpSuite) TestSlotsFollowTheClassLevelNotTheCharacterLevel() {
	levels := make([]LevelEntry, 0, 4)
	for i := 1; i <= 4; i++ {
		class := classes.Fighter
		if i == 1 {
			class = classes.Bard
		}
		levels = append(levels, LevelEntry{Level: i, ClassID: class, HitPointMethod: HitPointMethodAverage})
	}
	char := &Character{
		id:            "one-bard-level-among-four",
		classID:       classes.Bard,
		levels:        levels,
		experience:    ExperienceThresholdForLevel(MaxCharacterLevel),
		abilityScores: shared.AbilityScores{abilities.CON: 10, abilities.CHA: 10},
		resources:     make(map[coreResources.ResourceKey]*combat.RecoverableResource),
	}
	s.Require().Equal(4, char.GetLevel())
	s.Require().Equal(1, char.ClassLevel(classes.Bard))

	out, err := char.Advance(s.ctx, &AdvanceInput{
		ClassID:        classes.Bard,
		HitPointMethod: HitPointMethodAverage,
		Choices:        []choices.ChoiceData{spellChoice("bard-spells-2", spells.HealingWord)},
	})
	s.Require().NoError(err)
	s.Equal(5, out.Gained.CharacterLevel)
	s.Equal(2, out.Gained.ClassLevel)

	s.Equal(3, char.GetResource(resources.SpellSlotLevel1).Maximum(),
		"three 1st-level slots is BARD level 2; character level 5 would be four")
	s.Zero(char.GetResource(resources.SpellSlotLevel2).Maximum(),
		"a level-5 caster has 2nd-level slots and a bard 2 does not")

	// Hit dice are the other half of the same rule: they count every level,
	// whatever class took it.
	dice, ok := resourceChange(out, resources.HitDice)
	s.Require().True(ok)
	s.Equal(5, dice.To, "hit dice follow the CHARACTER level")
}

// TestABardAtThreeIsAskedForASecondLevelSpell pins the spell level a derived
// requirement asks at: a level-3 bard has 2nd-level slots, so the spell it
// learns is a 2nd-level one. This build has no 2nd-level spells, so the level
// is refused rather than answered with an empty list — which is a fact about
// the build, and the honest thing to say about it.
func (s *LevelUpSuite) TestABardAtThreeIsAskedForASecondLevelSpell() {
	reqs := choices.GetClassRequirementsGainedAtLevel(classes.Bard, 3)

	s.Require().NotNil(reqs.Spellbook)
	s.Equal(2, reqs.Spellbook.SpellLevel, "a level-3 bard casts at 2nd level")
	s.Empty(reqs.Spellbook.Options, "and this build has no 2nd-level bard spells")
}

// TestALevelWithNoContentToAnswerItIsRefused — a ranger learns two spells at
// level 2 and this build has no ranger spell list at all. Handing the player a
// level that silently taught them nothing is the failure this refuses.
func (s *LevelUpSuite) TestALevelWithNoContentToAnswerItIsRefused() {
	char := &Character{
		id:            "levelling-ranger",
		classID:       classes.Ranger,
		levels:        []LevelEntry{{Level: 1, ClassID: classes.Ranger, HitPointMethod: HitPointMethodMax}},
		experience:    ExperienceThresholdForLevel(2),
		abilityScores: shared.AbilityScores{abilities.CON: 10},
		resources:     make(map[coreResources.ResourceKey]*combat.RecoverableResource),
	}

	out, err := char.Advance(s.ctx, &AdvanceInput{
		ClassID:        classes.Ranger,
		HitPointMethod: HitPointMethodAverage,
	})

	s.Require().Error(err)
	s.Nil(out)
	s.ErrorContains(err, "this build has none")
	s.Equal(1, char.GetLevel())
}

// --- the slot table (R4.5, R4.6a) -----------------------------------------

func (s *LevelUpSuite) TestTheFullCasterSlotTableIsWhatTheBookSays() {
	for _, tc := range []struct {
		level int
		slots []int
	}{
		{level: 1, slots: []int{2}},
		{level: 2, slots: []int{3}},
		{level: 3, slots: []int{4, 2}},
		{level: 5, slots: []int{4, 3, 2}},
		{level: 20, slots: []int{4, 3, 3, 3, 3, 2, 2, 1, 1}},
	} {
		s.Run("", func() {
			row := classes.SpellProgressionAtLevel(classes.Bard, tc.level)
			s.Equal(tc.slots, row.SpellSlots, "bard level %d", tc.level)
		})
	}
}

func (s *LevelUpSuite) TestAWarlockIsUntouchedBecausePactMagicIsADifferentRule() {
	row := classes.SpellProgressionAtLevel(classes.Warlock, 5)

	s.Equal(classes.SpellSlotResetPactMagic, row.SlotReset)
	s.Equal(2, row.CantripsKnown, "the table above level 1 arrives with Pact Magic")
	s.Equal(2, row.SpellsKnown)
	s.Empty(choices.GetClassChoiceIDsGainedAtLevel(classes.Warlock, 2),
		"nothing about a warlock moves in this wave")
}

// TestNoClassAsksForOneChoiceIdAtTwoLevels pins R4.4a. The record keys a
// level's choices by this id, so two levels sharing one would make the record
// ambiguous about which level a choice belongs to.
func (s *LevelUpSuite) TestNoClassAsksForOneChoiceIdAtTwoLevels() {
	for classID := range classes.ClassData {
		s.Run(string(classID), func() {
			seen := make(map[choices.ChoiceID]int)
			for level := 1; level <= classes.MaxClassLevel; level++ {
				for _, id := range choices.GetClassChoiceIDsGainedAtLevel(classID, level) {
					if first, had := seen[id]; had {
						s.Failf("duplicate choice id",
							"%s asks for %q at level %d and again at level %d",
							classID, id, first, level)
					}
					seen[id] = level
				}
			}
		})
	}
}
