package encounter_test

import (
	"context"
	"testing"

	"github.com/stretchr/testify/suite"

	toolkitcore "github.com/KirkDiggler/rpg-toolkit/core"
	"github.com/KirkDiggler/rpg-toolkit/encounter"
	"github.com/KirkDiggler/rpg-toolkit/encounter/core"
)

// alwaysHitResolver is a deterministic StrikeResolver test helper that
// always returns a hit with a configurable damage value. Used by combat
// and death tests where the only thing that matters is that an attack
// lands and produces a known damage amount (e.g., enough to kill the
// monster in one hit). Real rulebook chains run in rpg-api integration
// tests, not here.
type alwaysHitResolver struct {
	damage     int
	damageType string

	// hasAdvantage/hasDisadvantage/advantageSources/disadvantageSources let
	// tests configure the canned StrikeOutcome the same way a real resolver
	// would copy them from combat.AttackResult (#726). Zero values (false,
	// nil) preserve every existing caller's behavior.
	hasAdvantage        bool
	hasDisadvantage     bool
	advantageSources    []*toolkitcore.Ref
	disadvantageSources []*toolkitcore.Ref
}

func (r alwaysHitResolver) ResolveStrike(_ encounter.StrikeInput) (*encounter.StrikeOutcome, error) {
	return &encounter.StrikeOutcome{
		Hit:                 true,
		AttackRoll:          20,
		AttackBonus:         4,
		TargetAC:            10,
		Damage:              r.damage,
		DamageType:          r.damageType,
		HasAdvantage:        r.hasAdvantage,
		HasDisadvantage:     r.hasDisadvantage,
		AdvantageSources:    r.advantageSources,
		DisadvantageSources: r.disadvantageSources,
	}, nil
}

// StrikeResolverWiringSuite covers the wiring contract for StrikeResolver
// (the encounter SDK side of the Wave 2.11a integration). The resolver
// implementation itself is exercised through the rpg-api integration tests.
type StrikeResolverWiringSuite struct {
	suite.Suite
	transport *encounter.InMemoryTransport
	broker    *encounter.Broker
}

func TestStrikeResolverWiringSuite(t *testing.T) {
	suite.Run(t, new(StrikeResolverWiringSuite))
}

func (s *StrikeResolverWiringSuite) SetupTest() {
	s.transport = encounter.NewInMemoryTransport()
	s.broker = encounter.NewBroker(s.transport)
}

func (s *StrikeResolverWiringSuite) TearDownTest() {
	_ = s.broker.Close()
	_ = s.transport.Close()
}

// TakeAction returns ErrNoStrikeResolver when no StrikeResolver is
// wired. Production must wire one via WithStrikeResolver; this guards
// against misconfiguration.
func (s *StrikeResolverWiringSuite) TestTakeAction_ErrNoStrikeResolver() {
	enc := encounter.New(context.Background(), "enc-no-resolver", s.broker)
	s.Require().NoError(enc.AddPlayer(encounter.PlayerInput{
		PlayerID: alicePlayerID, EntityID: aliceEntityID,
		Position: core.Hex{}, SightRange: 10,
		HP: 12, MaxHP: 12, AC: 14, AttackBonus: 4,
		DamageDice: damage1d8plus2, DamageType: damageSlashing,
	}))
	s.Require().NoError(enc.AddMonster(encounter.MonsterInput{
		ID: gobEntityID, Position: core.Hex{Q: 1, R: 0, S: -1},
		HP: 7, MaxHP: 7, AC: 15,
		DataJSON: testGoblinDataJSON(s.T(), gobEntityID),
	}))
	// alice and the goblin are in mutual LoS, so AddMonster already
	// auto-transitioned to TURN_BASED; an explicit SetMode here would be
	// redundant and error.
	for enc.ActiveActor() != aliceEntityID {
		_, _, err := enc.EndTurn(context.Background(), enc.ActiveActor())
		s.Require().NoError(err)
	}

	err := enc.TakeAction(alicePlayerID,
		encounter.ActionRef{Module: refModuleDnd5e, Type: refTypeAction, ID: "attack"},
		encounter.ActionTarget{EntityID: gobEntityID},
	)
	s.ErrorIs(err, encounter.ErrNoStrikeResolver)
}

// TakeAction calls the wired StrikeResolver and uses its outcome to
// mutate state and publish events.
func (s *StrikeResolverWiringSuite) TestTakeAction_UsesResolverOutcome() {
	enc := encounter.New(context.Background(), "enc-resolver", s.broker,
		encounter.WithStrikeResolver(alwaysHitResolver{damage: 5, damageType: damageSlashing}),
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
		DataJSON: testGoblinDataJSON(s.T(), gobEntityID),
	}))
	// alice and the goblin are in mutual LoS, so AddMonster already
	// auto-transitioned to TURN_BASED; an explicit SetMode here would be
	// redundant and error.
	for enc.ActiveActor() != aliceEntityID {
		_, _, err := enc.EndTurn(context.Background(), enc.ActiveActor())
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
