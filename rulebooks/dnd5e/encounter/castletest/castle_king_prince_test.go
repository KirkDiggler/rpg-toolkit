// Copyright (C) 2026 Kirk Diggler
// SPDX-License-Identifier: GPL-3.0-or-later

package castletest

// castle_king_prince_test.go is THE RIGHTFUL KING IN THE CELLS
// (rpg-project#1023, the Keep's shared cell): the castle's wiring of a plain
// line that recruits and a plain line that tells a way out. The party, failed
// at the gate twice and marched to the cells, finds the old king's son among
// the prisoners. Taken on, his side turns the party's, he follows the party
// and tells them the weak place in the cell, a drain at the back that comes
// up in the armoury. Left behind, he tells it all the same. Each rule is
// proved on its own site (the ally walk, the kitchen passage); this is only
// the castle's wiring, played from the jail.

import (
	"context"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/encounter"
)

// The rightful king's ids, as the file gives them.
const (
	kingPrince    = encounter.MemberID("prince")
	trueCrown     = encounter.FactionID("true-crown")
	kingCellDrain = encounter.PropID("cell-drain")
)

// Where the cells scene goes, by the file's regions and floor.
var (
	theCells       = encounter.RegionID("region-cells")
	theArmoury     = encounter.RegionID("region-armoury")
	cellsEastEnd   = castleCell(-16, 9)
	besideTheDrain = castleCell(-20, 13)
)

// toThePrince is the jail with the hero walked to the rightful king, who
// has not been spoken to: nobody knows the drain, and he is the party's to
// take on.
func toThePrince(t *testing.T) *kingScene {
	t.Helper()
	s := theJail(t)
	require.Equal(t, theCells, s.regionOf(s.at(hero)), "precondition: the hero was marched to the cells")
	require.Equal(t, theCells, s.regionOf(s.at(kingPrince)), "the rightful king is in the cells")
	s.walkTo(s.at(kingPrince))
	require.Equal(t, theCells, s.regionOf(s.at(hero)))
	_, err := s.enc.PropOptionsFor(hero, kingCellDrain)
	require.ErrorIs(t, err, encounter.ErrNoProp, "precondition: nobody knows the drain is there")
	require.Equal(t, encounter.StanceNeutral, s.stance(trueCrown, encounter.FactionParty), "he is civil to the party")
	s.sayPlain(kingPrince, "kill-the-king")

	return s
}

// sayPlain has the hero say one of a creature's plain lines: no check.
func (s *kingScene) sayPlain(target encounter.MemberID, option string) {
	s.t.Helper()
	_, err := s.enc.Speak(context.Background(), &encounter.SpeakInput{
		Range: speakReach, Actor: hero, Target: target, OptionID: option, Roller: rollsOne{},
		Revision: s.revisionOf(hero, target, option),
	})
	require.NoError(s.t, err, "%s offers %q", target, option)
}

// throughTheDrain walks the hero to the drain and takes it.
func (s *kingScene) throughTheDrain() {
	s.t.Helper()
	s.walkTo(besideTheDrain)
	offered, err := s.enc.PropOptionsFor(hero, kingCellDrain)
	require.NoError(s.t, err, "the hero knows the drain")
	require.NotEmpty(s.t, offered)
	_, err = s.enc.Interact(&encounter.InteractInput{
		Range: speakReach, Actor: hero, Prop: kingCellDrain, OptionID: "to-armoury", Roller: rollsOne{},
	})
	require.NoError(s.t, err)
	require.Equal(s.t, theArmoury, s.regionOf(s.at(hero)), "the passage comes up in the armoury")
}

// TestMadKingTheRightfulKingJoinsAndShowsTheWayOut: taken on with no check,
// the rightful king's side is the party's and the king's court his enemy;
// he follows the hero across the cell, and the hero knows the drain and
// takes it out to the armoury, out of the gate's sight.
func TestMadKingTheRightfulKingJoinsAndShowsTheWayOut(t *testing.T) {
	t.Parallel()
	s := toThePrince(t)
	require.True(t, s.offersTo(hero, kingPrince, "join-us"))

	s.sayPlain(kingPrince, "join-us")

	require.True(t, s.learners("prince-recruited")[kingPrince], "he took the party's offer")
	require.Equal(t, encounter.StanceAllied, s.stance(trueCrown, encounter.FactionParty), "his side is the party's")
	require.False(t, s.hostile(kingPrince, hero))
	require.Equal(t, encounter.StanceHostile, s.stance(trueCrown, loyalists), "the king's court is his enemy")
	require.False(t, s.offersTo(hero, kingPrince, "join-us"), "taken on once")

	post := s.at(kingPrince)
	s.walkTo(cellsEastEnd)
	s.markTime(2, kingPrince)
	require.Positive(t, s.pickedTowardAlly(kingPrince), "he follows the party")
	require.NotEqual(t, post, s.at(kingPrince), "he left where he stood")
	require.Less(t, s.enc.Distance(s.at(kingPrince), s.at(hero)), s.enc.Distance(post, s.at(hero)),
		"and came toward the hero")

	s.throughTheDrain()
	require.False(t, s.learners("prisoners-escaped")[kingCaptain], "nobody at the gate saw the hero come out")
}

// TestMadKingTheRightfulKingLeftBehindStillTellsTheWay: left behind, he is
// not the party's side and holds where he stands, and still tells the drain.
func TestMadKingTheRightfulKingLeftBehindStillTellsTheWay(t *testing.T) {
	t.Parallel()
	s := toThePrince(t)
	post := s.at(kingPrince)

	s.sayPlain(kingPrince, "leave-him")

	require.False(t, s.learners("prince-recruited")[kingPrince])
	require.Equal(t, encounter.StanceNeutral, s.stance(trueCrown, encounter.FactionParty), "left, he is not the party's side")
	s.throughTheDrain()
	require.Equal(t, post, s.at(kingPrince), "he stays in the cell")
}
