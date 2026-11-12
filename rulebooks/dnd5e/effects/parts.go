// Copyright (C) 2026 Kirk Diggler
// SPDX-License-Identifier: GPL-3.0-or-later

package effects

import (
	"encoding/json"
	"fmt"
	"strings"

	"github.com/KirkDiggler/rpg-toolkit/core"
	generic "github.com/KirkDiggler/rpg-toolkit/mechanics/effects"
	"github.com/KirkDiggler/rpg-toolkit/mechanics/effects/machine"
	dndevents "github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/events"
)

const (
	// KindConcentration requests the maintainer's save and ends on zero HP.
	KindConcentration generic.KindName = "concentration"
	// KindDuration counts authored holder turn ends independently of concentration.
	KindDuration generic.KindName = "duration"
)

// PartDeclaration selects a reusable meaning without a named-spell branch.
// Its immutable parameters are validated by that meaning's one kind entry.
type PartDeclaration struct {
	Key        string           `json:"key"`
	Kind       generic.KindName `json:"kind"`
	Params     json.RawMessage  `json:"params,omitempty"`
	When       *machine.Guard   `json:"when,omitempty"`
	Amount     *generic.Amount  `json:"amount,omitempty"`
	OnTerminal generic.EndScope `json:"on_terminal"`
}

// BindPartInput supplies explicit attachment identity and authored attribution.
// Owner is the exact application coordinator, not an inferred source qualifier.
type BindPartInput struct {
	Declaration   PartDeclaration
	Ref           core.Ref
	HolderID      string
	ApplicationID string
	DisplayName   string
	Origin        dndevents.ConditionOrigin
	Owner         dndevents.ConditionAddress
	// Address is the exact local attachment, distinct from the coordinator Owner.
	Address dndevents.ConditionAddress
}

// PartCapabilities describes meaning the application compiler may query without
// inspecting kind names or interpreting machine state in resolution.
type PartCapabilities struct {
	Concentration bool
	Duration      bool
}

// BindPartOutput supplies a local generic runtime declaration and its capabilities.
type BindPartOutput struct {
	Part         generic.PartInput
	Capabilities PartCapabilities
}

// DurationInfo is a view of the duration part's sole counter/grace authority.
type DurationInfo struct {
	Remaining int
	SkipNext  bool
}

// PartInfo describes currently active maintenance without mutation or attachment.
// Source names/refs and ownership are supplied by the enclosing application.
type PartInfo struct {
	Concentration bool
	Duration      *DurationInfo
}

// InspectPartInput names an authored declaration and its exact saved runtime.
type InspectPartInput struct {
	Declaration PartDeclaration
	Snapshot    generic.Snapshot
}

type boundPart struct {
	recipe machine.Recipe
}

// BindPart builds a reusable part without payment, attachment or dice. It refuses
// absent binding facts, unsupported kinds, invalid params and scope conflicts.
// The condition registry remains the attachment catalog; this is its vocabulary.
func BindPart(in *BindPartInput) (*BindPartOutput, error) {
	if in == nil {
		return nil, fmt.Errorf("part binding is required")
	}
	if strings.TrimSpace(in.Declaration.Key) == "" || strings.TrimSpace(in.HolderID) == "" || strings.TrimSpace(in.ApplicationID) == "" {
		return nil, fmt.Errorf("part key, holder and application id are required")
	}
	if err := in.Ref.IsValid(); err != nil {
		return nil, fmt.Errorf("part attachment ref: %w", err)
	}
	definition, ok := kindTable[in.Declaration.Kind]
	if !ok || definition.part == nil {
		return nil, fmt.Errorf("unsupported effect part %q", in.Declaration.Kind)
	}
	description, err := describePart(in.Declaration)
	if err != nil {
		return nil, err
	}
	part, err := definition.part(in)
	if err != nil {
		return nil, err
	}
	input := BindInput{Origin: in.Origin, DisplayName: in.DisplayName}
	if in.Address != (dndevents.ConditionAddress{}) {
		if in.Address.MemberID != in.HolderID || in.Address.ConditionRef != in.Ref.String() {
			return nil, fmt.Errorf("local attachment disagrees with part holder/ref")
		}
		input.Attachment = &AttachmentIdentity{Address: in.Address, ApplicationID: in.ApplicationID}
	}
	if definition.checkBinding != nil && definition.payload != nil {
		output, err := definition.payload(in.Declaration)
		if err != nil {
			return nil, err
		}
		if err := definition.checkBinding(output, input); err != nil {
			return nil, err
		}
	}
	vocabulary, err := Bind(&input)
	if err != nil {
		return nil, err
	}
	origin := in.Origin
	return &BindPartOutput{
		Part: generic.PartInput{Key: in.Declaration.Key, OnTerminal: in.Declaration.OnTerminal,
			Compose: generic.ComposeInput{
				Recipe: part.recipe, HolderID: in.HolderID, SourceRef: &origin.Ref,
				MachineVocabulary: vocabulary.Machine, OutputVocabulary: vocabulary.Outputs,
			}},
		Capabilities: description.capabilities,
	}, nil
}

// InspectPart asks the owning meaning to interpret its state, with no rolling or
// mutation. It refuses unknown parts, malformed parameters and incomplete state.
func InspectPart(in *InspectPartInput) (*PartInfo, error) {
	if in == nil {
		return nil, fmt.Errorf("part inspection is required")
	}
	if err := ValidatePart(&ValidatePartInput{Declaration: in.Declaration}); err != nil {
		return nil, err
	}
	definition, ok := kindTable[in.Declaration.Kind]
	if !ok || definition.inspectPart == nil {
		return nil, fmt.Errorf("unsupported part inspection %q", in.Declaration.Kind)
	}
	return definition.inspectPart(in)
}
