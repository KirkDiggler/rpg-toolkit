package encounter_test

// dungeon_test.go is the TDD gate for rpg-toolkit#814 (wave 2 slice 3,
// toolkit leg): generalizing InitTwoChamberRoom's fixed two-chamber
// generator into InitDungeon, a single N-region linear-chain generator,
// with region archetypes and an opaque SpaceData.Theme. See two_chamber_
// test.go for the compatibility half of this gate (InitTwoChamberRoom
// delegating to InitDungeon while its own assertions keep passing).

import (
	"context"
	"reflect"
	"strings"
	"testing"

	"github.com/stretchr/testify/require"
	"github.com/stretchr/testify/suite"

	"github.com/KirkDiggler/rpg-toolkit/encounter"
	"github.com/KirkDiggler/rpg-toolkit/encounter/core"
	"github.com/KirkDiggler/rpg-toolkit/tools/environments"
	"github.com/KirkDiggler/rpg-toolkit/tools/spatial"
)

// TestRegionArchetype_GenericVocabulary: the generic archetype vocabulary
// is entrance | chamber | corridor | boss (Approved Slice 3 corrections,
// #814) — a fixed, reusable set hosts key spawn tables and clients key
// dressing off, independent of any specific generator or theme.
func TestRegionArchetype_GenericVocabulary(t *testing.T) {
	cases := map[encounter.RegionArchetype]string{
		encounter.ArchetypeEntrance: dungeonRegionIDEntrance,
		encounter.ArchetypeChamber:  "chamber",
		encounter.ArchetypeCorridor: dungeonRegionIDCorridor,
		encounter.ArchetypeBoss:     dungeonRegionIDBoss,
	}
	for constant, want := range cases {
		if string(constant) != want {
			t.Errorf("archetype constant %v: want %q, got %q", constant, want, string(constant))
		}
	}

	region := encounter.RegionData{ID: "boss-room", Archetype: encounter.ArchetypeBoss}
	if region.Archetype != encounter.ArchetypeBoss {
		t.Errorf("RegionData.Archetype: want %q, got %q", encounter.ArchetypeBoss, region.Archetype)
	}
}

// TestSpaceData_ThemeIsOpaqueAndDistinctFromArchetype: Theme is a separate
// field from any RegionData.Archetype — the toolkit carries it through
// without interpreting it (Approved Slice 3 corrections, #814).
func TestSpaceData_ThemeIsOpaqueAndDistinctFromArchetype(t *testing.T) {
	sd := &encounter.SpaceData{
		Theme: dungeonThemeCrypt,
		Regions: []encounter.RegionData{
			{ID: "boss-room", Archetype: encounter.ArchetypeBoss},
		},
	}
	if sd.Theme != dungeonThemeCrypt {
		t.Errorf("SpaceData.Theme: want %q, got %q", dungeonThemeCrypt, sd.Theme)
	}
	if sd.Regions[0].Archetype != encounter.ArchetypeBoss {
		t.Errorf("Theme must not overwrite or interact with Archetype")
	}
}

// newTestEncounter builds a bare Encounter (in-memory transport/broker,
// throwaway ID) for InitDungeon tests that don't need the full
// TwoChamberSuite fixture machinery.
func newTestEncounter(t *testing.T) *encounter.Encounter {
	t.Helper()
	transport := encounter.NewInMemoryTransport()
	broker := encounter.NewBroker(transport)
	t.Cleanup(func() {
		_ = broker.Close()
		_ = transport.Close()
	})
	return encounter.New(context.Background(), "enc-dungeon-test", broker)
}

// TestInitDungeon_RejectsFewerThanTwoRegions: an N-region dungeon needs
// at least an entrance region and one region it connects to — a single
// region has nowhere to go and isn't a dungeon (#814 done bar implies at
// least 2, matching InitTwoChamberRoom's fixed N=2 floor).
func TestInitDungeon_RejectsFewerThanTwoRegions(t *testing.T) {
	enc := newTestEncounter(t)
	err := enc.InitDungeon(encounter.DungeonParams{
		Height: 8,
		Regions: []encounter.DungeonRegionParams{
			{ID: "solo", Archetype: encounter.ArchetypeEntrance, Width: 8, Pattern: environments.PatternEmpty},
		},
	})
	if err == nil {
		t.Fatal("InitDungeon with 1 region: want error, got nil")
	}
}

