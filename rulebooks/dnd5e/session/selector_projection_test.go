// Copyright (C) 2026 Kirk Diggler
// SPDX-License-Identifier: GPL-3.0-or-later

package session

import (
	"context"
	"encoding/json"
	"reflect"
	"sort"
	"testing"

	"github.com/cyberphone/json-canonicalization/go/src/webpki.org/jsoncanonicalizer"
	"github.com/stretchr/testify/require"

	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/abilities"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/character"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/classes"
	combatActions "github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/combat/actions"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/monster/monsters"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/proficiencies"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/races"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/shared"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/spells"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/weapons"
)

// selectorMechanicalFields is every field of the [combatActions.Definition]
// type tree that IS action identity. Each one reaches the declaration
// selector, either written field for field into the allow-list projection
// (selector_projection.go) or carried inside a root type the projection
// reuses whole.
//
// Display words that were already selector material before the projection
// existed stay mechanical so every held ID is unchanged (provider-design O1):
// Definition.Name, CastOption.Label, and the two below that ride reused types
// — CastArea.MembershipName and contributions.Source's Name and Label. They
// name things; none of them is explanatory prose.
var selectorMechanicalFields = []string{
	"actions.AbilityContribution.Ability",
	"actions.AbilityContribution.Modifier",
	"actions.AttackDelivery.Melee",
	"actions.AttackDelivery.Ranged",
	"actions.AttackProfile.Ability",
	"actions.AttackProfile.AttackBonus",
	"actions.AttackProfile.Category",
	"actions.AttackProfile.Damage",
	"actions.AttackProfile.Delivery",
	"actions.AttackProfile.IsOffHandAttack",
	"actions.AttackProfile.OnHit",
	"actions.AttackProfile.Weapon",
	"actions.CastArea.Catches",
	"actions.CastArea.Footprint",
	"actions.CastArea.MembershipName",
	"actions.CastArea.MembershipRef",
	"actions.CastArea.ObscuresSight",
	"actions.CastConcentration.SkipFirstTurnEnd",
	"actions.CastConcentration.TurnEnds",
	"actions.CastEffect.CounterpartKey",
	"actions.CastEffect.IndependentDuration",
	"actions.CastEffect.OptionKey",
	"actions.CastEffect.Parameters",
	"actions.CastEffect.Recipient",
	"actions.CastEffect.Ref",
	"actions.CastEffect.SaveDCKey",
	"actions.CastMove.Cells",
	"actions.CastMove.CellsByOption",
	"actions.CastMove.Pays",
	"actions.CastMove.Policy",
	"actions.CastMove.Provokes",
	"actions.CastMove.Speed",
	"actions.CastMove.Turn",
	"actions.CastOption.ID",
	"actions.CastOption.Label",
	"actions.CastProfile.Area",
	"actions.CastProfile.Attack",
	"actions.CastProfile.Casting",
	"actions.CastProfile.Concentration",
	"actions.CastProfile.Damage",
	"actions.CastProfile.DamageIfInjured",
	"actions.CastProfile.Effects",
	"actions.CastProfile.Healing",
	"actions.CastProfile.HealingExcludes",
	"actions.CastProfile.MaxTargets",
	"actions.CastProfile.MinTargets",
	"actions.CastProfile.Move",
	"actions.CastProfile.Options",
	"actions.CastProfile.RangeFeet",
	"actions.CastProfile.RecipientBlockedBy",
	"actions.CastProfile.Save",
	"actions.CastProfile.Stabilize",
	"actions.CastProfile.Target",
	"actions.ConditionApplication.CounterpartKey",
	"actions.ConditionApplication.Parameters",
	"actions.ConditionApplication.Ref",
	"actions.ConditionApplication.Save",
	"actions.Definition.Attack",
	"actions.Definition.Cast",
	"actions.Definition.Cost",
	"actions.Definition.Name",
	"actions.Definition.Ref",
	"actions.Definition.Sequence",
	"actions.Footprint.Origin",
	"actions.Footprint.Shape",
	"actions.Footprint.SizeFeet",
	"actions.MeleeDelivery.ReachFeet",
	"actions.RangedDelivery.LongFeet",
	"actions.RangedDelivery.NormalFeet",
	"actions.SequenceProfile.Steps",
	"actions.SequenceStep.Action",
	"actions.SequenceStep.Disadvantage",
	"actions.WeaponContext.OffHandWeaponRef",
	"actions.WeaponContext.Ref",
	"actions.WeaponContext.Slot",
	"actions.WeaponContext.TwoHanded",
	"combat.SpellCasting.Level",
	"combat.SpellCasting.Time",
	"combat.SpendProfile.Capacity",
	"combat.SpendProfile.Grants",
	"combat.SpendProfile.Pools",
	"combat.SpendProfile.Requires",
	"combat.SpendProfile.Slots",
	"contributions.Source.Label",
	"contributions.Source.Name",
	"contributions.Source.Ref",
	"contributions.Source.SourceID",
	"core.Ref.ID",
	"core.Ref.Module",
	"core.Ref.Type",
	"damage.Damage.Dice",
	"damage.Damage.FlatBonus",
	"damage.Damage.Properties",
	"damage.Damage.Type",
	"healing.Declaration.Dice",
	"healing.Declaration.Modifiers",
	"healing.Modifier.Amount",
	"healing.Modifier.Source",
	"saves.SaveGate.Abilities",
	"saves.SaveGate.DC",
	"saves.SaveGate.OnSuccess",
	"saves.SaveGate.Recurrence",
}

