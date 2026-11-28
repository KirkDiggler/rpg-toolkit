// Copyright (C) 2026 Kirk Diggler
// SPDX-License-Identifier: GPL-3.0-or-later

package features

import (
	"encoding/json"
	"strings"

	"github.com/KirkDiggler/rpg-toolkit/core"
	"github.com/KirkDiggler/rpg-toolkit/rpgerr"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/combat/actions"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/conditions"
	dnd5eEvents "github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/events"
)

// refuseWhileHoldingInput names the activating owner, the condition a
// self-applied feature puts on it, and the refusal's reason.
type refuseWhileHoldingInput struct {
	Owner  core.Entity
	Ref    *core.Ref
	Reason string
}

// refuseWhileHolding refuses activating a self-applied feature while its owner
// already holds the condition that feature applies. The sheet would replace
// the held condition with the new one (one condition per identity); the
// operator ruled on #1944 that an already-active Rage or Reckless Attack is
// refused instead, so activating it again burns no charge and no action. The
// refusal is CodeConflictingState with the given reason. An owner that cannot
// report its conditions is refused too — not knowing is not "not holding".
func refuseWhileHolding(in *refuseWhileHoldingInput) error {
	holder, ok := in.Owner.(interface {
		GetConditions() []dnd5eEvents.ConditionBehavior
	})
	if !ok {
		return rpgerr.New(rpgerr.CodeInvalidArgument, "owner does not expose its conditions")
	}
	for _, condition := range holder.GetConditions() {
		if condition != nil && condition.Ref() != nil && condition.Ref().String() == in.Ref.String() {
			return rpgerr.New(rpgerr.CodeConflictingState, in.Reason)
		}
	}
	return nil
}

// heldStatusReason is the one place the words of "she already holds it" are
// written: "already " and the status's display name in lower case, so Raging
// reads "already raging". A status with no display reads "already holding"
// and its ref.
func heldStatusReason(ref core.Ref) string {
	if display, ok := conditions.DisplayFor(ref); ok && display.Name != "" {
		return "already " + strings.ToLower(display.Name)
	}
	return "already holding " + ref.String()
}

// RefuseHeldStatusInput names a description and the live holder who would use it.
type RefuseHeldStatusInput struct {
	// Definition is the description about to be used. Required.
	Definition *actions.Definition
	// Holder is the live user, asked through GetConditions. Required.
	Holder core.Entity
}

// RefuseHeldStatus refuses a description that puts a status on its user while
// the user holds that status. A description that puts none, and a user who does
// not hold it, are not refused. The refusal is CodeConflictingState and its
// whole text is the reason, "already raging" for Rage. A holder that cannot
// report its conditions is refused: not knowing is not "not holding".
func RefuseHeldStatus(in *RefuseHeldStatusInput) error {
	if in == nil || in.Definition == nil {
		return rpgerr.New(rpgerr.CodeNil, "no description to ask whether its status is held")
	}
	ref, puts := in.Definition.StatusOnUser()
	if !puts {
		return nil
	}
	return refuseWhileHolding(&refuseWhileHoldingInput{Owner: in.Holder, Ref: &ref, Reason: heldStatusReason(ref)})
}

// RefuseHeldStatusStoredInput names a description and a stored sheet's statuses.
type RefuseHeldStatusStoredInput struct {
	// Definition is the description about to be used. Required.
	Definition *actions.Definition
	// Stored is the sheet's stored conditions.
	Stored []json.RawMessage
}

// RefuseHeldStatusStored asks [RefuseHeldStatus]'s question of a stored sheet
// through [conditions.HoldsRef]. A stored condition that cannot be read is an
// error, never a "not holding".
func RefuseHeldStatusStored(in *RefuseHeldStatusStoredInput) error {
	if in == nil || in.Definition == nil {
		return rpgerr.New(rpgerr.CodeNil, "no description to ask whether its status is held")
	}
	ref, puts := in.Definition.StatusOnUser()
	if !puts {
		return nil
	}
	held, err := conditions.HoldsRef(in.Stored, &ref)
	if err != nil {
		return rpgerr.Wrapf(err, "cannot tell whether the sheet already holds %s", ref.String())
	}
	if held {
		return rpgerr.New(rpgerr.CodeConflictingState, heldStatusReason(ref))
	}
	return nil
}
