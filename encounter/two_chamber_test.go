package encounter_test

// two_chamber_test.go is the integration gate for rpg-toolkit#804 (wave 2
// slice 2, toolkit leg): InitTwoChamberRoom emits 2 chambers + 1 plain door
// + an entrance cell + per-chamber region tags, all in ONE continuous Space
// (design doc Fork 1 — chambers are a region TAG, not separate
// spatial.Rooms). Gate (from the issue): a connected walkable path
// entrance->door->chamber 2; a closed door blocks LoS+movement between
// chambers; region tags partition the chambers; deterministic under a
// named seed (the slice2TwoChambersSeed fixture below) while
// entropy-default (seed 0) varies.

import (
	"context"
	"reflect"
	"testing"

	"github.com/stretchr/testify/require"
	"github.com/stretchr/testify/suite"

	toolkitcore "github.com/KirkDiggler/rpg-toolkit/core"
	"github.com/KirkDiggler/rpg-toolkit/encounter"
	"github.com/KirkDiggler/rpg-toolkit/encounter/core"
	"github.com/KirkDiggler/rpg-toolkit/tools/environments"
	"github.com/KirkDiggler/rpg-toolkit/tools/spatial"
)

// slice2TwoChambersSeed is the named devseed fixture the issue calls for:
// a fixed seed reproducing the exact same two-chamber layout every run,
// used by TestLayout_DeterministicSeedVsEntropyDefault and available to
// any future playtest/devseed wiring that wants the identical dungeon.
const slice2TwoChambersSeed int64 = 424242

// Fixture dimensions: large enough that RandomPattern actually places a
// handful of interior walls (roomArea*density/10, capped at 12) so the
// connectivity gate is a real proof, not a vacuous one over an empty room.
const (
	chamberW         = 12
	chamberH         = 8
	twoChamberDoorID = core.EntityID("door-chamber-1-2")
)

// probeEntity is a throwaway core.Entity used to query room.CanPlaceEntity
// for wall/door-blocking purposes, mirroring the package-private
// wallCheckEntity (space.go) — this test lives in encounter_test, which
// can't see that unexported type.
type probeEntity struct{}

func (probeEntity) GetID() string                   { return "bfs-probe" }
func (probeEntity) GetType() toolkitcore.EntityType { return "bfs-probe" }

// reachableFrom returns every hex walkable from start via the room's grid
// neighbors, stopping at any wall/door-blocked cell — proves connectivity
// the same way movement does (room.CanPlaceEntity), independent of any
// particular path shape.
func reachableFrom(room spatial.Room, start core.Hex) map[core.Hex]bool {
	visited := map[core.Hex]bool{start: true}
	queue := []core.Hex{start}
	for len(queue) > 0 {
		cur := queue[0]
		queue = queue[1:]
		for _, npos := range room.GetGrid().GetNeighbors(cur.ToPosition()) {
			nh := core.HexFromPosition(npos)
			if visited[nh] {
				continue
			}
			if !room.CanPlaceEntity(probeEntity{}, npos) {
				continue
			}
			visited[nh] = true
			queue = append(queue, nh)
		}
	}
	return visited
}

// regionHexSet returns the hex membership of the named region as a plain
// map, or nil if no region with that ID exists.
func regionHexSet(sd *encounter.SpaceData, id string) map[core.Hex]bool {
	for _, r := range sd.Regions {
		if r.ID != id {
			continue
		}
		out := make(map[core.Hex]bool, len(r.Hexes))
		for _, h := range r.Hexes.Slice() {
			out[h] = true
		}
		return out
	}
	return nil
}

// doorAdjacentChamber2Hex is the chamber-2 cell immediately across the
// door — the door-to-chamber required path's From point in
// generateTwoChamberLayout, so it's guaranteed wall-free by construction.
// Duplicated from the generator's own formula rather than exported: this
// is the test asserting the generator's construction guarantee actually
// holds, not consuming a public API.
func doorAdjacentChamber2Hex() core.Hex {
	return core.HexFromPosition(spatial.Position{X: float64(chamberW + 1), Y: float64(chamberH / 2)})
}

// doorAdjacentChamber1Hex is the chamber-1 cell immediately before the
// door (the entrance-to-door required path's To point) — the last hex
// movement can reach while the door is closed, since a closed door blocks
// its OWN cell (rpg-toolkit#790), not just the far side of it.
func doorAdjacentChamber1Hex() core.Hex {
	return core.HexFromPosition(spatial.Position{X: float64(chamberW - 1), Y: float64(chamberH / 2)})
}

