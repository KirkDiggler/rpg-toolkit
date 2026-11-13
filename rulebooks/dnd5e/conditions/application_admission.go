// Copyright (C) 2026 Kirk Diggler
// SPDX-License-Identifier: GPL-3.0-or-later

package conditions

import (
	"fmt"

	generic "github.com/KirkDiggler/rpg-toolkit/mechanics/effects"
	dndevents "github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/events"
)

// Admission reads supplied current holdings and fixed pending intent. It adds
// no reservation store and grants replacement only through the exact old owner.
func (p *PreparedApplication) checkAdmission(existing []ParticipantConditions, planned map[dndevents.ConditionAddress]bool, resuming bool) error {
	allowedID := ""
	if resuming {
		allowedID = p.id
	} else if p.replacement != nil {
		allowedID = p.replacement.SourceID
	}
	var allowed *LinkedCondition
	for _, participant := range existing {
		for _, condition := range participant.Conditions {
			if owner, ok := condition.(*LinkedCondition); ok && owner.role == LinkedCoordinator && owner.application == allowedID && owner.member == p.coordinator {
				allowed = owner
			}
		}
	}
	for _, participant := range existing {
		for _, condition := range participant.Conditions {
			if condition == nil || condition.Ref() == nil {
				return fmt.Errorf("preflight received an unnamed existing condition")
			}
			address := ConditionAddressOf(participant.MemberID, condition)
			if address.MemberID != participant.MemberID {
				return fmt.Errorf("existing condition belongs to a foreign holder")
			}
			if planned[address] && !ownedForAdmission(allowed, address, condition) {
				return fmt.Errorf("intended address %v collides with an unrelated existing attachment", address)
			}
			owner, ok := condition.(*LinkedCondition)
			if !ok || owner.role != LinkedCoordinator {
				continue
			}
			for _, key := range owner.lifetime.Snapshot().PendingMembers {
				member, ok := owner.memberBinding(key)
				if !ok {
					return fmt.Errorf("pending owner has an unknown intended binding")
				}
				if planned[member.Address] && owner != allowed {
					return fmt.Errorf("intended address %v collides with another owner's pending binding", member.Address)
				}
			}
		}
	}
	return nil
}
func ownedForAdmission(owner *LinkedCondition, address dndevents.ConditionAddress, condition dndevents.ConditionBehavior) bool {
	if owner == nil || owner.lifetime.Snapshot().Phase != generic.LifetimeLive {
		return false
	}
	if condition == owner {
		return true
	}
	for _, key := range owner.lifetime.Snapshot().LiveMembers {
		member, ok := owner.memberBinding(key)
		if !ok || member.Address != address {
			continue
		}
		linked, isLinked := condition.(*LinkedCondition)
		if member.Mode == LinkAdopted {
			return !isLinked
		}
		return isLinked && linked.application == owner.application && linked.role == LinkedRecipient && linked.bindings.Owner != nil && *linked.bindings.Owner == owner.ConditionAddress() && linked.bindings.MemberKey == key
	}
	return false
}
