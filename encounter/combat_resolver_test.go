package encounter_test

import (
	"testing"

	"github.com/stretchr/testify/suite"

	"github.com/KirkDiggler/rpg-toolkit/encounter"
	"github.com/KirkDiggler/rpg-toolkit/encounter/core"
)

// alwaysHitResolver is a deterministic CombatResolver test helper that
// always returns a hit with a configurable damage value. Used by combat
// and death tests where the only thing that matters is that an attack
// lands and produces a known damage amount (e.g., enough to kill the
// monster in one hit). Real rulebook chains run in rpg-api integration
// tests, not here.
type alwaysHitResolver struct {
	damage     int
	damageType string
}

func (r alwaysHitResolver) ResolveAttack(_ encounter.AttackInput) (*encounter.AttackOutcome, error) {
	return &encounter.AttackOutcome{
		Hit:         true,
		AttackRoll:  20,
		AttackBonus: 4,
		TargetAC:    10,
		Damage:      r.damage,
		DamageType:  r.damageType,
	}, nil
}

// CombatResolverWiringSuite covers the wiring contract for CombatResolver
// (the encounter SDK side of the Wave 2.11a integration). The resolver
// implementation itself is exercised through the rpg-api integration tests.
type CombatResolverWiringSuite struct {
	suite.Suite
	transport *encounter.InMemoryTransport
	broker    *encounter.Broker
}

func TestCombatResolverWiringSuite(t *testing.T) {
	suite.Run(t, new(CombatResolverWiringSuite))
}

func (s *CombatResolverWiringSuite) SetupTest() {
	s.transport = encounter.NewInMemoryTransport()
	s.broker = encounter.NewBroker(s.transport)
}

func (s *CombatResolverWiringSuite) TearDownTest() {
	_ = s.broker.Close()
	_ = s.transport.Close()
}

// TakeAction returns ErrNoCombatResolver when no CombatResolver is
// wired. Production must wire one via WithCombatResolver; this guards
// against misconfiguration.
func (s *CombatResolverWiringSuite) TestTakeAction_ErrNoCombatResolver() {
	enc := encounter.New("enc-no-resolver", s.broker)
	s.Require().NoError(enc.AddPlayer(encounter.PlayerInput{
		PlayerID: alicePlayerID, EntityID: aliceEntityID,
		Position: core.Hex{}, SightRange: 10,
		HP: 12, MaxHP: 12, AC: 14, AttackBonus: 4,
		DamageDice: damage1d8plus2, DamageType: damageSlashing,
	}))
	s.Require().NoError(enc.AddMonster(encounter.MonsterInput{
		ID: gobEntityID, Position: core.Hex{Q: 1, R: 0, S: -1},
		HP: 7, MaxHP: 7, AC: 15,
	}))
	s.Require().NoError(enc.SetMode(core.ModeTurnBased))
	for enc.ActiveActor() != aliceEntityID {
		_, _, err := enc.EndTurn(enc.ActiveActor())
		s.Require().NoError(err)
	}

	err := enc.TakeAction(alicePlayerID,
		encounter.ActionRef{Module: refModuleDnd5e, Type: refTypeAction, ID: "attack"},
		encounter.ActionTarget{EntityID: gobEntityID},
	)
	s.ErrorIs(err, encounter.ErrNoCombatResolver)
}

// TakeAction calls the wired CombatResolver and uses its outcome to
// mutate state and publish events.
func (s *CombatResolverWiringSuite) TestTakeAction_UsesResolverOutcome() {
	enc := encounter.New("enc-resolver", s.broker,
		encounter.WithCombatResolver(alwaysHitResolver{damage: 5, damageType: damageSlashing}),
	)
	s.Require().NoError(enc.AddPlayer(encounter.PlayerInput{
		PlayerID: alicePlayerID, EntityID: aliceEntityID,
		Position: core.Hex{}, SightRange: 10,
		HP: 12, MaxHP: 12, AC: 14, AttackBonus: 4,
		DamageDice: damage1d8plus2, DamageType: damageSlashing,
	}))
	s.Require().NoError(enc.AddMonster(encounter.MonsterInput{
		ID: gobEntityID, Position: core.Hex{Q: 1, R: 0, S: -1},
		HP: 7, MaxHP: 7, AC: 15,
	}))
	s.Require().NoError(enc.SetMode(core.ModeTurnBased))
	for enc.ActiveActor() != aliceEntityID {
		_, _, err := enc.EndTurn(enc.ActiveActor())
		s.Require().NoError(err)
	}

	s.Require().NoError(enc.TakeAction(alicePlayerID,
		encounter.ActionRef{Module: refModuleDnd5e, Type: refTypeAction, ID: "attack"},
		encounter.ActionTarget{EntityID: gobEntityID},
	))

	// Resolver dealt 5 damage; goblin started at 7 HP → should be at 2.
	persisted := enc.ToData()
	s.Equal(2, persisted.Monsters[gobEntityID].HP)
}
