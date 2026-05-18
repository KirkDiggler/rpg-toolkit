// Copyright (C) 2024 Kirk Diggler
// SPDX-License-Identifier: GPL-3.0-or-later

package conditions_test

import (
	"context"
	"encoding/json"
	"sync"
	"testing"

	"github.com/stretchr/testify/suite"

	"github.com/KirkDiggler/rpg-toolkit/core"
	"github.com/KirkDiggler/rpg-toolkit/events"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/combat"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/conditions"
	dnd5eEvents "github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/events"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/gamectx"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/refs"
	"github.com/KirkDiggler/rpg-toolkit/tools/spatial"
)

// oaTestEntity implements core.Entity for room placement in OA tests.
type oaTestEntity struct {
	id         string
	entityType core.EntityType
}

func (e *oaTestEntity) GetID() string            { return e.id }
func (e *oaTestEntity) GetType() core.EntityType { return e.entityType }

// OpportunityAttackConditionSuite covers the OA condition's MovementChain
// subscription, geometry predicate, readiness gate, and JSON round-trip.
type OpportunityAttackConditionSuite struct {
	suite.Suite
	ctx  context.Context
	bus  events.EventBus
	room spatial.Room
}

func TestOpportunityAttackConditionSuite(t *testing.T) {
	suite.Run(t, new(OpportunityAttackConditionSuite))
}

func (s *OpportunityAttackConditionSuite) SetupTest() {
	s.ctx = context.Background()
	s.bus = events.NewEventBus()

	grid := spatial.NewSquareGrid(spatial.SquareGridConfig{
		Width:  20,
		Height: 20,
	})
	s.room = spatial.NewBasicRoom(spatial.BasicRoomConfig{
		ID:   "test-room",
		Type: "dungeon",
		Grid: grid,
	})
}

// placeEntity puts an entity at the given square. Wraps PlaceEntity for the
// test's terser shape.
func (s *OpportunityAttackConditionSuite) placeEntity(id string, kind core.EntityType, x, y float64) {
	err := s.room.PlaceEntity(&oaTestEntity{id: id, entityType: kind}, spatial.Position{X: x, Y: y})
	s.Require().NoError(err)
}

// subscribeTriggers returns a slice that buffers all ReactionTriggerEvents
// published during the chain execution. The test inspects this after running
// the move chain.
func (s *OpportunityAttackConditionSuite) subscribeTriggers() *[]dnd5eEvents.ReactionTriggerEvent {
	mu := &sync.Mutex{}
	collected := &[]dnd5eEvents.ReactionTriggerEvent{}
	topic := dnd5eEvents.ReactionTriggerTopic.On(s.bus)
	_, err := topic.Subscribe(s.ctx, func(_ context.Context, e dnd5eEvents.ReactionTriggerEvent) error {
		mu.Lock()
		*collected = append(*collected, e)
		mu.Unlock()
		return nil
	})
	s.Require().NoError(err)
	return collected
}

func (s *OpportunityAttackConditionSuite) TestApplyAndRemove() {
	oa := conditions.NewOpportunityAttackCondition("fighter-1")
	s.False(oa.IsApplied())

	s.Require().NoError(oa.Apply(s.ctx, s.bus))
	s.True(oa.IsApplied())

	// Re-apply should error
	s.Error(oa.Apply(s.ctx, s.bus))

	s.Require().NoError(oa.Remove(s.ctx, s.bus))
	s.False(oa.IsApplied())
}

func (s *OpportunityAttackConditionSuite) TestPublishesTriggerWhenReadyAndLeavingReach() {
	// Fighter at (5,5), goblin moves from adjacent (5,6) to non-adjacent (5,8).
	s.placeEntity("fighter-1", "character", 5, 5)
	s.placeEntity("goblin-1", "monster", 5, 6)

	oa := conditions.NewOpportunityAttackCondition("fighter-1")
	s.Require().NoError(oa.Apply(s.ctx, s.bus))

	collected := s.subscribeTriggers()
	ctx := gamectx.WithRoom(s.ctx, s.room)
	ctx = gamectx.WithReactionReadiness(ctx, gamectx.ReactionReadinessMap{
		"fighter-1": {refs.Conditions.OpportunityAttack().String(): true},
	})

	// Run via the explicit context — re-bind suite ctx for chain run.
	c := events.NewStagedChain[*dnd5eEvents.MovementChainEvent](combat.ModifierStages)
	movements := dnd5eEvents.MovementChain.On(s.bus)
	event := &dnd5eEvents.MovementChainEvent{
		EntityID:     "goblin-1",
		EntityType:   "monster",
		FromPosition: dnd5eEvents.Position{X: 5, Y: 6},
		ToPosition:   dnd5eEvents.Position{X: 5, Y: 8},
	}
	mc, err := movements.PublishWithChain(ctx, event, c)
	s.Require().NoError(err)
	_, err = mc.Execute(ctx, event)
	s.Require().NoError(err)

	s.Require().Len(*collected, 1, "expected exactly one OA trigger event")
	got := (*collected)[0]
	s.Equal("fighter-1", got.ReactorID)
	s.Equal(refs.Conditions.OpportunityAttack().String(), got.ConditionRef)
	s.Equal(dnd5eEvents.TriggerKindMovementOA, got.TriggerKind)
	s.Equal("goblin-1", got.SourceEntity)
}

