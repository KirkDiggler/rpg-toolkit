// Copyright (C) 2026 Kirk Diggler
// SPDX-License-Identifier: GPL-3.0-or-later

package resolution

import (
	"encoding/json"
	"reflect"
	"testing"

	"github.com/stretchr/testify/require"

	coreResources "github.com/KirkDiggler/rpg-toolkit/core/resources"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/abilities"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/character"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/languages"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/proficiencies"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/saves"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/shared"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/skills"
)

// TestCloneCharacterDataOwnsEveryMutableField is the structural guard on the
// record copy at the LongRest boundary.
//
// Reflection makes the fixture exhaustive rather than exemplary: every map,
// slice, pointer, or interface field currently declared on character.Data must
// be populated and separately owned by the clone. A future mutable field starts
// nil here and fails until its fixture value and clone policy are both explicit.
func TestCloneCharacterDataOwnsEveryMutableField(t *testing.T) {
	source := &character.Data{
		AbilityScores:  shared.AbilityScores{abilities.STR: 16},
		DeathSaveState: &saves.DeathSaveState{Failures: 1},
		Skills: map[skills.Skill]shared.ProficiencyLevel{
			skills.Athletics: shared.Proficient,
		},
		SavingThrows: map[abilities.Ability]shared.ProficiencyLevel{
			abilities.STR: shared.Proficient,
		},
		Languages:           []languages.Language{languages.Common},
		ArmorProficiencies:  []proficiencies.Armor{proficiencies.ArmorLight},
		WeaponProficiencies: []proficiencies.Weapon{proficiencies.WeaponSimple},
		ToolProficiencies:   []proficiencies.Tool{proficiencies.ToolSmith},
		Inventory: []character.InventoryItemData{
			{Type: shared.EquipmentTypeWeapon, ID: "longsword", Quantity: 1},
		},
		EquipmentSlots: character.EquipmentSlots{character.SlotMainHand: "longsword"},
		SpellSlots:     map[int]character.SpellSlotData{1: {Max: 2, Used: 1}},
		ClassResources: map[shared.ClassResourceType]character.ResourceData{
			shared.ClassResourceSecondWind: {
				Name: "Second Wind", Max: 1, Current: 0, Resets: shared.ResetTypeShortRest,
			},
		},
		Resources: map[coreResources.ResourceKey]character.RecoverableResourceData{
			coreResources.ResourceKey("test-pool"): {
				Current: 0, Maximum: 1, ResetType: coreResources.ResetLongRest,
			},
		},
		Features:   []json.RawMessage{json.RawMessage(`{"feature":true}`)},
		Conditions: []json.RawMessage{json.RawMessage(`{"condition":true}`)},
		ActionEconomy: &character.ActionEconomyData{
			TurnNumber: 3,
			Granted:    map[character.GrantedActionKey]int{character.GrantedAttacks: 2},
		},
	}

	clone := cloneCharacterData(source)
	require.NotSame(t, source, clone)

	sourceValue := reflect.ValueOf(source).Elem()
	cloneValue := reflect.ValueOf(clone).Elem()
	dataType := sourceValue.Type()

	mutableFields := 0
	for i := 0; i < dataType.NumField(); i++ {
		field := dataType.Field(i)
		if !isMutableCharacterDataKind(field.Type.Kind()) {
			continue
		}
		mutableFields++

		t.Run(field.Name, func(t *testing.T) {
			sourceField := sourceValue.Field(i)
			cloneField := cloneValue.Field(i)

			require.False(t, sourceField.IsNil(),
				"fixture must populate every mutable character.Data field; add %s and its clone policy",
				field.Name)
			require.False(t, cloneField.IsNil(),
				"cloneCharacterData dropped mutable field %s", field.Name)
			require.NotEqual(t,
				mutableCharacterDataIdentity(t, sourceField),
				mutableCharacterDataIdentity(t, cloneField),
				"cloneCharacterData shares mutable field %s with its source", field.Name)
		})
	}
	require.Positive(t, mutableFields, "character.Data unexpectedly has no mutable fields")

	// ActionEconomy is a pointer and Granted is mutable state nested beneath
	// it. Top-level pointer ownership alone would not protect this map.
	require.NotSame(t, source.ActionEconomy, clone.ActionEconomy)
	require.NotEqual(t,
		reflect.ValueOf(source.ActionEconomy.Granted).Pointer(),
		reflect.ValueOf(clone.ActionEconomy.Granted).Pointer(),
		"ActionEconomy.Granted must be separately owned")
	clone.ActionEconomy.Granted[character.GrantedAttacks] = 99
	require.Equal(t, 2, source.ActionEconomy.Granted[character.GrantedAttacks],
		"mutating the clone's nested grant bank must not reach the source")
}

func isMutableCharacterDataKind(kind reflect.Kind) bool {
	switch kind {
	case reflect.Map, reflect.Slice, reflect.Pointer, reflect.Interface:
		return true
	default:
		return false
	}
}

func mutableCharacterDataIdentity(t *testing.T, value reflect.Value) uintptr {
	t.Helper()

	for value.Kind() == reflect.Interface {
		require.False(t, value.IsNil(), "mutable interface fixture value must not be nil")
		value = value.Elem()
	}

	switch value.Kind() {
	case reflect.Map, reflect.Slice, reflect.Pointer:
		return value.Pointer()
	default:
		require.FailNowf(t, "mutable interface fixture has no identity",
			"got dynamic kind %s; populate it with a map, slice, or pointer", value.Kind())
		return 0
	}
}
