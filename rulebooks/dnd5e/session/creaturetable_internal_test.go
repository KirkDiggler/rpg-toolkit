// Copyright (C) 2026 Kirk Diggler
// SPDX-License-Identifier: GPL-3.0-or-later

package session

import (
	"sort"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/KirkDiggler/rpg-toolkit/core"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/encounter"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/encounter/dungeonspec"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/monster/table"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/refs"
)

// The rulebook's default table is laid UNDER the author's orders, which is why
// a placement with no `on:` still fights (rpg-project#465 §1, layer 1).
func TestTheRulebooksDefaultIsFoldedUnderTheAuthorsOrders(t *testing.T) {
	tests := []struct {
		name     string
		authored encounter.Table
		wantKeys []encounter.AnswerKey
		assert   func(t *testing.T, folded encounter.Table)
	}{
		{
			name:     "a placement that authored nothing fights from its kind's own table",
			authored: nil,
			wantKeys: []encounter.AnswerKey{encounter.AnswerTime},
			assert: func(t *testing.T, folded encounter.Table) {
				require.Equal(t, shippedDefault(t)[encounter.AnswerTime], folded[encounter.AnswerTime],
					"the rulebook's own entries, entire — nothing was laid over them")
			},
		},
		{
			name: "a placement that authored only a social key keeps the default's time",
			authored: encounter.Table{
				encounter.AnswerIntimidated: {{Weight: 1, Flee: true}},
			},
			wantKeys: []encounter.AnswerKey{encounter.AnswerIntimidated, encounter.AnswerTime},
			assert: func(t *testing.T, folded encounter.Table) {
				require.Equal(t, shippedDefault(t)[encounter.AnswerTime], folded[encounter.AnswerTime],
					"the author said nothing about this creature's turns, so the rulebook still does")
				require.True(t, folded[encounter.AnswerIntimidated][0].Flee)
			},
		},
		{
			name: "a placement's own time key replaces the default's WHOLESALE",
			authored: encounter.Table{
				encounter.AnswerTime: {{
					Weight: 1,
					When:   &encounter.When{Enemy: encounter.EnemyNone},
					Toward: &encounter.Selector{Word: encounter.SelectorEnemy},
				}},
			},
			wantKeys: []encounter.AnswerKey{encounter.AnswerTime},
			assert: func(t *testing.T, folded encounter.Table) {
				require.Len(t, folded[encounter.AnswerTime], 1,
					"one entry, not six: there is no merging of entry lists, so an author "+
						"never has to reason about what was added to what")
				require.NotNil(t, folded[encounter.AnswerTime][0].Toward,
					"and it is the author's entry that survived, not the rulebook's")
			},
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			folded, err := foldedTable(refs.Monsters.Goblin(), tc.authored)

			require.NoError(t, err)
			require.ElementsMatch(t, tc.wantKeys, keysOf(folded))
			tc.assert(t, folded)
		})
	}
}

// The fold refuses rather than answering a creature with no policy at all.
func TestTheFoldRefusesARefTheRulebookCannotAnswerFor(t *testing.T) {
	tests := []struct {
		name string
		ref  *core.Ref
	}{
		{name: "no ref at all", ref: nil},
		{name: "a ref naming nothing", ref: &core.Ref{}},
		{name: "a ref missing its type", ref: &core.Ref{Module: "dnd5e", ID: "goblin"}},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			folded, err := foldedTable(tc.ref, nil)

			require.Error(t, err, "a creature placed with no policy would stand there looking like a choice")
			require.Nil(t, folded)
		})
	}
}

// A temperament WORD is resolved to the numbers the rulebook says it means,
// before the composition ever sees it (design §3).
func TestATemperamentsWordIsResolvedToItsProfile(t *testing.T) {
	resolved, err := resolvedTemper(encounter.Temper{Word: string(table.TemperCoward)})

	require.NoError(t, err)
	require.Equal(t, "coward", resolved.Word)
	require.Nil(t, resolved.Mix, "a word that was authored outright is never dealt for")
	require.Equal(t, encounter.TemperProfile{
		Attack: 50, Toward: 50, Away: 300, Flee: 300, Hold: 100,
	}, resolved.Profile,
		"the design's own §3 row, in percent: halve what closes, treble what leaves")
}

// No temperament is a soldier, and a soldier is the zero profile — one state
// rather than two that have to agree.
func TestNoTemperamentIsNothingAtAll(t *testing.T) {
	resolved, err := resolvedTemper(encounter.Temper{})

	require.NoError(t, err)
	require.Equal(t, encounter.Temper{}, resolved,
		"the composition reads the zero profile as every word at 100, which IS the soldier")
}

