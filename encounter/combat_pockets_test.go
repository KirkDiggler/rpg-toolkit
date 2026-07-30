package encounter_test

// combat_pockets_test.go is the integration gate for rpg-toolkit#794 (wave
// 2 slice 1b, design doc Fork 2): initiative scoped to the ENGAGED pocket
// (LoS-having monsters, not every monster in the space), a non-terminal
// TURN_BASED->FREE_ROAM exit when a pocket clears but monsters remain
// elsewhere, and ModeEnded reserved for the whole-dungeon clear.
//
// Uses slice 1's door machinery (merged, toolkit#791) to build the two
// separate pockets the gate calls for: two monster groups behind a closed
// door — sighting group A starts combat with ONLY group A in initiative;
// clearing A returns to FREE_ROAM, not ModeEnded; opening the door and
// sighting group B starts a FRESH pocket; clearing B (the last monster
// anywhere) fires ModeEnded.
//
// Positions reuse doors_slice1_test.go's line convention: Hex{Q:0, R:-k,
// S:k} maps to offset position {X:0, Y:k} (col fixed, row increasing),
// keeping every position collinear and comfortably inside the room by
// construction.

import (
	"context"
	"testing"
	"time"

	"github.com/stretchr/testify/suite"

	"github.com/KirkDiggler/rpg-toolkit/encounter"
	"github.com/KirkDiggler/rpg-toolkit/encounter/core"
	"github.com/KirkDiggler/rpg-toolkit/encounter/events"
	"github.com/KirkDiggler/rpg-toolkit/tools/environments"
)

const (
	pocketGobA = core.EntityID("goblin-pocket-a")
	pocketGobB = core.EntityID("goblin-pocket-b")
)

type CombatPocketsSuite struct {
	suite.Suite
	ctx       context.Context
	transport *encounter.InMemoryTransport
	broker    *encounter.Broker
	enc       *encounter.Encounter
}

func TestCombatPocketsSuite(t *testing.T) {
	suite.Run(t, new(CombatPocketsSuite))
}

// SetupTest builds a 20x20 empty-pattern room (no interior walls beyond
// the one door under test — see doors_slice1_test.go's SetupTest for why
// this isolates the behavior under test rather than depending on
// generated wall placement), a closed door at k=3, alice at k=0 with
// SightRange 10, group A (a single 1-HP goblin) at k=1 -- same side of
// the door as alice, visible immediately -- and group B (a single 1-HP
// goblin) at k=6, beyond the closed door.
//
// Fixture order matters twice over:
//  1. The door is added BEFORE the player, same reason as
//     doors_slice1_test.go: AddPlayer computes the player's initial
//     revealed set immediately against whatever e.room looks like at that
//     moment.
//  2. Group B is added BEFORE group A. AddMonster appends unconditionally
//     to an ALREADY-ModeTurnBased Initiative (the "reinforcement" case,
//     encounter.go's AddMonster doc) — that's a different, pre-existing
//     code path this PR does not touch (see this PR's Scope-decisions).
//     Adding group A second means group A's own AddMonster call is what
//     triggers checkCombatEntry, and rollInitiative scopes to whichever
//     monsters are ENGAGED at that exact moment -- group A (visible),
//     never group B (behind the door, unengaged, still just sitting in
//     e.data.Monsters). Reversing this order would instead exercise
//     AddMonster's unconditional reinforcement-append and prove nothing
//     about rollInitiative's own scoping.
func (s *CombatPocketsSuite) SetupTest() {
	s.ctx = context.Background()
	s.transport = encounter.NewInMemoryTransport()
	s.broker = encounter.NewBroker(s.transport)
	s.enc = encounter.New(s.ctx, "enc-combat-pockets", s.broker,
		encounter.WithRoller(fixedMaxRoller{}),
		encounter.WithCombatResolver(alwaysHitResolver{damage: 999, damageType: damageSlashing}),
	)

	s.Require().NoError(s.enc.InitRoom(20, 20, environments.PatternEmpty))
	s.Require().NoError(s.enc.AddDoor("door-1", lineHex(3), false))
	s.Require().NoError(s.enc.AddPlayer(encounter.PlayerInput{
		PlayerID: alicePlayerID, EntityID: aliceEntityID,
		Position: lineHex(0), SightRange: 10,
		HP: 20, MaxHP: 20, AC: 14, AttackBonus: 5,
		DamageDice: dice1d6, DamageType: damageSlashing,
	}))
	s.Require().NoError(s.enc.AddMonster(encounter.MonsterInput{
		ID: pocketGobB, Position: lineHex(6),
		HP: 1, MaxHP: 7, AC: 5, Speed: 6,
		MonsterRef: monsterRefGoblin,
	}))
	s.Require().NoError(s.enc.AddMonster(encounter.MonsterInput{
		ID: pocketGobA, Position: lineHex(1),
		HP: 1, MaxHP: 7, AC: 5, Speed: 6,
		MonsterRef: monsterRefGoblin,
	}))
}

