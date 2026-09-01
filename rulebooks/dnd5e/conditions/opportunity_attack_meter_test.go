// Copyright (C) 2026 Kirk Diggler
// SPDX-License-Identifier: GPL-3.0-or-later

package conditions

import (
	"context"
	"encoding/json"
	"errors"
	"sync"
	"testing"

	"github.com/stretchr/testify/require"
	"github.com/stretchr/testify/suite"

	coreCombat "github.com/KirkDiggler/rpg-toolkit/core/combat"

	"github.com/KirkDiggler/rpg-toolkit/core"
	coreResources "github.com/KirkDiggler/rpg-toolkit/core/resources"
	"github.com/KirkDiggler/rpg-toolkit/events"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/combat"
	dnd5eEvents "github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/events"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/gamectx"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/refs"
	"github.com/KirkDiggler/rpg-toolkit/tools/spatial"
)

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

// character builds a reactor that keeps an action economy, plus the keeper
// that owns its sheet. This is a player: it pays for its reaction.
func (s *OpportunityAttackMeterSuite) character(id string, reactions int) *fakeSheetKeeper {
	return s.sheetFor(&fakeConditionOwner{id: id, hasEconomy: true, reactions: reactions})
}

// monster builds a reactor that keeps NO action economy, plus the keeper that
// owns its sheet — and that keeper has no spend row, exactly as the real
// monster keeper has none. This is the purseless case, and the absence is
// where it now lives.
func (s *OpportunityAttackMeterSuite) monster(id string) *fakeSheetKeeper {
	return s.sheetFor(&fakeConditionOwner{id: id})
}

func (s *OpportunityAttackMeterSuite) sheetFor(sheet *fakeConditionOwner) *fakeSheetKeeper {
	keeper, err := keeperFor(s.ctx, s.bus, sheet)
	s.Require().NoError(err)

	return keeper
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

	keeper := s.character("fighter-1", 1)
	oa := NewOpportunityAttackCondition("fighter-1")
	s.Require().NoError(oa.Apply(s.ctx, s.bus))

	collected := s.triggers()
	ctx := castOf(s.readyCtx("fighter-1"), keeper.sheet)

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

	keeper := s.character("fighter-1", 1)
	oa := NewOpportunityAttackCondition("fighter-1")
	s.Require().NoError(oa.Apply(s.ctx, s.bus))

	collected := s.triggers()
	ctx := castOf(s.readyCtx("fighter-1"), keeper.sheet)

	s.walkAway(ctx, "wolf-1", spatial.Position{X: 5, Y: 6}, spatial.Position{X: 5, Y: 8})
	s.Require().Len(*collected, 1)
	s.Require().True(oa.UsedThisTurn)

	dirtiedBefore := keeper.dirtied
	s.Require().NoError(dnd5eEvents.TurnStartTopic.On(s.bus).Publish(
		ctx, dnd5eEvents.TurnStartEvent{SubjectID: "fighter-1", Round: 2}))

	s.False(oa.UsedThisTurn, "the reactor's own turn start refreshes the reaction")
	s.Greater(keeper.dirtied, dirtiedBefore, "a refreshed meter must be persisted")

	// The SLOT refreshes too, and not from here: a character's action economy
	// is reseeded by Character.StartTurn, which grants one reaction per turn.
	// Two meters, two owners — the condition refreshes the flag it keeps, and
	// the sheet refreshes the slot it keeps. Done by hand because this package
	// cannot import character, and stated rather than sidestepped with a
	// reactor that has no economy: a fighter has one.
	keeper.sheet.reactions = 1

	s.place("wolf-2", "monster", 4, 5)
	s.walkAway(ctx, "wolf-2", spatial.Position{X: 4, Y: 5}, spatial.Position{X: 1, Y: 5})
	s.Len(*collected, 2, "a refreshed reaction swings again")
}

// Somebody ELSE's turn beginning is exactly the window a reaction is spent in.
// Refreshing on it would make the meter meaningless.
func (s *OpportunityAttackMeterSuite) TestAnotherMembersTurnStartDoesNotRefreshIt() {
	s.place("fighter-1", "character", 5, 5)
	s.place("wolf-1", "monster", 5, 6)

	keeper := s.character("fighter-1", 1)
	oa := NewOpportunityAttackCondition("fighter-1")
	s.Require().NoError(oa.Apply(s.ctx, s.bus))

	collected := s.triggers()
	ctx := castOf(s.readyCtx("fighter-1"), keeper.sheet)

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

	keeper := s.character("fighter-1", 1)
	oa := NewOpportunityAttackCondition("fighter-1")
	s.Require().NoError(oa.Apply(s.ctx, s.bus))

	collected := s.triggers()
	s.walkAway(castOf(s.readyCtx("fighter-1"), keeper.sheet), "wolf-1",
		spatial.Position{X: 5, Y: 6}, spatial.Position{X: 5, Y: 8})

	s.Require().Len(*collected, 1)
	s.Equal(0, keeper.sheet.reactions, "the reaction slot is spent, not merely flagged")
	s.Equal([]coreCombat.ActionType{coreCombat.ActionReaction}, keeper.spent)
}

