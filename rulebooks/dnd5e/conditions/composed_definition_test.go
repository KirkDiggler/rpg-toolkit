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
	generic "github.com/KirkDiggler/rpg-toolkit/mechanics/effects"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/abilities"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/combat"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/conditions"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/contributions"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/effects"
	dndevents "github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/events"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/refs"
)

type composedDefinitionRestoreSuite struct{ suite.Suite }

func TestComposedDefinitionRestoreSuite(t *testing.T) {
	suite.Run(t, new(composedDefinitionRestoreSuite))
}

type localDefinitionWire struct {
	Ref      core.Ref                   `json:"ref"`
	MemberID string                     `json:"member_id"`
	Origin   *dndevents.ConditionOrigin `json:"origin"`
	Bindings *struct {
		Parts []effects.PartDeclaration `json:"parts"`
	} `json:"bindings"`
	Runtime *generic.AssemblySnapshot `json:"runtime"`
}

func (s *composedDefinitionRestoreSuite) create() dndevents.ConditionBehavior {
	out, err := conditions.CreateFromRef(&conditions.CreateFromRefInput{Ref: refs.Conditions.Raging().String(), MemberID: "holder", Origin: &dndevents.ConditionOrigin{Ref: *refs.Features.Rage(), EntityID: "holder"}, Config: json.RawMessage(`{"damage_bonus":{"fixed":3}}`)})
	s.Require().NoError(err)
	return out.Condition
}
func (s *composedDefinitionRestoreSuite) saved(c dndevents.ConditionBehavior) localDefinitionWire {
	raw, err := c.ToJSON()
	s.Require().NoError(err)
	var data localDefinitionWire
	s.Require().NoError(json.Unmarshal(raw, &data))
	s.Require().NotNil(data.Bindings)
	s.Require().NotEmpty(data.Bindings.Parts, "active state retains resolved declarations, not a latest definition selector")
	s.Require().NotNil(data.Runtime)
	s.Require().NotEmpty(data.Runtime.Parts)
	return data
}
func (s *composedDefinitionRestoreSuite) TestResolvedPartsAreTheSavedAuthority() {
	c := s.create()
	data := s.saved(c)
	s.Equal(refs.Conditions.Raging().String(), data.Ref.String())
	s.Equal("holder", data.Origin.EntityID)
	found := false
	for i := range data.Bindings.Parts {
		if data.Bindings.Parts[i].Kind == effects.KindDamageBonus {
			found = true
			value := 5
			data.Bindings.Parts[i].Amount.Fixed = &value
		}
	}
	s.True(found)
	changed, err := json.Marshal(data)
	s.Require().NoError(err)
	restored, err := conditions.LoadJSON(changed)
	s.Require().NoError(err)
	frame := contributions.Frame{Actor: "holder", Action: contributions.ActionFacts{Roll: contributions.Known(contributions.RollKindAttack), Ability: contributions.Known(abilities.STR), Melee: contributions.Known(true), WeaponPool: contributions.Known(true)}}
	rows, err := conditions.AssessActionEffects(&conditions.AssessActionEffectsInput{Conditions: []dndevents.ConditionBehavior{restored}, Frame: frame})
	s.Require().NoError(err)
	s.Equal("+5 damage", rows.Effects[0].Benefit, "load uses the valid saved resolved amount, not a preset rebuilt from input bindings")
	original, err := conditions.AssessActionEffects(&conditions.AssessActionEffectsInput{Conditions: []dndevents.ConditionBehavior{c}, Frame: frame})
	s.Require().NoError(err)
	s.Equal("+3 damage", original.Effects[0].Benefit)
}
func (s *composedDefinitionRestoreSuite) TestProgressIsDescribedByProviderAfterReload() {
	c := s.create()
	ctx, bus := context.Background(), events.NewEventBus()
	s.Require().NoError(c.Apply(ctx, bus))
	s.Require().NoError(dndevents.TurnEndTopic.On(bus).Publish(ctx, dndevents.TurnEndEvent{SubjectID: "holder", Round: 4}))
	raw, err := c.ToJSON()
	s.Require().NoError(err)
	s.Require().NoError(c.Remove(ctx, bus))
	loaded, err := conditions.LoadJSON(raw)
	s.Require().NoError(err)
	s.Require().NoError(loaded.Apply(ctx, bus))
	s.T().Cleanup(func() { s.NoError(loaded.Remove(ctx, bus)) })
	_, err = dndevents.PostAttackRollChain.On(bus).PublishWithChain(ctx, &dndevents.PostAttackRollEvent{AttackerID: "holder", WouldHit: false}, events.NewStagedChain[*dndevents.PostAttackRollEvent](combat.ModifierStages))
	s.Require().NoError(err)
	s.Require().NoError(dndevents.TurnEndTopic.On(bus).Publish(ctx, dndevents.TurnEndEvent{SubjectID: "holder", Round: 9}))
	info, err := conditions.DescribeCondition(&conditions.DescribeConditionInput{Condition: loaded})
	s.Require().NoError(err)
	s.Contains(info.Display.Detail, "Active for 6 turns")
	s.Equal("holder", info.Origin.EntityID)
	var reader map[string]json.RawMessage
	raw, err = loaded.ToJSON()
	s.Require().NoError(err)
	s.Require().NoError(json.Unmarshal(raw, &reader))
	var runtime map[string]json.RawMessage
	s.Require().NoError(json.Unmarshal(reader["runtime"], &runtime))
	s.Contains(runtime, "parts")
	s.NotContains(runtime, "bag")
	s.NotContains(runtime, "state")
}
func (s *composedDefinitionRestoreSuite) TestIncompleteTerminalOrContradictoryStateRefuses() {
	c := s.create()
	ctx, bus := context.Background(), events.NewEventBus()
	s.Require().NoError(c.Apply(ctx, bus))
	s.Require().NoError(dndevents.TurnEndTopic.On(bus).Publish(ctx, dndevents.TurnEndEvent{SubjectID: "holder", Round: 4}))
	data := s.saved(c)
	s.Require().NoError(c.Remove(ctx, bus))
	original, err := json.Marshal(data)
	s.Require().NoError(err)
	for _, mutate := range []func(*localDefinitionWire){
		func(d *localDefinitionWire) { d.Runtime = nil },
		func(d *localDefinitionWire) { delete(d.Runtime.Parts, "maintenance/duration") },
		func(d *localDefinitionWire) {
			d.Runtime.Parts["extra"] = generic.Snapshot{State: "active", Bag: map[string]any{}}
		},
		func(d *localDefinitionWire) {
			state := d.Runtime.Parts["maintenance/duration"]
			state.State = "ended"
			d.Runtime.Parts["maintenance/duration"] = state
		},
		func(d *localDefinitionWire) {
			state := d.Runtime.Parts["maintenance/combat-activity"]
			state.State = "fresh"
			d.Runtime.Parts["maintenance/combat-activity"] = state
		},
		func(d *localDefinitionWire) { d.Origin = nil },
		func(d *localDefinitionWire) { d.Bindings = nil },
		func(d *localDefinitionWire) { d.Bindings.Parts[0].Amount = nil },
		func(d *localDefinitionWire) { d.Bindings.Parts[0].OnTerminal = generic.EndApplication },
	} {
		var invalid localDefinitionWire
		s.Require().NoError(json.Unmarshal(original, &invalid))
		mutate(&invalid)
		raw, err := json.Marshal(invalid)
		s.Require().NoError(err)
		got, err := conditions.LoadJSON(raw)
		s.Error(err)
		s.Nil(got)
	}
	old := json.RawMessage(`{"ref":"dnd5e:conditions:raging","member_id":"holder","origin":{"ref":"dnd5e:features:rage","entity_id":"holder"},"bindings":{"damage_bonus":{"fixed":3}},"runtime":{"state":"fresh","bag":{"saw_turn_end":false,"round_activated":0,"turns_active":0,"was_hit_this_turn":false,"did_attack_this_turn":false}}}`)
	got, err := conditions.LoadJSON(old)
	s.Error(err)
	s.Nil(got)
}
