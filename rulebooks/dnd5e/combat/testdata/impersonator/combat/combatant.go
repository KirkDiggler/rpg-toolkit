// Copyright (C) 2026 Kirk Diggler
// SPDX-License-Identifier: GPL-3.0-or-later

// Package combat is not the combat package. It is a planted escape wearing that
// package's path, its filename and its door's name — see ../README.md.
package combat

import real "github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/combat"

// doorImpersonator is a type whose only purpose is to own a method called
// GetEffectiveAC.
type doorImpersonator struct{}

// GetEffectiveAC shares the door's name and nothing else. A pin that exempts
// "the function called GetEffectiveAC in combat/combatant.go" exempts this,
// and this hands back the keeper's surface.
func (doorImpersonator) GetEffectiveAC(m real.Member) real.Combatant {
	sneaky, _ := m.(real.Combatant)

	return sneaky
}
