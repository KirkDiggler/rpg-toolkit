package encounter

import (
	"fmt"

	"github.com/KirkDiggler/rpg-toolkit/encounter/core"
)

// Region IDs InitTwoChamberRoom assigns. Hosts (e.g. rpg-api's per-chamber
// monster seeding, design doc §Q5) key off these constants rather than
// hardcoding the strings.
const (
	// RegionChamber1 tags chamber 1's hexes — the entrance chamber.
	RegionChamber1 = "chamber-1"
	// RegionChamber2 tags chamber 2's hexes — across the door.
	RegionChamber2 = "chamber-2"
)

// TwoChamberRoomParams configures Encounter.InitTwoChamberRoom.
type TwoChamberRoomParams struct {
	// ChamberWidth and ChamberHeight size EACH chamber (not the combined
	// space) — minimum 4x4, the same floor RandomPattern's margin
	// heuristic needs to leave any interior open. The combined space's
	// Width is 2*ChamberWidth+1 (one column reserved for the shared
	// boundary + doorway); Height is ChamberHeight.
	ChamberWidth, ChamberHeight int

	// Pattern is the interior wall pattern generated independently for
	// EACH chamber (e.g. environments.PatternRandom for tactical cover,
	// environments.PatternEmpty for none). Defaults to
	// environments.PatternRandom when empty.
	Pattern string

	// RandomSeed reproduces the WHOLE layout (both chambers' interior
	// walls) when non-zero — entropy-seeded otherwise, matching InitRoom
	// (rpg-toolkit#787).
	RandomSeed int64

	// DoorID is the entity id for the plain door generated between the
	// two chambers. Required — mirrors AddDoor, which this composes with
	// internally.
	DoorID core.EntityID
}

// InitTwoChamberRoom builds a two-chamber dungeon: two chambers placed
// side by side in ONE continuous Space (design doc Fork 1 — chambers are
// a cheap region TAG on SpaceData, not separate spatial.Rooms), connected
// by a single plain door in their shared boundary wall, with a designated
// entrance cell in chamber 1 (SpaceData.Entrance — replaces the
// roomCenterHex() placeholder downstream, rpg-api#648) and per-chamber
// region tags (SpaceData.Regions) for spawn placement and, via LoS,
// combat pockets (rpg-toolkit#796).
//
// rpg-toolkit#814: this is now a thin compatibility wrapper over the
// generalized InitDungeon (N=2, chamber 1 tagged ArchetypeEntrance,
// chamber 2 tagged ArchetypeChamber) — the fixed-two-chamber wall-
// generation implementation this doc used to describe is retired in
// favor of the one generic N-region linear-chain generator. Kept as a
// separate public entry point (rather than removed outright) because
// rpg-api hasn't moved over to InitDungeon yet — see the issue's "Retire
// InitTwoChamberRoom once rpg-api moves over" note. Every behavioral
// guarantee below is unchanged: connectivity BY CONSTRUCTION, the same
// entrance/door/region placement, the same closed-by-default door via
// AddDoor.
func (e *Encounter) InitTwoChamberRoom(params TwoChamberRoomParams) error {
	if params.ChamberWidth < 4 || params.ChamberHeight < 4 {
		return fmt.Errorf("chamber dimensions must be at least 4x4 (got %dx%d)",
			params.ChamberWidth, params.ChamberHeight)
	}
	if params.DoorID == "" {
		return fmt.Errorf("door id required")
	}

	return e.InitDungeon(DungeonParams{
		Height:     params.ChamberHeight,
		RandomSeed: params.RandomSeed,
		Regions: []DungeonRegionParams{
			{ID: RegionChamber1, Archetype: ArchetypeEntrance, Width: params.ChamberWidth, Pattern: params.Pattern},
			{ID: RegionChamber2, Archetype: ArchetypeChamber, Width: params.ChamberWidth, Pattern: params.Pattern},
		},
		Connectors: []DungeonConnectorParams{{DoorID: params.DoorID}},
	})
}
