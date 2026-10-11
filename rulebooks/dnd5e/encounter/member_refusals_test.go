// Copyright (C) 2026 Kirk Diggler
// SPDX-License-Identifier: GPL-3.0-or-later

package encounter_test

import (
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/encounter"
)

// TestEachMemberRefusalAnswersItsOwnSentinel pins one meaning per refusal at the
// doors a caller switches on: an empty id is ErrNoMember, an id the roster
// already holds is ErrDuplicateMember, an id that names nobody is ErrNotMember
// and a member that cannot be seated as authored is ErrBadMember. No answer
// carries a second member sentinel, so a caller that handles one never
// mistakes it for another.
func TestEachMemberRefusalAnswersItsOwnSentinel(t *testing.T) {
	members := []error{
		encounter.ErrNoMember, encounter.ErrDuplicateMember, encounter.ErrNotMember, encounter.ErrBadMember,
	}
	player := func(id encounter.MemberID) encounter.MemberInput {
		return encounter.MemberInput{ID: id, Kind: encounter.KindPlayer, Position: cellAt(0, 0)}
	}
	open := func(t *testing.T) *encounter.Encounter {
		t.Helper()
		setup := emptyWorld(player(alice))
		setup.Participation = everyoneStanding{}
		enc, err := encounter.NewEncounter(setup)
		require.NoError(t, err)
		return enc
	}

	cases := map[string]struct {
		err  func(t *testing.T) error
		want error
	}{
		"setup with an empty id": {
			func(t *testing.T) error {
				_, err := encounter.NewEncounter(emptyWorld(player("")))
				return err
			}, encounter.ErrNoMember,
		},
		"setup naming one id twice": {
			func(t *testing.T) error {
				_, err := encounter.NewEncounter(emptyWorld(player(alice), player(alice)))
				return err
			}, encounter.ErrDuplicateMember,
		},
		"join with an empty id": {
			func(t *testing.T) error {
				_, err := open(t).Join(&encounter.JoinInput{Member: "", Kind: encounter.KindPlayer, Cell: cellAt(1, 0)})
				return err
			}, encounter.ErrNoMember,
		},
		"join under an id already present": {
			func(t *testing.T) error {
				_, err := open(t).Join(&encounter.JoinInput{Member: alice, Kind: encounter.KindPlayer, Cell: cellAt(1, 0)})
				return err
			}, encounter.ErrDuplicateMember,
		},
		"exit by a stranger": {
			func(t *testing.T) error {
				_, err := open(t).Exit(&encounter.ExitInput{Member: "stranger"})
				return err
			}, encounter.ErrNotMember,
		},
		"setup with a threat that has nothing to beat": {
			func(t *testing.T) error {
				bad := player(bob)
				bad.Intimidate = []encounter.CheckApproach{{Ability: "intimidation", DC: 0}}
				_, err := encounter.NewEncounter(emptyWorld(bad))
				return err
			}, encounter.ErrBadMember,
		},
	}
	for name, tc := range cases {
		t.Run(name, func(t *testing.T) {
			err := tc.err(t)
			require.ErrorIs(t, err, tc.want)
			for _, other := range members {
				if other != tc.want {
					require.NotErrorIs(t, err, other, "the answer carries one member meaning")
				}
			}
		})
	}
}
