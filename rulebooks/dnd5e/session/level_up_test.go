// Copyright (C) 2026 Kirk Diggler
// SPDX-License-Identifier: GPL-3.0-or-later

package session_test

import (
	"context"
	"testing"

	"github.com/stretchr/testify/suite"

	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/refs"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/resources"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/session"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/spells"
)

// LevelUpSuite covers the write half of taking a level.
//
// The assertions split three ways, and keeping them apart is the point: what
// the REPOSITORY holds afterwards (the durable claim), what the verb REPORTED
// (the account the host renders), and what it REFUSED. A suite that only
// checked the report would pass with a verb that never saved — the flaw a
// mutant found in every fake in this package once already.
type LevelUpSuite struct {
	suite.Suite
}

func TestLevelUpSuite(t *testing.T) {
	suite.Run(t, new(LevelUpSuite))
}

// bardTakesHealingWord is the level-2 submission this suite's happy path uses:
// the one option the bard's own view of the row still offers.
func bardTakesHealingWord() []session.LevelChoiceSubmission {
	return []session.LevelChoiceSubmission{{
		ChoiceID:   "bard-spells-2",
		Selections: []string{spellRef(spells.HealingWord)},
	}}
}

// TestABardTakesTheLevelAndTheSheetHoldsIt is the durable claim.
//
// The report is checked second and the STORED sheet first, because the report
// is what a verb that forgot to save would still produce.
func (s *LevelUpSuite) TestABardTakesTheLevelAndTheSheetHoldsIt() {
	mgr, characters := advancementManager(s.T(), testDice{}, advancingBard("belwyn"))

	out, err := mgr.LevelUp(context.Background(), &session.LevelUpInput{
		Character:      "belwyn",
		HitPointMethod: session.HitPointsAverage,
		Choices:        bardTakesHealingWord(),
	})
	s.Require().NoError(err)

	stored := characters.byID["belwyn"]
	s.Equal(2, stored.Level, "the stored sheet is level 2")
	s.Len(stored.Levels, 2, "and its record says so in its own words")
	s.Contains(stored.KnownSpells, spellRef(spells.HealingWord))
	s.Len(stored.KnownSpells, 5, "four known before, one learned here")

	s.Equal([]string{"character:belwyn"}, out.Saved.Written)
	s.Empty(out.Saved.Failed)
	s.Equal(2, out.Gained.CharacterLevel)
	s.Equal(2, out.Gained.ClassLevel)
}

// TestTheLevelReportsWhichPoolsItMoved pins the account a host renders.
//
// Both rows below are TRANSITIONS from a nonzero maximum, which is what makes
// them discriminating: a fixture whose pools started empty would report 0 → 3
// for a slot it gained and 0 → 3 for a slot it already had, and neither the
// report nor this test could tell those apart.
func (s *LevelUpSuite) TestTheLevelReportsWhichPoolsItMoved() {
	mgr, _ := advancementManager(s.T(), testDice{}, advancingBard("belwyn"))

	out, err := mgr.LevelUp(context.Background(), &session.LevelUpInput{
		Character:      "belwyn",
		HitPointMethod: session.HitPointsAverage,
		Choices:        bardTakesHealingWord(),
	})
	s.Require().NoError(err)

	s.Contains(out.Gained.Resources, session.ResourceMaximumChange{
		Key: string(resources.SpellSlotLevel1), Name: "1st-level Spell Slots", From: 2, To: 3,
	})
	s.Contains(out.Gained.Resources, session.ResourceMaximumChange{
		Key: string(resources.HitDice), Name: "Hit Dice", From: 1, To: 2,
	})
}

// TestAFighterAveragesItsHitDie is the arithmetic the host must never do.
//
// Eight is 6 — half a d10 plus one — plus 2 for Constitution 14. A host that
// supplied a number instead would be holding a game rule; the method names how,
// and the rulebook produces the value.
func (s *LevelUpSuite) TestAFighterAveragesItsHitDie() {
	mgr, _ := advancementManager(s.T(), testDice{}, advancingFighter("ferrin"))

	out, err := mgr.LevelUp(context.Background(), &session.LevelUpInput{
		Character:      "ferrin",
		HitPointMethod: session.HitPointsAverage,
	})
	s.Require().NoError(err)

	s.Equal(8, out.Gained.HitPointGain, "6 for an averaged d10 plus 2 for CON 14")
	s.Contains(out.Gained.Features, refs.Features.ActionSurge().String())
	s.Equal(2, out.Gained.ProficiencyBonus)
}

