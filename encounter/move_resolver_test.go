package encounter_test

// Wave 2.11e — MovementResolver tests.
//
// The MovementResolver is the encounter SDK's seam for delegating per-step
// movement mechanics (MovementChain execution, OA triggering) to a rulebook
// implementation. Without a resolver, Encounter.Move uses the legacy
// single-jump behavior (mutate position to path[-1], no chain). With a
// resolver, Move iterates per-step, calling resolver.ResolveStep per hex
// and draining ReactionTriggerEvents from the encounter bus.
//
// Wave 2.11e scope: NPC-OA-only (Q1=(b) per director signoff on #658).
// Player-pause branch deferred to #665.

import (
	"context"
	"testing"
	"time"

	"github.com/stretchr/testify/suite"

	toolkitcore "github.com/KirkDiggler/rpg-toolkit/core"
	tkenc "github.com/KirkDiggler/rpg-toolkit/encounter"
	encountercore "github.com/KirkDiggler/rpg-toolkit/encounter/core"
	"github.com/KirkDiggler/rpg-toolkit/encounter/events"
	dnd5events "github.com/KirkDiggler/rpg-toolkit/events"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/damage"
	dnd5eEvents "github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/events"
)

// stubMovementResolver implements MovementResolver and records ResolveStep
// calls. It can optionally publish a ReactionTriggerEvent on the encounter
// bus during ResolveStep (simulating an OA condition's onMovementChain
// subscriber). The resolver can also be configured to mark Prevented on a
// specific step index to simulate Disengage-prevention or a similar block.
type stubMovementResolver struct {
	calls []tkenc.MovementStepInput

	// preventAtCall, when non-nil, sets Prevented=true on the call at the
	// given 0-based step index. The SDK should stop the move at that step's
	// FromHex (not advance to ToHex).
	preventAtCall *int

	// preventReason is the human-readable reason set when Prevented fires.
	preventReason string

	// publishOnStep, when set, is invoked at each ResolveStep call with the
	// bus + step index so tests can simulate OA condition triggers firing
	// during the step. Triggers flow exclusively through the bus
	// subscription per director review on PR #667 — there is no
	// resolver-returned trigger slot to stub.
	publishOnStep func(bus dnd5events.EventBus, stepIdx int)
}

func (s *stubMovementResolver) ResolveStep(input tkenc.MovementStepInput) (*tkenc.MovementStepResult, error) {
	stepIdx := len(s.calls)
	s.calls = append(s.calls, input)

	// Bus arrives via input.EventBus (Wave 2.11e #673). Pre-#673 the stub
	// captured the bus from a post-construction setter — that test-only
	// hack masked the production gap until rpg-api#539's resolver impl
	// surfaced it.
	if s.publishOnStep != nil && input.EventBus != nil {
		s.publishOnStep(input.EventBus, stepIdx)
	}

	result := &tkenc.MovementStepResult{}
	if s.preventAtCall != nil && stepIdx == *s.preventAtCall {
		result.Prevented = true
		result.PreventReason = s.preventReason
	}
	return result, nil
}

type MovementResolverSuite struct {
	suite.Suite
	transport *tkenc.InMemoryTransport
	broker    *tkenc.Broker
	enc       *tkenc.Encounter
	resolver  *stubMovementResolver
}

func TestMovementResolverSuite(t *testing.T) {
	suite.Run(t, new(MovementResolverSuite))
}

// SetupTestNoResolver builds an encounter without a MovementResolver wired.
// Used by the legacy-fallback regression test below.
func (s *MovementResolverSuite) setupNoResolver() {
	s.transport = tkenc.NewInMemoryTransport()
	s.broker = tkenc.NewBroker(s.transport)
	s.enc = tkenc.New(context.Background(), "enc-moveres", s.broker)
	s.Require().NoError(s.enc.AddPlayer(tkenc.PlayerInput{
		PlayerID: alicePlayerID, EntityID: aliceEntityID,
		Position: encountercore.Hex{}, SightRange: 10,
	}))
}

// SetupTest is the default; wires a stubMovementResolver and seeds alice +
// bob. Most tests use this shape; the no-resolver test calls
// setupNoResolver() explicitly.
func (s *MovementResolverSuite) SetupTest() {
	s.transport = tkenc.NewInMemoryTransport()
	s.broker = tkenc.NewBroker(s.transport)
	s.resolver = &stubMovementResolver{}
	s.enc = tkenc.New(context.Background(), "enc-moveres", s.broker,
		tkenc.WithMovementResolver(s.resolver))

	s.Require().NoError(s.enc.AddPlayer(tkenc.PlayerInput{
		PlayerID: alicePlayerID, EntityID: aliceEntityID,
		Position: encountercore.Hex{}, SightRange: 10,
	}))
	s.Require().NoError(s.enc.AddPlayer(tkenc.PlayerInput{
		PlayerID: bobPlayerID, EntityID: bobEntityID,
		Position: encountercore.Hex{Q: 5, R: 0, S: -5}, SightRange: 10,
	}))
}

// TestMove_NoResolver_LegacyBehavior verifies the regression-guard shape
// from active.md B8: when no MovementResolver is wired, Encounter.Move
// mutates position directly without per-step iteration and does not call
// any per-step rulebook machinery. This is the existing single-jump
// behavior for non-combat encounters.
func (s *MovementResolverSuite) TestMove_NoResolver_LegacyBehavior() {
	s.setupNoResolver()

	path := []encountercore.Hex{
		{Q: 1, R: 0, S: -1},
		{Q: 2, R: 0, S: -2},
		{Q: 3, R: 0, S: -3},
	}
	err := s.enc.Move(alicePlayerID, path)
	s.Require().NoError(err)

	// Alice landed at the final hex (single-jump).
	aliceData := s.enc.ToData().Players[alicePlayerID]
	s.Require().NotNil(aliceData)
	s.Equal(encountercore.Hex{Q: 3, R: 0, S: -3}, aliceData.View.Position,
		"no-resolver path should jump straight to path[-1]")
}

