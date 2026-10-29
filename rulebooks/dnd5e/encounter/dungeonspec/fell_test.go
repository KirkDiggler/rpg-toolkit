// Copyright (C) 2026 Kirk Diggler
// SPDX-License-Identifier: GPL-3.0-or-later

package dungeonspec_test

// fell_test.go is the `fell` deed in the dialect (rpg-project#716): the word
// reads like every deed, `who` is sealed to a monster placed in this file,
// and `who` on any other deed is refused by that deed's name.

import (
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/encounter"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/encounter/dungeonspec"
)

// captainWhen places the heirloom's captain with one `time` row under when.
func captainWhen(t *testing.T, when string) string {
	t.Helper()

	return withPlacement(t, `  - { id: captain, ref: "dnd5e:monsters:skeleton-captain", at: [23,5], targeting: closest,
      holds: [vault-map], on: { time: [ { when: `+when+`, away: enemy }, { hold: {} } ] } }`)
}

// TestAFellRowCompilesWithWho: `fell` with `who`, a span and a scope is
// carried onto the compiled condition, and the file validates clean.
func TestAFellRowCompilesWithWho(t *testing.T) {
	for _, tc := range []struct {
		name  string
		when  string
		who   encounter.MemberID
		scope encounter.DeedScope
	}{
		{name: "fell within 3", when: `{ fell: { within: 3 } }`},
		{name: "fell, who, on ally", when: `{ fell: { who: captain, within: 3, on: ally } }`,
			who: "captain", scope: encounter.ScopeAlly},
		{name: "fell, as actor", when: `{ fell: { within: 2, as: actor } }`, scope: encounter.ScopeActor},
	} {
		t.Run(tc.name, func(t *testing.T) {
			source := captainWhen(t, tc.when)
			require.Empty(t, defectsIn(t, source))

			compiled, err := dungeonspec.Load([]byte(source))
			require.NoError(t, err)
			var when *encounter.When
			for _, m := range compiled.Monsters {
				if m.MemberID == "captain" {
					when = &m.Table[encounter.AnswerTime][0].When[0]
				}
			}
			require.NotNil(t, when, "the captain's row compiled")
			require.Equal(t, encounter.When{Deed: "fell", Within: when.Within, Who: tc.who, Scope: tc.scope}, *when)
		})
	}
}

// TestWhoOnAnotherDeedIsRefusedByItsName: `who` names who fell, and the
// refusal names the deed it was written on and the word that takes it.
func TestWhoOnAnotherDeedIsRefusedByItsName(t *testing.T) {
	for _, deed := range []string{"attacked", "intimidated", "persuaded", "fled"} {
		t.Run(deed, func(t *testing.T) {
			_, err := dungeonspec.Decode([]byte(captainWhen(t, `{ `+deed+`: { within: 3, who: captain } }`)))
			require.Error(t, err)
			require.Contains(t, err.Error(), "`who` names the one who fell, and only `fell` takes it; `"+deed+"` does not")
			require.Contains(t, err.Error(), "line ", "at the line it was written on")
		})
	}
}

// TestWhoIsSealedToAMonsterPlacedHere: a `who` naming a prop, or nobody
// this file places, is refused at the row, inside a `not` too.
func TestWhoIsSealedToAMonsterPlacedHere(t *testing.T) {
	for _, tc := range []struct {
		name string
		when string
		says string
	}{
		{name: "a prop", when: `{ fell: { who: heirloom, within: 3 } }`,
			says: `"heirloom" is a prop, and only a monster can fall`},
		{name: "nobody placed", when: `{ fell: { who: king, within: 3 } }`,
			says: `"king" is not a placement in this dungeon`},
		{name: "a prop, inside a not", when: `{ not: { fell: { who: hall-scroll, within: 3 } } }`,
			says: `"hall-scroll" is a prop, and only a monster can fall`},
	} {
		t.Run(tc.name, func(t *testing.T) {
			requireDefect(t, defectsIn(t, captainWhen(t, tc.when)), "on.time[0].when", tc.says)
		})
	}
}

// TestAnEmptyWhoNamesNobody: `who:` with no id is refused rather than read
// as "every fall", which is what omitting it already means.
func TestAnEmptyWhoNamesNobody(t *testing.T) {
	_, err := dungeonspec.Decode([]byte(captainWhen(t, `{ fell: { within: 3, who: "" } }`)))
	require.Error(t, err)
	require.Contains(t, err.Error(), "names nobody")
}
