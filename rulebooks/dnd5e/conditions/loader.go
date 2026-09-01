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

type conditionLoader func(json.RawMessage) (dnd5eEvents.ConditionBehavior, error)

var conditionLoaders = map[string]conditionLoader{
	refs.Conditions.Raging().String(): func(data json.RawMessage) (dnd5eEvents.ConditionBehavior, error) {
		raging := &RagingCondition{}
		if err := raging.loadJSON(data); err != nil {
			return nil, rpgerr.Wrap(err, "failed to load raging condition")
		}
		return raging, nil
	},
	refs.Conditions.BrutalCritical().String(): func(data json.RawMessage) (dnd5eEvents.ConditionBehavior, error) {
		brutal := &BrutalCriticalCondition{}
		if err := brutal.loadJSON(data); err != nil {
			return nil, rpgerr.Wrap(err, "failed to load brutal critical condition")
		}
		return brutal, nil
	},
	refs.Conditions.UnarmoredDefense().String(): func(data json.RawMessage) (dnd5eEvents.ConditionBehavior, error) {
		unarmored := &UnarmoredDefenseCondition{}
		if err := unarmored.loadJSON(data); err != nil {
			return nil, rpgerr.Wrap(err, "failed to load unarmored defense condition")
		}
		return unarmored, nil
	},
	refs.Conditions.FightingStyleArchery().String(): func(data json.RawMessage) (dnd5eEvents.ConditionBehavior, error) {
		archery := NewFightingStyleArcheryCondition("")
		if err := archery.loadJSON(data); err != nil {
			return nil, rpgerr.Wrap(err, "failed to load archery fighting style condition")
		}
		return archery, nil
	},
	refs.Conditions.FightingStyleDefense().String(): func(data json.RawMessage) (dnd5eEvents.ConditionBehavior, error) {
		defense := NewFightingStyleDefenseCondition("")
		if err := defense.loadJSON(data); err != nil {
			return nil, rpgerr.Wrap(err, "failed to load defense fighting style condition")
		}
		return defense, nil
	},
	refs.Conditions.FightingStyleDueling().String(): func(data json.RawMessage) (dnd5eEvents.ConditionBehavior, error) {
		dueling := NewFightingStyleDuelingCondition("")
		if err := dueling.loadJSON(data); err != nil {
			return nil, rpgerr.Wrap(err, "failed to load dueling fighting style condition")
		}
		return dueling, nil
	},
	refs.Conditions.FightingStyleGreatWeaponFighting().String(): func(data json.RawMessage) (dnd5eEvents.ConditionBehavior, error) {
		gwf := NewFightingStyleGreatWeaponFightingCondition("", nil)
		if err := gwf.loadJSON(data); err != nil {
			return nil, rpgerr.Wrap(err, "failed to load great weapon fighting style condition")
		}
		return gwf, nil
	},
	refs.Conditions.FightingStyleProtection().String(): func(data json.RawMessage) (dnd5eEvents.ConditionBehavior, error) {
		protection := NewFightingStyleProtectionCondition("")
		if err := protection.loadJSON(data); err != nil {
			return nil, rpgerr.Wrap(err, "failed to load protection fighting style condition")
		}
		return protection, nil
	},
	refs.Conditions.FightingStyleTwoWeaponFighting().String(): func(data json.RawMessage) (dnd5eEvents.ConditionBehavior, error) {
		twf := NewFightingStyleTwoWeaponFightingCondition("")
		if err := twf.loadJSON(data); err != nil {
			return nil, rpgerr.Wrap(err, "failed to load two-weapon fighting style condition")
		}
		return twf, nil
	},
	refs.Conditions.ImprovedCritical().String(): func(data json.RawMessage) (dnd5eEvents.ConditionBehavior, error) {
		ic := &ImprovedCriticalCondition{}
		if err := ic.loadJSON(data); err != nil {
			return nil, rpgerr.Wrap(err, "failed to load improved critical condition")
		}
		return ic, nil
	},
	refs.Conditions.RecklessAttack().String(): func(data json.RawMessage) (dnd5eEvents.ConditionBehavior, error) {
		ra := &RecklessAttackCondition{}
		if err := ra.loadJSON(data); err != nil {
			return nil, rpgerr.Wrap(err, "failed to load reckless attack condition")
		}
		return ra, nil
	},
	refs.Conditions.MartialArts().String(): func(data json.RawMessage) (dnd5eEvents.ConditionBehavior, error) {
		ma := &MartialArtsCondition{}
		if err := ma.loadJSON(data); err != nil {
			return nil, rpgerr.Wrap(err, "failed to load martial arts condition")
		}
		return ma, nil
	},
	refs.Conditions.UnarmoredMovement().String(): func(data json.RawMessage) (dnd5eEvents.ConditionBehavior, error) {
		um := &UnarmoredMovementCondition{}
		if err := um.loadJSON(data); err != nil {
			return nil, rpgerr.Wrap(err, "failed to load unarmored movement condition")
		}
		return um, nil
	},
	refs.Features.SneakAttack().String(): func(data json.RawMessage) (dnd5eEvents.ConditionBehavior, error) {
		sneak := &SneakAttackCondition{}
		if err := sneak.loadJSON(data); err != nil {
			return nil, rpgerr.Wrap(err, "failed to load sneak attack condition")
		}
		return sneak, nil
	},
	refs.Conditions.Disengaging().String(): func(data json.RawMessage) (dnd5eEvents.ConditionBehavior, error) {
		disengaging := &DisengagingCondition{}
		if err := disengaging.loadJSON(data); err != nil {
			return nil, rpgerr.Wrap(err, "failed to load disengaging condition")
		}
		return disengaging, nil
	},
	refs.Conditions.Dodging().String(): func(data json.RawMessage) (dnd5eEvents.ConditionBehavior, error) {
		dodging := &DodgingCondition{}
		if err := dodging.loadJSON(data); err != nil {
			return nil, rpgerr.Wrap(err, "failed to load dodging condition")
		}
		return dodging, nil
	},
	refs.Conditions.Prone().String(): func(data json.RawMessage) (dnd5eEvents.ConditionBehavior, error) {
		prone := &ProneCondition{}
		if err := prone.loadJSON(data); err != nil {
			return nil, rpgerr.Wrap(err, "failed to load prone condition")
		}
		return prone, nil
	},
	refs.Conditions.Hidden().String(): func(data json.RawMessage) (dnd5eEvents.ConditionBehavior, error) {
		hidden := &HiddenCondition{}
		if err := hidden.loadJSON(data); err != nil {
			return nil, rpgerr.Wrap(err, "failed to load hidden condition")
		}
		return hidden, nil
	},
	refs.Conditions.Helped().String(): func(data json.RawMessage) (dnd5eEvents.ConditionBehavior, error) {
		helped := &HelpedCondition{}
		if err := helped.loadJSON(data); err != nil {
			return nil, rpgerr.Wrap(err, "failed to load helped condition")
		}
		return helped, nil
	},
	refs.Conditions.Unconscious().String(): func(data json.RawMessage) (dnd5eEvents.ConditionBehavior, error) {
		uc := &UnconsciousCondition{}
		if err := uc.loadJSON(data); err != nil {
			return nil, rpgerr.Wrap(err, "failed to load unconscious condition")
		}
		return uc, nil
	},
	refs.Conditions.OpportunityAttack().String(): func(data json.RawMessage) (dnd5eEvents.ConditionBehavior, error) {
		oa := &OpportunityAttackCondition{}
		if err := oa.loadJSON(data); err != nil {
			return nil, rpgerr.Wrap(err, "failed to load opportunity attack condition")
		}
		return oa, nil
	},
	refs.Spells.Shield().String(): func(data json.RawMessage) (dnd5eEvents.ConditionBehavior, error) {
		sh := &ShieldSpellCondition{}
		if err := sh.loadJSON(data); err != nil {
			return nil, rpgerr.Wrap(err, "failed to load shield spell condition")
		}
		return sh, nil
	},
}

// LoadJSON loads a condition from its JSON representation.
// The game server stores conditions as opaque JSON blobs;
// this function deserializes them into strongly-typed structs.
func LoadJSON(data json.RawMessage) (dnd5eEvents.ConditionBehavior, error) {
	// Peek at the complete ref so refs with a known ID under the wrong module
	// or type cannot route to a condition they do not canonically name.
	var peek struct {
		Ref core.Ref `json:"ref"`
	}

	if err := json.Unmarshal(data, &peek); err != nil {
		return nil, rpgerr.Wrap(err, "failed to peek at condition ref")
	}

	load, ok := conditionLoaders[peek.Ref.String()]
	if !ok {
		return nil, rpgerr.Newf(rpgerr.CodeInvalidArgument, "unknown condition ref: %s", peek.Ref.ID)
	}

	return load(data)
}