// TestMove_ResolverNoTriggers_FullPath verifies that with a resolver wired
// and no triggers/prevention, Move iterates per-step (one ResolveStep call
// per path hex) and alice lands at the final hex with the full path
// recorded.
func (s *MovementResolverSuite) TestMove_ResolverNoTriggers_FullPath() {
	path := []encountercore.Hex{
		{Q: 1, R: 0, S: -1},
		{Q: 2, R: 0, S: -2},
		{Q: 3, R: 0, S: -3},
	}
	err := s.enc.Move(alicePlayerID, path)
	s.Require().NoError(err)

	// One ResolveStep call per hex in the path.
	s.Require().Len(s.resolver.calls, 3, "expected one ResolveStep per path hex")

	// Each call records the correct From → To for the step.
	s.Equal(encountercore.Hex{}, s.resolver.calls[0].FromHex)
	s.Equal(encountercore.Hex{Q: 1, R: 0, S: -1}, s.resolver.calls[0].ToHex)
	s.Equal(encountercore.Hex{Q: 1, R: 0, S: -1}, s.resolver.calls[1].FromHex)
	s.Equal(encountercore.Hex{Q: 2, R: 0, S: -2}, s.resolver.calls[1].ToHex)
	s.Equal(encountercore.Hex{Q: 2, R: 0, S: -2}, s.resolver.calls[2].FromHex)
	s.Equal(encountercore.Hex{Q: 3, R: 0, S: -3}, s.resolver.calls[2].ToHex)

	// Alice's EntityID is correctly threaded through.
	s.Equal(encountercore.EntityID(aliceEntityID), s.resolver.calls[0].EntityID)

	// Alice landed at the final hex.
	aliceData := s.enc.ToData().Players[alicePlayerID]
	s.Equal(encountercore.Hex{Q: 3, R: 0, S: -3}, aliceData.View.Position)
}

// TestMove_ResolverPrevented_TruncatesPath verifies that when the resolver
// signals Prevented mid-path, Move stops at the previous hex and does not
// advance further. The truncated traveled path is reflected in the
// encounter's published events.
func (s *MovementResolverSuite) TestMove_ResolverPrevented_TruncatesPath() {
	preventAt := 1 // prevent on step 2 (0-indexed = 1)
	s.resolver.preventAtCall = &preventAt
	s.resolver.preventReason = "stub prevention for test"

	path := []encountercore.Hex{
		{Q: 1, R: 0, S: -1},
		{Q: 2, R: 0, S: -2},
		{Q: 3, R: 0, S: -3},
	}
	err := s.enc.Move(alicePlayerID, path)
	s.Require().NoError(err, "Prevented is not an error; movement just stops")

	// ResolveStep called twice: once succeeded, once prevented. No third call.
	s.Require().Len(s.resolver.calls, 2,
		"resolver should be called for step 1 (succeeded) and step 2 (prevented), not step 3")

	// Alice stopped at hex (1,0,-1) — the position before the prevented step.
	aliceData := s.enc.ToData().Players[alicePlayerID]
	s.Equal(encountercore.Hex{Q: 1, R: 0, S: -1}, aliceData.View.Position,
		"position should stop at the last successfully-traveled hex")
}

// TestMove_ResolverPublishesTrigger_BufferDrainedCleanly verifies that when
// the resolver publishes a ReactionTriggerEvent on the encounter bus during
// ResolveStep (simulating an OA condition firing), the SDK's trigger buffer
// catches it without erroring. In NPC-OA-only scope the SDK does not act on
// the trigger (the resolver impl resolves NPC OAs inline), but it MUST drain
// the buffer cleanly to avoid leaked subscriptions across steps.
//
// Regression guard from active.md B8: the probe test asserted triggerCount
// == 0 when no resolver was supplied. With a resolver, the SDK should
// install the buffer subscription, see the trigger, and drop it (NPC-OA
// scope) — without leaking subscriptions.
func (s *MovementResolverSuite) TestMove_ResolverPublishesTrigger_BufferDrainedCleanly() {
	// Stub publishes a ReactionTriggerEvent on the bus during step 1.
	s.resolver.publishOnStep = func(bus dnd5events.EventBus, stepIdx int) {
		if stepIdx == 0 {
			topic := dnd5eEvents.ReactionTriggerTopic.On(bus)
			_ = topic.Publish(context.Background(), dnd5eEvents.ReactionTriggerEvent{
				ReactorID:    gobEntityID, // NPC reactor (not in players map)
				ConditionRef: "dnd5e:conditions:opportunity_attack",
				TriggerKind:  dnd5eEvents.TriggerKindMovementOA,
				SourceEntity: aliceEntityID,
			})
		}
	}

	path := []encountercore.Hex{
		{Q: 1, R: 0, S: -1},
		{Q: 2, R: 0, S: -2},
	}
	err := s.enc.Move(alicePlayerID, path)
	s.Require().NoError(err, "NPC triggers in buffer should NOT cause Move to error")

	// All steps still ran (NPC trigger doesn't pause movement in 2.11e scope).
	s.Require().Len(s.resolver.calls, 2)

	// Alice reached the final hex.
	aliceData := s.enc.ToData().Players[alicePlayerID]
	s.Equal(encountercore.Hex{Q: 2, R: 0, S: -2}, aliceData.View.Position)
}

// TestMove_ResolverPublishesEvents_TruncatedPath verifies that when
// movement is truncated (Prevented mid-path), the encounter SDK publishes
// the MoveEvent with the actually-traveled hexes only, not the full
// requested path. This keeps the wire payload truthful per Q3 signoff.
func (s *MovementResolverSuite) TestMove_ResolverPublishesEvents_TruncatedPath() {
	// Subscribe alice to her own viewer stream so we can observe the move
	// event for her.
	sub, err := s.broker.Subscribe(s.enc.ID(), alicePlayerID)
	s.Require().NoError(err)
	defer func() { _ = sub.Close() }()

	preventAt := 2 // prevent on step 3 (0-indexed = 2)
	s.resolver.preventAtCall = &preventAt

	path := []encountercore.Hex{
		{Q: 1, R: 0, S: -1},
		{Q: 2, R: 0, S: -2},
		{Q: 3, R: 0, S: -3},
	}
	err = s.enc.Move(alicePlayerID, path)
	s.Require().NoError(err)

	// Drain the alice subscription with a generous timeout (broker forwards
	// events via a goroutine; default-select would race with forwarding,
	// and a tight deadline can flake under CI load).
	var moveEvt *events.MoveEvent
	deadline := time.After(2 * time.Second)
drainLoop:
	for {
		select {
		case evt, ok := <-sub.Events():
			if !ok {
				break drainLoop
			}
			if me, isMove := evt.(*events.MoveEvent); isMove {
				moveEvt = me
			}
		case <-deadline:
			break drainLoop
		}
	}
	s.Require().NotNil(moveEvt, "MoveEvent should have been published")

	// The MoveEvent should carry only the traveled segments (hexes 1 + 2),
	// not the full requested path of 3 hexes.
	moverSlice, ok := moveEvt.PerPlayer[alicePlayerID]
	s.Require().True(ok, "alice should see her own move")
	s.Equal([]encountercore.Hex{
		{Q: 1, R: 0, S: -1},
		{Q: 2, R: 0, S: -2},
	}, moverSlice.SeenSegments,
		"MoveEvent should carry only the actually-traveled segments")
}

