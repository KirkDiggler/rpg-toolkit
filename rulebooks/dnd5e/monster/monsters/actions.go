// Copyright (C) 2026 Kirk Diggler
// SPDX-License-Identifier: GPL-3.0-or-later

package monsters

import (
	"fmt"

	combatActions "github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/combat/actions"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/monster"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/weapons"
)

func mustAddAction(m *monster.Monster, definition combatActions.Definition) {
	if err := m.AddAction(definition); err != nil {
		panic(fmt.Sprintf("invalid monster action %s: %v", definition.Ref.String(), err))
	}
}

// mustAddWeapon arms a monster with catalog weapons, IN THE ORDER GIVEN —
// which is the order the drivers read, so melee goes first for a monster that
// should swing when you are standing on top of it (rpg-project#448).
//
// The sibling of mustAddAction one function up, and it panics for the same
// reason: a stat block that names a weapon the catalog does not have is a
// programming error in content that ships with the toolkit, not a runtime
// condition any caller could handle.
func mustAddWeapon(m *monster.Monster, ids ...weapons.WeaponID) {
	for _, id := range ids {
		if err := m.AddWeapon(id); err != nil {
			panic(fmt.Sprintf("cannot arm %s with %s: %v", m.Name(), id, err))
		}
	}
}
