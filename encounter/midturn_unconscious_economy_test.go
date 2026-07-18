package encounter_test

// rpg-toolkit#781 — "unconscious PCs take full turns, inconsistently across
// rounds ... landed a killing blow while unconscious."
//
// Root cause: seedActorTurn (turn_economy.go) correctly zeroes a player's
// action economy when their OWN turn starts and they are already at 0 HP —
// see turn_economy_downed_test.go. But nothing re-checked economy when a
// player instead went unconscious MID-turn, after their economy was already
// seeded in full. The concrete vector: a player's own Move triggers an
// opportunity attack (combat.MoveEntity -> triggerOpportunityAttack ->
// ResolveAttack) that drops them to 0 HP partway through their turn.
// applyUnconsciousOnZeroHP (death.go) applies the Unconscious condition but,
// pre-fix, left the player's already-seeded 1/1/1/30 economy untouched —
// they could still call TakeAction and land an attack after going down.
// This file proves the fix: going unconscious while ACTIVE zeroes the
// remainder of that turn's economy immediately (reusing char.EndTurn, the
// same call seedActorTurn's own downed-at-turn-start branch uses).

import (
	"context"
	"encoding/json"
	"testing"

	"github.com/stretchr/testify/suite"

	"github.com/KirkDiggler/rpg-toolkit/encounter"
	"github.com/KirkDiggler/rpg-toolkit/encounter/core"
	dnd5events "github.com/KirkDiggler/rpg-toolkit/events"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/abilities"
	dnd5eCharacter "github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/character"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/classes"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/races"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/shared"
)

const (
	muePlayerID = core.PlayerID("mue-alice")
	mueEntityID = core.EntityID("char-mue-alice")
	mueGoblinID = core.EntityID("mue-goblin")
)

// MidTurnUnconsciousEconomySuite proves the #781 fix.
type MidTurnUnconsciousEconomySuite struct {
	suite.Suite
	ctx       context.Context
	transport *encounter.InMemoryTransport
	broker    *encounter.Broker
	resolver  *stubMovementResolver
	enc       *encounter.Encounter
}

func TestMidTurnUnconsciousEconomySuite(t *testing.T) {
	suite.Run(t, new(MidTurnUnconsciousEconomySuite))
}

// aliceLowHPSeededCharDataJSON builds a hydratable Fighter at 5 HP with a
// pre-seeded full ActionEconomy (1/1/1/30) — simulating a character mid-turn,
// about to be knocked down by an OA before they've spent everything.
func (s *MidTurnUnconsciousEconomySuite) aliceLowHPSeededCharDataJSON() json.RawMessage {
	s.T().Helper()
	data := &dnd5eCharacter.Data{
		ID:               string(mueEntityID),
		PlayerID:         string(muePlayerID),
		Name:             string(mueEntityID),
		Level:            3,
		ProficiencyBonus: 2,
		ClassID:          classes.Fighter,
		RaceID:           races.Human,
		AbilityScores: shared.AbilityScores{
			abilities.STR: 14, abilities.DEX: 12, abilities.CON: 14,
			abilities.INT: 10, abilities.WIS: 10, abilities.CHA: 10,
		},
		HitPoints:    5,
		MaxHitPoints: 20,
		ArmorClass:   10,
		ActionEconomy: &dnd5eCharacter.ActionEconomyData{
			TurnNumber: 1, ActionsRemaining: 1, BonusActionsRemaining: 1,
			ReactionsRemaining: 1, MovementRemaining: 30,
		},
	}
	raw, err := json.Marshal(data)
	s.Require().NoError(err)
	return raw
}

