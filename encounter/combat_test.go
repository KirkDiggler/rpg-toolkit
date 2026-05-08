package encounter_test

import (
	"context"
	"testing"
	"time"

	"github.com/stretchr/testify/suite"

	"github.com/KirkDiggler/rpg-toolkit/encounter"
	"github.com/KirkDiggler/rpg-toolkit/encounter/core"
	"github.com/KirkDiggler/rpg-toolkit/encounter/events"
)

// CombatSuite covers the Wave 2.8 verbs (SetMode, EndTurn, TakeAction,
// NPCAct) and the new combat events. Fixture: alice + bob + goblin-1.
type CombatSuite struct {
	suite.Suite
	ctx       context.Context
	transport *encounter.InMemoryTransport
	broker    *encounter.Broker
	enc       *encounter.Encounter
	aliceSub  *encounter.Subscription
	bobSub    *encounter.Subscription
}

func TestCombatSuite(t *testing.T) {
	suite.Run(t, new(CombatSuite))
}

func (s *CombatSuite) SetupTest() {
	s.ctx = context.Background()
	s.transport = encounter.NewInMemoryTransport()
	s.broker = encounter.NewBroker(s.transport)
	s.enc = encounter.New("enc-combat", s.broker)

	s.Require().NoError(s.enc.AddPlayer(encounter.PlayerInput{
		PlayerID: "alice", EntityID: "char-alice",
		Position: core.Hex{}, SightRange: 10,
		HP: 12, MaxHP: 12, AC: 14, AttackBonus: 4,
		DamageDice: "1d8+2", DamageType: "slashing",
	}))
	s.Require().NoError(s.enc.AddPlayer(encounter.PlayerInput{
		PlayerID: "bob", EntityID: "char-bob",
		Position: core.Hex{Q: 1, R: 0, S: -1}, SightRange: 10,
		HP: 10, MaxHP: 10, AC: 13, AttackBonus: 3,
		DamageDice: "1d6+1", DamageType: "piercing",
	}))
	s.Require().NoError(s.enc.AddMonster(encounter.MonsterInput{
		ID:       "goblin-1",
		Position: core.Hex{Q: 1, R: 0, S: -1},
		HP:       7, MaxHP: 7, AC: 15, Speed: 6,
		MonsterRef:  "dnd5e:monsters:goblin",
		AttackBonus: 4, DamageDice: "1d6+2", DamageType: "slashing",
	}))

	var err error
	s.aliceSub, err = s.broker.Subscribe("enc-combat", "alice")
	s.Require().NoError(err)
	s.bobSub, err = s.broker.Subscribe("enc-combat", "bob")
	s.Require().NoError(err)
}

func (s *CombatSuite) TearDownTest() {
	if s.aliceSub != nil {
		_ = s.aliceSub.Close()
	}
	if s.bobSub != nil {
		_ = s.bobSub.Close()
	}
	_ = s.broker.Close()
	_ = s.transport.Close()
}

// SetMode flip to TURN_BASED rolls initiative, fires ModeChangedEvent +
// TurnStartedEvent, and gates verbs on mode.
func (s *CombatSuite) TestSetMode_FlipsAndPublishes() {
	s.Equal(core.ModeFreeRoam, s.enc.Mode())
	s.Equal(core.EntityID(""), s.enc.ActiveActor())

	s.Require().NoError(s.enc.SetMode(core.ModeTurnBased))
	s.Equal(core.ModeTurnBased, s.enc.Mode())
	s.NotEqual(core.EntityID(""), s.enc.ActiveActor())

	seen := collectTypes(s.aliceSub, 500*time.Millisecond)
	s.Contains(seen, "*events.ModeChangedEvent")
	s.Contains(seen, "*events.TurnStartedEvent")
}

// SetMode rejects redundant flips.
func (s *CombatSuite) TestSetMode_RejectsRedundant() {
	s.Require().NoError(s.enc.SetMode(core.ModeTurnBased))
	s.Error(s.enc.SetMode(core.ModeTurnBased))
}

// TakeAction in FreeRoam mode returns ErrNotTurnBased.
func (s *CombatSuite) TestTakeAction_RejectedOutsideTurnBased() {
	err := s.enc.TakeAction("alice",
		encounter.ActionRef{Module: "dnd5e", Type: "action", ID: "attack"},
		encounter.ActionTarget{EntityID: "goblin-1"},
	)
	s.ErrorIs(err, encounter.ErrNotTurnBased)
}

