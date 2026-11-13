// Copyright (C) 2026 Kirk Diggler
// SPDX-License-Identifier: GPL-3.0-or-later

package conditions

import (
	"encoding/json"
	"errors"
	"testing"

	"github.com/stretchr/testify/suite"

	generic "github.com/KirkDiggler/rpg-toolkit/mechanics/effects"
	dndfx "github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/effects"
	dndevents "github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/events"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/refs"
)

type namedLinkedConstructionSuite struct{ suite.Suite }

func TestNamedLinkedConstructionSuite(t *testing.T) { suite.Run(t, new(namedLinkedConstructionSuite)) }

func (s *namedLinkedConstructionSuite) TestNamedFactoryErrorPropagates() {
	condition, err := namedFixture(namedFixtureInput{Ref: refs.Conditions.Blessed(), MemberID: "recipient", SourceID: "caster-a", SourceRef: refs.Spells.Bless()})
	s.Require().NoError(err)
	stored, err := condition.ToJSON()
	s.Require().NoError(err)
	cause := errors.New("invalid built-in definition")
	calls := 0
	bad := namedLinkedDescriptor(namedLinkedDescriptorInput{
		Ref: refs.Conditions.Blessed(), Display: Display{Name: "Bless", Detail: "Adds to rolls"},
		Parts: func() ([]dndfx.PartDeclaration, error) { calls++; return nil, cause },
	})
	s.Zero(calls, "registering a definition cannot compile it through an initialization panic")
	key := refs.Conditions.Blessed().String()
	original := registry[key]
	registry[key] = bad
	s.T().Cleanup(func() { registry[key] = original })
	cfg := linkedRecipientConfig()
	cfg.Bindings.Parts = nil
	raw, err := json.Marshal(cfg)
	s.Require().NoError(err)
	made, err := CreateFromRef(&CreateFromRefInput{Ref: key, MemberID: "recipient", Origin: &dndevents.ConditionOrigin{Ref: *refs.Spells.Bless(), EntityID: "caster-a"}, Config: raw})
	s.ErrorIs(err, cause)
	s.Nil(made)
	loaded, err := LoadJSON(stored)
	s.ErrorIs(err, cause)
	s.Nil(loaded)
	prepared, err := PrepareApplication(&PrepareApplicationInput{
		ID: "new-application", CoordinatorID: "caster-a", Origin: dndevents.ConditionOrigin{Ref: *refs.Spells.Bless(), EntityID: "caster-a"},
		Existing: []ParticipantConditions{{MemberID: "caster-a"}, {MemberID: "recipient"}},
		Spec:     ApplicationSpec{Name: "Bless", CoordinatorParts: []dndfx.PartDeclaration{{Key: "hold", Kind: dndfx.KindConcentration, OnTerminal: generic.EndApplication}}, Recipients: []RecipientSpec{{MemberID: "recipient", Ref: key, AddressSourceID: "caster-a", Lifetime: LifetimeShared}}},
	})
	s.ErrorIs(err, cause)
	s.Nil(prepared)
	s.Equal(3, calls, "each error reaches its caller; no failed factory becomes absent content")
}

func (s *namedLinkedConstructionSuite) TestBlessBaneHaveExplicitRest() {
	for _, entry := range []descriptor{blessedDescriptor, banedDescriptor} {
		parts, err := entry.linkedParts()
		s.Require().NoError(err)
		s.Require().Len(parts, 2)
		s.Equal(dndfx.KindAddToRoll, parts[0].Kind)
		s.Equal(generic.EndPart, parts[0].OnTerminal)
		s.Equal(dndfx.KindEndOn, parts[1].Kind)
		s.Equal(generic.EndAttachment, parts[1].OnTerminal)
		s.JSONEq(`{"beat":"rest","reason":"rest"}`, string(parts[1].Params))
		parts[0].Amount.Dice = "9d20"
		again, err := entry.linkedParts()
		s.Require().NoError(err)
		s.Equal("1d4", again[0].Amount.Dice, "editing one construction cannot rewrite named content")
	}
}

func (s *namedLinkedConstructionSuite) TestEmptyNamedDefinitionRefuses() {
	entry := namedLinkedDescriptor(namedLinkedDescriptorInput{Ref: refs.Conditions.Blessed(), Display: Display{Name: "Bless", Detail: "Adds to rolls"}, Parts: func() ([]dndfx.PartDeclaration, error) { return nil, nil }})
	parts, err := entry.linkedParts()
	s.Error(err)
	s.Nil(parts)
}
