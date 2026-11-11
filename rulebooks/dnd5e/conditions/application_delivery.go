// Copyright (C) 2026 Kirk Diggler
// SPDX-License-Identifier: GPL-3.0-or-later

package conditions

import (
	"context"
	"fmt"
	"slices"

	generic "github.com/KirkDiggler/rpg-toolkit/mechanics/effects"
	dndevents "github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/events"
)

type applicationDelivery struct {
	condition   dndevents.ConditionBehavior
	application string
}

func (b *ApplicationBinding) validateCoordinator(owner *LinkedCondition) error {
	if existing := b.owners[owner.application]; existing != nil && existing != owner {
		return fmt.Errorf("application %q already has a coordinator", owner.application)
	}
	for _, existing := range b.owners {
		if existing != owner && existing.member == owner.member && existing.concentrates && owner.concentrates {
			return fmt.Errorf("maintainer still holds another concentration application")
		}
	}
	for _, key := range owner.lifetime.Snapshot().PendingMembers {
		member, ok := owner.memberBinding(key)
		if !ok || !b.holders[member.Address.MemberID] {
			return fmt.Errorf("pending member requires its intended binding and loaded holder")
		}
		if child, ok := b.conditions[member.Address].(*LinkedCondition); ok && child.application == owner.application {
			return fmt.Errorf("pending member %q has an unfinalized actual attachment", key)
		}
		if delivery, ok := b.delivered[member.Address]; ok && delivery.application == owner.application {
			return fmt.Errorf("pending member %q has an unfinalized actual delivery", key)
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

func (b *ApplicationBinding) intendedDelivery(application string, address dndevents.ConditionAddress) (*LinkedCondition, LinkedMemberBinding, error) {
	owner := b.owners[application]
	if owner == nil || !owner.IsApplied() || owner.lifetime.Snapshot().Phase != generic.LifetimeLive {
		return nil, LinkedMemberBinding{}, fmt.Errorf("delivery has no current attached application owner")
	}
	for _, member := range owner.bindings.Members {
		if member.Address == address {
			if !slices.Contains(owner.lifetime.Snapshot().PendingMembers, member.Key) {
				return nil, LinkedMemberBinding{}, fmt.Errorf("recipient %q is already finalized", member.Key)
			}
			return owner, member, nil
		}
	}
	return nil, LinkedMemberBinding{}, fmt.Errorf("delivery address is outside the fixed intended list")
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
	application := event.ApplicationID
	linked, isLinked := event.Condition.(*LinkedCondition)
	if isLinked {
		if application != "" && application != linked.application {
			return fmt.Errorf("delivery names a foreign application")
		}
		application = linked.application
	}
	if existing := b.conditions[address]; existing != nil {
		if existing == event.Condition {
			if !isLinked && application != "" {
				expected := b.delivered[address].application
				if claim, ok := b.owned[address]; ok {
					expected = claim.owner.application
				}
				if application != expected {
					return fmt.Errorf("delivery names a foreign application")
				}
			}
			return nil
		}
		return fmt.Errorf("delivery must remove the exact existing address before replacement")
	}
	if isLinked {
		if linked.transport != nil && linked.transport != b {
			return fmt.Errorf("delivery is bound to another interaction")
		}
		if linked.role == LinkedCoordinator {
			if err := b.validateCoordinator(linked); err != nil {
				return err
			}
			if len(linked.lifetime.Snapshot().LiveMembers) != 0 {
				return fmt.Errorf("new coordinator delivery must begin with pending recipients")
			}
			b.owners[linked.application] = linked
		} else {
			owner, intended, err := b.intendedDelivery(application, address)
			if err != nil {
				return err
			}
			if intended.Mode != LinkComposed || intended.Key != linked.bindings.MemberKey || *linked.bindings.Owner != owner.ConditionAddress() {
				return fmt.Errorf("recipient disagrees with its fixed intended binding")
			}
		}
		linked.transport = b
		b.linked = append(b.linked, linked)
	} else if application != "" {
		_, intended, err := b.intendedDelivery(application, address)
		if err != nil {
			return err
		}
		if intended.Mode != LinkAdopted {
			return fmt.Errorf("delivery does not match its linked mode")
		}
	} else {
		for _, owner := range b.owners {
			for _, key := range owner.lifetime.Snapshot().PendingMembers {
				intended, _ := owner.memberBinding(key)
				if intended.Address == address {
					return fmt.Errorf("adopted intended delivery requires its application id")
				}
			}
		}
	}
	b.conditions[address] = event.Condition
	if !isLinked || linked.role == LinkedRecipient {
		b.delivered[address] = applicationDelivery{condition: event.Condition, application: application}
	}
	return nil
}

// FinalizeRecipientInput resolves one fixed intended shared entry after actual
// delivery or a no-effect outcome. ApplicationID identifies this invocation.
type FinalizeRecipientInput struct {
	ApplicationID string
	Key           string
	Delivered     bool
}

// FinalizeRecipientOutput reports actual live cardinality, not pending entries.
type FinalizeRecipientOutput struct {
	Before int
	After  int
}

// FinalizeRecipient validates real delivery before resolving a pending entry once.
// Missing/ended owners, wrong generations, failed attachment, repeated resolution
// and invented no-effect outcomes are errors. It marks coordinator state dirty.
func (b *ApplicationBinding) FinalizeRecipient(ctx context.Context, in *FinalizeRecipientInput) (*FinalizeRecipientOutput, error) {
	if in == nil || b.closed {
		return nil, fmt.Errorf("finalization requires input and live binding")
	}
	owner := b.owners[in.ApplicationID]
	if owner == nil || !owner.IsApplied() || owner.lifetime.Snapshot().Phase != generic.LifetimeLive {
		return nil, fmt.Errorf("finalization has no current attached application owner")
	}
	member, ok := owner.memberBinding(in.Key)
	if !ok || !slices.Contains(owner.lifetime.Snapshot().PendingMembers, in.Key) {
		return nil, fmt.Errorf("unknown or already finalized recipient %q", in.Key)
	}
	condition := b.conditions[member.Address]
	delivery := b.delivered[member.Address]
	if in.Delivered {
		if condition == nil || !condition.IsApplied() || delivery.condition != condition || delivery.application != owner.application {
			return nil, fmt.Errorf("finalization requires actual applied delivery from this application")
		}
		if _, claimed := b.owned[member.Address]; claimed {
			return nil, fmt.Errorf("delivery already belongs to another live membership")
		}
		if err := b.validateMember(owner, member); err != nil {
			return nil, err
		}
	} else {
		if delivery.condition != nil && delivery.application == owner.application {
			return nil, fmt.Errorf("no-effect outcome contradicts actual delivery")
		}
		if linked, ok := condition.(*LinkedCondition); ok && linked.application == owner.application {
			return nil, fmt.Errorf("no-effect outcome contradicts linked attachment")
		}
	}
	finalized, err := owner.lifetime.FinalizeMember(generic.FinalizeMemberInput{Key: in.Key, Delivered: in.Delivered})
	if err != nil {
		return nil, err
	}
	if in.Delivered {
		b.owned[member.Address] = linkedOwnership{owner: owner, key: member.Key, mode: member.Mode}
	}
	if err := owner.partChanged(ctx, b.bus, generic.AssemblyChange{}); err != nil {
		return nil, err
	}
	return &FinalizeRecipientOutput{Before: finalized.Before, After: finalized.After}, nil
}

// CheckComplete validates every successful mutable boundary, including a pause.
// Pending intended entries without an attachment are valid. Orphaned/unfinalized
// actual recipients, failed attachment and incomplete end state remain errors.
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
					return fmt.Errorf("recipient %v has no finalized owner membership", address)
				}
			}
		}
	}
	return nil
}
