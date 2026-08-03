package encounter

import (
	"fmt"
	"sort"

	"github.com/KirkDiggler/rpg-toolkit/encounter/core"
	"github.com/KirkDiggler/rpg-toolkit/tools/environments"
	"github.com/KirkDiggler/rpg-toolkit/tools/spatial"
)

// PartyStartParams configures the toolkit-owned party-start reservation for a
// dungeon. Anchor is optional: nil retains InitDungeon's generated entrance.
// SeatCount is the number of ordered positions to reserve; zero keeps the
// legacy one-seat reservation at the resolved anchor. Hosts set SeatCount to
// their party capacity rather than treating any value as a toolkit-wide limit.
type PartyStartParams struct {
	// Anchor is an optional absolute encounter hex. It must be inside exactly
	// one region, never in a connector column, and must not collide with
	// authored blocking content.
	Anchor *core.Hex

	// SeatCount is the number of ordered party positions to reserve. Zero
	// reserves only Anchor for backwards-compatible direct InitDungeon callers.
	SeatCount int
}

// ResolvePartySpawnPositionsInput requests ordered party-start positions for
// one encounter startup.
type ResolvePartySpawnPositionsInput struct {
	// Count is the number of party members to seat.
	Count int
}

// ResolvePartySpawnPositionsOutput contains the first requested positions from
// the encounter's stored party-start reservation. Positions[0] is always the
// same hex as SpaceData.Entrance.
type ResolvePartySpawnPositionsOutput struct {
	Positions []core.Hex
}

// PartySpawnCapacityError reports an attempted party-start request larger than
// the initialized dungeon's stored reservation.
type PartySpawnCapacityError struct {
	Requested int
	Available int
}

// Error describes the requested and available party-start seat counts.
func (e *PartySpawnCapacityError) Error() string {
	return fmt.Sprintf(
		"party spawn request for %d position(s) exceeds %d reserved party-start seat(s)",
		e.Requested, e.Available,
	)
}

// ResolvePartySpawnPositions returns the first Count deterministic positions
// from the party-start reservation created by InitDungeon. It never searches,
// relocates, or falls back: a request larger than the available reservation
// returns PartySpawnCapacityError and no positions.
func (e *Encounter) ResolvePartySpawnPositions(
	in ResolvePartySpawnPositionsInput,
) (ResolvePartySpawnPositionsOutput, error) {
	if in.Count < 1 {
		return ResolvePartySpawnPositionsOutput{}, fmt.Errorf(
			"party spawn request count must be at least 1 (got %d)", in.Count)
	}
	if e.data.Space == nil {
		return ResolvePartySpawnPositionsOutput{}, fmt.Errorf("resolve party spawn positions: no dungeon initialized")
	}
	available := len(e.data.Space.PartyStartPositions)
	if in.Count > available {
		return ResolvePartySpawnPositionsOutput{}, &PartySpawnCapacityError{
			Requested: in.Count,
			Available: available,
		}
	}
	positions := append([]core.Hex(nil), e.data.Space.PartyStartPositions[:in.Count]...)
	return ResolvePartySpawnPositionsOutput{Positions: positions}, nil
}

type partyStartReservation struct {
	anchor        spatial.CubeCoordinate
	seats         []spatial.CubeCoordinate
	seatsByRegion []map[spatial.CubeCoordinate]struct{}
	regionIndex   int
}

