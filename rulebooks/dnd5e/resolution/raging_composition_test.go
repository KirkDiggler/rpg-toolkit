// Copyright (C) 2026 Kirk Diggler
// SPDX-License-Identifier: GPL-3.0-or-later

package resolution

import (
	"context"
	"encoding/json"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/KirkDiggler/rpg-toolkit/dice"
	"github.com/KirkDiggler/rpg-toolkit/events"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/conditions"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/encounter"
	dndevents "github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/events"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/monster/monsters"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/refs"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/resources"
)

// ragingForTest constructs current data through the real shared creator rather
// than retaining a fixture dependency on the removed handwritten runtime type.
func ragingForTest(t testing.TB, member string) dndevents.ConditionBehavior {
	t.Helper()
	out, err := conditions.CreateFromRef(&conditions.CreateFromRefInput{Ref: refs.Conditions.Raging().String(), MemberID: member, Origin: &dndevents.ConditionOrigin{Ref: *refs.Features.Rage(), EntityID: member}, Config: json.RawMessage(`{"damage_bonus":{"barbarian_level":true}}`)})
	require.NoError(t, err)
	return out.Condition
}

// TestNativeDamageSustainsRagingAcrossReload covers the post-apply protocol,
// not a test publishing the legacy instruction-like DamageReceived event.
func (s *ActivationTestSuite) TestNativeDamageSustainsRagingAcrossReload() {
	wolf := monsters.NewWolf(wolfID).ToData()
	activated, err := s.run(&ActivationInput{MemberID: heroID, Ability: refs.Features.Rage(), Roller: dice.NewRoller()}, actionWorld(s.T(), 2), []Participant{{Character: s.barbarian(2)}, {Monster: wolf}})
	s.Require().NoError(err)
	boundary, err := NewBoundary(&BoundaryInput{Crossed: []encounter.Boundary{{Kind: encounter.TurnEnded, Subject: heroID, Round: 4}}})
	s.Require().NoError(err)
	first, err := s.resolveMachine(boundary, activated.World, []Participant{{Character: dirty(activated, heroID)}, {Monster: wolf}})
	s.Require().NoError(err)
	live := dirty(first, heroID)
	s.Require().NotNil(live)
	bus := events.NewEventBus()
	legacyNotifications := 0
	_, err = dndevents.DamageReceivedTopic.On(bus).Subscribe(s.ctx, func(context.Context, dndevents.DamageReceivedEvent) error { legacyNotifications++; return nil })
	s.Require().NoError(err)
	hit, err := s.resolveOnBus(bus, NewStrike(&StrikeInput{AttackerID: wolfID, TargetID: heroID, Definition: claw("1d6"), Roller: &actionRoller{singles: []int{18}, damage: [][]int{{6}}}}), first.World, []Participant{{Character: live}, {Monster: wolf}}, nil)
	s.Require().NoError(err)
	struck := hit.Outcome.(StrikeOutcome)
	s.Require().True(struck.Hit)
	s.Equal(3, struck.Damage, "six physical damage is resisted once")
	damaged := dirty(hit, heroID)
	s.Require().NotNil(damaged)
	s.Equal(12, damaged.HitPoints, "applied once to the original 15 HP")
	s.Zero(legacyNotifications, "the resolution must not republish the legacy damage instruction")
	var saved conditions.ComposedData
	s.Require().NoError(json.Unmarshal(conditionWithRefOrNil(damaged.Conditions, refs.Conditions.Raging()), &saved))
	s.Require().NotNil(saved.Runtime)
	s.Equal(true, saved.Runtime.Bag["was_hit_this_turn"], "positive native damage is persisted as sustain activity")
	boundary, err = NewBoundary(&BoundaryInput{Crossed: []encounter.Boundary{{Kind: encounter.TurnEnded, Subject: heroID, Round: 5}}})
	s.Require().NoError(err)
	next, err := s.resolveMachine(boundary, hit.World, []Participant{{Character: damaged}, {Monster: wolf}})
	s.Require().NoError(err)
	sustained := dirty(next, heroID)
	s.Require().NotNil(sustained)
	raw := conditionWithRefOrNil(sustained.Conditions, refs.Conditions.Raging())
	s.Require().NotEmpty(raw, "damage, not a renewed grace turn, sustains the reloaded condition")
	s.Require().NoError(json.Unmarshal(raw, &saved))
	s.Equal(float64(2), saved.Runtime.Bag["turns_active"])
	s.Equal(false, saved.Runtime.Bag["was_hit_this_turn"])
	s.Equal(1, sustained.Resources[resources.RageCharges].Current)
	boundary, err = NewBoundary(&BoundaryInput{Crossed: []encounter.Boundary{{Kind: encounter.TurnEnded, Subject: heroID, Round: 6}}})
	s.Require().NoError(err)
	ended, err := s.resolveMachine(boundary, next.World, []Participant{{Character: sustained}, {Monster: wolf}})
	s.Require().NoError(err)
	s.NotContains(conditionRefs(dirty(ended, heroID)), refs.Conditions.Raging().String(), "one hit cannot sustain every later turn")
}

func (s *ActivationTestSuite) TestRagingCompositionSurvivesInteractionReload() {
	activated, err := s.run(&ActivationInput{MemberID: heroID, Ability: refs.Features.Rage(), Roller: dice.NewRoller()}, s.world(), []Participant{{Character: s.barbarian(2)}})
	s.Require().NoError(err)
	sheet := dirty(activated, heroID)
	s.Require().NotNil(sheet)
	s.Equal(1, sheet.Resources[resources.RageCharges].Current)
	boundary, err := NewBoundary(&BoundaryInput{Crossed: []encounter.Boundary{{Kind: encounter.TurnEnded, Subject: heroID, Round: 4}}})
	s.Require().NoError(err)
	first, err := s.resolveMachine(boundary, activated.World, []Participant{{Character: sheet}})
	s.Require().NoError(err)
	changed := dirty(first, heroID)
	s.Require().NotNil(changed, "first nonterminal transition must leave a dirty sheet across the real resolution boundary")
	var stored conditions.ComposedData
	s.Require().NoError(json.Unmarshal(conditionWithRefOrNil(changed.Conditions, refs.Conditions.Raging()), &stored))
	s.Require().NotNil(stored.Runtime)
	s.Equal("raging", stored.Runtime.State)
	s.Equal(float64(1), stored.Runtime.Bag["turns_active"])
	s.Equal(heroID, stored.Origin.EntityID)
	s.Equal(1, changed.Resources[resources.RageCharges].Current)
	boundary, err = NewBoundary(&BoundaryInput{Crossed: []encounter.Boundary{{Kind: encounter.TurnEnded, Subject: heroID, Round: 5}}})
	s.Require().NoError(err)
	second, err := s.resolveMachine(boundary, first.World, []Participant{{Character: changed}})
	s.Require().NoError(err)
	ended := dirty(second, heroID)
	s.Require().NotNil(ended)
	s.NotContains(conditionRefs(ended), refs.Conditions.Raging().String(), "a new interaction must not re-grant first-turn grace")
	s.Equal(1, ended.Resources[resources.RageCharges].Current)
}
