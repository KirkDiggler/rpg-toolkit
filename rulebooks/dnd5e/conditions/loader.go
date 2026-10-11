// Copyright (C) 2024 Kirk Diggler
// SPDX-License-Identifier: GPL-3.0-or-later

package conditions

import (
	"encoding/json"
	"errors"

	"github.com/KirkDiggler/rpg-toolkit/core"
	"github.com/KirkDiggler/rpg-toolkit/rpgerr"
	dnd5eEvents "github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/events"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/refs"
)

// ErrRetiredCondition reports a saved condition whose type no longer exists.
// A sheet loader drops such an entry, under every load policy, rather than
// failing the sheet: the condition carried no rule, and what it recorded is
// now answered elsewhere. See [IsRetired].
var ErrRetiredCondition = errors.New("condition type is retired")

// retiredConditions are the refs a saved sheet may still carry for a condition
// type that no longer exists. A retired ref is not unregistered: it names a
// type that was deleted, and a reader drops it where it would refuse a ref
// nothing ever registered. [isRetiredRef] is the one read of this list.
//
// In Fog recorded membership in a Fog Cloud area. Membership is the
// encounter's answer, asked when needed and never stored on a sheet; its ref
// lives on only as the area's membership label.
//
// Unarmored Movement was a condition that listened to nothing: Speed reads the
// monk's levels itself, so the blob carried no rule.
//
// The Shield spell condition was a reaction nothing playable can hold yet; it
// returns on the post-roll window when a sheet can hold it, so no dead entry
// stands in for it meanwhile.
var retiredConditions = map[string]bool{
	refs.Conditions.InFog().String():             true,
	refs.Conditions.UnarmoredMovement().String(): true,
	refs.Spells.Shield().String():                true,
}

// isRetiredRef reports whether ref names a retired condition type. It is the
// membership read of the retired list, so every reader uses one list.
func isRetiredRef(ref string) bool {
	return retiredConditions[ref]
}

// IsRetired reports whether data is a saved condition of a retired type,
// which a sheet loader drops. Data that does not parse is not retired; the
// loader reports it as it always has.
func IsRetired(data json.RawMessage) bool {
	var peek struct {
		Ref core.Ref `json:"ref"`
	}
	if err := json.Unmarshal(data, &peek); err != nil {
		return false
	}
	return isRetiredRef(peek.Ref.String())
}

// LoadJSON loads a condition from its JSON representation.
// The game server stores conditions as opaque JSON blobs;
// this function deserializes them into strongly-typed structs.
//
// Errors: unparsable data, an unregistered ref (wrapping [ErrUnknownCondition]), a retired type (wrapping
// [ErrRetiredCondition], which a sheet loader drops), or the condition's own
// load error.
func LoadJSON(data json.RawMessage) (dnd5eEvents.ConditionBehavior, error) {
	// Peek at the complete ref so refs with a known ID under the wrong module
	// or type cannot route to a condition they do not canonically name.
	var peek struct {
		Ref core.Ref `json:"ref"`
	}

	if err := json.Unmarshal(data, &peek); err != nil {
		return nil, rpgerr.Wrap(err, "failed to peek at condition ref")
	}

	if isRetiredRef(peek.Ref.String()) {
		return nil, rpgerr.WrapWithCode(ErrRetiredCondition, rpgerr.CodeInvalidArgument,
			"retired condition ref: "+peek.Ref.String())
	}

	entry, err := lookup(peek.Ref.String())
	if err != nil {
		return nil, err
	}

	return entry.load(data)
}
