// Copyright (C) 2026 Kirk Diggler
// SPDX-License-Identifier: GPL-3.0-or-later

package monster_test

import (
	"encoding/json"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/KirkDiggler/rpg-toolkit/core"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/monster"
)

// Test 8.
func TestAStatBlocksWordsAreJudged(t *testing.T) {
	cases := map[string]string{
		"an entry with a cost (no shared charges)": `[{"use":"second wind","with":{"level":1},"cost":{"charges":1}}]`,
		"an unknown ability":                       `[{"use":"fireball"}]`,
		"a uses.max of 0":                          `[{"use":"second wind","with":{"level":1},"uses":{"max":0}}]`,
		"a rest that is not one":                   `[{"use":"second wind","with":{"level":1},"uses":{"max":1,"refills":{"on":["nap"]}}}]`,
	}
	for name, lets := range cases {
		t.Run(name, func(t *testing.T) {
			checked := monster.CheckLets(&monster.CheckLetsInput{ID: "guard", Lets: json.RawMessage(lets)})
			require.Error(t, checked)
			assert.Contains(t, checked.Error(), `stat block "guard"`)
			assert.Regexp(t, `^stat block "guard"`, checked.Error())

			in := humanOver(lets)
			in.Ref = guardRef()
			built, err := monster.FromTemplate(&in)
			require.Error(t, err)
			assert.Nil(t, built)
			assert.Regexp(t, `^stat block "guard"`, err.Error())
			assert.Equal(t, checked.Error(), err.Error(), "one judge, one sentence")
		})
	}
}

func TestAnAbsentAndAnEmptyLetsBuildACreatureWithNoKit(t *testing.T) {
	for name, lets := range map[string]json.RawMessage{"absent": nil, "empty": json.RawMessage(`[]`)} {
		t.Run(name, func(t *testing.T) {
			require.NoError(t, monster.CheckLets(&monster.CheckLetsInput{ID: "guard", Lets: lets}))

			in := humanOver("")
			in.Template.Lets = lets
			m, err := monster.FromTemplate(&in)
			require.NoError(t, err)
			assert.Nil(t, m.ToData().Kit)
			assert.Nil(t, m.ToData().Left)
			assert.Empty(t, m.Grants())
		})
	}
}

// Test 9.
func TestMergeTakesLetsWholeOrTheBases(t *testing.T) {
	base := monster.Template{Lets: json.RawMessage(`[{"use":"second wind","with":{"level":1}}]`)}
	stated := monster.Template{Lets: json.RawMessage(`[{"use":"second wind","with":{"level":2}}]`)}

	assert.JSONEq(t, string(stated.Lets), string(stated.Merge(base).Lets), "a stated list replaces the base's whole")
	assert.JSONEq(t, string(base.Lets), string(monster.Template{}.Merge(base).Lets), "an unstated one is the base's")
	assert.Empty(t, monster.Template{}.Merge(monster.Template{}).Lets)
	assert.Equal(t, "[]", string(monster.Template{Lets: json.RawMessage(`[]`)}.Merge(base).Lets), "an empty list is stated")

	merged := stated.Merge(base)
	merged.Lets[0] = '{'
	merged.Lets[1] = '}'
	assert.JSONEq(t, `[{"use":"second wind","with":{"level":2}}]`, string(stated.Lets), "shares no bytes with the template")
	assert.JSONEq(t, `[{"use":"second wind","with":{"level":1}}]`, string(base.Lets), "or with the base")

	fromBase := monster.Template{}.Merge(base)
	fromBase.Lets[0] = '{'
	assert.JSONEq(t, `[{"use":"second wind","with":{"level":1}}]`, string(base.Lets))
}

func guardRef() *core.Ref { return &core.Ref{Module: "dnd5e", Type: "monsters", ID: "guard"} }
