package encounter_test

// doors_slice1_test.go is the integration gate for rpg-toolkit#790 (wave 2
// slice 1): closed doors block movement AND LoS via the existing wall
// machinery; OpenDoor unblocks and reveals through the doorway; and NPC
// visibility events don't leak through a closed door (folding in the
// rpg-api#648 gate finding). Nothing renders doors yet — this is toolkit
// integration coverage, not a mouse playtest (that lands in Slice 3).
//
// All hexes here sit on the single straight line Hex{Q:0, R:-k, S:k} for
// k=0,1,2,... — chosen because it maps to offset positions {X:0, Y:k}
// (col fixed, row increasing), keeping every position comfortably inside a
// generously-sized room and making "alice, door, and the monster are
// exactly collinear" true by construction rather than by coincidence.

import (
	"context"
	"testing"
	"time"

	"github.com/stretchr/testify/suite"

	"github.com/KirkDiggler/rpg-toolkit/encounter"
	"github.com/KirkDiggler/rpg-toolkit/encounter/core"
	"github.com/KirkDiggler/rpg-toolkit/tools/environments"
)

// lineHex returns the k-th hex on this file's test line.
func lineHex(k int) core.Hex {
	return core.Hex{Q: 0, R: -k, S: k}
}

type DoorBlockingSuite struct {
	suite.Suite
	transport *encounter.InMemoryTransport
	broker    *encounter.Broker
	enc       *encounter.Encounter
	aliceSub  *encounter.Subscription
}

func TestDoorBlockingSuite(t *testing.T) {
	suite.Run(t, new(DoorBlockingSuite))
}

// SetupTest builds a 20x20 empty-pattern room (no interior walls — the
// only blocking cell is the door itself, so every assertion below
// isolates door behavior instead of incidentally depending on generated
// wall placement), a closed door at k=3, alice at k=0 with SightRange 8
// (comfortably covering the k=6 monster if nothing blocked it), and a
// monster at k=6, beyond the door.
//
// Order matters: the door is added BEFORE the player. AddPlayer computes
// and reveals the player's initial visible set immediately, using
// whatever e.room looks like at that moment (view.go/AddPlayer) — adding
// the player first would reveal the monster's hex before the door existed
// to block it, which is a fixture-ordering bug, not a product one (a real
// dungeon's geometry is built before anyone stands in it).
func (s *DoorBlockingSuite) SetupTest() {
	s.transport = encounter.NewInMemoryTransport()
	s.broker = encounter.NewBroker(s.transport)
	s.enc = encounter.New(context.Background(), "enc-doors-slice1", s.broker)

	s.Require().NoError(s.enc.InitRoom(20, 20, environments.PatternEmpty))
	s.Require().NoError(s.enc.AddDoor("door-1", lineHex(3), false))
	s.Require().NoError(s.enc.AddPlayer(encounter.PlayerInput{
		PlayerID: alicePlayerID, EntityID: aliceEntityID,
		Position: lineHex(0), SightRange: 8,
	}))
	s.Require().NoError(s.enc.AddMonster(encounter.MonsterInput{
		ID: gobEntityID, Position: lineHex(6),
		HP: 7, MaxHP: 7, AC: 12, Speed: 30,
		DamageDice: dice1d6, DamageType: damagePiercing,
	}))

	var err error
	s.aliceSub, err = s.broker.Subscribe("enc-doors-slice1", alicePlayerID)
	s.Require().NoError(err)
}

func (s *DoorBlockingSuite) TearDownTest() {
	_ = s.aliceSub.Close()
	_ = s.broker.Close()
	_ = s.transport.Close()
}

