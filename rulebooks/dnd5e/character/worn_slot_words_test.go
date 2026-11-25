// Copyright (C) 2026 Kirk Diggler
// SPDX-License-Identifier: GPL-3.0-or-later

package character_test

import (
	"encoding/json"
	"slices"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/character"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/magicitems"
)

// magicitems writes its slot strings itself and may not import character, so
// this holds the two lists together from this side.
func TestAuthoredSlotWordsAreTheWornSlots(t *testing.T) {
	produced := map[string]bool{}
	for _, word := range []string{"cloak", "amulet", "ring", "belt", "helm", "boots"} {
		out, err := magicitems.Compile(&magicitems.CompileInput{
			ID:       "x",
			Authored: json.RawMessage(`{"name":"X","slot":"` + word + `","lets":[{"use":"second wind","with":{"level":1}}]}`),
		})
		require.NoError(t, err, word)
		for _, s := range out.Description.Slots {
			produced[s] = true
			assert.True(t, slices.Contains(character.WornSlots(), character.InventorySlot(s)), "slot %q is not a worn slot", s)
		}
	}
	for _, worn := range character.WornSlots() {
		assert.True(t, produced[string(worn)], "worn slot %q is produced by no slot word", worn)
	}
}
