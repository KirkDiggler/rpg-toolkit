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

// removedTypeBlobs are saved conditions of types the game no longer has. None
// is registered, so each is an unknown ref like any other.
var removedTypeBlobs = map[string]json.RawMessage{
	"in fog":             json.RawMessage(`{"ref":{"module":"dnd5e","type":"conditions","id":"in_fog"},"member_id":"gob"}`),
	"unarmored movement": json.RawMessage(`{"ref":{"module":"dnd5e","type":"conditions","id":"unarmored_movement"},"member_id":"gob","monk_level":3}`),
	"shield spell":       json.RawMessage(`{"ref":{"module":"dnd5e","type":"spells","id":"shield"},"member_id":"gob"}`),
}

// TestARemovedTypesBlobFailsLoadJSONAndHeldAddressesWithErrUnknownCondition: a
// stored blob naming a type the game removed is not skipped by any reader. It
// is an unregistered ref, and the readers refuse it as they refuse any other.
func TestARemovedTypesBlobFailsLoadJSONAndHeldAddressesWithErrUnknownCondition(t *testing.T) {
	prone := blobOf(t, NewProneCondition("gob"))
	for name, removed := range removedTypeBlobs {
		t.Run(name, func(t *testing.T) {
			_, err := LoadJSON(removed)
			require.ErrorIs(t, err, ErrUnknownCondition)

			_, err = HeldAddresses("gob", []json.RawMessage{removed, prone})
			require.ErrorIs(t, err, ErrUnknownCondition)
		})
	}
}

// TestHeldAddressesAddsNothingTheStoredListLacks: the reader answers the
// stored record alone. A list that does not name the opportunity attack holds
// none; one that names it holds it once.
func TestHeldAddressesAddsNothingTheStoredListLacks(t *testing.T) {
	prone := blobOf(t, NewProneCondition("gob"))

	without, err := HeldAddresses("gob", []json.RawMessage{prone})
	require.NoError(t, err)
	require.NotContains(t, without, oaOf("gob"))
	require.Contains(t, without, dnd5eEvents.ConditionAddress{
		MemberID: "gob", ConditionRef: refs.Conditions.Prone().String(),
	})

	with, err := HeldAddresses("gob", []json.RawMessage{prone, blobOf(t, NewOpportunityAttackCondition("gob"))})
	require.NoError(t, err)
	once := 0
	for _, address := range with {
		if address == oaOf("gob") {
			once++
		}
	}
	require.Equal(t, 1, once, "the stored opportunity attack is held exactly once")
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
