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
//
// Errors: unparsable data, an unregistered ref (wrapping [ErrUnknownCondition]),
// or the condition's own load error. A ref whose type was removed from the
// game is unregistered like any other.
func LoadJSON(data json.RawMessage) (dnd5eEvents.ConditionBehavior, error) {
	// Peek at the complete ref so refs with a known ID under the wrong module
	// or type cannot route to a condition they do not canonically name.
	var peek struct {
		Ref core.Ref `json:"ref"`
	}

	if err := json.Unmarshal(data, &peek); err != nil {
		return nil, rpgerr.Wrap(err, "failed to peek at condition ref")
	}

	entry, err := lookup(peek.Ref.String())
	if err != nil {
		return nil, err
	}

	return entry.load(data)
}
