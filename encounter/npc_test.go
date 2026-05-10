package encounter_test

import (
	"context"
	"encoding/json"
	"testing"
	"time"

	"github.com/stretchr/testify/suite"

	"github.com/KirkDiggler/rpg-toolkit/encounter"
	"github.com/KirkDiggler/rpg-toolkit/encounter/core"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/monster"
)

// monsterRefGoblin is the canonical monster ref used across NPC fixtures.
const monsterRefGoblin = "dnd5e:monsters:goblin"

// NPCSuite covers the full NPCAct path through monster.TakeTurn —
// distinct from CombatSuite which uses the scripted DataJSON-less path.
type NPCSuite struct {
	suite.Suite
	ctx       context.Context
	transport *encounter.InMemoryTransport
	broker    *encounter.Broker
	enc       *encounter.Encounter
	aliceSub  *encounter.Subscription
}

func TestNPCSuite(t *testing.T) {
	suite.Run(t, new(NPCSuite))
}

func (s *NPCSuite) SetupTest() {
	s.ctx = context.Background()
	s.transport = encounter.NewInMemoryTransport()
	s.broker = encounter.NewBroker(s.transport)
	s.enc = encounter.New("enc-npc", s.broker,
		encounter.WithCombatResolver(alwaysHitResolver{damage: 4, damageType: damageSlashing}),
	)

	// Build a goblin and serialize it.
	gob := monster.NewGoblin(gobEntityID)
	gobData := gob.ToData()
	dataJSON, err := json.Marshal(gobData)
	s.Require().NoError(err)

	// Alice adjacent to goblin so the goblin can attack on its turn.
	s.Require().NoError(s.enc.AddPlayer(encounter.PlayerInput{
		PlayerID: alicePlayerID, EntityID: aliceEntityID,
		Position: core.Hex{}, SightRange: 10,
		HP: 12, MaxHP: 12, AC: 14,
	}))
	s.Require().NoError(s.enc.AddMonster(encounter.MonsterInput{
		ID:       gobEntityID,
		Position: core.Hex{Q: 1, R: 0, S: -1},
		HP:       7, MaxHP: 7, AC: 15, Speed: 6,
		MonsterRef:  monsterRefGoblin,
		DataJSON:    dataJSON,
		AttackBonus: 4, DamageDice: damage1d6plus2, DamageType: damageSlashing,
	}))

	s.aliceSub, err = s.broker.Subscribe("enc-npc", alicePlayerID)
	s.Require().NoError(err)
}

func (s *NPCSuite) TearDownTest() {
	if s.aliceSub != nil {
		_ = s.aliceSub.Close()
	}
	_ = s.broker.Close()
	_ = s.transport.Close()
}

// NPCAct via monster.TakeTurn captures the dnd5e AttackEvent the goblin's
// scimitar action emits and re-publishes it as an encounter
// AttackResolvedEvent.
func (s *NPCSuite) TestNPCAct_GoblinTakeTurn_PublishesAttack() {
	s.Require().NoError(s.enc.SetMode(core.ModeTurnBased))
	for s.enc.ActiveActor() != gobEntityID {
		_, _, err := s.enc.EndTurn(s.enc.ActiveActor())
		s.Require().NoError(err)
	}
	drainSub(s.aliceSub, 100*time.Millisecond)

	err := s.enc.NPCAct(s.ctx, gobEntityID)
	s.Require().NoError(err)

	seen := collectTypes(s.aliceSub, time.Second)
	s.Contains(seen, "*events.AttackResolvedEvent")
}

// NPCAct outside TURN_BASED returns ErrNotTurnBased.
func (s *NPCSuite) TestNPCAct_RequiresTurnBased() {
	err := s.enc.NPCAct(s.ctx, gobEntityID)
	s.ErrorIs(err, encounter.ErrNotTurnBased)
}

// NPCAct against an unknown id returns ErrNotYourTurn (the active-actor
// check fires before the existence check).
func (s *NPCSuite) TestNPCAct_RejectsUnknownNPC() {
	s.Require().NoError(s.enc.SetMode(core.ModeTurnBased))
	// Cycle to goblin to satisfy the active-actor gate, then try to act
	// for a non-existent npc — the test exercises ErrNotYourTurn (the
	// active-actor check fires before the existence check).
	for s.enc.ActiveActor() != gobEntityID {
		_, _, err := s.enc.EndTurn(s.enc.ActiveActor())
		s.Require().NoError(err)
	}
	err := s.enc.NPCAct(s.ctx, "ghost")
	s.ErrorIs(err, encounter.ErrNotYourTurn)
}

// NPCAct returns ErrNoCombatResolver when no CombatResolver is wired.
// Mirrors the guard on TakeAction (player path) — production must wire one
// via WithCombatResolver.
func (s *NPCSuite) TestNPCAct_ErrNoCombatResolver() {
	gob := monster.NewGoblin(gobEntityID)
	gobData := gob.ToData()
	dataJSON, err := json.Marshal(gobData)
	s.Require().NoError(err)

	enc := encounter.New("enc-npc-no-resolver", s.broker)
	s.Require().NoError(enc.AddPlayer(encounter.PlayerInput{
		PlayerID: alicePlayerID, EntityID: aliceEntityID,
		Position: core.Hex{}, SightRange: 10,
		HP: 12, MaxHP: 12, AC: 14,
	}))
	s.Require().NoError(enc.AddMonster(encounter.MonsterInput{
		ID:       gobEntityID,
		Position: core.Hex{Q: 1, R: 0, S: -1},
		HP:       7, MaxHP: 7, AC: 15, Speed: 6,
		MonsterRef: monsterRefGoblin,
		DataJSON:   dataJSON,
	}))
	s.Require().NoError(enc.SetMode(core.ModeTurnBased))
	for enc.ActiveActor() != gobEntityID {
		_, _, endErr := enc.EndTurn(enc.ActiveActor())
		s.Require().NoError(endErr)
	}

	err = enc.NPCAct(s.ctx, gobEntityID)
	s.ErrorIs(err, encounter.ErrNoCombatResolver)
}

// NPCAct (scripted path — no DataJSON) returns ErrNoCombatResolver when
// no resolver is wired.
func (s *NPCSuite) TestNPCAct_Scripted_ErrNoCombatResolver() {
	enc := encounter.New("enc-npc-scripted-no-resolver", s.broker)
	s.Require().NoError(enc.AddPlayer(encounter.PlayerInput{
		PlayerID: alicePlayerID, EntityID: aliceEntityID,
		Position: core.Hex{}, SightRange: 10,
		HP: 12, MaxHP: 12, AC: 14,
	}))
	// No DataJSON — triggers the scripted path.
	s.Require().NoError(enc.AddMonster(encounter.MonsterInput{
		ID:       gobEntityID,
		Position: core.Hex{Q: 1, R: 0, S: -1},
		HP:       7, MaxHP: 7, AC: 15, Speed: 6,
		AttackBonus: 4, DamageDice: damage1d6plus2, DamageType: damageSlashing,
	}))
	s.Require().NoError(enc.SetMode(core.ModeTurnBased))
	for enc.ActiveActor() != gobEntityID {
		_, _, endErr := enc.EndTurn(enc.ActiveActor())
		s.Require().NoError(endErr)
	}

	err := enc.NPCAct(s.ctx, gobEntityID)
	s.ErrorIs(err, encounter.ErrNoCombatResolver)
}
