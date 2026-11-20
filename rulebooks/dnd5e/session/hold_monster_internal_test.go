// Copyright (C) 2026 Kirk Diggler
// SPDX-License-Identifier: GPL-3.0-or-later

package session

import (
	"context"
	"encoding/json"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/conditions"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/encounter"
	dnd5eEvents "github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/events"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/monster"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/refs"
)

// heldOgreRecord is an ogre saved holding a Held status — one cannot-move
// part on its recipient half — as the record a session holds between verbs.
// Built without a bus: the condition is created and saved straight onto the
// record, as a give's landing leaves it.
func heldOgreRecord(t *testing.T) *monster.Data {
	t.Helper()
	ref := refs.Conditions.LinkedEffects().String()
	config := json.RawMessage(`{"application_id":"held:ogre","address_source_id":"held:ogre","role":"recipient",` +
		`"bindings":{"name":"Held","owner":{"member_id":"hall-1","condition_ref":"` + ref + `","source_id":"held:ogre"},` +
		`"member_key":"ogre-1","parts":[{"key":"hold","kind":"cannot","on_terminal":"part","params":{"what":["move"]}}]}}`)
	made, err := conditions.CreateFromRef(&conditions.CreateFromRefInput{Ref: ref, MemberID: "ogre-1",
		Origin: &dnd5eEvents.ConditionOrigin{Ref: *refs.Conditions.LinkedEffects(), EntityID: "hall-1"}, Config: config})
	require.NoError(t, err)
	blob, err := made.Condition.ToJSON()
	require.NoError(t, err)
	return &monster.Data{ID: "ogre-1", Name: "Ogre", HitPoints: 20, MaxHitPoints: 20, ArmorClass: 11,
		Conditions: []json.RawMessage{blob}}
}

// Catches the sheet fact read off a bare load (rpg-project#990): a monster
// saved while held answers held to the seam, though the same record loaded
// bare carries its conditions unapplied and holds nothing.
func TestAMonsterSavedWhileHeldIsAnsweredHeld(t *testing.T) {
	ctx := context.Background()
	record := heldOgreRecord(t)
	require.NotEmpty(t, record.Conditions, "precondition: the hold is saved")

	bare, err := monster.Load(ctx, record)
	require.NoError(t, err)
	unread, err := bare.CannotMove()
	require.NoError(t, err)
	require.False(t, unread.CannotMove, "precondition: a bare load holds nothing — the window the seam must not answer from")

	held, err := monsterCannotMove(ctx, "ogre-1", record)
	require.NoError(t, err)
	require.Equal(t, []string{"Held"}, held, "the seam answers from the conditions loaded")

	free, err := monsterCannotMove(ctx, "ogre-1", &monster.Data{ID: "ogre-1", Name: "Ogre", HitPoints: 20, MaxHitPoints: 20, ArmorClass: 11})
	require.NoError(t, err)
	require.Empty(t, free, "a monster holding nothing may move")
}

// Catches the hold dropped between the composition and a hosted driver
// (rpg-project#990): a held creature's view names what holds it, and an
// unheld one's names nothing.
func TestAHeldCreaturesViewNamesWhatHoldsIt(t *testing.T) {
	held := projectMonsterView(encounter.MonsterView{Self: "ogre-1", CannotMove: []string{"Held"}})
	require.Equal(t, []string{"Held"}, held.CannotMove)
	free := projectMonsterView(encounter.MonsterView{Self: "ogre-1"})
	require.Empty(t, free.CannotMove, "a creature nothing holds may move")
}