// selectorProseFields is every field of the definition's type tree that is
// explanatory content and must never reach the selector (R11).
var selectorProseFields = []string{
	"actions.CastOption.Description",
	"actions.Definition.Description",
}

// selectorMirrors names, for each definition type the projection mirrors
// rather than reuses, its mirror. A prose field may live ONLY on one of these
// types: on a type the projection reuses whole, nothing would keep it out.
var selectorMirrors = map[reflect.Type]reflect.Type{
	reflect.TypeOf(combatActions.Definition{}):  reflect.TypeOf(selectorDefinition{}),
	reflect.TypeOf(combatActions.CastProfile{}): reflect.TypeOf(selectorCastProfile{}),
	reflect.TypeOf(combatActions.CastOption{}):  reflect.TypeOf(selectorCastOption{}),
}

// definitionTypeTreeFields walks t's type tree — structs, pointers, slices,
// arrays and maps — and returns every struct field, exported or not, keyed
// "pkg.Type.Field", with the struct type that declares it.
func definitionTypeTreeFields(root reflect.Type) map[string]reflect.Type {
	seen := map[reflect.Type]bool{}
	out := map[string]reflect.Type{}
	var walk func(reflect.Type)
	walk = func(t reflect.Type) {
		switch t.Kind() {
		case reflect.Pointer, reflect.Slice, reflect.Array:
			walk(t.Elem())
			return
		case reflect.Map:
			walk(t.Key())
			walk(t.Elem())
			return
		case reflect.Struct:
		default:
			return
		}
		if seen[t] {
			return
		}
		seen[t] = true
		for i := 0; i < t.NumField(); i++ {
			field := t.Field(i)
			out[t.String()+"."+field.Name] = t
			walk(field.Type)
		}
	}
	walk(root)
	return out
}

// TestSelectorProjectionClassifiesEveryDefinitionField is R11's guard. Every
// field of the definition's type tree is in exactly one of the two lists; a
// field in neither fails, so a new mechanical field cannot silently escape
// identity and new prose cannot silently enter it. A listed field that no
// longer exists fails too, so the lists cannot rot into a pass.
//
// It then holds the projection to the classification: each mirror carries
// every mechanical field of the type it mirrors, under the same JSON tag, and
// none of its prose.
func TestSelectorProjectionClassifiesEveryDefinitionField(t *testing.T) {
	tree := definitionTypeTreeFields(reflect.TypeOf(combatActions.Definition{}))

	classified := map[string]string{}
	for _, f := range selectorMechanicalFields {
		require.NotContains(t, classified, f, "%s is listed twice", f)
		classified[f] = "mechanical"
	}
	for _, f := range selectorProseFields {
		require.NotContains(t, classified, f, "%s is classified both mechanical and prose", f)
		classified[f] = "prose"
	}

	var unclassified []string
	for f := range tree {
		if _, ok := classified[f]; !ok {
			unclassified = append(unclassified, f)
		}
	}
	sort.Strings(unclassified)
	require.Empty(t, unclassified,
		"classify each field as mechanical (selector identity) or prose (never selector material) "+
			"in selector_projection_test.go, and mirror it in selector_projection.go if it is prose")

	for f := range classified {
		require.Contains(t, tree, f, "%s is classified but no longer exists in the definition's type tree", f)
	}

	for _, f := range selectorProseFields {
		owner := tree[f]
		require.Contains(t, selectorMirrors, owner,
			"prose field %s lives on %s, which the projection reuses whole; mirror that type so the prose stays out", f, owner)
	}

	for source, mirror := range selectorMirrors {
		mirrored := map[string]reflect.StructField{}
		for i := 0; i < mirror.NumField(); i++ {
			mirrored[mirror.Field(i).Name] = mirror.Field(i)
		}
		for i := 0; i < source.NumField(); i++ {
			field := source.Field(i)
			key := source.String() + "." + field.Name
			got, present := mirrored[field.Name]
			switch classified[key] {
			case "prose":
				require.False(t, present, "%s is prose and must not be in %s", key, mirror)
			case "mechanical":
				require.True(t, present, "%s is mechanical and missing from %s", key, mirror)
				require.Equal(t, field.Tag.Get("json"), got.Tag.Get("json"), "%s must keep its JSON tag in %s", key, mirror)
				delete(mirrored, field.Name)
			}
		}
		require.Empty(t, mirrored, "%s carries fields %s does not have", mirror, source)
	}
}