func (s *OpportunityAttackConditionSuite) TestNoTriggerWhenReadinessOff() {
	s.placeEntity("fighter-1", "character", 5, 5)
	s.placeEntity("goblin-1", "monster", 5, 6)

	oa := conditions.NewOpportunityAttackCondition("fighter-1")
	s.Require().NoError(oa.Apply(s.ctx, s.bus))

	collected := s.subscribeTriggers()
	ctx := gamectx.WithRoom(s.ctx, s.room)
	// readiness map present but OA flag false
	ctx = gamectx.WithReactionReadiness(ctx, gamectx.ReactionReadinessMap{
		"fighter-1": {refs.Conditions.OpportunityAttack().String(): false},
	})

	c := events.NewStagedChain[*dnd5eEvents.MovementChainEvent](combat.ModifierStages)
	movements := dnd5eEvents.MovementChain.On(s.bus)
	event := &dnd5eEvents.MovementChainEvent{
		EntityID:     "goblin-1",
		FromPosition: dnd5eEvents.Position{X: 5, Y: 6},
		ToPosition:   dnd5eEvents.Position{X: 5, Y: 8},
	}
	mc, err := movements.PublishWithChain(ctx, event, c)
	s.Require().NoError(err)
	_, err = mc.Execute(ctx, event)
	s.Require().NoError(err)

	s.Empty(*collected, "no trigger expected when readiness is off")
}

func (s *OpportunityAttackConditionSuite) TestNoTriggerWhenOAPrevented() {
	// Mover added Disengaging-style OA prevention to the event before the
	// OA condition runs.
	s.placeEntity("fighter-1", "character", 5, 5)
	s.placeEntity("goblin-1", "monster", 5, 6)

	oa := conditions.NewOpportunityAttackCondition("fighter-1")
	s.Require().NoError(oa.Apply(s.ctx, s.bus))

	collected := s.subscribeTriggers()
	ctx := gamectx.WithRoom(s.ctx, s.room)
	ctx = gamectx.WithReactionReadiness(ctx, gamectx.ReactionReadinessMap{
		"fighter-1": {refs.Conditions.OpportunityAttack().String(): true},
	})

	event := &dnd5eEvents.MovementChainEvent{
		EntityID:     "goblin-1",
		FromPosition: dnd5eEvents.Position{X: 5, Y: 6},
		ToPosition:   dnd5eEvents.Position{X: 5, Y: 8},
		OAPreventionSources: []dnd5eEvents.MovementModifierSource{
			{Name: "Disengaging", SourceType: "condition", EntityID: "goblin-1"},
		},
	}
	c := events.NewStagedChain[*dnd5eEvents.MovementChainEvent](combat.ModifierStages)
	movements := dnd5eEvents.MovementChain.On(s.bus)
	mc, err := movements.PublishWithChain(ctx, event, c)
	s.Require().NoError(err)
	_, err = mc.Execute(ctx, event)
	s.Require().NoError(err)

	s.Empty(*collected, "no trigger expected when OA is prevented")
}

func (s *OpportunityAttackConditionSuite) TestNoTriggerOnSelfMovement() {
	s.placeEntity("fighter-1", "character", 5, 5)

	oa := conditions.NewOpportunityAttackCondition("fighter-1")
	s.Require().NoError(oa.Apply(s.ctx, s.bus))

	collected := s.subscribeTriggers()
	ctx := gamectx.WithRoom(s.ctx, s.room)
	ctx = gamectx.WithReactionReadiness(ctx, gamectx.ReactionReadinessMap{
		"fighter-1": {refs.Conditions.OpportunityAttack().String(): true},
	})

	event := &dnd5eEvents.MovementChainEvent{
		EntityID:     "fighter-1", // self moving
		FromPosition: dnd5eEvents.Position{X: 5, Y: 5},
		ToPosition:   dnd5eEvents.Position{X: 5, Y: 8},
	}
	c := events.NewStagedChain[*dnd5eEvents.MovementChainEvent](combat.ModifierStages)
	movements := dnd5eEvents.MovementChain.On(s.bus)
	mc, err := movements.PublishWithChain(ctx, event, c)
	s.Require().NoError(err)
	_, err = mc.Execute(ctx, event)
	s.Require().NoError(err)

	s.Empty(*collected, "no trigger expected on self movement")
}

