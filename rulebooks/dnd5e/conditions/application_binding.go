// Copyright (C) 2026 Kirk Diggler
// SPDX-License-Identifier: GPL-3.0-or-later

package conditions

import (
	"context"
	"errors"
	"fmt"
	"strings"

	"github.com/KirkDiggler/rpg-toolkit/events"
	generic "github.com/KirkDiggler/rpg-toolkit/mechanics/effects"
	dndevents "github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/events"
)

// ParticipantConditions names the complete supplied condition universe for one
// participant. Missing holders are not inferred from whoever happened to attach.
type ParticipantConditions struct {
	MemberID   string
	Conditions []dndevents.ConditionBehavior
}

// BindApplicationsInput supplies already-loaded conditions and the interaction's
// only bus. Binding validates before installing any handler or runtime reference.
type BindApplicationsInput struct {
	Participants []ParticipantConditions
	Bus          events.EventBus
}

type linkedOwnership struct {
	owner *LinkedCondition
	key   string
	mode  LinkMode
}

// ApplicationBinding is an interaction-local address index and lifecycle
// transport. Its durable authorities remain in the coordinator/recipient data;
// Close revokes the transport without ending those applications.
type ApplicationBinding struct {
	bus          events.EventBus
	holders      map[string]bool
	conditions   map[dndevents.ConditionAddress]dndevents.ConditionBehavior
	owners       map[string]*LinkedCondition
	owned        map[dndevents.ConditionAddress]linkedOwnership
	linked       []*LinkedCondition
	removedSubID string
	appliedSubID string
	closed       bool
}

// BindApplications validates exact links, competing ownership and concentration
// claims across the supplied universe, then binds removal transport. It does not
// attach parts, mutate membership, spend, roll or read an external repository.
func BindApplications(in *BindApplicationsInput) (*ApplicationBinding, error) {
	if in == nil || in.Bus == nil {
		return nil, fmt.Errorf("application binding requires participants and a bus")
	}
	b := &ApplicationBinding{bus: in.Bus, holders: map[string]bool{},
		conditions: map[dndevents.ConditionAddress]dndevents.ConditionBehavior{},
		owners:     map[string]*LinkedCondition{}, owned: map[dndevents.ConditionAddress]linkedOwnership{}}
	claims := map[string]bool{}
	for _, participant := range in.Participants {
		if strings.TrimSpace(participant.MemberID) == "" || b.holders[participant.MemberID] {
			return nil, fmt.Errorf("application universe contains a missing or duplicate holder")
		}
		b.holders[participant.MemberID] = true
		for _, condition := range participant.Conditions {
			if condition == nil || condition.Ref() == nil {
				return nil, fmt.Errorf("holder %q has an unnamed condition", participant.MemberID)
			}
			address := ConditionAddressOf(participant.MemberID, condition)
			if address.MemberID != participant.MemberID {
				return nil, fmt.Errorf("condition address belongs to another holder")
			}
			if _, duplicate := b.conditions[address]; duplicate {
				return nil, fmt.Errorf("duplicate condition address %v", address)
			}
			b.conditions[address] = condition
			if linked, ok := condition.(*LinkedCondition); ok {
				if linked.transport != nil || linked.IsApplied() {
					return nil, fmt.Errorf("linked binding requires unbound, not-yet-attached conditions")
				}
				b.linked = append(b.linked, linked)
				if linked.role == LinkedCoordinator {
					if b.owners[linked.application] != nil {
						return nil, fmt.Errorf("duplicate live application %q", linked.application)
					}
					b.owners[linked.application] = linked
					info, err := linked.MaintenanceInfo()
					if err != nil {
						return nil, err
					}
					if info.Concentration != nil {
						if claims[linked.member] {
							return nil, fmt.Errorf("holder %q has multiple concentration claims", linked.member)
						}
						claims[linked.member] = true
					}
				}
			}
		}
	}
	for _, linked := range b.linked {
		if linked.role != LinkedCoordinator {
			continue
		}
		for _, key := range linked.lifetime.Snapshot().LiveMembers {
			member, ok := linked.memberBinding(key)
			if !ok {
				return nil, fmt.Errorf("application %q has unbound member %q", linked.application, key)
			}
			if _, claimed := b.owned[member.Address]; claimed {
				return nil, fmt.Errorf("condition %v is claimed by multiple owners", member.Address)
			}
			if err := b.validateMember(linked, member); err != nil {
				return nil, err
			}
			b.owned[member.Address] = linkedOwnership{owner: linked, key: key, mode: member.Mode}
		}
	}
	for _, linked := range b.linked {
		if linked.role == LinkedRecipient {
			claim, ok := b.owned[linked.ConditionAddress()]
			if !ok || claim.owner.application != linked.application || claim.key != linked.bindings.MemberKey {
				return nil, fmt.Errorf("recipient %v has no matching live owner membership", linked.ConditionAddress())
			}
		}
	}
	id, err := dndevents.ConditionRemovedTopic.On(b.bus).Subscribe(context.Background(), b.onRemoved)
	if err != nil {
		return nil, err
	}
	b.removedSubID = id
	appliedID, err := dndevents.ConditionAppliedTopic.On(b.bus).Subscribe(context.Background(), b.onApplied)
	if err != nil {
		return nil, errors.Join(err, b.bus.Unsubscribe(context.Background(), id))
	}
	b.appliedSubID = appliedID
	for _, linked := range b.linked {
		linked.transport = b
	}
	return b, nil
}

