// Copyright (C) 2026 Kirk Diggler
// SPDX-License-Identifier: GPL-3.0-or-later

package dungeonspec

import (
	"os"
	"strings"
	"testing"

	"github.com/stretchr/testify/require"
	"gopkg.in/yaml.v3"
)

// TestSingleRoomActionAugmentStrictDecode accepts the two carrier keys and
// keeps strict decoding: a near-miss remains an author-facing refusal.
func TestSingleRoomActionAugmentStrictDecode(t *testing.T) {
	raw, err := os.ReadFile(castleKitchen)
	require.NoError(t, err)

	t.Run("accepts both keys", func(t *testing.T) {
		source := strings.Replace(string(raw), "    actions: [dnd5e:weapons:spear]", "    actions: [dnd5e:weapons:spear]\n    addActions: [dnd5e:weapons:crossbow]\n    dropActions: [dnd5e:weapons:spear]", 1)
		decoded, err := DecodeSingleRoom(SingleRoomDecodeInput{Source: []byte(source)})
		require.NoError(t, err)
		require.Equal(t, []string{"dnd5e:weapons:crossbow"}, decoded.Spec.Templates["guard"].AddActions)
		require.Equal(t, []string{"dnd5e:weapons:spear"}, decoded.Spec.Templates["guard"].DropActions)
	})

	t.Run("misspelled key refuses by name", func(t *testing.T) {
		source := strings.Replace(string(raw), "    actions: [dnd5e:weapons:spear]", "    addAction: [dnd5e:weapons:crossbow]", 1)
		_, err := DecodeSingleRoom(SingleRoomDecodeInput{Source: []byte(source)})
		require.ErrorIs(t, err, ErrBadSpec)
		var validation *ValidationError
		require.ErrorAs(t, err, &validation)
		require.Contains(t, validation.Errors, FieldError{Path: "templates.guard.addAction", Message: `"addAction" is not a key this build reads: they are abilities, actions, addActions, armor, base, dropActions, experience, hitDice, name, proficiency, skills`})
	})
}

// TestSingleRoomActionAugmentRefusesNonWeapon carries the same field-path
// grammar refusal that actions has at both new carrier locations.
func TestSingleRoomActionAugmentRefusesNonWeapon(t *testing.T) {
	raw, err := os.ReadFile(castleKitchen)
	require.NoError(t, err)

	cases := []struct {
		name, source, path string
	}{
		{
			name: "template add", path: "templates.guard.addActions[0]",
			source: strings.Replace(string(raw), "    actions: [dnd5e:weapons:spear]", "    addActions: [dnd5e:armor:shield]", 1),
		},
		{
			name: "binding drop", path: "room.room.monsterBindings.guard-1.dropActions[0]",
			source: strings.Replace(string(raw), "      guard-1: {faction: watch}", "      guard-1: {faction: watch, dropActions: [dnd5e:armor:shield]}", 1),
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			_, err := DecodeSingleRoom(SingleRoomDecodeInput{Source: []byte(tc.source)})
			require.ErrorIs(t, err, ErrBadSpec)
			var validation *ValidationError
			require.ErrorAs(t, err, &validation)
			require.Contains(t, validation.Errors, FieldError{Path: tc.path, Message: `"dnd5e:armor:shield" is not a weapon: an action names a weapon as dnd5e:weapons:<id>`})
		})
	}
}

// TestSingleRoomActionAugmentRoundTrip preserves both ordered action deltas on
// templates and bindings as authored carrier data.
func TestSingleRoomActionAugmentRoundTrip(t *testing.T) {
	raw, err := os.ReadFile(castleKitchen)
	require.NoError(t, err)
	source := strings.Replace(string(raw), "    actions: [dnd5e:weapons:spear]", "    actions: [dnd5e:weapons:spear]\n    addActions: [dnd5e:weapons:crossbow, dnd5e:weapons:dagger]\n    dropActions: [dnd5e:weapons:spear]", 1)
	source = strings.Replace(source, "      guard-1: {faction: watch}", "      guard-1: {faction: watch, addActions: [dnd5e:weapons:crossbow, dnd5e:weapons:dagger], dropActions: [dnd5e:weapons:spear]}", 1)

	decoded, err := DecodeSingleRoom(SingleRoomDecodeInput{Source: []byte(source)})
	require.NoError(t, err)
	encodedTemplate, err := yaml.Marshal(decoded.Spec.Templates["guard"])
	require.NoError(t, err)
	var template TemplateSpec
	require.NoError(t, yaml.Unmarshal(encodedTemplate, &template))
	require.Equal(t, decoded.Spec.Templates["guard"].AddActions, template.AddActions)
	require.Equal(t, decoded.Spec.Templates["guard"].DropActions, template.DropActions)
	encodedBinding, err := yaml.Marshal(decoded.Spec.Room.Gameplay.MonsterBindings["guard-1"])
	require.NoError(t, err)
	var binding RoomMonsterBinding
	require.NoError(t, yaml.Unmarshal(encodedBinding, &binding))
	require.Equal(t, decoded.Spec.Room.Gameplay.MonsterBindings["guard-1"].AddActions, binding.AddActions)
	require.Equal(t, decoded.Spec.Room.Gameplay.MonsterBindings["guard-1"].DropActions, binding.DropActions)
}

// TestSingleRoomActionAugmentCompileCarries keeps action deltas at the
// dungeonspec-to-session boundary without resolving their rulebook meaning.
func TestSingleRoomActionAugmentCompileCarries(t *testing.T) {
	raw, err := os.ReadFile(castleKitchen)
	require.NoError(t, err)
	source := strings.Replace(string(raw), "    actions: [dnd5e:weapons:spear]", "    addActions: [dnd5e:weapons:crossbow]\n    dropActions: [dnd5e:weapons:spear]", 1)
	source = strings.Replace(source, "      guard-1: {faction: watch}", "      guard-1: {faction: watch, addActions: [dnd5e:weapons:crossbow], dropActions: [dnd5e:weapons:club]}", 1)

	compiled, err := Load([]byte(source))
	require.NoError(t, err)
	require.Equal(t, []string{"dnd5e:weapons:crossbow"}, compiled.Templates["guard"].AddActions)
	require.Equal(t, []string{"dnd5e:weapons:spear"}, compiled.Templates["guard"].DropActions)
	require.Equal(t, []string{"dnd5e:weapons:crossbow"}, compiled.Monsters[0].AddActions)
	require.Equal(t, []string{"dnd5e:weapons:club"}, compiled.Monsters[0].DropActions)
}

// TestSingleRoomActionAugmentNothingStatedPreservesExistingOutput proves that
// omitted carrier fields neither serialize nor alter existing compiled output.
func TestSingleRoomActionAugmentNothingStatedPreservesExistingOutput(t *testing.T) {
	raw, err := os.ReadFile(castleKitchen)
	require.NoError(t, err)
	decoded, err := DecodeSingleRoom(SingleRoomDecodeInput{Source: raw})
	require.NoError(t, err)
	encoded, err := yaml.Marshal(decoded.Spec)
	require.NoError(t, err)
	require.NotContains(t, string(encoded), "addActions:")
	require.NotContains(t, string(encoded), "dropActions:")

	compiled, err := CompileSingleRoom(CompileSingleRoomInput{Spec: decoded.Spec})
	require.NoError(t, err)
	require.Nil(t, compiled.Templates["guard"].AddActions)
	require.Nil(t, compiled.Templates["guard"].DropActions)
	require.Nil(t, compiled.Monsters[0].AddActions)
	require.Nil(t, compiled.Monsters[0].DropActions)
}