// --- Wave 2.11e (#668) — NPC-movement direction tests ---
//
// The wave-goal-refined sentence (#50) names BOTH movement directions:
// player→enemy AND enemy→player. PR #667 shipped the player direction
// via Encounter.Move; this slice ships the mirror through
// Encounter.applyNPCMovement (the path NPCAct uses for monster.TakeTurn's
// movement output). Same MovementResolver seam; same per-step iteration;
// same buffered trigger drain.
//
// Tests below drive NPC movement via Encounter.MoveNPCSteps — a small
// public seam that bypasses the full NPCAct → monster.TakeTurn flow so we
// can pin the movement path exactly (rather than depending on the
// monster's AI targeting). The same internal iteration code runs either
// way; the seam just gives tests deterministic input.

// addGoblinForNPCMovementTests adds a goblin combatant to the encounter
// so NPC-movement tests have a real MonsterData to push through the
// resolver-mediated iteration.
func (s *MovementResolverSuite) addGoblinForNPCMovementTests() {
	s.Require().NoError(s.enc.AddMonster(tkenc.MonsterInput{
		ID:       gobEntityID,
		Position: encountercore.Hex{Q: 10, R: 0, S: -10},
		HP:       7, MaxHP: 7, AC: 15, Speed: 6,
		AttackBonus: 4,
		DamageDice:  damage1d6plus1,
		DamageType:  damageSlashing,
		DataJSON:    testGoblinDataJSON(s.T(), gobEntityID),
	}))
}

// TestNPCMove_NoResolver_LegacyBehavior verifies the regression-guard B8
// MIRROR shape (#668 issue body): without a MovementResolver wired,
// applyNPCMovement single-jumps the goblin to the final hex without per-
// step iteration and does not invoke any chain machinery.
func (s *MovementResolverSuite) TestNPCMove_NoResolver_LegacyBehavior() {
	// Setup without resolver — rebuild the encounter wiring-free.
	s.transport = tkenc.NewInMemoryTransport()
	s.broker = tkenc.NewBroker(s.transport)
	s.enc = tkenc.New(context.Background(), "enc-moveres-npc", s.broker)
	s.Require().NoError(s.enc.AddPlayer(tkenc.PlayerInput{
		PlayerID: alicePlayerID, EntityID: aliceEntityID,
		Position: encountercore.Hex{}, SightRange: 10,
	}))
	s.addGoblinForNPCMovementTests()

	path := []encountercore.Hex{
		{Q: 9, R: 0, S: -9},
		{Q: 8, R: 0, S: -8},
		{Q: 7, R: 0, S: -7},
	}
	err := s.enc.MoveNPCSteps(gobEntityID, path)
	s.Require().NoError(err)

	// Goblin landed at the final hex (single-jump).
	mon := s.enc.ToData().Monsters[gobEntityID]
	s.Require().NotNil(mon)
	s.Equal(encountercore.Hex{Q: 7, R: 0, S: -7}, mon.Position,
		"no-resolver NPC path should jump straight to path[-1]")
}

// TestNPCMove_ResolverNoTriggers_FullPath verifies per-step iteration
// when a resolver is wired: one ResolveStep call per hex; goblin lands
// at the final hex.
func (s *MovementResolverSuite) TestNPCMove_ResolverNoTriggers_FullPath() {
	s.addGoblinForNPCMovementTests()
	startHex := encountercore.Hex{Q: 10, R: 0, S: -10}

	path := []encountercore.Hex{
		{Q: 9, R: 0, S: -9},
		{Q: 8, R: 0, S: -8},
		{Q: 7, R: 0, S: -7},
	}
	err := s.enc.MoveNPCSteps(gobEntityID, path)
	s.Require().NoError(err)

	// One ResolveStep call per hex.
	s.Require().Len(s.resolver.calls, 3,
		"expected one ResolveStep per NPC path hex")

	// First step from goblin's start hex.
	s.Equal(startHex, s.resolver.calls[0].FromHex)
	s.Equal(path[0], s.resolver.calls[0].ToHex)
	s.Equal(encountercore.EntityID(gobEntityID), s.resolver.calls[0].EntityID,
		"resolver call must carry the NPC's entity ID as mover")

	// Goblin landed at the final hex.
	mon := s.enc.ToData().Monsters[gobEntityID]
	s.Equal(path[2], mon.Position)
}

// TestNPCMove_ResolverPrevented_TruncatesPath verifies that when the
// resolver signals Prevented mid-path, the NPC stops at the previous hex
// and does not advance further (mirror of the player-direction truncation
// test).
func (s *MovementResolverSuite) TestNPCMove_ResolverPrevented_TruncatesPath() {
	s.addGoblinForNPCMovementTests()

	preventAt := 1 // prevent on step 2 (0-indexed = 1)
	s.resolver.preventAtCall = &preventAt
	s.resolver.preventReason = "stub prevention for npc test"

	path := []encountercore.Hex{
		{Q: 9, R: 0, S: -9},
		{Q: 8, R: 0, S: -8},
		{Q: 7, R: 0, S: -7},
	}
	err := s.enc.MoveNPCSteps(gobEntityID, path)
	s.Require().NoError(err, "Prevented is not an error; movement just stops")

	// Two calls — step 1 succeeded, step 2 returned Prevented; step 3 skipped.
	s.Require().Len(s.resolver.calls, 2,
		"resolver should be called for step 1 (succeeded) and step 2 (prevented), not step 3")

	// Goblin stopped at step 1's destination — did not advance to step 2.
	mon := s.enc.ToData().Monsters[gobEntityID]
	s.Equal(encountercore.Hex{Q: 9, R: 0, S: -9}, mon.Position,
		"NPC position should stop at the last successfully-traveled hex")
}

