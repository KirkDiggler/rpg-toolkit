// Copyright (C) 2026 Kirk Diggler
// SPDX-License-Identifier: GPL-3.0-or-later

package session_test

import (
	"encoding/json"
	"fmt"
	"testing"

	"github.com/KirkDiggler/rpg-toolkit/core"
	generic "github.com/KirkDiggler/rpg-toolkit/mechanics/effects"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/conditions"
	dndfx "github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/effects"
	dndevents "github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/events"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/refs"
	"github.com/stretchr/testify/require"
)

func fixtureApplicationID(member string) string { return "fixture:" + member }
func fixtureMemberKey(address dndevents.ConditionAddress) string {
	return fmt.Sprintf("%q/%q/%q", address.MemberID, address.ConditionRef, address.SourceID)
}

type holdFixtureInput struct {
	Member   string
	Source   string
	Name     string
	Turns    int
	Skip     bool
	Children []dndevents.ConditionAddress
}

// Existing-state fixtures use the current creator/codec, with explicitly live
// membership. Native casting/continuation tests create and deliver normally.
func fixtureHold(t *testing.T, in holdFixtureInput) *conditions.LinkedCondition {
	t.Helper()
	source, err := core.ParseString(in.Source)
	require.NoError(t, err)
	var members []conditions.LinkedMemberBinding
	for _, address := range in.Children {
		mode := conditions.LinkAdopted
		if address.ConditionRef == refs.Conditions.Blessed().String() || address.ConditionRef == refs.Conditions.Baned().String() {
			mode = conditions.LinkComposed
		}
		members = append(members, conditions.LinkedMemberBinding{Key: fixtureMemberKey(address), Address: address, Mode: mode})
	}
	clock, err := json.Marshal(struct {
		Turns int  `json:"turn_ends"`
		Skip  bool `json:"skip_first"`
	}{in.Turns, in.Skip})
	require.NoError(t, err)
	config, err := json.Marshal(conditions.LinkedConditionConfig{ApplicationID: fixtureApplicationID(in.Member), Role: conditions.LinkedCoordinator, AddressSourceID: fixtureApplicationID(in.Member), Bindings: &conditions.LinkedBindings{Name: in.Name, Members: members, Parts: []dndfx.PartDeclaration{
		{Key: "hold", Kind: dndfx.KindConcentration, OnTerminal: generic.EndApplication},
		{Key: "clock", Kind: dndfx.KindDuration, Params: clock, OnTerminal: generic.EndApplication},
		{Key: "empty", Kind: dndfx.KindUntilEmpty, OnTerminal: generic.EndApplication},
		{Key: "rest", Kind: dndfx.KindEndOn, Params: json.RawMessage(`{"beat":"rest","reason":"rest"}`), OnTerminal: generic.EndApplication},
		{Key: "combat", Kind: dndfx.KindEndOn, Params: json.RawMessage(`{"beat":"combat_end","reason":"combat_end"}`), OnTerminal: generic.EndApplication},
	}}})
	require.NoError(t, err)
	created, err := conditions.CreateFromRef(&conditions.CreateFromRefInput{Ref: refs.Conditions.LinkedEffects().String(), MemberID: in.Member, Origin: &dndevents.ConditionOrigin{Ref: *source, EntityID: in.Member}, Config: config})
	require.NoError(t, err)
	raw, err := created.Condition.ToJSON()
	require.NoError(t, err)
	var data conditions.LinkedConditionData
	require.NoError(t, json.Unmarshal(raw, &data))
	data.Runtime.Lifetime.LiveMembers = append([]string{}, data.Runtime.Lifetime.PendingMembers...)
	data.Runtime.Lifetime.PendingMembers = []string{}
	raw, err = json.Marshal(data)
	require.NoError(t, err)
	loaded, err := conditions.LoadJSON(raw)
	require.NoError(t, err)
	return loaded.(*conditions.LinkedCondition)
}

type namedFixtureInput struct {
	Ref            *core.Ref
	Member, Source string
	Origin         *core.Ref
}

func fixtureNamed(t *testing.T, in namedFixtureInput) *conditions.LinkedCondition {
	t.Helper()
	owner := dndevents.ConditionAddress{MemberID: in.Source, ConditionRef: refs.Conditions.LinkedEffects().String(), SourceID: fixtureApplicationID(in.Source)}
	address := dndevents.ConditionAddress{MemberID: in.Member, ConditionRef: in.Ref.String(), SourceID: in.Source}
	display, ok := conditions.DisplayFor(*in.Ref)
	require.True(t, ok)
	config, err := json.Marshal(conditions.LinkedConditionConfig{ApplicationID: fixtureApplicationID(in.Source), Role: conditions.LinkedRecipient, AddressSourceID: in.Source, Bindings: &conditions.LinkedBindings{Name: display.Name, Owner: &owner, MemberKey: fixtureMemberKey(address)}})
	require.NoError(t, err)
	created, err := conditions.CreateFromRef(&conditions.CreateFromRefInput{Ref: in.Ref.String(), MemberID: in.Member, Origin: &dndevents.ConditionOrigin{Ref: *in.Origin, EntityID: in.Source}, Config: config})
	require.NoError(t, err)
	return created.Condition.(*conditions.LinkedCondition)
}
