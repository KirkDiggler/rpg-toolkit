// Copyright (C) 2026 Kirk Diggler
// SPDX-License-Identifier: GPL-3.0-or-later

package resolution

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"strings"

	"github.com/KirkDiggler/rpg-toolkit/core"
	"github.com/KirkDiggler/rpg-toolkit/events"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/conditions"
	dnd5eEvents "github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/events"
)

var (
	// ErrBadStatusLibrary indicates a status library that does not compile:
	// the content an author wrote is wrong, so nothing is given.
	ErrBadStatusLibrary = errors.New("resolution: status library does not compile")

	// ErrUnknownStatus indicates a status id the library does not define.
	ErrUnknownStatus = errors.New("resolution: status is not in its library")
)

// GiveStatusInput is one member receiving one status an author wrote.
type GiveStatusInput struct {
	// Interaction is the cast, the world and the seams, built exactly as a verb
	// builds them for any other resolution. GiveStatus supplies the machine, so
	// Interaction.Machine must be nil. Every sheet of the run belongs in its
	// Participants: a status is an application, and an application is admitted
	// against everything the run holds.
	Interaction *Input

	// Member is who receives the status. They hold all of it: the status is
	// bound with the member as both its user and its recipient.
	Member string

	// Library is the authored status library, in the conditions definition
	// library's JSON shape ({parts, benefit_sets, definitions}). It is compiled
	// strictly on every call; a library that does not compile is
	// [ErrBadStatusLibrary].
	Library json.RawMessage

	// StatusID is the definition in Library to give. An id the library does not
	// define is [ErrUnknownStatus].
	StatusID string

	// SourceID is who gave it — the member whose line, item or hand the status
	// came from. It names the origin and need not have a sheet in the run.
	SourceID string

	// Source is what gave it, as content names it (a site's status, say). It is
	// the origin's ref and must be a valid ref.
	Source core.Ref

	// ApplicationID is the status's identity, chosen by the caller. THE SAME ID
	// MEANS THE SAME STATUS: giving a member an application id they already hold
	// gives nothing and answers [GiveStatusOutput.AlreadyHeld]. Pick one id per
	// status per member; an id another member holds is refused by admission.
	ApplicationID string
}

// GiveStatusOutcome is what one give produced.
type GiveStatusOutcome struct {
	// AlreadyHeld is true when the member already held the application, and
	// nothing was given.
	AlreadyHeld bool

	// Effects is each condition the status put on the member, coordinator
	// first. Empty when AlreadyHeld.
	Effects []ImposedEffect
}

func (GiveStatusOutcome) isOutcome() {}

// GiveStatusOutput is the give's answer and the interaction that produced it.
type GiveStatusOutput struct {
	// AlreadyHeld is true when the member already held this application id: the
	// give was a no-op, not an error, and no sheet changed.
	AlreadyHeld bool

	// Effects is what the status put on the member, already applied.
	Effects []ImposedEffect

	// Resolved is the interaction's own output. The caller saves its dirty
	// sheets exactly as it saves any other verb's, which is how the status
	// becomes durable on the member's record.
	Resolved *Output
}

// GiveStatus gives one member one status an author wrote, on resolution's own
// bus (ADR-0038).
//
// The status is a data-authored condition definition: it is compiled, bound
// with the member as user and recipient and the source as its origin, admitted
// against every sheet of the run, and delivered — coordinator first, then each
// benefit, each finalized as it lands. How long it lasts is the definition's
// own maintenance; a status whose lifetime is end_on run_end comes off when
// [Depart] departs its holder.
//
// Refusals, each before anything is published: [ErrNilInput] for a missing
// input or interaction, [ErrBadAction] for a missing member, application id,
// source or a machine already supplied, [ErrBadStatusLibrary],
// [ErrUnknownStatus], and [ErrBadParticipant] for a member not in the run.
func GiveStatus(ctx context.Context, in *GiveStatusInput) (*GiveStatusOutput, error) {
	machine, err := newGiveStatus(in)
	if err != nil {
		return nil, err
	}

	interaction := *in.Interaction
	interaction.Machine = machine
	out, err := Resolve(ctx, &interaction)
	if err != nil {
		return nil, err
	}

	outcome, ok := out.Outcome.(GiveStatusOutcome)
	if !ok {
		return nil, fmt.Errorf("%w: giving %q produced %T", ErrBadStep, in.StatusID, out.Outcome)
	}

	return &GiveStatusOutput{AlreadyHeld: outcome.AlreadyHeld, Effects: outcome.Effects, Resolved: out}, nil
}

// giveStatusMachine is one give, as steps over data.
type giveStatusMachine struct {
	in          *GiveStatusInput
	definitions *conditions.CompiledDefinitions
}

// newGiveStatus validates the give and compiles the library before any
// interaction is opened.
func newGiveStatus(in *GiveStatusInput) (Machine, error) {
	if in == nil {
		return nil, ErrNilInput
	}
	if in.Interaction == nil {
		return nil, fmt.Errorf("%w: a status is given inside an interaction", ErrNilInput)
	}
	if in.Interaction.Machine != nil {
		return nil, fmt.Errorf("%w: giving a status is its own machine", ErrBadAction)
	}
	if strings.TrimSpace(in.Member) == "" {
		return nil, fmt.Errorf("%w: a status is given to nobody", ErrBadAction)
	}
	if strings.TrimSpace(in.ApplicationID) == "" {
		return nil, fmt.Errorf("%w: status %q has no application id", ErrBadAction, in.StatusID)
	}
	if strings.TrimSpace(in.SourceID) == "" {
		return nil, fmt.Errorf("%w: status %q was given by nobody", ErrBadAction, in.StatusID)
	}
	source := in.Source
	if err := source.IsValid(); err != nil {
		return nil, fmt.Errorf("%w: status %q source: %w", ErrBadAction, in.StatusID, err)
	}
	compiled, err := conditions.CompileDefinitions(&conditions.CompileDefinitionsInput{Data: in.Library})
	if err != nil {
		return nil, fmt.Errorf("%w: %w", ErrBadStatusLibrary, err)
	}
	if err := conditions.ValidateDefinitionReference(&conditions.ValidateDefinitionReferenceInput{Definitions: compiled.Definitions, DefinitionID: in.StatusID}); err != nil {
		return nil, fmt.Errorf("%w: %w", ErrUnknownStatus, err)
	}

	return &giveStatusMachine{in: in, definitions: compiled.Definitions}, nil
}