func (s *CombatPocketsSuite) TearDownTest() {
	_ = s.broker.Close()
	_ = s.transport.Close()
}

// initiativeContains reports whether id is in the encounter's current
// Initiative slice.
func (s *CombatPocketsSuite) initiativeContains(id core.EntityID) bool {
	for _, entry := range s.enc.ToData().Initiative {
		if entry == id {
			return true
		}
	}
	return false
}

// endTurnUntilActive cycles EndTurn until alice is the active actor (the
// same pattern encounter_end_condition_sweep_test.go and integration_test.go
// use), bounded so a bug that never lands on alice fails the test instead
// of hanging.
func (s *CombatPocketsSuite) endTurnUntilActive() {
	for i := 0; s.enc.ActiveActor() != aliceEntityID && i < 8; i++ {
		_, _, err := s.enc.EndTurn(s.ctx, s.enc.ActiveActor())
		s.Require().NoError(err)
	}
	s.Require().Equal(core.EntityID(aliceEntityID), s.enc.ActiveActor(), "setup must land on alice's turn")
}

// TestSightingA_StartsPocket_OnlyAInInitiative is the setup-time
// assertion: after SetupTest, sighting group A must have started combat
// with ONLY group A in initiative -- group B, behind the closed door,
// must not have joined even though AddMonster ran for it too.
func (s *CombatPocketsSuite) TestSightingA_StartsPocket_OnlyAInInitiative() {
	s.Require().Equal(core.ModeTurnBased, s.enc.Mode())
	s.True(s.initiativeContains(pocketGobA), "group A must be in the pocket that sighted it")
	s.False(s.initiativeContains(pocketGobB),
		"group B, behind the closed door, must not join a pocket it isn't engaged with")
}

// TestClearingA_ReturnsFreeRoam_NotEnded: killing group A (the whole
// current pocket) with group B still alive elsewhere must exit to
// FREE_ROAM, not fire the terminal ModeEnded.
func (s *CombatPocketsSuite) TestClearingA_ReturnsFreeRoam_NotEnded() {
	sub, err := s.broker.Subscribe("enc-combat-pockets", alicePlayerID)
	s.Require().NoError(err)
	defer func() { _ = sub.Close() }()

	s.endTurnUntilActive()
	s.Require().NoError(s.enc.TakeAction(alicePlayerID,
		encounter.ActionRef{Module: refModuleDnd5e, Type: refTypeAction, ID: actionIDAttackTest},
		encounter.ActionTarget{EntityID: pocketGobA},
	))

	s.Equal(core.ModeFreeRoam, s.enc.Mode(),
		"clearing the current pocket with monsters remaining elsewhere must exit to FREE_ROAM")
	s.Empty(s.enc.ToData().Initiative, "FREE_ROAM must not carry a stale initiative order")

	evts := collectTypes(sub, time.Second)
	s.Contains(evts, "*events.ModeChangedEvent", "the pocket-exit must publish ModeChangedEvent")
	s.NotContains(evts, "*events.EncounterEndedEvent",
		"clearing one pocket must not fire the terminal encounter-end event")

	// Group B must still be alive and unengaged -- this is the whole
	// point of "not terminal."
	s.Contains(s.enc.ToData().Monsters, pocketGobB)
}

