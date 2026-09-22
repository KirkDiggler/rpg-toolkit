// Copyright (C) 2026 Kirk Diggler
// SPDX-License-Identifier: GPL-3.0-or-later

package choices_test

import (
	"testing"

	"github.com/stretchr/testify/suite"

	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/character/choices"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/classes"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/spells"
)

// SpellChoiceSuite covers the derived spell and cantrip requirements — the
// half of a class's question that comes out of its progression table.
type SpellChoiceSuite struct {
	suite.Suite
}

func TestSpellChoiceSuite(t *testing.T) {
	suite.Run(t, new(SpellChoiceSuite))
}

// TestTheLevelOneConstantsAreWhatTheComposerProduces is the guard on R4.4a.
//
// The published constants and the composed identity are two homes for one
// string, and the composed one is what a level-up publishes. A constant that
// drifted would leave creation and advancement naming different choices for
// the same question, which no test of either alone would show.
func (s *SpellChoiceSuite) TestTheLevelOneConstantsAreWhatTheComposerProduces() {
	for _, tc := range []struct {
		constant choices.ChoiceID
		composed choices.ChoiceID
	}{
		{choices.WizardCantrips1, choices.CantripChoiceID(classes.Wizard, 1)},
		{choices.WizardSpells1, choices.SpellChoiceID(classes.Wizard, 1)},
		{choices.ClericCantrips1, choices.CantripChoiceID(classes.Cleric, 1)},
		{choices.ClericSpells1, choices.SpellChoiceID(classes.Cleric, 1)},
		{choices.BardCantrips1, choices.CantripChoiceID(classes.Bard, 1)},
		{choices.BardSpells1, choices.SpellChoiceID(classes.Bard, 1)},
		{choices.DruidCantrips1, choices.CantripChoiceID(classes.Druid, 1)},
		{choices.SorcererCantrips1, choices.CantripChoiceID(classes.Sorcerer, 1)},
		{choices.SorcererSpells1, choices.SpellChoiceID(classes.Sorcerer, 1)},
		{choices.WarlockCantrips1, choices.CantripChoiceID(classes.Warlock, 1)},
		{choices.WarlockSpells1, choices.SpellChoiceID(classes.Warlock, 1)},
	} {
		s.Equal(tc.constant, tc.composed)
	}
}

// TestTheIdentityCarriesTheClassLevel — two levels of one class must never
// share a choice id, because the record keys a level's choices by it.
func (s *SpellChoiceSuite) TestTheIdentityCarriesTheClassLevel() {
	s.Equal(choices.ChoiceID("bard-spells-2"), choices.SpellChoiceID(classes.Bard, 2))
	s.Equal(choices.ChoiceID("bard-cantrips-4"), choices.CantripChoiceID(classes.Bard, 4))
	s.NotEqual(choices.SpellChoiceID(classes.Bard, 1), choices.SpellChoiceID(classes.Bard, 2))
}

// Catalog-only Light is selectable for Bard as well as Cleric, but gains no cast.
func (s *SpellChoiceSuite) TestBardCantripsIncludeExplicitCatalogEntries() {
	req := choices.GetClassRequirements(classes.Bard).Cantrips
	s.Require().NotNil(req)
	s.Contains(req.Options, spells.Light)
	s.False(spells.HasCastProfile(spells.Light))
	for _, id := range spells.Castable(spells.BardCantrips) {
		s.Contains(req.Options, id)
	}
	for _, id := range req.Options {
		data := spells.GetData(id)
		s.Require().NotNil(data)
		s.True(spells.HasCastProfile(id) || data.NotYetImplemented)
	}
}

// TestACantripQuestionArrivesWhenTheColumnMoves — a bard's cantrips known goes
// two to three at level 4 and nowhere else before 10, so that is where the
// question is and nowhere else.
func (s *SpellChoiceSuite) TestACantripQuestionArrivesWhenTheColumnMoves() {
	for level := 2; level <= 3; level++ {
		s.Nil(choices.GetClassRequirementsGainedAtLevel(classes.Bard, level).Cantrips,
			"bard level %d", level)
	}

	atFour := choices.GetClassRequirementsGainedAtLevel(classes.Bard, 4).Cantrips
	s.Require().NotNil(atFour, "two cantrips become three at level 4")
	s.Equal(1, atFour.Count)
	s.Equal(choices.ChoiceID("bard-cantrips-4"), atFour.ID)
	s.Equal("Choose 1 cantrip", atFour.Label, "one cantrip, singular")

	for level := 5; level <= 9; level++ {
		s.Nil(choices.GetClassRequirementsGainedAtLevel(classes.Bard, level).Cantrips,
			"bard level %d", level)
	}
}

// TestClericPreparationIsAChoiceRatherThanTheWholeList — the cleric's
// supported list is asked for entire, and Count equal to the option count is
// what "no choice" looks like from inside a requirement.
func (s *SpellChoiceSuite) TestClericPreparationIsAChoiceRatherThanTheWholeList() {
	req := choices.GetClassRequirements(classes.Cleric).Spellbook

	s.Require().NotNil(req)
	s.Greater(len(req.Options), req.Count)
	s.Equal("Choose 4 Cleric spells to prepare", req.Label)
}

// TestANonCasterIsAskedNothingAtAnyLevel — no progression table, no question,
// at any of the twenty levels.
func (s *SpellChoiceSuite) TestANonCasterIsAskedNothingAtAnyLevel() {
	for _, classID := range []classes.Class{
		classes.Fighter, classes.Barbarian, classes.Monk, classes.Rogue,
	} {
		for level := 1; level <= classes.MaxClassLevel; level++ {
			reqs := choices.GetClassRequirementsGainedAtLevel(classID, level)
			s.Nil(reqs.Cantrips, "%s level %d", classID, level)
			s.Nil(reqs.Spellbook, "%s level %d", classID, level)
		}
	}
}