// TestNPCMove_StartHexInPath_TrimmedCorrectly verifies the trim shape
// for monster.TakeTurn's output: TurnResult.Movement includes the start
// hex per its contract (monster/monster.go:645 — "include start
// position, then each hex moved to"). Without trimming, the per-step
// iteration would see a no-op FromHex==ToHex first step and could
// misinterpret "prevented on first real move" as a non-empty traveled
// path. Copilot review on PR #672 flagged this; the fix lives in
// applyNPCMovementSteps and benefits both call sites.
//
// Test drives MoveNPCSteps with a start-included path (matching
// TakeTurn's shape) and asserts:
//   - resolver.calls equals len(path) - 1 (not len(path) — the start
//     hex was trimmed before iteration)
//   - first ResolveStep call has FromHex == startHex, ToHex == path[1]
//   - goblin lands at the final destination
func (s *MovementResolverSuite) TestNPCMove_StartHexInPath_TrimmedCorrectly() {
	s.addGoblinForNPCMovementTests()
	startHex := encountercore.Hex{Q: 10, R: 0, S: -10}

	// Start-included path — mirrors monster.TakeTurn's TurnResult.Movement
	// shape (start hex followed by destination hexes).
	path := []encountercore.Hex{
		startHex, // path[0] = current position (TakeTurn includes this)
		{Q: 9, R: 0, S: -9},
		{Q: 8, R: 0, S: -8},
	}
	err := s.enc.MoveNPCSteps(gobEntityID, path)
	s.Require().NoError(err)

	// Only 2 ResolveStep calls — the start hex was trimmed.
	s.Require().Len(s.resolver.calls, 2,
		"start hex must be trimmed; expected 2 calls for 3-hex path")

	// First call is the FIRST REAL step (from startHex to path[1]).
	s.Equal(startHex, s.resolver.calls[0].FromHex,
		"first call's FromHex should be the start hex")
	s.Equal(encountercore.Hex{Q: 9, R: 0, S: -9}, s.resolver.calls[0].ToHex,
		"first call's ToHex should be path[1] (path[0] was trimmed)")

	// Goblin landed at the final destination.
	mon := s.enc.ToData().Monsters[gobEntityID]
	s.Equal(encountercore.Hex{Q: 8, R: 0, S: -8}, mon.Position)
}

// TestNPCMove_OnlyStartHex_NoOp verifies the edge case where the path
// consists solely of the start hex (caller asked the NPC to "move" to
// where they already are). After trimming, the path is empty → no-op,
// no events fired.
func (s *MovementResolverSuite) TestNPCMove_OnlyStartHex_NoOp() {
	s.addGoblinForNPCMovementTests()
	startHex := encountercore.Hex{Q: 10, R: 0, S: -10}

	err := s.enc.MoveNPCSteps(gobEntityID, []encountercore.Hex{startHex})
	s.Require().NoError(err, "single-start-hex path is a no-op, not an error")

	s.Empty(s.resolver.calls, "no ResolveStep calls — nothing real to iterate")

	mon := s.enc.ToData().Monsters[gobEntityID]
	s.Equal(startHex, mon.Position, "position unchanged")
}

// TestMoveNPCSteps_EmptyPath_Errors verifies MoveNPCSteps aligns with
// Encounter.Move's empty-path convention — returns an error instead of
// silently no-op'ing. Copilot review flag on PR #672.
func (s *MovementResolverSuite) TestMoveNPCSteps_EmptyPath_Errors() {
	s.addGoblinForNPCMovementTests()

	err := s.enc.MoveNPCSteps(gobEntityID, nil)
	s.Require().Error(err, "empty path must error, matching Encounter.Move convention")
	s.Contains(err.Error(), "empty path")
}

// TestNPCMove_ResolverPublishesTrigger_BufferDrainedCleanly mirrors the
// player-direction buffer-drain regression guard. When the resolver
// publishes a ReactionTriggerEvent on the encounter bus during ResolveStep
// (simulating a player's OA condition firing against the moving NPC),
// the SDK's trigger buffer catches it without erroring. In NPC-OA-only
// scope the SDK does not act on the trigger (the resolver impl resolves
// player OAs inline against the NPC), but the buffer MUST drain cleanly
// to avoid leaked subscriptions across steps.
func (s *MovementResolverSuite) TestNPCMove_ResolverPublishesTrigger_BufferDrainedCleanly() {
	s.addGoblinForNPCMovementTests()

	// Stub publishes a ReactionTriggerEvent during step 1 — simulates
	// alice's OA condition firing because the goblin is leaving her reach.
	s.resolver.publishOnStep = func(bus dnd5events.EventBus, stepIdx int) {
		if stepIdx == 0 {
			topic := dnd5eEvents.ReactionTriggerTopic.On(bus)
			_ = topic.Publish(context.Background(), dnd5eEvents.ReactionTriggerEvent{
				ReactorID:    aliceEntityID, // player reactor (in players map)
				ConditionRef: "dnd5e:conditions:opportunity_attack",
				TriggerKind:  dnd5eEvents.TriggerKindMovementOA,
				SourceEntity: gobEntityID,
			})
		}
	}

	path := []encountercore.Hex{
		{Q: 9, R: 0, S: -9},
		{Q: 8, R: 0, S: -8},
	}
	err := s.enc.MoveNPCSteps(gobEntityID, path)
	s.Require().NoError(err,
		"player-reactor triggers in buffer should NOT cause NPC Move to error in NPC-OA-only scope")

	// All steps still ran.
	s.Require().Len(s.resolver.calls, 2)

	// Goblin reached the final hex.
	mon := s.enc.ToData().Monsters[gobEntityID]
	s.Equal(path[1], mon.Position)
}

// --- Wave 2.11e (#675) — Move-iteration OA damage application ---
//
// The goal-sentence direction for #675: when an OA fires mid-Move (player
// or NPC mover provokes a reaction attack that the rulebook resolves
// inline), the encounter SDK must observe the resulting damage on the bus
// and apply HP delta + publish encounter-side DamageDealtEvent. Without
// this, the goal sentence "OA-class reactions work end-to-end" doesn't
// hold — chain fires, dice roll, but target's HP never moves.
//
// These tests use the stub resolver to publish DamageReceivedEvent on the
// bus during ResolveStep (simulating combat.ResolveAttack inside
// combat.MoveEntity's triggerOpportunityAttack path) and verify the SDK
// dispatches HP + DamageDealtEvent.

