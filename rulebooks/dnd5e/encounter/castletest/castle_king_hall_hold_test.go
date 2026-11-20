// Copyright (C) 2026 Kirk Diggler
// SPDX-License-Identifier: GPL-3.0-or-later

package castletest

// castle_king_hall_hold_test.go is THE KEEP'S HOLD, a small proof scene
// (rpg-project#990): the hero tries to shoulder past the great hall guard on
// his rounds. A failed check gives the status `held` to the speaker's party he
// sees — `to: party_in_sight`, a hold the author wrote, carried to the
// session as the author's words — and a success gives nothing. Both start
// from one stored moment, so the grant is proved to survive a save and a
// load.

import (
	"encoding/json"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/encounter"
	"github.com/KirkDiggler/rpg-toolkit/tools/spatial"
)

// besideTheHallGuard is the great hall floor beside hall-1's first cell,
// axial (13, -11).
var besideTheHallGuard = spatial.Position{X: 7, Y: -11}

// alsoInTheHall is the great hall floor beside the hero, in the hall
// guard's sight.
var alsoInTheHall = spatial.Position{X: 6, Y: -11}

func TestMadKingShoulderingPastTheHallGuardAndFailingHoldsThePartyHeSees(t *testing.T) {
	t.Parallel()
	s := openTheCastleAs(t, castleHost{
		at: besideTheHallGuard,
		players: []encounter.MemberInput{
			{ID: kingFighter, Kind: encounter.KindPlayer, Position: alsoInTheHall},
			// The rogue is down in the kitchen, out of the hall guard's sight.
			{ID: kingRogue, Kind: encounter.KindPlayer, Position: besideTheCook},
		},
	})
	require.True(t, s.offers(kingHall1, "push-past"), "the hall guard can be shouldered past")
	blob, err := json.Marshal(s.enc.ToData())
	require.NoError(t, err)

	stopped := loadTheSceneOn(t, blob, s.compiled, s.host)
	out, err := stopped.speak(kingHall1, "push-past", "athletics", false)
	require.NoError(t, err)
	var held []encounter.MemberID
	for _, given := range out.Statuses {
		held = append(held, given.Member)
		require.Equal(t, "held", given.Grant.ID)
		require.JSONEq(t, `{"name":"Held","changes":[{"cannot":["move"]}],`+
			`"maintenance":[{"ends_on":"fight starts"},{"ends_on":"run"}],"again":"ignored"}`, string(given.Grant.Status),
			"the hold is carried in the author's words, unread")
	}
	require.ElementsMatch(t, []encounter.MemberID{hero, kingFighter}, held,
		"the speaker and the companion he sees are held; the rogue in the kitchen is not")
	require.Contains(t, stopped.saidBy(hero, kingHall1), "Not so fast. All of you, stay right there.")

	past := loadTheSceneOn(t, blob, s.compiled, s.host)
	out, err = past.speak(kingHall1, "push-past", "athletics", true)
	require.NoError(t, err)
	require.Empty(t, out.Statuses, "a success gives nothing")
	require.Contains(t, past.saidBy(hero, kingHall1), "Oi! Mind where you’re going.")
}
