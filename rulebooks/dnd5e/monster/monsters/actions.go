// Copyright (C) 2026 Kirk Diggler
// SPDX-License-Identifier: GPL-3.0-or-later

package monsters

import (
	"fmt"

	combatActions "github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/combat/actions"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/monster"
)

func mustAddAction(m *monster.Monster, definition combatActions.Definition) {
	if err := m.AddAction(definition); err != nil {
		panic(fmt.Sprintf("invalid monster action %s: %v", definition.Ref.String(), err))
	}
}