func (c *LinkedCondition) memberBinding(key string) (LinkedMemberBinding, bool) {
	for _, member := range c.bindings.Members {
		if member.Key == key {
			return member, true
		}
	}
	return LinkedMemberBinding{}, false
}

func (b *ApplicationBinding) validateMember(owner *LinkedCondition, member LinkedMemberBinding) error {
	if !b.holders[member.Address.MemberID] {
		return fmt.Errorf("application %q requires missing holder %q", owner.application, member.Address.MemberID)
	}
	condition, present := b.conditions[member.Address]
	if !present {
		return fmt.Errorf("application %q requires missing member %v", owner.application, member.Address)
	}
	linked, isLinked := condition.(*LinkedCondition)
	if member.Mode == LinkComposed {
		if !isLinked || linked.role != LinkedRecipient || linked.application != owner.application ||
			linked.bindings.MemberKey != member.Key || *linked.bindings.Owner != owner.ConditionAddress() {
			return fmt.Errorf("member %v belongs to a foreign application or owner", member.Address)
		}
	} else if isLinked {
		return fmt.Errorf("linked member %v cannot be adopted without its backlink", member.Address)
	}
	return nil
}

func (b *ApplicationBinding) onRemoved(ctx context.Context, event dndevents.ConditionRemovedEvent) error {
	if b.closed {
		return nil
	}
	address := event.Address()
	condition, present := b.conditions[address]
	if !present {
		return nil // a duplicate fact cannot discover a replacement by ref
	}
	if event.ApplicationID != "" {
		id := ""
		if linked, ok := condition.(*LinkedCondition); ok {
			id = linked.application
		} else if claim, ok := b.owned[address]; ok {
			id = claim.owner.application
		}
		if id != event.ApplicationID {
			return fmt.Errorf("removal names a stale or foreign application")
		}
	}
	if linked, ok := condition.(*LinkedCondition); ok && linked.role == LinkedCoordinator {
		if linked.lifetime.Snapshot().Phase == generic.LifetimeLive {
			return b.endOwner(ctx, linked, "external", event.Reason, true)
		}
		delete(b.conditions, address)
		delete(b.owners, linked.application)
		return nil
	}
	claim, claimed := b.owned[address]
	if claimed {
		member, ok := claim.owner.memberBinding(claim.key)
		if !ok {
			return fmt.Errorf("removed member has no bound address")
		}
		if err := b.validateMember(claim.owner, member); err != nil {
			return err
		}
	}
	delete(b.conditions, address)
	delete(b.owned, address)
	if !claimed || claim.owner.lifetime.Snapshot().Phase != generic.LifetimeLive {
		return nil
	}
	lost, err := claim.owner.lifetime.MemberRemoved(generic.MemberRemovedInput{Key: claim.key})
	if err != nil || !lost.Removed {
		return err
	}
	if err := claim.owner.partChanged(ctx, b.bus, generic.AssemblyChange{}); err != nil {
		return err
	}
	return dndevents.ApplicationMembersChangedTopic.On(b.bus).Publish(ctx, dndevents.ApplicationMembersChangedEvent{
		ApplicationID: claim.owner.application, Owner: claim.owner.ConditionAddress(), Before: lost.Before, After: lost.After,
	})
}