// A faction's MIX crosses with every word's profile filled in, and the
// composition deals one from it at the door.
func TestAMixCarriesEveryWordsProfileForTheCompositionToDealFrom(t *testing.T) {
	resolved, err := resolvedTemper(encounter.Temper{
		Mix: map[string]int{"coward": 1, "soldier": 2, "aggressive": 1},
	})

	require.NoError(t, err)
	require.Empty(t, resolved.Word, "nothing is dealt HERE — this seam owns no dice")
	require.Equal(t, map[string]int{"coward": 1, "soldier": 2, "aggressive": 1}, resolved.Mix,
		"the author's shares, verbatim: they are the die's faces")
	require.Len(t, resolved.Profiles, 3,
		"and what every word in it MEANS, because the composition cannot look one up")
	require.Equal(t, 300, resolved.Profiles["aggressive"].Attack)
	require.Equal(t, 300, resolved.Profiles["coward"].Away)
	require.Equal(t, 100, resolved.Profiles["soldier"].Hold)
}

// A word this rulebook does not know REFUSES, on a placement and inside a mix
// alike. Answering "solider" with a soldier's profile is the silent degrade
// the whole slice exists to avoid.
func TestAnUnknownTemperamentRefuses(t *testing.T) {
	tests := []struct {
		name string
		in   encounter.Temper
	}{
		{name: "a mistyped word on the placement", in: encounter.Temper{Word: "solider"}},
		{name: "a fourth temperament nobody designed", in: encounter.Temper{Word: "brave"}},
		{
			name: "a mistyped word inside a faction's mix",
			in:   encounter.Temper{Mix: map[string]int{"soldier": 2, "cowardly": 1}},
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			resolved, err := resolvedTemper(tc.in)

			require.Error(t, err, "a placement that played perfectly well would never teach anybody the word never landed")
			require.Equal(t, encounter.Temper{}, resolved,
				"and the zero value it returns is NOT a soldier to be used anyway")
		})
	}
}

// THE TWO SEALED LISTS AGREE, and this is the only place that can check it.
//
// The temperament words are sealed twice on purpose: the rulebook ships what
// they MEAN (percent profiles, content a walk tunes) and the composition
// refuses `temper: brave` on the author's form before a table is ever rolled.
// Neither module may import the other — the composition may not import a
// rulebook (its C1) — so the two lists cannot be derived from one another.
//
// This module imports both, which makes it the agreement. A word added to one
// and not the other fails here, which is the cheapest place it can fail: the
// alternative is an author writing a word the dialect accepts and the rulebook
// cannot price, discovered on somebody's turn.
func TestTheTemperVocabulariesAgree(t *testing.T) {
	fromComposition := append([]string(nil), encounter.TemperWords...)
	sort.Strings(fromComposition)

	fromRulebook := []string{
		string(table.TemperSoldier), string(table.TemperCoward), string(table.TemperAggressive),
	}
	sort.Strings(fromRulebook)

	require.Equal(t, fromRulebook, fromComposition,
		"the words the dialect refuses by and the words the rulebook prices are one vocabulary")

	for _, word := range encounter.TemperWords {
		profile, err := profileOf(word)
		require.NoError(t, err, "%s: the composition accepts this word, so the rulebook must price it", word)
		require.NotEqual(t, encounter.TemperProfile{}, profile,
			"%s: an all-zero profile would silence every word on the creature's table", word)
		require.True(t, encounter.ValidTemperWord(word))
	}

	require.False(t, encounter.ValidTemperWord("brave"), "and the seal holds from the other side")
	_, err := profileOf("brave")
	require.Error(t, err)
}

// shippedDefault is the rulebook's own table for a goblin, compiled — the
// thing the fold is supposed to lay underneath.
//
// COMPARED AGAINST RATHER THAN COUNTED. What "the default survived entire"
// means is that it is byte-for-byte what the rulebook ships, and an entry count
// would say that badly: it passes for a table whose entries were swapped, and
// it fails for a content change that this test has no business having an
// opinion about. The rulebook is free to tune its own creature; what this test
// owns is the LAYERING.
func shippedDefault(t *testing.T) encounter.Table {
	t.Helper()

	def, err := table.Default(*refs.Monsters.Goblin())
	require.NoError(t, err)
	compiled, err := dungeonspec.CompileTable(def.Source)
	require.NoError(t, err)
	require.NotEmpty(t, compiled[encounter.AnswerTime],
		"a default that said nothing about a creature's time would make this whole test vacuous")

	return compiled
}

// keysOf is a table's keys, for an assertion about WHICH triggers a fold
// produced rather than about what is under each.
func keysOf(t encounter.Table) []encounter.AnswerKey {
	out := make([]encounter.AnswerKey, 0, len(t))
	for key := range t {
		out = append(out, key)
	}

	return out
}