// warhammerDefinition is a real compiled warhammer swing, assembled from a
// loaded sheet the way offer compilation assembles one.
func warhammerDefinition(t *testing.T) combatActions.Definition {
	t.Helper()
	sheet, err := character.Load(context.Background(), &character.Data{
		ID: "cleric", PlayerID: "player-cleric", Name: "cleric", Level: 1, ClassID: classes.Fighter, RaceID: races.Human,
		AbilityScores: shared.AbilityScores{
			abilities.STR: 16, abilities.DEX: 10, abilities.CON: 12,
			abilities.INT: 10, abilities.WIS: 14, abilities.CHA: 10,
		},
		HitPoints: 10, MaxHitPoints: 10, ProficiencyBonus: 2,
		WeaponProficiencies: []proficiencies.Weapon{proficiencies.WeaponMartial},
		Inventory: []character.InventoryItemData{
			{Type: shared.EquipmentTypeWeapon, ID: string(weapons.Warhammer), Quantity: 1},
		},
		EquipmentSlots: character.EquipmentSlots{character.SlotMainHand: string(weapons.Warhammer)},
	})
	require.NoError(t, err)
	definition, err := character.AssembleAttack(sheet, &character.AssembleAttackInput{Slot: character.SlotMainHand})
	require.NoError(t, err)
	return definition
}

func castDefinitionFor(t *testing.T, input spells.CastDefinitionInput) combatActions.Definition {
	t.Helper()
	definition := spells.CastDefinition(input)
	require.NotNil(t, definition, "this build must carry cast content for %s", input.Spell)
	require.NoError(t, definition.Validate())
	return *definition
}

// projectionFixtures are real definitions covering every arm and every
// mirrored field kind: a weapon swing, a cast with options and a condition, a
// cast that moves its target, a cast that binds two held weapons, and a
// multiattack script.
func projectionFixtures(t *testing.T) map[string]combatActions.Definition {
	t.Helper()
	boss := monsters.NewGoblinBoss("boss").Actions()
	require.NotNil(t, boss[0].Sequence, "the goblin boss leads with its multiattack")
	return map[string]combatActions.Definition{
		"warhammer": warhammerDefinition(t),
		"command":   *goldenCastDefinition(t),
		"thorn whip": castDefinitionFor(t, spells.CastDefinitionInput{
			Spell: spells.Thornwhip, SpellSaveDC: 13, SpellAttackBonus: 5, SpellcastingAbility: abilities.WIS,
		}),
		"two-weapon shillelagh": castDefinitionFor(t, spells.CastDefinitionInput{
			Spell: spells.Shillelagh, SpellSaveDC: 13, SpellAttackBonus: 5, SpellcastingAbility: abilities.WIS,
			HeldWeapons: []spells.HeldWeapon{
				{Slot: string(character.SlotMainHand), ItemID: "club-1", WeaponID: weapons.Club, Name: "Club"},
				{Slot: string(character.SlotOffHand), ItemID: "staff-1", WeaponID: weapons.Quarterstaff, Name: "Quarterstaff"},
			},
		}),
		"multiattack": boss[0],
	}
}

// withoutProse returns a deep-enough copy of definition with every prose
// field cleared, leaving the caller's definition untouched.
func withoutProse(definition combatActions.Definition) combatActions.Definition {
	definition.Description = ""
	if definition.Cast != nil {
		cast := *definition.Cast
		if cast.Options != nil {
			cast.Options = append([]combatActions.CastOption(nil), cast.Options...)
			for i := range cast.Options {
				cast.Options[i].Description = ""
			}
		}
		definition.Cast = &cast
	}
	return definition
}

func canonicalJSON(t *testing.T, value any) string {
	t.Helper()
	raw, err := json.Marshal(value)
	require.NoError(t, err)
	canonical, err := jsoncanonicalizer.Transform(raw)
	require.NoError(t, err)
	return string(canonical)
}

// TestSelectorProjectionEqualsDefinitionWithoutProse: on real definitions,
// the projection's canonical document is exactly the definition's with its
// prose cleared — no mechanical field dropped, renamed or re-encoded.
func TestSelectorProjectionEqualsDefinitionWithoutProse(t *testing.T) {
	for name, definition := range projectionFixtures(t) {
		t.Run(name, func(t *testing.T) {
			stripped := withoutProse(definition)
			require.Equal(t,
				canonicalJSON(t, &stripped),
				canonicalJSON(t, selectorDefinitionOf(&definition)))
		})
	}
}

// TestProjectionFixturesCarryProse keeps the equality test honest: the
// fixtures carry the prose the projection is meant to drop, so equality is
// not passing on definitions that had none.
func TestProjectionFixturesCarryProse(t *testing.T) {
	fixtures := projectionFixtures(t)
	require.NotEmpty(t, fixtures["command"].Description)
	require.NotEmpty(t, fixtures["command"].Cast.Options[0].Description)
	require.NotEmpty(t, fixtures["thorn whip"].Cast.Options[0].Description)
	require.NotNil(t, fixtures["thorn whip"].Cast.Move)
	require.NotEmpty(t, fixtures["two-weapon shillelagh"].Cast.Options)
}
