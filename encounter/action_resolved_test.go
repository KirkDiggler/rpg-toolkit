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

// ActionResolvedSuite proves the resolved-action event spine on the REAL
// TakeAction path (broker subscription, not a stub): every action emits a
// first-class ActionResolvedEvent (Inv 9) carrying action_ref + economy
// consumed, the attack's effect events share its correlation id (Inv 8), and
// the broker stamps a deterministic game-event time (Inv 5).
type ActionResolvedSuite struct {
	suite.Suite
	clockAt   time.Time
	transport *encounter.InMemoryTransport
	broker    *encounter.Broker
	enc       *encounter.Encounter
	aliceSub  *encounter.Subscription
}

func TestActionResolvedSuite(t *testing.T) {
	suite.Run(t, new(ActionResolvedSuite))
}

func (s *ActionResolvedSuite) SetupTest() {
	s.clockAt = time.Date(2026, 6, 1, 15, 4, 5, 0, time.UTC)
	s.transport = encounter.NewInMemoryTransport()
	// Inject a deterministic clock so OccurredAt is asserted exactly.
	s.broker = encounter.NewBrokerWithClock(s.transport, core.FixedClock{At: s.clockAt})
	s.enc = encounter.New(context.Background(), "enc-ar", s.broker,
		encounter.WithCombatResolver(alwaysHitResolver{damage: 8, damageType: damageSlashing}),
	)

	s.Require().NoError(s.enc.AddPlayer(encounter.PlayerInput{
		PlayerID: alicePlayerID, EntityID: aliceEntityID,
		Position: core.Hex{}, SightRange: 10,
		HP: 12, MaxHP: 12, AC: 14, AttackBonus: 4,
		DamageDice: damage1d8plus2, DamageType: damageSlashing,
	}))
	s.Require().NoError(s.enc.AddMonster(encounter.MonsterInput{
		ID:       gobEntityID,
		Position: core.Hex{Q: 1, R: 0, S: -1},
		HP:       7, MaxHP: 7, AC: 15, Speed: 6,
		MonsterRef:  monsterRefGoblin,
		AttackBonus: 4, DamageDice: "1d6+2", DamageType: damageSlashing,
	}))

	var err error
	s.aliceSub, err = s.broker.Subscribe("enc-ar", alicePlayerID)
	s.Require().NoError(err)
}

func (s *ActionResolvedSuite) TearDownTest() {
	if s.aliceSub != nil {
		_ = s.aliceSub.Close()
	}
	_ = s.broker.Close()
	_ = s.transport.Close()
}

// A real TakeAction attack emits ActionResolved + AttackResolved + DamageDealt;
// all three share one correlation id, the ActionResolved carries action_ref +
// economy_consumed, and every event is stamped with the fixed clock time.
func (s *ActionResolvedSuite) TestTakeAction_EmitsCorrelatedResolvedAction() {
	s.Require().NoError(s.enc.SetMode(core.ModeTurnBased))
	for s.enc.ActiveActor() != aliceEntityID {
		_, _, err := s.enc.EndTurn(context.Background(), s.enc.ActiveActor())
		s.Require().NoError(err)
	}
	drainSub(s.aliceSub, 100*time.Millisecond)

	err := s.enc.TakeAction(alicePlayerID,
		encounter.ActionRef{Module: refModuleDnd5e, Type: refTypeAction, ID: actionIDAttackTest},
		encounter.ActionTarget{EntityID: gobEntityID},
	)
	s.Require().NoError(err)

	evts := collectEventsTyped(s.aliceSub, 500*time.Millisecond)

	var (
		action *events.ActionResolvedEvent
		attack *events.AttackResolvedEvent
		damage *events.DamageDealtEvent
	)
	for _, e := range evts {
		switch ev := e.(type) {
		case *events.ActionResolvedEvent:
			action = ev
		case *events.AttackResolvedEvent:
			attack = ev
		case *events.DamageDealtEvent:
			damage = ev
		}
	}

	s.Require().NotNil(action, "ActionResolvedEvent must be emitted for every action (Inv 9)")
	s.Require().NotNil(attack, "AttackResolvedEvent rides alongside")
	s.Require().NotNil(damage, "alwaysHitResolver hits, so DamageDealtEvent rides alongside")

	// Inv 9: the resolved-action event carries action_ref + economy_consumed.
	s.Equal(core.EntityID(aliceEntityID), action.ActorID)
	s.Equal("dnd5e:action:attack", action.ActionRef)
	s.Equal(core.EntityID(gobEntityID), action.TargetID)
	s.Equal(1, action.EconomyConsumed.Actions)

	// Inv 8: the cause event and its effects share one correlation id.
	s.NotEmpty(action.CorrelationID(), "resolved-action event must carry a correlation id")
	s.Equal(action.CorrelationID(), attack.CorrelationID(),
		"attack-resolved must share the action's correlation id")
	s.Equal(action.CorrelationID(), damage.CorrelationID(),
		"damage-dealt must share the action's correlation id")

	// Inv 5: the broker stamps game-event time at publish from the clock.
	s.Equal(s.clockAt, action.OccurredAt())
	s.Equal(s.clockAt, attack.OccurredAt())
	s.Equal(s.clockAt, damage.OccurredAt())

	// The resolved-action event leads its effects in sequence (cause first).
	s.Less(action.Sequence(), attack.Sequence())
	s.Less(attack.Sequence(), damage.Sequence())
}