// TestClosedDoor_BlocksLineOfSight: the monster (k=6) is within alice's
// SightRange (8) but on the far side of the closed door (k=3) on the same
// line — IsLineOfSightBlocked must report true, and the monster must not
// appear in alice's revealed hexes.
func (s *DoorBlockingSuite) TestClosedDoor_BlocksLineOfSight() {
	s.Require().True(
		s.enc.Room().IsLineOfSightBlocked(lineHex(0).ToPosition(), lineHex(6).ToPosition()),
		"closed door must block LoS to the hex beyond it",
	)
	snap := s.enc.SnapshotFor(alicePlayerID)
	s.False(snap.RevealedHexes.Has(lineHex(6)), "monster hex beyond a closed door must not be revealed")
}

// TestClosedDoor_BlocksMovement: alice tries to walk straight through the
// door; Move must truncate at the last hex before the door (k=2), not pass
// through it.
func (s *DoorBlockingSuite) TestClosedDoor_BlocksMovement() {
	path := []core.Hex{lineHex(1), lineHex(2), lineHex(3), lineHex(4)}
	s.Require().NoError(s.enc.Move(alicePlayerID, path))

	after := s.enc.ToData().Players[alicePlayerID].View.Position
	s.Equal(lineHex(2), after, "alice must stop one hex short of the closed door, not pass through it")
}

// TestClosedDoor_MonsterCuesDoNotLeak: rpg-api#648's finding, folded in
// here. Before rpg-toolkit#790, viewerCanSee was a pure hexDistance check,
// so a monster move beyond a closed door (well within alice's raw
// SightRange) would still publish a visible MoveEvent to alice. With the
// door closed, moving the monster must produce NO event on alice's
// subscription at all (applyNPCMovementSteps skips publishing entirely
// when no viewer can see the move) — not an event with an empty slice.
func (s *DoorBlockingSuite) TestClosedDoor_MonsterCuesDoNotLeak() {
	s.Require().NoError(s.enc.MoveNPCSteps(gobEntityID, []core.Hex{lineHex(7)}))

	events := collectTypes(s.aliceSub, 300*time.Millisecond)
	s.Empty(events, "monster movement beyond a closed door must not reach a player who can't see through it")
}

// TestOpenDoor_UnblocksAndRevealsBeyond: opening the door must (1) let
// movement pass through and (2) reveal the monster's hex in the same
// action, without alice needing to walk up to it first.
func (s *DoorBlockingSuite) TestOpenDoor_UnblocksAndRevealsBeyond() {
	s.Require().NoError(s.enc.OpenDoor(alicePlayerID, "door-1"))

	s.False(
		s.enc.Room().IsLineOfSightBlocked(lineHex(0).ToPosition(), lineHex(6).ToPosition()),
		"open door must no longer block LoS",
	)
	snap := s.enc.SnapshotFor(alicePlayerID)
	s.True(snap.RevealedHexes.Has(lineHex(6)),
		"opening the door must reveal the monster's hex beyond it in the same action, not require walking up to it")

	path := []core.Hex{lineHex(1), lineHex(2), lineHex(3), lineHex(4)}
	s.Require().NoError(s.enc.Move(alicePlayerID, path))
	after := s.enc.ToData().Players[alicePlayerID].View.Position
	s.Equal(lineHex(4), after, "an open door must let movement pass straight through")
}

// TestOpenDoor_MonsterCuesReachThroughOpenDoor: the mirror of
// TestClosedDoor_MonsterCuesDoNotLeak — once the door is open, the exact
// same monster move DOES reach alice.
func (s *DoorBlockingSuite) TestOpenDoor_MonsterCuesReachThroughOpenDoor() {
	s.Require().NoError(s.enc.OpenDoor(alicePlayerID, "door-1"))
	drainSub(s.aliceSub, 200*time.Millisecond) // drain the DoorOpened/HexRevealed pair

	s.Require().NoError(s.enc.MoveNPCSteps(gobEntityID, []core.Hex{lineHex(7)}))

	events := collectTypes(s.aliceSub, 300*time.Millisecond)
	s.Contains(events, "*events.MoveEvent",
		"monster movement through an open door must reach a player who can see through it")
}
