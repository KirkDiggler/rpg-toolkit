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

	tkenc "github.com/KirkDiggler/rpg-toolkit/encounter"
	encountercore "github.com/KirkDiggler/rpg-toolkit/encounter/core"
	"github.com/KirkDiggler/rpg-toolkit/encounter/events"
	dnd5events "github.com/KirkDiggler/rpg-toolkit/events"
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

	// bus is captured from the first call so we can publish into it.
	bus dnd5events.EventBus
}

func (s *stubMovementResolver) ResolveStep(input tkenc.MovementStepInput) (*tkenc.MovementStepResult, error) {
	stepIdx := len(s.calls)
	s.calls = append(s.calls, input)

	// Stub captures the bus from the encounter's WithMovementResolver wiring
	// path. Real resolvers know their own bus; the stub uses whatever is
	// stashed by SetupTest.
	if s.publishOnStep != nil && s.bus != nil {
		s.publishOnStep(s.bus, stepIdx)
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
	s.enc = tkenc.New("enc-moveres", s.broker)
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
	s.enc = tkenc.New("enc-moveres", s.broker,
		tkenc.WithMovementResolver(s.resolver))

	// Stash the bus on the resolver so publishOnStep can inject triggers.
	// The Encounter's bus is exposed via EventBus() (Wave 2.11d).
	s.resolver.bus = s.enc.EventBus()

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
