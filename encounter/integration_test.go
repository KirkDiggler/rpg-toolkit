package encounter_test

import (
	"encoding/json"
	"reflect"
	"sync/atomic"
	"testing"
	"time"

	"github.com/KirkDiggler/rpg-toolkit/encounter"
	"github.com/KirkDiggler/rpg-toolkit/encounter/core"
	dnd5events "github.com/KirkDiggler/rpg-toolkit/events"
	"github.com/stretchr/testify/suite"
)

type IntegrationSuite struct {
	suite.Suite
	transport *encounter.InMemoryTransport
	broker    *encounter.Broker
	enc       *encounter.Encounter
	aliceSub  *encounter.Subscription
	bobSub    *encounter.Subscription
}

func TestIntegrationSuite(t *testing.T) {
	suite.Run(t, new(IntegrationSuite))
}

func (s *IntegrationSuite) SetupTest() {
	s.transport = encounter.NewInMemoryTransport()
	s.broker = encounter.NewBroker(s.transport)
	s.enc = encounter.New("enc-walking-skel", s.broker)

	s.Require().NoError(s.enc.AddPlayer(encounter.PlayerInput{
		PlayerID: "alice", EntityID: "char-alice",
		Position: core.Hex{}, SightRange: 4,
	}))
	s.Require().NoError(s.enc.AddPlayer(encounter.PlayerInput{
		PlayerID: "bob", EntityID: "char-bob",
		Position: core.Hex{Q: 2, R: 0, S: -2}, SightRange: 4,
	}))
	s.enc.AddDoor("door-east", core.Hex{Q: 4, R: 0, S: -4}, false)

	var err error
	s.aliceSub, err = s.broker.Subscribe("enc-walking-skel", "alice")
	s.Require().NoError(err)
	s.bobSub, err = s.broker.Subscribe("enc-walking-skel", "bob")
	s.Require().NoError(err)
}

func (s *IntegrationSuite) TearDownTest() {
	_ = s.aliceSub.Close()
	_ = s.bobSub.Close()
	_ = s.broker.Close()
	_ = s.transport.Close()
}

// Alice moves toward Bob. Bob is within sight (distance 2 between her
// destination and his position). Both should receive MoveEvent.
func (s *IntegrationSuite) TestSlice_MoveTowardEachOther() {
	path := []core.Hex{{Q: 1, R: 0, S: -1}}
	s.Require().NoError(s.enc.Move("alice", path))

	aliceEvents := collectTypes(s.aliceSub, 500*time.Millisecond)
	bobEvents := collectTypes(s.bobSub, 500*time.Millisecond)

	s.Contains(aliceEvents, "*events.MoveEvent")
	s.Contains(bobEvents, "*events.MoveEvent",
		"bob within distance 2 should see alice move")
}

// Door opens at distance 4 from alice (sight range 4) and distance 2 from
// bob (sight range 4). Both should see the door event.
func (s *IntegrationSuite) TestSlice_OpenDoor() {
	s.Require().NoError(s.enc.OpenDoor("alice", "door-east"))

	aliceEvents := collectTypes(s.aliceSub, 500*time.Millisecond)
	bobEvents := collectTypes(s.bobSub, 500*time.Millisecond)

	s.Contains(aliceEvents, "*events.DoorOpenedEvent")
	s.Contains(bobEvents, "*events.DoorOpenedEvent")
}

// Persistence round-trip: serialize, "restart," replay another action,
// observe events still flow and prior reveal persisted.
func (s *IntegrationSuite) TestSlice_RoundTripPersistence() {
	s.Require().NoError(s.enc.Move("alice", []core.Hex{{Q: 1, R: 0, S: -1}}))
	drainSub(s.aliceSub, 200*time.Millisecond)
	drainSub(s.bobSub, 200*time.Millisecond)

	payload, err := json.Marshal(s.enc.ToData())
	s.Require().NoError(err)

	// Simulate restart.
	_ = s.broker.Close()
	_ = s.transport.Close()
	s.transport = encounter.NewInMemoryTransport()
	s.broker = encounter.NewBroker(s.transport)

	var loaded encounter.Data
	s.Require().NoError(json.Unmarshal(payload, &loaded))
	enc2, err := encounter.LoadFromData(&loaded, s.broker)
	s.Require().NoError(err)

	aliceSub2, err := s.broker.Subscribe("enc-walking-skel", "alice")
	s.Require().NoError(err)
	defer func() { _ = aliceSub2.Close() }()

	s.Require().NoError(enc2.Move("alice", []core.Hex{{Q: 2, R: 0, S: -2}}))

	aliceEvents := collectTypes(aliceSub2, 500*time.Millisecond)
	s.Contains(aliceEvents, "*events.MoveEvent",
		"after reload, encounter publishes events through the new broker")

	snap := enc2.SnapshotFor("alice")
	s.True(snap.RevealedHexes.Has(core.Hex{Q: 1, R: 0, S: -1}),
		"reveal from round-1 move should persist through round-trip")
}

