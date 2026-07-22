package encounter_test

// obstacle_test.go covers rpg-toolkit#818's generic static-obstacle
// infrastructure: a persisted ObstacleData shape on SpaceData (stable
// ID/Ref, absolute position, BlocksMovement/BlocksLoS), full
// ToData/LoadFromData round-trip, and rebuild parity with walls/doors —
// obstacles are placed into the exact same spatial.Room truth so
// movement and line-of-sight consult them identically. Infrastructure
// only: no crypt-specific placement (#819), no cover math, no
// interaction verbs.

import (
	"context"
	"encoding/json"
	"testing"

	"github.com/stretchr/testify/suite"

	"github.com/KirkDiggler/rpg-toolkit/encounter"
	"github.com/KirkDiggler/rpg-toolkit/encounter/core"
	"github.com/KirkDiggler/rpg-toolkit/encounter/perception"
	"github.com/KirkDiggler/rpg-toolkit/tools/environments"
)

type ObstacleSuite struct {
	suite.Suite
	transport *encounter.InMemoryTransport
	broker    *encounter.Broker
}

func TestObstacleSuite(t *testing.T) {
	suite.Run(t, new(ObstacleSuite))
}

func (s *ObstacleSuite) SetupTest() {
	s.transport = encounter.NewInMemoryTransport()
	s.broker = encounter.NewBroker(s.transport)
}

// obstacleTestHex is a fixed, verified-in-bounds hex (offset (3,3) under
// pointy-top orientation) inside every 10x10 room this file builds — the
// same cell space_test.go's TestLoadFromData_DuplicateWallEntries_Tolerated
// pins for the identical reason: a known cell with all neighbors in-bounds
// too, so a single-hex approach-and-move always lands cleanly.
var obstacleTestHex = core.Hex{Q: 3, R: -5, S: 2}

// TestObstacle_ToData_LoadFromData_RoundTrip covers the #818 done bar's
// core requirement: every field on every obstacle instance survives a
// ToData/LoadFromData round-trip exactly.
func (s *ObstacleSuite) TestObstacle_ToData_LoadFromData_RoundTrip() {
	enc := encounter.New(context.Background(), "enc-obstacle-rt", s.broker)
	s.Require().NoError(enc.InitRoom(10, 10, environments.PatternEmpty))

	pos := core.Hex{Q: 2, R: -3, S: 1}
	s.Require().NoError(enc.AddObstacle("sarcophagus-1", "dnd5e:obstacles:sarcophagus", pos, true, true))

	original := enc.ToData()
	s.Require().NotNil(original.Space)
	s.Require().Len(original.Space.Obstacles, 1)
	got := original.Space.Obstacles[0]
	s.Equal(core.EntityID("sarcophagus-1"), got.ID)
	s.Equal("dnd5e:obstacles:sarcophagus", got.Ref)
	s.Equal(pos, got.Position)
	s.True(got.BlocksMovement)
	s.True(got.BlocksLoS)

	reloaded, err := encounter.LoadFromData(context.Background(), original, s.broker)
	s.Require().NoError(err)

	again := reloaded.ToData()
	s.Require().NotNil(again.Space)
	s.Require().Len(again.Space.Obstacles, 1)
	s.Equal(got, again.Space.Obstacles[0], "obstacle must round-trip exactly through LoadFromData")
}

// TestObstacle_BlocksMovement_True covers the #818 done bar's movement
// invariant: the rebuilt spatial.Room agrees with an obstacle's
// BlocksMovement=true exactly as it does for a wall — Move truncates at
// the obstacle's hex rather than passing through it.
func (s *ObstacleSuite) TestObstacle_BlocksMovement_True() {
	enc := encounter.New(context.Background(), "enc-obstacle-move-block", s.broker)
	s.Require().NoError(enc.InitRoom(10, 10, environments.PatternEmpty))
	s.Require().NoError(enc.AddObstacle("pillar-1", "dnd5e:obstacles:pillar", obstacleTestHex, true, false))

	start := perception.HexNeighbors(obstacleTestHex)[0]
	s.Require().NoError(enc.AddPlayer(encounter.PlayerInput{
		PlayerID: "alice", EntityID: "char-alice",
		Position: start, SightRange: 10,
	}))

	s.Require().NoError(enc.Move("alice", []core.Hex{obstacleTestHex}))

	after := enc.ToData().Players["alice"].View.Position
	s.Equal(start, after, "player must not have moved onto the obstacle's hex")
}