func (s *MidTurnUnconsciousEconomySuite) SetupTest() {
	s.ctx = context.Background()
	s.transport = encounter.NewInMemoryTransport()
	s.broker = encounter.NewBroker(s.transport)
	s.resolver = &stubMovementResolver{}

	s.enc = encounter.New(s.ctx, "enc-mue-1", s.broker,
		encounter.WithMovementResolver(s.resolver),
		encounter.WithCombatResolver(alwaysHitResolver{damage: 999, damageType: damageSlashing}))
	s.Require().NoError(s.enc.AddPlayer(encounter.PlayerInput{
		PlayerID: muePlayerID, EntityID: mueEntityID,
		Position: core.Hex{}, SightRange: 10,
		HP: 5, MaxHP: 20, AC: 10, AttackBonus: 4,
		DamageDice: damage1d8plus2, DamageType: damageSlashing,
		DataJSON: s.aliceLowHPSeededCharDataJSON(),
	}))
	s.Require().NoError(s.enc.AddMonster(encounter.MonsterInput{
		ID: mueGoblinID, Position: core.Hex{Q: 5, R: 0, S: -5},
		HP: 7, MaxHP: 7, AC: 15, Speed: 6,
		AttackBonus: 4, DamageDice: damage1d6plus2, DamageType: damageSlashing,
	}))

	// Round-trip through Data so the hydration cascade holds alice's
	// character on the live bus (mirrors every other suite's fixture
	// pattern — LoadFromData is the only place conditions Apply()).
	raw, err := json.Marshal(s.enc.ToData())
	s.Require().NoError(err)
	var data encounter.Data
	s.Require().NoError(json.Unmarshal(raw, &data))
	loaded, err := encounter.LoadFromData(s.ctx, &data, s.broker,
		encounter.WithMovementResolver(s.resolver),
		encounter.WithCombatResolver(alwaysHitResolver{damage: 999, damageType: damageSlashing}))
	s.Require().NoError(err)
	s.enc = loaded

	// alice's DataJSON carries a pre-seeded economy, so cycling to her turn
	// doesn't depend on SetMode's seedActorTurn call to populate it. She and
	// the goblin are far enough apart that AddMonster (above, before the
	// round-trip) didn't auto-enter combat — flip explicitly.
	if s.enc.Mode() != core.ModeTurnBased {
		s.Require().NoError(s.enc.SetMode(core.ModeTurnBased))
	}
	for s.enc.ActiveActor() != mueEntityID {
		_, _, err := s.enc.EndTurn(s.ctx, s.enc.ActiveActor())
		s.Require().NoError(err)
	}
}

func (s *MidTurnUnconsciousEconomySuite) TearDownTest() {
	_ = s.broker.Close()
	_ = s.transport.Close()
}

// TestOADropsActivePlayerToZero_ZeroesRemainingEconomy is the goal-behavior
// proof: alice, mid-own-turn with a full pre-seeded economy, takes 6 OA
// damage on step 0 of her own Move (5 HP -> 0), goes Unconscious, and her
// action economy for the rest of THIS turn is zeroed — a subsequent
// TakeAction must fail, not resolve a "killing blow while unconscious."
func (s *MidTurnUnconsciousEconomySuite) TestOADropsActivePlayerToZero_ZeroesRemainingEconomy() {
	s.resolver.publishOnStep = func(bus dnd5events.EventBus, stepIdx int) {
		if stepIdx == 0 {
			publishOAHit(bus, string(mueGoblinID), string(mueEntityID), 6)
		}
	}

	path := []core.Hex{{Q: 1, R: 0, S: -1}}
	// Move must still succeed: the mover physically traveled to where she
	// was standing when the OA landed, then went unconscious — encounter.go
	// skips the now-zeroed movement spend for an incapacitated mover rather
	// than surfacing a confusing ErrInsufficientMovement over a move (and
	// the damage/Unconscious application) that demonstrably did happen.
	s.Require().NoError(s.enc.Move(muePlayerID, path))

	aliceAfter := s.enc.ToData().Players[muePlayerID]
	s.Require().NotNil(aliceAfter)
	s.Equal(0, aliceAfter.HP, "OA damage must have dropped alice to 0 HP")

	err := s.enc.TakeAction(muePlayerID,
		encounter.ActionRef{Module: refModuleDnd5e, Type: refTypeAction, ID: "attack"},
		encounter.ActionTarget{EntityID: mueGoblinID},
	)
	s.Require().Error(err, "alice must not be able to land an attack after going unconscious mid-turn")
	s.ErrorIs(err, encounter.ErrActionUnaffordable,
		"the rejection must be economy-based (zeroed on the unconscious transition), not some other failure")
}

// TestAliveActivePlayer_UnaffectedByFix is the negative control: an OA hit
// that does NOT drop the mover to 0 HP leaves their economy untouched — the
// fix only fires on the >0 -> 0 transition, matching applyUnconsciousOnZeroHP
// everywhere else in this package.
func (s *MidTurnUnconsciousEconomySuite) TestAliveActivePlayer_UnaffectedByFix() {
	s.resolver.publishOnStep = func(bus dnd5events.EventBus, stepIdx int) {
		if stepIdx == 0 {
			publishOAHit(bus, string(mueGoblinID), string(mueEntityID), 2)
		}
	}

	path := []core.Hex{{Q: 1, R: 0, S: -1}}
	s.Require().NoError(s.enc.Move(muePlayerID, path))

	aliceAfter := s.enc.ToData().Players[muePlayerID]
	s.Require().NotNil(aliceAfter)
	s.Equal(3, aliceAfter.HP, "alice should have taken 2 damage and still be alive (5 -> 3)")

	err := s.enc.TakeAction(muePlayerID,
		encounter.ActionRef{Module: refModuleDnd5e, Type: refTypeAction, ID: "attack"},
		encounter.ActionTarget{EntityID: mueGoblinID},
	)
	s.Require().NoError(err, "a player who survives an OA keeps their turn's economy untouched")
}
