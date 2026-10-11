// Copyright (C) 2026 Kirk Diggler
// SPDX-License-Identifier: GPL-3.0-or-later

package conditions

import (
	"encoding/json"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/damage"
	dnd5eEvents "github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/events"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/refs"
)

func blobOf(t *testing.T, condition dnd5eEvents.ConditionBehavior) json.RawMessage {
	t.Helper()
	data, err := condition.ToJSON()
	require.NoError(t, err)
	return data
}

// TestARetiredBlobIsNotHeldAndDoesNotFail: a sheet that still carries a blob of
// a type that no longer exists reports what it does hold, with no error. The
// sheet loaders drop the same blobs, so the reader agrees with them. What is
// left is Prone and the free reaction every combatant carries by existing.
func TestARetiredBlobIsNotHeldAndDoesNotFail(t *testing.T) {
	prone := blobOf(t, NewProneCondition("gob"))
	for name, retired := range map[string]json.RawMessage{
		"in fog":             json.RawMessage(`{"ref":{"module":"dnd5e","type":"conditions","id":"in_fog"},"member_id":"gob"}`),
		"unarmored movement": json.RawMessage(`{"ref":{"module":"dnd5e","type":"conditions","id":"unarmored_movement"},"member_id":"gob","monk_level":3}`),
		"shield spell":       json.RawMessage(`{"ref":{"module":"dnd5e","type":"spells","id":"shield"},"member_id":"gob"}`),
	} {
		t.Run(name, func(t *testing.T) {
			held, err := HeldAddresses("gob", []json.RawMessage{retired, prone})
			require.NoError(t, err)
			require.Equal(t, []dnd5eEvents.ConditionAddress{
				{MemberID: "gob", ConditionRef: refs.Conditions.Prone().String()},
				oaOf("gob"),
			}, held, "Prone, and nothing for the retired blob")
		})
	}
}

// TestStoredTraitsAreHeld: a monster's stat-block traits are conditions, so
// the sheet's holdings list them at their own refs.
func TestStoredTraitsAreHeld(t *testing.T) {
	held, err := HeldAddresses("skeleton", []json.RawMessage{
		blobOf(t, Immunity("skeleton", damage.Poison)),
		blobOf(t, Vulnerability("skeleton", damage.Bludgeoning)),
		blobOf(t, NewOpportunityAttackCondition("skeleton")),
	})
	require.NoError(t, err)
	require.Equal(t, []dnd5eEvents.ConditionAddress{
		{MemberID: "skeleton", ConditionRef: refs.MonsterTraits.Immunity().String()},
		{MemberID: "skeleton", ConditionRef: refs.MonsterTraits.Vulnerability().String()},
		oaOf("skeleton"),
	}, held)
}
