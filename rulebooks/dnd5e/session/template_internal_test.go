// Copyright (C) 2026 Kirk Diggler
// SPDX-License-Identifier: GPL-3.0-or-later

package session

import (
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/abilities"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/armor"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/encounter/dungeonspec"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/monster"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/refs"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/skills"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/weapons"
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
			_, err := templateOf("cook", tc.spec)
			require.ErrorIs(t, err, tc.want)
			require.Contains(t, err.Error(), tc.text)
		})
	}
}

// TestTemplateOfCarriesAbsenceAsAbsence is R8 at the conversion: an unstated
// armour, skill list or weapon list stays nil so the merge reads it as the
// base's, and experience is carried as written, never inherited.
func TestTemplateOfCarriesAbsenceAsAbsence(t *testing.T) {
	tmpl, err := templateOf("cook", dungeonspec.TemplateSpec{Base: "dnd5e:monsters:human"})
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

// TestATemplateBaseMustBeARulebookBase: the session looks up the base the
// template names in the rulebook's bases. A base nothing ships, and a
// rulebook monster that is not a base, are both refused by name before
// assembly.
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
	out, err := DeriveTemplate(&DeriveTemplateInput{Ref: "dnd5e:monsters:guard", Spec: castleGuard})
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
	_, err := DeriveTemplate(&DeriveTemplateInput{Ref: "dnd5e:monsters:guard", Spec: spec})
	require.ErrorIs(t, err, ErrUnknownContent)
	require.Contains(t, err.Error(), "dnd5e:monsters:elf")
}

// TestATemplateIsNamedAsItselfNeverAsItsBase: name is identity. An unnamed
// template is called by its id; a named one by what the author wrote.
func TestATemplateIsNamedAsItselfNeverAsItsBase(t *testing.T) {
	out, err := DeriveTemplate(&DeriveTemplateInput{Ref: "dnd5e:monsters:guard", Spec: castleGuard})
	require.NoError(t, err)
	require.Equal(t, "guard", out.Block.Name, "the template's id, not the human's name")

	named := castleGuard
	named.Name = "Castle Guard"
	out, err = DeriveTemplate(&DeriveTemplateInput{Ref: "dnd5e:monsters:guard", Spec: named})
	require.NoError(t, err)
	require.Equal(t, "Castle Guard", out.Block.Name)
}

// TestTemplateOfCarriesEveryStatedField pins the one conversion field by
// field: a spec that states everything converts to exactly this template.
// Dropping any field the author wrote fails here, not as a plausible wrong
// number downstream.
func TestTemplateOfCarriesEveryStatedField(t *testing.T) {
	tmpl, err := templateOf("captain", dungeonspec.TemplateSpec{
		Base:        "dnd5e:monsters:human",
		Name:        "Captain of the Watch",
		Abilities:   map[string]int{"str": 15, "dex": 14, "con": 14, "int": 11, "wis": 12, "cha": 14},
		HitDice:     "10d8",
		Armor:       "dnd5e:armor:breastplate",
		Proficiency: 3,
		Skills:      []string{"perception", "intimidation"},
		Actions:     []string{"dnd5e:weapons:longsword", "dnd5e:weapons:javelin"},
		Experience:  450,
	})
	require.NoError(t, err)
	worn := armor.ArmorID("breastplate")
	require.Equal(t, monster.Template{
		Base: refs.Monsters.Human(),
		Name: "Captain of the Watch",
		Abilities: map[abilities.Ability]int{
			abilities.STR: 15, abilities.DEX: 14, abilities.CON: 14,
			abilities.INT: 11, abilities.WIS: 12, abilities.CHA: 14,
		},
		HitDice:     "10d8",
		Armor:       &worn,
		Proficiency: 3,
		Skills:      []skills.Skill{skills.Perception, skills.Intimidation},
		Actions:     []weapons.WeaponID{weapons.Longsword, weapons.Javelin},
		Experience:  450,
	}, tmpl)
}

