package encounter_test

// rpg-toolkit#733 — playtest evidence: a goblin kept attacking a corpse
// round after round because nothing filtered NPC targeting by player HP.
// Two call sites feed a monster's targeting:
//   - closestPlayer (scripted path, no monster DataJSON) — exercised here via
//     npcActScripted / NPCAct directly.
//   - buildPerception (full monster.TakeTurn path, DataJSON present) — feeds
//     monster.PerceptionData.Enemies, which ClosestEnemy()/TargetLowestHP/
//     TargetLowestAC all read from.
//
// Both must skip any player at HP<=0 (dead or unconscious) — this file
// proves both never target the downed player, across multiple turns.

import (
	"context"
	"encoding/json"
	"testing"
	"time"

	"github.com/stretchr/testify/suite"

	"github.com/KirkDiggler/rpg-toolkit/encounter"
	"github.com/KirkDiggler/rpg-toolkit/encounter/core"
	"github.com/KirkDiggler/rpg-toolkit/encounter/events"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/monster"
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

// TestScriptedPath_ClosestPlayer_NeverTargetsDownedPlayer is the closestPlayer
// regression proof, reproducing the exact reported soft-lock: a downed player
// positioned CLOSER to the goblin than the alive player. npcActScripted has
// no adjacency gate of its own (unlike the DataJSON/Scimitar path's
// MeleeAction.CanActivate) — pre-#733-fix this deterministically attacks the
// corpse every single turn. Across 3 goblin turns, the fix must make the
// goblin attack ONLY the live player, never the downed one.
//
// rpg-toolkit#864: npcActScripted now also gates on reach (the shared
// range/reach validation every actor's attack path goes through), so the
// live player must be positioned within melee reach of the goblin, not just
// "farther than the downed one" — the downed player sits on the goblin's own
// hex (distance 0, the closest possible) and the live player one hex away
// (distance 1, still in reach), preserving "downed is strictly closer" while
// keeping the correct target attackable.
func (s *NPCTargetingDownedSuite) TestScriptedPath_ClosestPlayer_NeverTargetsDownedPlayer() {
	encID := core.EncounterID("enc-ntd-scripted")
	enc := encounter.New(s.ctx, encID, s.broker,
		encounter.WithCombatResolver(alwaysHitResolver{damage: 1, damageType: damageSlashing}),
	)
	// Downed player co-located with the goblin (distance 0) — strictly
	// closer than the alive player.
	s.Require().NoError(enc.AddPlayer(encounter.PlayerInput{
		PlayerID: "ntd-down", EntityID: ntdDownEntityID,
		Position: core.Hex{}, SightRange: 10,
		HP: 0, MaxHP: 12, AC: 10,
	}))
	// Alive player is one hex away (distance 1) — farther than the downed
	// player, but still within the goblin's default melee reach.
	s.Require().NoError(enc.AddPlayer(encounter.PlayerInput{
		PlayerID: ntdAlivePlayerID, EntityID: ntdAliveEntityID,
		Position: core.Hex{Q: 1, R: 0, S: -1}, SightRange: 10,
		HP: 12, MaxHP: 12, AC: 10,
	}))
	// No DataJSON — triggers the scripted (closestPlayer) path.
	s.Require().NoError(enc.AddMonster(encounter.MonsterInput{
		ID: gobEntityID, Position: core.Hex{},
		HP: 7, MaxHP: 7, AC: 15, Speed: 6,
		AttackBonus: 4, DamageDice: damage1d6plus2, DamageType: damageSlashing,
	}))

	aliceSub, err := s.broker.Subscribe(encID, ntdAlivePlayerID)
	s.Require().NoError(err)
	defer func() { _ = aliceSub.Close() }()

	// The downed player is in LoS of the goblin, so AddMonster already
	// auto-transitioned to TURN_BASED; an explicit SetMode here would be
	// redundant and error.

	for turn := 0; turn < 3; turn++ {
		for enc.ActiveActor() != gobEntityID {
			_, _, endErr := enc.EndTurn(s.ctx, enc.ActiveActor())
			s.Require().NoError(endErr)
		}
		drainSub(aliceSub, 50*time.Millisecond)

		s.Require().NoError(enc.NPCAct(s.ctx, gobEntityID))

		seen := collectEventsTyped(aliceSub, 300*time.Millisecond)
		var attack *events.AttackResolvedEvent
		for _, e := range seen {
			if a, ok := e.(*events.AttackResolvedEvent); ok {
				attack = a
			}
		}
		s.Require().NotNil(attack, "turn %d: goblin must attack someone", turn)
		s.Equal(ntdAliveEntityID, attack.TargetID,
			"turn %d: goblin must target the live player, never the downed one (closestPlayer HP filter)", turn)

		_, _, endErr := enc.EndTurn(s.ctx, gobEntityID)
		s.Require().NoError(endErr)
	}
}

// TestFullAIPath_BuildPerception_NeverTargetsDownedPlayer is the
// buildPerception regression proof, using the full monster.TakeTurn AI path
// (DataJSON goblin, ClosestEnemy() targeting via ScimitarAction) rather than
// the scripted stand-in. The downed player is adjacent (would be
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

	gob := monster.NewGoblin(gobEntityID)
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