// oaHitAttackRoll, oaHitAttackBonus, oaHitTargetAC are the fixed roll
// detail publishOAHit uses for every simulated OA hit (18 + 4 = 22 beats
// AC 15 across every fixture in this file). Only attacker/target/amount
// vary between call sites.
const (
	oaHitAttackRoll  = 18
	oaHitAttackBonus = 4
	oaHitTargetAC    = 15
)

// publishOAHit publishes the pair of bus events a real resolver publishes
// for one OA hit — combat.ResolveAttackHit's PostAttackRollEvent (always)
// followed by combat.ApplyAttackOutcome's DamageReceivedEvent (hit only).
// Test stubs simulate both, in that order, matching production's
// synchronous ResolveAttack call. See #715: before the fix, tests only
// simulated the damage half, which masked the OA path never publishing
// AttackResolvedEvent at all.
func publishOAHit(bus dnd5events.EventBus, attackerID, targetID string, amount int) {
	rolls := dnd5eEvents.PostAttackRollChain.On(bus)
	_, _ = rolls.PublishWithChain(context.Background(), &dnd5eEvents.PostAttackRollEvent{
		AttackerID:  attackerID,
		TargetID:    targetID,
		OriginalAC:  oaHitTargetAC,
		AttackRoll:  oaHitAttackRoll,
		AttackBonus: oaHitAttackBonus,
		TotalAttack: oaHitAttackRoll + oaHitAttackBonus,
		WouldHit:    true,
	}, dnd5events.NewStagedChain[*dnd5eEvents.PostAttackRollEvent](nil))

	dmg := dnd5eEvents.DamageReceivedTopic.On(bus)
	_ = dmg.Publish(context.Background(), dnd5eEvents.DamageReceivedEvent{
		TargetID:   targetID,
		SourceID:   attackerID,
		Amount:     amount,
		DamageType: damage.Slashing,
	})
}

// TestMove_PlayerMoves_OADamagesPlayer verifies the player-mover direction:
// alice moves past goblin (who has an OA condition); resolver publishes a
// DamageReceivedEvent targeting alice during step 0; encounter SDK applies
// HP delta to alice + publishes DamageDealtEvent.
func (s *MovementResolverSuite) TestMove_PlayerMoves_OADamagesPlayer() {
	s.addGoblinForNPCMovementTests()

	// Seed alice's HP so we can detect the delta.
	aliceBefore := s.enc.ToData().Players[alicePlayerID]
	s.Require().NotNil(aliceBefore)
	aliceBefore.HP = 20
	aliceBefore.MaxHP = 20

	// Stub publishes the roll + damage pair on the bus during step 0 — the
	// step where alice leaves the goblin's reach (the rulebook fires
	// triggerOpportunityAttack → ResolveAttack here).
	s.resolver.publishOnStep = func(bus dnd5events.EventBus, stepIdx int) {
		if stepIdx == 0 {
			publishOAHit(bus, string(gobEntityID), string(aliceEntityID), 6)
		}
	}

	// Subscribe alice to her own viewer stream to observe DamageDealtEvent.
	sub, err := s.broker.Subscribe(s.enc.ID(), alicePlayerID)
	s.Require().NoError(err)
	defer func() { _ = sub.Close() }()

	path := []encountercore.Hex{
		{Q: 1, R: 0, S: -1},
		{Q: 2, R: 0, S: -2},
	}
	err = s.enc.Move(alicePlayerID, path)
	s.Require().NoError(err)

	// HP applied: alice 20 → 14.
	aliceAfter := s.enc.ToData().Players[alicePlayerID]
	s.Equal(14, aliceAfter.HP, "alice HP should drop by OA damage amount (6)")

	// DamageDealtEvent published with target=alice, source=goblin, amount=6.
	var dmgEvt *events.DamageDealtEvent
	deadline := time.After(2 * time.Second)
drainLoop:
	for {
		select {
		case evt, ok := <-sub.Events():
			if !ok {
				break drainLoop
			}
			if de, isDmg := evt.(*events.DamageDealtEvent); isDmg {
				dmgEvt = de
				break drainLoop
			}
		case <-deadline:
			break drainLoop
		}
	}
	s.Require().NotNil(dmgEvt, "DamageDealtEvent should have been published")
	s.Equal(encountercore.EntityID(aliceEntityID), dmgEvt.TargetID)
	s.Equal(encountercore.EntityID(gobEntityID), dmgEvt.SourceID)
	s.Equal(6, dmgEvt.Amount)
	s.Equal("slashing", dmgEvt.DamageType)
	s.Equal(14, dmgEvt.HPAfter)
}

// TestMove_OAAttackResolved_CarriesAdvantageDisadvantage verifies the
// Move-path OA route (npc.go's publishMoveAttackResolved, distinct from the
// TakeAction path's publishAttackOutcome) also threads HasDisadvantage +
// DisadvantageSources onto its AttackResolvedEvent (#726). This route
// builds the event straight from dnd5eEvents.PostAttackRollEvent -- not
// from AttackOutcome/CombatResolver -- so it needs its own field-threading
// check; PostAttackRollEvent gained the same two bool + two ref-slice
// fields for exactly this reason.
func (s *MovementResolverSuite) TestMove_OAAttackResolved_CarriesAdvantageDisadvantage() {
	s.addGoblinForNPCMovementTests()

	dodgingRef := &toolkitcore.Ref{Module: refModuleDnd5e, Type: "conditions", ID: "dodging"}
	s.resolver.publishOnStep = func(bus dnd5events.EventBus, stepIdx int) {
		if stepIdx != 0 {
			return
		}
		rolls := dnd5eEvents.PostAttackRollChain.On(bus)
		_, _ = rolls.PublishWithChain(context.Background(), &dnd5eEvents.PostAttackRollEvent{
			AttackerID:          string(gobEntityID),
			TargetID:            string(aliceEntityID),
			OriginalAC:          oaHitTargetAC,
			AttackRoll:          5,
			AttackBonus:         oaHitAttackBonus,
			TotalAttack:         5 + oaHitAttackBonus,
			WouldHit:            false,
			HasDisadvantage:     true,
			DisadvantageSources: []*toolkitcore.Ref{dodgingRef},
		}, dnd5events.NewStagedChain[*dnd5eEvents.PostAttackRollEvent](nil))
	}

	sub, err := s.broker.Subscribe(s.enc.ID(), alicePlayerID)
	s.Require().NoError(err)
	defer func() { _ = sub.Close() }()

	path := []encountercore.Hex{
		{Q: 1, R: 0, S: -1},
		{Q: 2, R: 0, S: -2},
	}
	s.Require().NoError(s.enc.Move(alicePlayerID, path))

	var attackEvt *events.AttackResolvedEvent
	deadline := time.After(2 * time.Second)
drainLoop:
	for {
		select {
		case evt, ok := <-sub.Events():
			if !ok {
				break drainLoop
			}
			if ae, isAttack := evt.(*events.AttackResolvedEvent); isAttack {
				attackEvt = ae
				break drainLoop
			}
		case <-deadline:
			break drainLoop
		}
	}
	s.Require().NotNil(attackEvt, "AttackResolvedEvent should have been published for the OA roll")
	s.False(attackEvt.HasAdvantage)
	s.True(attackEvt.HasDisadvantage)
	s.Require().Len(attackEvt.DisadvantageSources, 1)
	s.Equal(dodgingRef, attackEvt.DisadvantageSources[0])
}

