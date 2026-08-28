// Copyright (C) 2026 Kirk Diggler
// SPDX-License-Identifier: GPL-3.0-or-later

package conditions_test

import (
	"context"
	"encoding/json"
	"errors"
	"sync"
	"testing"

	"github.com/stretchr/testify/suite"

	coreCombat "github.com/KirkDiggler/rpg-toolkit/core/combat"
	coreResources "github.com/KirkDiggler/rpg-toolkit/core/resources"

	"github.com/KirkDiggler/rpg-toolkit/core"
	"github.com/KirkDiggler/rpg-toolkit/events"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/combat"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/conditions"
	dnd5eEvents "github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/events"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/gamectx"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/refs"
	"github.com/KirkDiggler/rpg-toolkit/tools/spatial"
)

// fakeReactor stands in for the reactor's own live sheet.
//
// It can be built WITHOUT a purse (withPurse false), which is the monster
// case and not a degenerate one: monsters carry no action economy in this
// rulebook at all, so "there is no reaction slot to read" is the production
// shape rather than a gap in the fake. SetOwner asserts the two halves
// independently, so an owner that only marks dirty is a real configuration.
type fakeReactor struct {
	withPurse bool
	reactions int
	dirtied   int
	spent     []coreCombat.ActionType
}

func (f *fakeReactor) MarkDirty() { f.dirtied++ }

func (f *fakeReactor) InCombat() bool { return true }

func (f *fakeReactor) SlotsLeft(slot coreCombat.ActionType) int {
	if slot == coreCombat.ActionReaction {
		return f.reactions
	}
	return 0
}

func (f *fakeReactor) SpendSlots(slot coreCombat.ActionType, n int) {
	if slot == coreCombat.ActionReaction {
		f.reactions -= n
	}
	f.spent = append(f.spent, slot)
}

func (f *fakeReactor) CapacityLeft(_ combat.CapacityType) int       { return 0 }
func (f *fakeReactor) PoolLeft(_ coreResources.ResourceKey) int     { return 0 }
func (f *fakeReactor) SpendCapacity(_ combat.CapacityType, _ int)   {}
func (f *fakeReactor) SpendPool(_ coreResources.ResourceKey, _ int) {}
func (f *fakeReactor) BankCapacity(_ combat.CapacityType, _ int)    {}

// purseless is the monster shape: it marks dirty and nothing else, so a type
// assertion to combat.Ledger fails and the condition finds no purse.
type purseless struct{ dirtied int }

func (p *purseless) MarkDirty() { p.dirtied++ }

// failAfterBus is a real bus that refuses the Nth subscribe, and remembers
// every Unsubscribe it was asked for.
//
// A wrapper rather than a hand-rolled fake so the SUCCESSFUL subscribes are
// the genuine article: the point of the test is what happens to a real, live
// subscription when a later one fails, and a fake that never actually
// registered anything could not tell a rollback from a no-op.
type failAfterBus struct {
	events.EventBus
	allow        int
	seen         int
	unsubscribed []string
}

func (b *failAfterBus) Subscribe(ctx context.Context, topic events.Topic, handler any) (string, error) {
	b.seen++
	if b.seen > b.allow {
		return "", errRefusedSubscribe
	}
	return b.EventBus.Subscribe(ctx, topic, handler)
}

func (b *failAfterBus) Unsubscribe(ctx context.Context, id string) error {
	b.unsubscribed = append(b.unsubscribed, id)
	return b.EventBus.Unsubscribe(ctx, id)
}

var errRefusedSubscribe = errors.New("bus refused the subscription")

// oaMeterEntity implements core.Entity for room placement.
type oaMeterEntity struct {
	id   string
	kind core.EntityType
}

func (e *oaMeterEntity) GetID() string            { return e.id }
func (e *oaMeterEntity) GetType() core.EntityType { return e.kind }

// OpportunityAttackMeterSuite pins the once-per-turn meter Kirk ruled on
// 2026-08-28: "characters have to pay for it. the condition can still track it
// was used but players have a cost."
//
// The two meters have two distinct jobs and both are tested here. UsedThisTurn
// is the one EVERY reactor carries and is what makes the rule enforceable for
// monsters. The reaction slot is an additional charge only a sheet can bear,
// and is what keeps OA and Protection fighting style mutually exclusive.
type OpportunityAttackMeterSuite struct {
	suite.Suite
	ctx  context.Context
	bus  events.EventBus
	room spatial.Room
}

func TestOpportunityAttackMeterSuite(t *testing.T) {
	suite.Run(t, new(OpportunityAttackMeterSuite))
}