// TestDeriveTemplateEchoesTheCaptain is the block for a template that states
// every score but INT and WIS, its own proficiency and two weapons.
func TestDeriveTemplateEchoesTheCaptain(t *testing.T) {
	out, err := DeriveTemplate(&DeriveTemplateInput{Ref: "dnd5e:monsters:captain", Spec: dungeonspec.TemplateSpec{
		Base:        "dnd5e:monsters:human",
		Abilities:   map[string]int{"str": 15, "dex": 14, "con": 14, "cha": 14},
		HitDice:     "10d8",
		Armor:       "dnd5e:armor:breastplate",
		Proficiency: 3,
		Actions:     []string{"dnd5e:weapons:longsword", "dnd5e:weapons:javelin"},
		Experience:  150,
	}})
	require.NoError(t, err)
	block := out.Block
	require.Equal(t, 65, block.HitPoints, "10d8 averages 45, plus CON 14's +2 on each of ten dice")
	require.Equal(t, 16, block.ArmorClass, "a breastplate is 14 plus DEX 14's +2, at the medium-armor cap")
	require.Equal(t, 3, block.ProficiencyBonus)
	require.Equal(t, 10, block.PassivePerception, "10 and WIS 10's +0 from the base; perception is untrained")
	require.Equal(t, 150, block.Experience, "the author's experience reaches the block")
	require.Equal(t, []DerivedAttack{
		{WeaponRef: "dnd5e:weapons:longsword", AttackBonus: 5, Damage: "1d8+2"},
		{WeaponRef: "dnd5e:weapons:javelin", AttackBonus: 5, Damage: "1d6+2"},
	}, block.Attacks, "STR 15's +2 and proficiency 3 to hit; STR to damage")
}

// TestATemplateTheRulebookRefusesKeepsItsReason: an assembly refusal is
// ErrInvalidWorld with the rulebook's reason as text, so the author can read
// what to fix. The dialect refuses this score at shape, so only a hand-built
// compile reaches it.
func TestATemplateTheRulebookRefusesKeepsItsReason(t *testing.T) {
	_, err := instantiate("thing-1", "dnd5e:monsters:thing", nil,
		&dungeonspec.TemplateSpec{Base: "dnd5e:monsters:human", Abilities: map[string]int{"str": 31}})
	require.ErrorIs(t, err, ErrInvalidWorld)
	require.Contains(t, err.Error(), "str score 31")
}

// TestAMalformedBaseIsABadRef keeps this seam's split between malformed and
// unknown: `human` is not a ref at all, so it is ErrBadRef, not "no such
// base".
func TestAMalformedBaseIsABadRef(t *testing.T) {
	for _, base := range []string{"human", ""} {
		_, err := DeriveTemplate(&DeriveTemplateInput{Ref: "dnd5e:monsters:cook",
			Spec: dungeonspec.TemplateSpec{Base: base, Actions: []string{"dnd5e:weapons:dagger"}}})
		require.ErrorIs(t, err, ErrBadRef, "base %q", base)
		require.NotErrorIs(t, err, ErrUnknownContent, "base %q", base)
	}
}

// TestDeriveTemplateRefusesARefItCannotLoad: DeriveTemplate is a public
// entry, so it checks the ref the host hands it as instantiate does.
func TestDeriveTemplateRefusesARefItCannotLoad(t *testing.T) {
	_, err := DeriveTemplate(&DeriveTemplateInput{Ref: "homebrew:monsters:guard", Spec: castleGuard})
	require.ErrorIs(t, err, ErrNoLoader)
	_, err = DeriveTemplate(&DeriveTemplateInput{Ref: "guard", Spec: castleGuard})
	require.ErrorIs(t, err, ErrBadRef)
	_, err = DeriveTemplate(nil)
	require.ErrorIs(t, err, ErrNilInput)
}

// TestDeriveTemplateRefusesATemplateThatShadowsTheRulebook: the authoring
// echo refuses shadowing exactly as launch does, because both come through
// the one assembly. A constructor (`goblin`) and a base (`human`) both count.
func TestDeriveTemplateRefusesATemplateThatShadowsTheRulebook(t *testing.T) {
	for _, ref := range []string{"dnd5e:monsters:goblin", "dnd5e:monsters:human"} {
		_, err := DeriveTemplate(&DeriveTemplateInput{Ref: ref, Spec: castleGuard})
		require.ErrorIs(t, err, ErrShadowedRef, ref)
		require.Contains(t, err.Error(), "rename the template", ref)
	}
}

// TestTheDerivedBlockIsIdentifiedByItsTemplate: no member exists at
// authoring time, so the block's id is the template's.
func TestTheDerivedBlockIsIdentifiedByItsTemplate(t *testing.T) {
	out, err := DeriveTemplate(&DeriveTemplateInput{Ref: "dnd5e:monsters:guard", Spec: castleGuard})
	require.NoError(t, err)
	require.Equal(t, "guard", out.Block.ID)
}