// straightRowPath returns the Move-ready hex sequence from just after from
// up to and including to, stepping one X unit at a time along a shared row
// (from.ToPosition().Y must equal to.ToPosition().Y) — the shape of the
// generator's own guaranteed wall-free "required path" between an entrance
// and a door-adjacent cell. rpg-toolkit#864: OpenDoor/AttemptUnlock now
// require the actor to actually be adjacent, so fixtures across this
// package's dungeon/chamber tests use this to walk there via Move first,
// instead of teleporting a player next to a door they've never approached.
func straightRowPath(from, to core.Hex) []core.Hex {
	fp, tp := from.ToPosition(), to.ToPosition()
	step := 1.0
	if tp.X < fp.X {
		step = -1.0
	}
	var path []core.Hex
	for x := fp.X + step; ; x += step {
		path = append(path, core.HexFromPosition(spatial.Position{X: x, Y: fp.Y}))
		if x == tp.X {
			break
		}
	}
	return path
}

// TwoChamberSuite exercises InitTwoChamberRoom's fixed named-seed layout
// (slice2TwoChambersSeed) — a fresh Encounter per test.
type TwoChamberSuite struct {
	suite.Suite
	transport *encounter.InMemoryTransport
	broker    *encounter.Broker
	enc       *encounter.Encounter
}

func TestTwoChamberSuite(t *testing.T) {
	suite.Run(t, new(TwoChamberSuite))
}

func (s *TwoChamberSuite) SetupTest() {
	s.transport = encounter.NewInMemoryTransport()
	s.broker = encounter.NewBroker(s.transport)
	s.enc = encounter.New(context.Background(), "enc-two-chamber", s.broker)
	s.Require().NoError(s.enc.InitTwoChamberRoom(encounter.TwoChamberRoomParams{
		ChamberWidth:  chamberW,
		ChamberHeight: chamberH,
		Pattern:       environments.PatternRandom,
		RandomSeed:    slice2TwoChambersSeed,
		DoorID:        twoChamberDoorID,
	}))
}

func (s *TwoChamberSuite) TearDownTest() {
	_ = s.broker.Close()
	_ = s.transport.Close()
}

// TestClosedDoor_BlocksMovementAndLoS_BetweenChambers: with the door in
// its default closed state, nothing reachable from the entrance (via
// walkable hex-adjacency, exactly like Move's truncateAtWall) falls inside
// chamber 2's region, and a direct LoS check into chamber 2 is blocked.
func (s *TwoChamberSuite) TestClosedDoor_BlocksMovementAndLoS_BetweenChambers() {
	data := s.enc.ToData()
	entrance := data.Space.Entrance
	door := data.Doors[twoChamberDoorID].Position

	reachable := reachableFrom(s.enc.Room(), entrance)
	s.True(reachable[doorAdjacentChamber1Hex()],
		"entrance must reach up to the cell right before the door")
	s.False(reachable[door],
		"a closed door blocks its OWN cell (rpg-toolkit#790), so the door cell itself must not be reachable")

	chamber2 := regionHexSet(data.Space, encounter.RegionChamber2)
	s.Require().NotEmpty(chamber2)
	for h := range reachable {
		s.False(chamber2[h], "closed door must block all movement into chamber 2; reached %v", h)
	}

	s.True(
		s.enc.Room().IsLineOfSightBlocked(entrance.ToPosition(), doorAdjacentChamber2Hex().ToPosition()),
		"closed door must block LoS from chamber 1's entrance into chamber 2",
	)
}

// TestOpenDoor_ConnectsEntranceThroughDoorToChamber2: opening the door
// must connect the entrance to at least one chamber-2 hex via a walkable
// path, and clear LoS across the doorway — the gate's "connected walkable
// path entrance->door->chamber 2".
func (s *TwoChamberSuite) TestOpenDoor_ConnectsEntranceThroughDoorToChamber2() {
	data := s.enc.ToData()
	entrance := data.Space.Entrance

	s.Require().NoError(s.enc.AddPlayer(encounter.PlayerInput{
		PlayerID: alicePlayerID, EntityID: aliceEntityID,
		Position: entrance, SightRange: 30,
	}))
	// rpg-toolkit#864: OpenDoor requires adjacency — walk alice up to the
	// door along the generator's guaranteed wall-free required path first.
	s.Require().NoError(s.enc.Move(alicePlayerID, straightRowPath(entrance, doorAdjacentChamber1Hex())))
	s.Require().NoError(s.enc.OpenDoor(alicePlayerID, twoChamberDoorID))

	reachable := reachableFrom(s.enc.Room(), entrance)
	s.True(reachable[doorAdjacentChamber2Hex()],
		"opening the door must connect the entrance to chamber 2 via a walkable path")

	chamber2 := regionHexSet(s.enc.ToData().Space, encounter.RegionChamber2)
	reachedChamber2 := false
	for h := range reachable {
		if chamber2[h] {
			reachedChamber2 = true
			break
		}
	}
	s.True(reachedChamber2, "opening the door must connect the entrance to chamber 2's region")

	s.False(
		s.enc.Room().IsLineOfSightBlocked(entrance.ToPosition(), doorAdjacentChamber2Hex().ToPosition()),
		"an open door must no longer block LoS into chamber 2",
	)
}

