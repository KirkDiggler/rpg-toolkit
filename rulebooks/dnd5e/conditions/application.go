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
	// LifetimeIndependent leaves a current payload outside shared membership.
	LifetimeIndependent RecipientLifetime = "independent"
)

// RecipientSpec fixes one intended holder and registered delivery shape. Linked
// payloads use Parts; adopted current payloads use Config. These cannot mix.
type RecipientSpec struct {
	MemberID        string                  `json:"member_id"`
	Ref             string                  `json:"ref"`
	Name            string                  `json:"name,omitempty"`
	AddressSourceID string                  `json:"address_source_id,omitempty"`
	Parts           []dndfx.PartDeclaration `json:"parts,omitempty"`
	Config          json.RawMessage         `json:"config,omitempty"`
	Lifetime        RecipientLifetime       `json:"lifetime"`
}

// ApplicationSpec is authored composition, not mutable upkeep or an action cost.
type ApplicationSpec struct {
	Name             string                  `json:"name"`
	CoordinatorParts []dndfx.PartDeclaration `json:"coordinator_parts"`
	Recipients       []RecipientSpec         `json:"recipients"`
}

// PrepareApplicationInput supplies identity and the complete preflight universe.
type PrepareApplicationInput struct {
	ID            string
	CoordinatorID string
	Origin        dndevents.ConditionOrigin
	Spec          ApplicationSpec
	Existing      []ParticipantConditions
}

// PreparedApplication owns copied construction data for the fixed intended list.
// Replacement is descriptive; only the source's paid door may act on it.
type PreparedApplication struct {
	id          string
	coordinator string
	origin      dndevents.ConditionOrigin
	spec        ApplicationSpec
	holders     map[string]bool
	members     []LinkedMemberBinding
	replacement *dndevents.ConditionAddress
	resumed     bool
}

// ID returns invocation identity, reused through resume/reload.
func (p *PreparedApplication) ID() string { return p.id }

// Replacement returns the exact current concentration owner, when one exists.
func (p *PreparedApplication) Replacement() (dndevents.ConditionAddress, bool) {
	if p.replacement == nil {
		return dndevents.ConditionAddress{}, false
	}
	return *p.replacement, true
}

// CreateRecipientInput selects one fixed candidate; its holder cannot be replaced.
type CreateRecipientInput struct {
	Prepared  *PreparedApplication
	SpecIndex int
}

// RecipientCandidate describes intended identity, not proof of actual delivery.
type RecipientCandidate struct {
	Key       string
	Address   dndevents.ConditionAddress
	LinkMode  LinkMode
	SpecIndex int
	Shared    bool
}

// CreateRecipientOutput supplies unbound behavior and its fixed intended identity.
type CreateRecipientOutput struct {
	Condition dndevents.ConditionBehavior
	Candidate RecipientCandidate
}

// CreateApplicationInput supplies a fresh prepared invocation before delivery.
type CreateApplicationInput struct{ Prepared *PreparedApplication }

// CreateApplicationOutput contains the coordinator to attach after payment and
// replacement, before recipient delivery. All intended shared entries are pending.
type CreateApplicationOutput struct{ Condition dndevents.ConditionBehavior }

// ResumeApplicationInput reconstructs candidate creation from the source's frozen
// declaration and existing owner. It cannot create a missing or ended application.
type ResumeApplicationInput struct {
	ID       string
	Spec     ApplicationSpec
	Existing []ParticipantConditions
}

func cloneDeclarations(in []dndfx.PartDeclaration) []dndfx.PartDeclaration {
	out := make([]dndfx.PartDeclaration, len(in))
	copy(out, in)
	for i := range out {
		out[i].Params = slices.Clone(in[i].Params)
	}
	return out
}

// PrepareApplication validates every concrete candidate and describes replacement
// without attaching, paying, rolling or displacing anything. Duplicate invocation
// IDs or addresses, absent holders and invalid payload configuration are errors.
func PrepareApplication(in *PrepareApplicationInput) (*PreparedApplication, error) {
	return prepareApplication(in, false)
}

