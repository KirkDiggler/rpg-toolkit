// Copyright (C) 2024 Kirk Diggler
// SPDX-License-Identifier: GPL-3.0-or-later

package monstertraits

import (
	"context"
	"encoding/json"

	"github.com/KirkDiggler/rpg-toolkit/core"
	"github.com/KirkDiggler/rpg-toolkit/dice"
	"github.com/KirkDiggler/rpg-toolkit/events"
	"github.com/KirkDiggler/rpg-toolkit/rpgerr"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/damage/affinity"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/monster"

	dnd5eEvents "github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/events"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/refs"
)

// LoadJSON loads a monster trait from its JSON representation.
// The game server stores traits as opaque JSON blobs;
// this function deserializes them into strongly-typed structs.
//
// Note: This loader requires a dice.Roller for traits like Undead Fortitude
// that need to make saving throws.
func LoadJSON(data json.RawMessage, roller dice.Roller) (dnd5eEvents.ConditionBehavior, error) {
	// Peek at the ref to determine trait type
	var peek struct {
		Ref core.Ref `json:"ref"`
	}

	if err := json.Unmarshal(data, &peek); err != nil {
		return nil, rpgerr.Wrap(err, "failed to peek at monster trait ref")
	}

	// Route based on ref ID
	switch peek.Ref.ID {
	case refs.DamageAffinities.Resistance().ID,
		refs.DamageAffinities.Vulnerability().ID,
		refs.DamageAffinities.Immunity().ID:
		condition, err := affinity.LoadJSON(data)
		if err != nil {
			return nil, rpgerr.Wrap(err, "failed to load damage affinity")
		}
		return condition, nil

	case refs.MonsterTraits.Immunity().ID:
		trait := &immunityCondition{}
		if err := trait.loadJSON(data); err != nil {
			return nil, rpgerr.Wrap(err, "failed to load immunity trait")
		}
		return trait, nil

	case refs.MonsterTraits.Vulnerability().ID:
		trait := &vulnerabilityCondition{}
		if err := trait.loadJSON(data); err != nil {
			return nil, rpgerr.Wrap(err, "failed to load vulnerability trait")
		}
		return trait, nil

	case refs.MonsterTraits.ConditionImmunity().ID:
		trait := &ConditionImmunityTrait{}
		if err := trait.loadJSON(data); err != nil {
			return nil, rpgerr.Wrap(err, "failed to load condition immunity trait")
		}
		return trait, nil

	case refs.MonsterTraits.PackTactics().ID:
		trait := &packTacticsCondition{}
		if err := trait.loadJSON(data); err != nil {
			return nil, rpgerr.Wrap(err, "failed to load pack tactics trait")
		}
		return trait, nil

	case refs.MonsterTraits.UndeadFortitude().ID:
		if roller == nil {
			return nil, rpgerr.New(rpgerr.CodeInvalidArgument, "roller is required for undead fortitude trait")
		}
		trait := &undeadFortitudeCondition{roller: roller}
		if err := trait.loadJSON(data); err != nil {
			return nil, rpgerr.Wrap(err, "failed to load undead fortitude trait")
		}
		return trait, nil

	default:
		return nil, rpgerr.Newf(rpgerr.CodeInvalidArgument, "unknown monster trait ref: %s", peek.Ref.ID)
	}
}

// AllTraitRefs returns the canonical ref strings for every monster trait
// type this package knows how to load (rpg-toolkit#778) — mirrors LoadJSON's
// dispatch switch exactly; a trait ref added as a new LoadJSON case must be
// added here too, or it won't be recognized as structurally permanent
// (never live-announced) by encounter's ActiveConditions snapshot
// projection. Monster traits (Immunity/Vulnerability/PackTactics/
// UndeadFortitude) are attached at monster construction — the monster-side
// equivalent of a character's Grant.Conditions — and never go through
// Encounter.ActivateFeature's live broker bridge, so they belong in the
// same excluded set as character.StructurallyPermanentConditionRefs.
//
// This hand-mirror is a real, documented drift risk: TestAllTraitRefs_NoPhantomEntries
// (loader_test.go) catches a phantom entry (a ref listed here LoadJSON
// doesn't actually recognize), but cannot catch a missing entry (a new
// LoadJSON case forgotten here) — that direction needs LoadJSON
// restructured around an enumerable registry this function could derive
// from instead of mirroring. Tracked as rpg-toolkit#780.
func AllTraitRefs() []string {
	return []string{
		refs.DamageAffinities.Resistance().String(),
		refs.DamageAffinities.Vulnerability().String(),
		refs.DamageAffinities.Immunity().String(),
		refs.MonsterTraits.Immunity().String(),
		refs.MonsterTraits.Vulnerability().String(),
		refs.MonsterTraits.ConditionImmunity().String(),
		refs.MonsterTraits.PackTactics().String(),
		refs.MonsterTraits.UndeadFortitude().String(),
	}
}

// LoadMonsterConditions is a helper function that loads conditions/traits from JSON data
// and applies them to a monster. This is needed because the monster package
// cannot import the monstertraits package directly (import cycle).
//
// Usage:
//
//	mon, err := monster.LoadFromData(ctx, data, bus)
//	if err := monstertraits.LoadMonsterConditions(ctx, mon, data.Conditions, bus, roller); err != nil {
//	    // handle error
//	}
func LoadMonsterConditions(
	ctx context.Context,
	m *monster.Monster,
	conditionData []json.RawMessage,
	bus events.EventBus,
	roller dice.Roller,
) error {
	for _, data := range conditionData {
		condition, err := LoadJSON(data, roller)
		if err != nil {
			return rpgerr.Wrap(err, "failed to load monster condition")
		}

		// Apply the condition so it subscribes to events
		if err := condition.Apply(ctx, bus); err != nil {
			// Clean up any partial subscriptions
			_ = condition.Remove(ctx, bus)
			return rpgerr.Wrap(err, "failed to apply monster condition")
		}

		m.AddCondition(condition)
	}
	return nil
}
