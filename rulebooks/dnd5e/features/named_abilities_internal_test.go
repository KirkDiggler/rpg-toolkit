// Copyright (C) 2026 Kirk Diggler
// SPDX-License-Identifier: GPL-3.0-or-later

package features

import (
	"encoding/json"
	"fmt"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/KirkDiggler/rpg-toolkit/core"
	coreCombat "github.com/KirkDiggler/rpg-toolkit/core/combat"
	coreResources "github.com/KirkDiggler/rpg-toolkit/core/resources"
	dnd5eCombat "github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/combat"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/combat/actions"
	dnd5eEvents "github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/events"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/healing"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/refs"
)

var testSurgeRef = core.Ref{Module: "test", Type: "features", ID: "test_surge"}

// testSurge is a second ability that exists only for the length of one test.
func testSurge() *namedAbility {
	return &namedAbility{
		word:  "test surge",
		ref:   func() core.Ref { return testSurgeRef },
		label: "Test Surge",
		check: func(raw json.RawMessage) error {
			if len(raw) != 0 {
				return fmt.Errorf("test surge takes no values, got %s", raw)
			}
			return nil
		},
		describe: func(in *describeInput) (*actions.Definition, error) {
			return &actions.Definition{
				Ref:  testSurgeRef,
				Name: "Test Surge",
				Cost: &dnd5eCombat.SpendProfile{Pools: in.pools},
			}, nil
		},
	}
}

func testGiver() dnd5eEvents.RollSource {
	ref, err := core.ParseString("example:items:test_ring")
	if err != nil {
		panic(err)
	}
	return dnd5eEvents.RollSource{Ref: ref, Name: "Test ring", Label: "Ring level", SourceID: "ring-1"}
}

// Test 1: a second entry, added in one place, is found by all five.
func TestASecondEntryIsFoundByAllFive(t *testing.T) {
	before := NamedAbilities()
	require.Equal(t, []string{"second wind"}, before)

	remove := namedAbilityTable.add(testSurge())
	removed := false
	defer func() {
		if !removed {
			remove()
		}
	}()

	ref, ok := NamedAbility("test surge")
	require.True(t, ok)
	assert.Equal(t, testSurgeRef, ref)

	assert.Equal(t, []string{"second wind", "test surge"}, NamedAbilities(), "both, sorted")

	require.NoError(t, ValidateParameters(&ValidateParametersInput{Ref: testSurgeRef}))
	err := ValidateParameters(&ValidateParametersInput{Ref: testSurgeRef, Parameters: json.RawMessage(`{"level": 1}`)})
	require.Error(t, err)
	assert.Equal(t, `test surge takes no values, got {"level": 1}`, err.Error())

	definition, err := DescribeFromRef(&DescribeFromRefInput{Ref: testSurgeRef, Giver: testGiver()})
	require.NoError(t, err)
	assert.Equal(t, &actions.Definition{Ref: testSurgeRef, Name: "Test Surge", Cost: &dnd5eCombat.SpendProfile{}}, definition)

	words := AbilityWords(&AbilityWordsInput{}).Words
	require.Len(t, words, 2)
	assert.Equal(t, "second wind", words[0].Name)
	assert.Equal(t, "test surge", words[1].Name)
	assert.Equal(t, "Test Surge", words[1].Label)
	assert.Empty(t, words[1].Values)

	remove()
	removed = true
	assert.Equal(t, before, NamedAbilities(), "the table is as it was")
	_, ok = NamedAbility("test surge")
	assert.False(t, ok)
	assert.Error(t, ValidateParameters(&ValidateParametersInput{Ref: testSurgeRef}))
	assert.Len(t, AbilityWords(&AbilityWordsInput{}).Words, 1)
}

