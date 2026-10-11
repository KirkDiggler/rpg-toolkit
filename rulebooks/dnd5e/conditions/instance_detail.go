// Copyright (C) 2026 Kirk Diggler
// SPDX-License-Identifier: GPL-3.0-or-later

package conditions

import (
	dnd5eEvents "github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/events"
)

// InstanceDetailProvider is the optional capability of a condition instance
// that has something to say about itself which its type cannot: Command's
// word. A type with nothing per-instance to say does not implement it.
type InstanceDetailProvider interface {
	// InstanceDetail is free text the instance writes about itself. Empty
	// means the instance has nothing to say.
	InstanceDetail() string
}

// InstanceDetailOf is the one read of an instance's detail. A condition that
// does not implement InstanceDetailProvider has none. Callers never assert the
// interface themselves and never write a detail from anything but the instance.
func InstanceDetailOf(condition dnd5eEvents.ConditionBehavior) string {
	if provider, ok := condition.(InstanceDetailProvider); ok {
		return provider.InstanceDetail()
	}
	return ""
}
