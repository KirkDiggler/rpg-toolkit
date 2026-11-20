// Copyright (C) 2026 Kirk Diggler
// SPDX-License-Identifier: GPL-3.0-or-later

package session

import (
	"context"
	"encoding/json"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/character"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/conditions"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/encounter"
	dnd5eEvents "github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/events"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/monster"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/refs"
)

// sheet_facts_memo_internal_test.go is the per-verb sheet facts memo
// (rpg-toolkit#2493): a member's sheet is read once per verb, and a sheet the
// verb writes — a hold given, or ended — is read afresh at the next ask.

// countingCharacters is a character repository that keeps what is saved and
// counts what is read.
type countingCharacters struct {
	byID  map[string]*character.Data
	reads map[string]int
}

func (c countingCharacters) GetCharacter(_ context.Context, id string) (*character.Data, error) {
	c.reads[id]++
	data, ok := c.byID[id]
	if !ok {
		return nil, ErrNotFound
	}
	copied := *data
	return &copied, nil
}

func (c countingCharacters) SaveCharacter(_ context.Context, data *character.Data) error {
	copied := *data
	c.byID[data.ID] = &copied
	return nil
}

// heldBlob is a Held status's recipient half for member: one cannot-move part.
func heldBlob(t *testing.T, member string) json.RawMessage {
	t.Helper()
	ref := refs.Conditions.LinkedEffects().String()
	config := json.RawMessage(`{"application_id":"held:` + member + `","address_source_id":"held:` + member + `","role":"recipient",` +
		`"bindings":{"name":"Held","owner":{"member_id":"hall-1","condition_ref":"` + ref + `","source_id":"held:` + member + `"},` +
		`"member_key":"` + member + `","parts":[{"key":"hold","kind":"cannot","on_terminal":"part","params":{"what":["move"]}}]}}`)
	made, err := conditions.CreateFromRef(&conditions.CreateFromRefInput{Ref: ref, MemberID: member,
		Origin: &dnd5eEvents.ConditionOrigin{Ref: *refs.Conditions.LinkedEffects(), EntityID: "hall-1"}, Config: config})
	require.NoError(t, err)
	blob, err := made.Condition.ToJSON()
	require.NoError(t, err)
	return blob
}

// Catches a memo that reads twice, or one that keeps an answer past a write
// of the sheet it came from: the first ask reads each sheet, the second reads
// none, and after the verb gives alice a hold through the store and ends the
// skeleton's through the record, each next ask reads that one sheet afresh
// and sees it.
func TestASheetIsReadOncePerVerbAndAWrittenSheetIsReadAgain(t *testing.T) {
	alice := dwarfCharacterRecord("alice", 10)
	skeleton, err := instantiate("skel-1", "dnd5e:monsters:skeleton", nil, nil)
	require.NoError(t, err)
	free := append([]json.RawMessage(nil), skeleton.Conditions...)
	skeleton.Conditions = append(skeleton.Conditions, heldBlob(t, "skel-1"))

	repo := countingCharacters{byID: map[string]*character.Data{"alice": alice}, reads: map[string]int{}}
	data := &SessionData{NPCs: []monster.Data{*skeleton}}
	scope := &writeScope{data: data, standing: standingSeam{
		ctx: context.Background(), sheets: storeOf(repo), data: data,
		kinds: map[string]encounter.MemberKind{"alice": encounter.KindPlayer, "skel-1": encounter.KindMonster},
		memo:  newParticipationMemo(), facts: newSheetFactsMemo(),
	}}
	seam := sheetsBeside(scope.standing)
	both := []encounter.MemberID{"alice", "skel-1"}
	ask := func() map[encounter.MemberID]encounter.SheetFacts {
		out, err := seam.Sheets(both)
		require.NoError(t, err)
		return out
	}

	loads := monsterSheetLoads.Load()
	first := ask()
	require.Empty(t, first["alice"].CannotMove)
	require.Equal(t, []string{"Held"}, first["skel-1"].CannotMove)
	require.Equal(t, 1, repo.reads["alice"])
	require.Equal(t, loads+1, monsterSheetLoads.Load())

	ask()
	require.Equal(t, 1, repo.reads["alice"], "a sheet already read this verb is not read again")
	require.Equal(t, loads+1, monsterSheetLoads.Load(), "nor is a monster loaded again")

	// The verb gives alice a hold: her sheet saved through the verb's store.
	held := dwarfCharacterRecord("alice", 10)
	held.Conditions = append(held.Conditions, heldBlob(t, "alice"))
	require.NoError(t, sheetStore{repo: repo, scope: scope}.save(context.Background(), held))
	// And ends the skeleton's: its sheet replaced in the record.
	ended := data.NPCs[0]
	ended.Conditions = free
	scope.replaceMonsterSheet(&ended)

	after := ask()
	require.Equal(t, []string{"Held"}, after["alice"].CannotMove, "the hold given this verb is seen")
	require.Empty(t, after["skel-1"].CannotMove, "the hold ended this verb is seen")
	require.Equal(t, 2, repo.reads["alice"], "the written sheet is read once more")
}