// Start checks the member, answers a repeated give, and otherwise prepares
// the application against the whole cast and delivers it in bus-owned steps.
func (m *giveStatusMachine) Start(_ context.Context, cast *Participants) (Step, error) {
	member := m.in.Member
	if _, ok := cast.Character(member); !ok {
		if _, ok := cast.Monster(member); !ok {
			return nil, fmt.Errorf("%w: %q is not in the run", ErrBadParticipant, member)
		}
	}
	// Checked BEFORE preparing: admission refuses an application id that is
	// already held, and a repeated give is an answer, not a failure.
	if holdsApplication(cast, member, m.in.ApplicationID) {
		return Done{Outcome: GiveStatusOutcome{AlreadyHeld: true}}, nil
	}
	if cast.applications == nil {
		return nil, fmt.Errorf("%w: giving a status requires an interaction binding", ErrBadStep)
	}

	origin := dnd5eEvents.ConditionOrigin{Ref: m.in.Source, EntityID: m.in.SourceID}
	bound, err := conditions.BindDefinition(&conditions.BindDefinitionInput{
		Definitions: m.definitions, DefinitionID: m.in.StatusID,
		UserID: member, RecipientID: member, ApplicationID: m.in.ApplicationID, Origin: origin,
	})
	if err != nil {
		return nil, fmt.Errorf("%w: bind status %q: %w", ErrBadAction, m.in.StatusID, err)
	}
	prepared, err := conditions.PrepareApplication(&conditions.PrepareApplicationInput{
		ID: m.in.ApplicationID, CoordinatorID: bound.CoordinatorID, Origin: origin,
		Spec: bound.Spec, Existing: applicationParticipants(cast),
	})
	if err != nil {
		return nil, fmt.Errorf("%w: admit status %q: %w", ErrBadAction, m.in.StatusID, err)
	}
	owner, err := conditions.CreateApplication(&conditions.CreateApplicationInput{Prepared: prepared})
	if err != nil {
		return nil, err
	}
	name := bound.Spec.Name

	deliver := Gather{name: "give status " + name, run: func(ctx context.Context, bus events.EventBus) (Step, error) {
		coordinator, err := cast.entity(bound.CoordinatorID)
		if err != nil {
			return nil, err
		}
		if err := dnd5eEvents.ConditionAppliedTopic.On(bus).Publish(ctx, dnd5eEvents.ConditionAppliedEvent{
			Target: coordinator, Source: dnd5eEvents.ConditionSourceSite, Condition: owner.Condition, ApplicationID: m.in.ApplicationID,
		}); err != nil {
			return nil, fmt.Errorf("give status %q to %q: %w", m.in.StatusID, member, err)
		}
		effects := []ImposedEffect{statusEffect(name, bound.CoordinatorID, owner.Condition)}
		for index := range bound.Spec.Recipients {
			made, err := conditions.CreateRecipient(&conditions.CreateRecipientInput{Prepared: prepared, SpecIndex: index})
			if err != nil {
				return nil, err
			}
			recipientID := made.Candidate.Address.MemberID
			target, err := cast.entity(recipientID)
			if err != nil {
				return nil, err
			}
			delivery := preparedCondition{behavior: made.Condition, name: name}
			if made.Candidate.Shared {
				delivery.applicationID = m.in.ApplicationID
				delivery.memberKey = made.Candidate.Key
			}
			replaced, err := publishCondition(ctx, bus, cast, delivery, target, dnd5eEvents.ConditionSourceSite)
			if err != nil {
				return nil, err
			}
			effects = append(effects, replaced...)
			effects = append(effects, statusEffect(name, recipientID, made.Condition))
		}
		return Done{Outcome: GiveStatusOutcome{Effects: effects}}, nil
	}}

	if old, exists := prepared.Replacement(); exists {
		return publishRemoval(&ConditionRemoval{Owner: old, Reason: conditions.ConcentrationEndedRecast}, func(ImposedEffect) (Step, error) { return deliver, nil }), nil
	}
	return deliver, nil
}

// holdsApplication reports whether a member carries any part of an
// application — its coordinator or one of its benefits.
func holdsApplication(cast *Participants, memberID, applicationID string) bool {
	for _, condition := range heldConditions(cast, memberID) {
		linked, ok := condition.(interface {
			LinkedApplicationInfo() conditions.LinkedApplicationInfo
		})
		if ok && linked.LinkedApplicationInfo().ID == applicationID {
			return true
		}
	}
	return false
}

// statusEffect is one condition a status put on a member, named by the status.
func statusEffect(name, recipientID string, behavior dnd5eEvents.ConditionBehavior) ImposedEffect {
	address := conditions.ConditionAddressOf(recipientID, behavior)
	ref, _ := core.ParseString(address.ConditionRef)
	return ImposedEffect{
		Kind:        ImposedCondition,
		Ref:         ref,
		Description: name,
		RecipientID: recipientID,
		Address:     address,
		Detail:      conditions.InstanceDetailOf(behavior),
	}
}
