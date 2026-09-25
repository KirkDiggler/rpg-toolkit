// Copyright (C) 2026 Kirk Diggler
// SPDX-License-Identifier: GPL-3.0-or-later

package dungeonspec_test

import (
	"strings"
	"testing"

	"github.com/stretchr/testify/require"
)

func declarationSource(root, binding string) string {
	return v4With(root, binding)
}

func TestV4DeclarationsBindFactionAndInheritOrders(t *testing.T) {
	source := declarationSource("factions:\n  - { id: watch, mind: goblin-1, table: drill }\ntables:\n  drill:\n    time:\n      - { hold: {} }\n", "    monsterBindings:\n      goblin-1: { faction: watch }\n      goblin-2: { faction: watch }\n")
	compiled := loadSource(t, source)
	for _, id := range []string{"goblin-1", "goblin-2"} {
		m := monsterNamed(t, compiled, id)
		require.Equal(t, "watch", m.Faction)
		require.True(t, m.Table["time"][0].Hold)
	}
}

func TestV4DeclarationsValidateBindingMembership(t *testing.T) {
	for _, tc := range []struct{ name, binding, path, message string }{
		{"null faction", "goblin-1: { faction: null }", "room.room.monsterBindings.goblin-1.faction", "must not be null"},
		{"empty faction", "goblin-1: { faction: '' }", "room.room.monsterBindings.goblin-1.faction", "must not be empty"},
		{"mind in another faction", "goblin-1: { faction: other }", "factions[0].mind", ""},
	} {
		t.Run(tc.name, func(t *testing.T) {
			errs := refusals(t, declarationSource("factions:\n  - { id: watch, mind: goblin-1 }\n  - { id: other }\n", "    monsterBindings:\n      "+tc.binding+"\n"))
			require.NotEmpty(t, errs)
			if tc.message != "" {
				requireExactDefect(t, errs, tc.path, tc.message)
				return
			}
			found := false
			for _, e := range errs {
				if e.Path == tc.path {
					found = true
					require.Contains(t, e.Message, "own faction")
				}
			}
			require.True(t, found, "mind membership must be checked against bindings")
		})
	}
}

func TestV4DeclarationsRejectOldMonstersKey(t *testing.T) {
	source := strings.ReplaceAll(declarationSource("", ""), "    monsterDeclarations:", "    monsters:")
	errs := refusals(t, source)
	requireExactDefect(t, errs, "room.room.monsters", "monsters has been renamed to monsterDeclarations; rename this key")
}

func TestV4DeclarationsRejectFactionOnDeclaration(t *testing.T) {
	source := strings.Replace(declarationSource("", ""), "id: goblin-1,", "id: goblin-1, faction: watch,", 1)
	errs := refusals(t, source)
	requireExactDefect(t, errs, "room.room.monsterDeclarations[0].faction", "faction belongs in monsterBindings, keyed by this monster's id; move it there")
}
