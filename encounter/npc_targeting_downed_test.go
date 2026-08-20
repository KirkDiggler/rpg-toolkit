package encounter_test

// rpg-toolkit#733 — playtest evidence: a goblin kept attacking a corpse
// round after round because nothing filtered NPC targeting by player HP.
// buildPerception (full monster.TakeTurn path) feeds
// monster.PerceptionData.Enemies, which ClosestEnemy()/TargetLowestHP/
// TargetLowestAC all read from, and must skip any player at HP<=0 (dead or
// unconscious) — this file proves it never targets the downed player,
// across multiple turns.
//
// A second call site, closestPlayer (the npcActScripted empty-DataJSON
// fallback path), had its own equivalent test here; rpg-toolkit#895's
// no-fallback rider deleted npcActScripted and closestPlayer along with it.

import (
	"context"
	"encoding/json"
	"testing"
	"time"

	"github.com/stretchr/testify/suite"

	"github.com/KirkDiggler/rpg-toolkit/encounter"
	"github.com/KirkDiggler/rpg-toolkit/encounter/core"
	"github.com/KirkDiggler/rpg-toolkit/encounter/events"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/monster/monsters"
)

const (
	ntdAlivePlayerID = core.PlayerID("ntd-alive")
	ntdAliveEntityID = core.EntityID("char-ntd-alive")
	ntdDownEntityID  = core.EntityID("char-ntd-down")
)

// NPCTargetingDownedSuite covers the #733 targeting-skip fix.
type NPCTargetingDownedSuite struct {
	suite.Suite
	ctx       context.Context
	transport *encounter.InMemoryTransport
	broker    *encounter.Broker
}

func TestNPCTargetingDownedSuite(t *testing.T) {
	suite.Run(t, new(NPCTargetingDownedSuite))
}

func (s *NPCTargetingDownedSuite) SetupTest() {
	s.ctx = context.Background()
	s.transport = encounter.NewInMemoryTransport()
	s.broker = encounter.NewBroker(s.transport)
}

func (s *NPCTargetingDownedSuite) TearDownTest() {
	_ = s.broker.Close()
	_ = s.transport.Close()
}

// TestFullAIPath_BuildPerception_NeverTargetsDownedPlayer is the
// buildPerception regression proof, using the full monster.TakeTurn AI path
// (DataJSON goblin, ClosestEnemy() targeting via ScimitarAction) — the ONLY
// path left after rpg-toolkit#895's no-fallback rider deleted npcActScripted
// and its closestPlayer targeting helper (a second formerly-tested call site,
// TestScriptedPath_ClosestPlayer_NeverTargetsDownedPlayer, went with them).
// The downed player is adjacent (would be
// ClosestEnemy() and in melee range pre-fix); the alive player is farther
// and non-adjacent. This asserts the negative: across the goblin's turn(s),
// it never attacks or damages the downed player. (It does not assert the
// goblin successfully chases down the alive player — that depends on
// movement/pathfinding mechanics unrelated to this fix; only that the downed
// player is structurally excluded from being perceived as a target at all.)
func (s *NPCTargetingDownedSuite) TestFullAIPath_BuildPerception_NeverTargetsDownedPlayer() {
	encID := core.EncounterID("enc-ntd-full-ai")
	enc := encounter.New(s.ctx, encID, s.broker,
		encounter.WithCombatResolver(alwaysHitResolver{damage: 1, damageType: damageSlashing}),
	)
	s.Require().NoError(enc.AddPlayer(encounter.PlayerInput{
		PlayerID: "ntd-down", EntityID: ntdDownEntityID,
		Position: core.Hex{Q: 1, R: 0, S: -1}, SightRange: 10,
		HP: 0, MaxHP: 12, AC: 10,
	}))
	s.Require().NoError(enc.AddPlayer(encounter.PlayerInput{
		PlayerID: ntdAlivePlayerID, EntityID: ntdAliveEntityID,
		Position: core.Hex{Q: 2, R: 0, S: -2}, SightRange: 10,
		HP: 12, MaxHP: 12, AC: 10,
	}))

	gob := monsters.NewGoblin(gobEntityID)
	gobData := gob.ToData()
	dataJSON, err := json.Marshal(gobData)
	s.Require().NoError(err)
	s.Require().NoError(enc.AddMonster(encounter.MonsterInput{
		ID:       gobEntityID,
		Position: core.Hex{},
		HP:       7, MaxHP: 7, AC: 15, Speed: 6,
		MonsterRef:  monsterRefGoblin,
		DataJSON:    dataJSON,
		AttackBonus: 4, DamageDice: damage1d6plus2, DamageType: damageSlashing,
	}))

	aliceSub, err := s.broker.Subscribe(encID, ntdAlivePlayerID)
	s.Require().NoError(err)
	defer func() { _ = aliceSub.Close() }()

	// Both players are in LoS of the goblin, so AddMonster already
	// auto-transitioned to TURN_BASED; an explicit SetMode here would be
	// redundant and error.

	for turn := 0; turn < 2; turn++ {
		for enc.ActiveActor() != gobEntityID {
			_, _, endErr := enc.EndTurn(s.ctx, enc.ActiveActor())
			s.Require().NoError(endErr)
		}
		drainSub(aliceSub, 50*time.Millisecond)

		s.Require().NoError(enc.NPCAct(s.ctx, gobEntityID))

		seen := collectEventsTyped(aliceSub, 300*time.Millisecond)
		for _, e := range seen {
			switch ev := e.(type) {
			case *events.AttackResolvedEvent:
				s.NotEqual(ntdDownEntityID, ev.TargetID,
					"turn %d: goblin must never attack the downed player (buildPerception HP filter)", turn)
			case *events.DamageDealtEvent:
				s.NotEqual(ntdDownEntityID, ev.TargetID,
					"turn %d: goblin must never damage the downed player (buildPerception HP filter)", turn)
			}
		}

		_, _, endErr := enc.EndTurn(s.ctx, gobEntityID)
		s.Require().NoError(endErr)
	}

	// The downed player's HP must be completely untouched.
	s.Equal(0, enc.ToData().Players["ntd-down"].HP)
}
