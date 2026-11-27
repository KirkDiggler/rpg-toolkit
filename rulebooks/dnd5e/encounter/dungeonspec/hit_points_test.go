// Copyright (C) 2026 Kirk Diggler
// SPDX-License-Identifier: GPL-3.0-or-later

package dungeonspec_test

import (
	"encoding/json"
	"strings"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/KirkDiggler/rpg-toolkit/mind/behavior"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/encounter"
)

// hit_points_test.go is the dialect's half of `hit_points` (rpg-project#1131,
// slice 4b): a row's `when` can ask for a share of the creature's own maximum
// hit points. Modelled on dash_test.go's threat tests. What the number means is
// mind/behavior's; which shapes this build reads, and how a refusal sounds, are
// this dialect's.

const hitPointsShape = "`hit_points` takes { at_most: N }, a whole percent of the creature's own maximum hit points"

// hitPointsRow is a `time` row that holds in the one place and does nothing.
func hitPointsRow(when string) string {
	return `{ when: ` + when + `, hold: {} }`
}

// Test 1: it compiles, alone, beside another condition, and inside a `not`.
func TestAHitPointsConditionCompiles(t *testing.T) {
	t.Run("alone", func(t *testing.T) {
		rows := captainRows(t, dashingCaptain(t, hitPointsRow(`{ hit_points: { at_most: 30 } }`)))
		require.Len(t, rows, 1)
		require.Equal(t, []encounter.When{{HitPoints: &behavior.HitPointsWhen{AtMost: 30}}}, rows[0].When)
	})
	t.Run("beside another condition", func(t *testing.T) {
		rows := captainRows(t, dashingCaptain(t, hitPointsRow(`[{ enemy: reach }, { hit_points: { at_most: 100 } }]`)))
		require.Len(t, rows, 1)
		require.Equal(t, []encounter.When{
			{Enemy: encounter.EnemyReach},
			{HitPoints: &behavior.HitPointsWhen{AtMost: 100}},
		}, rows[0].When)
	})
	t.Run("inside not", func(t *testing.T) {
		rows := captainRows(t, dashingCaptain(t, hitPointsRow(`{ not: { hit_points: { at_most: 1 } } }`)))
		require.Len(t, rows, 1)
		require.Len(t, rows[0].When, 1)
		require.Nil(t, rows[0].When[0].HitPoints, "the outer condition is the not")
		require.NotNil(t, rows[0].When[0].Not)
		require.Equal(t, &behavior.HitPointsWhen{AtMost: 1}, rows[0].When[0].Not.HitPoints)
	})
}

// Test 2: the shape is refused at the row, each with its line and its sentence.
func TestAHitPointsConditionThatIsNotOneShareIsRefused(t *testing.T) {
	const share = "a share of the creature's own maximum is a whole percent from 1 to 100"
	for _, tc := range []struct {
		name string
		when string
		says []string
	}{
		{"a bare number", `hit_points: 30`, []string{hitPointsShape, "and this is not that"}},
		{"a word", `hit_points: low`, []string{hitPointsShape, "and this is not that"}},
		{"zero", `hit_points: { at_most: 0 }`, []string{"`hit_points: { at_most: 0 }`: " + share}},
		{"over a hundred", `hit_points: { at_most: 101 }`, []string{"`hit_points: { at_most: 101 }`: " + share}},
		{"a fraction", `hit_points: { at_most: 12.5 }`, []string{"`hit_points: { at_most: 12.5 }`: " + share}},
		{"a word for a number", `hit_points: { at_most: low }`, []string{"`hit_points: { at_most: low }`: " + share}},
		{"another comparison", `hit_points: { at_least: 30 }`, []string{hitPointsShape, "and this writes `at_least`"}},
		{"a second key", `hit_points: { at_most: 30, of: max }`, []string{hitPointsShape, "and this writes `of`"}},
		{"nothing in it", `hit_points: {}`, []string{hitPointsShape, "and this writes nothing"}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			defects := decodeDefects(t, dashingCaptain(t, hitPointsRow(`{ `+tc.when+` }`)))
			requireDefect(t, defects, append([]string{"line "}, tc.says...)...)
			t.Log(defects)
		})
	}
}

// Test 3: one condition per entry still holds.
func TestAHitPointsEntryIsStillOneCondition(t *testing.T) {
	requireDefect(t, decodeDefects(t, dashingCaptain(t, hitPointsRow(`{ hit_points: { at_most: 30 }, enemy: reach }`))),
		"a `when` entry is one condition, and this names 2")
}

// Test 4: the two sentences that list what a `when` can be name it.
func TestTheWhenSentencesListHitPoints(t *testing.T) {
	t.Run("not a mapping", func(t *testing.T) {
		defects := decodeDefects(t, dashingCaptain(t, `{ when: 5, hold: {} }`))
		requireDefect(t, defects, "a `when` is one of", "{ hit_points: { at_most: N } } or { not: { … } }")
		t.Log(defects)
	})
	t.Run("an unknown key", func(t *testing.T) {
		defects := decodeDefects(t, dashingCaptain(t, hitPointsRow(`{ hitpoints: { at_most: 30 } }`)))
		requireDefect(t, defects, "`hitpoints` is not a deed this build knows",
			"(and `enemy`, `heard`, `fact`, `threat`, `hit_points`, `not`)")
		t.Log(defects)
	})
}

// Test 4b: under a verdict, a `when` is refused whole, `hit_points` as any other.
func TestAHitPointsConditionIsRefusedUnderAnOutcome(t *testing.T) {
	defects := refusals(t, optionsOnGoblin(
		"          - { id: hi, say: \"Hi.\", check: [{ ability: persuasion, dc: 10 }], "+
			"success: [{ when: { hit_points: { at_most: 30 } }, say: \"Ow.\" }] }\n"))
	t.Log(defects)
	require.NotEmpty(t, defects)
	var found bool
	for _, d := range defects {
		found = found || (strings.Contains(d.Path, ".success[0].when") && strings.Contains(d.Message, "`success` is already the condition"))
	}
	require.True(t, found, "%v", defects)
}

// Test 6: a site that never says the word compiles to the same picture, with
// no key for it. The castle's picture is the guard for every newer key.
func TestACompiledSiteThatNeverAsksHitPointsHasNoHitPointsKey(t *testing.T) {
	picture := pictureOf(t, "testdata/world-builder-v4-castle.yaml")
	require.Zero(t, strings.Count(picture, `"HitPoints"`), "a condition that does not ask leaves no key")

	compiled := captainRows(t, dashingCaptain(t, hitPointsRow(`{ hit_points: { at_most: 30 } }`)))
	raw, err := json.Marshal(compiled)
	require.NoError(t, err)
	require.Contains(t, string(raw), `"HitPoints":{"AtMost":30}`)
}
