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
