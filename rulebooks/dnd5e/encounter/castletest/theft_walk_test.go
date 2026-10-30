// Copyright (C) 2026 Kirk Diggler
// SPDX-License-Identifier: GPL-3.0-or-later

package castletest

// theft_walk_test.go is THE THEFT WALK (rpg-project#719, Frank's list S5):
// the walk site cut from the castle's gatehouse, played through the castle's
// own host. The hero is caught lifting a pie and the cook's lines are gone
// for the hero; the bard outside, who saw nothing, is still offered them
// (D3: the speaker's knowledge); and the pedlar still talks.

import (
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/encounter"
	"github.com/KirkDiggler/rpg-toolkit/tools/spatial"
)

const theftWalkFixture = "testdata/theft-walk.yaml"

// The theft walk's cast, by the ids the file gives them.
const (
	theftCook   = encounter.MemberID("cook")
	theftPedlar = encounter.MemberID("pedlar")
	theftBard   = encounter.MemberID("bard")
)

// offersTo is whether a creature offers a member a line now.
func (s *kingScene) offersTo(member, target encounter.MemberID, option string) bool {
	s.t.Helper()
	options, err := s.enc.OptionsFor(member, target)
	require.NoError(s.t, err)
	for _, o := range options {
		if o.ID == option {
			return true
		}
	}

	return false
}

// TestTheTheftWalk: caught, the thief loses the cook's lines and nobody
// else does, and the pedlar still talks.
func TestTheTheftWalk(t *testing.T) {
	t.Parallel()
	s := openTheCastleAs(t, castleHost{
		at:       spatial.Position{X: 3, Y: 11}, // axial (-2, 11), beside the cook
		fixture:  theftWalkFixture,
		players:  []encounter.MemberInput{{ID: theftBard, Kind: encounter.KindPlayer, Position: spatial.Position{X: 7, Y: 16}}}, // axial (-1, 16), outside
		noReload: true,
	})
	require.True(t, s.offersTo(hero, theftCook, "pilfer"), "precondition: nobody has been caught")
	require.True(t, s.offersTo(hero, theftCook, "gossip"))

	s.said(theftCook, "pilfer", "sleight-of-hand", false)

	require.False(t, s.offersTo(hero, theftCook, "pilfer"), "the thief gets no second try")
	require.False(t, s.offersTo(hero, theftCook, "gossip"), "and the cook's gossip is closed to the thief")
	require.True(t, s.offersTo(theftBard, theftCook, "gossip"), "the bard outside saw nothing (D3)")
	require.True(t, s.offersTo(hero, theftPedlar, "tonight"), "the pedlar's line has no requires and stays")

}
