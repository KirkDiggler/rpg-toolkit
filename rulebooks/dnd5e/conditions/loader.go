// Copyright (C) 2024 Kirk Diggler
// SPDX-License-Identifier: GPL-3.0-or-later

package conditions

import (
	"encoding/json"

	"github.com/KirkDiggler/rpg-toolkit/core"
	"github.com/KirkDiggler/rpg-toolkit/rpgerr"
	dnd5eEvents "github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/events"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/refs"
)

// LoadJSON loads a condition from its JSON representation.
// The game server stores conditions as opaque JSON blobs;
// this function deserializes them into strongly-typed structs.
func LoadJSON(data json.RawMessage) (dnd5eEvents.ConditionBehavior, error) {
	// Peek at the ref to determine condition type
	var peek struct {
		Ref core.Ref `json:"ref"`
	}

	if err := json.Unmarshal(data, &peek); err != nil {
		return nil, rpgerr.Wrap(err, "failed to peek at condition ref")
	}

	// Route based on ref ID
	switch peek.Ref.ID {
	case refs.Conditions.Raging().ID:
		raging := &RagingCondition{}
		if err := raging.loadJSON(data); err != nil {
			return nil, rpgerr.Wrap(err, "failed to load raging condition")
		}
		return raging, nil

	case refs.Conditions.BrutalCritical().ID:
		brutal := &BrutalCriticalCondition{}
		if err := brutal.loadJSON(data); err != nil {
			return nil, rpgerr.Wrap(err, "failed to load brutal critical condition")
		}
		return brutal, nil

	case refs.Conditions.UnarmoredDefense().ID:
		unarmored := &UnarmoredDefenseCondition{}
		if err := unarmored.loadJSON(data); err != nil {
			return nil, rpgerr.Wrap(err, "failed to load unarmored defense condition")
		}
		return unarmored, nil

	case refs.Conditions.FightingStyleArchery().ID:
		archery := NewFightingStyleArcheryCondition("")
		if err := archery.loadJSON(data); err != nil {
			return nil, rpgerr.Wrap(err, "failed to load archery fighting style condition")
		}
		return archery, nil

	case refs.Conditions.FightingStyleDefense().ID:
		defense := NewFightingStyleDefenseCondition("")
		if err := defense.loadJSON(data); err != nil {
			return nil, rpgerr.Wrap(err, "failed to load defense fighting style condition")
		}
		return defense, nil

	case refs.Conditions.FightingStyleDueling().ID:
		dueling := NewFightingStyleDuelingCondition("")
		if err := dueling.loadJSON(data); err != nil {
			return nil, rpgerr.Wrap(err, "failed to load dueling fighting style condition")
		}
		return dueling, nil

	case refs.Conditions.FightingStyleGreatWeaponFighting().ID:
		gwf := NewFightingStyleGreatWeaponFightingCondition("", nil)
		if err := gwf.loadJSON(data); err != nil {
			return nil, rpgerr.Wrap(err, "failed to load great weapon fighting style condition")
		}
		return gwf, nil

	case refs.Conditions.FightingStyleProtection().ID:
		protection := NewFightingStyleProtectionCondition("")
		if err := protection.loadJSON(data); err != nil {
			return nil, rpgerr.Wrap(err, "failed to load protection fighting style condition")
		}
		return protection, nil

	case refs.Conditions.FightingStyleTwoWeaponFighting().ID:
		twf := NewFightingStyleTwoWeaponFightingCondition("")
		if err := twf.loadJSON(data); err != nil {
			return nil, rpgerr.Wrap(err, "failed to load two-weapon fighting style condition")
		}
		return twf, nil

	case refs.Conditions.ImprovedCritical().ID:
		ic := &ImprovedCriticalCondition{}
		if err := ic.loadJSON(data); err != nil {
			return nil, rpgerr.Wrap(err, "failed to load improved critical condition")
		}
		return ic, nil

	case refs.Conditions.MartialArts().ID:
		ma := &MartialArtsCondition{}
		if err := ma.loadJSON(data); err != nil {
			return nil, rpgerr.Wrap(err, "failed to load martial arts condition")
		}
		return ma, nil

	case refs.Conditions.UnarmoredMovement().ID:
		um := &UnarmoredMovementCondition{}
		if err := um.loadJSON(data); err != nil {
			return nil, rpgerr.Wrap(err, "failed to load unarmored movement condition")
		}
		return um, nil

	case refs.Features.SneakAttack().ID:
		sneak := &SneakAttackCondition{}
		if err := sneak.loadJSON(data); err != nil {
			return nil, rpgerr.Wrap(err, "failed to load sneak attack condition")
		}
		return sneak, nil

	case refs.Conditions.Disengaging().ID:
		disengaging := &DisengagingCondition{}
		if err := disengaging.loadJSON(data); err != nil {
			return nil, rpgerr.Wrap(err, "failed to load disengaging condition")
		}
		return disengaging, nil

	case refs.Conditions.Dodging().ID:
		dodging := &DodgingCondition{}
		if err := dodging.loadJSON(data); err != nil {
			return nil, rpgerr.Wrap(err, "failed to load dodging condition")
		}
		return dodging, nil

	default:
		return nil, rpgerr.Newf(rpgerr.CodeInvalidArgument, "unknown condition ref: %s", peek.Ref.ID)
	}
}
