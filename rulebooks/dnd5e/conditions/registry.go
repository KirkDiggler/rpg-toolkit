// Copyright (C) 2026 Kirk Diggler
// SPDX-License-Identifier: GPL-3.0-or-later

package conditions

import (
	"errors"
	"fmt"
	"maps"
	"slices"

	"github.com/KirkDiggler/rpg-toolkit/rpgerr"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/combat/weaponattack"
	dnd5eEvents "github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/events"
)

// ErrUnknownCondition is wrapped by every read handed a ref the registry does
// not hold. A ref on a stored blob that nothing registers is never read as
// "not held".
var ErrUnknownCondition = errors.New("condition ref is not registered")

// ErrNotCreatable is wrapped by [CreateFromRef] for a registered type whose
// descriptor has no create.
var ErrNotCreatable = errors.New("condition type is not created from a ref")

// descriptors is every registered condition type, one value per type, each
// declared in its type's own file. The order is the order of the entries in
// the design; nothing reads it as a priority.
var descriptors = []descriptor{
	shillelaghDescriptor,
	ragingDescriptor,
	brutalCriticalDescriptor,
	unarmoredDefenseDescriptor,
	fightingStyleArcheryDescriptor,
	fightingStyleDefenseDescriptor,
	fightingStyleDuelingDescriptor,
	fightingStyleGreatWeaponFightingDescriptor,
	fightingStyleProtectionDescriptor,
	fightingStyleTwoWeaponFightingDescriptor,
	improvedCriticalDescriptor,
	recklessAttackDescriptor,
	martialArtsDescriptor,
	sneakAttackDescriptor,
	disengagingDescriptor,
	dodgingDescriptor,
	proneDescriptor,
	hiddenDescriptor,
	inspiredDescriptor,
	helpedDescriptor,
	unconsciousDescriptor,
	opportunityAttackDescriptor,
	trueStrikeDescriptor,
	bladeWardDescriptor,
	viciousMockeryDescriptor,
	commandedDescriptor,
	concentratingDescriptor,
	blessedDescriptor,
	banedDescriptor,
	divineFavorDescriptor,
	faerieFireDescriptor,
	shieldOfFaithDescriptor,
	guidedDescriptor,
	resistanceDescriptor,
	sanctuaryDescriptor,
	sanctuaryImmuneDescriptor,
	guidingBoltDescriptor,
	immunityDescriptor,
	vulnerabilityDescriptor,
	packTacticsDescriptor,
	undeadFortitudeDescriptor,
}

// registry is descriptors keyed by ref.String(). [newRegistry] refuses a
// malformed list, so the package does not load with one.
var registry = mustRegistry(descriptors)

// newRegistry keys list by ref.String() and refuses a malformed entry: a nil
// ref, two entries with one ref, a nil load, an empty display name, a bearing
// other than notBearing with an empty detail, an answering againstHolder
// without a heldRule, and a heldRule on a bearing that does not answer. The
// error names the entry's ref, or its index when it has none.
func newRegistry(list []descriptor) (map[string]descriptor, error) {
	keyed := make(map[string]descriptor, len(list))
	for index, entry := range list {
		if entry.ref == nil {
			return nil, fmt.Errorf("condition descriptor %d has no ref", index)
		}
		key := entry.ref.String()
		if _, duplicate := keyed[key]; duplicate {
			return nil, fmt.Errorf("condition descriptor %d (%s) repeats a ref another entry holds", index, key)
		}
		if entry.load == nil {
			return nil, fmt.Errorf("condition descriptor %d (%s) has no load", index, key)
		}
		if entry.display.Name == "" {
			return nil, fmt.Errorf("condition descriptor %d (%s) has no display name", index, key)
		}
		if (entry.ownAction.kind != bearingNone || entry.againstHolder.kind != bearingNone) &&
			entry.display.Detail == "" {
			return nil, fmt.Errorf("condition descriptor %d (%s) bears on actions but has no display detail", index, key)
		}
		if entry.againstHolder.kind == bearingAnswers && entry.heldRule == nil {
			return nil, fmt.Errorf("condition descriptor %d (%s) answers for attacks against its holder but has no heldRule",
				index, key)
		}
		if entry.againstHolder.kind != bearingAnswers && entry.heldRule != nil {
			return nil, fmt.Errorf("condition descriptor %d (%s) has a heldRule but does not answer for attacks against its holder",
				index, key)
		}
		keyed[key] = entry
	}
	return keyed, nil
}

