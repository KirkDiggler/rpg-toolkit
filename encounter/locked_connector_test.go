package encounter_test

// locked_connector_test.go is the TDD gate for rpg-toolkit#815: extending
// InitDungeon's connector spec so a connector can declare its generated
// door Locked (+ LockDC/LockAbility/LockTool), reusing DoorData's existing
// Wave 2.9 lock-state fields verbatim rather than inventing a parallel
// representation. Scope is generation + validation + persistence + the
// existing AttemptUnlock/SubmitCheck/OpenDoor round-trip against a
// GENERATED locked door — no new verb/RPC (see prompts_test.go for the
// hand-mutated-DoorData half of that machinery's own gate).

import (
	"context"
	"encoding/json"
	"strings"
	"testing"

	"github.com/stretchr/testify/require"
	"github.com/stretchr/testify/suite"

	"github.com/KirkDiggler/rpg-toolkit/encounter"
	"github.com/KirkDiggler/rpg-toolkit/tools/environments"
)

// dungeonBossDoorLockDC is the fixture DC used by every locked-connector
// test in this file — an arbitrary but fixed value distinct from
// prompts_test.go's own fixture DCs (15, 17) only by coincidence; reusing
// abilityDEX/thievesTool from prompts_test.go (same package) keeps the
// lock-check vocabulary identical across both files.
const dungeonBossDoorLockDC = 15

// threeRegionDungeonParamsWithLockedBossDoor mirrors dungeon_test.go's
// threeRegionDungeonParams (entrance/corridor/boss, same widths/height/
// seed/theme/door IDs) but marks connector 1 (corridor -> boss) as a
// locked connector — the shape #815's done bar describes: one plain
// connector, one locked connector guarding the boss region.
func threeRegionDungeonParamsWithLockedBossDoor(seed int64) encounter.DungeonParams {
	params := threeRegionDungeonParams(seed)
	params.Connectors[1].Locked = true
	params.Connectors[1].LockDC = dungeonBossDoorLockDC
	params.Connectors[1].LockAbility = abilityDEX
	params.Connectors[1].LockTool = thievesTool
	return params
}

// --- Validation: locked connector config is checked contextually, before
// any mutation of encounter data (mirrors the file's other
// validate-then-reject gates, e.g. TestInitDungeon_RejectsDuplicateConnectorDoorIDs). ---

// TestInitDungeon_RejectsLockedConnectorWithoutDC: a locked connector with
// LockDC<=0 has no meaningful skill-check DC — AttemptUnlock would issue a
// prompt no roll could ever succeed against sensibly. Reject at generation
// time rather than producing an unwinnable door.
func TestInitDungeon_RejectsLockedConnectorWithoutDC(t *testing.T) {
	enc := newTestEncounter(t)
	params := validTwoRegionParams()
	params.Connectors[0].Locked = true
	params.Connectors[0].LockAbility = abilityDEX
	err := enc.InitDungeon(params)
	if err == nil {
		t.Fatal("InitDungeon with a locked connector and no LockDC: want error, got nil")
	}
	if !strings.Contains(err.Error(), "connector 0") {
		t.Errorf("error must identify the offending connector by index; got %q", err.Error())
	}
}

// TestInitDungeon_RejectsLockedConnectorWithoutAbility: a locked connector
// with no LockAbility has no ability for SubmitCheck's CharacterResolver to
// resolve a modifier against — reject at generation time.
func TestInitDungeon_RejectsLockedConnectorWithoutAbility(t *testing.T) {
	enc := newTestEncounter(t)
	params := validTwoRegionParams()
	params.Connectors[0].Locked = true
	params.Connectors[0].LockDC = dungeonBossDoorLockDC
	err := enc.InitDungeon(params)
	if err == nil {
		t.Fatal("InitDungeon with a locked connector and no LockAbility: want error, got nil")
	}
	if !strings.Contains(err.Error(), "connector 0") {
		t.Errorf("error must identify the offending connector by index; got %q", err.Error())
	}
}

