// Copyright (C) 2026 Kirk Diggler
// SPDX-License-Identifier: GPL-3.0-or-later

package magicitems_test

import (
	"encoding/json"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/KirkDiggler/rpg-toolkit/core"
	coreResources "github.com/KirkDiggler/rpg-toolkit/core/resources"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/magicitems"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/refs"
)

const cloakBlock = `{
  "name": "Lieutenant's cloak",
  "slot": "cloak",
  "charges": {"max": 1, "refills": {"on": ["short rest", "long rest"]}},
  "lets": [{"use": "second wind", "with": {"level": 1}, "cost": {"charges": 1}}]
}`

func compile(t *testing.T, id, block string) (*magicitems.CompileOutput, error) {
	t.Helper()
	return magicitems.Compile(&magicitems.CompileInput{ID: id, Authored: json.RawMessage(block)})
}

func mustCompile(t *testing.T, id, block string) magicitems.Description {
	t.Helper()
	out, err := compile(t, id, block)
	require.NoError(t, err)
	require.NotNil(t, out)
	return out.Description
}

// Test 1
func TestCompileTheCloak(t *testing.T) {
	d := mustCompile(t, "lieutenants-cloak", cloakBlock)

	assert.Equal(t, core.Ref{Module: "site", Type: "items", ID: "lieutenants-cloak"}, d.Ref)
	assert.Equal(t, "Lieutenant's cloak", d.Name)
	assert.Equal(t, []string{"cloak"}, d.Slots)
	assert.Equal(t, map[string]magicitems.Pool{
		"charges": {Maximum: 1, RestoreOn: []coreResources.ResetType{coreResources.ResetShortRest, coreResources.ResetLongRest}},
	}, d.Pools)
	require.Len(t, d.Grants, 1)
	grant := d.Grants["second-wind"]
	assert.Equal(t, *refs.Features.SecondWind(), grant.Ability)
	assert.JSONEq(t, `{"level":1}`, string(grant.Parameters))
	assert.Equal(t, "charges", grant.Pool)
	assert.Equal(t, 1, grant.Uses)

	require.NoError(t, d.Validate())
	item, err := magicitems.New("cloak-7", d)
	require.NoError(t, err)
	assert.Equal(t, map[string]int{"charges": 1}, item.Left)
}

func block(slot string) string {
	return `{"name":"X","slot":"` + slot + `","lets":[{"use":"second wind","with":{"level":1}}]}`
}

// Test 2
func TestCompileSlots(t *testing.T) {
	want := map[string][]string{
		"cloak":  {"cloak"},
		"amulet": {"amulet"},
		"ring":   {"ring_left", "ring_right"},
		"belt":   {"belt"},
		"helm":   {"helm"},
		"boots":  {"boots"},
	}
	for word, slots := range want {
		t.Run(word, func(t *testing.T) {
			assert.Equal(t, slots, mustCompile(t, "x", block(word)).Slots)
		})
	}
}

// Test 3
func TestCompileRefills(t *testing.T) {
	never := `{"name":"X","slot":"cloak","charges":{"max":2},"lets":[{"use":"second wind","with":{"level":1}}]}`
	d := mustCompile(t, "x", never)
	assert.Equal(t, magicitems.Pool{Maximum: 2}, d.Pools["charges"])
	assert.Empty(t, d.Pools["charges"].RestoreOn)

	long := `{"name":"X","slot":"cloak","charges":{"max":2,"refills":{"on":["long rest"]}},"lets":[{"use":"second wind","with":{"level":1}}]}`
	d = mustCompile(t, "x", long)
	assert.Equal(t, []coreResources.ResetType{coreResources.ResetLongRest}, d.Pools["charges"].RestoreOn)
}

// Test 4
func TestCompileNoCostCostsNothing(t *testing.T) {
	with := `{"name":"X","slot":"cloak","charges":{"max":2},"lets":[{"use":"second wind","with":{"level":1}}]}`
	for _, b := range []string{block("cloak"), with} {
		g := mustCompile(t, "x", b).Grants["second-wind"]
		assert.Empty(t, g.Pool)
		assert.Zero(t, g.Uses)
	}
}