func (s *OpportunityAttackMeterSuite) SetupTest() {
	s.ctx = context.Background()
	s.bus = events.NewEventBus()
	s.room = spatial.NewBasicRoom(spatial.BasicRoomConfig{
		ID:   "meter-room",
		Type: "dungeon",
		Grid: spatial.NewSquareGrid(spatial.SquareGridConfig{Width: 20, Height: 20}),
	})
}

func (s *OpportunityAttackMeterSuite) place(id string, kind core.EntityType, x, y float64) {
	s.Require().NoError(s.room.PlaceEntity(&oaMeterEntity{id: id, kind: kind}, spatial.Position{X: x, Y: y}))
}

func (s *OpportunityAttackMeterSuite) triggers() *[]dnd5eEvents.ReactionTriggerEvent {
	mu := &sync.Mutex{}
	collected := &[]dnd5eEvents.ReactionTriggerEvent{}
	_, err := dnd5eEvents.ReactionTriggerTopic.On(s.bus).Subscribe(
		s.ctx, func(_ context.Context, e dnd5eEvents.ReactionTriggerEvent) error {
			mu.Lock()
			defer mu.Unlock()
			*collected = append(*collected, e)
			return nil
		})
	s.Require().NoError(err)
	return collected
}

// readyCtx is the context a live movement fold runs under: a room to read
// geometry from, and the reactor readied for OA.
func (s *OpportunityAttackMeterSuite) readyCtx(reactor string) context.Context {
	ctx := gamectx.WithRoom(s.ctx, s.room)
	return gamectx.WithReactionReadiness(ctx, gamectx.ReactionReadinessMap{
		reactor: {refs.Conditions.OpportunityAttack().String(): true},
	})
}

// walkAway folds one step that leaves the reactor's reach, which is the whole
// predicate: adjacent at from, out of reach at to.
func (s *OpportunityAttackMeterSuite) walkAway(ctx context.Context, mover string, from, to spatial.Position) {
	event := &dnd5eEvents.MovementChainEvent{
		EntityID:     mover,
		EntityType:   "monster",
		FromPosition: dnd5eEvents.Position{X: from.X, Y: from.Y},
		ToPosition:   dnd5eEvents.Position{X: to.X, Y: to.Y},
	}
	c := events.NewStagedChain[*dnd5eEvents.MovementChainEvent](combat.ModifierStages)
	mc, err := dnd5eEvents.MovementChain.On(s.bus).PublishWithChain(ctx, event, c)
	s.Require().NoError(err)
	_, err = mc.Execute(ctx, event)
	s.Require().NoError(err)
}

// A reaction is once per round. Before this the condition had no memory at
// all, so every enemy that fled past a fighter in one round drew its own
// swing — the whole party's worth of free attacks.
func (s *OpportunityAttackMeterSuite) TestASecondEnemyFleeingTheSameTurnGetsAway() {
	s.place("fighter-1", "character", 5, 5)
	s.place("wolf-1", "monster", 5, 6)
	s.place("wolf-2", "monster", 4, 5)

	oa := conditions.NewOpportunityAttackCondition("fighter-1")
	oa.SetOwner(&purseless{})
	s.Require().NoError(oa.Apply(s.ctx, s.bus))

	collected := s.triggers()
	ctx := s.readyCtx("fighter-1")

	s.walkAway(ctx, "wolf-1", spatial.Position{X: 5, Y: 6}, spatial.Position{X: 5, Y: 8})
	s.walkAway(ctx, "wolf-2", spatial.Position{X: 4, Y: 5}, spatial.Position{X: 1, Y: 5})

	s.Require().Len(*collected, 1, "the second fleeing enemy must not draw a second reaction")
	s.Equal("wolf-1", (*collected)[0].SourceEntity)
	s.True(oa.UsedThisTurn)
}