// TestInitDungeon_RejectsInvalidLockedConnector_LeavesNoPartialState: an
// invalid locked-connector config must be caught by validateDungeonParams
// (called before any generation) and must not leave a partially-built
// Space or any staged doors behind — same atomicity contract
// TestInitDungeon_RejectsDuplicateConnectorDoorIDs pins for the other
// pre-mutation validation gate.
func TestInitDungeon_RejectsInvalidLockedConnector_LeavesNoPartialState(t *testing.T) {
	enc := newTestEncounter(t)
	params := threeRegionDungeonParams(dungeonSeed)
	params.Connectors[1].Locked = true // LockDC/LockAbility left zero-value: invalid

	err := enc.InitDungeon(params)
	if err == nil {
		t.Fatal("InitDungeon with an invalid locked connector: want error, got nil")
	}

	data := enc.ToData()
	if data.Space != nil {
		t.Error("a rejected InitDungeon call must not mutate encounter data (Space)")
	}
	if len(data.Doors) != 0 {
		t.Errorf("a rejected InitDungeon call must not stage any doors; got %d", len(data.Doors))
	}
}

// --- Generation: a locked connector produces a closed+locked door;
// plain connectors (and the two-chamber compatibility wrapper) stay
// unlocked. ---

// TestInitDungeon_GeneratesLockedBossDoor_ClosedAndLocked: connector 1's
// lock config must land on the generated DoorData verbatim (Locked,
// LockDC, LockAbility, LockTool), the door must start closed (mirrors
// every other generated door), and connector 0's plain door must remain
// entirely unlocked — a locked connector must not leak lock state onto
// its sibling.
func TestInitDungeon_GeneratesLockedBossDoor_ClosedAndLocked(t *testing.T) {
	enc := newTestEncounter(t)
	require.NoError(t, enc.InitDungeon(threeRegionDungeonParamsWithLockedBossDoor(dungeonSeed)))

	data := enc.ToData()
	door0 := data.Doors[dungeonDoor0ID]
	door1 := data.Doors[dungeonDoor1ID]

	require.False(t, door0.Locked, "connector 0 (plain) must generate an unlocked door")
	require.Equal(t, 0, door0.LockDC)
	require.Empty(t, door0.LockAbility)
	require.Empty(t, door0.LockTool)

	require.True(t, door1.Locked, "connector 1 (locked) must generate a locked door")
	require.False(t, door1.Open, "a generated locked door must start closed, same as a plain door")
	require.Equal(t, dungeonBossDoorLockDC, door1.LockDC)
	require.Equal(t, abilityDEX, door1.LockAbility)
	require.Equal(t, thievesTool, door1.LockTool)
}

// TestInitTwoChamberRoom_GeneratedDoor_RemainsUnlocked: the
// InitTwoChamberRoom compatibility wrapper never sets DungeonConnectorParams'
// new lock fields, so its zero-value Connectors entry must keep generating
// a plain unlocked door — a regression guard distinct from
// TestInitDungeon_GeneratesLockedBossDoor_ClosedAndLocked's "connector 0
// stays unlocked" assertion because this exercises the actual public
// entry point rpg-api still calls, not InitDungeon directly.
func TestInitTwoChamberRoom_GeneratedDoor_RemainsUnlocked(t *testing.T) {
	enc := newTestEncounter(t)
	require.NoError(t, enc.InitTwoChamberRoom(encounter.TwoChamberRoomParams{
		ChamberWidth: 8, ChamberHeight: 8, Pattern: environments.PatternEmpty, DoorID: "door-compat",
	}))
	door := enc.ToData().Doors["door-compat"]
	require.False(t, door.Locked, "InitTwoChamberRoom's compatibility wrapper must never generate a locked door")
}

// --- Persistence: a generated locked door round-trips through
// ToData/JSON/LoadFromData and still blocks afterward. ---

