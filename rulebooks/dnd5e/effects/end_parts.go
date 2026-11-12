// Copyright (C) 2026 Kirk Diggler
// SPDX-License-Identifier: GPL-3.0-or-later

package effects

import (
	"encoding/json/jsontext"
	"fmt"
	"slices"
	"strings"

	"github.com/KirkDiggler/rpg-toolkit/core"
	generic "github.com/KirkDiggler/rpg-toolkit/mechanics/effects"
	"github.com/KirkDiggler/rpg-toolkit/mechanics/effects/machine"
	dndevents "github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/events"
)

const (
	// KindUntilEmpty ends after the last linked member actually leaves.
	KindUntilEmpty generic.KindName = "until_empty"
	// KindEndOn supplies explicit holder-event endings independently of concentration.
	KindEndOn generic.KindName = "end_on"

	beatMembersChanged machine.Beat     = "members_changed"
	factApplication    machine.FactName = "application.id"
	factOwnerRef       machine.FactName = "application.owner_ref"
	factOwnerSource    machine.FactName = "application.owner_source"
	factMembersBefore  machine.FactName = "application.members_before"
	factMembersAfter   machine.FactName = "application.members_after"
)

type endOnParams struct {
	Beat   machine.Beat `json:"beat"`
	Reason string       `json:"reason"`
}

func endPartRecipe(ref core.Ref, transition machine.Transition) machine.Recipe {
	return machine.Recipe{Ref: ref, Initial: partStateActive, Bag: machine.Bag{},
		States: map[string]machine.State{
			partStateActive: {Transitions: []machine.Transition{transition}},
			partStateEnded:  {Terminal: true},
		}}
}

func parseEndOn(declaration PartDeclaration) (endOnParams, error) {
	if declaration.OnTerminal != generic.EndApplication && declaration.OnTerminal != generic.EndAttachment {
		return endOnParams{}, fmt.Errorf("end policy must end its attachment or application")
	}
	params, err := decodeParams[endOnParams](jsontext.Value(declaration.Params))
	if err != nil {
		return endOnParams{}, err
	}
	if !slices.Contains([]machine.Beat{BeatRest, BeatCombatEnd, BeatTurnStart, BeatDroppedToZero}, params.Beat) || strings.TrimSpace(params.Reason) == "" {
		return endOnParams{}, fmt.Errorf("end policy requires a supported holder beat and an explicit reason")
	}
	return params, nil
}

func bindEndOnPart(in *BindPartInput) (boundPart, error) {
	params, err := parseEndOn(in.Declaration)
	if err != nil {
		return boundPart{}, err
	}
	return boundPart{recipe: endPartRecipe(in.Ref, machine.Transition{
		On: params.Beat, About: Holder, To: partStateEnded, Reason: params.Reason,
	})}, nil
}

func bindUntilEmptyPart(in *BindPartInput) (boundPart, error) {
	if err := checkUnparameterizedMaintenance(in.Declaration); err != nil {
		return boundPart{}, err
	}
	if in.Owner.MemberID != in.HolderID || in.Owner.SourceID != in.ApplicationID {
		return boundPart{}, fmt.Errorf("until_empty must bind its exact application coordinator")
	}
	if _, err := core.ParseString(in.Owner.ConditionRef); err != nil {
		return boundPart{}, fmt.Errorf("until_empty owner ref: %w", err)
	}
	return boundPart{recipe: endPartRecipe(in.Ref, machine.Transition{
		On: beatMembersChanged, About: Holder, To: partStateEnded, Reason: "members_empty",
		When: &machine.Guard{All: []machine.Guard{
			{Fact: &machine.FactLeaf{Name: factApplication, Op: machine.OpEq, Value: in.ApplicationID}},
			{Fact: &machine.FactLeaf{Name: factOwnerRef, Op: machine.OpEq, Value: in.Owner.ConditionRef}},
			{Fact: &machine.FactLeaf{Name: factOwnerSource, Op: machine.OpEq, Value: in.Owner.SourceID}},
			{Fact: &machine.FactLeaf{Name: factMembersBefore, Op: machine.OpGte, Value: 1}},
			{Fact: &machine.FactLeaf{Name: factMembersAfter, Op: machine.OpEq, Value: 0}},
		}},
	})}, nil
}

func inspectEndOnPart(in *InspectPartInput) (*PartInfo, error) {
	if _, err := parseEndOn(in.Declaration); err != nil {
		return nil, err
	}
	return inspectEmptyPartState(in.Snapshot)
}

func inspectUntilEmptyPart(in *InspectPartInput) (*PartInfo, error) {
	if err := checkUnparameterizedMaintenance(in.Declaration); err != nil {
		return nil, err
	}
	return inspectEmptyPartState(in.Snapshot)
}

func inspectEmptyPartState(snapshot generic.Snapshot) (*PartInfo, error) {
	if snapshot.Bag == nil || len(snapshot.Bag) != 0 || (snapshot.State != partStateActive && snapshot.State != partStateEnded) {
		return nil, fmt.Errorf("end policy requires known state and an explicit empty bag")
	}
	return &PartInfo{}, nil
}

func membershipFact(kind machine.ScalarKind, read func(dndevents.ApplicationMembersChangedEvent) any) machine.FactSpec {
	return machine.FactSpec{Kind: kind, Read: func(frame any) (any, bool) {
		fact, ok := frame.(dndevents.ApplicationMembersChangedEvent)
		if !ok {
			return nil, false
		}
		return read(fact), true
	}}
}