// Sequence numbers monotonic across both events of one action.
func (s *IntegrationSuite) TestSlice_SequenceMonotonic() {
	s.Require().NoError(s.enc.Move("alice", []core.Hex{{Q: 1, R: 0, S: -1}}))

	var seqs []uint64
	timeout := time.After(500 * time.Millisecond)
loop:
	for {
		select {
		case evt, ok := <-s.aliceSub.Events():
			if !ok {
				break loop
			}
			seqs = append(seqs, evt.Sequence())
			if len(seqs) >= 2 {
				break loop
			}
		case <-timeout:
			break loop
		}
	}
	s.Require().Len(seqs, 2, "expected MoveEvent + HexRevealedEvent")
	s.True(seqs[1] > seqs[0], "sequence advances between events")
}

func drainSub(sub *encounter.Subscription, timeout time.Duration) {
	deadline := time.After(timeout)
	for {
		select {
		case _, ok := <-sub.Events():
			if !ok {
				return
			}
		case <-deadline:
			return
		}
	}
}

// --- Wave 2.11c integration tests ---

// busCapturingResolver is a CombatResolver test double that records the
// EventBus it receives on each ResolveAttack call. Used to verify the
// encounter SDK passes the same bus instance across multiple attacks.
type busCapturingResolver struct {
	callCount atomic.Int32
	buses     []dnd5events.EventBus
	damage    int
}

func (r *busCapturingResolver) ResolveAttack(input encounter.AttackInput) (*encounter.AttackOutcome, error) {
	r.callCount.Add(1)
	r.buses = append(r.buses, input.EventBus)
	return &encounter.AttackOutcome{
		Hit:        true,
		AttackRoll: 15,
		TargetAC:   10,
		Damage:     r.damage,
		DamageType: "slashing",
	}, nil
}

// ConditionPersistenceSuite verifies Wave 2.11c foundation:
// - The encounter-scoped bus is the same instance across attacks.
// - The resolver receives the bus on every call.
// - Reaction readiness round-trips through ToData/LoadFromData.
type ConditionPersistenceSuite struct {
	suite.Suite
	transport *encounter.InMemoryTransport
	broker    *encounter.Broker
}

func TestConditionPersistenceSuite(t *testing.T) {
	suite.Run(t, new(ConditionPersistenceSuite))
}

func (s *ConditionPersistenceSuite) SetupTest() {
	s.transport = encounter.NewInMemoryTransport()
	s.broker = encounter.NewBroker(s.transport)
}

func (s *ConditionPersistenceSuite) TearDownTest() {
	_ = s.broker.Close()
	_ = s.transport.Close()
}