// TURN START, not turn end. A reaction is spent on somebody else's turn, so a
// meter cleared at the end of its holder's turn would be full again for the
// entire window it governs. 2014 PHB: "you regain a spent reaction at the
// start of each of your turns."
func (s *OpportunityAttackMeterSuite) TestTheReactionRefreshesAtTheReactorsOwnTurnStart() {
	s.place("fighter-1", "character", 5, 5)
	s.place("wolf-1", "monster", 5, 6)

	owner := &purseless{}
	oa := conditions.NewOpportunityAttackCondition("fighter-1")
	oa.SetOwner(owner)
	s.Require().NoError(oa.Apply(s.ctx, s.bus))

	collected := s.triggers()
	ctx := s.readyCtx("fighter-1")

	s.walkAway(ctx, "wolf-1", spatial.Position{X: 5, Y: 6}, spatial.Position{X: 5, Y: 8})
	s.Require().Len(*collected, 1)
	s.Require().True(oa.UsedThisTurn)

	dirtiedBefore := owner.dirtied
	s.Require().NoError(dnd5eEvents.TurnStartTopic.On(s.bus).Publish(
		ctx, dnd5eEvents.TurnStartEvent{SubjectID: "fighter-1", Round: 2}))

	s.False(oa.UsedThisTurn, "the reactor's own turn start refreshes the reaction")
	s.Greater(owner.dirtied, dirtiedBefore, "a refreshed meter must be persisted")

	s.place("wolf-2", "monster", 4, 5)
	s.walkAway(ctx, "wolf-2", spatial.Position{X: 4, Y: 5}, spatial.Position{X: 1, Y: 5})
	s.Len(*collected, 2, "a refreshed reaction swings again")
}

// Somebody ELSE's turn beginning is exactly the window a reaction is spent in.
// Refreshing on it would make the meter meaningless.
func (s *OpportunityAttackMeterSuite) TestAnotherMembersTurnStartDoesNotRefreshIt() {
	s.place("fighter-1", "character", 5, 5)
	s.place("wolf-1", "monster", 5, 6)

	oa := conditions.NewOpportunityAttackCondition("fighter-1")
	oa.SetOwner(&purseless{})
	s.Require().NoError(oa.Apply(s.ctx, s.bus))

	collected := s.triggers()
	ctx := s.readyCtx("fighter-1")

	s.walkAway(ctx, "wolf-1", spatial.Position{X: 5, Y: 6}, spatial.Position{X: 5, Y: 8})
	s.Require().Len(*collected, 1)

	s.Require().NoError(dnd5eEvents.TurnStartTopic.On(s.bus).Publish(
		ctx, dnd5eEvents.TurnStartEvent{SubjectID: "wolf-2", Round: 1}))
	s.True(oa.UsedThisTurn, "another member's turn start must not refresh this reactor")
}

// Kirk's ruling: characters pay. The slot is what makes OA and Protection
// fighting style mutually exclusive, which they are in the rules — both spend
// the one reaction and the second to ask finds it gone.
func (s *OpportunityAttackMeterSuite) TestACharacterPaysTheReactionSlot() {
	s.place("fighter-1", "character", 5, 5)
	s.place("wolf-1", "monster", 5, 6)

	owner := &fakeReactor{withPurse: true, reactions: 1}
	oa := conditions.NewOpportunityAttackCondition("fighter-1")
	oa.SetOwner(owner)
	s.Require().NoError(oa.Apply(s.ctx, s.bus))

	collected := s.triggers()
	s.walkAway(s.readyCtx("fighter-1"), "wolf-1",
		spatial.Position{X: 5, Y: 6}, spatial.Position{X: 5, Y: 8})

	s.Require().Len(*collected, 1)
	s.Equal(0, owner.reactions, "the reaction slot is spent, not merely flagged")
	s.Equal([]coreCombat.ActionType{coreCombat.ActionReaction}, owner.spent)
}

// A fighter who already spent their reaction on Protection has none left for
// an opportunity attack. Without the purse gate the flag alone would let them
// do both.
func (s *OpportunityAttackMeterSuite) TestACharacterWithNoReactionLeftDoesNotSwing() {
	s.place("fighter-1", "character", 5, 5)
	s.place("wolf-1", "monster", 5, 6)

	owner := &fakeReactor{withPurse: true, reactions: 0}
	oa := conditions.NewOpportunityAttackCondition("fighter-1")
	oa.SetOwner(owner)
	s.Require().NoError(oa.Apply(s.ctx, s.bus))

	collected := s.triggers()
	s.walkAway(s.readyCtx("fighter-1"), "wolf-1",
		spatial.Position{X: 5, Y: 6}, spatial.Position{X: 5, Y: 8})

	s.Empty(*collected, "a spent reaction cannot be spent again")
	s.False(oa.UsedThisTurn, "a refused reaction must not consume the flag either")
}

