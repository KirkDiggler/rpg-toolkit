package encounter_test

// pocket_clear_move_teardown_test.go is the regression gate for
// rpg-toolkit#808: FREE_ROAM movement was wrongly rejected with
// ErrInsufficientMovement after a combat pocket cleared.
//
// Root cause: checkPocketCleared (combat.go, rpg-toolkit#794) exits a
// cleared pocket back to FREE_ROAM via SetMode(ModeFreeRoam), but never
// tore down the held players' combat economy the way the TERMINAL
// encounter-end path already does (endCombatForPlayers's ExitCombat call,
// death.go, rpg-toolkit#767). Move's budget gate (encounter.go) keys off
// char.InCombat() (actionEconomy != nil), not the encounter's mode, so a
// player who was mid-fight when their pocket cleared kept InCombat()==true
// with whatever MovementRemaining they had left, and the next Move was
// wrongly gated even though free-roam movement is meant to be unmetered.
//
// combat_pockets_test.go (#794's own gate) never caught this: its player
// (alice) is a flat stat-snapshot seat with no hydrated
// *character.Character, so InCombat() is never true for her regardless.
// These tests use a HYDRATED, round-tripped-through-LoadFromData player
// (loadRagingBarbVsGoblin's technique, encounter_end_condition_sweep_test.go)
// so the movement gate has real economy state to misbehave with.
//
// Geometry: reuses doors_slice1_test.go's lineHex(k) convention (k maps to
// {X:0, Y:k}), with the door pushed out to k=10 (rather than #794's k=3) so
// there is ample room (k=0..9) to spend movement freely without touching
// the still-closed door — this suite only opens it in the re-entry test.

import (
	"context"
	"encoding/json"
	"testing"

	"github.com/stretchr/testify/suite"

	"github.com/KirkDiggler/rpg-toolkit/encounter"
	"github.com/KirkDiggler/rpg-toolkit/encounter/core"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/abilities"
	dnd5eCharacter "github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/character"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/classes"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/races"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/shared"
	"github.com/KirkDiggler/rpg-toolkit/tools/environments"
)

const (
	pcmPlayerID = core.PlayerID("erin")
	pcmEntityID = core.EntityID("char-erin")
	pcmGobA     = core.EntityID("goblin-pcm-a")
	pcmGobB     = core.EntityID("goblin-pcm-b")
	pcmDoorID   = "door-pcm-1"
)

// spendHex is a single Move waypoint 5ft from erin's start (lineHex(0)) —
// well clear of the closed door at k=10.
var spendHex = lineHex(1)

// PocketClearMoveTeardownSuite exercises rpg-toolkit#808's fix end to end.
type PocketClearMoveTeardownSuite struct {
	suite.Suite
	ctx       context.Context
	transport *encounter.InMemoryTransport
	broker    *encounter.Broker
}

func TestPocketClearMoveTeardownSuite(t *testing.T) {
	suite.Run(t, new(PocketClearMoveTeardownSuite))
}

func (s *PocketClearMoveTeardownSuite) SetupTest() {
	s.ctx = context.Background()
	s.transport = encounter.NewInMemoryTransport()
	s.broker = encounter.NewBroker(s.transport)
}

func (s *PocketClearMoveTeardownSuite) TearDownTest() {
	_ = s.broker.Close()
	_ = s.transport.Close()
}

// erinCharJSON builds a level-1 Human Fighter (30ft speed) with no
// pre-seeded ActionEconomy — combat entry seeds it live via
// seedActiveActorIfUnseeded's LoadFromData catch-up, matching
// move_economy_test.go's charliCharJSON.
func (s *PocketClearMoveTeardownSuite) erinCharJSON() json.RawMessage {
	s.T().Helper()
	charData := &dnd5eCharacter.Data{
		ID: string(pcmEntityID), PlayerID: string(pcmPlayerID),
		Name: "Erin the Fighter", Level: 1,
		ClassID: classes.Fighter, RaceID: races.Human,
		AbilityScores: shared.AbilityScores{
			abilities.STR: 16, abilities.DEX: 14, abilities.CON: 14,
			abilities.INT: 10, abilities.WIS: 12, abilities.CHA: 8,
		},
		HitPoints: 12, MaxHitPoints: 12, ArmorClass: 16, ProficiencyBonus: 2,
	}
	raw, err := json.Marshal(charData)
	s.Require().NoError(err)
	return raw
}