// TakeAction by a non-active player returns ErrNotYourTurn.
func (s *CombatSuite) TestTakeAction_RejectedWhenNotYourTurn() {
	s.Require().NoError(s.enc.SetMode(core.ModeTurnBased))
	active := s.enc.ActiveActor()
	// Find the OTHER player and try to act.
	var attackerID core.PlayerID
	if active == "char-alice" {
		attackerID = "bob"
	} else {
		attackerID = "alice"
	}
	err := s.enc.TakeAction(attackerID,
		encounter.ActionRef{Module: "dnd5e", Type: "action", ID: "attack"},
		encounter.ActionTarget{EntityID: "goblin-1"},
	)
	s.ErrorIs(err, encounter.ErrNotYourTurn)
}

// TakeAction with an unknown action ref returns ErrUnsupportedAction.
func (s *CombatSuite) TestTakeAction_RejectsUnknownAction() {
	s.Require().NoError(s.enc.SetMode(core.ModeTurnBased))
	active := s.enc.ActiveActor()
	playerID := s.playerIDFor(active)
	if playerID == "" {
		s.T().Skip("active actor is an NPC; this test only covers player turns")
	}
	err := s.enc.TakeAction(playerID,
		encounter.ActionRef{Module: "dnd5e", Type: "action", ID: "shove"},
		encounter.ActionTarget{EntityID: "goblin-1"},
	)
	s.ErrorIs(err, encounter.ErrUnsupportedAction)
}

// TakeAction publishes AttackResolvedEvent (always); on hit a
// DamageDealtEvent rides alongside.
func (s *CombatSuite) TestTakeAction_PublishesAttackOutcome() {
	s.Require().NoError(s.enc.SetMode(core.ModeTurnBased))
	// Force alice to be active by ending turns until she is.
	for s.enc.ActiveActor() != "char-alice" {
		_, _, err := s.enc.EndTurn(s.enc.ActiveActor())
		s.Require().NoError(err)
	}
	drainSub(s.aliceSub, 100*time.Millisecond)
	drainSub(s.bobSub, 100*time.Millisecond)

	err := s.enc.TakeAction("alice",
		encounter.ActionRef{Module: "dnd5e", Type: "action", ID: "attack"},
		encounter.ActionTarget{EntityID: "goblin-1"},
	)
	s.Require().NoError(err)

	seenAlice := collectTypes(s.aliceSub, 500*time.Millisecond)
	s.Contains(seenAlice, "*events.AttackResolvedEvent")
	// The attack may have hit or missed; both branches should be safe to inspect.
}

// EndTurn publishes TurnEnded + TurnStarted; rotates Initiative.
func (s *CombatSuite) TestEndTurn_AdvancesInitiative() {
	s.Require().NoError(s.enc.SetMode(core.ModeTurnBased))
	first := s.enc.ActiveActor()
	drainSub(s.aliceSub, 100*time.Millisecond)
	drainSub(s.bobSub, 100*time.Millisecond)

	next, _, err := s.enc.EndTurn(first)
	s.Require().NoError(err)
	s.NotEqual(first, next)
	s.Equal(next, s.enc.ActiveActor())

	seen := collectTypes(s.aliceSub, 500*time.Millisecond)
	s.Contains(seen, "*events.TurnEndedEvent")
	s.Contains(seen, "*events.TurnStartedEvent")
}

// EndTurn called by a non-active actor errors with ErrNotYourTurn.
func (s *CombatSuite) TestEndTurn_RejectsWrongActor() {
	s.Require().NoError(s.enc.SetMode(core.ModeTurnBased))
	active := s.enc.ActiveActor()
	other := core.EntityID("char-alice")
	if active == "char-alice" {
		other = "char-bob"
	}
	_, _, err := s.enc.EndTurn(other)
	s.ErrorIs(err, encounter.ErrNotYourTurn)
}

// EndTurn outside TURN_BASED returns ErrNotTurnBased.
func (s *CombatSuite) TestEndTurn_RequiresTurnBased() {
	_, _, err := s.enc.EndTurn("char-alice")
	s.ErrorIs(err, encounter.ErrNotTurnBased)
}

