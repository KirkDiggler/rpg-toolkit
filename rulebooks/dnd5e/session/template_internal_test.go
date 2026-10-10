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

// TestATemplateBaseMustBeARulebookBase: the session resolves the base the
// template names and passes exactly that to FromTemplate, which no longer
// checks the match itself. A base nothing ships, and a rulebook monster that
// is not a base, are both refused by name before assembly.
func TestATemplateBaseMustBeARulebookBase(t *testing.T) {
	for _, base := range []string{"dnd5e:monsters:elf", "dnd5e:monsters:goblin"} {
		t.Run(base, func(t *testing.T) {
			_, err := instantiate("cook-1", "dnd5e:monsters:cook", nil,
				&dungeonspec.TemplateSpec{Base: base, Actions: []string{"dnd5e:weapons:dagger"}})
			require.ErrorIs(t, err, ErrUnknownContent)
			require.Contains(t, err.Error(), base)
		})
	}
}

// castleGuard is the castle kitchen's guard block, as the dialect carries it.
var castleGuard = dungeonspec.TemplateSpec{
	Base:      "dnd5e:monsters:human",
	Abilities: map[string]int{"str": 13, "con": 12, "wis": 11},
	HitDice:   "2d8",
	Armor:     "dnd5e:armor:chain-shirt",
	Skills:    []string{"perception"},
	Actions:   []string{"dnd5e:weapons:spear"},
}

// TestDeriveTemplateEchoesTheCastleGuard is the authoring-time echo read off
// the same assembly the launch uses: every number is the rulebook's.
func TestDeriveTemplateEchoesTheCastleGuard(t *testing.T) {
	out, err := DeriveTemplate(&DeriveTemplateInput{ID: "guard", Ref: "dnd5e:monsters:guard", Spec: castleGuard})
	require.NoError(t, err)
	block := out.Block
	require.Equal(t, "dnd5e:monsters:guard", block.Ref, "the template's ref, not the human's")
	require.Equal(t, 11, block.HitPoints, "2d8 averages 9, plus CON 12's +1 per die")
	require.Equal(t, 13, block.ArmorClass, "a chain shirt is 13 plus DEX 10's +0")
	require.Equal(t, 12, block.PassivePerception, "10, WIS 11's +0, and trained perception's +2")
	require.Equal(t, 2, block.ProficiencyBonus, "the human base's")
	require.Equal(t, map[string]int{"str": 13, "dex": 10, "con": 12, "int": 10, "wis": 11, "cha": 10}, block.Abilities,
		"the author's three scores over the base's six")
	require.Equal(t, []DerivedAttack{{WeaponRef: "dnd5e:weapons:spear", AttackBonus: 3, Damage: "1d6+1"}}, block.Attacks,
		"STR 13's +1 and proficiency +2 to hit; the spear's d6 plus STR to damage")
	require.Zero(t, block.Experience, "experience never inherits")
}

// TestDeriveTemplateRefusesAnUnknownBaseByName: the author is told which base.
func TestDeriveTemplateRefusesAnUnknownBaseByName(t *testing.T) {
	spec := castleGuard
	spec.Base = "dnd5e:monsters:elf"
	_, err := DeriveTemplate(&DeriveTemplateInput{ID: "guard", Ref: "dnd5e:monsters:guard", Spec: spec})
	require.ErrorIs(t, err, ErrUnknownContent)
	require.Contains(t, err.Error(), "dnd5e:monsters:elf")
}