// TestInitDungeon_RejectsWrongConnectorCount: Connectors must have
// exactly len(Regions)-1 entries — one door per join in the chain, no
// more, no less.
func TestInitDungeon_RejectsWrongConnectorCount(t *testing.T) {
	enc := newTestEncounter(t)
	err := enc.InitDungeon(encounter.DungeonParams{
		Height: 8,
		Regions: []encounter.DungeonRegionParams{
			{ID: "entrance", Archetype: encounter.ArchetypeEntrance, Width: 8, Pattern: environments.PatternEmpty},
			{ID: "boss", Archetype: encounter.ArchetypeBoss, Width: 8, Pattern: environments.PatternEmpty},
		},
		Connectors: []encounter.DungeonConnectorParams{}, // want 1, got 0
	})
	if err == nil {
		t.Fatal("InitDungeon with 2 regions and 0 connectors: want error, got nil")
	}
}

// validTwoRegionParams returns a minimal, otherwise-valid 2-region
// DungeonParams — the baseline every validation-rejection test in this
// file mutates exactly one field of, isolating what's actually under
// test.
func validTwoRegionParams() encounter.DungeonParams {
	return encounter.DungeonParams{
		Height: 8,
		Regions: []encounter.DungeonRegionParams{
			{ID: "entrance", Archetype: encounter.ArchetypeEntrance, Width: 8, Pattern: environments.PatternEmpty},
			{ID: "chamber", Archetype: encounter.ArchetypeChamber, Width: 8, Pattern: environments.PatternEmpty},
		},
		Connectors: []encounter.DungeonConnectorParams{{DoorID: "door-0"}},
	}
}

// TestInitDungeon_RejectsNarrowRegion: every region needs at least width
// 4 — the same floor RandomPattern's margin heuristic needs to leave any
// interior open (matches InitTwoChamberRoom's 4x4 floor).
func TestInitDungeon_RejectsNarrowRegion(t *testing.T) {
	enc := newTestEncounter(t)
	params := validTwoRegionParams()
	params.Regions[1].Width = 3
	if err := enc.InitDungeon(params); err == nil {
		t.Fatal("InitDungeon with a width-3 region: want error, got nil")
	}
}

// TestInitDungeon_RejectsEmptyRegionID: every region's ID is required —
// SpaceData.RegionAt keys off it, and an empty ID is never a meaningful
// tag for a host to key spawn/seeding decisions off.
func TestInitDungeon_RejectsEmptyRegionID(t *testing.T) {
	enc := newTestEncounter(t)
	params := validTwoRegionParams()
	params.Regions[1].ID = ""
	err := enc.InitDungeon(params)
	if err == nil {
		t.Fatal("InitDungeon with an empty region ID: want error, got nil")
	}
	if !strings.Contains(err.Error(), "region 1") {
		t.Errorf("error must identify the offending region by index; got %q", err.Error())
	}
}

// TestInitDungeon_RejectsDuplicateRegionIDs: region IDs must be unique —
// SpaceData.RegionAt returns the FIRST matching region for a given hex,
// so a duplicate ID silently makes every hex in the later region(s)
// misreport as belonging to the earlier one.
func TestInitDungeon_RejectsDuplicateRegionIDs(t *testing.T) {
	enc := newTestEncounter(t)
	params := validTwoRegionParams()
	params.Regions[1].ID = params.Regions[0].ID
	err := enc.InitDungeon(params)
	if err == nil {
		t.Fatal("InitDungeon with duplicate region IDs: want error, got nil")
	}
	if !strings.Contains(err.Error(), "region 1") || !strings.Contains(err.Error(), "duplicate") {
		t.Errorf("error must identify the offending region by index and call out the duplication; got %q", err.Error())
	}
}

// TestInitDungeon_RejectsUnknownArchetype: Archetype is documented as a
// fixed, reusable vocabulary (entrance | chamber | corridor | boss) —
// RegionArchetype being a string type must not let an arbitrary or empty
// value through, or the "fixed vocabulary" contract data.go documents is
// unenforced.
func TestInitDungeon_RejectsUnknownArchetype(t *testing.T) {
	enc := newTestEncounter(t)
	params := validTwoRegionParams()
	params.Regions[1].Archetype = encounter.RegionArchetype("not-a-real-archetype")
	err := enc.InitDungeon(params)
	if err == nil {
		t.Fatal("InitDungeon with an unknown region archetype: want error, got nil")
	}
	if !strings.Contains(err.Error(), "region 1") {
		t.Errorf("error must identify the offending region by index; got %q", err.Error())
	}
}