// TestNPCMove_NPCMoves_OADamagesMonster verifies the NPC-mover direction:
// goblin retreats past alice (who has an OA condition); resolver publishes
// a DamageReceivedEvent targeting goblin during step 0; encounter SDK
// applies HP delta to the monster + publishes DamageDealtEvent.
func (s *MovementResolverSuite) TestNPCMove_NPCMoves_OADamagesMonster() {
	s.addGoblinForNPCMovementTests()

	// Seed alice in melee range so the OA fiction makes sense (positions
	// are not validated by the SDK in this test — pure event flow).
	aliceBefore := s.enc.ToData().Players[alicePlayerID]
	s.Require().NotNil(aliceBefore)
	aliceBefore.HP = 20
	aliceBefore.MaxHP = 20

	// Stub publishes the roll + damage pair on step 0: alice (player) hits
	// the goblin (NPC) on its way out. Source = alice (player), Target =
	// goblin.
	s.resolver.publishOnStep = func(bus dnd5events.EventBus, stepIdx int) {
		if stepIdx == 0 {
			publishOAHit(bus, string(aliceEntityID), string(gobEntityID), 5)
		}
	}

	sub, err := s.broker.Subscribe(s.enc.ID(), alicePlayerID)
	s.Require().NoError(err)
	defer func() { _ = sub.Close() }()

	path := []encountercore.Hex{
		{Q: 9, R: 0, S: -9},
		{Q: 8, R: 0, S: -8},
	}
	err = s.enc.MoveNPCSteps(gobEntityID, path)
	s.Require().NoError(err)

	// HP applied: goblin 7 → 2.
	monAfter := s.enc.ToData().Monsters[gobEntityID]
	s.Equal(2, monAfter.HP, "goblin HP should drop by OA damage amount (5)")

	// DamageDealtEvent published with target=goblin, source=alice, amount=5.
	var dmgEvt *events.DamageDealtEvent
	deadline := time.After(2 * time.Second)
drainLoop:
	for {
		select {
		case evt, ok := <-sub.Events():
			if !ok {
				break drainLoop
			}
			if de, isDmg := evt.(*events.DamageDealtEvent); isDmg {
				dmgEvt = de
				break drainLoop
			}
		case <-deadline:
			break drainLoop
		}
	}
	s.Require().NotNil(dmgEvt, "DamageDealtEvent should have been published for NPC-target OA")
	s.Equal(encountercore.EntityID(gobEntityID), dmgEvt.TargetID)
	s.Equal(encountercore.EntityID(aliceEntityID), dmgEvt.SourceID)
	s.Equal(5, dmgEvt.Amount)
	s.Equal(2, dmgEvt.HPAfter)
}

// TestMove_OAKillsMonster_FiresKillChain verifies that when Move-path OA
// damage drops a monster's HP from >0 to 0, the SDK fires the kill chain
// (EntityDiedEvent + EntityRemovedEvent + checkEncounterEnd) — the same
// chain applyCapturedDamage drives for NPCAct-path damage.
func (s *MovementResolverSuite) TestMove_OAKillsMonster_FiresKillChain() {
	s.addGoblinForNPCMovementTests()

	// Stub damage exceeds goblin's HP (7) so the kill chain triggers.
	s.resolver.publishOnStep = func(bus dnd5events.EventBus, stepIdx int) {
		if stepIdx == 0 {
			publishOAHit(bus, string(aliceEntityID), string(gobEntityID), 10)
		}
	}

	sub, err := s.broker.Subscribe(s.enc.ID(), alicePlayerID)
	s.Require().NoError(err)
	defer func() { _ = sub.Close() }()

	path := []encountercore.Hex{
		{Q: 9, R: 0, S: -9},
		{Q: 8, R: 0, S: -8},
	}
	err = s.enc.MoveNPCSteps(gobEntityID, path)
	s.Require().NoError(err)

	// Goblin removed from encounter state (killEntity drops the entry).
	_, stillPresent := s.enc.ToData().Monsters[gobEntityID]
	s.False(stillPresent, "goblin should be removed from encounter state after lethal OA")

	// Full kill chain observed in alice's stream: EntityDied → EntityRemoved
	// → EncounterEnded (goblin was the lone monster). The drain breaks on
	// EncounterEndedEvent — safe because killEntity publishes in order
	// (Died, Removed, then encounterEnd-check triggers EncounterEnded), so
	// by the time we see EncounterEnded the prior two must have fired. The
	// order-dependence is noted in case a future refactor changes the
	// publish sequence; if so, this drain needs to be widened to read past
	// EncounterEnded.
	var diedEvt *events.EntityDiedEvent
	sawRemoved := false
	sawEnded := false
	deadline := time.After(2 * time.Second)
drainLoop:
	for {
		select {
		case evt, ok := <-sub.Events():
			if !ok {
				break drainLoop
			}
			switch e := evt.(type) {
			case *events.EntityDiedEvent:
				diedEvt = e
			case *events.EntityRemovedEvent:
				sawRemoved = true
			case *events.EncounterEndedEvent:
				sawEnded = true
				break drainLoop
			}
		case <-deadline:
			break drainLoop
		}
	}
	s.Require().NotNil(diedEvt, "EntityDiedEvent should fire on >0 → 0 transition")
	s.Equal(encountercore.EntityID(gobEntityID), diedEvt.EntityID)
	s.Equal(encountercore.EntityID(aliceEntityID), diedEvt.KillerID)
	s.True(sawRemoved, "EntityRemovedEvent should fire after EntityDied")
	s.True(sawEnded, "EncounterEndedEvent should fire when last monster dies")
}