// A fighter who already spent their reaction on Protection has none left for
// an opportunity attack. Without the purse gate the flag alone would let them
// do both.
func (s *OpportunityAttackMeterSuite) TestACharacterWithNoReactionLeftDoesNotSwing() {
	s.place("fighter-1", "character", 5, 5)
	s.place("wolf-1", "monster", 5, 6)

	keeper := s.character("fighter-1", 0)
	oa := NewOpportunityAttackCondition("fighter-1")
	s.Require().NoError(oa.Apply(s.ctx, s.bus))

	collected := s.triggers()
	s.walkAway(castOf(s.readyCtx("fighter-1"), keeper.sheet), "wolf-1",
		spatial.Position{X: 5, Y: 6}, spatial.Position{X: 5, Y: 8})

	s.Empty(*collected, "a spent reaction cannot be spent again")
	s.Empty(keeper.spent, "and nothing was billed for a swing that did not happen")
	s.False(oa.UsedThisTurn, "a refused reaction must not consume the flag either")
}

// The asymmetry IS the rule. A monster carries no action economy at all, so
// requiring a purse would refuse it a reaction it is entitled to, and having
// no flag would give it an unlimited one.
func (s *OpportunityAttackMeterSuite) TestAMonsterReactsWithNoPurseAndIsStillMetered() {
	s.place("wolf-1", "monster", 5, 5)
	s.place("rogue-1", "character", 5, 6)
	s.place("rogue-2", "character", 4, 5)

	keeper := s.monster("wolf-1")
	oa := NewOpportunityAttackCondition("wolf-1")
	s.Require().NoError(oa.Apply(s.ctx, s.bus))

	collected := s.triggers()
	ctx := castOf(s.readyCtx("wolf-1"), keeper.sheet)

	s.walkAway(ctx, "rogue-1", spatial.Position{X: 5, Y: 6}, spatial.Position{X: 5, Y: 8})
	s.Require().Len(*collected, 1, "a monster with no economy still gets its reaction")

	s.walkAway(ctx, "rogue-2", spatial.Position{X: 4, Y: 5}, spatial.Position{X: 1, Y: 5})
	s.Len(*collected, 1, "and is still held to one per turn by the flag alone")
	s.Empty(keeper.spent, "the bill went out and its keeper has no row to pay it")
	s.Positive(keeper.dirtied, "but the meter it DOES keep is still written down")
}

// A reactor nobody can look up does not react at all, and this is a fold with
// no cast installed — the one state the old owner handle could not produce.
//
// It is not the monster case. A monster IS in the cast and answers for itself
// (see the test above); this is a fold assembled without the cast that
// resolution's one door installs on every path, so there is no sheet to ask.
// Reacting here would hand a free reaction to any character whose cast went
// missing, which is the silently-absent-handle failure this migration removes.
func (s *OpportunityAttackMeterSuite) TestAReactorNobodyCanLookUpDoesNotReact() {
	s.place("wolf-1", "monster", 5, 5)
	s.place("rogue-1", "character", 5, 6)

	oa := NewOpportunityAttackCondition("wolf-1")
	s.Require().NoError(oa.Apply(s.ctx, s.bus))

	collected := s.triggers()

	s.walkAway(s.readyCtx("wolf-1"), "rogue-1",
		spatial.Position{X: 5, Y: 6}, spatial.Position{X: 5, Y: 8})

	s.Empty(*collected, "no sheet to ask is not a yes")
	s.False(oa.UsedThisTurn, "and a reaction that never happened must not burn the flag")
}

// The meter is persisted for SneakAttackData's stated reason: every call
// reconstructs the condition from JSON, so a runtime-only flag resets on each
// RPC and meters nothing at all.
func (s *OpportunityAttackMeterSuite) TestTheMeterSurvivesTheJSONRoundTrip() {
	s.place("fighter-1", "character", 5, 5)
	s.place("wolf-1", "monster", 5, 6)

	keeper := s.character("fighter-1", 1)
	oa := NewOpportunityAttackCondition("fighter-1")
	s.Require().NoError(oa.Apply(s.ctx, s.bus))
	s.walkAway(castOf(s.readyCtx("fighter-1"), keeper.sheet), "wolf-1",
		spatial.Position{X: 5, Y: 6}, spatial.Position{X: 5, Y: 8})
	s.Require().True(oa.UsedThisTurn)

	raw, err := oa.ToJSON()
	s.Require().NoError(err)

	var data OpportunityAttackConditionData
	s.Require().NoError(json.Unmarshal(raw, &data))
	s.True(data.UsedThisTurn, "a spent reaction that is not written down is not spent")

	reloaded, err := LoadJSON(raw)
	s.Require().NoError(err)
	restored, ok := reloaded.(*OpportunityAttackCondition)
	s.Require().True(ok, "the loader must route this ref back to an OA condition")
	s.True(restored.UsedThisTurn, "the meter must survive the load, not just the save")
}