// TestSlice_ConditionStatePersistsAcrossAttacks validates that the encounter
// SDK passes the same EventBus instance to the resolver on every attack call.
// This is the foundational guarantee that conditions Apply()'d once at
// rehydration remain subscribed across all attacks in the encounter (enabling
// SneakAttack.UsedThisTurn to hold for a whole turn, etc.).
func (s *ConditionPersistenceSuite) TestSlice_ConditionStatePersistsAcrossAttacks() {
	resolver := &busCapturingResolver{damage: 3}
	enc := encounter.New("enc-bus-test", s.broker,
		encounter.WithCombatResolver(resolver),
	)

	aliceSub, err := s.broker.Subscribe("enc-bus-test", "alice")
	s.Require().NoError(err)
	defer func() { _ = aliceSub.Close() }()

	// Add combatants.
	s.Require().NoError(enc.AddPlayer(encounter.PlayerInput{
		PlayerID: "alice", EntityID: "char-alice",
		Position: core.Hex{}, SightRange: 10,
		HP: 20, MaxHP: 20, AC: 14,
		DamageDice: "1d8", DamageType: "slashing",
	}))
	s.Require().NoError(enc.AddMonster(encounter.MonsterInput{
		ID:       "goblin-1",
		Position: core.Hex{Q: 1},
		HP:       20, MaxHP: 20, AC: 12,
		DamageDice: "1d6", DamageType: "piercing",
	}))

	// Start combat — alice must go first for the test to be deterministic.
	s.Require().NoError(enc.SetMode(core.ModeTurnBased))
	for enc.ActiveActor() != "char-alice" {
		_, _, err := enc.EndTurn(enc.ActiveActor())
		s.Require().NoError(err)
	}

	// First attack.
	s.Require().NoError(enc.TakeAction("alice",
		encounter.ActionRef{Module: "dnd5e", Type: "action", ID: "attack"},
		encounter.ActionTarget{EntityID: "goblin-1"},
	))

	// Second attack (same turn — encounter SDK does not enforce multi-attack
	// limits; that's the resolver's job).
	s.Require().NoError(enc.TakeAction("alice",
		encounter.ActionRef{Module: "dnd5e", Type: "action", ID: "attack"},
		encounter.ActionTarget{EntityID: "goblin-1"},
	))

	// Both calls must have gone to the resolver.
	s.EqualValues(2, resolver.callCount.Load(), "resolver must be called for both attacks")

	// Both calls must carry the same bus instance (encounter-scoped, not per-attack).
	// Use pointer identity — DeepEqual on an interface could match two
	// structurally identical but distinct bus objects.
	s.Require().Len(resolver.buses, 2)
	s.Equal(
		reflect.ValueOf(resolver.buses[0]).Pointer(),
		reflect.ValueOf(resolver.buses[1]).Pointer(),
		"encounter SDK must pass the same EventBus instance across attacks within an encounter",
	)

	// The bus must not be nil.
	s.NotNil(resolver.buses[0], "encounter bus passed to resolver must not be nil")
}

// TestSlice_ReactionReadinessPersistsThroughRoundTrip verifies that the
// ReactionReadiness map survives a ToData/LoadFromData cycle and that the
// resolver receives a non-nil bus after rehydration.
func (s *ConditionPersistenceSuite) TestSlice_ReactionReadinessPersistsThroughRoundTrip() {
	resolver := &busCapturingResolver{damage: 2}
	enc := encounter.New("enc-rt-test", s.broker,
		encounter.WithCombatResolver(resolver),
	)

	s.Require().NoError(enc.AddPlayer(encounter.PlayerInput{
		PlayerID: "alice", EntityID: "char-alice",
		Position: core.Hex{}, SightRange: 4,
		HP: 15, MaxHP: 15, AC: 13,
		DamageDice: "1d6", DamageType: "slashing",
	}))
	s.Require().NoError(enc.AddMonster(encounter.MonsterInput{
		ID:       "goblin-1",
		Position: core.Hex{Q: 1},
		HP:       10, MaxHP: 10, AC: 11,
		DamageDice: "1d4", DamageType: "piercing",
	}))

	// Mutate readiness before serialising.
	s.Require().NoError(enc.SetReactionReady("char-alice", encounter.OAReactionRef, false))
	s.Require().NoError(enc.SetReactionReady("char-alice", "dnd5e:conditions:shield", true))

	// Serialize.
	raw, err := json.Marshal(enc.ToData())
	s.Require().NoError(err)

	// Rehydrate.
	var restored encounter.Data
	s.Require().NoError(json.Unmarshal(raw, &restored))
	enc2, err := encounter.LoadFromData(&restored, s.broker,
		encounter.WithCombatResolver(resolver),
	)
	s.Require().NoError(err)

	// Readiness map survived.
	s.False(enc2.IsReactionReady("char-alice", encounter.OAReactionRef),
		"alice opted out of OA before serialise — must persist")
	s.True(enc2.IsReactionReady("char-alice", "dnd5e:conditions:shield"),
		"alice opted into Shield before serialise — must persist")
	s.True(enc2.IsReactionReady("goblin-1", encounter.OAReactionRef),
		"goblin default OA ready must survive round-trip")

	// The rehydrated encounter has a non-nil bus.
	s.NotNil(enc2.EventBus(), "rehydrated encounter must have a non-nil EventBus")
}
