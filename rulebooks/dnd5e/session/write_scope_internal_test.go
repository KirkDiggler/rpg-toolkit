// Copyright (C) 2026 Kirk Diggler
// SPDX-License-Identifier: GPL-3.0-or-later

package session

import (
	"encoding/json"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/conditions"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/encounter"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/monster"
)

// TestOnlyAWrittenMonsterIsFingerprinted pins freshness's cost (rpg-toolkit#1958
// item 16): a monster's conditions are fingerprinted when a verb first writes
// its sheet, against what it held before that write, and a monster the verb
// never writes is never read.
func TestOnlyAWrittenMonsterIsFingerprinted(t *testing.T) {
	scope := &writeScope{data: &SessionData{NPCs: []monster.Data{
		{ID: "goblin-1"}, {ID: "goblin-2"},
	}}}
	before := conditionKey(seenConditions("goblin-1", nil))

	dodging, err := (&conditions.DodgingCondition{MemberID: "goblin-1"}).ToJSON()
	require.NoError(t, err)
	scope.replaceMonsterSheet(&monster.Data{ID: "goblin-1", Conditions: []json.RawMessage{dodging}})
	scope.replaceMonsterSheet(&monster.Data{ID: "goblin-1"})

	require.Equal(t, before, scope.npcConditionsBefore["goblin-1"],
		"the fingerprint is what goblin-1 held before the verb's FIRST write, not its second")
	_, keyed := scope.npcConditionsBefore["goblin-2"]
	require.False(t, keyed, "goblin-2 was never written, so it is never fingerprinted")
	require.True(t, scope.touched, "a replaced sheet changes the session record")
}

// TestAWrittenCharacterIsTouchedWithoutReadingTheReport: the members freshness
// re-checks are carried on the scope as IDs, and the report names each
// aggregate once however many times it was saved.
func TestAWrittenCharacterIsTouchedWithoutReadingTheReport(t *testing.T) {
	scope := &writeScope{written: []string{"exploration:alice"}}

	scope.noteCharacterWritten("alice")
	scope.noteCharacterWritten("alice")

	require.Equal(t, map[encounter.MemberID]bool{"alice": true}, scope.sheetsWritten)
	require.Equal(t, []string{"exploration:alice", "character:alice"}, scope.written)
}
