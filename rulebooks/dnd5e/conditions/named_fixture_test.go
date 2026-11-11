// Copyright (C) 2026 Kirk Diggler
// SPDX-License-Identifier: GPL-3.0-or-later

package conditions

import (
	"encoding/json"
	"fmt"

	"github.com/KirkDiggler/rpg-toolkit/core"
	dndevents "github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/events"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/refs"
)

type namedFixtureInput struct {
	Ref       *core.Ref
	MemberID  string
	SourceID  string
	SourceRef *core.Ref
}

// namedFixture creates current data for local behavior tests. It does not claim
// a complete application graph; application admission is exercised separately.
func namedFixture(in namedFixtureInput) (*LinkedCondition, error) {
	if in.Ref == nil || in.SourceRef == nil {
		return nil, fmt.Errorf("fixture requires explicit refs")
	}
	owner := dndevents.ConditionAddress{MemberID: in.SourceID, ConditionRef: refs.Conditions.LinkedEffects().String(), SourceID: "application:" + in.SourceID}
	config, err := json.Marshal(LinkedConditionConfig{ApplicationID: owner.SourceID, AddressSourceID: in.SourceID, Role: LinkedRecipient,
		Bindings: &LinkedBindings{Name: map[bool]string{true: "Bane", false: "Bless"}[in.Ref.String() == refs.Conditions.Baned().String()],
			Owner: &owner, MemberKey: in.MemberID}})
	if err != nil {
		return nil, err
	}
	created, err := CreateFromRef(&CreateFromRefInput{Ref: in.Ref.String(), MemberID: in.MemberID,
		Origin: &dndevents.ConditionOrigin{Ref: *in.SourceRef, EntityID: in.SourceID}, Config: config})
	if err != nil {
		return nil, err
	}
	linked, ok := created.Condition.(*LinkedCondition)
	if !ok {
		return nil, fmt.Errorf("fixture ref did not create linked behavior")
	}
	return linked, nil
}

func namedBlobForTest(in namedFixtureInput) json.RawMessage {
	condition, err := namedFixture(in)
	if err != nil {
		panic(err)
	}
	raw, err := condition.ToJSON()
	if err != nil {
		panic(err)
	}
	return raw
}