// --- Issue #715 — Move-path OA emits the same shape as the normal attack
// path: AttackResolvedEvent always (hit or miss), DamageDealtEvent only on
// hit.
//
// Before this fix, iterateMovementStepsForEntity only observed
// DamageReceivedTopic: a miss (which combat.ApplyAttackOutcome never
// publishes damage for) produced no encounter event at all, and a hit
// produced a bare DamageDealtEvent with no AttackResolvedEvent alongside
// it — a different shape than TakeAction's publishAttackOutcome, which
// always emits AttackResolvedEvent and gates DamageDealtEvent on Hit.
//
// combat.ResolveAttackHit unconditionally publishes one PostAttackRollEvent
// per attack (hit or miss) via PostAttackRollChain; these tests simulate
// that real behavior directly through the stub resolver.

// TestMove_OAMisses_EmitsAttackResolvedNoDamage verifies that an OA miss
// triggered during Move publishes AttackResolvedEvent{Hit: false} with the
// roll detail, and does NOT publish DamageDealtEvent. Mirrors the normal
// attack path's miss shape (attackResolved only, HP unchanged).
func (s *MovementResolverSuite) TestMove_OAMisses_EmitsAttackResolvedNoDamage() {
	s.addGoblinForNPCMovementTests()

	aliceBefore := s.enc.ToData().Players[alicePlayerID]
	s.Require().NotNil(aliceBefore)
	aliceBefore.HP = 20
	aliceBefore.MaxHP = 20

	// Stub publishes ONLY a PostAttackRollEvent with WouldHit=false — no
	// DamageReceivedEvent, matching combat.ApplyAttackOutcome's `if !hit {
	// return }` early-out (attack_phases.go) which never reaches the
	// damage-publish step on a miss.
	s.resolver.publishOnStep = func(bus dnd5events.EventBus, stepIdx int) {
		if stepIdx == 0 {
			rolls := dnd5eEvents.PostAttackRollChain.On(bus)
			_, _ = rolls.PublishWithChain(context.Background(), &dnd5eEvents.PostAttackRollEvent{
				AttackerID:  string(gobEntityID),
				TargetID:    string(aliceEntityID),
				OriginalAC:  15,
				AttackRoll:  8,
				AttackBonus: 4,
				TotalAttack: 12,
				WouldHit:    false,
			}, dnd5events.NewStagedChain[*dnd5eEvents.PostAttackRollEvent](nil))
		}
	}

	sub, err := s.broker.Subscribe(s.enc.ID(), alicePlayerID)
	s.Require().NoError(err)
	defer func() { _ = sub.Close() }()

	path := []encountercore.Hex{
		{Q: 1, R: 0, S: -1},
		{Q: 2, R: 0, S: -2},
	}
	err = s.enc.Move(alicePlayerID, path)
	s.Require().NoError(err)

	// HP unchanged.
	aliceAfter := s.enc.ToData().Players[alicePlayerID]
	s.Equal(20, aliceAfter.HP, "a miss must not change HP")

	var attackEvt *events.AttackResolvedEvent
	var dmgEvt *events.DamageDealtEvent
	deadline := time.After(2 * time.Second)
drainLoopMiss:
	for {
		select {
		case evt, ok := <-sub.Events():
			if !ok {
				break drainLoopMiss
			}
			switch e := evt.(type) {
			case *events.AttackResolvedEvent:
				attackEvt = e
			case *events.DamageDealtEvent:
				dmgEvt = e
			}
		case <-deadline:
			break drainLoopMiss
		}
	}

	s.Require().NotNil(attackEvt, "AttackResolvedEvent must be published on an OA miss")
	s.False(attackEvt.Hit, "attack-resolved must report the miss")
	s.Equal(encountercore.EntityID(gobEntityID), attackEvt.AttackerID)
	s.Equal(encountercore.EntityID(aliceEntityID), attackEvt.TargetID)
	s.Equal(8, attackEvt.AttackRoll)
	s.Equal(4, attackEvt.AttackBonus)
	s.Equal(15, attackEvt.TargetAC)
	s.Nil(dmgEvt, "a miss must not publish DamageDealtEvent")
}

// TestMove_OAHits_EmitsAttackResolvedAndDamage verifies that an OA hit
// triggered during Move publishes BOTH AttackResolvedEvent{Hit: true} and
// DamageDealtEvent, matching the normal attack path's hit shape.
func (s *MovementResolverSuite) TestMove_OAHits_EmitsAttackResolvedAndDamage() {
	s.addGoblinForNPCMovementTests()

	aliceBefore := s.enc.ToData().Players[alicePlayerID]
	s.Require().NotNil(aliceBefore)
	aliceBefore.HP = 20
	aliceBefore.MaxHP = 20

	s.resolver.publishOnStep = func(bus dnd5events.EventBus, stepIdx int) {
		if stepIdx == 0 {
			publishOAHit(bus, string(gobEntityID), string(aliceEntityID), 6)
		}
	}

	sub, err := s.broker.Subscribe(s.enc.ID(), alicePlayerID)
	s.Require().NoError(err)
	defer func() { _ = sub.Close() }()

	path := []encountercore.Hex{
		{Q: 1, R: 0, S: -1},
		{Q: 2, R: 0, S: -2},
	}
	err = s.enc.Move(alicePlayerID, path)
	s.Require().NoError(err)

	aliceAfter := s.enc.ToData().Players[alicePlayerID]
	s.Equal(14, aliceAfter.HP, "alice HP should drop by OA damage amount (6)")

	var attackEvt *events.AttackResolvedEvent
	var dmgEvt *events.DamageDealtEvent
	deadline := time.After(2 * time.Second)
drainLoopHit:
	for {
		select {
		case evt, ok := <-sub.Events():
			if !ok {
				break drainLoopHit
			}
			switch e := evt.(type) {
			case *events.AttackResolvedEvent:
				attackEvt = e
			case *events.DamageDealtEvent:
				dmgEvt = e
			}
			if attackEvt != nil && dmgEvt != nil {
				break drainLoopHit
			}
		case <-deadline:
			break drainLoopHit
		}
	}

	s.Require().NotNil(attackEvt, "AttackResolvedEvent must be published on an OA hit")
	s.True(attackEvt.Hit)
	s.Equal(18, attackEvt.AttackRoll)
	s.Equal(4, attackEvt.AttackBonus)
	s.Equal(15, attackEvt.TargetAC)

	s.Require().NotNil(dmgEvt, "DamageDealtEvent must be published on an OA hit")
	s.Equal(encountercore.EntityID(aliceEntityID), dmgEvt.TargetID)
	s.Equal(encountercore.EntityID(gobEntityID), dmgEvt.SourceID)
	s.Equal(6, dmgEvt.Amount)
	s.Equal(14, dmgEvt.HPAfter)
}

