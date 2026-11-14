// Copyright (C) 2026 Kirk Diggler
// SPDX-License-Identifier: GPL-3.0-or-later

package conditions_test

import (
	"context"
	"encoding/json"
	"testing"

	"github.com/stretchr/testify/suite"

	"github.com/KirkDiggler/rpg-toolkit/core"
	"github.com/KirkDiggler/rpg-toolkit/events"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/conditions"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/contributions"
	dndevents "github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/events"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/refs"
)

type composedRagingSuite struct{ suite.Suite }

func itemOrigin() *dndevents.ConditionOrigin {
	return &dndevents.ConditionOrigin{Ref: core.Ref{Module: "example", Type: "items", ID: "raging_ring"}, EntityID: "ring-17"}
}

func (s *composedRagingSuite) create(bindings string) dndevents.ConditionBehavior {
	out, err := conditions.CreateFromRef(&conditions.CreateFromRefInput{Ref: refs.Conditions.Raging().String(), MemberID: "holder", Origin: itemOrigin(), Config: json.RawMessage(bindings)})
	s.Require().NoError(err)
	return out.Condition
}

func (s *composedRagingSuite) data(c dndevents.ConditionBehavior) conditions.ComposedData {
	raw, err := c.ToJSON()
	s.Require().NoError(err)
	var data conditions.ComposedData
	s.Require().NoError(json.Unmarshal(raw, &data))
	return data
}

func (s *composedRagingSuite) TestSharedRagingCreationHasIndependentStateAndOrigin() {
	first := s.create(`{"damage_bonus":{"fixed":3}}`)
	second := s.create(`{"damage_bonus":{"barbarian_level":true}}`)
	ctx := context.Background()
	bus := events.NewEventBus()
	s.Require().NoError(first.Apply(ctx, bus))
	s.T().Cleanup(func() { s.NoError(first.Remove(ctx, bus)) })
	changed := 0
	_, err := dndevents.ConditionStateChangedTopic.On(bus).Subscribe(ctx, func(_ context.Context, event dndevents.ConditionStateChangedEvent) error {
		if event.MemberID == "holder" {
			changed++
		}
		return nil
	})
	s.Require().NoError(err)
	s.Require().NoError(dndevents.TurnEndTopic.On(bus).Publish(ctx, dndevents.TurnEndEvent{SubjectID: "holder", Round: 4}))
	a, b := s.data(first), s.data(second)
	s.Equal("active", a.Runtime.Parts["maintenance/combat-activity"].State)
	s.Equal(float64(1), a.Runtime.Parts["maintenance/duration"].Bag["turns_active"])
	s.Equal("fresh", b.Runtime.Parts["maintenance/combat-activity"].State)
	s.Equal(float64(0), b.Runtime.Parts["maintenance/duration"].Bag["turns_active"])
	s.Equal(2, changed, "the independent grace and clock changes both reach the idempotent host dirty-state topic")
	origin, ok := first.(dndevents.ConditionOriginProvider)
	s.Require().True(ok)
	s.Equal("ring-17", origin.ConditionOrigin().EntityID)
	if addressed, ok := first.(dndevents.ConditionAddressProvider); ok {
		s.Empty(addressed.ConditionAddress().SourceID, "origin is not stacking identity")
	}
	s.Require().NotNil(a.Bindings.Parts[0].Amount.Fixed)
	s.Equal(3, *a.Bindings.Parts[0].Amount.Fixed)
	s.False(second.IsApplied())
}

func (s *composedRagingSuite) TestStrictRestorePreservesProgressAndRefusesOldOrIncompleteState() {
	c := s.create(`{"damage_bonus":{"fixed":3}}`)
	ctx := context.Background()
	bus := events.NewEventBus()
	s.Require().NoError(c.Apply(ctx, bus))
	s.Require().NoError(dndevents.TurnEndTopic.On(bus).Publish(ctx, dndevents.TurnEndEvent{SubjectID: "holder", Round: 4}))
	raw, err := c.ToJSON()
	s.Require().NoError(err)
	s.Require().NoError(c.Remove(ctx, bus))
	restored, err := conditions.LoadJSON(raw)
	s.Require().NoError(err)
	s.False(restored.IsApplied())
	s.Require().NoError(restored.Apply(ctx, bus))
	s.Require().NoError(dndevents.TurnEndTopic.On(bus).Publish(ctx, dndevents.TurnEndEvent{SubjectID: "holder", Round: 5}))
	s.False(restored.IsApplied(), "reload must not grant a second first-turn grace")
	s.Equal("ring-17", s.data(restored).Origin.EntityID)
	for _, mutate := range []func(*conditions.ComposedData){
		func(d *conditions.ComposedData) { d.Runtime = nil },
		func(d *conditions.ComposedData) {
			delete(d.Runtime.Parts["maintenance/combat-activity"].Bag, "attacked")
		},
		func(d *conditions.ComposedData) { d.Runtime.Parts["maintenance/combat-activity"].Bag["extra"] = true },
		func(d *conditions.ComposedData) {
			state := d.Runtime.Parts["maintenance/combat-activity"]
			state.State = "fresh"
			d.Runtime.Parts["maintenance/combat-activity"] = state
		},
		func(d *conditions.ComposedData) {
			state := d.Runtime.Parts["maintenance/combat-activity"]
			state.State = "ended"
			d.Runtime.Parts["maintenance/combat-activity"] = state
		},
		func(d *conditions.ComposedData) { d.Origin = nil },
		func(d *conditions.ComposedData) { d.Bindings.Parts[0].Amount = nil },
	} {
		var data conditions.ComposedData
		s.Require().NoError(json.Unmarshal(raw, &data))
		mutate(&data)
		bad, err := json.Marshal(data)
		s.Require().NoError(err)
		got, err := conditions.LoadJSON(bad)
		s.Error(err)
		s.Nil(got)
	}
	old := json.RawMessage(`{"ref":"dnd5e:conditions:raging","member_id":"holder","source":"dnd5e:features:rage","turns_active":1}`)
	got, err := conditions.LoadJSON(old)
	s.Error(err)
	s.Nil(got, "old characters are recreated, never normalized silently")
}

