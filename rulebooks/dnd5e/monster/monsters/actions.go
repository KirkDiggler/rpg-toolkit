// Copyright (C) 2026 Kirk Diggler
// SPDX-License-Identifier: GPL-3.0-or-later

package monsters

import "github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/monster"

func mustAction(action monster.MonsterAction, err error) monster.MonsterAction {
	if err != nil {
		panic(err)
	}
	return action
}
