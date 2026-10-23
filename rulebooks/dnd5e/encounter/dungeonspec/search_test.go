// Copyright (C) 2026 Kirk Diggler
// SPDX-License-Identifier: GPL-3.0-or-later

package dungeonspec_test

import (
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/encounter"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/encounter/dungeonspec"
)

// search_test.go is the dialect's half of pursuit (rpg-project#668 R4):
// `enemy: lost` and `search: { distance: N }`, with six for an omitted
// distance, refused where they cannot mean anything.

// searchingCaptain is the heirloom tomb's captain with this `time` table.
func searchingCaptain(t *testing.T, time string) string {
	t.Helper()

	return withPlacement(t, `  - { id: captain, ref: "dnd5e:monsters:skeleton-captain", at: [23,5], targeting: closest,
      holds: [vault-map], on: { time: [ `+time+` ] } }`)
}

// TestASearchCompilesWithItsDistanceOrSix: an authored distance arrives as
// written, an omitted one as [dungeonspec.DefaultSearchDistance], and the
// condition beside it as the lost band.
func TestASearchCompilesWithItsDistanceOrSix(t *testing.T) {
	require.Equal(t, 6, dungeonspec.DefaultSearchDistance)
	for _, tc := range []struct {
		name   string
		search string
		want   int
	}{
		{name: "written", search: `{ distance: 3 }`, want: 3},
		{name: "omitted", search: `{}`, want: dungeonspec.DefaultSearchDistance},
	} {
		t.Run(tc.name, func(t *testing.T) {
			source := searchingCaptain(t, `{ when: { enemy: lost }, search: `+tc.search+` }`)
			require.Empty(t, defectsIn(t, source))
			compiled, err := dungeonspec.Load([]byte(source))
			require.NoError(t, err)
			for _, m := range compiled.Monsters {
				if m.ID != "captain" {
					continue
				}
				row := m.Table[encounter.AnswerTime][0]
				require.NotNil(t, row.Search)
				require.Equal(t, tc.want, row.Search.Distance)
				require.Equal(t, encounter.HeadingWent, row.Search.Heading, "an omitted heading is went")
				require.Equal(t, encounter.EnemyLost, row.When[0].Enemy)

				return
			}
			require.Fail(t, "the fixture still places the captain")
		})
	}
}

// TestASearchCompilesWithItsHeadingOrWent is rpg-project#679 R10 at the
// form: every heading word arrives as written, and an omitted one as
// [dungeonspec.DefaultSearchHeading], which is went.
func TestASearchCompilesWithItsHeadingOrWent(t *testing.T) {
	require.Equal(t, encounter.HeadingWent, dungeonspec.DefaultSearchHeading)
	for _, tc := range []struct {
		search string
		want   encounter.SearchHeading
	}{
		{search: `{}`, want: encounter.HeadingWent},
		{search: `{ heading: went }`, want: encounter.HeadingWent},
		{search: `{ heading: faced }`, want: encounter.HeadingFaced},
		{search: `{ heading: left, distance: 3 }`, want: encounter.HeadingLeft},
		{search: `{ heading: right }`, want: encounter.HeadingRight},
	} {
		t.Run(tc.search, func(t *testing.T) {
			source := searchingCaptain(t, `{ when: { enemy: lost }, search: `+tc.search+` }`)
			require.Empty(t, defectsIn(t, source))
			compiled, err := dungeonspec.Load([]byte(source))
			require.NoError(t, err)
			for _, m := range compiled.Monsters {
				if m.ID == "captain" {
					require.Equal(t, tc.want, m.Table[encounter.AnswerTime][0].Search.Heading)

					return
				}
			}
			require.Fail(t, "the fixture still places the captain")
		})
	}
}

// TestASearchIsRefusedWhereItCannotMeanAnything: a distance below one, a
// search beside another word, and a search under a verdict.
func TestASearchIsRefusedWhereItCannotMeanAnything(t *testing.T) {
	for _, tc := range []struct {
		name string
		on   string
		path string
		says []string
	}{
		{
			name: "a search that walks nowhere",
			on:   `on: { time: [ { when: { enemy: lost }, search: { distance: 0 } } ] }`,
			path: "place[18].on.time[0]",
			says: []string{"`search` distance 0: a search walks at least one cell"},
		},
		{
			name: "a heading that is not one of the four",
			on:   `on: { time: [ { when: { enemy: lost }, search: { heading: backwards } } ] }`,
			path: "place[18].on.time[0]",
			says: []string{"`search` heading \"backwards\": a search follows went, faced, left or right"},
		},
		{
			name: "a search beside a walk",
			on:   `on: { time: [ { search: {}, toward: enemy } ] }`,
			path: "place[18].on.time[0]",
			says: []string{"an entry does one thing", "`toward` and `search`"},
		},
		{
			name: "a search under an option's verdict",
			on:   `options: [ { id: hi, say: "Hi.", check: [{ ability: persuasion, dc: 10 }], failure: [ { search: {} } ] } ]`,
			path: "place[18].options[0].failure[0].search",
			says: []string{"what a creature does with time"},
		},
	} {
		t.Run(tc.name, func(t *testing.T) {
			source := withPlacement(t, `  - { id: captain, ref: "dnd5e:monsters:skeleton-captain", at: [23,5], targeting: closest,
      holds: [vault-map], `+tc.on+` }`)
			requireDefect(t, defectsIn(t, source), append([]string{tc.path}, tc.says...)...)
		})
	}
}

// TestAnUnknownSearchKeyIsRefused: `search` reads `distance` and `heading` and
// nothing else.
func TestAnUnknownSearchKeyIsRefused(t *testing.T) {
	_, err := dungeonspec.Decode([]byte(searchingCaptain(t, `{ search: { cells: 6 } }`)))
	require.Error(t, err)
	require.Contains(t, err.Error(), "cells")
}

// TestASearchIsAMapping: a bare number is not a search; the distance is
// named.
func TestASearchIsAMapping(t *testing.T) {
	_, err := dungeonspec.Decode([]byte(searchingCaptain(t, `{ search: 6 }`)))
	require.Error(t, err)
	require.Contains(t, err.Error(), "`search` is { distance: N, heading: went }")
}
