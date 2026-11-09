// Copyright (C) 2026 Kirk Diggler
// SPDX-License-Identifier: GPL-3.0-or-later

package character

import (
	"encoding/json"
	"fmt"

	"github.com/KirkDiggler/rpg-toolkit/core"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/conditions"
	dndevents "github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/events"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/refs"
)

type namedFixtureInput struct {
	Ref                *core.Ref
	MemberID, SourceID string
	SourceRef          *core.Ref
}

// namedFixture supplies current creator-produced data to local sheet tests;
// these fixtures do not claim a native cast or a complete application graph.
func namedFixture(in namedFixtureInput) (*conditions.LinkedCondition, error) {
	if in.Ref == nil || in.SourceRef == nil {
		return nil, fmt.Errorf("fixture requires refs")
	}
	owner := dndevents.ConditionAddress{MemberID: in.SourceID, ConditionRef: refs.Conditions.LinkedEffects().String(), SourceID: "application:" + in.SourceID}
	name := "Bless"
	if in.Ref.String() == refs.Conditions.Baned().String() {
		name = "Bane"
	}
	raw, err := json.Marshal(conditions.LinkedConditionConfig{ApplicationID: owner.SourceID, AddressSourceID: in.SourceID, Role: conditions.LinkedRecipient,
		Bindings: &conditions.LinkedBindings{Name: name, Owner: &owner, MemberKey: in.MemberID}})
	if err != nil {
		return nil, err
	}
	out, err := conditions.CreateFromRef(&conditions.CreateFromRefInput{Ref: in.Ref.String(), MemberID: in.MemberID, Origin: &dndevents.ConditionOrigin{Ref: *in.SourceRef, EntityID: in.SourceID}, Config: raw})
	if err != nil {
		return nil, err
	}
	return out.Condition.(*conditions.LinkedCondition), nil
}
