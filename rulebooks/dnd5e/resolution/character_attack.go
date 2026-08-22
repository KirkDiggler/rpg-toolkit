// Copyright (C) 2026 Kirk Diggler
// SPDX-License-Identifier: GPL-3.0-or-later

package resolution

import (
	"fmt"

	"github.com/KirkDiggler/rpg-toolkit/core"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/abilities"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/character"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/refs"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/weapons"
)

const (
	// defaultMeleeReach is the melee reach used when the weapon carries no
	// Reach property: 5 feet, the 5e default and the reach of the
	// canonical unarmed strike.
	//
	// FEET, not cells (Kirk, rpg-project#254 review) — reach and speed are
	// both authored in feet everywhere in this codebase's data, matching
	// monster.ActionData.Reach and monster.SpeedData; a cell is 5 feet, and
	// the ONE place that needs cells (session's reach gate, and the
	// monster-turn's movement budget) converts once, at the point of
	// comparison against Distance, rather than this compiler guessing at a
	// conversion the gate is better placed to own.
	defaultMeleeReach = 5

	// reachPropertyReach is the melee reach a weapon with the Reach
	// property grants: 10 feet (glaive, halberd, spear-with-reach-variant,
	// pike) — see defaultMeleeReach's doc for the feet-not-cells note.
	reachPropertyReach = 10
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
// Only STATIC facts: the weapon's canonical damage pools, whether finesse lets
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
// Monk martial arts dice — an upgraded unarmed die keyed to level — are a
// future compiler's business; this one always compiles the catalog's plain
// 1d1 (see "An empty hand is not a refusal" below, rpg-toolkit#1168).
//
// Reach and adjacency are UNENFORCED BY THIS COMPILER AND THE STRIKE MACHINE
// IT FEEDS — the profile now carries Reach (rpg-toolkit#1010), but nothing
// here checks a target's distance against it; that gate lives above this
// seam, in whatever caller has both combatants' positions (session.Attack).
// The strike itself still does not check distance at all, exactly as for
// the bite.
//
// # An empty hand is not a refusal
//
// It used to be: an empty main hand refused with ErrBadAttack, on the
// argument that a silent fallback is how a character who dropped their
// sword keeps "attacking" and nobody notices. That argument proved too
// much. The 5e rule is that an unarmed strike is always available — Kirk's
// ruling, verbatim: "you can always attack you would just punch but can
// still attack" (rpg-toolkit#1168) — so a compiler that refused it was not
// guarding against a silent fallback, it was refusing a real swing. See
// [equippedWeapon] for where the substitution happens; proficiency for it
// is forced below rather than asked of [character.Character.IsProficientWith],
// because nothing on a sheet grants "unarmed strike" as a trained weapon and
// every 5e character is proficient with it regardless.
//
// ErrBadAttack still stands for what it always meant beyond that one case: a
// sheet the rulebook cannot read — an unknown weapon ref, a slot holding
// something that is not a weapon at all, or a slot whose entry names an item
// that is not in inventory. That last one matters precisely because it looks
// like the empty-hand case from the outside: both end with
// [character.Character.GetEquippedSlot] returning nil. Only one of them is a
// character choosing to fight bare-handed; the other is a record this
// rulebook cannot trust, and conflating the two would compile a corrupt
// sheet into a perfectly ordinary swing. See [equippedWeapon].
func AttackFromCharacter(c *character.Character, in *CharacterAttackInput) (AttackProfile, error) {
	if c == nil {
		return AttackProfile{}, fmt.Errorf("%w: no character to compile", ErrNilInput)
	}
	if in == nil {
		return AttackProfile{}, fmt.Errorf("%w: no attack input", ErrNilInput)
	}

	weapon, unarmed, err := equippedWeapon(c, in.Slot)
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
	// and nothing else. An unarmed strike is the one exception the rulebook
	// itself grants everybody — see the doc comment above — so it is never
	// asked of IsProficientWith, which has no grant to answer with.
	attackBonus := modifier
	if unarmed || c.IsProficientWith(weapon) {
		attackBonus += c.ProficiencyBonus()
	}

	pools, err := weapon.DamageForGrip(in.TwoHanded)
	if err != nil {
		return AttackProfile{}, fmt.Errorf("%w: %w", ErrBadAttack, err)
	}

	reach := defaultMeleeReach
	if weapon.HasProperty(weapons.PropertyReach) {
		reach = reachPropertyReach
	}

	profile := AttackProfile{
		Ref:         ref,
		Name:        weapon.Name,
		AttackBonus: attackBonus,
		Reach:       reach,
		Damage:      copyDamagePools(pools),
		// Which ability swung is the fact effects predicate on: Rage pays out
		// on melee Strength attacks and needs to know this was one. Its
		// arithmetic remains separately attributable in AbilityModifier.
		AbilityUsed:     ability,
		AbilityModifier: modifier,
		// Weapons declare no rider. A gate on a character's attack comes from
		// class content (a monk's Flurry), which compiles elsewhere.
		Gate: nil,
		// Static equipment facts a predicate like Dueling decides eligibility
		// from (rpg-toolkit#1178) — knowable here, from the sheet, and never
		// re-derived live from a registry.
		TwoHanded:        in.TwoHanded || weapon.HasProperty(weapons.PropertyTwoHanded),
		OffHandWeaponRef: otherHandWeaponRef(c, in.Slot),
	}
	if err := profile.validate(); err != nil {
		return AttackProfile{}, err
	}

	return profile, nil
}

// otherHandWeaponRef reports the weapon, if any, occupying the hand OTHER
// than the one a swing compiled from — nil when that hand is empty or holds
// something that is not a weapon (a shield, most often).
//
// Unlike equippedWeapon, this draws no unarmed-strike distinction: "nothing
// to swing" and "nothing occupying the other hand" are different questions,
// and only the first one is a rule (rpg-toolkit#1168). This is purely a
// static equipment fact for a chain-fold predicate like Dueling to read off
// the compiled profile instead of a live gamectx lookup (rpg-toolkit#1178).
// A slot naming an item that turns out not to be in inventory reads the
// same as an empty slot here — conservatively "no weapon" either way — since
// that distinction (rpg-toolkit#1173) is equippedWeapon's concern for the
// hand actually being swung, not this one's for the other hand.
func otherHandWeaponRef(c *character.Character, slot character.InventorySlot) *core.Ref {
	var other character.InventorySlot
	switch slot {
	case character.SlotMainHand:
		other = character.SlotOffHand
	case character.SlotOffHand:
		other = character.SlotMainHand
	default:
		return nil
	}

	equipped := c.GetEquippedSlot(other)
	if equipped == nil {
		return nil
	}

	w := equipped.AsWeapon()
	if w == nil {
		return nil
	}

	return refs.Weapons.ByID(string(w.ID))
}

// equippedWeapon reads the weapon out of a slot: what is equipped there, or
// the catalog's unarmed strike when nothing is — the rule, not a gap
// (rpg-toolkit#1168). unarmed is true only in that substitution case, which
// is what tells the caller to force proficiency rather than ask the sheet
// for a grant no sheet has.
//
// Still refuses by name for a caller mistake (no slot named at all) and for
// a slot the rulebook cannot read as a weapon — equipped but not a weapon,
// which an empty hand is not.
func equippedWeapon(c *character.Character, slot character.InventorySlot) (weapon *weapons.Weapon, unarmed bool, err error) {
	if slot == "" {
		return nil, false, fmt.Errorf("%w: no equipment slot named", ErrBadAttack)
	}

	equipped := c.GetEquippedSlot(slot)
	if equipped == nil {
		// GetEquippedSlot returning nil conflates two states this compiler must
		// not: a slot with no entry at all — a genuinely empty hand, which 5e
		// says is always an unarmed strike (rpg-toolkit#1168, see "An empty
		// hand is not a refusal" above) — and a slot whose entry names an item
		// that is not in inventory, which is an UNREADABLE SHEET, not an empty
		// hand. Reading the slot entry directly (rather than trusting the
		// resolved *EquippedItem) is what tells the two apart.
		itemID := c.ToData().EquipmentSlots.Get(slot)
		if itemID == "" {
			w := weapons.SpecialWeapons[weapons.UnarmedStrike]
			return &w, true, nil
		}
		return nil, false, fmt.Errorf(
			"%w: %q names %q which is not in the inventory", ErrBadAttack, slot, itemID)
	}

	weapon = equipped.AsWeapon()
	if weapon == nil {
		return nil, false, fmt.Errorf("%w: %q holds no weapon", ErrBadAttack, slot)
	}

	return weapon, false, nil
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
