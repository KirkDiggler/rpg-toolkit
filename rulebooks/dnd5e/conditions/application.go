// Copyright (C) 2026 Kirk Diggler
// SPDX-License-Identifier: GPL-3.0-or-later

package conditions

import (
	"encoding/json"
	"fmt"
	"slices"
	"strings"

	generic "github.com/KirkDiggler/rpg-toolkit/mechanics/effects"
	dndfx "github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/effects"
	dndevents "github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/events"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/refs"
)

// RecipientLifetime declares whether a delivery joins this shared lifetime.
type RecipientLifetime string

const (
	// LifetimeShared joins exact delivered attachments to the coordinator.
	LifetimeShared RecipientLifetime = "shared"
	// LifetimeIndependent leaves an existing current payload outside membership.
	LifetimeIndependent RecipientLifetime = "independent"
)

// RecipientSpec names one registered delivery shape. Linked payloads use Parts;
// current adopted payloads use Config. The two construction authorities cannot mix.
type RecipientSpec struct {
	Ref             string
	Name            string
	AddressSourceID string
	Parts           []dndfx.PartDeclaration
	Config          json.RawMessage
	Lifetime        RecipientLifetime
}

// ApplicationSpec is authored composition, not mutable upkeep or an action cost.
type ApplicationSpec struct {
	Name             string
	CoordinatorParts []dndfx.PartDeclaration
	Recipients       []RecipientSpec
}

// PrepareApplicationInput supplies identity and the actual preflight universe.
// No ID, holder, applying content or budget is invented by preparation.
type PrepareApplicationInput struct {
	ID            string
	CoordinatorID string
	Origin        dndevents.ConditionOrigin
	Spec          ApplicationSpec
	Existing      []ParticipantConditions
}

// PreparedApplication owns copied construction data. Its replacement is a pure
// description; the caller may deliver it only after the normal action door pays.
type PreparedApplication struct {
	id          string
	coordinator string
	origin      dndevents.ConditionOrigin
	spec        ApplicationSpec
	holders     map[string]bool
	replacement *dndevents.ConditionAddress
}

// ID returns this invocation's opaque identity, reused through resume/reload.
func (p *PreparedApplication) ID() string { return p.id }

// Replacement returns an exact current concentration owner by value, if any.
func (p *PreparedApplication) Replacement() (dndevents.ConditionAddress, bool) {
	if p.replacement == nil {
		return dndevents.ConditionAddress{}, false
	}
	return *p.replacement, true
}

// CreateRecipientInput binds a known target to one already-prepared delivery.
type CreateRecipientInput struct {
	Prepared  *PreparedApplication
	SpecIndex int
	MemberID  string
}

// DeliveryReceipt describes actual delivered identity; it is not permission to
// claim a condition that the interaction did not apply. SpecIndex avoids parsing Key.
type DeliveryReceipt struct {
	Key       string
	Address   dndevents.ConditionAddress
	LinkMode  LinkMode
	SpecIndex int
	Shared    bool
}

// CreateRecipientOutput provides behavior and its prospective receipt. The
// receipt becomes evidence only after the caller's real delivery succeeds.
type CreateRecipientOutput struct {
	Condition dndevents.ConditionBehavior
	Receipt   DeliveryReceipt
}

// SealApplicationInput names only successful deliveries in one bound interaction.
type SealApplicationInput struct {
	Prepared  *PreparedApplication
	Delivered []DeliveryReceipt
	Binding   *ApplicationBinding
}

// SealApplicationOutput is the coordinator the caller must actually deliver.
// CheckComplete refuses a result that omits that final application step.
type SealApplicationOutput struct{ Condition dndevents.ConditionBehavior }

func cloneDeclarations(in []dndfx.PartDeclaration) []dndfx.PartDeclaration {
	out := make([]dndfx.PartDeclaration, len(in))
	copy(out, in)
	for i := range out {
		out[i].Params = slices.Clone(in[i].Params)
	}
	return out
}

