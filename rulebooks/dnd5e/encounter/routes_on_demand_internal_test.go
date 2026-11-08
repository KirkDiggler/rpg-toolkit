// Copyright (C) 2026 Kirk Diggler
// SPDX-License-Identifier: GPL-3.0-or-later

package encounter

import (
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/KirkDiggler/rpg-toolkit/mind/behavior"
	"github.com/KirkDiggler/rpg-toolkit/tools/spatial"
)

// TestTableReadsRoutes pins which authored rows can read a route off the view
// (rpg-project#879 S3): `toward` or `dash` naming a member by a word, an
// `enemy` band measured in steps, or either inside a bundle. A hold, a route
// of cells, a sound, an attack, an away, a search and `enemy: none` cannot.
func TestTableReadsRoutes(t *testing.T) {
	cell := Selector{At: &[]spatial.Position{{X: 1, Y: 1}}[0]}
	for _, tc := range []struct {
		name  string
		rows  []Answer
		reads bool
	}{
		{"hold", []Answer{{Hold: true}}, false},
		{"toward a cell", []Answer{{Toward: &cell}}, false},
		{"toward a route", []Answer{{Toward: &Selector{Path: []spatial.Position{{X: 1}, {X: 2}}}}}, false},
		{"enemy none, toward a cell", []Answer{{When: []When{{Enemy: EnemyNone}}, Toward: &cell}}, false},
		{"enemy seen, hold", []Answer{{When: []When{{Enemy: EnemySeen}}, Hold: true}}, false},
		{"toward an ally", []Answer{{Toward: &Selector{Word: SelectorAlly}}}, true},
		{"dash at the nearest enemy", []Answer{{Dash: &Selector{Word: SelectorEnemy}}}, true},
		{"toward a sound", []Answer{{Toward: &Selector{Word: SelectorSound}}}, false},
		{"away from the nearest enemy", []Answer{{Away: &Selector{Word: SelectorEnemy}}}, false},
		{"attack the nearest enemy", []Answer{{Attack: &Selector{Word: SelectorEnemy}}}, false},
		{"search", []Answer{{Search: &behavior.Search{}}}, false},
		{"enemy walk", []Answer{{When: []When{{Enemy: EnemyWalk}}, Hold: true}}, true},
		{"not enemy far", []Answer{{When: []When{{Not: &When{Enemy: EnemyFar}}}, Hold: true}}, true},
		{"inside a bundle", []Answer{{Rows: []Answer{{Hold: true}, {Toward: &Selector{Word: SelectorAlly}}}}}, true},
	} {
		t.Run(tc.name, func(t *testing.T) { require.Equal(t, tc.reads, tableReadsRoutes(tc.rows)) })
	}
}

// Canonical policy must keep the route-on-demand optimization: exact-cell
// duties need no per-sighting route; any attachment that reads one must get
// it, including a lower-priority role. No policy keeps opaque host routes.
func TestAViewSearchesRoutesOnlyForATableThatReadsThem(t *testing.T) {
	enc := alliesEnc(t, StanceAllied)
	veteran := enc.members["veteran"]
	budget := TurnBudget{AttacksLeft: 1, MovementFeet: 30}

	require.NoError(t, enc.useBehaviors(veteran, ordinaryBehavior(Table{AnswerTime: {{Weight: 1, Hold: true}}}), nil, nil))
	view, err := enc.buildMonsterView(veteran, budget, 0)
	require.NoError(t, err)
	require.Empty(t, seenOf(t, view, "hero").Path, "a table that only holds reads no route")
	require.Empty(t, seenOf(t, view, "hero").AwayPath)

	require.NoError(t, enc.useBehaviors(veteran, ordinaryBehavior(follower()), nil, nil))
	view, err = enc.buildMonsterView(veteran, budget, 0)
	require.NoError(t, err)
	require.NotEmpty(t, seenOf(t, view, "hero").Path, "a table that walks toward an ally reads its route")

	// Selection has not happened while the view is built. Union the tables
	// conservatively rather than pre-running the composer or dropping a role.
	quiet := ordinaryBehavior(Table{AnswerTime: {{Weight: 1, Hold: true}}}).Instances[0]
	quiet.Instance.ID = "quiet"
	following := ordinaryBehavior(follower()).Instances[0]
	following.Instance.ID = "following"
	require.NoError(t, enc.useBehaviors(veteran, &BehaviorInput{Instances: []BoundBehavior{quiet, following}}, nil, nil))
	view, err = enc.buildMonsterView(veteran, budget, 0)
	require.NoError(t, err)
	require.NotEmpty(t, seenOf(t, view, "hero").Path, "a lower-priority attachment's route is still supplied")

	require.NoError(t, enc.useBehaviors(veteran, ordinaryBehavior(Table{AnswerTime: {{Weight: 1, Toward: &Selector{At: &spatial.Position{X: 5, Y: 1}}}}}), nil, nil))
	view, err = enc.buildMonsterView(veteran, budget, 0)
	require.NoError(t, err)
	require.Empty(t, seenOf(t, view, "hero").Path, "an authored exact-cell leg routes only when executed")

	require.NoError(t, enc.useBehaviors(veteran, nil, nil, nil))
	view, err = enc.buildMonsterView(veteran, budget, 0)
	require.NoError(t, err)
	require.NotEmpty(t, seenOf(t, view, "hero").Path, "no table: a driver this package cannot see may read it")
}
