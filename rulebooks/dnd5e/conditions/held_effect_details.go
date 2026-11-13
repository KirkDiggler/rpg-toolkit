// Copyright (C) 2026 Kirk Diggler
// SPDX-License-Identifier: GPL-3.0-or-later

package conditions

import (
	"fmt"
	"strings"

	"github.com/KirkDiggler/rpg-toolkit/core"
	generic "github.com/KirkDiggler/rpg-toolkit/mechanics/effects"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/contributions"
	dndfx "github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/effects"
	dndevents "github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/events"
)

// ProjectHeldEffectsInput supplies a caller-owned snapshot of one holder's
// conditions. The caller controls whether this is observed or authoritative.
type ProjectHeldEffectsInput struct {
	HolderID   string
	Conditions []dndevents.ConditionBehavior
}

// ProjectHeldEffectsOutput carries value-only details; no live condition escapes.
type ProjectHeldEffectsOutput struct {
	Details []contributions.HeldEffectDetail
}

// ProjectHeldEffects captures available configured target outputs. It works on
// loaded, unattached state as well as live state and retains end observations.
// Unknown refs, mismatched holders, duplicate addresses and invalid parts refuse.
// Fixed named conditions retain their by-reference assessment path.
func ProjectHeldEffects(in *ProjectHeldEffectsInput) (*ProjectHeldEffectsOutput, error) {
	if in == nil || strings.TrimSpace(in.HolderID) == "" {
		return nil, fmt.Errorf("held-effect projection requires a holder")
	}
	out := &ProjectHeldEffectsOutput{Details: []contributions.HeldEffectDetail{}}
	seen := map[dndevents.ConditionAddress]bool{}
	for _, condition := range in.Conditions {
		if condition == nil || condition.Ref() == nil {
			return nil, fmt.Errorf("held-effect projection received an unnamed condition")
		}
		if _, err := lookup(condition.Ref().String()); err != nil {
			return nil, err
		}
		address := ConditionAddressOf(in.HolderID, condition)
		if address.MemberID != in.HolderID || seen[address] {
			return nil, fmt.Errorf("held-effect projection has a foreign or duplicate address")
		}
		seen[address] = true
		linked, ok := condition.(*LinkedCondition)
		if !ok {
			continue
		}
		detail := contributions.HeldEffectDetail{MemberID: linked.member, Held: contributions.HeldCondition{Ref: address.ConditionRef, SourceID: address.SourceID}, ApplicationID: linked.application, DisplayName: linked.bindings.Name, OriginRef: linked.origin.Ref, OriginID: linked.origin.EntityID, Outputs: []contributions.ConfiguredOutput{}}
		for _, output := range linked.assembly.CurrentOutputs() {
			projected, err := linked.projectOutput(output)
			if err != nil {
				return nil, err
			}
			if projected.AgainstHolder {
				detail.Outputs = append(detail.Outputs, contributions.ConfiguredOutput{ID: output.Output.ID(), Definition: projected.Definition})
			}
		}
		out.Details = append(out.Details, detail)
	}
	return out, nil
}

func (c *LinkedCondition) projectOutput(output generic.PartOutput) (*dndfx.ProjectPartOutputOutput, error) {
	for _, declaration := range c.bindings.Parts {
		if declaration.Key == output.PartKey {
			return dndfx.ProjectPartOutput(&dndfx.ProjectPartOutputInput{Declaration: declaration, Output: output.Output})
		}
	}
	return nil, fmt.Errorf("observed part lacks its declaration")
}

func assessConfiguredHolding(frame contributions.Frame, holder string, held contributions.HeldCondition) ([]contributions.Effect, error) {
	for _, detail := range frame.HeldEffects {
		if detail.MemberID != holder || detail.Held != held {
			continue
		}
		rows := []contributions.Effect{}
		for _, output := range detail.Outputs {
			answer, err := dndfx.AssessConfiguredOutput(&dndfx.AssessConfiguredOutputInput{Detail: detail, OutputID: output.ID, Frame: frame})
			if err != nil {
				return nil, err
			}
			answer.Effect.ID = "target:" + output.ID
			rows = append(rows, answer.Effect)
		}
		return rows, nil
	}
	ref, err := core.ParseString(held.Ref)
	if err != nil {
		return nil, err
	}
	return []contributions.Effect{{ID: heldEffectID(held), Source: contributions.Source{Ref: ref, Name: "Configured effects"}, Description: "The target holds configured effects whose details are not known.", State: contributions.StateDepends, Reason: "Depends on the target's configured effects", Participation: contributions.ContributesNow}}, nil
}
