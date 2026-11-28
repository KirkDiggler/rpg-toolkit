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

func cloakDescription() magicitems.Description {
	return magicitems.Description{
		Ref:   core.Ref{Module: "site", Type: "items", ID: "lieutenants-cloak"},
		Name:  "Lieutenant's cloak",
		Slots: []string{"cloak"},
		Pools: map[string]magicitems.Pool{
			"charges": {Maximum: 1, RestoreOn: []coreResources.ResetType{coreResources.ResetShortRest, coreResources.ResetLongRest}},
		},
		Grants: map[string]magicitems.Grant{
			"second-wind": {
				Ability:    *refs.Features.SecondWind(),
				Parameters: json.RawMessage(`{"level":1}`),
				Pool:       "charges",
				Uses:       1,
			},
		},
	}
}

func TestNewFillsEveryPool(t *testing.T) {
	item, err := magicitems.New("cloak-1", cloakDescription())
	require.NoError(t, err)

	assert.Equal(t, "cloak-1", item.ID)
	assert.Equal(t, map[string]int{"charges": 1}, item.Left)
	require.NoError(t, item.Validate())
}

func TestARoundTripThroughJSONGivesAnEqualItem(t *testing.T) {
	item, err := magicitems.New("cloak-1", cloakDescription())
	require.NoError(t, err)

	raw, err := json.Marshal(item)
	require.NoError(t, err)

	var back magicitems.Data
	require.NoError(t, json.Unmarshal(raw, &back))
	assert.Equal(t, item, back)
	require.NoError(t, back.Validate())
}

func TestNewCopiesWhatItIsGiven(t *testing.T) {
	d := cloakDescription()
	item, err := magicitems.New("cloak-1", d)
	require.NoError(t, err)

	d.Slots[0] = "belt"
	d.Pools["charges"] = magicitems.Pool{Maximum: 9}
	d.Grants["second-wind"].Parameters[10] = '9'

	assert.Equal(t, []string{"cloak"}, item.Description.Slots)
	assert.Equal(t, 1, item.Description.Pools["charges"].Maximum)
	assert.JSONEq(t, `{"level":1}`, string(item.Description.Grants["second-wind"].Parameters))
}

func TestCloneIsIndependent(t *testing.T) {
	item, err := magicitems.New("cloak-1", cloakDescription())
	require.NoError(t, err)

	c := item.Clone()
	c.Left["charges"] = 0
	c.Description.Slots[0] = "belt"
	c.Description.Grants["second-wind"].Parameters[10] = '9'
	c.Description.Pools["charges"].RestoreOn[0] = coreResources.ResetNever

	assert.Equal(t, 1, item.Left["charges"])
	assert.Equal(t, []string{"cloak"}, item.Description.Slots)
	assert.JSONEq(t, `{"level":1}`, string(item.Description.Grants["second-wind"].Parameters))
	assert.Equal(t, coreResources.ResetShortRest, item.Description.Pools["charges"].RestoreOn[0])
}

func TestEveryRefusalNamesTheThing(t *testing.T) {
	cases := []struct {
		name   string
		change func(d *magicitems.Description)
		says   string
	}{
		{"invalid ref", func(d *magicitems.Description) { d.Ref = core.Ref{} }, "ref"},
		{"empty name", func(d *magicitems.Description) { d.Name = "" }, "name"},
		{"no slots", func(d *magicitems.Description) { d.Slots = nil }, "slot"},
		{"empty slot", func(d *magicitems.Description) { d.Slots = []string{""} }, "slot"},
		{"repeated slot", func(d *magicitems.Description) { d.Slots = []string{"cloak", "cloak"} }, `slot "cloak"`},
		{"no grants", func(d *magicitems.Description) { d.Grants = nil }, "grant"},
		{"bad parameters", func(d *magicitems.Description) {
			g := d.Grants["second-wind"]
			g.Parameters = json.RawMessage(`{"level":0}`)
			d.Grants["second-wind"] = g
		}, "second wind level 0 must be at least 1"},
		{"unknown ability", func(d *magicitems.Description) {
			g := d.Grants["second-wind"]
			g.Ability = core.Ref{Module: "dnd5e", Type: "features", ID: "no_such_ability"}
			d.Grants["second-wind"] = g
		}, "no described ability"},
		{"undeclared pool", func(d *magicitems.Description) {
			g := d.Grants["second-wind"]
			g.Pool = "mana"
			d.Grants["second-wind"] = g
		}, `pool "mana"`},
		{"pool without uses", func(d *magicitems.Description) {
			g := d.Grants["second-wind"]
			g.Uses = 0
			d.Grants["second-wind"] = g
		}, "uses"},
		{"uses without pool", func(d *magicitems.Description) {
			g := d.Grants["second-wind"]
			g.Pool = ""
			d.Grants["second-wind"] = g
		}, "uses"},
		{"pool maximum below one", func(d *magicitems.Description) {
			d.Pools["charges"] = magicitems.Pool{Maximum: 0}
		}, `pool "charges"`},
		{"restore_on other", func(d *magicitems.Description) {
			d.Pools["charges"] = magicitems.Pool{Maximum: 1, RestoreOn: []coreResources.ResetType{coreResources.ResetDawn}}
		}, `"dawn"`},
		{"restore_on repeated", func(d *magicitems.Description) {
			d.Pools["charges"] = magicitems.Pool{Maximum: 1, RestoreOn: []coreResources.ResetType{coreResources.ResetLongRest, coreResources.ResetLongRest}}
		}, `"long_rest"`},
		{"bad pool name", func(d *magicitems.Description) {
			d.Pools["bad name"] = magicitems.Pool{Maximum: 1}
		}, `"bad name"`},
		{"empty grant name", func(d *magicitems.Description) {
			d.Grants[""] = d.Grants["second-wind"]
		}, "grant"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			d := cloakDescription()
			d = magicitems.Data{Description: d}.Clone().Description
			tc.change(&d)

			err := d.Validate()
			require.Error(t, err)
			assert.Contains(t, err.Error(), tc.says)

			_, err = magicitems.New("x", d)
			require.Error(t, err, "New refuses what Validate refuses")
		})
	}
}

func TestEveryDataRefusalNamesTheThing(t *testing.T) {
	cases := []struct {
		name   string
		change func(d *magicitems.Data)
		says   string
	}{
		{"empty id", func(d *magicitems.Data) { d.ID = "" }, "id"},
		{"invalid description", func(d *magicitems.Data) { d.Description.Name = "" }, "name"},
		{"undeclared pool in Left", func(d *magicitems.Data) { d.Left["mana"] = 1 }, `"mana"`},
		{"pool missing from Left", func(d *magicitems.Data) { d.Left = map[string]int{} }, `"charges"`},
		{"count below zero", func(d *magicitems.Data) { d.Left["charges"] = -1 }, `"charges"`},
		{"count above maximum", func(d *magicitems.Data) { d.Left["charges"] = 2 }, `"charges"`},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			item, err := magicitems.New("cloak-1", cloakDescription())
			require.NoError(t, err)
			tc.change(&item)

			err = item.Validate()
			require.Error(t, err)
			assert.Contains(t, err.Error(), tc.says)
		})
	}
}

func TestACountOfZeroIsValid(t *testing.T) {
	item, err := magicitems.New("cloak-1", cloakDescription())
	require.NoError(t, err)
	item.Left["charges"] = 0
	require.NoError(t, item.Validate())
}