// resolvePartyStartReservation selects every party seat from declared semantic
// floor cells before InitDungeon generates any walls or obstacles. Seat order is
// deterministic. A generated anchor first uses its existing entrance-to-door
// row, preserving the legacy generator's clear route; an authored anchor uses
// increasing hex distance, with offset row/column as stable tie-breakers. All
// seats remain in the anchor's semantic region, so a closed connector cannot
// split an assembling party.
func resolvePartyStartReservation(
	params DungeonParams, starts []int, totalWidth, doorRow int,
) (partyStartReservation, error) {
	seatCount := params.PartyStart.SeatCount
	if seatCount == 0 {
		seatCount = 1
	}

	anchor := spatial.OffsetCoordinateToCubeWithOrientation(
		spatial.Position{X: 0, Y: float64(doorRow)}, spatial.HexOrientationPointyTop)
	if params.PartyStart.Anchor != nil {
		anchor = params.PartyStart.Anchor.ToCube()
	}
	if anchor.X+anchor.Y+anchor.Z != 0 {
		return partyStartReservation{}, fmt.Errorf("party start anchor %v is not a valid cube coordinate", anchor)
	}
	anchorPosition := anchor.ToOffsetCoordinateWithOrientation(spatial.HexOrientationPointyTop)
	anchorCol, anchorRow := int(anchorPosition.X), int(anchorPosition.Y)
	if anchorCol < 0 || anchorCol >= totalWidth || anchorRow < 0 || anchorRow >= params.Height {
		return partyStartReservation{}, fmt.Errorf(
			"party start anchor at col=%d row=%d is outside dungeon footprint width=%d height=%d",
			anchorCol, anchorRow, totalWidth, params.Height)
	}

	regionIndex := -1
	for i, region := range params.Regions {
		if anchorCol < starts[i] || anchorCol >= starts[i]+region.Width {
			continue
		}
		if regionIndex != -1 {
			return partyStartReservation{}, fmt.Errorf(
				"party start anchor at col=%d row=%d belongs to more than one region", anchorCol, anchorRow)
		}
		regionIndex = i
	}
	if regionIndex == -1 {
		return partyStartReservation{}, fmt.Errorf(
			"party start anchor at col=%d row=%d is a connector gap, not a semantic room cell", anchorCol, anchorRow)
	}

	blockers := authoredPartyStartBlockers(params, starts)
	if blocker, blocked := blockers[anchor]; blocked {
		return partyStartReservation{}, fmt.Errorf(
			"party start anchor at col=%d row=%d collides with authored blocking %s", anchorCol, anchorRow, blocker)
	}

	type candidate struct {
		cube     spatial.CubeCoordinate
		col, row int
		distance int
	}
	candidates := make([]candidate, 0, params.Regions[regionIndex].Width*params.Height-1)
	for localCol := 0; localCol < params.Regions[regionIndex].Width; localCol++ {
		for row := 0; row < params.Height; row++ {
			col := starts[regionIndex] + localCol
			cube := spatial.OffsetCoordinateToCubeWithOrientation(
				spatial.Position{X: float64(col), Y: float64(row)}, spatial.HexOrientationPointyTop)
			if cube == anchor {
				continue
			}
			if _, blocked := blockers[cube]; blocked {
				continue
			}
			candidates = append(candidates, candidate{
				cube:     cube,
				col:      col,
				row:      row,
				distance: anchor.Distance(cube),
			})
		}
	}
	generatedAnchor := params.PartyStart.Anchor == nil
	sort.Slice(candidates, func(i, j int) bool {
		if generatedAnchor {
			leftOnRoute := candidates[i].row == doorRow
			rightOnRoute := candidates[j].row == doorRow
			if leftOnRoute != rightOnRoute {
				return leftOnRoute
			}
			if leftOnRoute && candidates[i].col != candidates[j].col {
				return candidates[i].col < candidates[j].col
			}
		}
		if candidates[i].distance != candidates[j].distance {
			return candidates[i].distance < candidates[j].distance
		}
		if candidates[i].row != candidates[j].row {
			return candidates[i].row < candidates[j].row
		}
		return candidates[i].col < candidates[j].col
	})
	if 1+len(candidates) < seatCount {
		return partyStartReservation{}, fmt.Errorf(
			"party start anchor at col=%d row=%d requires %d seats but region %q has only %d usable authored floor cells",
			anchorCol, anchorRow, seatCount, params.Regions[regionIndex].ID, 1+len(candidates))
	}

	reservation := partyStartReservation{
		anchor:        anchor,
		seats:         make([]spatial.CubeCoordinate, 0, seatCount),
		seatsByRegion: make([]map[spatial.CubeCoordinate]struct{}, len(params.Regions)),
		regionIndex:   regionIndex,
	}
	reservation.seatsByRegion[regionIndex] = make(map[spatial.CubeCoordinate]struct{}, seatCount)
	reservation.seats = append(reservation.seats, anchor)
	reservation.seatsByRegion[regionIndex][anchor] = struct{}{}
	for i := 0; i < seatCount-1; i++ {
		reservation.seats = append(reservation.seats, candidates[i].cube)
		reservation.seatsByRegion[regionIndex][candidates[i].cube] = struct{}{}
	}
	return reservation, nil
}