func (s *composedRagingSuite) TestCreationRefusesAmbiguousOrMissingBindings() {
	for _, bindings := range []string{``, `null`, `{}`, `{"damage_bonus":{"fixed":0}}`, `{"damage_bonus":{"fixed":3,"barbarian_level":true}}`, `{"damage_bonus":{"fixed":3},"extra":1}`, `{"damage_bonus":{"fixed":3}} {}`} {
		out, err := conditions.CreateFromRef(&conditions.CreateFromRefInput{Ref: refs.Conditions.Raging().String(), MemberID: "holder", Origin: itemOrigin(), Config: json.RawMessage(bindings)})
		s.Error(err)
		s.Nil(out)
	}
	for _, in := range []*conditions.CreateFromRefInput{
		{Ref: refs.Conditions.Raging().String(), MemberID: "holder", Config: json.RawMessage(`{"damage_bonus":{"fixed":3}}`)},
		{Ref: refs.Conditions.Raging().String(), MemberID: "holder", Origin: itemOrigin(), SourceRef: refs.Features.Rage().String(), Config: json.RawMessage(`{"damage_bonus":{"fixed":3}}`)},
		{Ref: refs.Conditions.Dodging().String(), MemberID: "holder", Origin: itemOrigin()},
	} {
		out, err := conditions.CreateFromRef(in)
		s.Error(err)
		s.Nil(out)
	}
	s.NoError(conditions.ValidateBindings(&conditions.ValidateBindingsInput{Ref: refs.Conditions.Raging().String(), Bindings: json.RawMessage(`{"damage_bonus":{"fixed":3}}`)}))
	s.Error(conditions.ValidateBindings(&conditions.ValidateBindingsInput{Ref: refs.Conditions.Dodging().String(), Bindings: json.RawMessage(`{}`)}))
}

func (s *composedRagingSuite) TestFourInformationRowsDoNotApplyAllBenefitsToAnAttack() {
	c := s.create(`{"damage_bonus":{"fixed":3}}`)
	frame := contributions.Frame{Actor: "holder", Action: contributions.ActionFacts{Roll: contributions.Known(contributions.RollKindAttack)}}
	out, err := conditions.AssessActionEffects(&conditions.AssessActionEffectsInput{Conditions: []dndevents.ConditionBehavior{c}, Frame: frame})
	s.Require().NoError(err)
	s.Require().Len(out.Effects, 4)
	ids := map[string]bool{}
	for i, effect := range out.Effects {
		s.False(ids[effect.ID])
		ids[effect.ID] = true
		s.Equal("ring-17", effect.Source.SourceID)
		s.NotEmpty(effect.Description)
		want := contributions.StateDoesNotApply
		if i == 0 {
			want = contributions.StateDepends
		}
		s.Equal(want, effect.State)
		s.Empty(effect.Benefit)
	}
}

func (s *composedRagingSuite) TestCanonicalDataCarriesStringRefsForReaders() {
	c := s.create(`{"damage_bonus":{"fixed":3}}`)
	raw, err := c.ToJSON()
	s.Require().NoError(err)
	var reader struct {
		Ref    string `json:"ref"`
		Origin struct {
			Ref string `json:"ref"`
		} `json:"origin"`
	}
	s.Require().NoError(json.Unmarshal(raw, &reader), "the native reader identifies the canonical string ref")
	s.Equal("dnd5e:conditions:raging", reader.Ref)
	s.Equal("example:items:raging_ring", reader.Origin.Ref)
}

func TestSharedComposedRaging(t *testing.T) { suite.Run(t, new(composedRagingSuite)) }