func (b *ApplicationBinding) endCondition(ctx context.Context, request linkedEndRequest) error {
	if b.closed || request.Condition == nil || b.conditions[request.Condition.ConditionAddress()] != request.Condition {
		return fmt.Errorf("application end has no current bound attachment")
	}
	if request.Scope == generic.EndPart {
		address := request.Condition.ConditionAddress()
		return dndevents.ConditionRemovedTopic.On(b.bus).Publish(ctx, dndevents.ConditionRemovedEvent{
			MemberID: address.MemberID, ConditionRef: address.ConditionRef, SourceID: address.SourceID, ApplicationID: request.Condition.application, Reason: request.Reason,
		})
	}
	if request.Condition.role != LinkedCoordinator {
		return fmt.Errorf("recipient cannot request application upkeep end")
	}
	return b.endOwner(ctx, request.Condition, "part:"+request.PartKey, request.Reason, false)
}

func (b *ApplicationBinding) endOwner(ctx context.Context, owner *LinkedCondition, requestor, reason string, ownerFact bool) error {
	if owner.lifetime.Snapshot().Phase != generic.LifetimeLive {
		return nil
	}
	keys := owner.lifetime.Snapshot().LiveMembers
	members := make([]LinkedMemberBinding, 0, len(keys))
	for _, key := range keys {
		member, ok := owner.memberBinding(key)
		if !ok {
			return fmt.Errorf("application end has unknown member %q", key)
		}
		if err := b.validateMember(owner, member); err != nil {
			return err
		}
		if !b.conditions[member.Address].IsApplied() {
			return fmt.Errorf("application end requires attached member %v", member.Address)
		}
		members = append(members, member)
	}
	if !owner.IsApplied() {
		return fmt.Errorf("application end requires its attached coordinator")
	}
	plan, err := owner.lifetime.RequestEnd(generic.EndRequest{Requestor: requestor, Reason: reason})
	if err != nil || !plan.Accepted {
		return err
	}
	if err := owner.assembly.BeginEnd(); err != nil {
		return err
	}
	for _, member := range members {
		if linked, ok := b.conditions[member.Address].(*LinkedCondition); ok {
			if err := linked.assembly.BeginEnd(); err != nil {
				return err
			}
		}
	}
	removed := make([]dndevents.ConditionAddress, 0, len(members))
	for _, member := range members {
		address := member.Address
		if err := dndevents.ConditionRemovedTopic.On(b.bus).Publish(ctx, dndevents.ConditionRemovedEvent{
			MemberID: address.MemberID, ConditionRef: address.ConditionRef, SourceID: address.SourceID, ApplicationID: owner.application, Reason: plan.Reason,
		}); err != nil {
			return err
		}
		removed = append(removed, address)
	}
	if owner.concentrates {
		if err := dndevents.ConcentrationEndedTopic.On(b.bus).Publish(ctx, dndevents.ConcentrationEndedEvent{
			CasterID: owner.member, SpellRef: owner.origin.Ref.String(), SpellName: owner.bindings.Name,
			Reason: plan.Reason, Removed: removed,
		}); err != nil {
			return err
		}
	}
	if !ownerFact {
		address := owner.ConditionAddress()
		if err := dndevents.ConditionRemovedTopic.On(b.bus).Publish(ctx, dndevents.ConditionRemovedEvent{
			MemberID: address.MemberID, ConditionRef: address.ConditionRef, SourceID: address.SourceID, ApplicationID: owner.application, Reason: plan.Reason,
		}); err != nil {
			return err
		}
	} else {
		delete(b.conditions, owner.ConditionAddress())
		delete(b.owners, owner.application)
		if err := owner.Remove(ctx, b.bus); err != nil {
			return err
		}
	}
	return owner.lifetime.CompleteEnd()
}

// Close detaches the interaction transport and runtime links without ending any
// saved application. Repeated close is harmless; no global registration remains.
func (b *ApplicationBinding) Close(ctx context.Context) error {
	if b.closed {
		return nil
	}
	if err := errors.Join(b.bus.Unsubscribe(ctx, b.removedSubID), b.bus.Unsubscribe(ctx, b.appliedSubID)); err != nil {
		return err
	}
	b.closed = true
	for _, linked := range b.linked {
		if linked.transport == b {
			linked.transport = nil
		}
	}
	return nil
}
