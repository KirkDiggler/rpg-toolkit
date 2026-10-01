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

// PresentedWeaponOutput reports the first catalog weapon in the ordered list.
// An empty WeaponID means the list names no catalog weapon (or is absent),
// not that somebody observed empty hands.
type PresentedWeaponOutput struct {
	WeaponID weapons.WeaponID
}

// PresentedWeapon answers the temporary monster weapon presentation policy:
// the first weapon entry is rendered, skipping Multiattack sequences and other
// non-weapon actions. It never sorts the list, chooses a weapon by range, reads
// which attack was used, or recursively interprets sequence components.
//
// This is not equipped state and gives no permission to switch weapons. A future
// equipped-state mechanism must account for the action cost of a weapon change;
// monsters receive no exemption. This projection does not change combat rules.
//
// A nil input, invalid visited action identity or unknown first catalog weapon
// is refused. A list with no weapon produces a non-nil answer with an empty
// WeaponID. Once the first weapon is found, later entries are not consulted.
// The input and its definitions are never modified.
func PresentedWeapon(in *PresentedWeaponInput) (*PresentedWeaponOutput, error) {
	if in == nil {
		return nil, rpgerr.New(rpgerr.CodeInvalidArgument, "monster weapon presentation input is required")
	}
	out := &PresentedWeaponOutput{}
	for index, action := range in.Actions {
		ref := action.Ref
		if err := ref.IsValid(); err != nil {
			return nil, rpgerr.Wrapf(err, "invalid monster action identity at index %d", index)
		}
		if ref.Module != refs.Module || ref.Type != refs.TypeWeapons {
			continue
		}
		weapon, err := weapons.GetByID(weapons.WeaponID(ref.ID))
		if err != nil {
			return nil, rpgerr.Wrap(err, "cannot present first monster weapon")
		}
		out.WeaponID = weapon.ID
		return out, nil
	}
	return out, nil
}