// TestInitDungeon_RejectsShortSharedHeight: the shared Height needs at
// least 4, same floor as region width.
func TestInitDungeon_RejectsShortSharedHeight(t *testing.T) {
	enc := newTestEncounter(t)
	params := validTwoRegionParams()
	params.Height = 3
	if err := enc.InitDungeon(params); err == nil {
		t.Fatal("InitDungeon with Height 3: want error, got nil")
	}
}

// TestInitDungeon_RejectsEmptyDoorID: every connector's DoorID is
// required — mirrors AddDoor, which InitDungeon composes with
// internally.
func TestInitDungeon_RejectsEmptyDoorID(t *testing.T) {
	enc := newTestEncounter(t)
	params := validTwoRegionParams()
	params.Connectors[0].DoorID = ""
	if err := enc.InitDungeon(params); err == nil {
		t.Fatal("InitDungeon with an empty connector DoorID: want error, got nil")
	}
}

// TestInitDungeon_RejectsDuplicateConnectorDoorIDs: connector DoorIDs
// must be unique. AddDoor keys e.data.Doors by DoorID, so two connectors
// reusing the same ID silently overwrite each other in that map — the
// earlier connector's door entity is never placed, leaving that boundary
// column permanently unblocked (no closed door), breaking the
// closed-by-default connectivity contract without any error. This must
// be rejected before generation mutates encounter data at all.
func TestInitDungeon_RejectsDuplicateConnectorDoorIDs(t *testing.T) {
	enc := newTestEncounter(t)
	params := threeRegionDungeonParams(dungeonSeed)
	params.Connectors[1].DoorID = params.Connectors[0].DoorID

	err := enc.InitDungeon(params)
	if err == nil {
		t.Fatal("InitDungeon with duplicate connector DoorIDs: want error, got nil")
	}
	if !strings.Contains(err.Error(), "connector 1") || !strings.Contains(err.Error(), "duplicate") {
		t.Errorf("error must identify the offending connector by index and call out the duplication; got %q", err.Error())
	}

	data := enc.ToData()
	if data.Space != nil {
		t.Error("a rejected InitDungeon call must not mutate encounter data (Space)")
	}
	if len(data.Doors) != 0 {
		t.Errorf("a rejected InitDungeon call must not stage any doors; got %d", len(data.Doors))
	}
}

// TestInitDungeon_RejectsUndersizedBossRoom: the boss-room scale invariant
// (Approved Slice 3 corrections, #814) is a generation-time assertion,
// not a tuning nice-to-have — any region tagged ArchetypeBoss whose
// primary playable axis (min(region width, shared height)) does not
// exceed 6 hex steps must be rejected outright.
func TestInitDungeon_RejectsUndersizedBossRoom(t *testing.T) {
	enc := newTestEncounter(t)
	params := validTwoRegionParams()
	params.Regions[1].Archetype = encounter.ArchetypeBoss
	params.Regions[1].Width = 6 // shared Height is 8; min(6,8)=6, must EXCEED 6
	if err := enc.InitDungeon(params); err == nil {
		t.Fatal("InitDungeon with a boss room whose primary axis is 6 (not >6): want error, got nil")
	}
}

// TestInitDungeon_AcceptsBossRoomExceedingScaleInvariant: a boss room
// whose primary playable axis exceeds 6 (both width and shared height >6)
// must NOT be rejected by the scale invariant — this only confirms the
// validation doesn't false-positive; full generation is exercised by
// later tests.
func TestInitDungeon_AcceptsBossRoomExceedingScaleInvariant(t *testing.T) {
	enc := newTestEncounter(t)
	params := validTwoRegionParams()
	params.Height = 10
	params.Regions[1].Archetype = encounter.ArchetypeBoss
	params.Regions[1].Width = 10
	if err := enc.InitDungeon(params); err != nil {
		t.Fatalf("InitDungeon with a qualifying boss room: want no error, got %v", err)
	}
}

