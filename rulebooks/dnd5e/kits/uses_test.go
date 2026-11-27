// Copyright (C) 2026 Kirk Diggler
// SPDX-License-Identifier: GPL-3.0-or-later

package kits_test

import (
	"encoding/json"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	coreResources "github.com/KirkDiggler/rpg-toolkit/core/resources"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/kits"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/magicitems"
)

const letsOwn = `[{"use": "second wind", "with": {"level": 1}, "uses": {"max": 1, "refills": {"on": ["short rest", "long rest"]}}}]`

// Test 1: the three rows, where Design 1 puts them.
func TestTheUsesWordsAreTheLastRowOfTheirGroupsAndEveryOlderRowStaysPut(t *testing.T) {
	words := kits.Words(&kits.WordsInput{}).Words
	var keys []string
	for _, w := range words {
		keys = append(keys, string(w.Place)+" "+w.Word)
	}
	assert.Equal(t, []string{
		"charges max", "charges refills", "refills on",
		"lets entry use", "lets entry with", "lets entry cost", "lets entry uses",
		"cost charges", "uses max", "uses refills",
	}, keys)

	byKey := map[string]kits.Word{}
	for _, w := range words {
		byKey[string(w.Place)+" "+w.Word] = w
	}
	entry := byKey["lets entry uses"]
	assert.Equal(t, kits.ShapeEntries, entry.Shape)
	assert.Equal(t, kits.PlaceUses, entry.Holds)
	assert.False(t, entry.Required)
	assert.Equal(t, "how many times it can be used before it comes back", entry.Label)

	limit := byKey["uses max"]
	assert.Equal(t, kits.ShapeWholeNumber, limit.Shape)
	assert.True(t, limit.Required)
	assert.Equal(t, &kits.Bounds{Min: 1, Max: 1000}, limit.Bounds)
	assert.Equal(t, "can be used {value} times", limit.Label)

	refills := byKey["uses refills"]
	assert.Equal(t, kits.ShapeEntries, refills.Shape)
	assert.Equal(t, kits.PlaceRefills, refills.Holds)
	assert.False(t, refills.Required)
	assert.Equal(t, "when its uses come back", refills.Label)

	// An item publishes them too, after its own five and the kit's.
	itemWords := magicitems.Words(&magicitems.WordsInput{}).Words
	assert.Equal(t, words, itemWords[len(itemWords)-len(words):])
}

// Test 2: an entry's own count is a pool of its own, named by the grant.
func TestAnEntryWithItsOwnUsesCompilesToAPoolOfItsOwnAndAGrantPayingOneFromIt(t *testing.T) {
	out, err := kits.Compile(&kits.CompileInput{Holder: statBlockHolder, Lets: json.RawMessage(letsOwn)})
	require.NoError(t, err)

	assert.Equal(t, map[string]kits.Pool{"second-wind": {
		Maximum: 1, RestoreOn: []coreResources.ResetType{coreResources.ResetShortRest, coreResources.ResetLongRest},
	}}, out.Kit.Pools)
	require.Contains(t, out.Kit.Grants, "second-wind")
	assert.Equal(t, "second-wind", out.Kit.Grants["second-wind"].Pool)
	assert.Equal(t, 1, out.Kit.Grants["second-wind"].Uses)
	assert.NoError(t, out.Kit.Validate(statBlockHolder))

	t.Run("no refills never comes back", func(t *testing.T) {
		out, err := kits.Compile(&kits.CompileInput{Holder: statBlockHolder,
			Lets: json.RawMessage(`[{"use": "second wind", "with": {"level": 1}, "uses": {"max": 3}}]`)})
		require.NoError(t, err)
		assert.Equal(t, kits.Pool{Maximum: 3}, out.Kit.Pools["second-wind"])
	})
	t.Run("an item reads the same words the same way", func(t *testing.T) {
		item, err := magicitems.Compile(&magicitems.CompileInput{ID: "it",
			Authored: json.RawMessage(`{"name": "N", "slot": "cloak", "lets": ` + letsOwn + `}`)})
		require.NoError(t, err)
		assert.Equal(t, out.Kit.Pools, item.Description.Pools)
		assert.Equal(t, out.Kit.Grants, item.Description.Grants)
	})
}

// Test 3: two counts in one kit. Only one ability is named today and an
// ability may appear once, so two entries cannot be written; the shared
// `charges` beside an entry's own uses is the two-counts case that can.
func TestTheSharedChargesAndAnEntrysOwnUsesAreTwoCountsEachGrantNamingItsOwn(t *testing.T) {
	out, err := kits.Compile(&kits.CompileInput{
		Holder:  statBlockHolder,
		Charges: json.RawMessage(charges),
		Lets:    json.RawMessage(letsOwn),
	})
	require.NoError(t, err)

	require.Len(t, out.Kit.Pools, 2)
	assert.Equal(t, 2, out.Kit.Pools["charges"].Maximum)
	assert.Equal(t, 1, out.Kit.Pools["second-wind"].Maximum)
	assert.Equal(t, "second-wind", out.Kit.Grants["second-wind"].Pool, "it is paid from its own count, not the shared one")
	assert.NoError(t, out.Kit.Validate(statBlockHolder))

	_, err = kits.Compile(&kits.CompileInput{Holder: statBlockHolder, Lets: json.RawMessage(
		`[{"use": "second wind", "with": {"level": 1}, "uses": {"max": 1}}, {"use": "second wind", "with": {"level": 2}, "uses": {"max": 1}}]`)})
	require.Error(t, err, "one ability appears once, so two own counts cannot be written yet")
}

// Test 6: neither cost nor uses is unlimited, as it was.
func TestAnEntryWithNeitherCostNorUsesIsUnlimited(t *testing.T) {
	out, err := kits.Compile(&kits.CompileInput{Holder: statBlockHolder,
		Charges: json.RawMessage(charges),
		Lets:    json.RawMessage(`[{"use": "second wind", "with": {"level": 1}}]`)})
	require.NoError(t, err)
	assert.Empty(t, out.Kit.Grants["second-wind"].Pool)
	assert.Zero(t, out.Kit.Grants["second-wind"].Uses)
	assert.Len(t, out.Kit.Pools, 1, "no pool is made for it")
}