// loadErinVsTwoPockets builds group A (visible immediately, k=1) and group
// B (behind a closed door at k=10, monster at k=15) with erin's DataJSON
// attached, then round-trips through ToData/LoadFromData
// (loadRagingBarbVsGoblin's technique) so the cascade hydrates erin's
// *character.Character onto the returned encounter's combatant map —
// required for InCombat()/ExitCombat to mean anything. Group B is added
// BEFORE group A (combat_pockets_test.go's ordering note): group A's own
// AddMonster call must be what triggers checkCombatEntry, scoping
// initiative to the engaged pocket only.
func (s *PocketClearMoveTeardownSuite) loadErinVsTwoPockets() *encounter.Encounter {
	s.T().Helper()
	enc := encounter.New(s.ctx, "enc-pocket-clear-move-teardown", s.broker,
		encounter.WithRoller(fixedMaxRoller{}),
		encounter.WithCombatResolver(alwaysHitResolver{damage: 999, damageType: damageSlashing}))
	s.Require().NoError(enc.InitRoom(20, 20, environments.PatternEmpty))
	s.Require().NoError(enc.AddDoor(pcmDoorID, lineHex(10), false))
	s.Require().NoError(enc.AddPlayer(encounter.PlayerInput{
		PlayerID: pcmPlayerID, EntityID: pcmEntityID,
		Position: lineHex(0), SightRange: 10,
		HP: 12, MaxHP: 12, AC: 16, AttackBonus: 5,
		DamageDice: moveEconDamage, DamageType: damageSlashing,
		DataJSON: s.erinCharJSON(),
	}))
	s.Require().NoError(enc.AddMonster(encounter.MonsterInput{
		ID: pcmGobB, Position: lineHex(15),
		HP: 1, MaxHP: 7, AC: 5, Speed: 6,
		MonsterRef: monsterRefGoblin,
	}))
	s.Require().NoError(enc.AddMonster(encounter.MonsterInput{
		ID: pcmGobA, Position: lineHex(1),
		HP: 1, MaxHP: 7, AC: 5, Speed: 6,
		MonsterRef: monsterRefGoblin,
	}))

	raw, err := json.Marshal(enc.ToData())
	s.Require().NoError(err)
	var data encounter.Data
	s.Require().NoError(json.Unmarshal(raw, &data))
	loaded, err := encounter.LoadFromData(s.ctx, &data, s.broker,
		encounter.WithRoller(fixedMaxRoller{}),
		encounter.WithCombatResolver(alwaysHitResolver{damage: 999, damageType: damageSlashing}))
	s.Require().NoError(err)
	return loaded
}

// endTurnUntilErin cycles EndTurn until erin is the active actor, bounded
// so a bug that never lands on erin fails the test instead of hanging.
func (s *PocketClearMoveTeardownSuite) endTurnUntilErin(enc *encounter.Encounter) {
	for i := 0; enc.ActiveActor() != pcmEntityID && i < 8; i++ {
		_, _, err := enc.EndTurn(s.ctx, enc.ActiveActor())
		s.Require().NoError(err)
	}
	s.Require().Equal(pcmEntityID, enc.ActiveActor(), "setup must land on erin's turn")
}

