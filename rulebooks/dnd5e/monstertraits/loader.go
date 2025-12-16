// Copyright (C) 2024 Kirk Diggler
// SPDX-License-Identifier: GPL-3.0-or-later

package monstertraits

import (
	"encoding/json"

	"github.com/KirkDiggler/rpg-toolkit/core"
	"github.com/KirkDiggler/rpg-toolkit/dice"
	"github.com/KirkDiggler/rpg-toolkit/rpgerr"
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