// PrepareApplication validates authored linked parts and describes concentration
// replacement without attaching, spending, rolling or displacing anything.
// Adopted configs are validated by CreateRecipient against an actual candidate;
// native preflight must construct those candidates before payment as well.
func PrepareApplication(in *PrepareApplicationInput) (*PreparedApplication, error) {
	if in == nil || strings.TrimSpace(in.ID) == "" || strings.TrimSpace(in.CoordinatorID) == "" || strings.TrimSpace(in.Spec.Name) == "" {
		return nil, fmt.Errorf("application preparation requires id, coordinator and name")
	}
	p := &PreparedApplication{id: in.ID, coordinator: in.CoordinatorID, origin: in.Origin, spec: in.Spec, holders: map[string]bool{}}
	p.spec.CoordinatorParts = cloneDeclarations(in.Spec.CoordinatorParts)
	p.spec.Recipients = slices.Clone(in.Spec.Recipients)
	owner, err := buildLinked(LinkedConditionData{Ref: *refs.Conditions.LinkedEffects(), MemberID: p.coordinator,
		AddressSourceID: p.id, ApplicationID: p.id, Role: LinkedCoordinator, Origin: &p.origin,
		Bindings: &LinkedBindings{Name: p.spec.Name, Parts: p.spec.CoordinatorParts}}, false)
	if err != nil {
		return nil, err
	}
	for _, participant := range in.Existing {
		if strings.TrimSpace(participant.MemberID) == "" || p.holders[participant.MemberID] {
			return nil, fmt.Errorf("preflight has missing or duplicate holder")
		}
		p.holders[participant.MemberID] = true
		for _, condition := range participant.Conditions {
			linked, ok := condition.(*LinkedCondition)
			if !ok {
				continue
			}
			if linked.application == p.id {
				return nil, fmt.Errorf("application id %q already exists", p.id)
			}
			if linked.role == LinkedCoordinator && linked.member == p.coordinator && linked.concentrates && owner.concentrates {
				if p.replacement != nil {
					return nil, fmt.Errorf("maintainer has multiple existing concentration claims")
				}
				address := linked.ConditionAddress()
				p.replacement = &address
			}
		}
	}
	if !p.holders[p.coordinator] {
		return nil, fmt.Errorf("coordinator is absent from the preflight universe")
	}
	for i := range p.spec.Recipients {
		spec := &p.spec.Recipients[i]
		entry, err := lookup(spec.Ref)
		if err != nil || entry.create == nil {
			return nil, fmt.Errorf("recipient %d does not name a creatable registered ref", i)
		}
		if spec.Lifetime != LifetimeShared && spec.Lifetime != LifetimeIndependent {
			return nil, fmt.Errorf("recipient %d needs an explicit lifetime policy", i)
		}
		spec.Parts = cloneDeclarations(in.Spec.Recipients[i].Parts)
		spec.Config = slices.Clone(in.Spec.Recipients[i].Config)
		if entry.linked {
			if spec.Lifetime != LifetimeShared || spec.AddressSourceID == "" || in.Spec.Recipients[i].Parts == nil || len(spec.Config) != 0 {
				return nil, fmt.Errorf("linked recipient %d requires shared parts and explicit address identity", i)
			}
			keys := map[string]bool{}
			for _, part := range spec.Parts {
				if keys[part.Key] || part.OnTerminal != generic.EndPart {
					return nil, fmt.Errorf("recipient %d has duplicate keys or application upkeep", i)
				}
				keys[part.Key] = true
				if err := dndfx.ValidatePart(&dndfx.ValidatePartInput{Declaration: part}); err != nil {
					return nil, err
				}
			}
		} else if len(spec.Parts) != 0 {
			return nil, fmt.Errorf("adopted recipient %d cannot carry linked part declarations", i)
		}
	}
	return p, nil
}

