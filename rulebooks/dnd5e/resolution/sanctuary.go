// Copyright (C) 2026 Kirk Diggler
// SPDX-License-Identifier: GPL-3.0-or-later

package resolution

import (
	"context"
	"fmt"

	"github.com/KirkDiggler/rpg-toolkit/dice"
	"github.com/KirkDiggler/rpg-toolkit/events"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/abilities"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/conditions"
	dnd5eEvents "github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/events"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/refs"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/saves"
)

// WardOutcome records one Sanctuary ward that stopped a single-target attack
// or harmful spell cold, before any roll against the target: the attacker's
// own failed Wisdom save against the warding caster's spell save DC. See
// docs/ideas/cleric/plan.md's Sanctuary section for the full design — in
// particular why this lives in resolution rather than as a subscription on
// [conditions.SanctuaryCondition] itself.
type WardOutcome struct {
	// SourceID is the caster whose Sanctuary blocked this attempt.
	SourceID string

	// Ability is the ability the attacker's failed save was made with —
	// always Wisdom for Sanctuary today, but carried explicitly rather than
	// left for a caller to hardcode, the same reason every other saving
	// throw in this package names its own ability instead of leaving it
	// implicit.
	Ability abilities.Ability

	// Save is the attacker's failed saving throw, for the story.
	Save *saves.SavingThrowResult
}

// sanctuaryWardsOn returns every active Sanctuary condition on memberID, one
// per caster currently warding them. RAW allows more than one at once —
// nothing stops two different casters from each concentrating on their own
// Sanctuary over the same target — so an attacker must clear every ward that
// applies, not just the first one found.
func sanctuaryWardsOn(cast *Participants, memberID string) []*conditions.SanctuaryCondition {
	var wards []*conditions.SanctuaryCondition
	for _, condition := range heldConditions(cast, memberID) {
		if ward, ok := condition.(*conditions.SanctuaryCondition); ok {
			wards = append(wards, ward)
		}
	}
	return wards
}

// sanctuaryImmuneTo reports whether attackerID already holds immunity to
// casterID's Sanctuary, earned by a prior successful ward save this fight.
func sanctuaryImmuneTo(cast *Participants, attackerID, casterID string) bool {
	for _, condition := range heldConditions(cast, attackerID) {
		if immune, ok := condition.(*conditions.SanctuaryImmuneCondition); ok && immune.SourceID == casterID {
			return true
		}
	}
	return false
}

// pendingSanctuaryWards is sanctuaryWardsOn filtered down to the casters
// attackerID is not already immune to — the ones that still need a save.
// Empty when attackerID and targetID are the same: nothing stops a warded
// creature from targeting itself, and RAW's ward is about who ELSE targets
// it.
func pendingSanctuaryWards(cast *Participants, attackerID, targetID string) []*conditions.SanctuaryCondition {
	if attackerID == targetID {
		return nil
	}
	var pending []*conditions.SanctuaryCondition
	for _, ward := range sanctuaryWardsOn(cast, targetID) {
		if !sanctuaryImmuneTo(cast, attackerID, ward.SourceID) {
			pending = append(pending, ward)
		}
	}
	return pending
}

// wardSaveDC reads the warding caster's own spell save DC. Sanctuary is
// Cleric-only in this build, so its caster is always a character.
func wardSaveDC(cast *Participants, casterID string) int {
	if caster, ok := cast.Character(casterID); ok {
		return caster.SpellSaveDC()
	}
	return 0
}

// wardSaveInput builds the attacker's Wisdom save against one Sanctuary ward.
func wardSaveInput(attackerID string, ward *conditions.SanctuaryCondition, dc int, roller dice.Roller) *SaveInput {
	return &SaveInput{
		SaverID: attackerID,
		Ability: abilities.WIS,
		DC:      dc,
		Cause: dnd5eEvents.SaveCause{
			Trigger:        dnd5eEvents.SaveTriggerSpell,
			EffectRef:      refs.Spells.Sanctuary(),
			InstigatorID:   ward.SourceID,
			InstigatorType: "character",
		},
		D20Source: dnd5eEvents.RollSource{
			Ref: refs.Spells.Sanctuary(), Name: conditions.SanctuaryName, SourceID: ward.SourceID,
		},
		Roller: roller,
	}
}

// applySanctuaryImmunity grants attackerID the short immunity to casterID's
// Sanctuary that a successful ward save earns. It PUBLISHES rather than
// calls Apply itself — [prepareCondition]/[publishCondition]'s own shape:
// the owning keeper (character/monster) is the one thing that both applies a
// condition to its bus AND marks the sheet dirty for persistence, so every
// delivered condition reaches the bus this way rather than by resolution
// calling Apply directly.
func applySanctuaryImmunity(ctx context.Context, bus events.EventBus, cast *Participants, attackerID, casterID string) error {
	immune, err := conditions.NewSanctuaryImmuneCondition(conditions.NewSanctuaryImmuneConditionInput{
		MemberID: attackerID, SourceID: casterID, SourceRef: refs.Spells.Sanctuary(),
	})
	if err != nil {
		return err
	}
	target, err := cast.entity(attackerID)
	if err != nil {
		return err
	}
	if err := dnd5eEvents.ConditionAppliedTopic.On(bus).Publish(ctx, dnd5eEvents.ConditionAppliedEvent{
		Target: target, Type: dnd5eEvents.ConditionType(immune.Ref().ID),
		Source: dnd5eEvents.ConditionSourceSpell, Condition: immune,
	}); err != nil {
		return fmt.Errorf("apply sanctuary immunity to %q: %w", attackerID, err)
	}
	return nil
}

// endSanctuaryIfHeld ends actorID's OWN Sanctuary ward, if they hold one, the
// moment they make an attack — RAW's self-break clause ("the spell ends if
// the warded creature makes an attack or casts a spell that affects an
// enemy"). Every Strike is definitionally an attack, so this runs
// unconditionally for the attacker of one; a harmful cast's equivalent call
// checks its own profile shape first. It fires on the ATTEMPT, regardless of
// whether this same attack is itself blocked by a DIFFERENT creature's
// Sanctuary — declaring the attack is what ends the caster's own ward, not
// whether it lands.
func endSanctuaryIfHeld(ctx context.Context, bus events.EventBus, cast *Participants, actorID string) error {
	for _, ward := range sanctuaryWardsOn(cast, actorID) {
		// Publish FIRST, [conditions.GuidedCondition.end]'s own order and its
		// own reason: the removal lands on the bus while this ward is still
		// the thing that owned it. Remove() is idempotent (guarded by its own
		// IsApplied check), so it is safe to call here even though the
		// keeper's onConditionRemoved handler will also call it on its own
		// reference to the same object.
		if err := dnd5eEvents.ConditionRemovedTopic.On(bus).Publish(ctx, dnd5eEvents.ConditionRemovedEvent{
			MemberID:     actorID,
			ConditionRef: refs.Conditions.Sanctuary().String(),
			SourceID:     ward.SourceID,
			Reason:       "attacked",
		}); err != nil {
			return fmt.Errorf("publish sanctuary self-break for %q: %w", actorID, err)
		}
		if err := ward.Remove(ctx, bus); err != nil {
			return fmt.Errorf("end sanctuary on %q: %w", actorID, err)
		}
	}
	return nil
}