// TestObstacle_BlocksMovement_False covers the complement: an obstacle
// that only blocks line of sight (BlocksMovement=false) must NOT impede
// movement through its hex.
func (s *ObstacleSuite) TestObstacle_BlocksMovement_False() {
	enc := encounter.New(context.Background(), "enc-obstacle-move-pass", s.broker)
	s.Require().NoError(enc.InitRoom(10, 10, environments.PatternEmpty))
	s.Require().NoError(enc.AddObstacle("haze-1", "dnd5e:obstacles:haze", obstacleTestHex, false, true))

	start := perception.HexNeighbors(obstacleTestHex)[0]
	s.Require().NoError(enc.AddPlayer(encounter.PlayerInput{
		PlayerID: "alice", EntityID: "char-alice",
		Position: start, SightRange: 10,
	}))

	s.Require().NoError(enc.Move("alice", []core.Hex{obstacleTestHex}))

	after := enc.ToData().Players["alice"].View.Position
	s.Equal(obstacleTestHex, after, "a non-movement-blocking obstacle must not impede movement")
}

// TestObstacle_BlocksLoS_True covers the LoS half of the #818 done bar:
// an obstacle with BlocksLoS=true must block room.IsLineOfSightBlocked
// between two hexes on opposite sides of it, exactly like a wall.
// HexNeighbors(h)[0] and [3] are diametrically opposite directions (their
// deltas negate each other), so the obstacle sits exactly on the line
// between them.
func (s *ObstacleSuite) TestObstacle_BlocksLoS_True() {
	enc := encounter.New(context.Background(), "enc-obstacle-los-block", s.broker)
	s.Require().NoError(enc.InitRoom(10, 10, environments.PatternEmpty))
	s.Require().NoError(enc.AddObstacle("statue-1", "dnd5e:obstacles:statue", obstacleTestHex, false, true))

	neighbors := perception.HexNeighbors(obstacleTestHex)
	from, to := neighbors[0], neighbors[3]

	s.Require().NotNil(enc.Room())
	s.True(enc.Room().IsLineOfSightBlocked(from.ToPosition(), to.ToPosition()),
		"an obstacle with BlocksLoS=true must block LoS across its hex")
}

// TestObstacle_BlocksLoS_False covers the complement: an obstacle that
// only blocks movement (BlocksLoS=false) must NOT block line of sight
// through its hex.
func (s *ObstacleSuite) TestObstacle_BlocksLoS_False() {
	enc := encounter.New(context.Background(), "enc-obstacle-los-pass", s.broker)
	s.Require().NoError(enc.InitRoom(10, 10, environments.PatternEmpty))
	s.Require().NoError(enc.AddObstacle("rubble-1", "dnd5e:obstacles:rubble", obstacleTestHex, true, false))

	neighbors := perception.HexNeighbors(obstacleTestHex)
	from, to := neighbors[0], neighbors[3]

	s.Require().NotNil(enc.Room())
	s.False(enc.Room().IsLineOfSightBlocked(from.ToPosition(), to.ToPosition()),
		"an obstacle with BlocksLoS=false must not block LoS across its hex")
}