// TestPocketClear_TearsDownEconomy_FreeRoamMoveSucceeds is the primary
// goal-behavior proof: after group A (the whole current pocket) dies with
// group B still alive elsewhere, and erin had FULLY spent her movement
// budget landing the killing blow, a further FREE_ROAM move must succeed —
// free-roam movement is unmetered, and this is the bug's own repro shape:
// MovementRemaining==0 carried into FREE_ROAM.
func (s *PocketClearMoveTeardownSuite) TestPocketClear_TearsDownEconomy_FreeRoamMoveSucceeds() {
	enc := s.loadErinVsTwoPockets()
	s.endTurnUntilErin(enc)

	pre := enc.ActorTurnState(pcmEntityID)
	s.Require().NotNil(pre.Economy, "erin must be seeded into the pocket A fight")
	s.Equal(30, pre.Economy.MovementRemaining, "Human fighter seeds 30ft")

	// Spend the ENTIRE budget before the killing blow (6 hexes, oscillating
	// between lineHex(0) and lineHex(1) so the cumulative path distance is
	// 30ft while ending back at the starting hex, well clear of the closed
	// door at k=10) — the exact repro shape: MovementRemaining==0 the
	// instant the pocket clears.
	s.Require().NoError(enc.Move(pcmPlayerID,
		[]core.Hex{spendHex, lineHex(0), spendHex, lineHex(0), spendHex, lineHex(0)}))
	mid := enc.ActorTurnState(pcmEntityID)
	s.Require().Equal(0, mid.Economy.MovementRemaining,
		"test premise: movement fully spent before the killing blow")

	s.Require().NoError(enc.TakeAction(pcmPlayerID,
		encounter.ActionRef{Module: refModuleDnd5e, Type: refTypeAction, ID: actionIDAttackTest},
		encounter.ActionTarget{EntityID: pcmGobA},
	))
	s.Require().Equal(core.ModeFreeRoam, enc.Mode(),
		"test premise: pocket A cleared to FREE_ROAM, group B alive elsewhere")

	// The regression: with MovementRemaining still 0 from the pre-kill
	// spend, the OLD gate (keyed on InCombat(), not Mode) rejected ANY
	// further move in FREE_ROAM.
	s.Require().NoError(enc.Move(pcmPlayerID, []core.Hex{spendHex}),
		"a free-roam move must not be gated by the now-stale in-combat movement budget (rpg-toolkit#808)")
}

// TestWithinPocket_MovementStillEnforced_WhenGenuinelyExhausted proves the
// fix does not loosen in-combat enforcement: while pocket A is STILL
// active (group A alive), a move requested after the budget is fully spent
// must still be rejected with ErrInsufficientMovement.
func (s *PocketClearMoveTeardownSuite) TestWithinPocket_MovementStillEnforced_WhenGenuinelyExhausted() {
	enc := s.loadErinVsTwoPockets()
	s.endTurnUntilErin(enc)

	s.Require().NoError(enc.Move(pcmPlayerID,
		[]core.Hex{spendHex, lineHex(0), spendHex, lineHex(0), spendHex, lineHex(0)}))
	s.Require().Equal(core.ModeTurnBased, enc.Mode(), "test premise: pocket A still active, group A alive")

	err := enc.Move(pcmPlayerID, []core.Hex{spendHex})
	s.Require().Error(err, "a genuinely exhausted budget must still reject movement while in combat")
	s.ErrorIs(err, encounter.ErrInsufficientMovement)
}

// TestPocketClear_ThenReEntry_ReseedsFullMovementBudget proves re-engaging
// a fresh pocket restores a full movement budget for the actor's turn —
// SetMode(TurnBased)'s existing seedActorTurn->StartTurn call, unaffected
// by this fix's teardown.
func (s *PocketClearMoveTeardownSuite) TestPocketClear_ThenReEntry_ReseedsFullMovementBudget() {
	enc := s.loadErinVsTwoPockets()
	s.endTurnUntilErin(enc)

	s.Require().NoError(enc.TakeAction(pcmPlayerID,
		encounter.ActionRef{Module: refModuleDnd5e, Type: refTypeAction, ID: actionIDAttackTest},
		encounter.ActionTarget{EntityID: pcmGobA},
	))
	s.Require().Equal(core.ModeFreeRoam, enc.Mode(), "test premise: pocket A cleared")

	s.Require().NoError(enc.OpenDoor(pcmPlayerID, pcmDoorID))
	// Move adjacent to group B (k=14, one hex from B at k=15) — gains LoS
	// through the now-open door and re-triggers checkCombatEntry, exactly
	// like combat_pockets_test.go's TestOpenDoorAndSightB_StartsFreshPocket.
	path := make([]core.Hex, 0, 14)
	for k := 1; k <= 14; k++ {
		path = append(path, lineHex(k))
	}
	s.Require().NoError(enc.Move(pcmPlayerID, path))
	s.Require().Equal(core.ModeTurnBased, enc.Mode(), "sighting group B must start a fresh pocket")

	s.endTurnUntilErin(enc)
	post := enc.ActorTurnState(pcmEntityID)
	s.Require().NotNil(post.Economy)
	s.Equal(30, post.Economy.MovementRemaining,
		"re-entering a fresh pocket must re-seed a full movement budget")
}

