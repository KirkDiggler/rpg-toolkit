// Copyright (C) 2026 Kirk Diggler
// SPDX-License-Identifier: GPL-3.0-or-later

package spells

import (
	"encoding/json"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/combat/actions"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/conditions"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/refs"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/weapons"
)

// HeldWeapon is one equipped weapon candidate supplied by the caster's sheet.
// Content decides eligibility; session and the client do not encode that rule.
type HeldWeapon struct {
	Slot     string
	ItemID   string
	WeaponID weapons.WeaponID
	Name     string
}

func bindShillelagh(profile *actions.CastProfile, input CastDefinitionInput) bool {
	if input.SpellcastingAbility == "" {
		return false
	}
	config := conditions.ShillelaghConfig{Ability: input.SpellcastingAbility}
	for _, weapon := range input.HeldWeapons {
		if weapon.WeaponID != weapons.Club && weapon.WeaponID != weapons.Quarterstaff {
			continue
		}
		if weapon.Slot == "" || weapon.ItemID == "" {
			continue
		}
		label := weapon.Name + " (" + weapon.Slot + ")"
		config.Weapons = append(config.Weapons, conditions.HeldWeapon{Slot: weapon.Slot, ItemID: weapon.ItemID, Label: label})
	}
	if len(config.Weapons) == 0 {
		return false
	}
	effect := actions.CastEffect{Recipient: actions.CastRecipientCaster, Ref: *refs.Conditions.Shillelagh()}
	if len(config.Weapons) == 1 {
		config.WeaponSlot = config.Weapons[0].Slot
	} else {
		effect.OptionKey = "weapon_slot"
		for _, weapon := range config.Weapons {
			profile.Options = append(profile.Options, actions.CastOption{ID: weapon.Slot, Label: weapon.Label})
		}
	}
	// This structure contains only strings and slices, so marshaling cannot fail.
	effect.Parameters, _ = json.Marshal(config)
	profile.Effects = []actions.CastEffect{effect}
	return true
}
