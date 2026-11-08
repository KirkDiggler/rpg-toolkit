// Copyright (C) 2026 Kirk Diggler
// SPDX-License-Identifier: GPL-3.0-or-later

package session

import (
	"context"
	"encoding/json"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/character"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/encounter"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/monster"
)

// TestAMonsterSheetWrittenMidVerbIsReadAtTheNextAssessment holds the law the
// verb's participation memo must keep (rpg-project#892): an answer kept for a
// monster is forgotten when the verb writes that monster's sheet, so the next
// assessment attaches the written sheet and answers from it. Without the
// forget in writeScope.replaceMonsterSheet the guard stays up after the blow
// that dropped it.
func TestAMonsterSheetWrittenMidVerbIsReadAtTheNextAssessment(t *testing.T) {
	counted := CountParticipationAttachesForTest(t)
	guard, err := instantiate("guard", "dnd5e:monsters:skeleton", nil, nil)
	require.NoError(t, err)
	other, err := instantiate("other", "dnd5e:monsters:skeleton", nil, nil)
	require.NoError(t, err)

	data := &SessionData{NPCs: []monster.Data{*guard, *other}}
	scope := &writeScope{data: data, standing: standingSeam{
		ctx:    context.Background(),
		sheets: storeOf(participationCharacterStore{byID: map[string]*character.Data{}}),
		data:   data,
		kinds:  map[string]encounter.MemberKind{"guard": encounter.KindMonster, "other": encounter.KindMonster},
		memo:   newParticipationMemo(),
	}}
	both := []encounter.MemberID{"guard", "other"}

	down, err := standingSet(scope.standing, both)
	require.NoError(t, err)
	assert.False(t, down["guard"], "precondition: the guard is up")
	_, attached := counted()

	// Asked again with nothing written: nothing is attached again.
	_, err = standingSet(scope.standing, both)
	require.NoError(t, err)
	_, again := counted()
	assert.Equal(t, attached, again, "an unwritten sheet is not attached a second time in one verb")

	dropped := data.NPCs[0]
	dropped.HitPoints = 0
	scope.replaceMonsterSheet(&dropped)

	down, err = standingSet(scope.standing, both)
	require.NoError(t, err)
	assert.True(t, down["guard"], "the written sheet is what the next assessment reads")
	assert.False(t, down["other"])
	_, after := counted()
	assert.Equal(t, again+1, after, "only the written sheet is attached again")
}

// TestAKeptCompulsionStillDrivesAndAWrittenOneIsSeen: a commanded monster
// asked about twice in one verb, as a raise asks, is Driven both times — the
// second answer comes from the memo, so a kept compulsion that read false
// would hand the guard back its own turn. A write that lifts the compulsion
// is seen at the next assessment.
func TestAKeptCompulsionStillDrivesAndAWrittenOneIsSeen(t *testing.T) {
	counted := CountParticipationAttachesForTest(t)
	guard, err := instantiate("guard", "dnd5e:monsters:skeleton", nil, nil)
	require.NoError(t, err)
	free := append([]json.RawMessage(nil), guard.Conditions...)
	guard.Conditions = append(guard.Conditions, commandedBlob(t, "guard", "bard", "approach"))

	data := &SessionData{NPCs: []monster.Data{*guard}}
	scope := &writeScope{data: data, standing: standingSeam{
		ctx:    context.Background(),
		sheets: storeOf(participationCharacterStore{byID: map[string]*character.Data{}}),
		data:   data,
		kinds:  map[string]encounter.MemberKind{"guard": encounter.KindMonster},
		memo:   newParticipationMemo(),
	}}
	turnOf := func() encounter.TurnParticipation {
		t.Helper()
		assessment, err := scope.standing.Assess([]encounter.MemberID{"guard"})
		require.NoError(t, err)
		for _, member := range assessment.Members {
			if member.Member == "guard" {
				return member.Turn
			}
		}
		t.Fatal("the guard is not in the assessment")
		return ""
	}

	assert.Equal(t, encounter.TurnParticipationDriven, turnOf(), "precondition: the commanded guard is driven")
	_, first := counted()
	assert.Equal(t, encounter.TurnParticipationDriven, turnOf(), "the kept answer still carries the compulsion")
	_, second := counted()
	require.Equal(t, first, second, "precondition: the second answer came from the memo")

	lifted := data.NPCs[0]
	lifted.Conditions = free
	scope.replaceMonsterSheet(&lifted)
	assert.Equal(t, encounter.TurnParticipationWait, turnOf(), "a write that lifts the compulsion is seen")
}
