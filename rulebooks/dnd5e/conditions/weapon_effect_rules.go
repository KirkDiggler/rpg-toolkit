// Copyright (C) 2026 Kirk Diggler
// SPDX-License-Identifier: GPL-3.0-or-later

package conditions

import (
	"fmt"

	"github.com/KirkDiggler/rpg-toolkit/core"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/contributions"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/refs"
)

// weapon_effect_rules.go holds the rules that answer from the frame's weapon
// facts: which weapon the attack is made with, how it is held, and what kind
// of attack it is. Each condition's execution — its fold handler, or the
// attack-assembly override — decides with the same predicate.

var (
	_ contributions.ActionAssessor = (*RecklessAttackCondition)(nil)
	_ contributions.ActionAssessor = (*FightingStyleDuelingCondition)(nil)
	_ contributions.ActionAssessor = (*FightingStyleGreatWeaponFightingCondition)(nil)
	_ contributions.ActionAssessor = (*FightingStyleTwoWeaponFightingCondition)(nil)
	_ contributions.ActionAssessor = (*MartialArtsCondition)(nil)
	_ contributions.ActionAssessor = (*ShillelaghCondition)(nil)
)

// frameWeaponID reads the frame's weapon as a catalogue ID. known is false
// when the weapon is unknown; an empty ID with known true is an attack made
// with no weapon. A weapon fact that is not a ref fails Frame.Validate first.
func frameWeaponID(frame contributions.Frame) (id string, known bool) {
	weapon, known := frame.Action.Weapon.Get()
	if !known || weapon == "" {
		return "", known
	}
	ref, err := core.ParseString(weapon)
	if err != nil {
		return "", false
	}
	return ref.ID, true
}

// isAttackRoll reports whether the frame asks about an attack roll.
func isAttackRoll(frame contributions.Frame) bool {
	roll, _ := frame.Action.Roll.Get()
	return roll == contributions.RollKindAttack
}

// AssessAction answers whether Reckless Attack's advantage bears on the framed
// attack: its holder's melee attacks that are not opportunity attacks.
// Advantage granted to attacks against the holder is not its action.
func (r *RecklessAttackCondition) AssessAction(
	in *contributions.AssessActionInput,
) (*contributions.AssessActionOutput, error) {
	return r.rule().AssessAction(in)
}

func (r *RecklessAttackCondition) rule() recklessAttackRule {
	return recklessAttackRule{owner: r.MemberID}
}

// recklessAttackRule holds only the facts Reckless Attack's own-attack
// predicate uses.
type recklessAttackRule struct {
	owner string
}

func (r recklessAttackRule) AssessAction(in *contributions.AssessActionInput) (*contributions.AssessActionOutput, error) {
	frame, err := frameOf(in, "reckless attack")
	if err != nil {
		return nil, err
	}
	if frame.Actor != r.owner {
		return assessed(contributions.DoesNotApply, "Reckless Attack affects only its holder's attacks"), nil
	}
	if !isAttackRoll(frame) {
		return assessed(contributions.DoesNotApply, "Reckless Attack affects only attack rolls"), nil
	}
	melee, meleeKnown := frame.Action.Melee.Get()
	opportunity, opportunityKnown := frame.Action.Opportunity.Get()
	if meleeKnown && !melee {
		return assessed(contributions.DoesNotApply, "Reckless Attack needs a melee attack"), nil
	}
	if opportunityKnown && opportunity {
		return assessed(contributions.DoesNotApply, "Reckless Attack does not cover opportunity attacks"), nil
	}
	if !meleeKnown || !opportunityKnown {
		return assessed(contributions.Depends, "Depends on the kind of attack"), nil
	}
	out := assessed(contributions.Applies, "The attack is a melee attack on your turn")
	out.Answer.Benefit = "Advantage on the attack roll"
	return out, nil
}

// AssessAction answers whether Dueling's +2 bears on the framed attack.
func (f *FightingStyleDuelingCondition) AssessAction(
	in *contributions.AssessActionInput,
) (*contributions.AssessActionOutput, error) {
	return f.rule().AssessAction(in)
}

func (f *FightingStyleDuelingCondition) rule() duelingRule {
	return duelingRule{owner: f.CharacterID}
}

// duelingRule holds only the facts Dueling's predicate uses: a real melee
// weapon — the unarmed strike is not one — held in one hand, with no weapon
// in the other.
type duelingRule struct {
	owner string
}