// TestObstacle_CoexistsWithWallsAndDoors covers the #818 done bar's
// coexistence requirement: a Space carrying a wall, a closed door, and an
// obstacle at three DISTINCT hexes must have all three block/unblock
// independently and correctly after rebuild — the obstacle is additive,
// not a replacement for the existing wall/door machinery.
func (s *ObstacleSuite) TestObstacle_CoexistsWithWallsAndDoors() {
	wallHex := core.Hex{Q: 3, R: -5, S: 2}     // offset (3,3)
	doorHex := core.Hex{Q: 4, R: -6, S: 2}     // offset (4,3)
	obstacleHex := core.Hex{Q: 5, R: -7, S: 2} // offset (5,3)

	data := encounter.NewData("enc-obstacle-coexist")
	data.Doors["door-1"] = &encounter.DoorData{ID: "door-1", Position: doorHex, Open: false}
	data.Space = &encounter.SpaceData{
		Walls: []environments.WallSegmentData{
			{Start: wallHex.ToCube(), End: wallHex.ToCube(), BlocksMovement: true, BlocksLoS: true},
		},
		Width:  10,
		Height: 10,
		Obstacles: []encounter.ObstacleData{
			{ID: "crate-1", Ref: "dnd5e:obstacles:crate", Position: obstacleHex, BlocksMovement: true, BlocksLoS: false},
		},
	}

	enc, err := encounter.LoadFromData(context.Background(), data, s.broker)
	s.Require().NoError(err)
	s.Require().NotNil(enc.Room())

	// Wall still blocks movement.
	wallStart := perception.HexNeighbors(wallHex)[0]
	s.Require().NoError(enc.AddPlayer(encounter.PlayerInput{
		PlayerID: "alice", EntityID: "char-alice", Position: wallStart, SightRange: 10,
	}))
	s.Require().NoError(enc.Move("alice", []core.Hex{wallHex}))
	s.Equal(wallStart, enc.ToData().Players["alice"].View.Position, "wall must still block movement")

	// Closed door still blocks movement.
	doorStart := perception.HexNeighbors(doorHex)[0]
	s.Require().NoError(enc.AddPlayer(encounter.PlayerInput{
		PlayerID: "bob", EntityID: "char-bob", Position: doorStart, SightRange: 10,
	}))
	s.Require().NoError(enc.Move("bob", []core.Hex{doorHex}))
	s.Equal(doorStart, enc.ToData().Players["bob"].View.Position, "closed door must still block movement")

	// Obstacle blocks movement too, independently.
	obstacleStart := perception.HexNeighbors(obstacleHex)[0]
	s.Require().NoError(enc.AddPlayer(encounter.PlayerInput{
		PlayerID: "carol", EntityID: "char-carol", Position: obstacleStart, SightRange: 10,
	}))
	s.Require().NoError(enc.Move("carol", []core.Hex{obstacleHex}))
	s.Equal(obstacleStart, enc.ToData().Players["carol"].View.Position, "obstacle must block movement")
}

// TestAddObstacle_RejectsEmptyID covers the #818 validation boundary:
// AddObstacle with an empty ID must fail with a contextual error and
// leave Data.Space.Obstacles unchanged (no partial state).
func (s *ObstacleSuite) TestAddObstacle_RejectsEmptyID() {
	enc := encounter.New(context.Background(), "enc-obstacle-empty-id", s.broker)
	s.Require().NoError(enc.InitRoom(10, 10, environments.PatternEmpty))

	err := enc.AddObstacle("", "dnd5e:obstacles:crate", obstacleTestHex, true, true)
	s.Require().Error(err)
	s.Empty(enc.ToData().Space.Obstacles, "a rejected AddObstacle must not leave a partial entry")
}

// TestAddObstacle_RejectsDuplicateID covers the #818 validation boundary:
// AddObstacle with an ID that already exists among the Space's obstacles
// must fail with a contextual error and leave Data.Space.Obstacles
// unchanged — this is load-bearing (not just tidiness): Obstacles is an
// order-preserving slice, and room.PlaceEntity keys placement by
// entity.GetID(), so an UNCHECKED duplicate ID would silently relocate
// the first obstacle's room occupancy rather than erroring.
func (s *ObstacleSuite) TestAddObstacle_RejectsDuplicateID() {
	enc := encounter.New(context.Background(), "enc-obstacle-dup-id", s.broker)
	s.Require().NoError(enc.InitRoom(10, 10, environments.PatternEmpty))
	s.Require().NoError(enc.AddObstacle("crate-1", "dnd5e:obstacles:crate", obstacleTestHex, true, true))

	otherHex := perception.HexNeighbors(obstacleTestHex)[2]
	err := enc.AddObstacle("crate-1", "dnd5e:obstacles:crate", otherHex, true, true)
	s.Require().Error(err)
	s.Require().Len(enc.ToData().Space.Obstacles, 1, "a rejected duplicate-ID add must not leave a second entry")
}

