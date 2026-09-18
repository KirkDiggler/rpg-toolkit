// Copyright (C) 2026 Kirk Diggler
// SPDX-License-Identifier: GPL-3.0-or-later

package monster

import (
	"github.com/KirkDiggler/rpg-toolkit/rpgerr"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/abilities"
	combatActions "github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/combat/actions"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/combat/weaponattack"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/weapons"
)

// A monster wields catalog weapons (rpg-project#448).
//
// Its arms used to be hand-typed attack definitions in its constructor, with
// the attack bonus and damage dice written out. They were the wielder's own
// numbers all along — a skeleton's "+4, 1d6+2" is DEX 14 and a CR-based +2
// standing behind a 1d6 shortbow — so the constructor now names the weapon
// and the arithmetic comes from the same place a character's does
// ([weaponattack.Assemble]).
//
// The three methods below are the monster's half of [weaponattack.Wielder].

// GetAbilityModifier reports the monster's modifier for one ability.
//
// Named to match the character's method of the same name, which is what lets
// both satisfy [weaponattack.Wielder] with no shim. It reads the same scores
// [Monster.GetSavingThrowModifier] does.
func (m *Monster) GetAbilityModifier(ability abilities.Ability) int {
	return m.abilityScores.Modifier(ability)
}

// IsProficientWith reports whether this monster is trained with a weapon.
//
// YES, ALWAYS, for every weapon (rpg-project#448 decision 7). A monster's
// stat block does not carry a training list, and every SRD weapon line for
// every monster the toolkit ships is reproduced exactly by adding the
// CR-based proficiency bonus — so "proficient" is the default rather than a
// fact somebody has to author, and a monster handed a weapon it was never
// written holding is right without a second field.
//
// THIS METHOD IS THE RESERVED SEAT, not a shortcut past one. The placement's
// `proficient: false` switch (decision 7, deliberately NOT built here) lands
// as one whole-monster field that this method reads. Nothing else about the
// assembly moves when it does.
func (m *Monster) IsProficientWith(_ *weapons.Weapon) bool {
	return true
}

// AddWeapon assembles a catalog weapon into this monster's own attack
// definition and appends it.
//
// The sibling of [Monster.AddAction], and named for it: both take one thing
// the monster can do and put it at the end of the list, and the list's ORDER
// is what the drivers read — they take the first action whose target is in
// reach, so melee listed first is what makes an adjacent skeleton swing
// rather than shoot. The difference is only where the numbers come from: an
// authored action arrives already compiled, a weapon is compiled here.
//
// The action's ref is the WEAPON's ref, exactly as it is for a character
// (`dnd5e:weapons:shortbow`). There is no such thing as a goblin shortbow.
func (m *Monster) AddWeapon(id weapons.WeaponID) error {
	definition, err := m.assembleWeapon(id)
	if err != nil {
		return err
	}
	return m.AddAction(definition)
}

// SetWeapons replaces every action this monster carries with the named
// weapons, in the order given.
//
// A SETTER, not an adder, and named in the family of [Monster.SetSpeed] for
// that reason: it states what this monster's arms ARE.
// That is what an author naming a placement's actions means — "this goblin
// carries a bow and nothing else" — and appending could never say it.
//
// ALL OR NOTHING. Every weapon is assembled and validated before the first
// one is stored, so a list with one bad entry leaves the monster holding what
// it already held rather than half a loadout. An empty list is refused: a
// monster with no actions is not something an author can currently mean, and
// silently disarming one would look exactly like the default case.
//
// It does not mark the sheet dirty, for [Monster.AddAction]'s reason — arming
// happens while the monster is being built, before any sheet exists to be out
// of date.
func (m *Monster) SetWeapons(ids []weapons.WeaponID) error {
	if len(ids) == 0 {
		return rpgerr.New(rpgerr.CodeInvalidArgument, "no weapons to arm the monster with")
	}

	assembled := make([]combatActions.Definition, 0, len(ids))
	for _, id := range ids {
		definition, err := m.assembleWeapon(id)
		if err != nil {
			return err
		}
		if err := definition.Validate(); err != nil {
			return rpgerr.Wrap(err, "invalid monster action")
		}
		assembled = append(assembled, definition.Clone())
	}

	m.actions = assembled
	return nil
}

func (m *Monster) assembleWeapon(id weapons.WeaponID) (combatActions.Definition, error) {
	weapon, err := weapons.GetByID(id)
	if err != nil {
		return combatActions.Definition{}, rpgerr.Wrapf(err, "cannot arm %q with %q", m.name, id)
	}
	return weaponattack.Assemble(&weaponattack.Input{Wielder: m, Weapon: &weapon})
}