// Test 5
func TestCompileRefusals(t *testing.T) {
	ok := `"name":"X","slot":"cloak"`
	lets := `"lets":[{"use":"second wind","with":{"level":1}}]`
	cases := []struct {
		name  string
		id    string
		block string
		words []string // each must appear in the sentence
	}{
		{"unknown top word", "c", `{` + ok + `,` + lets + `,"colour":"red"}`, []string{`"c"`, "colour"}},
		{"unknown word in charges", "c", `{` + ok + `,"charges":{"max":1,"size":2},` + lets + `}`, []string{`"c"`, "size", "charges"}},
		{"unknown word in refills", "c", `{` + ok + `,"charges":{"max":1,"refills":{"on":["long rest"],"often":1}},` + lets + `}`, []string{`"c"`, "often", "refills"}},
		{"unknown word in a lets entry", "c", `{` + ok + `,"lets":[{"use":"second wind","with":{"level":1},"when":"now"}]}`, []string{`"c"`, "when", "lets"}},
		{"no name", "c", `{"slot":"cloak",` + lets + `}`, []string{`"c"`, "name"}},
		{"empty name", "c", `{"name":"","slot":"cloak",` + lets + `}`, []string{`"c"`, "name"}},
		{"no slot", "c", `{"name":"X",` + lets + `}`, []string{`"c"`, "slot"}},
		{"slot cape", "c", `{"name":"X","slot":"cape",` + lets + `}`, []string{`"c"`, "slot", "cape", "cloak, amulet, ring, belt, helm, boots"}},
		{"max zero", "c", `{` + ok + `,"charges":{"max":0},` + lets + `}`, []string{`"c"`, "max"}},
		{"max too big", "c", `{` + ok + `,"charges":{"max":1001},` + lets + `}`, []string{`"c"`, "max"}},
		{"max fraction", "c", `{` + ok + `,"charges":{"max":1.5},` + lets + `}`, []string{`"c"`, "max", "whole number"}},
		{"no max", "c", `{` + ok + `,"charges":{},` + lets + `}`, []string{`"c"`, "max"}},
		{"empty on", "c", `{` + ok + `,"charges":{"max":1,"refills":{"on":[]}},` + lets + `}`, []string{`"c"`, "on"}},
		{"on dawn", "c", `{` + ok + `,"charges":{"max":1,"refills":{"on":["dawn"]}},` + lets + `}`, []string{`"c"`, "dawn", "short rest, long rest"}},
		{"on repeats", "c", `{` + ok + `,"charges":{"max":1,"refills":{"on":["long rest","long rest"]}},` + lets + `}`, []string{`"c"`, "on", "long rest", "more than once"}},
		{"no lets", "c", `{` + ok + `}`, []string{`"c"`, "lets"}},
		{"empty lets", "c", `{` + ok + `,"lets":[]}`, []string{`"c"`, "lets"}},
		{"entry without use", "c", `{` + ok + `,"lets":[{"with":{"level":1}}]}`, []string{`"c"`, "use"}},
		{"third wind", "c", `{` + ok + `,"lets":[{"use":"third wind","with":{"level":1}}]}`, []string{`"c"`, "use", "third wind", "second wind"}},
		{"same use twice", "c", `{` + ok + `,"lets":[{"use":"second wind","with":{"level":1}},{"use":"second wind","with":{"level":2}}]}`, []string{`"c"`, "use", "second wind", "more than once"}},
		{"cost zero", "c", `{` + ok + `,"charges":{"max":1},"lets":[{"use":"second wind","with":{"level":1},"cost":{"charges":0}}]}`, []string{`"c"`, "cost", "charges"}},
		{"cost without charges", "c", `{` + ok + `,"lets":[{"use":"second wind","with":{"level":1},"cost":{"charges":1}}]}`, []string{`"c"`, "cost", "charges"}},
		{"cost above max", "c", `{` + ok + `,"charges":{"max":1},"lets":[{"use":"second wind","with":{"level":1},"cost":{"charges":2}}]}`, []string{`"c"`, "lets[0]", "cost.charges is 2", "at most 1"}},
		{"cost names another", "c", `{` + ok + `,"charges":{"max":1},"lets":[{"use":"second wind","with":{"level":1},"cost":{"gold":1}}]}`, []string{`"c"`, "cost", "gold"}},
		{"id with space", "a b", block("cloak"), []string{`"a b"`, "id"}},
		{"empty id", "", block("cloak"), []string{"id"}},
		{"not an object", "c", `[1,2]`, []string{`"c"`, "object"}},
		{"empty authored", "c", ``, []string{`"c"`, "object"}},
		{"lets not a list", "c", `{` + ok + `,"lets":{"use":"second wind"}}`, []string{`"c"`, "lets", "list"}},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			out, err := compile(t, tc.id, tc.block)
			require.Error(t, err)
			assert.Nil(t, out)
			for _, w := range tc.words {
				assert.Contains(t, err.Error(), w)
			}
		})
	}

	t.Run("nil input", func(t *testing.T) {
		out, err := magicitems.Compile(nil)
		require.Error(t, err)
		assert.Nil(t, out)
	})
}

// Test 6
func TestCompileTheAbilityJudgesItsOwnValues(t *testing.T) {
	_, err := compile(t, "c", `{"name":"X","slot":"cloak","lets":[{"use":"second wind","with":{"level":0}}]}`)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "second wind level 0 must be at least 1")
	assert.Contains(t, err.Error(), `"c"`)
	assert.Contains(t, err.Error(), "with")

	_, err = compile(t, "c", `{"name":"X","slot":"cloak","lets":[{"use":"second wind","with":{"level":1,"bonus":2}}]}`)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "bonus")

	_, err = compile(t, "c", `{"name":"X","slot":"cloak","lets":[{"use":"second wind"}]}`)
	require.Error(t, err, "second wind requires values")
	assert.Contains(t, err.Error(), `"c"`)
}

// Test 7
func TestCompileKeepsNoReferenceToTheAuthorsBytes(t *testing.T) {
	authored := json.RawMessage(cloakBlock)
	out, err := magicitems.Compile(&magicitems.CompileInput{ID: "c", Authored: authored})
	require.NoError(t, err)
	for i := range authored {
		authored[i] = 'x'
	}
	assert.JSONEq(t, `{"level":1}`, string(out.Description.Grants["second-wind"].Parameters))
}

func TestCompileAcceptsAWholeNumberAndPassesPlainWords(t *testing.T) {
	d := mustCompile(t, "Cloak_7", `{"name":"X","slot":"helm","charges":{"max":1000},"lets":[{"use":"second wind","with":{"level":3},"cost":{"charges":1}}]}`)
	assert.Equal(t, 1000, d.Pools["charges"].Maximum)
	assert.Equal(t, "Cloak_7", d.Ref.ID)
}

func TestCompileCostEqualToMaxPasses(t *testing.T) {
	d := mustCompile(t, "x", `{"name":"X","slot":"cloak","charges":{"max":3},"lets":[{"use":"second wind","with":{"level":1},"cost":{"charges":3}}]}`)
	assert.Equal(t, 3, d.Grants["second-wind"].Uses)
}
