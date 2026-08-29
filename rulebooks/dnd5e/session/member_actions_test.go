// Copyright (C) 2026 Kirk Diggler
// SPDX-License-Identifier: GPL-3.0-or-later

package session

import (
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/KirkDiggler/rpg-toolkit/core"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/encounter"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/resolution"
)

// TestMemberActionsFrom is the drift-guard on a branch nothing reaches today.
//
// # Why a test for an unreachable branch
//
// memberActionsFrom refuses a nil attack, and Join cannot hand it one: the
// projection errors before returning a nil MainHand, because a character the
// rules can build at all HAS a main hand — an empty one is an unarmed strike.
// So the branch is defensive, and defensive code is exactly what rots: nobody
// exercises it, so nobody notices when the reason for it changes.
//
// What it guards against is drift in the OTHER module. The day resolution
// starts returning a nil MainHand without an error — a character kind with no
// hands, a projection that stops compiling attacks — this seam must refuse
// rather than seat a player who cannot swing. The alternative is an empty
// Actions list on the member record, which reads as "this member has no
// actions" and says nothing.
//
// It is a table because the two cases are one decision seen twice, and reading
// them side by side is the point: what arrives is either an attack or nothing,
// and nothing is an error.
func TestMemberActionsFrom(t *testing.T) {
	ref := core.Ref{Module: "dnd5e", Type: "weapons", ID: "unarmed-strike"}

	cases := []struct {
		name   string
		attack *resolution.AttackFacts
		want   []encounter.ActionView
		errIs  error
	}{
		{
			name:   "no attack compiled is refused, not seated empty",
			attack: nil,
			errIs:  ErrBadAttack,
		},
		{
			name: "a compiled attack maps field for field",
			attack: &resolution.AttackFacts{
				Ref: ref, Name: "Unarmed Strike", RangeFeet: 5, Kind: "melee",
			},
			want: []encounter.ActionView{{
				Ref: ref, Name: "Unarmed Strike", RangeFeet: 5, Kind: "melee",
			}},
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			actions, err := memberActionsFrom(tc.attack)

			if tc.errIs != nil {
				require.ErrorIs(t, err, tc.errIs)
				require.Nil(t, actions,
					"a refused compile seats nothing — an empty list would read as a member with no actions")

				return
			}

			require.NoError(t, err)
			require.Equal(t, tc.want, actions,
				"every field crosses: a dropped range or kind seats a member who cannot reach or "+
					"cannot be classified, and neither is visible on JoinOutput")
		})
	}
}