// CreateRecipient constructs against an actual admitted candidate, but does not
// apply it or spend. Its error therefore belongs in pure preflight, not after pay.
func CreateRecipient(in *CreateRecipientInput) (*CreateRecipientOutput, error) {
	if in == nil || in.Prepared == nil || in.SpecIndex < 0 || in.SpecIndex >= len(in.Prepared.spec.Recipients) {
		return nil, fmt.Errorf("recipient creation requires a prepared declaration")
	}
	p := in.Prepared
	if !p.holders[in.MemberID] {
		return nil, fmt.Errorf("recipient %q is outside the prepared universe", in.MemberID)
	}
	spec := p.spec.Recipients[in.SpecIndex]
	entry, err := lookup(spec.Ref)
	if err != nil {
		return nil, err
	}
	key := fmt.Sprintf("%d:%q", in.SpecIndex, in.MemberID)
	create := &CreateFromRefInput{Ref: spec.Ref, MemberID: in.MemberID, Config: slices.Clone(spec.Config)}
	mode := LinkAdopted
	if entry.linked {
		mode = LinkComposed
		name := spec.Name
		if name == "" {
			name = entry.display.Name
		}
		owner := dndevents.ConditionAddress{MemberID: p.coordinator, ConditionRef: refs.Conditions.LinkedEffects().String(), SourceID: p.id}
		config := LinkedConditionConfig{ApplicationID: p.id, Role: LinkedRecipient, AddressSourceID: spec.AddressSourceID,
			Bindings: &LinkedBindings{Name: name, Parts: cloneDeclarations(spec.Parts), Owner: &owner, MemberKey: key}}
		create.Config, err = json.Marshal(config)
		if err != nil {
			return nil, err
		}
	}
	if entry.acceptsOrigin {
		origin := p.origin
		create.Origin = &origin
	} else {
		create.SourceRef = p.origin.Ref.String()
	}
	created, err := CreateFromRef(create)
	if err != nil {
		return nil, err
	}
	return &CreateRecipientOutput{Condition: created.Condition, Receipt: DeliveryReceipt{
		Key: key, Address: ConditionAddressOf(in.MemberID, created.Condition), LinkMode: mode,
		SpecIndex: in.SpecIndex, Shared: spec.Lifetime == LifetimeShared,
	}}, nil
}

// SealApplication constructs a coordinator from real, applied receipts. It
// refuses duplicates, foreign applications, orphaned provisional recipients and
// a concentration owner the paid caller has not yet displaced. It does not publish.
func SealApplication(in *SealApplicationInput) (*SealApplicationOutput, error) {
	if in == nil || in.Prepared == nil || in.Binding == nil || in.Binding.closed {
		return nil, fmt.Errorf("sealing requires a prepared application and live binding")
	}
	p, b := in.Prepared, in.Binding
	if b.owners[p.id] != nil || !b.holders[p.coordinator] {
		return nil, fmt.Errorf("application already exists or coordinator is not loaded")
	}
	members := make([]LinkedMemberBinding, 0, len(in.Delivered))
	keys := map[string]bool{}
	addresses := map[dndevents.ConditionAddress]bool{}
	for _, receipt := range in.Delivered {
		if receipt.SpecIndex < 0 || receipt.SpecIndex >= len(p.spec.Recipients) || keys[receipt.Key] || addresses[receipt.Address] {
			return nil, fmt.Errorf("invalid or duplicate delivery receipt")
		}
		spec := p.spec.Recipients[receipt.SpecIndex]
		if receipt.Key != fmt.Sprintf("%d:%q", receipt.SpecIndex, receipt.Address.MemberID) ||
			receipt.Address.ConditionRef != spec.Ref || receipt.Shared != (spec.Lifetime == LifetimeShared) {
			return nil, fmt.Errorf("delivery receipt disagrees with its declaration")
		}
		condition := b.conditions[receipt.Address]
		if condition == nil || !condition.IsApplied() {
			return nil, fmt.Errorf("delivery receipt names no applied condition")
		}
		keys[receipt.Key], addresses[receipt.Address] = true, true
		if receipt.Shared {
			members = append(members, LinkedMemberBinding{Key: receipt.Key, Address: receipt.Address, Mode: receipt.LinkMode})
		}
	}
	coordinator, err := buildLinked(LinkedConditionData{Ref: *refs.Conditions.LinkedEffects(), MemberID: p.coordinator,
		AddressSourceID: p.id, ApplicationID: p.id, Role: LinkedCoordinator, Origin: &p.origin,
		Bindings: &LinkedBindings{Name: p.spec.Name, Parts: cloneDeclarations(p.spec.CoordinatorParts), Members: members}}, false)
	if err != nil {
		return nil, err
	}
	if err := b.validateCoordinator(coordinator); err != nil {
		return nil, err
	}
	for _, condition := range b.conditions {
		if linked, ok := condition.(*LinkedCondition); ok && linked.role == LinkedRecipient && linked.application == p.id {
			if !addresses[linked.ConditionAddress()] {
				return nil, fmt.Errorf("sealing would orphan an already-delivered recipient")
			}
		}
	}
	return &SealApplicationOutput{Condition: coordinator}, nil
}
