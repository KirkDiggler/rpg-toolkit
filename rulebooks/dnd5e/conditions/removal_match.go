// Copyright (C) 2026 Kirk Diggler
// SPDX-License-Identifier: GPL-3.0-or-later

package conditions

import dndevents "github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/events"

// ConditionRemovalMatchesInput names the held instance and the fact delivered to
// its keeper. Application identity is separate from reusable replacement identity.
type ConditionRemovalMatchesInput struct {
	MemberID  string
	Condition dndevents.ConditionBehavior
	Event     dndevents.ConditionRemovedEvent
}

// ConditionRemovalMatches refuses a linked fact for another invocation even
// when its replacement address is identical. Current unversioned facts still
// name their exact address. Adopted payload ownership is checked by the complete
// application binding, because the payload itself has no invented backlink.
func ConditionRemovalMatches(in ConditionRemovalMatchesInput) bool {
	if in.Condition == nil || in.Condition.Ref() == nil || ConditionAddressOf(in.MemberID, in.Condition) != in.Event.Address() {
		return false
	}
	if in.Event.ApplicationID != "" {
		if linked, ok := in.Condition.(interface{ LinkedApplicationInfo() LinkedApplicationInfo }); ok {
			return linked.LinkedApplicationInfo().ID == in.Event.ApplicationID
		}
	}
	return true
}
