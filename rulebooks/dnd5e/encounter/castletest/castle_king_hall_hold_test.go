// Copyright (C) 2026 Kirk Diggler
// SPDX-License-Identifier: GPL-3.0-or-later

package castletest

// castle_king_hall_hold_test.go is THE KEEP'S HOLD, a small proof scene
// (rpg-project#990): the hero tries to shoulder past the great hall guard on
// his rounds. A failed check gives the speaker the status `held` — a hold the
// author wrote, carried to the session as the author's words — and a success
// gives nothing. Both start from one stored moment, so the grant is proved to
// survive a save and a load.

import (
	"encoding/json"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/KirkDiggler/rpg-toolkit/tools/spatial"
)

// besideTheHallGuard is the great hall floor beside hall-1's first cell,
// axial (13, -11).
var besideTheHallGuard = spatial.Position{X: 7, Y: -11}

func TestMadKingShoulderingPastTheHallGuardAndFailingLeavesTheSpeakerHeld(t *testing.T) {
	t.Parallel()
	s := openTheCastle(t, besideTheHallGuard)
	require.True(t, s.offers(kingHall1, "push-past"), "the hall guard can be shouldered past")
	blob, err := json.Marshal(s.enc.ToData())
	require.NoError(t, err)

	stopped := loadTheSceneOn(t, blob, s.compiled, s.host)
	out, err := stopped.speak(kingHall1, "push-past", "athletics", false)
	require.NoError(t, err)
	require.NotEmpty(t, out.Statuses, "a failure gives a status")
	for _, given := range out.Statuses {
		require.Equal(t, hero, given.Member, "the speaker is given it")
		require.Equal(t, "held", given.Grant.ID)
		require.JSONEq(t, `{"name":"Held","changes":[{"cannot":["move"]}],`+
			`"maintenance":[{"ends_on":"fight starts"},{"ends_on":"run"}],"again":"ignored"}`, string(given.Grant.Status),
			"the hold is carried in the author's words, unread")
	}
	require.Contains(t, stopped.saidBy(hero, kingHall1), "Not so fast. You stay right there.")

	past := loadTheSceneOn(t, blob, s.compiled, s.host)
	out, err = past.speak(kingHall1, "push-past", "athletics", true)
	require.NoError(t, err)
	require.Empty(t, out.Statuses, "a success gives nothing")
	require.Contains(t, past.saidBy(hero, kingHall1), "Oi! Mind where you’re going.")
}