// authoredPartyStartBlockers returns the known authored cells a party seat
// cannot occupy before generated geometry exists: movement-blocking placed
// props and every ReservedCells marker, which dungeonspec uses for placed
// monsters and pinned bosses. Invalid direct InitDungeon inputs are left to
// their established placement validators later in generation.
func authoredPartyStartBlockers(
	params DungeonParams, starts []int,
) map[spatial.CubeCoordinate]string {
	blockers := make(map[spatial.CubeCoordinate]string)
	for i, region := range params.Regions {
		for _, placed := range region.PlacedObstacles {
			if !placed.BlocksMovement || !validLocalPartyStartCell(placed.At, region.Width, params.Height) {
				continue
			}
			cube := spatial.OffsetCoordinateToCubeWithOrientation(spatial.Position{
				X: float64(starts[i] + placed.At.Col), Y: float64(placed.At.Row),
			}, spatial.HexOrientationPointyTop)
			blockers[cube] = fmt.Sprintf("placed obstacle %q", placed.Ref)
		}
		for _, reserved := range region.ReservedCells {
			if !validLocalPartyStartCell(reserved, region.Width, params.Height) {
				continue
			}
			cube := spatial.OffsetCoordinateToCubeWithOrientation(spatial.Position{
				X: float64(starts[i] + reserved.Col), Y: float64(reserved.Row),
			}, spatial.HexOrientationPointyTop)
			blockers[cube] = "reserved monster or boss cell"
		}
	}
	return blockers
}

func validLocalPartyStartCell(cell LocalHex, width, height int) bool {
	return cell.Col >= 0 && cell.Col < width && cell.Row >= 0 && cell.Row < height
}

// requiredPathsForRegion extends the normal generator safety paths with the
// precomputed party envelope. The discrete strip below remains necessary
// because environments' continuous-line path check cannot prove every
// rounded hex cell is clear.
func (r partyStartReservation) requiredPathsForRegion(regionIndex, offsetX int) []environments.Path {
	if regionIndex != r.regionIndex || len(r.seats) < 2 {
		return nil
	}
	anchorPosition := r.anchor.ToOffsetCoordinateWithOrientation(spatial.HexOrientationPointyTop)
	from := spatial.Position{X: anchorPosition.X - float64(offsetX), Y: anchorPosition.Y}
	paths := make([]environments.Path, 0, len(r.seats)-1)
	for i, seat := range r.seats[1:] {
		seatPosition := seat.ToOffsetCoordinateWithOrientation(spatial.HexOrientationPointyTop)
		paths = append(paths, environments.Path{
			From:    from,
			To:      spatial.Position{X: seatPosition.X - float64(offsetX), Y: seatPosition.Y},
			Width:   dungeonPathWidth,
			Purpose: fmt.Sprintf("party-start-seat-%d", i+1),
		})
	}
	return paths
}

// stripPartyStartWalls enforces the precomputed party reservation at the
// discretized blocker boundary. It removes candidate pattern walls before they
// are added to the layout or handed to obstacle placement; it is not a seed
// retry, anchor relocation, or fallback placement.
func (r partyStartReservation) stripPartyStartWalls(
	regionIndex int, walls []environments.WallSegmentData,
) []environments.WallSegmentData {
	reserved := r.seatsByRegion[regionIndex]
	if len(reserved) == 0 {
		return walls
	}
	out := make([]environments.WallSegmentData, 0, len(walls))
	for _, wall := range walls {
		if _, protected := reserved[wall.Start]; protected {
			continue
		}
		out = append(out, wall)
	}
	return out
}

func (r partyStartReservation) positions() []core.Hex {
	positions := make([]core.Hex, len(r.seats))
	for i, seat := range r.seats {
		positions[i] = core.HexFromCube(seat)
	}
	return positions
}