func (r duelingRule) AssessAction(in *contributions.AssessActionInput) (*contributions.AssessActionOutput, error) {
	frame, err := frameOf(in, "dueling")
	if err != nil {
		return nil, err
	}
	if frame.Actor != r.owner {
		return assessed(contributions.DoesNotApply, "Dueling affects only its holder's attacks"), nil
	}
	if !isAttackRoll(frame) {
		return assessed(contributions.DoesNotApply, "Dueling adds only to weapon attacks"), nil
	}
	pool, poolKnown := frame.Action.WeaponPool.Get()
	weapon, weaponKnown := frameWeaponID(frame)
	melee, meleeKnown := frame.Action.Melee.Get()
	twoHanded, gripKnown := frame.Action.TwoHanded.Get()
	otherWeapon, otherKnown := frame.Action.OffHandWeapon.Get()
	if poolKnown && !pool {
		return assessed(contributions.DoesNotApply, "Dueling requires a weapon damage pool"), nil
	}
	if weaponKnown && (weapon == "" || weapon == refs.Weapons.UnarmedStrike().ID) {
		return assessed(contributions.DoesNotApply, "Dueling needs a melee weapon"), nil
	}
	if meleeKnown && !melee {
		return assessed(contributions.DoesNotApply, "Dueling needs a melee weapon"), nil
	}
	if gripKnown && twoHanded {
		return assessed(contributions.DoesNotApply, "Dueling needs the weapon in one hand"), nil
	}
	if otherKnown && otherWeapon {
		return assessed(contributions.DoesNotApply, "Dueling needs no other weapon in hand"), nil
	}
	if !poolKnown || !weaponKnown || !meleeKnown || !gripKnown || !otherKnown {
		return assessed(contributions.Depends, "Depends on the attack's weapon and grip"), nil
	}
	out := assessed(contributions.Applies, "A melee weapon in one hand and no other weapon")
	out.Answer.Benefit = "+2 damage"
	return out, nil
}

// AssessAction answers whether Great Weapon Fighting's rerolls bear on the
// framed attack.
func (f *FightingStyleGreatWeaponFightingCondition) AssessAction(
	in *contributions.AssessActionInput,
) (*contributions.AssessActionOutput, error) {
	return f.rule().AssessAction(in)
}

func (f *FightingStyleGreatWeaponFightingCondition) rule() greatWeaponFightingRule {
	return greatWeaponFightingRule{owner: f.MemberID}
}

// greatWeaponFightingRule holds only the facts Great Weapon Fighting's
// predicate uses: a melee weapon attack with the weapon held in both hands.
type greatWeaponFightingRule struct {
	owner string
}

func (r greatWeaponFightingRule) AssessAction(
	in *contributions.AssessActionInput,
) (*contributions.AssessActionOutput, error) {
	frame, err := frameOf(in, "great weapon fighting")
	if err != nil {
		return nil, err
	}
	if frame.Actor != r.owner {
		return assessed(contributions.DoesNotApply, "Great Weapon Fighting affects only its holder's attacks"), nil
	}
	if !isAttackRoll(frame) {
		return assessed(contributions.DoesNotApply, "Great Weapon Fighting adds only to weapon attacks"), nil
	}
	pool, poolKnown := frame.Action.WeaponPool.Get()
	melee, meleeKnown := frame.Action.Melee.Get()
	twoHanded, gripKnown := frame.Action.TwoHanded.Get()
	if poolKnown && !pool {
		return assessed(contributions.DoesNotApply, "Great Weapon Fighting requires a weapon damage pool"), nil
	}
	if meleeKnown && !melee {
		return assessed(contributions.DoesNotApply, "Great Weapon Fighting needs a melee weapon"), nil
	}
	if gripKnown && !twoHanded {
		return assessed(contributions.DoesNotApply, "Great Weapon Fighting needs the weapon in both hands"), nil
	}
	if !poolKnown || !meleeKnown || !gripKnown {
		return assessed(contributions.Depends, "Depends on the attack's weapon and grip"), nil
	}
	out := assessed(contributions.Applies, "A melee weapon held in both hands")
	out.Answer.Benefit = "Rerolls 1s and 2s on the weapon's damage dice"
	return out, nil
}

// AssessAction answers whether Two-Weapon Fighting's damage bears on the
// framed attack.
func (f *FightingStyleTwoWeaponFightingCondition) AssessAction(
	in *contributions.AssessActionInput,
) (*contributions.AssessActionOutput, error) {
	return f.rule().AssessAction(in)
}

func (f *FightingStyleTwoWeaponFightingCondition) rule() twoWeaponFightingRule {
	return twoWeaponFightingRule{owner: f.MemberID}
}