// TestInitDungeon_LockedBossDoor_RoundTripsThroughToDataLoadFromData
// rebuilds an Encounter from its own ToData() JSON snapshot (the
// LoadFromData path a host's Redis round-trip exercises on every call,
// mirroring reload_door_blocking_test.go's TestClosedDoor_StillBlocksAfterReload)
// and asserts a GENERATED locked door's lock state survives intact and
// still blocks movement/LoS on the reloaded instance.
func TestInitDungeon_LockedBossDoor_RoundTripsThroughToDataLoadFromData(t *testing.T) {
	transport := encounter.NewInMemoryTransport()
	defer func() { _ = transport.Close() }()
	broker := encounter.NewBroker(transport)
	defer func() { _ = broker.Close() }()

	enc := encounter.New(context.Background(), "enc-locked-persist", broker)
	require.NoError(t, enc.InitDungeon(threeRegionDungeonParamsWithLockedBossDoor(dungeonSeed)))

	payload, err := json.Marshal(enc.ToData())
	require.NoError(t, err)
	var data encounter.Data
	require.NoError(t, json.Unmarshal(payload, &data))

	reloaded, err := encounter.LoadFromData(context.Background(), &data, broker)
	require.NoError(t, err)

	door := reloaded.ToData().Doors[dungeonDoor1ID]
	require.True(t, door.Locked, "lock state must survive a ToData/JSON/LoadFromData round-trip")
	require.False(t, door.Open)
	require.Equal(t, dungeonBossDoorLockDC, door.LockDC)
	require.Equal(t, abilityDEX, door.LockAbility)
	require.Equal(t, thievesTool, door.LockTool)

	require.True(t,
		reloaded.Room().IsLineOfSightBlocked(
			dungeonRegionFarEdgeHex(1).ToPosition(), dungeonRegionNearEdgeHex(2).ToPosition()),
		"a generated locked door persisted via ToData must still block LoS after LoadFromData",
	)
}

// --- Integration: the full AttemptUnlock -> SubmitCheck -> OpenDoor loop
// against a GENERATED locked door (#815 done bar). ---

// LockedBossDoorSuite builds a fresh 3-region dungeon (entrance/corridor/
// boss) with connector 1 locked, one player at the entrance with a wired
// CharacterResolver, per test — mirrors DungeonSuite's fixture shape plus
// prompts_test.go's PromptsSuite resolver wiring.
type LockedBossDoorSuite struct {
	suite.Suite
	transport *encounter.InMemoryTransport
	broker    *encounter.Broker
	resolver  *fakeResolver
	enc       *encounter.Encounter
}

func TestLockedBossDoorSuite(t *testing.T) {
	suite.Run(t, new(LockedBossDoorSuite))
}

func (s *LockedBossDoorSuite) SetupTest() {
	s.transport = encounter.NewInMemoryTransport()
	s.broker = encounter.NewBroker(s.transport)
	// abilityMod=3, toolBonus=2: roll=1 totals 6 (fails DC 15), roll=20
	// totals 25 (succeeds) — the same fixed-resolver-plus-chosen-roll
	// determinism pattern prompts_test.go's PromptsSuite uses.
	s.resolver = &fakeResolver{abilityMod: 3, abilityOK: true, toolBonus: 2, toolOK: true}
	s.enc = encounter.New(
		context.Background(), "enc-locked-boss-door", s.broker, encounter.WithCharacterResolver(s.resolver),
	)
	s.Require().NoError(s.enc.InitDungeon(threeRegionDungeonParamsWithLockedBossDoor(dungeonSeed)))
}

func (s *LockedBossDoorSuite) TearDownTest() {
	_ = s.broker.Close()
	_ = s.transport.Close()
}

