// Copyright (C) 2026 Kirk Diggler
// SPDX-License-Identifier: GPL-3.0-or-later

package conditions

import (
	"context"
	"errors"
	"sync"
	"testing"

	"github.com/stretchr/testify/suite"

	coreCombat "github.com/KirkDiggler/rpg-toolkit/core/combat"

	"github.com/KirkDiggler/rpg-toolkit/core"
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

// OpportunityAttackMeterSuite pins the once-per-turn meter, which since Kirk's
// 2026-09-11 ruling is ONE meter kept by the reactor's own keeper: a
// character's reaction slot, a monster's single reaction. The condition keeps
// none of its own.
//
// The condition asks CanReact before it offers and publishes a spend once a
// swing has actually run. That is the whole of its part, and it is the same
// part for both kinds of creature — which is also what keeps this and
// Protection fighting style mutually exclusive, since both spend the one
// reaction and the second to ask finds it gone.
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

// monster builds a reactor that keeps no action economy, plus the keeper that
// owns its sheet — and that keeper meters the one reaction a monster has,
// exactly as the real monster keeper does.
func (s *OpportunityAttackMeterSuite) monster(id string) *fakeSheetKeeper {
	return s.sheetFor(&fakeConditionOwner{id: id})
}

func (s *OpportunityAttackMeterSuite) sheetFor(sheet *fakeConditionOwner) *fakeSheetKeeper {
	keeper, err := keeperFor(s.ctx, s.bus, sheet)
	s.Require().NoError(err)

	return keeper
}

// bills collects every spend request published on the bus, so a test can count
// what the condition ASKED for rather than what a keeper happened to pay. The
// two differ now that a debit can be floored, and "spends exactly once" is a
// claim about the asking.
func (s *OpportunityAttackMeterSuite) bills() *[]dnd5eEvents.SpendRequestedEvent {
	mu := &sync.Mutex{}
	collected := &[]dnd5eEvents.SpendRequestedEvent{}
	_, err := dnd5eEvents.SpendRequestedTopic.On(s.bus).Subscribe(
		s.ctx, func(_ context.Context, e dnd5eEvents.SpendRequestedEvent) error {
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

// taken publishes what the movement machine publishes once a reaction has
// actually RUN. The trigger is an offer and costs nothing; this is the bill.
// Every scene below that expects a spent reaction has to take one, because
// that is now the only thing that spends it.
func (s *OpportunityAttackMeterSuite) taken(ctx context.Context, reactor, against string) {
	s.Require().NoError(dnd5eEvents.ReactionTakenTopic.On(s.bus).Publish(ctx,
		dnd5eEvents.ReactionTakenEvent{
			ReactorID:    reactor,
			ConditionRef: refs.Conditions.OpportunityAttack().String(),
			TriggerKind:  dnd5eEvents.TriggerKindMovementOA,
			SourceEntity: against,
		}))
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
	s.taken(ctx, "fighter-1", "wolf-1")
	s.walkAway(ctx, "wolf-2", spatial.Position{X: 4, Y: 5}, spatial.Position{X: 1, Y: 5})

	s.Require().Len(*collected, 1, "the second fleeing enemy must not draw a second reaction")
	s.Equal("wolf-1", (*collected)[0].SourceEntity)
	s.Equal(0, keeper.sheet.reactions, "and the slot that stopped it is the fighter's own")
}

// TURN START, not turn end. A reaction is spent on somebody else's turn, so a
// meter cleared at the end of its holder's turn would be full again for the
// entire window it governs. 2014 PHB: "you regain a spent reaction at the
// start of each of your turns."
//
// Read from the CONDITION's side: what it can see of the refresh is that the
// gate it asks answers yes again. The keeper that owns the clearing is the
// monster's, and its own package proves the row.
func (s *OpportunityAttackMeterSuite) TestARefreshedReactionSwingsAgain() {
	s.place("wolf-1", "monster", 5, 5)
	s.place("rogue-1", "character", 5, 6)
	s.place("rogue-2", "character", 4, 5)

	keeper := s.monster("wolf-1")
	oa := NewOpportunityAttackCondition("wolf-1")
	s.Require().NoError(oa.Apply(s.ctx, s.bus))

	collected := s.triggers()
	ctx := castOf(s.readyCtx("wolf-1"), keeper.sheet)

	s.walkAway(ctx, "rogue-1", spatial.Position{X: 5, Y: 6}, spatial.Position{X: 5, Y: 8})
	s.Require().Len(*collected, 1)
	s.taken(ctx, "wolf-1", "rogue-1")
	s.Require().True(keeper.sheet.reactionSpent)

	dirtiedBefore := keeper.dirtied
	s.Require().NoError(dnd5eEvents.TurnStartTopic.On(s.bus).Publish(
		ctx, dnd5eEvents.TurnStartEvent{SubjectID: "wolf-1", Round: 2}))

	s.False(keeper.sheet.reactionSpent, "the reactor's own turn start refreshes the reaction")
	s.Greater(keeper.dirtied, dirtiedBefore, "a refreshed meter must be persisted")

	s.walkAway(ctx, "rogue-2", spatial.Position{X: 4, Y: 5}, spatial.Position{X: 1, Y: 5})
	s.Len(*collected, 2, "a refreshed reaction swings again")
}

// Somebody ELSE's turn beginning is exactly the window a reaction is spent in.
// Refreshing on it would make the meter meaningless.
func (s *OpportunityAttackMeterSuite) TestAnotherMembersTurnStartDoesNotRefreshIt() {
	s.place("wolf-1", "monster", 5, 5)
	s.place("rogue-1", "character", 5, 6)
	s.place("rogue-2", "character", 4, 5)

	keeper := s.monster("wolf-1")
	oa := NewOpportunityAttackCondition("wolf-1")
	s.Require().NoError(oa.Apply(s.ctx, s.bus))

	collected := s.triggers()
	ctx := castOf(s.readyCtx("wolf-1"), keeper.sheet)

	s.walkAway(ctx, "rogue-1", spatial.Position{X: 5, Y: 6}, spatial.Position{X: 5, Y: 8})
	s.Require().Len(*collected, 1)
	s.taken(ctx, "wolf-1", "rogue-1")

	s.Require().NoError(dnd5eEvents.TurnStartTopic.On(s.bus).Publish(
		ctx, dnd5eEvents.TurnStartEvent{SubjectID: "rogue-2", Round: 1}))

	s.True(keeper.sheet.reactionSpent, "another member's turn start must not refresh this reactor")
	s.walkAway(ctx, "rogue-2", spatial.Position{X: 4, Y: 5}, spatial.Position{X: 1, Y: 5})
	s.Len(*collected, 1, "so no second swing is offered")
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
	ctx := castOf(s.readyCtx("fighter-1"), keeper.sheet)
	s.walkAway(ctx, "wolf-1", spatial.Position{X: 5, Y: 6}, spatial.Position{X: 5, Y: 8})

	s.Require().Len(*collected, 1)
	s.Require().Empty(keeper.spent, "the OFFER is free; only a swing is billed")
	s.taken(ctx, "fighter-1", "wolf-1")

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
}

// A monster is metered by its own keeper, exactly as a character is by theirs.
//
// This used to be the asymmetry: a monster kept no economy, so the condition's
// own once-per-turn flag was the only thing holding it to one swing. Kirk
// reversed that on 2026-09-11, and the whole of the condition's part is now
// the same for both kinds — ask CanReact, publish the bill, let the keeper
// meter it.
func (s *OpportunityAttackMeterSuite) TestAMonsterIsMeteredByItsKeeper() {
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
	s.taken(ctx, "wolf-1", "rogue-1")

	s.Require().True(keeper.sheet.reactionSpent, "the bill landed on the sheet that has to pay it")

	s.walkAway(ctx, "rogue-2", spatial.Position{X: 4, Y: 5}, spatial.Position{X: 1, Y: 5})
	s.Len(*collected, 1, "and a monster that has swung is held to one per turn")
	s.Equal([]coreCombat.ActionType{coreCombat.ActionReaction}, keeper.spent)
	s.Positive(keeper.dirtied, "a spent reaction that is not written down is not spent")
}

// THE WHOLE REASON THE METER MOVED. Dissonant Whispers bills a monster's
// reaction to make it flee, and it never says a word to the opportunity
// attack — so a flag living on this condition cannot see the spend, and the
// wolf that just ran for its life would still swing as the rogue walks off.
//
// The gate is CanReact, the meter is the keeper's, and a reaction spent by
// anything at all is a reaction that is gone.
func (s *OpportunityAttackMeterSuite) TestAReactionSpentElsewhereIsStillSpent() {
	s.place("wolf-1", "monster", 5, 5)
	s.place("rogue-1", "character", 5, 6)

	keeper := s.monster("wolf-1")
	oa := NewOpportunityAttackCondition("wolf-1")
	s.Require().NoError(oa.Apply(s.ctx, s.bus))

	collected := s.triggers()
	ctx := castOf(s.readyCtx("wolf-1"), keeper.sheet)

	// A spell spends it. Not a reaction taken, not this condition's business:
	// just the bill, from somewhere that is not the opportunity attack. Who
	// sent it is attribution and nothing here reads it.
	s.Require().NoError(dnd5eEvents.SpendRequestedTopic.On(s.bus).Publish(ctx,
		dnd5eEvents.SpendRequestedEvent{
			MemberID:   "wolf-1",
			ActionType: coreCombat.ActionReaction,
			Amount:     1,
		}))
	s.Require().False(keeper.sheet.CanReact())

	s.walkAway(ctx, "rogue-1", spatial.Position{X: 5, Y: 6}, spatial.Position{X: 5, Y: 8})

	s.Empty(*collected, "a creature with no reaction left is offered no swing")
}

// The same question from the character's side, and it has always answered
// correctly: the slot is the meter, and something else spending it leaves
// nothing for an opportunity attack.
func (s *OpportunityAttackMeterSuite) TestACharactersReactionSpentElsewhereIsStillSpent() {
	s.place("fighter-1", "character", 5, 5)
	s.place("wolf-1", "monster", 5, 6)

	keeper := s.character("fighter-1", 1)
	oa := NewOpportunityAttackCondition("fighter-1")
	s.Require().NoError(oa.Apply(s.ctx, s.bus))

	collected := s.triggers()
	ctx := castOf(s.readyCtx("fighter-1"), keeper.sheet)

	s.Require().NoError(dnd5eEvents.SpendRequestedTopic.On(s.bus).Publish(ctx,
		dnd5eEvents.SpendRequestedEvent{
			MemberID:   "fighter-1",
			ActionType: coreCombat.ActionReaction,
			Amount:     1,
		}))
	s.Require().False(keeper.sheet.CanReact())

	s.walkAway(ctx, "wolf-1", spatial.Position{X: 5, Y: 6}, spatial.Position{X: 5, Y: 8})

	s.Empty(*collected, "a creature with no reaction left is offered no swing")
}

// THE OFFER IS FREE. This is R1 (rpg-project#392): the condition used to set
// its meter and bill the economy the instant the trigger published, before
// anything had decided whether a swing happened at all. So a friend walking
// past a fighter cost the fighter their reaction — the movement machine's
// hostility gate answers too late — and a player who is asked and holds paid
// for a swing they refused.
//
// The proof is the second enemy: the fighter's reaction is still there for
// them, because the first one was never taken.
func (s *OpportunityAttackMeterSuite) TestATriggerNobodyTakesCostsNothing() {
	s.place("fighter-1", "character", 5, 5)
	s.place("ally-1", "character", 5, 6)
	s.place("wolf-1", "monster", 4, 5)

	keeper := s.character("fighter-1", 1)
	oa := NewOpportunityAttackCondition("fighter-1")
	s.Require().NoError(oa.Apply(s.ctx, s.bus))

	collected := s.triggers()
	ctx := castOf(s.readyCtx("fighter-1"), keeper.sheet)

	s.walkAway(ctx, "ally-1", spatial.Position{X: 5, Y: 6}, spatial.Position{X: 5, Y: 8})

	s.Require().Len(*collected, 1, "the offer still goes out; who may take it is not this condition's call")
	s.Empty(keeper.spent, "a trigger nobody took must not bill the economy")
	s.Equal(1, keeper.sheet.reactions, "the reaction is still in hand")

	s.walkAway(ctx, "wolf-1", spatial.Position{X: 4, Y: 5}, spatial.Position{X: 1, Y: 5})
	s.Len(*collected, 2, "so the next mover is still offered a swing")
}

// ONE SWING, ONE BILL. Counted on the bus rather than on the keeper, because
// the bill is what this condition controls and the debit is not: a keeper
// floors what it cannot pay, so a second request would be invisible on the
// sheet and perfectly visible to any other subscriber.
func (s *OpportunityAttackMeterSuite) TestATakenReactionSpendsExactlyOnce() {
	s.place("fighter-1", "character", 5, 5)
	s.place("wolf-1", "monster", 5, 6)

	keeper := s.character("fighter-1", 1)
	oa := NewOpportunityAttackCondition("fighter-1")
	s.Require().NoError(oa.Apply(s.ctx, s.bus))

	collected := s.triggers()
	billed := s.bills()
	ctx := castOf(s.readyCtx("fighter-1"), keeper.sheet)
	s.walkAway(ctx, "wolf-1", spatial.Position{X: 5, Y: 6}, spatial.Position{X: 5, Y: 8})
	s.Require().Len(*collected, 1)

	s.taken(ctx, "fighter-1", "wolf-1")

	s.Require().Len(*billed, 1, "one swing asks the economy once")
	s.Equal(coreCombat.ActionReaction, (*billed)[0].ActionType)
	s.Equal(refs.Conditions.OpportunityAttack().String(), (*billed)[0].SourceRef.String(),
		"and says what is asking, so a log can name it")
	s.Equal([]coreCombat.ActionType{coreCombat.ActionReaction}, keeper.spent)
	s.Equal(0, keeper.sheet.reactions)
}

// A DUPLICATE EVENT IS NOT A SECOND REACTION, and what makes that true is the
// meter rather than anything this condition remembers.
//
// The flag that used to dedup here is gone. A second taken event for a reactor
// who has had no turn since bills again — the condition has no memory of the
// first — and the ledger's floor is what makes that harmless: you cannot spend
// a reaction you do not have.
func (s *OpportunityAttackMeterSuite) TestADuplicateTakenEventCannotSpendPastEmpty() {
	s.place("fighter-1", "character", 5, 5)
	s.place("wolf-1", "monster", 5, 6)

	keeper := s.character("fighter-1", 1)
	oa := NewOpportunityAttackCondition("fighter-1")
	s.Require().NoError(oa.Apply(s.ctx, s.bus))

	ctx := castOf(s.readyCtx("fighter-1"), keeper.sheet)
	s.walkAway(ctx, "wolf-1", spatial.Position{X: 5, Y: 6}, spatial.Position{X: 5, Y: 8})

	s.taken(ctx, "fighter-1", "wolf-1")
	s.taken(ctx, "fighter-1", "wolf-1")

	s.Equal([]coreCombat.ActionType{coreCombat.ActionReaction}, keeper.spent,
		"the slot is debited once, not once per event")
	s.Equal(0, keeper.sheet.reactions, "and never below empty")
	s.False(keeper.sheet.CanReact())
}

// One bus carries every combatant's conditions, so a taken event names its
// reactor and its ref for the same reason the trigger does. Somebody else's
// swing, and this member's OTHER reaction, must both leave this meter alone.
func (s *OpportunityAttackMeterSuite) TestATakenReactionThatIsNotMineSpendsNothing() {
	s.place("fighter-1", "character", 5, 5)
	s.place("wolf-1", "monster", 5, 6)

	keeper := s.character("fighter-1", 1)
	oa := NewOpportunityAttackCondition("fighter-1")
	s.Require().NoError(oa.Apply(s.ctx, s.bus))

	ctx := castOf(s.readyCtx("fighter-1"), keeper.sheet)
	s.walkAway(ctx, "wolf-1", spatial.Position{X: 5, Y: 6}, spatial.Position{X: 5, Y: 8})

	// Another member's opportunity attack.
	s.taken(ctx, "fighter-2", "wolf-1")
	s.Empty(keeper.spent, "another reactor's swing is not this reactor's bill")

	// This member, a different reaction.
	s.Require().NoError(dnd5eEvents.ReactionTakenTopic.On(s.bus).Publish(ctx,
		dnd5eEvents.ReactionTakenEvent{
			ReactorID:    "fighter-1",
			ConditionRef: refs.Spells.Shield().String(),
			TriggerKind:  dnd5eEvents.TriggerKindPostHit,
			SourceEntity: "wolf-1",
		}))
	s.Empty(keeper.spent, "another condition's reaction must not move the OA meter")
	s.Equal(1, keeper.sheet.reactions)
}

// A removed condition no longer hears the bill, the same way it no longer
// hears rests. An orphaned handler that kept spending would debit a member
// whose condition is gone.
func (s *OpportunityAttackMeterSuite) TestARemovedConditionIsNotBilled() {
	s.place("fighter-1", "character", 5, 5)
	s.place("wolf-1", "monster", 5, 6)

	keeper := s.character("fighter-1", 1)
	oa := NewOpportunityAttackCondition("fighter-1")
	s.Require().NoError(oa.Apply(s.ctx, s.bus))
	ctx := castOf(s.readyCtx("fighter-1"), keeper.sheet)
	s.walkAway(ctx, "wolf-1", spatial.Position{X: 5, Y: 6}, spatial.Position{X: 5, Y: 8})

	s.Require().NoError(oa.Remove(s.ctx, s.bus))
	s.taken(ctx, "fighter-1", "wolf-1")

	s.Empty(keeper.spent, "a removed condition must no longer hear the bill")
	s.Equal(1, keeper.sheet.reactions)
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
}

// A half-applied condition is worse than an unapplied one. Nil-ing the bus with
// a live subscription still recorded leaves IsApplied reporting false, Remove
// early-returning and unsubscribing nothing, and the orphaned movement handler
// still receiving events on a bus the condition no longer admits to holding.
func (s *OpportunityAttackMeterSuite) TestAFailedSecondSubscribeRollsTheFirstOneBack() {
	s.place("fighter-1", "character", 5, 5)
	s.place("wolf-1", "monster", 5, 6)

	// Allow the MovementChain subscribe, refuse the ReactionTaken one after
	// it. Those are the two subscriptions this condition has left: turn start
	// and rest went with the flag they cleared.
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

	// And the rollback leaves it appliable, which a condition that kept a live
	// subscription and a nil bus would not be.
	retryBus := events.NewEventBus()
	s.Require().NoError(oa.Apply(s.ctx, retryBus), "a rolled-back condition must be reusable")
	s.Require().NoError(oa.Remove(s.ctx, retryBus))
}
