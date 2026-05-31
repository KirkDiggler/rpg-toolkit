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

// Test-package fixture identifiers (extracted to satisfy goconst).
const (
	alicePlayerID  = "alice"
	bobPlayerID    = "bob"
	aliceEntityID  = "char-alice"
	bobEntityID    = "char-bob"
	gobEntityID    = "goblin-1"
	gob2EntityID   = "goblin-2"
	damageSlashing = "slashing"
	refModuleDnd5e = "dnd5e"
	refTypeAction  = "action"
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
	s.enc = encounter.New(context.Background(), "enc-combat", s.broker,
		encounter.WithCombatResolver(alwaysHitResolver{damage: 8, damageType: damageSlashing}),
	)

	s.Require().NoError(s.enc.AddPlayer(encounter.PlayerInput{
		PlayerID: "alice", EntityID: aliceEntityID,
		Position: core.Hex{}, SightRange: 10,
		HP: 12, MaxHP: 12, AC: 14, AttackBonus: 4,
		DamageDice: "1d8+2", DamageType: damageSlashing,
	}))
	s.Require().NoError(s.enc.AddPlayer(encounter.PlayerInput{
		PlayerID: "bob", EntityID: bobEntityID,
		Position: core.Hex{Q: 1, R: 0, S: -1}, SightRange: 10,
		HP: 10, MaxHP: 10, AC: 13, AttackBonus: 3,
		DamageDice: "1d6+1", DamageType: damagePiercing,
	}))
	s.Require().NoError(s.enc.AddMonster(encounter.MonsterInput{
		ID:       gobEntityID,
		Position: core.Hex{Q: 1, R: 0, S: -1},
		HP:       7, MaxHP: 7, AC: 15, Speed: 6,
		MonsterRef:  "dnd5e:monsters:goblin",
		AttackBonus: 4, DamageDice: "1d6+2", DamageType: damageSlashing,
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
		encounter.ActionTarget{EntityID: gobEntityID},
	)
	s.ErrorIs(err, encounter.ErrNotTurnBased)
}

// TakeAction by a non-active player returns ErrNotYourTurn.
func (s *CombatSuite) TestTakeAction_RejectedWhenNotYourTurn() {
	s.Require().NoError(s.enc.SetMode(core.ModeTurnBased))
	active := s.enc.ActiveActor()
	// Find the OTHER player and try to act.
	var attackerID core.PlayerID
	if active == aliceEntityID {
		attackerID = "bob"
	} else {
		attackerID = "alice"
	}
	err := s.enc.TakeAction(attackerID,
		encounter.ActionRef{Module: "dnd5e", Type: "action", ID: "attack"},
		encounter.ActionTarget{EntityID: gobEntityID},
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
		encounter.ActionTarget{EntityID: gobEntityID},
	)
	s.ErrorIs(err, encounter.ErrUnsupportedAction)
}

// TakeAction publishes AttackResolvedEvent (always); on hit a
// DamageDealtEvent rides alongside.
func (s *CombatSuite) TestTakeAction_PublishesAttackOutcome() {
	s.Require().NoError(s.enc.SetMode(core.ModeTurnBased))
	for s.enc.ActiveActor() != aliceEntityID {
		_, _, err := s.enc.EndTurn(context.Background(), s.enc.ActiveActor())
		s.Require().NoError(err)
	}
	drainSub(s.aliceSub, 100*time.Millisecond)
	drainSub(s.bobSub, 100*time.Millisecond)

	err := s.enc.TakeAction("alice",
		encounter.ActionRef{Module: "dnd5e", Type: "action", ID: "attack"},
		encounter.ActionTarget{EntityID: gobEntityID},
	)
	s.Require().NoError(err)

	seenAlice := collectTypes(s.aliceSub, 500*time.Millisecond)
	s.Contains(seenAlice, "*events.AttackResolvedEvent")
}

// TakeAction returns ErrNonCombatant when the active player has no
// combat snapshot. Documents the PlayerInput contract: zero combat
// fields opt the seat out of combat verbs.
func (s *CombatSuite) TestTakeAction_RejectsNonCombatant() {
	// Build a fresh encounter with a non-combatant alice.
	enc := encounter.New(context.Background(), "enc-noncomb", s.broker)
	s.Require().NoError(enc.AddPlayer(encounter.PlayerInput{
		PlayerID: "alice", EntityID: aliceEntityID,
		Position: core.Hex{}, SightRange: 10,
		// No HP / AC / DamageDice — non-combatant.
	}))
	s.Require().NoError(enc.AddMonster(encounter.MonsterInput{
		ID: gobEntityID, Position: core.Hex{Q: 1, R: 0, S: -1},
		HP: 7, MaxHP: 7, AC: 15,
	}))
	s.Require().NoError(enc.SetMode(core.ModeTurnBased))
	for enc.ActiveActor() != aliceEntityID {
		_, _, err := enc.EndTurn(context.Background(), enc.ActiveActor())
		s.Require().NoError(err)
	}
	err := enc.TakeAction("alice",
		encounter.ActionRef{Module: "dnd5e", Type: "action", ID: "attack"},
		encounter.ActionTarget{EntityID: gobEntityID},
	)
	s.ErrorIs(err, encounter.ErrNonCombatant)
}

// EndTurn publishes TurnEnded + TurnStarted; rotates Initiative.
func (s *CombatSuite) TestEndTurn_AdvancesInitiative() {
	s.Require().NoError(s.enc.SetMode(core.ModeTurnBased))
	first := s.enc.ActiveActor()
	drainSub(s.aliceSub, 100*time.Millisecond)
	drainSub(s.bobSub, 100*time.Millisecond)

	next, _, err := s.enc.EndTurn(context.Background(), first)
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
	other := core.EntityID(aliceEntityID)
	if active == aliceEntityID {
		other = bobEntityID
	}
	_, _, err := s.enc.EndTurn(context.Background(), other)
	s.ErrorIs(err, encounter.ErrNotYourTurn)
}

// EndTurn outside TURN_BASED returns ErrNotTurnBased.
func (s *CombatSuite) TestEndTurn_RequiresTurnBased() {
	_, _, err := s.enc.EndTurn(context.Background(), aliceEntityID)
	s.ErrorIs(err, encounter.ErrNotTurnBased)
}

// EndTurn returns ErrNoCombatants — does not panic — when initiative
// is empty (e.g. SetMode(TurnBased) flipped on an empty encounter).
// Regression test for the out-of-range panic Copilot flagged in #638.
func (s *CombatSuite) TestEndTurn_GuardsEmptyInitiative() {
	enc := encounter.New(context.Background(), "enc-empty", s.broker)
	// SetMode would normally roll initiative, but with no players or
	// monsters the Initiative slice ends up empty.
	s.Require().NoError(enc.SetMode(core.ModeTurnBased))
	s.Empty(enc.ActiveActor())

	_, _, err := enc.EndTurn(context.Background(), "anyone")
	s.ErrorIs(err, encounter.ErrNoCombatants)
}

// NPCAct (scripted path — no DataJSON) emits an attack event when a
// player is reachable.
func (s *CombatSuite) TestNPCAct_ScriptedAttackPublishes() {
	s.Require().NoError(s.enc.SetMode(core.ModeTurnBased))
	for s.enc.ActiveActor() != gobEntityID {
		_, _, err := s.enc.EndTurn(context.Background(), s.enc.ActiveActor())
		s.Require().NoError(err)
	}
	drainSub(s.aliceSub, 100*time.Millisecond)
	drainSub(s.bobSub, 100*time.Millisecond)

	err := s.enc.NPCAct(s.ctx, gobEntityID)
	s.Require().NoError(err)

	seen := collectTypes(s.aliceSub, 500*time.Millisecond)
	s.Contains(seen, "*events.AttackResolvedEvent")
}

// TakeAction omits non-viewers (out-of-LoS players) from PerPlayer
// entirely so the broker does not deliver to them. Mirrors Move /
// OpenDoor audience-routing.
func (s *CombatSuite) TestTakeAction_OmitsNonViewersFromAudience() {
	enc := encounter.New(context.Background(), "enc-combat-2", s.broker,
		encounter.WithCombatResolver(alwaysHitResolver{damage: 8, damageType: damageSlashing}),
	)
	s.Require().NoError(enc.AddPlayer(encounter.PlayerInput{
		PlayerID: "alice", EntityID: aliceEntityID,
		Position: core.Hex{}, SightRange: 10,
		HP: 12, MaxHP: 12, AC: 14, AttackBonus: 4,
		DamageDice: "1d8+2", DamageType: damageSlashing,
	}))
	s.Require().NoError(enc.AddPlayer(encounter.PlayerInput{
		PlayerID: "bob", EntityID: bobEntityID,
		Position: core.Hex{Q: 50, R: -25, S: -25}, SightRange: 5,
		HP: 10, MaxHP: 10, AC: 13,
	}))
	s.Require().NoError(enc.AddMonster(encounter.MonsterInput{
		ID:       gob2EntityID,
		Position: core.Hex{Q: 1, R: 0, S: -1},
		HP:       7, MaxHP: 7, AC: 15,
	}))
	farAliceSub, err := s.broker.Subscribe("enc-combat-2", "alice")
	s.Require().NoError(err)
	defer func() { _ = farAliceSub.Close() }()
	farBobSub, err := s.broker.Subscribe("enc-combat-2", "bob")
	s.Require().NoError(err)
	defer func() { _ = farBobSub.Close() }()

	s.Require().NoError(enc.SetMode(core.ModeTurnBased))
	for enc.ActiveActor() != aliceEntityID {
		_, _, endErr := enc.EndTurn(context.Background(), enc.ActiveActor())
		s.Require().NoError(endErr)
	}
	drainSub(farAliceSub, 100*time.Millisecond)
	drainSub(farBobSub, 100*time.Millisecond)

	err = enc.TakeAction("alice",
		encounter.ActionRef{Module: "dnd5e", Type: "action", ID: "attack"},
		encounter.ActionTarget{EntityID: gob2EntityID},
	)
	s.Require().NoError(err)

	// Alice can see her attack: she's in PerPlayer (Visible: true) and
	// her subscription delivers.
	aliceEvent := waitForAttackResolved(s.T(), farAliceSub, time.Second)
	s.Require().NotNil(aliceEvent, "alice should receive AttackResolvedEvent")
	s.True(aliceEvent.PerPlayer["alice"].Visible)
	_, bobInAudience := aliceEvent.PerPlayer["bob"]
	s.False(bobInAudience, "bob is out of LoS; he should be omitted from PerPlayer entirely")

	// Bob is out of LoS to both attacker and target — broker should NOT
	// deliver any AttackResolvedEvent to bob's subscription.
	bobEvent := waitForAttackResolved(s.T(), farBobSub, 200*time.Millisecond)
	s.Nil(bobEvent, "bob is out of LoS; no AttackResolvedEvent should be delivered")
}

// MonsterData round-trips through ToData / LoadFromData.
func (s *CombatSuite) TestMonsterData_RoundTrips() {
	s.Require().NoError(s.enc.SetMode(core.ModeTurnBased))
	persisted := s.enc.ToData()
	s.Require().Contains(persisted.Monsters, core.EntityID(gobEntityID))

	rehydrated, err := encounter.LoadFromData(context.Background(), persisted, s.broker)
	s.Require().NoError(err)
	s.Equal(core.ModeTurnBased, rehydrated.Mode())
	s.NotEqual(core.EntityID(""), rehydrated.ActiveActor())
}

// Helpers — match patterns from encounter_test.go / integration_test.go.

func (s *CombatSuite) playerIDFor(entityID core.EntityID) core.PlayerID {
	switch entityID {
	case aliceEntityID:
		return "alice"
	case bobEntityID:
		return "bob"
	}
	return ""
}

func waitForAttackResolved(
	t *testing.T, sub *encounter.Subscription, timeout time.Duration,
) *events.AttackResolvedEvent {
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
