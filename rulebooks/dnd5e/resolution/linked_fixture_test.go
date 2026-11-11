// Copyright (C) 2026 Kirk Diggler
// SPDX-License-Identifier: GPL-3.0-or-later

package resolution

import (
	"encoding/json"
	"fmt"
	"slices"
	"sync/atomic"
	"testing"

	"github.com/KirkDiggler/rpg-toolkit/core"
	generic "github.com/KirkDiggler/rpg-toolkit/mechanics/effects"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/conditions"
	effects "github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/effects"
	dndevents "github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/events"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/monster/monsters"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/refs"
	"github.com/stretchr/testify/require"
)

var testApplicationSequence atomic.Uint64

// Test callers supply distinct invocation identities just as the host supplies
// act/ordinal keys. This is fixture construction, never a production fallback.
func testApplicationID(t *testing.T) string {
	t.Helper()
	return fmt.Sprintf("test:%s:%d", t.Name(), testApplicationSequence.Add(1))
}

func fixtureApplicationID(member string) string { return "fixture:" + member }
func fixtureMemberKey(address dndevents.ConditionAddress) string {
	return fmt.Sprintf("%q/%q/%q", address.MemberID, address.ConditionRef, address.SourceID)
}

type holdFixtureInput struct {
	Member   string
	Source   *core.Ref
	Name     string
	Turns    int
	Skip     bool
	Children []dndevents.ConditionAddress
}

// This constructs an already-delivered saved fixture, not a historical reader.
// Real paid casting and per-entry admission are exercised by LinkedApplicationSuite.
func fixtureHold(t *testing.T, in holdFixtureInput) *conditions.LinkedCondition {
	t.Helper()
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
	config, err := json.Marshal(conditions.LinkedConditionConfig{ApplicationID: fixtureApplicationID(in.Member), Role: conditions.LinkedCoordinator, AddressSourceID: fixtureApplicationID(in.Member), Bindings: &conditions.LinkedBindings{Name: in.Name, Members: members, Parts: []effects.PartDeclaration{
		{Key: "hold", Kind: effects.KindConcentration, OnTerminal: generic.EndApplication},
		{Key: "clock", Kind: effects.KindDuration, Params: clock, OnTerminal: generic.EndApplication},
		{Key: "empty", Kind: effects.KindUntilEmpty, OnTerminal: generic.EndApplication},
		{Key: "rest", Kind: effects.KindEndOn, Params: json.RawMessage(`{"beat":"rest","reason":"rest"}`), OnTerminal: generic.EndApplication},
		{Key: "combat", Kind: effects.KindEndOn, Params: json.RawMessage(`{"beat":"combat_end","reason":"combat_end"}`), OnTerminal: generic.EndApplication},
	}}})
	require.NoError(t, err)
	created, err := conditions.CreateFromRef(&conditions.CreateFromRefInput{Ref: refs.Conditions.LinkedEffects().String(), MemberID: in.Member, Origin: &dndevents.ConditionOrigin{Ref: *in.Source, EntityID: in.Member}, Config: config})
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

func fixtureOwnerParticipant(t *testing.T, member string, source *core.Ref, children ...dndevents.ConditionAddress) Participant {
	t.Helper()
	hold := fixtureHold(t, holdFixtureInput{Member: member, Source: source, Name: "Fixture hold", Turns: 10, Children: children})
	raw, err := hold.ToJSON()
	require.NoError(t, err)
	data := monsters.NewWolf(member).ToData()
	data.Conditions = []json.RawMessage{raw}
	return Participant{Monster: data}
}

type holdView struct {
	SourceRef      core.Ref
	SourceName     string
	Remaining      int
	Grace          bool
	SourceEntityID string
	Children       []dndevents.ConditionAddress
	Address        dndevents.ConditionAddress
}

func fixtureHoldView(t *testing.T, raw json.RawMessage) *holdView {
	t.Helper()
	condition, err := conditions.LoadJSON(raw)
	require.NoError(t, err)
	linked, ok := condition.(*conditions.LinkedCondition)
	if !ok {
		return nil
	}
	info, err := linked.MaintenanceInfo()
	require.NoError(t, err)
	if info.Concentration == nil {
		return nil
	}
	out := &holdView{SourceRef: info.Concentration.SourceRef, SourceName: info.Concentration.SourceName, Address: info.Concentration.Owner, SourceEntityID: linked.ConditionOrigin().EntityID}
	if info.Duration != nil {
		out.Remaining = info.Duration.Remaining
		out.Grace = info.Duration.SkipNext
	}
	application := linked.LinkedApplicationInfo()
	for _, member := range application.Members {
		if slices.Contains(application.LiveMembers, member.Key) {
			out.Children = append(out.Children, member.Address)
		}
	}
	return out
}