// TestRegions_PartitionTheChambers: the two region tags are non-empty,
// disjoint, and match the notable cells (entrance in chamber 1, the
// door-adjacent cell in chamber 2); the door cell itself belongs to
// neither — it's the threshold between them.
func (s *TwoChamberSuite) TestRegions_PartitionTheChambers() {
	data := s.enc.ToData()
	s.Require().Len(data.Space.Regions, 2)

	chamber1 := regionHexSet(data.Space, encounter.RegionChamber1)
	chamber2 := regionHexSet(data.Space, encounter.RegionChamber2)
	s.Require().NotEmpty(chamber1)
	s.Require().NotEmpty(chamber2)
	s.Len(chamber1, chamberW*chamberH)
	s.Len(chamber2, chamberW*chamberH)

	for h := range chamber1 {
		s.False(chamber2[h], "chamber 1 and chamber 2 region hexes must be disjoint; %v is in both", h)
	}

	s.True(chamber1[data.Space.Entrance], "entrance must be tagged chamber-1")
	s.True(chamber2[doorAdjacentChamber2Hex()], "the door-adjacent chamber-2 cell must be tagged chamber-2")

	door := data.Doors[twoChamberDoorID].Position
	s.False(chamber1[door], "the door cell is the threshold — it must not be tagged chamber-1")
	s.False(chamber2[door], "the door cell is the threshold — it must not be tagged chamber-2")

	regionID, ok := data.Space.RegionAt(data.Space.Entrance)
	s.True(ok)
	s.Equal(encounter.RegionChamber1, regionID)
}

// TestRegionAt_NilReceiver: RegionAt is an exported helper hosts call
// directly off ToData().Space (Data.Space is nil for encounters with no
// spatial room, e.g. pre-wave-1 fixtures) — a nil *SpaceData must behave
// like a SpaceData with no regions ("", false), not panic.
func TestRegionAt_NilReceiver(t *testing.T) {
	var sd *encounter.SpaceData
	id, ok := sd.RegionAt(core.Hex{Q: 0, R: 0, S: 0})
	require.Equal(t, "", id)
	require.False(t, ok)
}

// TestLayout_DeterministicSeedVsEntropyDefault: the named fixture seed
// reproduces an identical layout (walls, entrance, door) across separate
// Encounters; seed 0 (entropy default) varies between calls, matching
// InitRoom's existing seeding contract (rpg-toolkit#787).
func (s *TwoChamberSuite) TestLayout_DeterministicSeedVsEntropyDefault() {
	build := func(seed int64) *encounter.Data {
		transport := encounter.NewInMemoryTransport()
		broker := encounter.NewBroker(transport)
		s.T().Cleanup(func() {
			_ = broker.Close()
			_ = transport.Close()
		})
		enc := encounter.New(context.Background(), "enc-two-chamber-determinism", broker)
		s.Require().NoError(enc.InitTwoChamberRoom(encounter.TwoChamberRoomParams{
			ChamberWidth:  chamberW,
			ChamberHeight: chamberH,
			Pattern:       environments.PatternRandom,
			RandomSeed:    seed,
			DoorID:        twoChamberDoorID,
		}))
		return enc.ToData()
	}

	a := build(slice2TwoChambersSeed)
	b := build(slice2TwoChambersSeed)
	s.Equal(a.Space.Walls, b.Space.Walls, "the slice2-two-chambers fixture seed must reproduce identical walls")
	s.Equal(a.Space.Entrance, b.Space.Entrance)
	s.Equal(a.Doors[twoChamberDoorID].Position, b.Doors[twoChamberDoorID].Position)

	// Entropy-default (seed 0) must vary between calls -- but at these
	// fixture dimensions, RandomPattern's own safety validation legitimately
	// falls back to an empty (walls-only-the-boundary) layout for a real
	// fraction of seeds (matching the documented InitRoom behavior,
	// space_test.go's spaceTestSeed comment: "an unseeded 10x10 PatternRandom
	// room only generates walls part of the time"). Two entropy draws can
	// coincidentally BOTH land on that empty layout, which is a correct
	// output, not a bug -- so this samples several independent pairs and
	// requires at least one differing pair, rather than asserting a single
	// draw always differs (which flakes on the documented coincidence).
	varied := false
	for i := 0; i < 5 && !varied; i++ {
		c := build(0)
		d := build(0)
		if !reflect.DeepEqual(c.Space.Walls, d.Space.Walls) {
			varied = true
		}
	}
	s.True(varied, "entropy-default (seed 0) must vary across independent InitTwoChamberRoom calls")
}