// TestAFighterRollsThroughTheHostsOwnDice proves the roller reaches the
// rulebook, and that it is the HOST's.
//
// Seven is not a value any average produces for a d10 — the averaged gain is 8
// — so a verb that ignored the method and averaged anyway would fail here, and
// so would one that reached for its own randomness.
func (s *LevelUpSuite) TestAFighterRollsThroughTheHostsOwnDice() {
	mgr, _ := advancementManager(s.T(), &sequenceDice{rolls: []int{5}}, advancingFighter("ferrin"))

	out, err := mgr.LevelUp(context.Background(), &session.LevelUpInput{
		Character:      "ferrin",
		HitPointMethod: session.HitPointsRolled,
	})
	s.Require().NoError(err)

	s.Equal(7, out.Gained.HitPointGain, "the scripted 5 plus 2 for CON 14")
}

// TestASpellTheBardAlreadyKnowsIsRefusedAndNothingIsWritten is the rulebook's
// refusal arriving through this seam's vocabulary.
//
// Bane is on the class's level-2 list and off this bard's own view of it. The
// refusal must name the spell — a player has to know which pick was rejected —
// and the sheet must be exactly as it was, because a level is one act.
func (s *LevelUpSuite) TestASpellTheBardAlreadyKnowsIsRefusedAndNothingIsWritten() {
	mgr, characters := advancementManager(s.T(), testDice{}, advancingBard("belwyn"))

	out, err := mgr.LevelUp(context.Background(), &session.LevelUpInput{
		Character:      "belwyn",
		HitPointMethod: session.HitPointsAverage,
		Choices: []session.LevelChoiceSubmission{{
			ChoiceID:   "bard-spells-2",
			Selections: []string{spellRef(spells.Bane)},
		}},
	})
	s.Require().Error(err)
	s.ErrorIs(err, session.ErrBadLevelRequest)
	s.Contains(err.Error(), "bane", "the refusal names the spell that was rejected")
	s.Nil(out)

	s.Zero(characters.saves, "a refused level writes nothing")
	s.Equal(1, characters.byID["belwyn"].Level)
	s.Len(characters.byID["belwyn"].KnownSpells, 4)
}

// TestAChoiceTheLevelNeverAskedIsRefusedHere is the seam's own refusal, made
// before the rulebook is called.
//
// The rulebook refuses this too, and the assertion on the MESSAGE is what
// distinguishes the two: "this level did not ask for choice" is written here
// and nowhere else, while the rulebook's own words name the class and the
// level. Deleting this refusal leaves the call passing and this test failing,
// which is the whole reason to assert on text rather than only on the
// sentinel.
func (s *LevelUpSuite) TestAChoiceTheLevelNeverAskedIsRefusedHere() {
	mgr, characters := advancementManager(s.T(), testDice{}, advancingBard("belwyn"))

	_, err := mgr.LevelUp(context.Background(), &session.LevelUpInput{
		Character:      "belwyn",
		HitPointMethod: session.HitPointsAverage,
		Choices: []session.LevelChoiceSubmission{{
			ChoiceID:   "bard-expertise-3",
			Selections: []string{spellRef(spells.HealingWord)},
		}},
	})
	s.Require().Error(err)
	s.ErrorIs(err, session.ErrBadLevelRequest)
	s.Contains(err.Error(), "this level did not ask for choice")
	s.Contains(err.Error(), "bard-expertise-3", "and says which one")
	s.Zero(characters.saves)
}

// TestASelectionThatNamesNoSpellIsRefusedBeforeTheRulebook covers the other
// half of a submission: the id is real and the selection is not.
func (s *LevelUpSuite) TestASelectionThatNamesNoSpellIsRefusedBeforeTheRulebook() {
	ctx := context.Background()

	for _, tc := range []struct {
		name      string
		selection string
		expect    string
	}{
		{name: "not a ref at all", selection: "healing-word", expect: "is not a ref"},
		{name: "not a spell", selection: "dnd5e:features:action_surge", expect: "does not name a spell"},
		{name: "a spell nobody carries", selection: "dnd5e:spells:wish", expect: "names no spell this build carries"},
	} {
		s.Run(tc.name, func() {
			mgr, characters := advancementManager(s.T(), testDice{}, advancingBard("belwyn"))

			_, err := mgr.LevelUp(ctx, &session.LevelUpInput{
				Character:      "belwyn",
				HitPointMethod: session.HitPointsAverage,
				Choices: []session.LevelChoiceSubmission{{
					ChoiceID:   "bard-spells-2",
					Selections: []string{tc.selection},
				}},
			})
			s.Require().Error(err)
			s.ErrorIs(err, session.ErrBadLevelRequest)
			s.Contains(err.Error(), tc.expect)
			s.Contains(err.Error(), tc.selection, "the refusal quotes what the host actually sent")
			s.Zero(characters.saves)
		})
	}
}

