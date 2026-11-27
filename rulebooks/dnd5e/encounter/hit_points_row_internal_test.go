// Copyright (C) 2026 Kirk Diggler
// SPDX-License-Identifier: GPL-3.0-or-later

package encounter

import (
	"encoding/json"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/KirkDiggler/rpg-toolkit/mind/behavior"
)

// hit_points_row_internal_test.go: the encounter's half of a row's `hit_points`
// condition (rpg-project#1131, slice 4b). Internal because it asks the
// unexported whenShorthand, cloneWhen, whenDataFrom and whenFromData.

func hurtAt(n int) When { return When{HitPoints: &behavior.HitPointsWhen{AtMost: n}} }

// Test 5, the WhenData half: saved as `hit_points`, alone and inside `not`,
// read back, and a condition that does not ask saves no key.
func TestAHitPointsConditionRoundTripsThroughWhenData(t *testing.T) {
	raw, err := json.Marshal(whenDataFrom(hurtAt(30)))
	require.NoError(t, err)
	require.Contains(t, string(raw), `"hit_points":{"at_most":30}`)
	require.Equal(t, hurtAt(30), whenFromData(whenDataFrom(hurtAt(30))))

	inner := hurtAt(30)
	not := When{Not: &inner}
	raw, err = json.Marshal(whenDataFrom(not))
	require.NoError(t, err)
	require.Contains(t, string(raw), `"not":{"hit_points":{"at_most":30}}`)
	back := whenFromData(whenDataFrom(not))
	require.NotNil(t, back.Not)
	require.Equal(t, 30, back.Not.HitPoints.AtMost)

	reach, err := json.Marshal(whenDataFrom(When{Enemy: EnemyReach}))
	require.NoError(t, err)
	require.NotContains(t, string(reach), "hit_points", "a condition that does not ask is saved as before")
}

// Test 7: said beside the row on the record.
func TestAHitPointsConditionIsSaidBesideItsRow(t *testing.T) {
	w := hurtAt(30)
	require.Equal(t, "hit points at most 30%", whenShorthand(&w))
	not := When{Not: &w}
	require.Equal(t, "not hit points at most 30%", whenShorthand(&not))
}

// Test 8: a clone is its own.
func TestAClonedHitPointsConditionIsItsOwn(t *testing.T) {
	table := Table{AnswerTime: {{Weight: 1, When: []When{hurtAt(30)}, Hold: true}}}
	clone := cloneTable(table)
	clone[AnswerTime][0].When[0].HitPoints.AtMost = 50
	require.Equal(t, 30, table[AnswerTime][0].When[0].HitPoints.AtMost, "the original still says 30")

	inner := hurtAt(30)
	copied := cloneWhen(When{Not: &inner})
	copied.Not.HitPoints.AtMost = 10
	require.Equal(t, 30, inner.HitPoints.AtMost)
}