// TestFailedCheck_LeavesDoorLockedRecoverableAndBlocking: a failed
// SubmitCheck against a generated locked door must clear the pending
// prompt (existing SubmitCheck contract) but must NOT open or unlock the
// door — it must remain locked, closed, still blocking movement/LoS into
// the boss region, and recoverable (a second AttemptUnlock must succeed).
func (s *LockedBossDoorSuite) TestFailedCheck_LeavesDoorLockedRecoverableAndBlocking() {
	data := s.enc.ToData()
	entrance := data.Space.Entrance
	s.Require().NoError(s.enc.AddPlayer(encounter.PlayerInput{
		PlayerID: alicePlayerID, EntityID: aliceEntityID,
		Position: entrance, SightRange: 30,
	}))
	s.Require().NoError(s.enc.OpenDoor(alicePlayerID, dungeonDoor0ID))

	_, err := s.enc.AttemptUnlock(alicePlayerID, dungeonDoor1ID)
	s.Require().NoError(err)
	result, err := s.enc.SubmitCheck(alicePlayerID, 1) // total 6 < DC 15
	s.Require().NoError(err)
	s.False(result.Success)

	door1 := s.enc.ToData().Doors[dungeonDoor1ID]
	s.True(door1.Locked, "a failed check must leave the door locked")
	s.False(door1.Open, "a failed check must leave the door closed")

	reachable := reachableFrom(s.enc.Room(), entrance)
	boss := regionHexSet(data.Space, dungeonRegionIDBoss)
	for h := range reachable {
		s.False(boss[h], "a failed check must not open door 1; the boss region must remain unreachable")
	}
	s.True(
		s.enc.Room().IsLineOfSightBlocked(
			dungeonRegionFarEdgeHex(1).ToPosition(), dungeonRegionNearEdgeHex(2).ToPosition()),
		"a failed check must leave door 1 blocking LoS into the boss region",
	)

	_, err = s.enc.AttemptUnlock(alicePlayerID, dungeonDoor1ID)
	s.Require().NoError(err, "the door must remain recoverable for another AttemptUnlock after a failed check")
}

// TestSuccessfulCheck_UnlocksOpensAndRevealsBossRegion: a successful
// SubmitCheck against a generated locked door must clear Locked, open the
// door (reusing the existing dispatchPromptAction -> OpenDoor path), connect
// the entrance through to the boss region, clear LoS across the doorway,
// and reveal into the boss region through it — the full #815 done-bar loop.
func (s *LockedBossDoorSuite) TestSuccessfulCheck_UnlocksOpensAndRevealsBossRegion() {
	data := s.enc.ToData()
	entrance := data.Space.Entrance
	s.Require().NoError(s.enc.AddPlayer(encounter.PlayerInput{
		PlayerID: alicePlayerID, EntityID: aliceEntityID,
		Position: entrance, SightRange: 30,
	}))
	s.Require().NoError(s.enc.OpenDoor(alicePlayerID, dungeonDoor0ID))

	boss := regionHexSet(data.Space, dungeonRegionIDBoss)
	before := s.enc.ToData().Players[alicePlayerID].View.RevealedHexes
	s.False(before.Has(dungeonRegionNearEdgeHex(2)), "the boss chamber must not be revealed before unlocking door 1")

	_, err := s.enc.AttemptUnlock(alicePlayerID, dungeonDoor1ID)
	s.Require().NoError(err)
	result, err := s.enc.SubmitCheck(alicePlayerID, 20) // total 25 >= DC 15
	s.Require().NoError(err)
	s.True(result.Success)

	door1 := s.enc.ToData().Doors[dungeonDoor1ID]
	s.False(door1.Locked, "a successful check must clear Locked")
	s.True(door1.Open, "a successful check must open the door")

	reachable := reachableFrom(s.enc.Room(), entrance)
	reachedBoss := false
	for h := range reachable {
		if boss[h] {
			reachedBoss = true
			break
		}
	}
	s.True(reachedBoss, "a successful check must open door 1 and connect the entrance to the boss region")

	s.False(
		s.enc.Room().IsLineOfSightBlocked(
			dungeonRegionFarEdgeHex(1).ToPosition(), dungeonRegionNearEdgeHex(2).ToPosition()),
		"a successful check must clear LoS blocking into the boss region",
	)

	after := s.enc.ToData().Players[alicePlayerID].View.RevealedHexes
	s.True(after.Has(dungeonRegionNearEdgeHex(2)),
		"opening the boss door via a successful check must reveal the boss chamber through the doorway")
}