// TestMove_DamageWithoutRoll_StillApplies is the Copilot review regression
// guard from PR #718: a MovementResolver implementation that publishes
// DamageReceivedEvent WITHOUT a preceding PostAttackRollEvent (a non-attack
// movement hazard, or a resolver that doesn't route through
// combat.ResolveAttackHit) must still have its damage applied — a strict
// roll-driven loop would otherwise silently drop the HP delta because the
// damage has no roll to pair with. Verifies HP still applies and
// DamageDealtEvent still publishes even with zero captured rolls.
func (s *MovementResolverSuite) TestMove_DamageWithoutRoll_StillApplies() {
	s.addGoblinForNPCMovementTests()

	aliceBefore := s.enc.ToData().Players[alicePlayerID]
	s.Require().NotNil(aliceBefore)
	aliceBefore.HP = 20
	aliceBefore.MaxHP = 20

	// Stub publishes ONLY a DamageReceivedEvent — no PostAttackRollEvent —
	// simulating a resolver/damage-source that doesn't produce roll detail.
	s.resolver.publishOnStep = func(bus dnd5events.EventBus, stepIdx int) {
		if stepIdx == 0 {
			topic := dnd5eEvents.DamageReceivedTopic.On(bus)
			_ = topic.Publish(context.Background(), dnd5eEvents.DamageReceivedEvent{
				TargetID:   string(aliceEntityID),
				SourceID:   string(gobEntityID),
				Amount:     4,
				DamageType: damage.Slashing,
			})
		}
	}

	sub, err := s.broker.Subscribe(s.enc.ID(), alicePlayerID)
	s.Require().NoError(err)
	defer func() { _ = sub.Close() }()

	path := []encountercore.Hex{
		{Q: 1, R: 0, S: -1},
		{Q: 2, R: 0, S: -2},
	}
	err = s.enc.Move(alicePlayerID, path)
	s.Require().NoError(err)

	aliceAfter := s.enc.ToData().Players[alicePlayerID]
	s.Equal(16, aliceAfter.HP, "unpaired damage must still apply, not be silently dropped")

	var dmgEvt *events.DamageDealtEvent
	deadline := time.After(2 * time.Second)
drainLoopUnpaired:
	for {
		select {
		case evt, ok := <-sub.Events():
			if !ok {
				break drainLoopUnpaired
			}
			if de, isDmg := evt.(*events.DamageDealtEvent); isDmg {
				dmgEvt = de
				break drainLoopUnpaired
			}
		case <-deadline:
			break drainLoopUnpaired
		}
	}
	s.Require().NotNil(dmgEvt, "DamageDealtEvent must still publish for unpaired damage")
	s.Equal(4, dmgEvt.Amount)
	s.Equal(16, dmgEvt.HPAfter)
}

// --- Issue #710 — Move-path OA's DamageDealtEvent shares its sibling
// AttackResolvedEvent's correlation id (Invariant 8), matching TakeAction's
// publishAttackOutcome. Before this fix, applyMoveDamage published
// DamageDealtEvent via a raw broker.Publish call that never stamped a
// correlation id (Stamp is only called by publishCorrelated), so the event
// always carried an empty correlation id on the wire — unlike its
// TakeAction-path sibling, which is correlated to the causing action.

// TestMove_OAHit_DamageSharesAttackResolvedCorrelationID verifies that an
// OA hit's AttackResolvedEvent and DamageDealtEvent carry the same
// non-empty correlation id.
func (s *MovementResolverSuite) TestMove_OAHit_DamageSharesAttackResolvedCorrelationID() {
	s.addGoblinForNPCMovementTests()

	aliceBefore := s.enc.ToData().Players[alicePlayerID]
	s.Require().NotNil(aliceBefore)
	aliceBefore.HP = 20
	aliceBefore.MaxHP = 20

	s.resolver.publishOnStep = func(bus dnd5events.EventBus, stepIdx int) {
		if stepIdx == 0 {
			publishOAHit(bus, string(gobEntityID), string(aliceEntityID), 6)
		}
	}

	sub, err := s.broker.Subscribe(s.enc.ID(), alicePlayerID)
	s.Require().NoError(err)
	defer func() { _ = sub.Close() }()

	path := []encountercore.Hex{
		{Q: 1, R: 0, S: -1},
		{Q: 2, R: 0, S: -2},
	}
	err = s.enc.Move(alicePlayerID, path)
	s.Require().NoError(err)

	var attackEvt *events.AttackResolvedEvent
	var dmgEvt *events.DamageDealtEvent
	deadline := time.After(2 * time.Second)
drainLoopCorr:
	for {
		select {
		case evt, ok := <-sub.Events():
			if !ok {
				break drainLoopCorr
			}
			switch e := evt.(type) {
			case *events.AttackResolvedEvent:
				attackEvt = e
			case *events.DamageDealtEvent:
				dmgEvt = e
			}
			if attackEvt != nil && dmgEvt != nil {
				break drainLoopCorr
			}
		case <-deadline:
			break drainLoopCorr
		}
	}

	s.Require().NotNil(attackEvt, "AttackResolvedEvent must be published on an OA hit")
	s.Require().NotNil(dmgEvt, "DamageDealtEvent must be published on an OA hit")
	s.NotEmpty(attackEvt.CorrelationID(), "attack-resolved must carry a correlation id")
	s.Equal(attackEvt.CorrelationID(), dmgEvt.CorrelationID(),
		"damage-dealt must share the OA's correlation id (Inv 8)")
}
