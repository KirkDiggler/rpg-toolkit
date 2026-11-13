// Copyright (C) 2026 Kirk Diggler
// SPDX-License-Identifier: GPL-3.0-or-later

package effects

import (
	"bytes"
	"encoding/json/jsontext"
	"encoding/json/v2"
	"fmt"

	"github.com/KirkDiggler/rpg-toolkit/core"
	generic "github.com/KirkDiggler/rpg-toolkit/mechanics/effects"
	"github.com/KirkDiggler/rpg-toolkit/mechanics/effects/machine"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/contributions"
	dndevents "github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/events"
)

// ProjectPartOutputInput pairs a current observable handle with its owned declaration.
type ProjectPartOutputInput struct {
	Declaration PartDeclaration
	Output      *generic.Bound
}

// ProjectPartOutputOutput contains detached authored data and its target bearing.
type ProjectPartOutputOutput struct {
	Definition    generic.Output
	AgainstHolder bool
}

// ProjectPartOutput validates and copies the actual observed output, using the
// same payload builder as binding. It never interprets terminal snapshots or bags.
func ProjectPartOutput(in *ProjectPartOutputInput) (*ProjectPartOutputOutput, error) {
	if in == nil || in.Output == nil {
		return nil, fmt.Errorf("part output projection requires declaration and output")
	}
	if err := ValidatePart(&ValidatePartInput{Declaration: in.Declaration}); err != nil {
		return nil, err
	}
	actual := in.Output.Definition()
	if actual.Kind != in.Declaration.Kind {
		return nil, fmt.Errorf("observed output kind disagrees with declaration")
	}
	def := kindTable[in.Declaration.Kind]
	result := &ProjectPartOutputOutput{Definition: actual}
	if def.payload != nil {
		expected, err := def.payload(in.Declaration)
		if err != nil {
			return nil, err
		}
		same, err := sameOutput(actual, expected)
		if err != nil {
			return nil, err
		}
		if !same {
			return nil, fmt.Errorf("observed output disagrees with declaration")
		}
	}
	if actual.Kind == KindAdvantage {
		m, err := advantageMeaning(actual.Params)
		if err != nil {
			return nil, err
		}
		result.AgainstHolder = m.domain == domainAttack && m.side == sideAgainstHolder
	}
	return result, nil
}

func canonicalData(in any) ([]byte, error) {
	raw, err := json.Marshal(in)
	if err != nil {
		return nil, err
	}
	var value any
	if err := json.Unmarshal(raw, &value); err != nil {
		return nil, err
	}
	return json.Marshal(value, json.Deterministic(true))
}
func sameOutput(a, b generic.Output) (bool, error) {
	left, err := canonicalData(a)
	if err != nil {
		return false, err
	}
	right, err := canonicalData(b)
	if err != nil {
		return false, err
	}
	return bytes.Equal(left, right), nil
}

// AssessConfiguredOutputInput identifies one captured output in the supplied
// frame. Detail is a selector/value check, never permission to consult live state.
type AssessConfiguredOutputInput struct {
	Detail   contributions.HeldEffectDetail
	OutputID string
	Frame    contributions.Frame
}

// AssessConfiguredOutput assesses one observed target output through the same
// vocabulary as execution. It constructs only an unattached observation runtime;
// no bus, rolling, state lookup or mutation occurs. Missing/contradictory detail,
// malformed output kinds/guards and invalid identity refuse the request.
func AssessConfiguredOutput(in *AssessConfiguredOutputInput) (*AssessOutput, error) {
	if in == nil {
		return nil, fmt.Errorf("configured output assessment is required")
	}
	if err := in.Frame.Validate(); err != nil {
		return nil, err
	}
	var captured *contributions.HeldEffectDetail
	for i := range in.Frame.HeldEffects {
		d := &in.Frame.HeldEffects[i]
		if d.MemberID == in.Detail.MemberID && d.Held == in.Detail.Held {
			captured = d
			break
		}
	}
	if captured == nil {
		return nil, fmt.Errorf("configured detail is absent from the frame")
	}
	left, err := canonicalData(captured)
	if err != nil {
		return nil, err
	}
	right, err := canonicalData(in.Detail)
	if err != nil {
		return nil, err
	}
	if !bytes.Equal(left, right) {
		return nil, fmt.Errorf("configured detail disagrees with the frame")
	}
	var output *generic.Output
	for _, value := range captured.Outputs {
		if value.ID == in.OutputID {
			definition := value.Definition.Clone()
			output = &definition
			break
		}
	}
	if output == nil {
		return nil, fmt.Errorf("unknown captured output %q", in.OutputID)
	}
	if output.Kind != KindAdvantage {
		return nil, fmt.Errorf("unsupported configured target output %q", output.Kind)
	}
	m, err := advantageMeaning(output.Params)
	if err != nil {
		return nil, err
	}
	if m.domain != domainAttack || m.side != sideAgainstHolder {
		return nil, fmt.Errorf("configured output does not bear against its holder")
	}
	ref, err := core.ParseString(captured.Held.Ref)
	if err != nil {
		return nil, err
	}
	identity := &AttachmentIdentity{Address: dndevents.ConditionAddress{MemberID: captured.MemberID, ConditionRef: captured.Held.Ref, SourceID: captured.Held.SourceID}, ApplicationID: captured.ApplicationID}
	origin := dndevents.ConditionOrigin{Ref: captured.OriginRef, EntityID: captured.OriginID}
	vocab, err := Bind(&BindInput{Origin: origin, DisplayName: captured.DisplayName, Attachment: identity})
	if err != nil {
		return nil, err
	}
	raw, err := json.Marshal(output)
	if err != nil {
		return nil, err
	}
	c, err := generic.Compose(generic.ComposeInput{Recipe: machine.Recipe{Ref: *ref, Initial: partStateActive, Bag: machine.Bag{}, States: map[string]machine.State{partStateActive: {Outputs: []jsontext.Value{raw}}}}, HolderID: captured.MemberID, MachineVocabulary: vocab.Machine, OutputVocabulary: vocab.Outputs})
	if err != nil {
		return nil, err
	}
	result, err := Assess(&AssessInput{Output: c.CurrentOutputs()[0], Query: Query{Action: &in.Frame}, Origin: origin, DisplayName: captured.DisplayName, Attachment: identity})
	if err != nil {
		return nil, err
	}
	result.Effect.ID = in.OutputID
	return result, nil
}