// TestPocketClear_CombatScopedConditionPersists_NoEndCombatSweep proves the
// pocket exit's teardown is ExitCombat-ONLY, not the terminal path's fuller
// endCombatForPlayers sweep (death.go): a combat-scoped condition (Raging,
// which subscribes to CombatEndTopic and self-removes on EndCombat) must
// SURVIVE a pocket clear — by design, the breather is not "combat over,"
// just "this pocket's fight is." Configurable condition-sweep on this path
// is deferred to a separate issue, not this fix.
func (s *PocketClearMoveTeardownSuite) TestPocketClear_CombatScopedConditionPersists_NoEndCombatSweep() {
	charJSON := ecsRagingBarbDataJSON(s.T(), string(bobEntityID), string(bobPlayerID), 16)

	enc := encounter.New(s.ctx, "enc-pocket-clear-condition-persist", s.broker,
		encounter.WithRoller(fixedMaxRoller{}),
		encounter.WithCombatResolver(alwaysHitResolver{damage: 999, damageType: damageSlashing}))
	s.Require().NoError(enc.InitRoom(20, 20, environments.PatternEmpty))
	s.Require().NoError(enc.AddDoor(pcmDoorID, lineHex(10), false))
	s.Require().NoError(enc.AddPlayer(encounter.PlayerInput{
		PlayerID: bobPlayerID, EntityID: bobEntityID,
		Position: lineHex(0), SightRange: 10,
		HP: 16, MaxHP: 16, AC: 14, AttackBonus: 5,
		DamageDice: ecsDamageDice, DamageType: damageSlashing,
		DataJSON: charJSON,
	}))
	s.Require().NoError(enc.AddMonster(encounter.MonsterInput{
		ID: pcmGobB, Position: lineHex(15),
		HP: 1, MaxHP: 7, AC: 5, Speed: 6,
		MonsterRef: monsterRefGoblin,
	}))
	s.Require().NoError(enc.AddMonster(encounter.MonsterInput{
		ID: pcmGobA, Position: lineHex(1),
		HP: 1, MaxHP: 7, AC: 5, Speed: 6,
		MonsterRef: monsterRefGoblin,
	}))

	raw, err := json.Marshal(enc.ToData())
	s.Require().NoError(err)
	var data encounter.Data
	s.Require().NoError(json.Unmarshal(raw, &data))
	loaded, err := encounter.LoadFromData(s.ctx, &data, s.broker,
		encounter.WithRoller(fixedMaxRoller{}),
		encounter.WithCombatResolver(alwaysHitResolver{damage: 999, damageType: damageSlashing}))
	s.Require().NoError(err)

	for i := 0; loaded.ActiveActor() != core.EntityID(bobEntityID) && i < 8; i++ {
		_, _, err := loaded.EndTurn(s.ctx, loaded.ActiveActor())
		s.Require().NoError(err)
	}
	s.Require().Equal(core.EntityID(bobEntityID), loaded.ActiveActor(), "setup must land on the barbarian's turn")

	s.Require().NoError(loaded.TakeAction(bobPlayerID,
		encounter.ActionRef{Module: refModuleDnd5e, Type: refTypeAction, ID: actionIDAttackTest},
		encounter.ActionTarget{EntityID: pcmGobA},
	))
	s.Require().Equal(core.ModeFreeRoam, loaded.Mode(), "test premise: pocket A cleared, group B alive elsewhere")

	persisted := loaded.ToData()
	playerData := persisted.Players[bobPlayerID]
	s.Require().NotNil(playerData)
	var charData dnd5eCharacter.Data
	s.Require().NoError(json.Unmarshal(playerData.DataJSON, &charData))
	s.NotEmpty(charData.Conditions,
		"raging must PERSIST across a non-terminal pocket clear (rpg-toolkit#808 scope decision) — "+
			"only the terminal encounter-end path (death.go's endCombatForPlayers) sweeps conditions via EndCombat")

	post := loaded.ActorTurnState(core.EntityID(bobEntityID))
	s.Nil(post.Economy, "ActionEconomy must still be torn down (ExitCombat) even though conditions persist")
}