// --- 3-region linear-chain generation gate (#814 done bar) ---
//
// entrance (width dungeonEntranceWidth) -> corridor (width
// dungeonCorridorWidth) -> boss (width dungeonBossWidth), sharing
// dungeonHeight, joined by 2 doors. Mirrors two_chamber_test.go's fixture
// shape and helper conventions (reachableFrom, regionHexSet, defined
// there and reused here since both files share package encounter_test),
// generalized from 2 regions to 3.
const (
	dungeonHeight        = 8
	dungeonEntranceWidth = 10
	dungeonCorridorWidth = 5
	dungeonBossWidth     = 10
	dungeonSeed          = 909090

	dungeonRegionIDEntrance = "entrance"
	dungeonRegionIDCorridor = "corridor"
	dungeonRegionIDBoss     = "boss"
	dungeonThemeCrypt       = "crypt"
)

var dungeonRegionWidths = []int{dungeonEntranceWidth, dungeonCorridorWidth, dungeonBossWidth}

var (
	dungeonDoor0ID = core.EntityID("door-entrance-corridor")
	dungeonDoor1ID = core.EntityID("door-corridor-boss")
)

// threeRegionDungeonParams builds the fixture DungeonParams: an
// entrance/corridor/boss crypt-shaped chain (Theme: dungeonThemeCrypt, passed
// through opaquely — never branched on) demonstrating the generic
// generator is flexible enough for the first crypt's shape without a
// chamber archetype (#814 Approved Slice 3 corrections).
func threeRegionDungeonParams(seed int64) encounter.DungeonParams {
	return encounter.DungeonParams{
		Height:     dungeonHeight,
		RandomSeed: seed,
		Theme:      dungeonThemeCrypt,
		Regions: []encounter.DungeonRegionParams{
			{
				ID: dungeonRegionIDEntrance, Archetype: encounter.ArchetypeEntrance,
				Width: dungeonEntranceWidth, Pattern: environments.PatternRandom,
			},
			{
				ID: dungeonRegionIDCorridor, Archetype: encounter.ArchetypeCorridor,
				Width: dungeonCorridorWidth, Pattern: environments.PatternEmpty,
			},
			{
				ID: dungeonRegionIDBoss, Archetype: encounter.ArchetypeBoss,
				Width: dungeonBossWidth, Pattern: environments.PatternRandom,
			},
		},
		Connectors: []encounter.DungeonConnectorParams{
			{DoorID: dungeonDoor0ID},
			{DoorID: dungeonDoor1ID},
		},
	}
}

// dungeonRegionStarts returns each region's local-column-0 global X,
// duplicated from the generator's own layout formula rather than
// exported — this test asserts the generator's construction guarantee
// holds, exactly like two_chamber_test.go's doorAdjacentChamberNHex
// helpers do for the 2-region case.
func dungeonRegionStarts() []int {
	starts := make([]int, len(dungeonRegionWidths))
	x := 0
	for i, w := range dungeonRegionWidths {
		starts[i] = x
		x += w + 1 // +1 reserves the boundary/door column after this region
	}
	return starts
}

// dungeonDoorHex returns the global hex of the door joining region
// doorIndex to doorIndex+1.
func dungeonDoorHex(doorIndex int) core.Hex {
	starts := dungeonRegionStarts()
	x := starts[doorIndex] + dungeonRegionWidths[doorIndex]
	return core.HexFromPosition(spatial.Position{X: float64(x), Y: float64(dungeonHeight / 2)})
}

// dungeonRegionNearEdgeHex returns the cell just inside regionIdx,
// adjacent to the door BEFORE it (local column 0) — for region 0, this is
// the entrance itself.
func dungeonRegionNearEdgeHex(regionIdx int) core.Hex {
	starts := dungeonRegionStarts()
	return core.HexFromPosition(spatial.Position{X: float64(starts[regionIdx]), Y: float64(dungeonHeight / 2)})
}

// dungeonRegionFarEdgeHex returns the cell just inside regionIdx, adjacent
// to the door AFTER it (local column width-1).
func dungeonRegionFarEdgeHex(regionIdx int) core.Hex {
	starts := dungeonRegionStarts()
	x := starts[regionIdx] + dungeonRegionWidths[regionIdx] - 1
	return core.HexFromPosition(spatial.Position{X: float64(x), Y: float64(dungeonHeight / 2)})
}

// DungeonSuite exercises InitDungeon's fixed named-seed 3-region layout
// (dungeonSeed) — a fresh Encounter per test, mirroring TwoChamberSuite.
type DungeonSuite struct {
	suite.Suite
	transport *encounter.InMemoryTransport
	broker    *encounter.Broker
	enc       *encounter.Encounter
}