// TestALevelNotYetEarnedIsRefusedAndSaysWhatItCosts is the experience gate,
// which lives in the rulebook and is reported through this seam's
// FailedPrecondition sentinel.
//
// The message must carry the threshold: "you cannot level up" sends a player
// nowhere, and "level 2 needs 300" tells them what they are playing for.
func (s *LevelUpSuite) TestALevelNotYetEarnedIsRefusedAndSaysWhatItCosts() {
	unearned := advancingBard("belwyn")
	unearned.Experience = 0
	mgr, characters := advancementManager(s.T(), testDice{}, unearned)

	_, err := mgr.LevelUp(context.Background(), &session.LevelUpInput{
		Character:      "belwyn",
		HitPointMethod: session.HitPointsAverage,
		Choices:        bardTakesHealingWord(),
	})
	s.Require().Error(err)
	s.ErrorIs(err, session.ErrCannotAdvance)
	s.NotErrorIs(err, session.ErrBadLevelRequest,
		"an unearned level is a fact about the game, not a malformed request")
	s.Contains(err.Error(), "300", "the refusal names what the level costs")
	s.Zero(characters.saves)
}

// TestAWizardsSubclassIsRefusedOnTheWriteToo is the read's refusal, proved to
// fire for a caller that never read.
//
// A host is not required to call NextLevel first, so a refusal that only lived
// there would be advisory. This is the same projection refusing the same row
// from the other side.
func (s *LevelUpSuite) TestAWizardsSubclassIsRefusedOnTheWriteToo() {
	mgr, characters := advancementManager(s.T(), testDice{}, advancingWizard("wren"))

	_, err := mgr.LevelUp(context.Background(), &session.LevelUpInput{
		Character:      "wren",
		HitPointMethod: session.HitPointsAverage,
	})
	s.Require().Error(err)
	s.ErrorIs(err, session.ErrLevelNotOffered)
	s.Contains(err.Error(), "subclass")
	s.Zero(characters.saves)
}

// TestAnUnofferedHitPointMethodIsRefusedBeforeAnyRead is the "reject cheap"
// pattern, and the assertion is on the REPOSITORY NOT BEING TOUCHED.
//
// "max" is the interesting row: the rulebook has that method and allows it at
// level 1 only, so a seam that passed it through would read a sheet, call
// Advance and come back with the rulebook's refusal — the same answer, three
// operations later, phrased in a vocabulary about level 1.
func (s *LevelUpSuite) TestAnUnofferedHitPointMethodIsRefusedBeforeAnyRead() {
	ctx := context.Background()

	for _, method := range []session.LevelUpHitPointMethod{"", "max", "ROLLED"} {
		s.Run(string(method), func() {
			mgr, characters := advancementManager(s.T(), testDice{}, advancingFighter("ferrin"))

			_, err := mgr.LevelUp(ctx, &session.LevelUpInput{
				Character: "ferrin", HitPointMethod: method,
			})
			s.Require().Error(err)
			s.ErrorIs(err, session.ErrBadLevelRequest)
			s.Zero(characters.loads, "no sheet is read for a method nobody offers")
			s.Zero(characters.saves)
		})
	}
}

// TestTheWriteRefusesWhatItCannotName is the shape half, matching the read's.
func (s *LevelUpSuite) TestTheWriteRefusesWhatItCannotName() {
	mgr, characters := advancementManager(s.T(), testDice{}, advancingFighter("ferrin"))
	ctx := context.Background()

	s.Run("nil input", func() {
		_, err := mgr.LevelUp(ctx, nil)
		s.ErrorIs(err, session.ErrNilInput)
	})

	s.Run("no character named", func() {
		_, err := mgr.LevelUp(ctx, &session.LevelUpInput{HitPointMethod: session.HitPointsAverage})
		s.ErrorIs(err, session.ErrNoMemberID)
		s.Zero(characters.loads)
	})

	s.Run("character absent", func() {
		_, err := mgr.LevelUp(ctx, &session.LevelUpInput{
			Character: "nobody", HitPointMethod: session.HitPointsAverage,
		})
		s.ErrorIs(err, session.ErrNoCharacter)
		s.Zero(characters.saves)
	})
}

// TestAFailedSaveSaysWhatDidNotLand is S6 for the narrowest possible write.
//
// One aggregate means the report can never be partial, but it must still say
// which aggregate failed: a bare error would leave a host unable to tell "the
// level did not happen" from "the level happened and the write did not".
func (s *LevelUpSuite) TestAFailedSaveSaysWhatDidNotLand() {
	characters := newFakeCharacters(advancingFighter("ferrin"))
	mgr := advancementManagerWith(s.T(), testDice{}, &refusingSaves{fakeCharacters: characters})

	out, err := mgr.LevelUp(context.Background(), &session.LevelUpInput{
		Character:      "ferrin",
		HitPointMethod: session.HitPointsAverage,
	})
	s.Require().Error(err)
	s.ErrorIs(err, session.ErrSaveFailed)
	s.ErrorIs(err, errSaveRefused, "the host's own error stays matchable")
	s.Nil(out)

	var saveErr *session.SaveError
	s.Require().ErrorAs(err, &saveErr)
	s.Equal([]string{"character:ferrin"}, saveErr.Report.Failed)
	s.Empty(saveErr.Report.Written, "nothing landed")
	s.False(saveErr.Report.Partial())
}