func TestOpportunityAttackMeterResetsOnLongRest(t *testing.T) {
	ctx := context.Background()
	bus := events.NewEventBus()
	raw := json.RawMessage(`{
		"ref":{"module":"dnd5e","type":"conditions","id":"opportunity_attack"},
		"member_id":"fighter-1","used_this_turn":true
	}`)

	loaded, err := LoadJSON(raw)
	require.NoError(t, err)
	oa, ok := loaded.(*OpportunityAttackCondition)
	require.True(t, ok)
	require.True(t, oa.UsedThisTurn)

	var changed []dnd5eEvents.ConditionStateChangedEvent
	_, err = dnd5eEvents.ConditionStateChangedTopic.On(bus).Subscribe(ctx,
		func(_ context.Context, event dnd5eEvents.ConditionStateChangedEvent) error {
			changed = append(changed, event)
			return nil
		})
	require.NoError(t, err)
	require.NoError(t, oa.Apply(ctx, bus))

	require.NoError(t, dnd5eEvents.RestTopic.On(bus).Publish(ctx, dnd5eEvents.RestEvent{
		RestType:    coreResources.ResetLongRest,
		CharacterID: "other-fighter",
	}))
	require.True(t, oa.UsedThisTurn, "another character's rest must not reset this meter")
	require.Empty(t, changed)

	require.NoError(t, dnd5eEvents.RestTopic.On(bus).Publish(ctx, dnd5eEvents.RestEvent{
		RestType:    coreResources.ResetShortRest,
		CharacterID: "fighter-1",
	}))
	require.True(t, oa.UsedThisTurn, "a short rest must not reset this meter")
	require.Empty(t, changed)

	require.NoError(t, dnd5eEvents.RestTopic.On(bus).Publish(ctx, dnd5eEvents.RestEvent{
		RestType:    coreResources.ResetLongRest,
		CharacterID: "fighter-1",
	}))
	require.False(t, oa.UsedThisTurn)
	require.Len(t, changed, 1)
	require.Equal(t, "fighter-1", changed[0].MemberID)
	require.Equal(t, refs.Conditions.OpportunityAttack().String(), changed[0].ConditionRef.String())

	serialized, err := oa.ToJSON()
	require.NoError(t, err)
	var data OpportunityAttackConditionData
	require.NoError(t, json.Unmarshal(serialized, &data))
	require.False(t, data.UsedThisTurn)
	require.True(t, oa.IsApplied(), "long rest resets but retains opportunity attack")

	require.NoError(t, dnd5eEvents.RestTopic.On(bus).Publish(ctx, dnd5eEvents.RestEvent{
		RestType:    coreResources.ResetLongRest,
		CharacterID: "fighter-1",
	}))
	require.Len(t, changed, 1, "an already-clear meter must not publish a state change")

	oa.UsedThisTurn = true
	require.NoError(t, oa.Remove(ctx, bus))
	require.NoError(t, dnd5eEvents.RestTopic.On(bus).Publish(ctx, dnd5eEvents.RestEvent{
		RestType:    coreResources.ResetLongRest,
		CharacterID: "fighter-1",
	}))
	require.True(t, oa.UsedThisTurn, "a removed condition must no longer hear rests")
	require.Len(t, changed, 1)
}

func TestOpportunityAttackMeterLongRestSubscribeFailureRollsBack(t *testing.T) {
	ctx := context.Background()
	bus := &failAfterBus{EventBus: events.NewEventBus(), allow: 2}
	oa := NewOpportunityAttackCondition("fighter-1")

	err := oa.Apply(ctx, bus)
	require.ErrorIs(t, err, errRefusedSubscribe)
	require.False(t, oa.IsApplied())
	require.Len(t, bus.unsubscribed, 2, "both earlier subscriptions must be rolled back")

	retryBus := events.NewEventBus()
	require.NoError(t, oa.Apply(ctx, retryBus), "a rolled-back condition must be reusable")
	require.NoError(t, oa.Remove(ctx, retryBus))
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

	oa := NewOpportunityAttackCondition("fighter-1")
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
