// Copyright (C) 2026 Kirk Diggler
// SPDX-License-Identifier: GPL-3.0-or-later

package resolution

import (
	"fmt"

	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/abilities"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/character"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/refs"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/weapons"
)

// CharacterAttackInput names which swing to compile.
//
// The grip is the caller's to state rather than the compiler's to infer. A
// versatile weapon in a free off hand is *usually* two-handed, but "usually"
// is a turn-economy question — a character may choose a one-handed grip to
// keep a hand free — and this compiler answers only what the sheet and the
// weapon already know.
type CharacterAttackInput struct {
	// Slot names the equipped weapon to swing. The main hand for a standard
	// attack; the off hand for the second swing of two-weapon fighting, whose
	// economy and ability-modifier rules live above this compiler.
	Slot character.InventorySlot

	// TwoHanded says the weapon is gripped in both hands. It changes the dice
	// of a versatile weapon and nothing else — not the ability, not the
	// proficiency, not the bonus.
	TwoHanded bool
}

// AttackFromCharacter compiles an equipped weapon and the sheet holding it
// into the same neutral profile a monster action compiles into. It is the
// character half of the seam [StrikeInput.Attack] names, and the machine
// cannot tell the two apart — that indistinguishability is the whole point
// (rpg-toolkit#1003).
//
// # What compiles here, and what does not
//
// Only STATIC facts: the weapon's dice and damage type, whether finesse lets
// DEX win, whether the sheet is proficient, and whether a versatile grip
// steps the die. Every one of them is knowable before the swing and cannot
// change between two swings of the same weapon by the same character.
//
// Situational effects are deliberately absent and MUST stay absent. Rage's
// damage bonus, Bless, Sneak Attack's dice, advantage from any source — each
// has its own predicate that decides per swing, and each arrives through the
// chain folds the machine already runs. A Raging character's profile is
// byte-identical to the same character's profile when calm; the +2 arrives at
// the damage fold, attributed to Raging's own ref. If a mechanic seems to
// want a new field here, it is almost certainly a chain contribution wearing
// a disguise.
//
// # Named gaps
//
// Melee only. A ranged weapon is refused by name for the same reason the
// monster's ranged action is: the strike has no range semantics — no range
// brackets, no long-range disadvantage, and prone's interaction reverses at
// distance. Compiler and machine both grow when that arrives.
//
// No unarmed strike. An empty slot is refused rather than falling back to the
// catalog's unarmed weapon, because a silent 1d1 fallback is how a character
// who dropped their sword keeps "attacking" and nobody notices. Monk martial
// arts dice are a future compiler's business.
//
// Reach and adjacency are unenforced, exactly as for the bite: the strike
// does not check distance at all. One shared, named gap, not one this case
// adds.
func AttackFromCharacter(c *character.Character, in *CharacterAttackInput) (AttackProfile, error) {
	if c == nil {
		return AttackProfile{}, fmt.Errorf("%w: no character to compile", ErrNilInput)
	}
	if in == nil {
		return AttackProfile{}, fmt.Errorf("%w: no attack input", ErrNilInput)
	}

	weapon, err := equippedWeapon(c, in.Slot)
	if err != nil {
		return AttackProfile{}, err
	}

	// Category, not the presence of a Range: a dagger is a melee weapon that
	// carries a thrown range, and refusing on Range != nil would reject every
	// thrown melee weapon in the catalog.
	if weapon.IsRanged() {
		return AttackProfile{}, fmt.Errorf(
			"%w: %q is a ranged weapon (the strike has no range semantics yet)", ErrBadAttack, weapon.ID)
	}

	ref := refs.Weapons.ByID(string(weapon.ID))
	if ref == nil {
		return AttackProfile{}, fmt.Errorf("%w: no ref for weapon %q", ErrBadAttack, weapon.ID)
	}

	ability := attackAbility(c, weapon)
	modifier := c.GetAbilityModifier(ability)

	// Proficiency is a fact about the sheet, not about the swing: a character
	// wielding a weapon they were never trained on adds their ability modifier
	// and nothing else.
	attackBonus := modifier
	if c.IsProficientWith(weapon) {
		attackBonus += c.ProficiencyBonus()
	}

	dice := weapon.Damage
	if in.TwoHanded {
		dice = weapon.VersatileDamage()
	}

	return AttackProfile{
		Ref:         ref,
		AttackBonus: attackBonus,
		// The ability modifier rides the notation rather than a separate
		// field, and that is load-bearing for critical hits: the machine
		// doubles a crit by rolling the pool a second time and takes the flat
		// modifier from the FIRST roll only, so "+3" inside "1d8+3" doubles
		// the dice and not the modifier — which is exactly ADR-0036's rule.
		DamageDice: fmt.Sprintf("%s%+d", dice, modifier),
		DamageType: weapon.DamageType,
		// Which ability swung. Not arithmetic — the modifier is already in the
		// notation above — but the fact effects predicate on: Rage pays out on
		// melee Strength attacks and needs to know this was one.
		AbilityUsed: ability,
		// Weapons declare no rider. A gate on a character's attack comes from
		// class content (a monk's Flurry), which compiles elsewhere.
		Gate: nil,
	}, nil
}

// equippedWeapon reads the weapon out of a slot, refusing an empty or
// non-weapon slot by name.
func equippedWeapon(c *character.Character, slot character.InventorySlot) (*weapons.Weapon, error) {
	if slot == "" {
		return nil, fmt.Errorf("%w: no equipment slot named", ErrBadAttack)
	}

	equipped := c.GetEquippedSlot(slot)
	if equipped == nil {
		return nil, fmt.Errorf("%w: nothing equipped in %q", ErrBadAttack, slot)
	}

	weapon := equipped.AsWeapon()
	if weapon == nil {
		return nil, fmt.Errorf("%w: %q holds no weapon", ErrBadAttack, slot)
	}

	return weapon, nil
}

// attackAbility picks the ability a melee weapon swings with: Strength,
// unless the weapon is finesse and the character's Dexterity is strictly
// better.
//
// Strictly better, not merely different: 5e lets a finesse wielder choose,
// and a tie is not a choice worth making — the two produce identical numbers,
// so STR wins by being the default rather than by being larger.
func attackAbility(c *character.Character, weapon *weapons.Weapon) abilities.Ability {
	if !weapon.HasProperty(weapons.PropertyFinesse) {
		return abilities.STR
	}

	if c.GetAbilityModifier(abilities.DEX) > c.GetAbilityModifier(abilities.STR) {
		return abilities.DEX
	}

	return abilities.STR
}
