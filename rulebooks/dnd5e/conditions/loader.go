// Copyright (C) 2024 Kirk Diggler
// SPDX-License-Identifier: GPL-3.0-or-later

package conditions

import (
	"encoding/json"

	"github.com/KirkDiggler/rpg-toolkit/core"
	"github.com/KirkDiggler/rpg-toolkit/rpgerr"
	dnd5eEvents "github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/events"
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
	case RagingID:
		raging := &RagingCondition{}
		if err := raging.loadJSON(data); err != nil {
			return nil, rpgerr.Wrap(err, "failed to load raging condition")
		}
		return raging, nil

	case BrutalCriticalID:
		brutal := &BrutalCriticalCondition{}
		if err := brutal.loadJSON(data); err != nil {
			return nil, rpgerr.Wrap(err, "failed to load brutal critical condition")
		}
		return brutal, nil

	case UnarmoredDefenseID:
		unarmored := &UnarmoredDefenseCondition{}
		if err := unarmored.loadJSON(data); err != nil {
			return nil, rpgerr.Wrap(err, "failed to load unarmored defense condition")
		}
		return unarmored, nil

	case FightingStyleID:
		fs := &FightingStyleCondition{}
		if err := fs.loadJSON(data); err != nil {
			return nil, rpgerr.Wrap(err, "failed to load fighting style condition")
		}
		return fs, nil

	default:
		return nil, rpgerr.Newf(rpgerr.CodeInvalidArgument, "unknown condition ref: %s", peek.Ref.ID)
	}
}
