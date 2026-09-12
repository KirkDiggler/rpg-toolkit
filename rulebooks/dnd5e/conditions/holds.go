// Copyright (C) 2026 Kirk Diggler
// SPDX-License-Identifier: GPL-3.0-or-later

package conditions

import (
	"encoding/json"

	"github.com/KirkDiggler/rpg-toolkit/core"
	"github.com/KirkDiggler/rpg-toolkit/rpgerr"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/refs"
)

// storedRef is the one field every condition's ToJSON writes, and the only one
// these two readers need to route on.
type storedRef struct {
	Ref core.Ref `json:"ref"`
}

// HoldsRef reports whether one of a sheet's stored condition blobs is the given
// ref, reading only the "ref" field every ToJSON writes.
//
// IT LOADS NOTHING AND ATTACHES NOTHING. The caller is the seam deciding how a
// member participates in a turn, which happens well before anything is put on a
// bus — and building a live condition to ask a question about a sheet would
// mean subscriptions created and torn down for an answer that is one string
// comparison. [LoadJSON] remains the only way to get behaviour.
//
// A blob that cannot be read is an ERROR rather than a no. The two answers
// differ: a sheet that does not hold the condition is an ordinary turn, and a
// sheet whose conditions cannot be read is a sheet nobody should be deciding
// turns from.
//
// A nil ref is refused for a plainer reason than either: there is nothing to
// compare against. core.Ref's String is on the pointer and dereferences, so
// without this the call panics on its own argument before it reads a single
// blob — and a panic in the seam that decides how a member takes its turn is
// not an answer anybody can act on.
func HoldsRef(stored []json.RawMessage, ref *core.Ref) (bool, error) {
	if ref == nil {
		return false, rpgerr.New(rpgerr.CodeInvalidArgument,
			"cannot look for a nil condition ref: there is nothing to compare a stored blob against")
	}
	want := ref.String()
	for index, blob := range stored {
		var named storedRef
		if err := json.Unmarshal(blob, &named); err != nil {
			return false, rpgerr.Wrapf(err, "failed to read the ref of stored condition %d", index)
		}
		if named.Ref.String() == want {
			return true, nil
		}
	}
	return false, nil
}

// DecodeCommanded returns the data of the NEWEST Commanded blob on a sheet, and
// whether there was one.
//
// This is the driver's read: the anchor and the word are the two facts the
// layer running a compelled turn cannot derive from the map. Like [HoldsRef] it
// builds no condition — the sheet's stored form already carries everything, and
// the only thing a live condition adds is the clock, which is not this reader's
// question.
//
// THE NEWEST, because a sheet may legitimately hold two. The caster is part of
// this condition's identity (see [CommandedCondition.ConditionAddress]), so two
// casters' Commands stand side by side rather than displacing each other —
// nothing removes another caster's spell. One turn cannot obey two words, and
// the one it obeys is the one said last, which is the last a sheet appended.
// Both conditions still end on their own clocks; only the driver picks.
//
// Every blob is read, so an unreadable one anywhere on the sheet is reported
// even when a valid compulsion was found before it. That is the same judgement
// [HoldsRef] makes: a sheet whose conditions cannot all be read is a sheet
// nobody should be deciding turns from.
func DecodeCommanded(stored []json.RawMessage) (*CommandedConditionData, bool, error) {
	want := refs.Conditions.Commanded().String()
	var newest *CommandedConditionData
	for index, blob := range stored {
		var named storedRef
		if err := json.Unmarshal(blob, &named); err != nil {
			return nil, false, rpgerr.Wrapf(err, "failed to read the ref of stored condition %d", index)
		}
		if named.Ref.String() != want {
			continue
		}
		var data CommandedConditionData
		if err := json.Unmarshal(blob, &data); err != nil {
			return nil, false, rpgerr.Wrapf(err, "failed to read stored commanded condition %d", index)
		}
		newest = &data
	}
	if newest == nil {
		return nil, false, nil
	}
	return newest, true, nil
}
