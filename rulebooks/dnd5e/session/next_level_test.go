// Copyright (C) 2026 Kirk Diggler
// SPDX-License-Identifier: GPL-3.0-or-later

package session_test

import (
	"context"
	"testing"

	"github.com/stretchr/testify/suite"

	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/character/choices"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/classes"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/refs"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/session"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/spells"
)

// NextLevelSuite covers the read half of taking a level.
//
// EVERY ASSERTION HERE IS ABOUT THE PROJECTION, not about what a level does.
// How many spells a bard learns at 2 and what Action Surge is are the
// rulebook's, tested where they live; this suite pins that the sheet's own
// view of the row reaches the host intact, that the availability signal is
// projected rather than decided, and that a question this seam cannot ask is
// refused instead of dropped.
type NextLevelSuite struct {
	suite.Suite
}

func TestNextLevelSuite(t *testing.T) {
	suite.Run(t, new(NextLevelSuite))
}

// TestABardIsOfferedOnlyTheSpellItDoesNotKnow is the duplicate-spell fix
// arriving at the seam (rpg-toolkit#1781).
//
// The class's level-2 row offers one spell from a list of five. This bard
// already holds four of them, so the only honest menu has ONE option on it,
// and the count stays 1 because how many spells a level teaches is a rule
// rather than a property of who is taking it. A projection driven by the class
// row instead of the character's would offer five, and the player would find
// out four of them were not choices only after picking one.
func (s *NextLevelSuite) TestABardIsOfferedOnlyTheSpellItDoesNotKnow() {
	mgr, _ := advancementManager(s.T(), testDice{}, advancingBard("belwyn"))

	out, err := mgr.NextLevel(context.Background(), &session.NextLevelInput{Character: "belwyn"})
	s.Require().NoError(err)

	s.Require().Len(out.Choices, 1, "level 2 asks a bard exactly one question")
	choice := out.Choices[0]
	s.Equal(session.LevelChoiceSpell, choice.Kind)
	s.Equal(1, choice.Count, "the count is the class's and is not reduced with the options")
	s.Equal(1, choice.SpellLevel)
	s.Equal([]string{spellRef(spells.HealingWord)}, choice.Options,
		"the four this bard already knows are not choices")
	s.Equal("bard-spells-2", choice.ID)
}

// TestABardsEntitlementIsProjectedNotDecided pins the availability signal as
// two numbers rather than a flag.
//
// The gap between Level and EntitledLevel IS "a level is waiting" (R4.10).
// This verb reports both and compares neither, because a boolean here would be
// a rule in the seam that owns no rules.
func (s *NextLevelSuite) TestABardsEntitlementIsProjectedNotDecided() {
	mgr, _ := advancementManager(s.T(), testDice{}, advancingBard("belwyn"))

	out, err := mgr.NextLevel(context.Background(), &session.NextLevelInput{Character: "belwyn"})
	s.Require().NoError(err)

	s.Equal(1, out.Level, "the sheet is still level 1")
	s.Equal(2, out.EntitledLevel, "and 300 experience has earned level 2")
	s.Equal(levelUpXP, out.Experience)
	s.Equal(900, out.NextThreshold, "the total level 3 needs")
	s.Equal(2, out.CharacterLevel, "the level it would take")
	s.Equal(2, out.ClassLevel)
	s.Equal("bard", out.Class)
	s.Equal(8, out.HitDie, "a bard's d8")
	s.Equal("belwyn", out.Character)
}

// TestAFighterIsAskedNothingAndGivenActionSurge is the other shape a level
// takes: no question, one grant.
//
// A level that asks nothing is a level and not an error — it is most levels of
// most classes — so an empty Choices here is the assertion, alongside the
// grant that proves the class table was actually read.
func (s *NextLevelSuite) TestAFighterIsAskedNothingAndGivenActionSurge() {
	mgr, _ := advancementManager(s.T(), testDice{}, advancingFighter("ferrin"))

	out, err := mgr.NextLevel(context.Background(), &session.NextLevelInput{Character: "ferrin"})
	s.Require().NoError(err)

	s.Empty(out.Choices, "fighter level 2 asks nothing")
	s.Contains(out.Features, refs.Features.ActionSurge().String())
	s.Equal(10, out.HitDie, "a fighter's d10")
	s.Equal(2, out.CharacterLevel)
	s.Equal(2, out.ClassLevel)
	s.Equal("fighter", out.Class)
}

