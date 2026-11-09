// Copyright (C) 2026 Kirk Diggler
// SPDX-License-Identifier: GPL-3.0-or-later

package conditions

import (
	"context"
	"fmt"

	generic "github.com/KirkDiggler/rpg-toolkit/mechanics/effects"
	dndevents "github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/events"
)

func (b *ApplicationBinding) validateCoordinator(owner *LinkedCondition) error {
	if existing := b.owners[owner.application]; existing != nil && existing != owner {
		return fmt.Errorf("application %q already has a coordinator", owner.application)
	}
	for _, existing := range b.owners {
		if existing != owner && existing.member == owner.member && existing.concentrates && owner.concentrates {
			return fmt.Errorf("maintainer still holds another concentration application")
		}
	}
	for _, key := range owner.lifetime.Snapshot().LiveMembers {
		member, ok := owner.memberBinding(key)
		if !ok {
			return fmt.Errorf("coordinator has an unbound live member")
		}
		if claim, claimed := b.owned[member.Address]; claimed && claim.owner != owner {
			return fmt.Errorf("condition %v is claimed by multiple owners", member.Address)
		}
		if err := b.validateMember(owner, member); err != nil {
			return err
		}
	}
	return nil
}

func (b *ApplicationBinding) onApplied(_ context.Context, event dndevents.ConditionAppliedEvent) error {
	if b.closed {
		return fmt.Errorf("application binding is closed")
	}
	if event.Target == nil || event.Condition == nil || event.Condition.Ref() == nil {
		return fmt.Errorf("application delivery needs a target and named condition")
	}
	member := event.Target.GetID()
	if !b.holders[member] {
		return fmt.Errorf("delivery target %q is outside the application universe", member)
	}
	address := ConditionAddressOf(member, event.Condition)
	if address.MemberID != member {
		return fmt.Errorf("delivery address names another holder")
	}
	if existing := b.conditions[address]; existing != nil {
		if existing == event.Condition {
			return nil
		}
		return fmt.Errorf("delivery must remove the exact existing address before replacement")
	}
	if linked, ok := event.Condition.(*LinkedCondition); ok {
		if linked.transport != nil && linked.transport != b {
			return fmt.Errorf("delivery is bound to another interaction")
		}
		if linked.role == LinkedCoordinator {
			if err := b.validateCoordinator(linked); err != nil {
				return err
			}
			for _, condition := range b.conditions {
				if child, isLinked := condition.(*LinkedCondition); isLinked && child.role == LinkedRecipient && child.application == linked.application {
					binding, present := linked.memberBinding(child.bindings.MemberKey)
					if !present || binding.Address != child.ConditionAddress() {
						return fmt.Errorf("coordinator delivery would orphan a provisional recipient")
					}
				}
			}
			b.owners[linked.application] = linked
			for _, key := range linked.lifetime.Snapshot().LiveMembers {
				binding, _ := linked.memberBinding(key)
				b.owned[binding.Address] = linkedOwnership{owner: linked, key: key, mode: binding.Mode}
			}
		} else if b.owners[linked.application] != nil {
			return fmt.Errorf("post-seal recipient recruitment is not supported")
		}
		linked.transport = b
		b.linked = append(b.linked, linked)
	}
	b.conditions[address] = event.Condition
	return nil
}

// CheckComplete validates the successful-result boundary after all deliveries.
// Provisional/orphaned recipients, failed attachment, or an unfinished end cannot
// be returned as a saveable application. It performs no cleanup or mutation.
func (b *ApplicationBinding) CheckComplete() error {
	if b.closed {
		return fmt.Errorf("application binding is closed")
	}
	for _, owner := range b.owners {
		if !owner.IsApplied() || owner.lifetime.Snapshot().Phase != generic.LifetimeLive {
			return fmt.Errorf("application %q is not completely attached/live", owner.application)
		}
		if err := b.validateCoordinator(owner); err != nil {
			return err
		}
	}
	for address, condition := range b.conditions {
		if linked, ok := condition.(*LinkedCondition); ok {
			if !linked.IsApplied() {
				return fmt.Errorf("linked attachment %v was not applied", address)
			}
			if linked.role == LinkedRecipient {
				claim, claimed := b.owned[address]
				if !claimed || claim.owner.application != linked.application || claim.key != linked.bindings.MemberKey {
					return fmt.Errorf("provisional recipient %v was not sealed", address)
				}
			}
		}
	}
	return nil
}