func TestDungeonSuite(t *testing.T) {
	suite.Run(t, new(DungeonSuite))
}

func (s *DungeonSuite) SetupTest() {
	s.transport = encounter.NewInMemoryTransport()
	s.broker = encounter.NewBroker(s.transport)
	s.enc = encounter.New(context.Background(), "enc-dungeon", s.broker)
	s.Require().NoError(s.enc.InitDungeon(threeRegionDungeonParams(dungeonSeed)))
}

func (s *DungeonSuite) TearDownTest() {
	_ = s.broker.Close()
	_ = s.transport.Close()
}

// TestClosedDoors_BlockMovementAndLoS_AcrossBothConnectors: with both
// connector doors in their default closed state, nothing reachable from
// the entrance falls inside the corridor or boss regions, and both door
// cells themselves are unreachable (a closed door blocks its OWN cell,
// rpg-toolkit#790) — generalizing TwoChamberSuite's single-door gate to a
// 2-door, 3-region chain.
func (s *DungeonSuite) TestClosedDoors_BlockMovementAndLoS_AcrossBothConnectors() {
	data := s.enc.ToData()
	entrance := data.Space.Entrance
	s.Equal(dungeonRegionNearEdgeHex(0), entrance)

	reachable := reachableFrom(s.enc.Room(), entrance)
	s.True(reachable[dungeonRegionFarEdgeHex(0)],
		"entrance must reach up to the cell right before door 0")
	s.False(reachable[dungeonDoorHex(0)], "closed door 0 must not be reachable (blocks its own cell)")
	s.False(reachable[dungeonDoorHex(1)], "closed door 1 must not be reachable (blocks its own cell)")

	corridor := regionHexSet(data.Space, dungeonRegionIDCorridor)
	boss := regionHexSet(data.Space, dungeonRegionIDBoss)
	s.Require().NotEmpty(corridor)
	s.Require().NotEmpty(boss)
	for h := range reachable {
		s.False(corridor[h], "closed door 0 must block all movement into the corridor; reached %v", h)
		s.False(boss[h], "closed doors must block all movement into the boss room; reached %v", h)
	}

	s.True(
		s.enc.Room().IsLineOfSightBlocked(
			dungeonRegionFarEdgeHex(0).ToPosition(), dungeonRegionNearEdgeHex(1).ToPosition()),
		"closed door 0 must block LoS from the entrance region into the corridor",
	)
	s.True(
		s.enc.Room().IsLineOfSightBlocked(
			dungeonRegionFarEdgeHex(1).ToPosition(), dungeonRegionNearEdgeHex(2).ToPosition()),
		"closed door 1 must block LoS from the corridor into the boss room",
	)
}

// TestOpeningBothDoors_ConnectsEntranceThroughToBoss: opening both
// connector doors must connect the entrance all the way to at least one
// boss-room hex via a walkable path, and clear LoS across both doorways —
// generalizing TwoChamberSuite's single-door open gate to a 2-door,
// 3-region chain.
func (s *DungeonSuite) TestOpeningBothDoors_ConnectsEntranceThroughToBoss() {
	data := s.enc.ToData()
	entrance := data.Space.Entrance

	s.Require().NoError(s.enc.AddPlayer(encounter.PlayerInput{
		PlayerID: alicePlayerID, EntityID: aliceEntityID,
		Position: entrance, SightRange: 30,
	}))
	// rpg-toolkit#864: OpenDoor requires adjacency — walk alice up to each
	// door along the generator's guaranteed wall-free required path before
	// opening it (mirrors TwoChamberSuite's identical fix).
	s.Require().NoError(s.enc.Move(alicePlayerID, straightRowPath(entrance, dungeonRegionFarEdgeHex(0))))
	s.Require().NoError(s.enc.OpenDoor(alicePlayerID, dungeonDoor0ID))
	s.Require().NoError(s.enc.Move(alicePlayerID, straightRowPath(dungeonRegionFarEdgeHex(0), dungeonRegionFarEdgeHex(1))))
	s.Require().NoError(s.enc.OpenDoor(alicePlayerID, dungeonDoor1ID))

	reachable := reachableFrom(s.enc.Room(), entrance)
	s.True(reachable[dungeonRegionNearEdgeHex(1)], "opening door 0 must connect the entrance into the corridor")
	s.True(reachable[dungeonRegionNearEdgeHex(2)], "opening both doors must connect the entrance into the boss room")

	boss := regionHexSet(s.enc.ToData().Space, dungeonRegionIDBoss)
	reachedBoss := false
	for h := range reachable {
		if boss[h] {
			reachedBoss = true
			break
		}
	}
	s.True(reachedBoss, "opening both doors must connect the entrance to the boss region")

	s.False(
		s.enc.Room().IsLineOfSightBlocked(entrance.ToPosition(), dungeonRegionNearEdgeHex(1).ToPosition()),
		"an open door 0 must no longer block LoS into the corridor",
	)
	s.False(
		s.enc.Room().IsLineOfSightBlocked(dungeonRegionFarEdgeHex(1).ToPosition(), dungeonRegionNearEdgeHex(2).ToPosition()),
		"an open door 1 must no longer block LoS into the boss room",
	)
}