// The asymmetry IS the rule. A monster carries no action economy at all, so
// requiring a purse would refuse it a reaction it is entitled to, and having
// no flag would give it an unlimited one.
func (s *OpportunityAttackMeterSuite) TestAMonsterReactsWithNoPurseAndIsStillMetered() {
	s.place("wolf-1", "monster", 5, 5)
	s.place("rogue-1", "character", 5, 6)
	s.place("rogue-2", "character", 4, 5)

	oa := conditions.NewOpportunityAttackCondition("wolf-1")
	oa.SetOwner(&purseless{})
	s.Require().NoError(oa.Apply(s.ctx, s.bus))

	collected := s.triggers()
	ctx := s.readyCtx("wolf-1")

	s.walkAway(ctx, "rogue-1", spatial.Position{X: 5, Y: 6}, spatial.Position{X: 5, Y: 8})
	s.Require().Len(*collected, 1, "a monster with no economy still gets its reaction")

	s.walkAway(ctx, "rogue-2", spatial.Position{X: 4, Y: 5}, spatial.Position{X: 1, Y: 5})
	s.Len(*collected, 1, "and is still held to one per turn by the flag alone")
}

// An owner satisfying neither half is not an error. The condition meters
// itself in memory and charges nothing — the same "nothing to do" default
// every other unmet check in this package takes.
func (s *OpportunityAttackMeterSuite) TestAnOwnerlessReactorStillMetersItself() {
	s.place("wolf-1", "monster", 5, 5)
	s.place("rogue-1", "character", 5, 6)
	s.place("rogue-2", "character", 4, 5)

	oa := conditions.NewOpportunityAttackCondition("wolf-1")
	s.Require().NoError(oa.Apply(s.ctx, s.bus))

	collected := s.triggers()
	ctx := s.readyCtx("wolf-1")

	s.walkAway(ctx, "rogue-1", spatial.Position{X: 5, Y: 6}, spatial.Position{X: 5, Y: 8})
	s.walkAway(ctx, "rogue-2", spatial.Position{X: 4, Y: 5}, spatial.Position{X: 1, Y: 5})

	s.Len(*collected, 1, "no owner is not an excuse to react twice")
}

// The meter is persisted for SneakAttackData's stated reason: every call
// reconstructs the condition from JSON, so a runtime-only flag resets on each
// RPC and meters nothing at all.
func (s *OpportunityAttackMeterSuite) TestTheMeterSurvivesTheJSONRoundTrip() {
	s.place("fighter-1", "character", 5, 5)
	s.place("wolf-1", "monster", 5, 6)

	oa := conditions.NewOpportunityAttackCondition("fighter-1")
	oa.SetOwner(&purseless{})
	s.Require().NoError(oa.Apply(s.ctx, s.bus))
	s.walkAway(s.readyCtx("fighter-1"), "wolf-1",
		spatial.Position{X: 5, Y: 6}, spatial.Position{X: 5, Y: 8})
	s.Require().True(oa.UsedThisTurn)

	raw, err := oa.ToJSON()
	s.Require().NoError(err)

	var data conditions.OpportunityAttackConditionData
	s.Require().NoError(json.Unmarshal(raw, &data))
	s.True(data.UsedThisTurn, "a spent reaction that is not written down is not spent")

	reloaded, err := conditions.LoadJSON(raw)
	s.Require().NoError(err)
	restored, ok := reloaded.(*conditions.OpportunityAttackCondition)
	s.Require().True(ok, "the loader must route this ref back to an OA condition")
	s.True(restored.UsedThisTurn, "the meter must survive the load, not just the save")
}

// A half-applied condition is worse than an unapplied one. Nil-ing the bus with
// a live subscription still recorded leaves IsApplied reporting false, Remove
// early-returning and unsubscribing nothing, and the orphaned movement handler
// still receiving events on a bus the condition no longer admits to holding.
func (s *OpportunityAttackMeterSuite) TestAFailedSecondSubscribeRollsTheFirstOneBack() {
	s.place("fighter-1", "character", 5, 5)
	s.place("wolf-1", "monster", 5, 6)

	// Allow the MovementChain subscribe, refuse the TurnStart one after it.
	bus := &failAfterBus{EventBus: s.bus, allow: 1}

	oa := conditions.NewOpportunityAttackCondition("fighter-1")
	err := oa.Apply(s.ctx, bus)
	s.Require().Error(err, "a condition that could not finish applying must say so")

	s.False(oa.IsApplied(), "a half-applied condition does not report itself applied")
	s.Require().Len(bus.unsubscribed, 1, "the movement subscription must be rolled back, not orphaned")

	// The real proof: the orphaned handler is gone from the underlying bus, so
	// a qualifying walk publishes nothing.
	collected := s.triggers()
	s.walkAway(s.readyCtx("fighter-1"), "wolf-1",
		spatial.Position{X: 5, Y: 6}, spatial.Position{X: 5, Y: 8})
	s.Empty(*collected, "a rolled-back condition must not still be listening")
}
