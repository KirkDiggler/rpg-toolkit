// Copyright (C) 2026 Kirk Diggler
// SPDX-License-Identifier: GPL-3.0-or-later

package session

import (
	"context"
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