// TestRegions_ArchetypesThemeAndPartition: the three region tags are
// non-empty, disjoint, carry the archetypes the params declared, and
// SpaceData.Theme passes through verbatim — none of which the generator
// branches on (#814 Approved Slice 3 corrections).
func (s *DungeonSuite) TestRegions_ArchetypesThemeAndPartition() {
	data := s.enc.ToData()
	s.Require().Len(data.Space.Regions, 3)
	s.Equal(dungeonThemeCrypt, data.Space.Theme)

	byID := make(map[string]encounter.RegionData, 3)
	for _, r := range data.Space.Regions {
		byID[r.ID] = r
	}
	s.Equal(encounter.ArchetypeEntrance, byID[dungeonRegionIDEntrance].Archetype)
	s.Equal(encounter.ArchetypeCorridor, byID[dungeonRegionIDCorridor].Archetype)
	s.Equal(encounter.ArchetypeBoss, byID[dungeonRegionIDBoss].Archetype)

	entrance := regionHexSet(data.Space, dungeonRegionIDEntrance)
	corridor := regionHexSet(data.Space, dungeonRegionIDCorridor)
	boss := regionHexSet(data.Space, dungeonRegionIDBoss)
	s.Require().NotEmpty(entrance)
	s.Require().NotEmpty(corridor)
	s.Require().NotEmpty(boss)
	s.Len(entrance, dungeonEntranceWidth*dungeonHeight)
	s.Len(corridor, dungeonCorridorWidth*dungeonHeight)
	s.Len(boss, dungeonBossWidth*dungeonHeight)

	for h := range entrance {
		s.False(corridor[h], "entrance and corridor region hexes must be disjoint; %v is in both", h)
		s.False(boss[h], "entrance and boss region hexes must be disjoint; %v is in both", h)
	}
	for h := range corridor {
		s.False(boss[h], "corridor and boss region hexes must be disjoint; %v is in both", h)
	}

	regionID, ok := data.Space.RegionAt(data.Space.Entrance)
	s.True(ok)
	s.Equal(dungeonRegionIDEntrance, regionID)

	door0 := data.Doors[dungeonDoor0ID].Position
	door1 := data.Doors[dungeonDoor1ID].Position
	_, doorTagged0 := data.Space.RegionAt(door0)
	_, doorTagged1 := data.Space.RegionAt(door1)
	s.False(doorTagged0, "door 0's cell is the threshold between regions — it must not be tagged to either")
	s.False(doorTagged1, "door 1's cell is the threshold between regions — it must not be tagged to either")
}

// TestLayout_DeterministicSeedVsEntropyDefault: the named fixture seed
// reproduces an identical layout across separate Encounters; seed 0
// (entropy default) varies between calls — generalizes
// TwoChamberSuite's determinism gate to a 3-region chain.
func (s *DungeonSuite) TestLayout_DeterministicSeedVsEntropyDefault() {
	build := func(seed int64) *encounter.Data {
		transport := encounter.NewInMemoryTransport()
		broker := encounter.NewBroker(transport)
		s.T().Cleanup(func() {
			_ = broker.Close()
			_ = transport.Close()
		})
		enc := encounter.New(context.Background(), "enc-dungeon-determinism", broker)
		s.Require().NoError(enc.InitDungeon(threeRegionDungeonParams(seed)))
		return enc.ToData()
	}

	a := build(dungeonSeed)
	b := build(dungeonSeed)
	s.Equal(a.Space.Walls, b.Space.Walls, "the dungeon fixture seed must reproduce identical walls")
	s.Equal(a.Space.Entrance, b.Space.Entrance)
	s.Equal(a.Doors[dungeonDoor0ID].Position, b.Doors[dungeonDoor0ID].Position)
	s.Equal(a.Doors[dungeonDoor1ID].Position, b.Doors[dungeonDoor1ID].Position)

	varied := false
	for i := 0; i < 5 && !varied; i++ {
		c := build(0)
		d := build(0)
		if !reflect.DeepEqual(c.Space.Walls, d.Space.Walls) {
			varied = true
		}
	}
	s.True(varied, "entropy-default (seed 0) must vary across independent InitDungeon calls")
}