func (s *OpportunityAttackConditionSuite) TestNoTriggerWhenStillInReach() {
	// Goblin moves from (5,6) to (6,6) — both within fighter's reach at (5,5).
	s.placeEntity("fighter-1", "character", 5, 5)
	s.placeEntity("goblin-1", "monster", 5, 6)

	oa := conditions.NewOpportunityAttackCondition("fighter-1")
	s.Require().NoError(oa.Apply(s.ctx, s.bus))

	collected := s.subscribeTriggers()
	ctx := gamectx.WithRoom(s.ctx, s.room)
	ctx = gamectx.WithReactionReadiness(ctx, gamectx.ReactionReadinessMap{
		"fighter-1": {refs.Conditions.OpportunityAttack().String(): true},
	})

	event := &dnd5eEvents.MovementChainEvent{
		EntityID:     "goblin-1",
		FromPosition: dnd5eEvents.Position{X: 5, Y: 6},
		ToPosition:   dnd5eEvents.Position{X: 6, Y: 6},
	}
	c := events.NewStagedChain[*dnd5eEvents.MovementChainEvent](combat.ModifierStages)
	movements := dnd5eEvents.MovementChain.On(s.bus)
	mc, err := movements.PublishWithChain(ctx, event, c)
	s.Require().NoError(err)
	_, err = mc.Execute(ctx, event)
	s.Require().NoError(err)

	s.Empty(*collected, "no trigger expected when mover stays in reach")
}

func (s *OpportunityAttackConditionSuite) TestNoTriggerWhenMoverNeverInReach() {
	// Goblin starts (and ends) outside fighter's reach.
	s.placeEntity("fighter-1", "character", 5, 5)
	s.placeEntity("goblin-1", "monster", 10, 10)

	oa := conditions.NewOpportunityAttackCondition("fighter-1")
	s.Require().NoError(oa.Apply(s.ctx, s.bus))

	collected := s.subscribeTriggers()
	ctx := gamectx.WithRoom(s.ctx, s.room)
	ctx = gamectx.WithReactionReadiness(ctx, gamectx.ReactionReadinessMap{
		"fighter-1": {refs.Conditions.OpportunityAttack().String(): true},
	})

	event := &dnd5eEvents.MovementChainEvent{
		EntityID:     "goblin-1",
		FromPosition: dnd5eEvents.Position{X: 10, Y: 10},
		ToPosition:   dnd5eEvents.Position{X: 11, Y: 10},
	}
	c := events.NewStagedChain[*dnd5eEvents.MovementChainEvent](combat.ModifierStages)
	movements := dnd5eEvents.MovementChain.On(s.bus)
	mc, err := movements.PublishWithChain(ctx, event, c)
	s.Require().NoError(err)
	_, err = mc.Execute(ctx, event)
	s.Require().NoError(err)

	s.Empty(*collected, "no trigger expected when mover never in reach")
}

func (s *OpportunityAttackConditionSuite) TestJSONRoundTrip() {
	oa := conditions.NewOpportunityAttackCondition("fighter-7")
	raw, err := oa.ToJSON()
	s.Require().NoError(err)

	loaded, err := conditions.LoadJSON(raw)
	s.Require().NoError(err)

	roundTripped, ok := loaded.(*conditions.OpportunityAttackCondition)
	s.Require().True(ok, "loader should return *OpportunityAttackCondition")
	s.Equal("fighter-7", roundTripped.CharacterID)
}

func (s *OpportunityAttackConditionSuite) TestJSONShapeContainsRef() {
	oa := conditions.NewOpportunityAttackCondition("c-1")
	raw, err := oa.ToJSON()
	s.Require().NoError(err)

	var data conditions.OpportunityAttackConditionData
	s.Require().NoError(json.Unmarshal(raw, &data))
	s.NotNil(data.Ref)
	s.Equal(refs.Conditions.OpportunityAttack().String(), data.Ref.String())
}