// TestAddObstacle_RejectsOutOfBoundsPosition covers the #818 validation
// boundary: AddObstacle at a position outside the room's grid bounds must
// fail with a contextual error, and the failed add must not leave a
// partial obstacle entry in Data.Space (atomicity mirrors AddDoor).
func (s *ObstacleSuite) TestAddObstacle_RejectsOutOfBoundsPosition() {
	enc := encounter.New(context.Background(), "enc-obstacle-oob", s.broker)
	s.Require().NoError(enc.InitRoom(10, 10, environments.PatternEmpty))

	outOfBounds := core.Hex{Q: 100, R: -150, S: 50}
	err := enc.AddObstacle("crate-1", "dnd5e:obstacles:crate", outOfBounds, true, true)
	s.Require().Error(err)
	s.Empty(enc.ToData().Space.Obstacles, "a rejected out-of-bounds add must not leave a partial entry")
}

// TestAddObstacle_RejectsDuplicateOccupancy covers the #818 validation
// boundary: AddObstacle at a hex already occupied by a movement-blocking
// wall must fail with a contextual error, atomically — no partial
// Data.Space/room state survives the failed call.
func (s *ObstacleSuite) TestAddObstacle_RejectsDuplicateOccupancy() {
	wallHex := core.Hex{Q: 3, R: -5, S: 2} // offset (3,3)
	data := encounter.NewData("enc-obstacle-dup-occupancy")
	data.Space = &encounter.SpaceData{
		Walls: []environments.WallSegmentData{
			{Start: wallHex.ToCube(), End: wallHex.ToCube(), BlocksMovement: true, BlocksLoS: true},
		},
		Width:  10,
		Height: 10,
	}
	enc, err := encounter.LoadFromData(context.Background(), data, s.broker)
	s.Require().NoError(err)

	addErr := enc.AddObstacle("crate-1", "dnd5e:obstacles:crate", wallHex, true, true)
	s.Require().Error(addErr, "an obstacle placed on an already-wall-occupied hex must be rejected")
	s.Empty(enc.ToData().Space.Obstacles, "a rejected duplicate-occupancy add must not leave a partial entry")
	s.Require().NotNil(enc.Room(), "the room itself must remain intact after the rejected add")
}

// TestObstacle_BackwardCompatible_NoObstacles covers the #818 done bar's
// backward-compatibility requirement: an encounter/SpaceData predating
// #818 (nil/empty Obstacles) must load and round-trip exactly as before —
// Obstacles omitted from the wire, no room-side change.
func (s *ObstacleSuite) TestObstacle_BackwardCompatible_NoObstacles() {
	enc := encounter.New(context.Background(), "enc-obstacle-compat", s.broker)
	s.Require().NoError(enc.InitRoom(10, 10, environments.PatternRandom, spaceTestSeed))

	original := enc.ToData()
	s.Require().NotNil(original.Space)
	s.Empty(original.Space.Obstacles, "a space with no obstacles must have an empty Obstacles slice")

	raw, err := json.Marshal(original)
	s.Require().NoError(err)
	s.NotContains(string(raw), `"obstacles"`, "Obstacles must be omitted from the wire when empty")

	reloaded, err := encounter.LoadFromData(context.Background(), original, s.broker)
	s.Require().NoError(err)
	s.Require().NotNil(reloaded.Room())
	s.Empty(reloaded.ToData().Space.Obstacles)
}