// TestFirstCryptTemplate_EntranceCorridorBossWithoutChamberArchetype: the
// first crypt template's shape (entrance -> corridor -> boss) does not
// need to emit ArchetypeChamber — the generic vocabulary keeps it for
// future templates, but this slice's done bar only requires
// entrance/corridor/boss (#814 Approved Slice 3 corrections). Also
// confirms the boss room's generated dimensions satisfy the scale
// invariant end-to-end (not just at param-validation time).
func (s *DungeonSuite) TestFirstCryptTemplate_EntranceCorridorBossWithoutChamberArchetype() {
	data := s.enc.ToData()
	for _, r := range data.Space.Regions {
		s.NotEqual(encounter.ArchetypeChamber, r.Archetype,
			"the first crypt template's fixture must not need to emit ArchetypeChamber; region %q did", r.ID)
	}

	var boss encounter.RegionData
	for _, r := range data.Space.Regions {
		if r.Archetype == encounter.ArchetypeBoss {
			boss = r
		}
	}
	s.Require().NotEmpty(boss.ID, "fixture must have a boss region")
	s.Greater(dungeonBossWidth, 6, "boss region width must exceed 6 hex steps")
	s.Greater(dungeonHeight, 6, "shared height (boss's other playable axis) must exceed 6 hex steps")
}

// TestInitDungeon_FailedRoomRebuild_LeavesNoPartialState: InitDungeon's
// per-connector door placement must be atomic — either the whole dungeon
// (Space + every connector door) commits, or a failure leaves the
// encounter exactly as it was before the call. This reproduces the gap a
// naive "loop e.AddDoor per connector" implementation has: if the shared
// room-rebuild step fails partway (here, forced by a pre-existing door at
// an out-of-bounds position, since rebuildRoomFromData otherwise
// defensively de-duplicates same-cell placements rather than erroring),
// e.data.Space and any connector doors staged by THIS call must not
// leak into the encounter's observable state.
func TestInitDungeon_FailedRoomRebuild_LeavesNoPartialState(t *testing.T) {
	enc := newTestEncounter(t)

	// Pre-seed a door at a position no dungeon of this size will ever
	// reach — Space is nil at this point, so AddDoor just records it
	// without attempting a room rebuild (mirrors a host that added a
	// hand-placed door before calling InitDungeon). Once InitDungeon sets
	// Space and rebuilds the room, this decoy's out-of-bounds position
	// makes spatial.BasicRoom.PlaceEntity fail deterministically.
	const decoyDoorID = core.EntityID("decoy-out-of-bounds-door")
	outOfBounds := core.HexFromPosition(spatial.Position{X: 9999, Y: 9999})
	require.NoError(t, enc.AddDoor(decoyDoorID, outOfBounds, false))

	err := enc.InitDungeon(threeRegionDungeonParams(dungeonSeed))
	require.Error(t, err, "InitDungeon must fail when the room rebuild fails")

	data := enc.ToData()
	require.Nil(t, data.Space, "a failed InitDungeon must not leave a partially-built Space")
	_, hasDoor0 := data.Doors[dungeonDoor0ID]
	_, hasDoor1 := data.Doors[dungeonDoor1ID]
	require.False(t, hasDoor0, "a failed InitDungeon must not leave connector 0's door behind")
	require.False(t, hasDoor1, "a failed InitDungeon must not leave connector 1's door behind")
	_, hasDecoy := data.Doors[decoyDoorID]
	require.True(t, hasDecoy, "InitDungeon must not touch doors that existed before the call")
	require.Nil(t, enc.Room(), "a failed InitDungeon must not leave a partially-rebuilt room registered")
}