// Test 2: the shared refusals name the ability by its word.
func TestSharedRefusalsNameTheAbilityByItsWord(t *testing.T) {
	defer namedAbilityTable.add(testSurge())()

	giver := testGiver()
	noRef, noLabel, noSource := giver, giver, giver
	noRef.Ref = nil
	noLabel.Label = ""
	noSource.SourceID = ""
	cases := []struct {
		name  string
		giver dnd5eEvents.RollSource
		pools map[coreResources.ResourceKey]int
		want  string
	}{
		{"no ref", noRef, nil, "test surge giver requires a Ref"},
		{"no label", noLabel, nil, "test surge giver requires a Label"},
		{"no source id", noSource, nil, "test surge giver requires a SourceID"},
		{"empty pool key", giver, map[coreResources.ResourceKey]int{"": 1}, "test surge pool key is empty"},
		{"pool amount zero", giver, map[coreResources.ResourceKey]int{"charges": 0}, `test surge pool "charges" amount 0 must be at least 1`},
	}
	for _, c := range cases {
		_, err := DescribeFromRef(&DescribeFromRefInput{Ref: testSurgeRef, Giver: c.giver, Pools: c.pools})
		require.Error(t, err, c.name)
		assert.Equal(t, c.want, err.Error(), c.name)
	}
}

// Test 3: the order of refusals. The values are judged first, then the giver,
// then the pools.
func TestTheOrderOfRefusalsIsUnchanged(t *testing.T) {
	sw := *refs.Features.SecondWind()
	badGiver := dnd5eEvents.RollSource{}
	badPools := map[coreResources.ResourceKey]int{"": 1}

	_, err := DescribeFromRef(&DescribeFromRefInput{Ref: sw, Parameters: json.RawMessage(`{"level":0}`), Giver: badGiver, Pools: badPools})
	require.Error(t, err)
	assert.Equal(t, "second wind level 0 must be at least 1", err.Error(), "parameters before giver and pools")

	_, err = DescribeFromRef(&DescribeFromRefInput{Ref: sw, Parameters: json.RawMessage(`{"level":1}`), Giver: badGiver, Pools: badPools})
	require.Error(t, err)
	assert.Equal(t, "second wind giver requires a Ref", err.Error(), "giver before pools")

	_, err = DescribeFromRef(&DescribeFromRefInput{Ref: sw, Parameters: json.RawMessage(`{"level":1}`), Giver: testGiver(), Pools: badPools})
	require.Error(t, err)
	assert.Equal(t, "second wind pool key is empty", err.Error())
}

// Test 4: Second Wind's description is what it was, written out by hand.
func TestSecondWindsDescriptionIsWhatItWas(t *testing.T) {
	giver := testGiver()
	got, err := DescribeFromRef(&DescribeFromRefInput{
		Ref:        *refs.Features.SecondWind(),
		Parameters: json.RawMessage(`{"level": 4}`),
		Giver:      giver,
		Pools:      map[coreResources.ResourceKey]int{"charges": 2},
	})
	require.NoError(t, err)

	want := &actions.Definition{
		Ref:         core.Ref{Module: "dnd5e", Type: "features", ID: "second_wind"},
		Name:        "Second Wind",
		Description: "Regain 1d10 hit points plus 4",
		Cost: &dnd5eCombat.SpendProfile{
			Slots: map[coreCombat.ActionType]int{coreCombat.ActionBonus: 1},
			Pools: map[coreResources.ResourceKey]int{"charges": 2},
		},
		Effect: &actions.EffectProfile{
			To: actions.EffectSelf,
			Healing: &healing.Declaration{
				Dice:      "1d10",
				Modifiers: []healing.Modifier{{Source: giver, Amount: 4}},
			},
		},
	}
	assert.Equal(t, want, got)
}

// Test 5: the table is well formed.
func TestTheTableIsWellFormed(t *testing.T) {
	require.NotEmpty(t, namedAbilityTable.entries)
	words := map[string]bool{}
	refsSeen := map[core.Ref]bool{}
	for _, entry := range namedAbilityTable.entries {
		assert.NotEmpty(t, entry.word)
		assert.False(t, words[entry.word], "word %q appears twice", entry.word)
		words[entry.word] = true
		assert.Equal(t, strings.ToLower(entry.word), entry.word, "%q is lower case", entry.word)
		assert.Equal(t, strings.Join(strings.Fields(entry.word), " "), entry.word, "%q has single spaces", entry.word)

		require.NotNil(t, entry.ref, entry.word)
		ref := entry.ref()
		assert.False(t, refsSeen[ref], "ref %s appears twice", ref.String())
		refsSeen[ref] = true

		assert.NotEmpty(t, entry.label, entry.word)
		assert.NotNil(t, entry.check, entry.word)
		assert.NotNil(t, entry.describe, entry.word)
	}
}