// TestAWizardsSubclassIsRefusedRatherThanDropped is the charter's own
// "a kind this seam was never taught is refused" pattern, at the READ.
//
// A wizard picks its arcane tradition at 2. This seam can translate spells and
// cantrips and nothing else, and the refusal is here rather than only on the
// write because a read that quietly returned the spell half of that row would
// hand a client a confirmation the write was always going to refuse — and the
// missing question would look exactly like a question that was never asked.
func (s *NextLevelSuite) TestAWizardsSubclassIsRefusedRatherThanDropped() {
	mgr, _ := advancementManager(s.T(), testDice{}, advancingWizard("wren"))

	out, err := mgr.NextLevel(context.Background(), &session.NextLevelInput{Character: "wren"})
	s.Require().Error(err)
	s.ErrorIs(err, session.ErrLevelNotOffered)
	s.Contains(err.Error(), "subclass", "the refusal names the kind it could not offer")
	s.Nil(out, "and no half-answer is invented for it")
}

// TestTheReadRefusesWhatItCannotName covers the shape refusals, each of which
// must be distinguishable from the others: a caller that passed nothing, a
// caller that named nobody, and a caller that named somebody who is not there.
func (s *NextLevelSuite) TestTheReadRefusesWhatItCannotName() {
	mgr, characters := advancementManager(s.T(), testDice{}, advancingBard("belwyn"))
	ctx := context.Background()

	s.Run("nil input", func() {
		_, err := mgr.NextLevel(ctx, nil)
		s.ErrorIs(err, session.ErrNilInput)
	})

	s.Run("no character named", func() {
		_, err := mgr.NextLevel(ctx, &session.NextLevelInput{})
		s.ErrorIs(err, session.ErrNoMemberID)
		s.Zero(characters.loads, "and the repository is never asked about nobody")
	})

	s.Run("character absent", func() {
		_, err := mgr.NextLevel(ctx, &session.NextLevelInput{Character: "nobody"})
		s.ErrorIs(err, session.ErrNoCharacter)
		s.Contains(err.Error(), "nobody")
	})
}

// TestTheReadSavesNothing is the claim the verb's name makes.
//
// It would be easy for a read to write: it loads a sheet, and the rulebook's
// own NextLevelRequirements edits the row it returns. Nothing here may reach
// the repository.
func (s *NextLevelSuite) TestTheReadSavesNothing() {
	mgr, characters := advancementManager(s.T(), testDice{}, advancingBard("belwyn"))

	_, err := mgr.NextLevel(context.Background(), &session.NextLevelInput{Character: "belwyn"})
	s.Require().NoError(err)
	s.Zero(characters.saves, "a read persists nothing")
	s.Equal(1, characters.byID["belwyn"].Level, "and the stored sheet is untouched")
}

// TestAnOptionTheCatalogCannotNameIsRefused is the other half of failing
// closed at the read, and it is REACHABLE TODAY rather than hypothetical.
//
// A druid's level-4 cantrip row offers Thorn Whip. The rulebook's choice
// vocabulary spells its id "thornwhip" and the ref catalog spells it
// "thorn-whip", so refs.Spells.ByID answers nil for an option that is really
// on the list. That is a content mismatch in the rulebook module, not here —
// and what this seam owes is to say so rather than hand the player a
// ten-option menu where the class authored eleven.
//
// The cost of the alternative is not cosmetic. A choice whose count equals its
// option list — the cleric's supported spells are one — becomes unanswerable
// the moment a single option is dropped, and the player only finds out after
// confirming. This test pins the loud failure; it starts passing for a
// different reason, still green, on the day the two spellings are reconciled,
// which is why it asserts on the spell's name rather than merely on an error.
func (s *NextLevelSuite) TestAnOptionTheCatalogCannotNameIsRefused() {
	mgr, _ := advancementManager(s.T(), testDice{}, advancingDruid("dara"))

	authored := choices.GetClassRequirementsGainedAtLevel(classes.Druid, 4).Cantrips
	s.Require().NotNil(authored, "the fixture is only interesting while this level asks for a cantrip")

	out, err := mgr.NextLevel(context.Background(), &session.NextLevelInput{Character: "dara"})
	if err == nil {
		// The ONLY acceptable success is a menu as long as the class
		// authored, which is what makes this row kill a silent drop rather
		// than accept it. A verb that skipped the option it could not name
		// arrives here with one fewer, and says nothing about it.
		s.Len(out.Choices, 1)
		s.Len(out.Choices[0].Options, len(authored.Options),
			"every option the class authored reaches the player, or the read refuses")
		s.T().Log("the rulebook's spell ids and its ref catalog now agree; this row no longer refuses")
		return
	}

	s.ErrorIs(err, session.ErrLevelNotOffered)
	s.Contains(err.Error(), "thornwhip", "the refusal names the option it could not put on the menu")
	s.Nil(out)
}
