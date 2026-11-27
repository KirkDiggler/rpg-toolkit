// Copyright (C) 2026 Kirk Diggler
// SPDX-License-Identifier: GPL-3.0-or-later

package encounter

import (
	"context"
	"encoding/json"
	"errors"
	"math/rand"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/KirkDiggler/rpg-toolkit/tools/spatial"
)

// activate_row_internal_test.go: the encounter's half of the `activate` row
// (rpg-project#1131, ADR-0051, slice 3b-2). Internal because it asks the
// unexported answerWord and validateEntry, and a driver's view directly.

// Test 9: the row is named, and refused where a creature does nothing with time.
func TestAnActivateRowIsNamedAndRefusedUnderAnOption(t *testing.T) {
	row := Answer{Weight: 1, Activate: "second wind"}
	require.Equal(t, "activate", answerWord(row))
	require.NoError(t, validateEntry(AnswerTime, 0, row))

	err := validateEntry(AnswerSuccess, 0, row)
	require.Error(t, err)
	require.True(t, errors.Is(err, ErrBadAnswer))
	require.Contains(t, err.Error(), "`activate` is what a creature does with time")
}

// Test 7, the AnswerData half: the saved shape of a row (the legacy flat
// table and an option's rows) writes `activate` only when the row says it, and
// reads it back.
func TestAnActivateRowRoundTripsThroughAnswerData(t *testing.T) {
	data := answerDataFrom(Answer{Weight: 1, Activate: "second wind"})
	require.Equal(t, "second wind", data.Activate)
	raw, err := json.Marshal(data)
	require.NoError(t, err)
	require.Contains(t, string(raw), `"activate":"second wind"`)
	require.Equal(t, "second wind", answerFromData(data).Activate)

	toward, err := json.Marshal(answerDataFrom(Answer{Weight: 1, Toward: &Selector{Word: SelectorEnemy}}))
	require.NoError(t, err)
	require.NotContains(t, string(toward), "activate", "a row that activates nothing is saved as before")
}

// A copy of a row is a copy: the word survives cloneAnswer.
func TestAnActivateRowSurvivesAClone(t *testing.T) {
	require.Equal(t, "second wind", cloneAnswer(Answer{Weight: 1, Activate: "second wind"}).Activate)
}

// Test 10 of the row's own slice: A VIEW THAT LISTS NO ABILITY NEVER OFFERS THE
// ROW. The view below lists no ability, so Facts.CanActivate is empty and, by
// mind/behavior's rule, the row is absent from every roll. (It was once never
// chosen because nothing filled the fact; a host that lists abilities now
// does, see activate_turn_test.go.)
func TestAnActivateRowIsAbsentFromAViewThatListsNoAbility(t *testing.T) {
	table := Table{AnswerTime: {
		{Weight: 1, Activate: "second wind"},
		{Weight: 1, When: []When{{Enemy: EnemyWalk}}, Toward: &Selector{Word: SelectorEnemy}},
		{Weight: 1, When: []When{{Enemy: EnemyReach}}, Attack: &Selector{Word: SelectorEnemy}},
	}}
	view := MonsterView{
		Self:      "lieutenant",
		Table:     table,
		Actions:   []ActionView{{Ref: allyCrossbow, RangeFeet: 5}},
		Position:  spatial.Position{X: 1, Y: 0},
		Budget:    TurnBudget{AttacksLeft: 1, MovementFeet: 30},
		SpeedFeet: 30,
		Seen: []SeenMember{{
			ID: "alice", Opposed: true, Standing: true, DistanceCells: 21, Path: row(20),
		}},
	}
	for seed := int64(1); seed <= 20; seed++ {
		decision, err := TableDriver{Roller: seededDie{rand.New(rand.NewSource(seed))}}.Act(view) //nolint:gosec // a test's seeded die
		require.NoError(t, err, "seed %d", seed)
		require.Empty(t, decision.Pick.Answer.Activate, "seed %d: the activate row is never the pick", seed)
		require.NotNil(t, decision.Intent, "seed %d: one of the other rows, or a hold, is", seed)
	}
}

// seededDie is a die whose every roll comes from one seed.
type seededDie struct{ r *rand.Rand }

func (d seededDie) Roll(_ context.Context, size int) (int, error) { return d.r.Intn(size) + 1, nil }

func (d seededDie) RollN(ctx context.Context, count, size int) ([]int, error) {
	out := make([]int, count)
	for i := range out {
		out[i], _ = d.Roll(ctx, size)
	}

	return out, nil
}