// TestOpenDoorAndSightB_StartsFreshPocket: after clearing A, opening the
// door and moving alice into B's sightline must start a FRESH pocket --
// group B only, Round reset to 1, no stale group-A remnants.
func (s *CombatPocketsSuite) TestOpenDoorAndSightB_StartsFreshPocket() {
	s.endTurnUntilActive()
	s.Require().NoError(s.enc.TakeAction(alicePlayerID,
		encounter.ActionRef{Module: refModuleDnd5e, Type: refTypeAction, ID: actionIDAttackTest},
		encounter.ActionTarget{EntityID: pocketGobA},
	))
	s.Require().Equal(core.ModeFreeRoam, s.enc.Mode(), "test premise: pocket A cleared")

	// rpg-toolkit#864: OpenDoor requires adjacency — walk alice up to the
	// door first (mirrors doors_slice1_test.go's identical fix).
	s.Require().NoError(s.enc.Move(alicePlayerID, []core.Hex{lineHex(1), lineHex(2)}))
	s.Require().NoError(s.enc.OpenDoor(alicePlayerID, "door-1"))
	// Move adjacent to group B (k=5, one hex from B at k=6) -- this is the
	// Move call that gains LoS through the now-open door and re-triggers
	// checkCombatEntry, exactly like doors_slice1_test.go's open-door-then-
	// move pattern.
	path := []core.Hex{lineHex(3), lineHex(4), lineHex(5)}
	s.Require().NoError(s.enc.Move(alicePlayerID, path))

	s.Require().Equal(core.ModeTurnBased, s.enc.Mode(), "sighting group B must start a fresh pocket")
	s.True(s.initiativeContains(pocketGobB), "group B must be in the fresh pocket")
	s.False(s.initiativeContains(pocketGobA), "group A is dead -- must not reappear in the fresh initiative")
	s.Equal(1, s.enc.ToData().Round, "a fresh pocket must start at round 1, not carry over pocket A's round count")
}

// TestClearingB_LastMonster_FiresModeEnded: killing group B -- the last
// monster anywhere in the encounter -- must fire the terminal ModeEnded,
// completing the gate's full sequence.
func (s *CombatPocketsSuite) TestClearingB_LastMonster_FiresModeEnded() {
	s.endTurnUntilActive()
	s.Require().NoError(s.enc.TakeAction(alicePlayerID,
		encounter.ActionRef{Module: refModuleDnd5e, Type: refTypeAction, ID: actionIDAttackTest},
		encounter.ActionTarget{EntityID: pocketGobA},
	))
	// rpg-toolkit#864: OpenDoor requires adjacency — walk alice up to the
	// door first (mirrors doors_slice1_test.go's identical fix).
	s.Require().NoError(s.enc.Move(alicePlayerID, []core.Hex{lineHex(1), lineHex(2)}))
	s.Require().NoError(s.enc.OpenDoor(alicePlayerID, "door-1"))
	s.Require().NoError(s.enc.Move(alicePlayerID,
		[]core.Hex{lineHex(3), lineHex(4), lineHex(5)}))
	s.Require().Equal(core.ModeTurnBased, s.enc.Mode(), "test premise: pocket B engaged")

	sub, err := s.broker.Subscribe("enc-combat-pockets", alicePlayerID)
	s.Require().NoError(err)
	defer func() { _ = sub.Close() }()

	s.endTurnUntilActive()
	s.Require().NoError(s.enc.TakeAction(alicePlayerID,
		encounter.ActionRef{Module: refModuleDnd5e, Type: refTypeAction, ID: actionIDAttackTest},
		encounter.ActionTarget{EntityID: pocketGobB},
	))

	s.Equal(core.ModeEnded, s.enc.Mode(), "the last monster anywhere dying must end the encounter")
	var ended *events.EncounterEndedEvent
	for _, evt := range drainEvents(sub, time.Second) {
		if e, ok := evt.(*events.EncounterEndedEvent); ok {
			ended = e
		}
	}
	s.Require().NotNil(ended, "ModeEnded must publish EncounterEndedEvent")
	s.Equal(encounter.EncounterEndedReasonAllHostilesDefeated, ended.Reason)
}
