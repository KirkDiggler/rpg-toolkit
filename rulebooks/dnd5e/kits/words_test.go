// Copyright (C) 2026 Kirk Diggler
// SPDX-License-Identifier: GPL-3.0-or-later

package kits_test

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/kits"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/magicitems"
)

// Test 6: the kit's words are the item's kit rows, in the same order.
func TestTheKitsWordsAreTheItemsRowsThatAreNotTheItems(t *testing.T) {
	var kitRows []kits.Word
	for _, w := range magicitems.Words(&magicitems.WordsInput{}).Words {
		if w.Place != magicitems.PlaceItem {
			kitRows = append(kitRows, w)
		}
	}
	require.NotEmpty(t, kitRows)
	assert.Equal(t, kitRows, kits.Words(&kits.WordsInput{}).Words)
}

func TestTheItemsWordsStartWithTheItemsOwnFiveThenTheKits(t *testing.T) {
	var got []string
	for _, w := range magicitems.Words(&magicitems.WordsInput{}).Words {
		got = append(got, string(w.Place)+" "+w.Word)
	}
	assert.Equal(t, []string{
		"item name", "item slot", "item unique", "item charges", "item lets",
		"charges max", "charges refills", "refills on",
		"lets entry use", "lets entry with", "lets entry cost", "cost charges",
	}, got)
}

func TestKitsWordsHandsOutItsOwnCopy(t *testing.T) {
	first := kits.Words(&kits.WordsInput{})
	first.Words[0].Label = "changed"
	first.Words[0].Bounds.Max = 1
	first.Words[2].Values[0].Word = "changed"

	again := kits.Words(&kits.WordsInput{})
	assert.Equal(t, "holds {value} charges", again.Words[0].Label)
	assert.Equal(t, 1000, again.Words[0].Bounds.Max)
	assert.Equal(t, "short rest", again.Words[2].Values[0].Word)
}
