// Copyright (C) 2026 Kirk Diggler
// SPDX-License-Identifier: GPL-3.0-or-later

package character

import "github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/combat"

// ParticipationView is the immutable combat-participation projection needed by
// Standing/Participation consumers. It deliberately contains no display data.
type ParticipationView struct {
	LifeState  combat.LifeState
	DeathSaves *DeathSaveProgress
}

// ParticipationView returns derived life state and optional detached Death Save
// progress without consulting display catalogs or publishing.
func (c *Character) ParticipationView() ParticipationView {
	if c == nil {
		return ParticipationView{LifeState: combat.LifeStateUnknown}
	}

	lifeState := c.lifeState()
	view := ParticipationView{LifeState: lifeState}
	if lifeState != combat.LifeStateConscious {
		progress := deathSaveProgress(c.deathSaveState)
		view.DeathSaves = &progress
	}
	return view
}