func prepareApplication(in *PrepareApplicationInput, resuming bool) (*PreparedApplication, error) {
	if in == nil || strings.TrimSpace(in.ID) == "" || strings.TrimSpace(in.CoordinatorID) == "" || strings.TrimSpace(in.Spec.Name) == "" {
		return nil, fmt.Errorf("application preparation requires id, coordinator and name")
	}
	p := &PreparedApplication{id: in.ID, coordinator: in.CoordinatorID, origin: in.Origin, spec: in.Spec, holders: map[string]bool{}, resumed: resuming}
	p.spec.CoordinatorParts = cloneDeclarations(in.Spec.CoordinatorParts)
	p.spec.Recipients = slices.Clone(in.Spec.Recipients)
	owner, err := p.coordinatorCondition()
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
				if !resuming {
					return nil, fmt.Errorf("application id %q already exists", p.id)
				}
				continue
			}
			if linked.role == LinkedCoordinator && linked.member == p.coordinator && linked.concentrates && owner.concentrates {
				if p.replacement != nil || resuming {
					return nil, fmt.Errorf("maintainer has competing concentration claims")
				}
				address := linked.ConditionAddress()
				p.replacement = &address
			}
		}
	}
	if !p.holders[p.coordinator] {
		return nil, fmt.Errorf("coordinator is absent from the preflight universe")
	}
	addresses := map[dndevents.ConditionAddress]bool{}
	for i := range p.spec.Recipients {
		spec := &p.spec.Recipients[i]
		if !p.holders[spec.MemberID] {
			return nil, fmt.Errorf("recipient %d has missing intended holder %q", i, spec.MemberID)
		}
		entry, err := lookup(spec.Ref)
		if err != nil || entry.create == nil {
			return nil, fmt.Errorf("recipient %d does not name a creatable registered ref", i)
		}
		if spec.Lifetime != LifetimeShared && spec.Lifetime != LifetimeIndependent {
			return nil, fmt.Errorf("recipient %d needs an explicit lifetime policy", i)
		}
		authored := in.Spec.Recipients[i].Parts
		if authored == nil && entry.linkedParts != nil {
			authored = entry.linkedParts
		}
		spec.Parts = cloneDeclarations(authored)
		spec.Config = slices.Clone(in.Spec.Recipients[i].Config)
		if entry.linked {
			if spec.Lifetime != LifetimeShared || spec.AddressSourceID == "" || authored == nil || len(spec.Config) != 0 {
				return nil, fmt.Errorf("linked recipient %d requires shared parts and explicit address identity", i)
			}
			if entry.linkedParts != nil {
				if err := sameLinkedParts(spec.Parts, entry.linkedParts); err != nil {
					return nil, err
				}
			}
			keys := map[string]bool{}
			for _, part := range spec.Parts {
				if keys[part.Key] || part.OnTerminal == generic.EndApplication {
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
		candidate, err := CreateRecipient(&CreateRecipientInput{Prepared: p, SpecIndex: i})
		if err != nil {
			return nil, fmt.Errorf("recipient %d: %w", i, err)
		}
		receipt := candidate.Candidate
		if addresses[receipt.Address] || receipt.Address == owner.ConditionAddress() {
			return nil, fmt.Errorf("duplicate intended recipient address")
		}
		addresses[receipt.Address] = true
		if receipt.Shared {
			p.members = append(p.members, LinkedMemberBinding{Key: receipt.Key, Address: receipt.Address, Mode: receipt.LinkMode})
		}
	}
	return p, nil
}

// CreateRecipient constructs the selected immutable candidate without attaching
// or spending. Preparation already validated every candidate before payment.
func CreateRecipient(in *CreateRecipientInput) (*CreateRecipientOutput, error) {
	if in == nil || in.Prepared == nil || in.SpecIndex < 0 || in.SpecIndex >= len(in.Prepared.spec.Recipients) {
		return nil, fmt.Errorf("recipient creation requires a prepared declaration")
	}
	p := in.Prepared
	spec := p.spec.Recipients[in.SpecIndex]
	entry, err := lookup(spec.Ref)
	if err != nil {
		return nil, err
	}
	key := fmt.Sprintf("%d:%q", in.SpecIndex, spec.MemberID)
	create := &CreateFromRefInput{Ref: spec.Ref, MemberID: spec.MemberID, Config: slices.Clone(spec.Config)}
	mode := LinkAdopted
	if entry.linked {
		mode = LinkComposed
		name := spec.Name
		if name == "" {
			name = entry.display.Name
		}
		owner := dndevents.ConditionAddress{MemberID: p.coordinator, ConditionRef: refs.Conditions.LinkedEffects().String(), SourceID: p.id}
		config := LinkedConditionConfig{ApplicationID: p.id, Role: LinkedRecipient, AddressSourceID: spec.AddressSourceID, Bindings: &LinkedBindings{Name: name, Parts: cloneDeclarations(spec.Parts), Owner: &owner, MemberKey: key}}
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
	return &CreateRecipientOutput{Condition: created.Condition, Candidate: RecipientCandidate{Key: key, Address: ConditionAddressOf(spec.MemberID, created.Condition), LinkMode: mode, SpecIndex: in.SpecIndex, Shared: spec.Lifetime == LifetimeShared}}, nil
}

func (p *PreparedApplication) coordinatorCondition() (*LinkedCondition, error) {
	return buildLinked(LinkedConditionData{Ref: *refs.Conditions.LinkedEffects(), MemberID: p.coordinator, AddressSourceID: p.id, ApplicationID: p.id, Role: LinkedCoordinator, Origin: &p.origin,
		Bindings: &LinkedBindings{Name: p.spec.Name, Parts: cloneDeclarations(p.spec.CoordinatorParts), Members: slices.Clone(p.members)}}, false)
}

// CreateApplication constructs a fresh coordinator with fixed pending bindings.
// It refuses missing or resumed input; it does not attach or displace an owner.
func CreateApplication(in *CreateApplicationInput) (*CreateApplicationOutput, error) {
	if in == nil || in.Prepared == nil || in.Prepared.resumed {
		return nil, fmt.Errorf("coordinator creation requires a fresh prepared application")
	}
	owner, err := in.Prepared.coordinatorCondition()
	if err != nil {
		return nil, err
	}
	return &CreateApplicationOutput{Condition: owner}, nil
}

// ResumeApplication rebuilds only candidate creation for an existing live owner.
// It refuses missing/competing owners, changed intended bindings or maintenance;
// it does not refill membership, recreate the coordinator or displace anything.
func ResumeApplication(in *ResumeApplicationInput) (*PreparedApplication, error) {
	if in == nil || in.ID == "" {
		return nil, fmt.Errorf("resume requires application identity")
	}
	var owner *LinkedCondition
	for _, participant := range in.Existing {
		for _, condition := range participant.Conditions {
			linked, ok := condition.(*LinkedCondition)
			if ok && linked.role == LinkedCoordinator && linked.application == in.ID {
				if owner != nil || linked.member != participant.MemberID {
					return nil, fmt.Errorf("resume has duplicate or misplaced owner")
				}
				owner = linked
			}
		}
	}
	if owner == nil || owner.lifetime.Snapshot().Phase != generic.LifetimeLive {
		return nil, fmt.Errorf("application %q has no current live owner to resume", in.ID)
	}
	p, err := prepareApplication(&PrepareApplicationInput{ID: in.ID, CoordinatorID: owner.member, Origin: owner.origin, Spec: in.Spec, Existing: in.Existing}, true)
	if err != nil {
		return nil, err
	}
	if p.spec.Name != owner.bindings.Name || len(p.members) != len(owner.bindings.Members) {
		return nil, fmt.Errorf("resume changed application bindings")
	}
	if err := sameLinkedParts(p.spec.CoordinatorParts, owner.bindings.Parts); err != nil {
		return nil, fmt.Errorf("resume changed maintenance: %w", err)
	}
	for i, member := range p.members {
		if member != owner.bindings.Members[i] {
			return nil, fmt.Errorf("resume changed intended recipient %q", member.Key)
		}
	}
	return p, nil
}