// NPCAct (scripted path — no DataJSON) emits an attack event when a
// player is reachable. We run the full cycle: alice attacks then ends
// turn, ending turn rotates to (probably) bob, bob ends turn rotates to
// goblin-1, NPCAct fires.
func (s *CombatSuite) TestNPCAct_ScriptedAttackPublishes() {
	s.Require().NoError(s.enc.SetMode(core.ModeTurnBased))
	// Cycle initiative until goblin-1 is the active actor.
	for s.enc.ActiveActor() != "goblin-1" {
		_, _, err := s.enc.EndTurn(s.enc.ActiveActor())
		s.Require().NoError(err)
	}
	drainSub(s.aliceSub, 100*time.Millisecond)
	drainSub(s.bobSub, 100*time.Millisecond)

	err := s.enc.NPCAct(s.ctx, "goblin-1")
	s.Require().NoError(err)

	seen := collectTypes(s.aliceSub, 500*time.Millisecond)
	s.Contains(seen, "*events.AttackResolvedEvent")
}

// TakeAction with viewer out of LoS marks PerPlayer Visible: false.
func (s *CombatSuite) TestTakeAction_PerViewerVisibility() {
	// Move bob far away so he can't see alice's attack.
	s.enc = encounter.New("enc-combat-2", s.broker)
	s.Require().NoError(s.enc.AddPlayer(encounter.PlayerInput{
		PlayerID: "alice", EntityID: "char-alice",
		Position: core.Hex{}, SightRange: 10,
		HP: 12, MaxHP: 12, AC: 14, AttackBonus: 4,
		DamageDice: "1d8+2", DamageType: "slashing",
	}))
	s.Require().NoError(s.enc.AddPlayer(encounter.PlayerInput{
		PlayerID: "bob", EntityID: "char-bob",
		Position: core.Hex{Q: 50, R: -25, S: -25}, SightRange: 5,
		HP: 10, MaxHP: 10, AC: 13,
	}))
	s.Require().NoError(s.enc.AddMonster(encounter.MonsterInput{
		ID:       "goblin-2",
		Position: core.Hex{Q: 1, R: 0, S: -1},
		HP:       7, MaxHP: 7, AC: 15,
	}))
	farAliceSub, err := s.broker.Subscribe("enc-combat-2", "alice")
	s.Require().NoError(err)
	defer func() { _ = farAliceSub.Close() }()
	farBobSub, err := s.broker.Subscribe("enc-combat-2", "bob")
	s.Require().NoError(err)
	defer func() { _ = farBobSub.Close() }()

	s.Require().NoError(s.enc.SetMode(core.ModeTurnBased))
	for s.enc.ActiveActor() != "char-alice" {
		_, _, endErr := s.enc.EndTurn(s.enc.ActiveActor())
		s.Require().NoError(endErr)
	}
	drainSub(farAliceSub, 100*time.Millisecond)
	drainSub(farBobSub, 100*time.Millisecond)

	err = s.enc.TakeAction("alice",
		encounter.ActionRef{Module: "dnd5e", Type: "action", ID: "attack"},
		encounter.ActionTarget{EntityID: "goblin-2"},
	)
	s.Require().NoError(err)

	// Alice is in audience and sees Visible: true (she's the attacker).
	// Bob is in audience but Visible: false (out of range).
	aliceEvent := waitForAttackResolved(s.T(), farAliceSub, time.Second)
	s.Require().NotNil(aliceEvent)
	s.True(aliceEvent.PerPlayer["alice"].Visible)
	s.False(aliceEvent.PerPlayer["bob"].Visible)
}

// MonsterData round-trips through ToData / LoadFromData.
func (s *CombatSuite) TestMonsterData_RoundTrips() {
	s.Require().NoError(s.enc.SetMode(core.ModeTurnBased))
	persisted := s.enc.ToData()
	s.Require().Contains(persisted.Monsters, core.EntityID("goblin-1"))

	rehydrated, err := encounter.LoadFromData(persisted, s.broker)
	s.Require().NoError(err)
	s.Equal(core.ModeTurnBased, rehydrated.Mode())
	s.NotEqual(core.EntityID(""), rehydrated.ActiveActor())
}

// Helpers — match patterns from encounter_test.go / integration_test.go.

func (s *CombatSuite) playerIDFor(entityID core.EntityID) core.PlayerID {
	switch entityID {
	case "char-alice":
		return "alice"
	case "char-bob":
		return "bob"
	}
	return ""
}

func waitForAttackResolved(t *testing.T, sub *encounter.Subscription, timeout time.Duration) *events.AttackResolvedEvent {
	t.Helper()
	deadline := time.After(timeout)
	for {
		select {
		case evt, ok := <-sub.Events():
			if !ok {
				return nil
			}
			if ar, ok := evt.(*events.AttackResolvedEvent); ok {
				return ar
			}
		case <-deadline:
			return nil
		}
	}
}
