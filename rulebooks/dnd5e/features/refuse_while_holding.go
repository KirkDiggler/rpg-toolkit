// Copyright (C) 2026 Kirk Diggler
// SPDX-License-Identifier: GPL-3.0-or-later

package features

import (
	"github.com/KirkDiggler/rpg-toolkit/core"
	"github.com/KirkDiggler/rpg-toolkit/rpgerr"
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