// twoWeaponFightingRule holds only the facts Two-Weapon Fighting's predicate
// uses. The base off-hand attack already keeps a negative modifier; the style
// restores only a positive one, so a modifier that is not positive adds
// nothing.
type twoWeaponFightingRule struct {
	owner string
}

func (r twoWeaponFightingRule) AssessAction(in *contributions.AssessActionInput) (*contributions.AssessActionOutput, error) {
	frame, err := frameOf(in, "two-weapon fighting")
	if err != nil {
		return nil, err
	}
	if frame.Actor != r.owner {
		return assessed(contributions.DoesNotApply, "Two-Weapon Fighting affects only its holder's attacks"), nil
	}
	if !isAttackRoll(frame) {
		return assessed(contributions.DoesNotApply, "Two-Weapon Fighting adds only to weapon attacks"), nil
	}
	offHand, offHandKnown := frame.Action.OffHandAttack.Get()
	modifier, modifierKnown := frame.Action.AbilityModifier.Get()
	if offHandKnown && !offHand {
		return assessed(contributions.DoesNotApply, "Two-Weapon Fighting adds only to the off-hand attack"), nil
	}
	if modifierKnown && modifier <= 0 {
		return assessed(contributions.DoesNotApply, "Your ability modifier adds nothing to damage"), nil
	}
	if !offHandKnown || !modifierKnown {
		return assessed(contributions.Depends, "Depends on the off-hand attack and your ability modifier"), nil
	}
	out := assessed(contributions.Applies, "This is the off-hand attack")
	out.Answer.Benefit = fmt.Sprintf("+%d damage", modifier)
	return out, nil
}

// AssessAction answers whether Martial Arts bears on the framed attack. Its
// ability and die are settled at attack assembly by WeaponAttackOverride,
// which decides with the same weapon predicate; this answer reads the
// assembled weapon and changes nothing.
func (ma *MartialArtsCondition) AssessAction(in *contributions.AssessActionInput) (*contributions.AssessActionOutput, error) {
	frame, err := frameOf(in, "martial arts")
	if err != nil {
		return nil, err
	}
	if frame.Actor != ma.MemberID {
		return assessed(contributions.DoesNotApply, "Martial Arts affects only its holder's attacks"), nil
	}
	if !isAttackRoll(frame) {
		return assessed(contributions.DoesNotApply, "Martial Arts affects only attack rolls"), nil
	}
	weapon, known := frameWeaponID(frame)
	if !known {
		return assessed(contributions.Depends, "Depends on the attack's weapon"), nil
	}
	unarmed, monk := martialArtsWeaponID(weapon)
	switch {
	case unarmed:
		out := assessed(contributions.Applies, "The attack is an unarmed strike")
		out.Answer.Benefit = fmt.Sprintf("Can use Dexterity; deals the %s Martial Arts die", ma.getMartialArtsDice())
		return out, nil
	case monk:
		out := assessed(contributions.Applies, "The attack is made with a monk weapon")
		out.Answer.Benefit = "Can use Dexterity"
		return out, nil
	default:
		return assessed(contributions.DoesNotApply, "Martial Arts needs an unarmed strike or a monk weapon"), nil
	}
}

// AssessAction answers whether Shillelagh bears on the framed attack: an
// attack with the enchanted weapon in the hand it was enchanted in, while the
// spell lasts. WeaponAttackOverride decides the swing's dice and ability with
// the same binding; this answer reads the assembled weapon and changes nothing.
func (s *ShillelaghCondition) AssessAction(in *contributions.AssessActionInput) (*contributions.AssessActionOutput, error) {
	frame, err := frameOf(in, "shillelagh")
	if err != nil {
		return nil, err
	}
	if frame.Actor != s.MemberID {
		return assessed(contributions.DoesNotApply, "Shillelagh affects only its caster's attacks"), nil
	}
	if !isAttackRoll(frame) {
		return assessed(contributions.DoesNotApply, "Shillelagh affects only attack rolls"), nil
	}
	if s.TurnEndsLeft <= 0 {
		return assessed(contributions.DoesNotApply, "Shillelagh has ended"), nil
	}
	weapon, weaponKnown := frameWeaponID(frame)
	slot, slotKnown := frame.Action.WeaponSlot.Get()
	if !weaponKnown || !slotKnown {
		return assessed(contributions.Depends, "Depends on the attack's weapon"), nil
	}
	if !s.binds(slot, weapon) {
		return assessed(contributions.DoesNotApply, "Shillelagh enchants a different weapon"), nil
	}
	out := assessed(contributions.Applies, "The attack is made with the enchanted weapon")
	out.Answer.Benefit = fmt.Sprintf("1d8 magical damage, using %s", s.Ability.Display())
	return out, nil
}
