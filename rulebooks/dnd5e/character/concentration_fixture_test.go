// Copyright (C) 2026 Kirk Diggler
// SPDX-License-Identifier: GPL-3.0-or-later

package character

import (
	"encoding/json"

	"github.com/KirkDiggler/rpg-toolkit/core"
	generic "github.com/KirkDiggler/rpg-toolkit/mechanics/effects"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/conditions"
	dndfx "github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/effects"
	dndevents "github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/events"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/refs"
)

type holdFixtureInput struct {
	Member  string
	Source  *core.Ref
	Name    string
	Turns   int
	Skip    bool
	Members []conditions.LinkedMemberBinding
}

func currentHoldFixture(in holdFixtureInput) (*conditions.LinkedCondition, error) {
	clock, err := json.Marshal(struct {
		TurnEnds int  `json:"turn_ends"`
		Skip     bool `json:"skip_first"`
	}{in.Turns, in.Skip})
	if err != nil {
		return nil, err
	}
	config, err := json.Marshal(conditions.LinkedConditionConfig{ApplicationID: "application:" + in.Member, AddressSourceID: "application:" + in.Member, Role: conditions.LinkedCoordinator,
		Bindings: &conditions.LinkedBindings{Name: in.Name, Members: in.Members, Parts: []dndfx.PartDeclaration{
			{Key: "concentration", Kind: dndfx.KindConcentration, OnTerminal: generic.EndApplication},
			{Key: "duration", Kind: dndfx.KindDuration, Params: clock, OnTerminal: generic.EndApplication},
			{Key: "empty", Kind: dndfx.KindUntilEmpty, OnTerminal: generic.EndApplication},
			{Key: "rest", Kind: dndfx.KindEndOn, Params: json.RawMessage(`{"beat":"rest","reason":"rest"}`), OnTerminal: generic.EndApplication},
			{Key: "combat", Kind: dndfx.KindEndOn, Params: json.RawMessage(`{"beat":"combat_end","reason":"combat_end"}`), OnTerminal: generic.EndApplication},
		}}})
	if err != nil {
		return nil, err
	}
	out, err := conditions.CreateFromRef(&conditions.CreateFromRefInput{Ref: refs.Conditions.LinkedEffects().String(), MemberID: in.Member, Origin: &dndevents.ConditionOrigin{Ref: *in.Source, EntityID: in.Member}, Config: config})
	if err != nil {
		return nil, err
	}
	// This fixture represents already-delivered persisted members, not a fresh
	// pending cast. Live creation/finalization is covered by real binding tests.
	raw, err := out.Condition.ToJSON()
	if err != nil {
		return nil, err
	}
	var data conditions.LinkedConditionData
	if err = json.Unmarshal(raw, &data); err != nil {
		return nil, err
	}
	data.Runtime.Lifetime.LiveMembers = append([]string{}, data.Runtime.Lifetime.PendingMembers...)
	data.Runtime.Lifetime.PendingMembers = []string{}
	raw, err = json.Marshal(data)
	if err != nil {
		return nil, err
	}
	loaded, err := conditions.LoadJSON(raw)
	if err != nil {
		return nil, err
	}
	return loaded.(*conditions.LinkedCondition), nil
}