// mustRegistry is [newRegistry] for the package variable; it panics with
// newRegistry's error.
func mustRegistry(list []descriptor) map[string]descriptor {
	keyed, err := newRegistry(list)
	if err != nil {
		panic(err)
	}
	return keyed
}

// lookup is the one registry read. An unregistered ref returns an error
// wrapping [ErrUnknownCondition] with CodeInvalidArgument.
func lookup(ref string) (descriptor, error) {
	entry, ok := registry[ref]
	if !ok {
		return descriptor{}, rpgerr.WrapWithCode(ErrUnknownCondition, rpgerr.CodeInvalidArgument,
			"unknown condition ref: "+ref)
	}
	return entry, nil
}

// RegisteredRefs lists every registered ref, sorted. A test that must cover
// every condition reads this rather than restating its size.
func RegisteredRefs() []string {
	return slices.Sorted(maps.Keys(registry))
}

// FreeReactionRefs lists the refs of [FreeReactions], the one list of
// reactions a combatant carries by existing, in the list's order.
func FreeReactionRefs() []string {
	reactions := FreeReactions("")
	refs := make([]string, 0, len(reactions))
	for _, reaction := range reactions {
		refs = append(refs, reaction.Ref().String())
	}
	return refs
}

// ActionBearingRefs lists, sorted, every registered ref whose own-action
// bearing answers or cannot yet answer — the conditions that yield an effect
// row for their holder's own action. It is a read of the registry, not a
// second list: a test that must cover every bearing condition reads this
// rather than restating its size.
func ActionBearingRefs() []string {
	return bearingRefs(func(entry descriptor) bearing { return entry.ownAction })
}

// TargetBearingRefs lists, sorted, every registered ref whose bearing on an
// attack against its holder answers or cannot yet answer — the conditions
// that yield a row when the target of an attack holds them. It is a read of
// the registry, not a second list.
func TargetBearingRefs() []string {
	return bearingRefs(func(entry descriptor) bearing { return entry.againstHolder })
}

// bearingRefs lists, sorted, the refs whose chosen bearing is not bearingNone.
func bearingRefs(chosen func(descriptor) bearing) []string {
	bearingOnes := make([]string, 0, len(registry))
	for ref, entry := range registry {
		if chosen(entry).kind != bearingNone {
			bearingOnes = append(bearingOnes, ref)
		}
	}
	slices.Sort(bearingOnes)
	return bearingOnes
}

// WeaponAttackOverrideInput is one held condition and the swing it may offer
// for.
type WeaponAttackOverrideInput struct {
	Condition dnd5eEvents.ConditionBehavior
	Swing     *weaponattack.OverrideInput
}

// WeaponAttackOverride asks a held condition's descriptor for its offer at
// attack assembly. A descriptor with no attackOverride answers an output with a
// nil Override: the type makes no offer.
//
// Errors: a nil input, a nil condition or a nil ref (CodeInvalidArgument), an
// unregistered ref (wrapping [ErrUnknownCondition]), and whatever the offer
// itself refuses.
func WeaponAttackOverride(in *WeaponAttackOverrideInput) (*weaponattack.OverrideOutput, error) {
	if in == nil {
		return nil, rpgerr.New(rpgerr.CodeInvalidArgument, "weapon attack override requires input")
	}
	if in.Condition == nil || in.Condition.Ref() == nil {
		return nil, rpgerr.New(rpgerr.CodeInvalidArgument, "weapon attack override requires a condition with a ref")
	}
	entry, err := lookup(in.Condition.Ref().String())
	if err != nil {
		return nil, err
	}
	if entry.attackOverride == nil {
		return &weaponattack.OverrideOutput{}, nil
	}
	return entry.attackOverride(in.Condition, in.Swing)
}
