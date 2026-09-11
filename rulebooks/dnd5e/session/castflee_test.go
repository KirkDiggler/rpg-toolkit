// Copyright (C) 2026 Kirk Diggler
// SPDX-License-Identifier: GPL-3.0-or-later

package session_test

import (
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/refs"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/spells"
)

// The flee, through the one verb. Dissonant Whispers is the first cast whose
// move is budgeted by the mover's own legs and priced at its own reaction, so
// these are the two facts this seam newly supplies: the speed off the roster
// row, and a price nobody could pay reported as a real outcome.
//
// EVERYTHING ELSE IS SOMEBODY ELSE'S. How far "away" lands is encounter's
// ruler, what half a save costs is resolution's trace, and what the spell says
// is content's. What is pinned here is that the budget arrives and that a move
// not taken is not a move that silently did nothing.

// whispersAt scripts a whisper the skeleton fails: a natural 1 against DC 13,
// then a 3d6 of ones. Three psychic leaves a 13-hit-point skeleton standing,
// because a dropped creature is not asked to run.
func (s *CastSuite) whispersAt(cells int) {
	s.T().Helper()
	s.scene(castingBardWithSpells("bard", spells.DissonantWhispers), cells, 1, 1, 1, 1)
}

// fleeResult is the cast beat's own account of the push: how far it went and
// what stopped it.
func (s *CastSuite) fleeResult() *storyBeat {
	s.T().Helper()
	story := s.storyOf("bard")
	for i := range story {
		if story[i].Result != nil && story[i].Result.Kind == "moved" {
			return &story[i]
		}
	}
	return nil
}

// walkedCells counts the movement beats one member's walk wrote — one per cell
// announced, which is what makes them comparable with the distance the cast
// beat claimed.
func (s *CastSuite) walkedCells(member string) int {
	s.T().Helper()
	walked := 0
	for _, beat := range s.storyOf("bard") {
		if beat.Beat == "moved" && beat.Member == member {
			walked++
			s.Equal(refs.Spells.DissonantWhispers().String(), beat.Cause,
				"a creature running from a whisper did not choose to; every cell names the spell")
		}
	}
	return walked
}

// slowTo rewrites a member's speed on the stored roster row, which is where
// the budget is read from.
func (s *CastSuite) slowTo(member string, feet int) {
	s.T().Helper()
	world := s.encounters.byID["world"]
	for i := range world.Members {
		if string(world.Members[i].ID) == member {
			world.Members[i].SpeedFeet = feet
			return
		}
	}
	s.FailNowf("no such member", "the roster has no %q to slow down", member)
}

// spendSkeletonReaction empties the monster's one reaction meter before the
// whisper asks for it.
func (s *CastSuite) spendSkeletonReaction() {
	s.T().Helper()
	for i := range s.sessions.byID["sess"].NPCs {
		if s.sessions.byID["sess"].NPCs[i].ID == "skeleton" {
			s.sessions.byID["sess"].NPCs[i].ReactionSpent = true
			return
		}
	}
	s.FailNow("no skeleton in the record")
}

// TestAFailedWhisperSendsTheCreatureRunningOnItsOwnLegs is the budget, and the
// budget is the whole of what this module contributes to the flee.
//
// A skeleton walks thirty feet, which is six cells. Nothing in the content
// says six — the profile says "its own speed" and stops — so a seam that could
// not read a speed off the roster refused the spell outright until this slice.
func (s *CastSuite) TestAFailedWhisperSendsTheCreatureRunningOnItsOwnLegs() {
	s.whispersAt(2)
	before := s.cellOf("skeleton")

	out, err := s.cast(spells.DissonantWhispers)
	s.Require().NoError(err)
	s.Require().NotNil(out.Saved)
	s.False(out.Saved.Succeeded, "a natural 1 fails, and only a failure runs")
	s.Positive(s.storedSkeleton(), "a dropped skeleton is not asked to run")

	moved := s.fleeResult()
	s.Require().NotNil(moved, "the cast's own account of how far the whisper sent it")
	s.Require().NotNil(moved.Result.Moved)
	s.Equal(6, *moved.Result.Moved, "a thirty-foot walk is six cells of open floor")
	s.Equal("skeleton", moved.Result.Target)
	s.Equal(refs.Spells.DissonantWhispers().String(), moved.Result.Ref)

	s.Equal(*moved.Result.Moved, s.walkedCells("skeleton"),
		"the walk went exactly as far as the cast beat said it would")
	s.NotEqual(before, s.cellOf("skeleton"), "and it is standing somewhere else")
}

// TestASlowerCreatureRunsNoFartherThanItsOwnSpeed is the same scene with one
// number changed, which is what makes the budget legible: a seam that had
// hardcoded a distance, or read the content's unset Cells, would answer the
// same for both.
func (s *CastSuite) TestASlowerCreatureRunsNoFartherThanItsOwnSpeed() {
	s.whispersAt(2)
	s.slowTo("skeleton", 25)

	_, err := s.cast(spells.DissonantWhispers)
	s.Require().NoError(err)

	moved := s.fleeResult()
	s.Require().NotNil(moved)
	s.Require().NotNil(moved.Result.Moved)
	s.Equal(5, *moved.Result.Moved, "twenty-five feet is five cells, and the floor is the same floor")
}

// TestACreatureWithNoReactionToSpendStaysWhereItIs — the price is the
// composition's, but the REPORT is this seam's. A move nobody could pay for is
// a real outcome with a reason, not a missing one: zero cells and the sentence
// resolution wrote.
func (s *CastSuite) TestACreatureWithNoReactionToSpendStaysWhereItIs() {
	s.whispersAt(2)
	s.spendSkeletonReaction()
	before := s.cellOf("skeleton")

	_, err := s.cast(spells.DissonantWhispers)
	s.Require().NoError(err)

	moved := s.fleeResult()
	s.Require().NotNil(moved, "a move that could not be paid for is still narrated")
	s.Require().NotNil(moved.Result.Moved)
	s.Zero(*moved.Result.Moved, "nothing was walked")
	s.Equal("has no reaction to spend", moved.Result.StoppedBy,
		"and the beat says why, in resolution's own words")

	s.Equal(before, s.cellOf("skeleton"), "the skeleton stands where it stood")
	s.Zero(s.walkedCells("skeleton"), "no walk was taken, so no cell was announced")
}

// TestTheWhisperStillDamagesWhatItCannotMove pins the two halves apart. The
// psychic damage is the gate's; the flee is the price's. A creature with no
// reaction still took the whisper.
func (s *CastSuite) TestTheWhisperStillDamagesWhatItCannotMove() {
	s.whispersAt(2)
	s.spendSkeletonReaction()
	before := s.storedSkeleton()

	_, err := s.cast(spells.DissonantWhispers)
	s.Require().NoError(err)

	s.Less(s.storedSkeleton(), before, "the psychic damage reached the stored sheet regardless")
}
