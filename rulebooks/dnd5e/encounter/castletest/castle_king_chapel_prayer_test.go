// Copyright (C) 2026 Kirk Diggler
// SPDX-License-Identifier: GPL-3.0-or-later

package castletest

// castle_king_chapel_prayer_test.go is THE KEEP'S CHAPEL BENEFIT
// (rpg-project#1036): a plain line on the chapel altar, `pray`, gives whoever
// does it a status an author wrote, advantage on ability checks until the run
// ends. The grant is returned on the interaction for the session to give. The
// encounter does not decide whether it stacks: that is the status's own
// `again: ignored`, said in the author's words and carried through.

import (
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/encounter"
	"github.com/KirkDiggler/rpg-toolkit/tools/spatial"
)

const (
	kingAltar = encounter.PropID("altar")

	// theSecondPraying is a second player who comes to the altar.
	theSecondPraying = encounter.MemberID("pilgrim")

	prayedStatus = `{"name":"Prayed","changes":[{"advantage":"checks"}],` +
		`"maintenance":[{"ends_on":"run"}],"again":"ignored"}`
)

// besideTheAltar is a chapel floor cell next to the altar, which stands on
// offset cell (20, -12).
var besideTheAltar = spatial.Position{X: 20, Y: -11}

// atTheAltar opens the castle with the hero and a pilgrim on cells beside the
// chapel altar.
func atTheAltar(t testing.TB) *kingScene {
	t.Helper()
	beside := besideTheAltar
	other := spatial.Position{X: besideTheAltar.X - 1, Y: besideTheAltar.Y}

	return openTheCastleAs(t, castleHost{
		at:      beside,
		players: []encounter.MemberInput{{ID: theSecondPraying, Kind: encounter.KindPlayer, Position: other}},
	})
}

// prays has a member do the altar's `pray`: a plain line, no check.
func (s *kingScene) prays(member encounter.MemberID) *encounter.InteractOutput {
	s.t.Helper()
	out, err := s.enc.Interact(&encounter.InteractInput{
		Range: speakReach, Actor: member, Prop: kingAltar, OptionID: "pray", Roller: rollsOne{},
	})
	require.NoError(s.t, err, "a plain line: no check")

	return out
}

func TestMadKingThePrayerLeavesTheSpeakerPrayed(t *testing.T) {
	t.Parallel()
	s := atTheAltar(t)

	out := s.prays(hero)
	require.NotEmpty(t, out.Statuses, "praying gives a status")
	for _, given := range out.Statuses {
		require.Equal(t, hero, given.Member, "the one who prays is given it, and nobody else")
		require.Equal(t, "prayed", given.Grant.ID)
		require.JSONEq(t, prayedStatus, string(given.Grant.Status))
	}
}

func TestMadKingThePrayerIsGivenOnceToEachWhoPrays(t *testing.T) {
	t.Parallel()
	s := atTheAltar(t)

	first := s.prays(hero)
	second := s.prays(theSecondPraying)
	require.NotEmpty(t, second.Statuses, "a second player who prays is granted it too")
	for _, given := range second.Statuses {
		require.Equal(t, theSecondPraying, given.Member, "each prays for themselves")
		require.Equal(t, "prayed", given.Grant.ID)
	}
	for _, given := range first.Statuses {
		require.Equal(t, hero, given.Member, "the pilgrim's prayer did not touch the hero's grant")
	}

	// The same player praying again is still returned a grant: whether it
	// stacks is the rules' `again: ignored`, not the encounter's to decide.
	again := s.prays(hero)
	require.NotEmpty(t, again.Statuses, "the encounter returns the grant; the rules decide whether it stacks")
	for _, given := range again.Statuses {
		require.Equal(t, hero, given.Member)
		require.Equal(t, "prayed", given.Grant.ID)
		require.JSONEq(t, prayedStatus, string(given.Grant.Status))
	}
}
