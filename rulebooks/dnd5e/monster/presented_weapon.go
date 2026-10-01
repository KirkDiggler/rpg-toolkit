// Copyright (C) 2026 Kirk Diggler
// SPDX-License-Identifier: GPL-3.0-or-later

package monster

import (
	"github.com/KirkDiggler/rpg-toolkit/rpgerr"
	combatActions "github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/combat/actions"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/refs"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/weapons"
)

// PresentedWeaponInput names the monster's existing ordered action definitions.
// Callers supply the stored or constructed sheet's list, never a second loadout.
type PresentedWeaponInput struct {
	Actions []combatActions.Definition
}

// PresentedWeaponOutput reports the catalog weapon at the top of the list.
// An empty WeaponID means the top action names no catalog weapon (or the list
// is absent), not that somebody observed empty hands.
type PresentedWeaponOutput struct {
	WeaponID weapons.WeaponID
}

// PresentedWeapon answers the temporary monster weapon presentation policy:
// only the top action's weapon is rendered. It never skips a natural attack,
// sorts the list, chooses a weapon by range, or reads which attack was used.
//
// This is not equipped state and gives no permission to switch weapons. A future
// equipped-state mechanism must account for the action cost of a weapon change;
// monsters receive no exemption. This projection does not change combat rules.
//
// A nil input, invalid top-action identity or unknown catalog weapon is refused.
// A non-weapon top action produces a non-nil answer with an empty WeaponID.
// The input and its definitions are never modified.
func PresentedWeapon(in *PresentedWeaponInput) (*PresentedWeaponOutput, error) {
	if in == nil {
		return nil, rpgerr.New(rpgerr.CodeInvalidArgument, "monster weapon presentation input is required")
	}
	out := &PresentedWeaponOutput{}
	if len(in.Actions) == 0 {
		return out, nil
	}
	ref := in.Actions[0].Ref
	if err := ref.IsValid(); err != nil {
		return nil, rpgerr.Wrap(err, "invalid top monster action identity")
	}
	if ref.Module != refs.Module || ref.Type != refs.TypeWeapons {
		return out, nil
	}
	weapon, err := weapons.GetByID(weapons.WeaponID(ref.ID))
	if err != nil {
		return nil, rpgerr.Wrap(err, "cannot present top monster weapon")
	}
	out.WeaponID = weapon.ID
	return out, nil
}
