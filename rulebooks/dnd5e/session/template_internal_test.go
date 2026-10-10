// Copyright (C) 2026 Kirk Diggler
// SPDX-License-Identifier: GPL-3.0-or-later

package session

import (
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/encounter/dungeonspec"
)

// TestTemplateOfRefusesWhatTheCatalogueDoesNotKnow is the conversion's half
// of rpg-project#555: the dialect carries strings and checks only their
// shape, so a template naming armour, a weapon, a skill or a score nothing
// here knows is refused at launch, with the same sentinels a placement's own
// `actions:` get from arm.
func TestTemplateOfRefusesWhatTheCatalogueDoesNotKnow(t *testing.T) {
	cook := func(edit func(*dungeonspec.TemplateSpec)) dungeonspec.TemplateSpec {
		spec := dungeonspec.TemplateSpec{Base: "dnd5e:monsters:human", Actions: []string{"dnd5e:weapons:dagger"}}
		edit(&spec)
		return spec
	}
	for _, tc := range []struct {
		name string
		spec dungeonspec.TemplateSpec
		want error
		text string
	}{
		{"unknown armor", cook(func(s *dungeonspec.TemplateSpec) { s.Armor = "dnd5e:armor:mithral-coat" }),
			ErrUnknownContent, "mithral-coat"},
		{"armor that is not armor", cook(func(s *dungeonspec.TemplateSpec) { s.Armor = "dnd5e:weapons:dagger" }),
			ErrUnknownContent, "dnd5e:weapons:dagger"},
		{"malformed armor", cook(func(s *dungeonspec.TemplateSpec) { s.Armor = "chain-shirt" }),
			ErrBadRef, "chain-shirt"},
		{"unknown weapon", cook(func(s *dungeonspec.TemplateSpec) { s.Actions = []string{"dnd5e:weapons:ladle"} }),
			ErrUnknownContent, "ladle"},
		{"unknown skill", cook(func(s *dungeonspec.TemplateSpec) { s.Skills = []string{"baking"} }),
			ErrUnknownContent, "baking"},
		{"unknown ability", cook(func(s *dungeonspec.TemplateSpec) { s.Abilities = map[string]int{"luck": 12} }),
			ErrUnknownContent, "luck"},
		{"an ability spelled in capitals", cook(func(s *dungeonspec.TemplateSpec) { s.Abilities = map[string]int{"STR": 12} }),
			ErrUnknownContent, "STR"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			_, err := templateOf(tc.spec)
			require.ErrorIs(t, err, tc.want)
			require.Contains(t, err.Error(), tc.text)
		})
	}
}

// TestTemplateOfCarriesAbsenceAsAbsence is R8 at the conversion: an unstated
// armour, skill list or weapon list stays nil so the merge reads it as the
// base's, and experience is carried as written, never inherited.
func TestTemplateOfCarriesAbsenceAsAbsence(t *testing.T) {
	tmpl, err := templateOf(dungeonspec.TemplateSpec{Base: "dnd5e:monsters:human"})
	require.NoError(t, err)
	require.Nil(t, tmpl.Armor)
	require.Nil(t, tmpl.Skills)
	require.Nil(t, tmpl.Actions)
	require.Nil(t, tmpl.Abilities)
	require.Zero(t, tmpl.Proficiency)
	require.Zero(t, tmpl.Experience)
	require.Equal(t, "dnd5e:monsters:human", tmpl.Base.String())
}

// TestATemplateNamedForTheBaseShadowsIt: `human` is rulebook content too, so
// a template by that name is refused like one named `goblin`.
func TestATemplateNamedForTheBaseShadowsIt(t *testing.T) {
	_, err := instantiate("human-1", "dnd5e:monsters:human", nil,
		&dungeonspec.TemplateSpec{Base: "dnd5e:monsters:human", Actions: []string{"dnd5e:weapons:dagger"}})
	require.ErrorIs(t, err, ErrShadowedRef)
}

// TestOnlyARulebookMonsterRefNamesATemplate: the id is everything after the
// second colon, but only on the `dnd5e:monsters` route.
func TestOnlyARulebookMonsterRefNamesATemplate(t *testing.T) {
	templates := map[string]dungeonspec.TemplateSpec{"guard": {Base: "dnd5e:monsters:human"}}
	require.NotNil(t, templateFor(templates, "dnd5e:monsters:guard"))
	require.Nil(t, templateFor(templates, "homebrew:monsters:guard"))
	require.Nil(t, templateFor(templates, "dnd5e:weapons:guard"))
	require.Nil(t, templateFor(templates, "not a ref"))
	require.Nil(t, templateFor(templates, "dnd5e:monsters:cook"))
}
