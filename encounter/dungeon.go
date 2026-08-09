package encounter

import (
	"context"
	"fmt"
	"math"
	"math/rand"

	"github.com/KirkDiggler/rpg-toolkit/encounter/core"
	"github.com/KirkDiggler/rpg-toolkit/tools/environments"
	"github.com/KirkDiggler/rpg-toolkit/tools/spatial"
)

// dungeonPathWidth is the corridor width reserved for the required
// connectivity paths within each region, and the MinPathWidth fed to each
// region's wall generation — matches RandomPattern's own margin heuristic
// (generateRandomWall: margin = max(2, MinPathWidth)), so the reserved
// path and the walls it clears agree on scale. Mirrors two_chamber.go's
// chamberPathWidth (rpg-toolkit#806) — retired in favor of this shared
// constant now that InitTwoChamberRoom delegates to InitDungeon.
const dungeonPathWidth = 2.0

// DungeonParams configures Encounter.InitDungeon with either an explicit canvas
// floor source or a legacy ordered linear chain of Regions joined by Connectors.
// Both modes emit ONE continuous Space (design doc Fork 1 — regions are tags on
// a Space, not separate spatial.Rooms). The room-chain mode generalizes
// TwoChamberRoomParams (rpg-toolkit#806) from a fixed N=2 to any N>=2
// (rpg-toolkit#814).
type DungeonParams struct {
	// FloorSource selects canvas dimensions or legacy room-chain geometry.
	// The zero value retains legacy room-chain semantics.
	FloorSource FloorSourceKind

	// Width is required only for canvas floors; room-chain width derives from Regions.
	Width int

	// FloorCells is the complete canonical canvas floor. Nil retains the v0.3
	// bounds rectangle for direct/legacy callers; dungeonspec always supplies
	// an explicit sorted snapshot for newly compiled canvas documents.
	FloorCells []core.Hex

	// EnvelopeEdges is the canonical generated boundary between FloorCells and
	// void/off-canvas neighbors. It is persisted verbatim into SpaceData.
	EnvelopeEdges []GeneratedEdge

	// RequireConnectedFloor marks region-union candidates that must pass the
	// runnable floor gate whenever InitDungeon is attempted. It is not a runtime
	// mechanics authority: persisted FloorCells and the masked grid are.
	RequireConnectedFloor bool

	// AbsolutePlacedObstacles and AbsoluteReservedCells are absolute authored
	// content accepted only with FloorSourceCanvas. The latter reserves absolute
	// monster spawns from party seating before any geometry is generated.
	AbsolutePlacedObstacles []AbsolutePlacedObstacleSpec
	AbsoluteReservedCells   []AbsoluteReservedCell

	// Key is the stable authored dungeon key. It is required only when
	// AuthoredEdges is non-empty because a door edge derives its stable DoorID
	// from this key plus its normalized absolute endpoint pair.
	Key string

	// Regions is the ordered linear chain, entrance-side first. At least 2
	// required — a single region has nowhere to connect to.
	Regions []DungeonRegionParams

	// SemanticRegions are canvas-authored scopes. They never route through the
	// room-chain generator and cannot create walls, doors, or content.
	SemanticRegions []SemanticRegionParams

	// Connectors is the ordered list of doors joining consecutive regions:
	// Connectors[i] joins Regions[i] to Regions[i+1]. Must have exactly
	// len(Regions)-1 entries.
	Connectors []DungeonConnectorParams

	// Height is shared by every region in the chain — the combined space's
	// grid height. Matches TwoChamberRoomParams.ChamberHeight generalized:
	// the old generator used one ChamberHeight for both chambers; this
	// keeps that simplicity rather than inventing per-region vertical
	// offsets/padding, which isn't required by any #814 done-bar case.
	// Minimum 4, the same floor RandomPattern's margin heuristic needs.
	Height int

	// RandomSeed reproduces the WHOLE layout (every region's interior
	// walls) when non-zero — entropy-seeded otherwise, matching InitRoom /
	// InitTwoChamberRoom (rpg-toolkit#787).
	RandomSeed int64

	// PartyStart configures the resolved party-start anchor and ordered
	// reservation. It is separate from per-region ReservedCells because a
	// party start may validly occupy a room's door row.
	PartyStart PartyStartParams

	// AuthoredEdges is the caller-compiled, normalized dungeon-owned edge
	// collection. Both endpoints must be adjacent semantic floor cells; a door
	// carries the exact AuthoredDoorID derived from Key. InitDungeon persists
	// their closed/unlocked DoorData; every rebuild registers solids and closed
	// authored doors as spatial boundaries without occupying either endpoint.
	AuthoredEdges []AuthoredEdge

	// Theme is opaque metadata copied verbatim to SpaceData.Theme — see
	// that field's doc. Never interpreted here.
	Theme string
}

// AbsolutePlacedObstacleSpec pins an authored prop at an absolute floor cell.
type AbsolutePlacedObstacleSpec struct {
	ID             core.EntityID
	Ref            string
	At             core.Hex
	BlocksMovement bool
	BlocksLoS      bool
	Facing         *uint32
}

// AbsoluteReservedCell names authored content (currently an absolute monster
// spawn) that must not be used by the party-start envelope.
type AbsoluteReservedCell struct {
	At   core.Hex
	Name string
}

// DungeonRegionParams configures one region in an InitDungeon chain.
type DungeonRegionParams struct {
	// ID is this region's tag in SpaceData.Regions / RegionAt. Caller-
	// assigned so hosts control naming (e.g. "entrance", "corridor",
	// "boss", or the legacy "chamber-1"/"chamber-2" InitTwoChamberRoom
	// preserves).
	ID string

	// Archetype identifies the region's generic role. See RegionArchetype.
	Archetype RegionArchetype

	// Width sizes this region (the shared Height comes from
	// DungeonParams.Height). Minimum 4, the same floor RandomPattern's
	// margin heuristic needs.
	Width int

	// Pattern is the interior wall pattern generated independently for
	// this region (e.g. environments.PatternRandom for tactical cover,
	// environments.PatternEmpty for none). Defaults to
	// environments.PatternRandom when empty.
	Pattern string

	// Obstacles are generic, content-agnostic physical set-piece specs
	// InitDungeon places into THIS region's floor as rpg-toolkit#818
	// ObstacleData instances (rpg-toolkit#819). Nil/empty for a region
	// with no set pieces — matches every #814/#817/#818 fixture, which
	// never sets this field and gets zero obstacles. InitDungeon never
	// interprets Ref/BlocksMovement/BlocksLoS; a themed caller (e.g. the
	// crypt template — see CryptDungeonParams) decides content, this
	// package only places it safely. See placeRegionObstacles for the
	// placement algorithm and its safety invariants.
	Obstacles []ObstacleSpec

	// PlacedObstacles pin specific obstacle instances to exact room-local
	// cells — verbatim placement, not a candidate-pool roll (design.md
	// §Design delta). Unlike Obstacles (best-effort: a region whose safe
	// floor can't fit every requested instance places as many as fit),
	// a PlacedObstacleSpec is a hard guarantee: InitDungeon fails outright
	// if any entry lands on the reserved doorRow, collides with another
	// placed entry, or lands on a wall cell — see placeRegionObstacles.
	// Authored-edge endpoints remain valid ordinary-prop cells.
	// Placed cells are excluded from Obstacles' rolled candidate pool, so
	// the two mechanisms coexist in the same region without collision.
	PlacedObstacles []PlacedObstacleSpec

	// ReservedCells are cells excluded from rolled-obstacle placement
	// WITHOUT emitting any obstacle data for them (the compiler reserves
	// placed-monster/boss cells here) — rpg-toolkit#842 gate finding: a
	// place: monster entry (or a pinned boss.at) compiles to a
	// SpawnInstruction, never a PlacedObstacleSpec, so without this field
	// the rolled-obstacle draw below has no idea that cell is already
	// spoken for and can roll a count-based obstacle directly onto it —
	// surfacing much later and far more confusingly as
	// Encounter.SeedMonsters failing to place the monster into a cell an
	// obstacle already occupies, on some fraction of seeds rather than
	// deterministically. Validated the same way as PlacedObstacles (bounds,
	// doorRow, wall cell, collision with a PlacedObstacles entry) — see
	// reserveCubes — but a reserved cell is otherwise inert: nothing is
	// added to SpaceData.Obstacles for it, since whatever occupies it is
	// the caller's responsibility to place (dungeonspec's SeedMonsters
	// call, for the case this field exists to fix).
	ReservedCells []LocalHex
}

// ObstacleSpec describes one kind of physical set-piece instance a caller
// wants InitDungeon to place into a specific region's floor —
// rpg-toolkit#819. Count instances sharing the same opaque Ref/
// BlocksMovement/BlocksLoS are each placed at their own safe hex within
// the region (never a wall, door, required-path, or primary-combat-axis
// cell — see placeRegionObstacles). Content-agnostic: encounter never
// interprets any field. Placement is best-effort: a region whose safe
// floor can't fit every requested instance places as many as DID fit
// (down to zero) rather than failing InitDungeon — rpg-toolkit#819's
// "a crypt missing one statue is fine; a crypt that fails to generate
// ... is not" done-bar requirement, generalized to any caller.
type ObstacleSpec struct {
	// Ref is copied verbatim into each placed ObstacleData.Ref — an
	// opaque content identifier (e.g. "dnd5e:props:pillar") this
	// package never interprets.
	Ref string

	// Count is how many instances of this spec to attempt to place.
	Count int

	// BlocksMovement/BlocksLoS are copied verbatim into each placed
	// ObstacleData.
	BlocksMovement bool
	BlocksLoS      bool

	// PreferBorder opts this spec into placeRegionObstacles' border-
	// hugging draw-order bias (rpg-toolkit#839's composition ask,
	// rpg-dnd5e-web#469: "dressing hugs walls/corners; floor centers
	// stay clear; focal pieces centered") — false by default, matching
	// every pre-#839 caller/fixture's original uniform-random placement
	// exactly (see placeRegionObstacles' doc: the zero-value case is a
	// byte-identical code path to the mechanism before #839, not merely
	// an equivalent one).
	//
	// This is PER-SPEC, not per-region (rpg-toolkit#840 gate finding):
	// an earlier revision applied the bias to every spec in a region
	// regardless of intent, which forced FOCAL pieces (a region's own
	// obelisk/coffin/altar/statues, always listed first) onto the
	// border before dressing ever got a turn — inverting the art
	// target instead of achieving it. Set true only on dressing/light-
	// anchor specs (e.g. CryptDungeonParams' brazier/torch-ornate/
	// candles/bone-pile/chain/skeleton-remains); leave false on focal/
	// structural specs so they draw uniformly from whatever the
	// border-preferring specs left over, which in practice biases them
	// toward the interior/center precisely because the borders filled
	// up first — without this package ever hard-coding "focal pieces
	// go in the center," a claim placeRegionObstacles has no way to
	// verify structurally.
	PreferBorder bool
}

// FacingEast is the canonical east-facing hex-direction index. These six
// values are intentionally aligned with the canonical YAML labels: E, NE, NW,
// W, SW, SE. They are metadata only; encounter geometry and collision do not
// interpret them.
const (
	FacingEast uint32 = iota
	// FacingNortheast is the canonical northeast-facing hex-direction index.
	FacingNortheast
	// FacingNorthwest is the canonical northwest-facing hex-direction index.
	FacingNorthwest
	// FacingWest is the canonical west-facing hex-direction index.
	FacingWest
	// FacingSouthwest is the canonical southwest-facing hex-direction index.
	FacingSouthwest
	// FacingSoutheast is the canonical southeast-facing hex-direction index.
	FacingSoutheast
)

// LocalHex is a region-local (pre-offsetX) grid cell: Col in [0,width),
// Row in [0,height) (the dungeon-shared height) — exactly the local (x,y)
// frame regionObstacleCandidates already scans (see that function's doc).
type LocalHex struct{ Col, Row int }

// String implements fmt.Stringer so error messages print "col=6 row=4"
// instead of Go's default struct format ("{6 4}"), which doesn't name
// which number is which.
func (h LocalHex) String() string {
	return fmt.Sprintf("col=%d row=%d", h.Col, h.Row)
}

// PlacedObstacleSpec pins one obstacle instance to an exact room-local cell
// — verbatim placement, not a candidate-pool roll (design.md §Design delta).
// encounter never interprets Ref, mirroring ObstacleSpec's existing
// content-agnostic contract.
type PlacedObstacleSpec struct {
	// Ref is copied verbatim into the placed ObstacleData.Ref — same
	// opaque-content-identifier contract as ObstacleSpec.Ref.
	Ref string

	// At is this instance's room-local cell. InitDungeon rejects At out of
	// [0,width)x[0,height) bounds, At.Row == doorRow, a cell already
	// claimed by another PlacedObstacleSpec in the same region, or a wall
	// cell — placed entries are guarantees, not best-effort (design.md
	// §Validation).
	At LocalHex

	// BlocksMovement/BlocksLoS are copied verbatim into the placed
	// ObstacleData, same as ObstacleSpec's fields of the same name.
	BlocksMovement bool
	BlocksLoS      bool

	// Facing is optional authored hex-facing metadata. A nil value is absent;
	// a non-nil pointer to FacingEast is the explicit E = 0 override. It is
	// copied to ObstacleData without affecting position or collision behavior.
	Facing *uint32
}

// DungeonConnectorParams configures the door joining two consecutive
// regions in an InitDungeon chain.
type DungeonConnectorParams struct {
	// DoorID is the entity id for the door generated at this connector.
	// Required — mirrors AddDoor, which InitDungeon composes with
	// internally.
	DoorID core.EntityID

	// Locked marks this connector's generated door as closed AND locked
	// (rpg-toolkit#815), reusing DoorData's existing Wave 2.9 lock-state
	// fields verbatim: AttemptUnlock/SubmitCheck become the path through
	// it (issuing and resolving a skill-check prompt) until a player
	// passes the configured check; OpenDoor alone does not gate on it —
	// see DoorData.Locked's doc for the full contract. Zero value
	// (false) generates a plain closed door, same as every connector
	// before #815 and the InitTwoChamberRoom compatibility wrapper,
	// which never sets this field.
	Locked bool

	// LockDC is the skill-check DC AttemptUnlock issues when Locked is
	// true, copied verbatim onto the generated door's DoorData.LockDC.
	// Required (> 0) when Locked is true — validateDungeonParams rejects
	// a locked connector with LockDC<=0 before any generation runs.
	// Ignored when Locked is false.
	LockDC int

	// LockAbility is an opaque ability identifier owned by the host/rulebook,
	// copied verbatim onto DoorData.LockAbility. For D&D 5e, use the canonical
	// lowercase abilities.DEX identifier ("dex"). Required (non-empty) when
	// Locked is true — validateDungeonParams rejects a locked connector with
	// an empty LockAbility. Ignored when Locked is false.
	LockAbility string

	// LockTool is an optional toolkit ref (e.g.
	// "dnd5e:item:thieves-tools") granting a tool-proficiency bonus on
	// the check, copied verbatim onto DoorData.LockTool. Empty means no
	// tool bonus applies; never required. Ignored when Locked is false.
	LockTool string
}

// InitDungeon builds an N-region linear-chain dungeon: an ordered list of
// regions placed side by side in ONE continuous Space, each pair of
// consecutive regions joined by a plain door, with a designated entrance
// cell in the first region (SpaceData.Entrance) and per-region archetype-
// tagged regions (SpaceData.Regions) for spawn placement and, via LoS,
// combat pockets. Generalizes InitTwoChamberRoom (rpg-toolkit#806) from a
// fixed two chambers to any N>=2 regions (rpg-toolkit#814).
//
// InitDungeon is atomic: either the whole dungeon (Space + every
// connector door) commits, or a failure leaves the encounter exactly as
// it was before the call. Connector doors are staged directly into
// e.data.Doors and committed via a SINGLE rebuildRoomFromData call —
// rather than looping e.AddDoor once per connector, which would rebuild
// the room N-1 times and, if a LATER connector's door failed to place,
// leave the earlier connectors' doors (and the freshly-set Space) behind
// despite returning an error. rebuildRoomFromData only swaps in
// e.room/e.roomOrchestrator (via registerRoom) after every wall/door in
// the batch places successfully, so a failure here never touches the
// room side; restoring data.Space and removing the doors staged by this
// call below completes the rollback.
func (e *Encounter) InitDungeon(params DungeonParams) error {
	if err := validateDungeonParams(params); err != nil {
		return err
	}
	authoredEdges, err := validateAndNormalizeAuthoredEdges(params)
	if err != nil {
		return err
	}
	if err := validateAuthoredDoorIDsAgainstConnectors(params.Connectors, authoredEdges); err != nil {
		return err
	}
	// All doors already held by this encounter predate the replacement Space,
	// so none may be treated as one of this call's newly staged authored doors.
	if err := validateClosedLegacyDoorsAtAuthoredEndpoints(
		e.data.Doors, authoredEndpointCubes(authoredEdges), nil,
	); err != nil {
		return fmt.Errorf("init dungeon: %w", err)
	}
	if err := validateDungeonDoorIDsAvailable(e.data.Doors, params.Connectors, authoredEdges); err != nil {
		return err
	}
	// Generation consumes the normalized records to remove only legacy wall
	// cell geometry from their endpoints. Props, actors, and party starts are
	// intentionally not reserved by authored-edge geometry.
	params.AuthoredEdges = authoredEdges

	layout, err := generateDungeonLayout(params)
	if err != nil {
		return fmt.Errorf("generate dungeon layout: %w", err)
	}

	dungeonKey := ""
	if len(authoredEdges) > 0 {
		dungeonKey = params.Key
	}
	source, _ := floorSourceKind(params.FloorSource)
	space := &SpaceData{
		Walls:                 layout.walls,
		Width:                 layout.width,
		Height:                params.Height,
		FloorSource:           source,
		FloorCells:            nil,
		EnvelopeEdges:         nil,
		RequireConnectedFloor: params.RequireConnectedFloor,
		Entrance:              core.HexFromCube(layout.entrance),
		PartyStartPositions:   layout.partyStartPositions,
		DungeonKey:            dungeonKey,
		AuthoredEdges:         authoredEdges,
		Regions:               layout.regions,
		SemanticRegions:       layout.semanticRegions,
		Theme:                 params.Theme,
		Obstacles:             layout.obstacles,
	}
	if source == FloorSourceCanvas {
		space.FloorCells = canonicalCanvasFloorCells(params)
		space.EnvelopeEdges = append([]GeneratedEdge(nil), params.EnvelopeEdges...)
		if params.EnvelopeEdges == nil {
			space.EnvelopeEdges = canvasEnvelopeEdges(canvasFloorForParams(params))
		}
	}

	// Validate overlay against the generated connector records before mutating
	// the real encounter. Physical non-connector records are intentionally not
	// removed here: SpaceData keeps generator truth intact and DescribeEdges
	// applies the deterministic authored replacement projection.
	generatedDoors := make(map[core.EntityID]*DoorData, len(layout.doors))
	for i, door := range layout.doors {
		connector := params.Connectors[i]
		generatedDoors[connector.DoorID] = &DoorData{
			ID:          connector.DoorID,
			Position:    core.HexFromCube(door),
			Open:        false,
			Locked:      connector.Locked,
			LockDC:      connector.LockDC,
			LockAbility: connector.LockAbility,
			LockTool:    connector.LockTool,
		}
	}
	provisional := &Encounter{data: &Data{Space: space, Doors: generatedDoors}}
	if _, err := provisional.canonicalGeneratedEdgeRecordsWithOverlay(authoredEdgesByKey(space)); err != nil {
		return fmt.Errorf("init dungeon: generated edge overlay: %w", err)
	}

	previousSpace := e.data.Space
	e.data.Space = space
	stagedDoorIDs := make([]core.EntityID, 0, len(layout.doors)+len(authoredEdges))
	for i := range layout.doors {
		connector := params.Connectors[i]
		dd := generatedDoors[connector.DoorID]
		e.data.Doors[connector.DoorID] = dd
		stagedDoorIDs = append(stagedDoorIDs, connector.DoorID)
	}
	for _, edge := range authoredEdges {
		if edge.Kind != GeneratedEdgeKindDoor {
			continue
		}
		// Position remains the normalized first endpoint for DoorData wire
		// compatibility. AuthoredEdge remains the edge-native authority for
		// boundary registration and interaction from either endpoint.
		e.data.Doors[edge.DoorID] = &DoorData{ID: edge.DoorID, Position: edge.From, Open: false}
		stagedDoorIDs = append(stagedDoorIDs, edge.DoorID)
	}

	if err := e.rebuildRoomFromData(); err != nil {
		for _, id := range stagedDoorIDs {
			delete(e.data.Doors, id)
		}
		e.data.Space = previousSpace
		return fmt.Errorf("init dungeon: rebuild room: %w", err)
	}
	return nil
}

func validateAuthoredDoorIDsAgainstConnectors(connectors []DungeonConnectorParams, authored []AuthoredEdge) error {
	connectorIDs := make(map[core.EntityID]struct{}, len(connectors))
	for _, connector := range connectors {
		connectorIDs[connector.DoorID] = struct{}{}
	}
	for index, edge := range authored {
		if edge.Kind != GeneratedEdgeKindDoor {
			continue
		}
		if _, collides := connectorIDs[edge.DoorID]; collides {
			return fmt.Errorf("authored edge %d: stable door id %q collides with connector door", index, edge.DoorID)
		}
	}
	return nil
}

// validateDungeonDoorIDsAvailable rejects staged connector/authored-door IDs
// that would overwrite a pre-existing door. InitDungeon can then roll back a
// later rebuild failure by deleting only its own newly-created entries, never
// guessing whether a map key used to belong to an earlier dungeon.
func validateDungeonDoorIDsAvailable(
	existing map[core.EntityID]*DoorData,
	connectors []DungeonConnectorParams,
	authored []AuthoredEdge,
) error {
	for index, connector := range connectors {
		if _, exists := existing[connector.DoorID]; exists {
			return fmt.Errorf("connector %d door id %q already exists", index, connector.DoorID)
		}
	}
	for index, edge := range authored {
		if edge.Kind != GeneratedEdgeKindDoor {
			continue
		}
		if _, exists := existing[edge.DoorID]; exists {
			return fmt.Errorf("authored edge %d door id %q already exists", index, edge.DoorID)
		}
	}
	return nil
}

// validateDungeonParams checks the structural and scale invariants
// InitDungeon depends on: at least 2 regions, exactly len(Regions)-1
// connectors with non-empty and unique DoorIDs, every region with a
// non-empty and unique ID, a known Archetype, every region/height at
// least 4, and the boss-room scale invariant (rpg-toolkit#814 Approved
// Slice 3 corrections) — a generation-time assertion, not eyeballing.
func validateDungeonParams(params DungeonParams) error {
	source, err := floorSourceKind(params.FloorSource)
	if err != nil {
		return err
	}
	if source == FloorSourceCanvas {
		return validateCanvasDungeonParams(params)
	}
	if len(params.Regions) < 2 {
		return fmt.Errorf("dungeon needs at least 2 regions (got %d)", len(params.Regions))
	}
	if len(params.Connectors) != len(params.Regions)-1 {
		return fmt.Errorf("dungeon needs exactly %d connectors for %d regions (got %d)",
			len(params.Regions)-1, len(params.Regions), len(params.Connectors))
	}
	if params.Height < 4 {
		return fmt.Errorf("dungeon height must be at least 4 (got %d)", params.Height)
	}
	if params.PartyStart.SeatCount < 0 {
		return fmt.Errorf("party start seat count must not be negative (got %d)", params.PartyStart.SeatCount)
	}
	for i, r := range params.Regions {
		if r.Width < 4 {
			return fmt.Errorf("region %d (%q) width must be at least 4 (got %d)", i, r.ID, r.Width)
		}
	}
	// Region IDs must be non-empty and unique: SpaceData.RegionAt returns
	// the FIRST matching region for a given hex, so an empty or duplicate
	// ID either produces a meaningless tag or silently makes every hex in
	// a later region misreport as belonging to an earlier one.
	seenRegionIDs := make(map[string]int, len(params.Regions))
	for i, r := range params.Regions {
		if r.ID == "" {
			return fmt.Errorf("region %d: id required", i)
		}
		if first, dup := seenRegionIDs[r.ID]; dup {
			return fmt.Errorf("region %d (%q): duplicate region id (already used by region %d)", i, r.ID, first)
		}
		seenRegionIDs[r.ID] = i
	}
	// Archetype is documented as a fixed, reusable vocabulary (data.go) —
	// enforce that here rather than letting RegionArchetype's underlying
	// string type accept anything.
	for i, r := range params.Regions {
		switch r.Archetype {
		case ArchetypeEntrance, ArchetypeChamber, ArchetypeCorridor, ArchetypeBoss:
		default:
			return fmt.Errorf("region %d (%q): unknown archetype %q", i, r.ID, r.Archetype)
		}
	}
	seenDoorIDs := make(map[core.EntityID]int, len(params.Connectors))
	for i, c := range params.Connectors {
		if c.DoorID == "" {
			return fmt.Errorf("connector %d: door id required", i)
		}
		if first, dup := seenDoorIDs[c.DoorID]; dup {
			return fmt.Errorf("connector %d (%q): duplicate door id (already used by connector %d)", i, c.DoorID, first)
		}
		seenDoorIDs[c.DoorID] = i
	}
	// A locked connector's check config is validated contextually, before
	// InitDungeon mutates any encounter data: LockDC/LockAbility only mean
	// anything when Locked is true, and AttemptUnlock/SubmitCheck depend
	// on both being present (a zero DC is never a meaningful skill-check
	// target; an empty ability leaves the CharacterResolver nothing to
	// resolve a modifier against). Rejecting here — rather than letting a
	// half-configured locked door generate — mirrors the file's other
	// pre-mutation validation gates (duplicate IDs, unknown archetypes).
	for i, c := range params.Connectors {
		if !c.Locked {
			continue
		}
		if c.LockDC <= 0 {
			return fmt.Errorf("connector %d (%q): locked connector requires LockDC > 0 (got %d)", i, c.DoorID, c.LockDC)
		}
		if c.LockAbility == "" {
			return fmt.Errorf("connector %d (%q): locked connector requires LockAbility", i, c.DoorID)
		}
	}
	for i, r := range params.Regions {
		if r.Archetype != ArchetypeBoss {
			continue
		}
		axis := r.Width
		if params.Height < axis {
			axis = params.Height
		}
		if axis <= 6 {
			return fmt.Errorf(
				"region %d (%q): boss room primary playable axis must exceed 6 hex steps (got %d)",
				i, r.ID, axis)
		}
	}
	return nil
}

// dungeonLayout is the geometry result of generateDungeonLayout, entirely
// in tools/spatial's coordinate types. Kept private — InitDungeon is the
// only caller, and a host reads the result back off SpaceData/DoorData
// after the call, not off this. Mirrors two_chamber.go's (now retired)
// twoChamberLayout, generalized to N regions.
type dungeonLayout struct {
	walls           []environments.WallSegmentData
	width           int
	regions         []RegionData
	semanticRegions []SemanticRegionData
	entrance        spatial.CubeCoordinate
	// doors[i] is the door cube coordinate joining Regions[i] to
	// Regions[i+1] — parallel to params.Connectors.
	doors []spatial.CubeCoordinate
	// obstacles are every placed ObstacleSpec instance across every
	// region, in absolute coordinates — rpg-toolkit#819. See
	// placeRegionObstacles.
	obstacles []ObstacleData
	// partyStartPositions is the stored deterministic reservation exposed by
	// ResolvePartySpawnPositions. Index zero is always entrance.
	partyStartPositions []core.Hex
}

// generateDungeonLayout builds each region's independently wall-generated
// interior, placed side by side in local-column order, joined by exactly
// one doorway gap per connector in the shared boundary column between
// consecutive regions.
//
// Layout (offset coordinates): region i occupies columns
// [start[i], start[i]+Width[i]); the boundary column between region i and
// i+1 sits at start[i]+Width[i] (every row solid except doorRow, which
// carries no wall — that cell is connector i's door). The entrance sits
// at region 0's far edge (column 0, doorRow) — "just inside the entrance,
// not center" per the design doc's playtest script, same as
// InitTwoChamberRoom.
//
// Connectivity is guaranteed BY CONSTRUCTION: region 0 reserves a
// required path from the entrance to its outgoing door; each interior
// region (index 1..N-2) reserves a required path spanning its full local
// width, incoming-door-adjacent to outgoing-door-adjacent; the terminal
// region (index N-1) reserves a required path from its incoming-door-
// adjacent cell to its own local center (not the far edge — nothing lies
// beyond it) — generalizing InitTwoChamberRoom's two-chamber required
// paths to N regions.
func generateDungeonLayout(params DungeonParams) (*dungeonLayout, error) {
	if params.FloorSource == FloorSourceCanvas {
		return generateCanvasDungeonLayout(params)
	}
	n := len(params.Regions)
	doorRow := params.Height / 2

	seed := params.RandomSeed
	if seed == 0 {
		//nolint:gosec // G404: deterministic game generation, not cryptographic
		seed = rand.Int63()
	}
	// Every region derives an independent (but reproducible) sub-seed from
	// the one input seed, so a caller-supplied seed reproduces the WHOLE
	// layout without every region rolling identical wall patterns.
	//nolint:gosec // G404: deterministic derivation from the caller's seed
	sub := rand.New(rand.NewSource(seed))
	regionSeeds := make([]int64, n)
	for i := range regionSeeds {
		regionSeeds[i] = sub.Int63()
	}
	// A second, independent per-region sub-seed for obstacle placement
	// (rpg-toolkit#819) — drawn from the SAME `sub` stream right after
	// every region's wall seed, so adding/removing/reordering
	// ObstacleSpecs never perturbs wall generation (regionSeeds is
	// already fully drawn above) and a caller-supplied seed reproduces
	// the WHOLE layout, obstacles included.
	obstacleSeeds := make([]int64, n)
	for i := range obstacleSeeds {
		obstacleSeeds[i] = sub.Int63()
	}

	starts := make([]int, n)
	x := 0
	for i, r := range params.Regions {
		starts[i] = x
		x += r.Width + 1 // +1 reserves the boundary/door column after this region
	}
	totalWidth := x - 1 // no trailing boundary column after the last region

	// Resolve authored endpoints before seed-generated walls are emitted. Only
	// legacy wall-cell geometry is stripped; ordinary props and party/start
	// content may share an endpoint and independently block that cell.
	authoredEndpoints := authoredEndpointCubes(params.AuthoredEdges)

	// Resolve every party seat before generating a single wall or obstacle.
	// Its reservation is threaded through the wall safety paths and obstacle
	// candidate pools below.
	partyStart, err := resolvePartyStartReservation(params, starts, totalWidth, doorRow)
	if err != nil {
		return nil, err
	}

	var segs []environments.WallSegmentData
	regions := make([]RegionData, n)
	doors := make([]spatial.CubeCoordinate, n-1)
	var obstacles []ObstacleData
	// connectorColumnCubes is every connector's FULL boundary column (door
	// cell included); connectorFlankingCubes is the same columns minus
	// their door cells -- rpg-toolkit#848. Both are populated as the
	// connector loop below runs and consumed once, after every region's
	// walls are known, by the perimeterEdgeWalls/connectorBoundaryEdgeWalls
	// calls at the bottom of this function. See connectorBoundaryEdgeWalls'
	// doc for why these can no longer be plain degenerate (Start == End)
	// entries in segs itself.
	connectorColumnCubes := make(map[spatial.CubeCoordinate]struct{})
	connectorFlankingCubes := make(map[spatial.CubeCoordinate]struct{})

	for i, r := range params.Regions {
		local := spatial.Position{X: 0, Y: float64(doorRow)}
		requiredPaths := make([]environments.Path, 0, 1+len(partyStart.seats))
		switch {
		case n == 1:
			// unreachable: validateDungeonParams enforces n>=2
		case i == 0:
			// entrance region: entrance -> outgoing door.
			farEdge := spatial.Position{X: float64(r.Width - 1), Y: float64(doorRow)}
			requiredPaths = []environments.Path{
				{From: local, To: farEdge, Width: dungeonPathWidth, Purpose: "entrance-to-door"},
			}
		case i == n-1 && r.Archetype != ArchetypeBoss:
			// non-boss terminal region: incoming-door-adjacent -> local
			// center. Purpose names the destination generically
			// ("region", not "chamber") since the terminal region can be
			// any non-boss archetype (chamber, ...) — this string only
			// ever surfaces in environments.validatePathSafety's
			// "required path '%s' is blocked" error, but it must still
			// describe what's actually there.
			center := spatial.Position{X: float64(r.Width) / 2, Y: float64(doorRow)}
			requiredPaths = []environments.Path{
				{From: local, To: center, Width: dungeonPathWidth, Purpose: "door-to-region-center"},
			}
		default:
			// interior region OR a boss-archetype region regardless of
			// its position in the chain (including the terminal slot,
			// which is where every #814 fixture puts it): incoming-
			// door-adjacent -> local FAR edge, not just center.
			// rpg-toolkit#819's hard invariant is that a boss region's
			// full primary playable axis (this exact row, spanning the
			// region's whole width) stays clear — the same full-width
			// reservation entrance/interior regions already get for
			// connectivity, extended to the boss archetype specifically
			// because "connectivity" alone (door-to-center) is not the
			// same requirement as "the whole tactical axis stays open."
			// This is necessary but NOT sufficient on its own — see
			// stripReservedAxisWalls below for why a second, discrete-
			// level guarantee is also required.
			farEdge := spatial.Position{X: float64(r.Width - 1), Y: float64(doorRow)}
			purpose := "door-to-door"
			if r.Archetype == ArchetypeBoss {
				purpose = "boss-primary-axis"
			}
			requiredPaths = []environments.Path{
				{From: local, To: farEdge, Width: dungeonPathWidth, Purpose: purpose},
			}
		}

		requiredPaths = append(requiredPaths, partyStart.requiredPathsForRegion(i, starts[i])...)

		pattern := r.Pattern
		if pattern == "" {
			pattern = environments.PatternRandom
		}
		walls, err := generateRegionWalls(r.Width, params.Height, pattern, regionSeeds[i], requiredPaths)
		if err != nil {
			return nil, fmt.Errorf("generate region %d (%q) walls: %w", i, r.ID, err)
		}
		regionWalls := regionWallSegments(walls, starts[i], 0)
		regionWalls = partyStart.stripPartyStartWalls(i, regionWalls)
		regionWalls = stripAuthoredEndpointWalls(regionWalls, authoredEndpoints)
		if r.Archetype == ArchetypeBoss {
			// Construction-time discrete safeguard (rpg-toolkit#819),
			// applied BEFORE this call's result is committed to
			// encounter state — not a seed retry, not a post-commit
			// repair. The required-path extension above declares
			// intent to tools/environments' validator, but that
			// validator's linesIntersect (a separate module, verified
			// read-only, not modified here — tracked upstream as
			// rpg-toolkit#827) degenerates to "no intersection" for ANY
			// wall segment that shares the required path's orientation
			// (both horizontal, here) — its parallel-lines denominator
			// is identically zero whenever BOTH lines are horizontal,
			// regardless of either line's actual Y position, so it
			// cannot distinguish a genuinely parallel-elsewhere wall
			// from one collinear with and overlapping the path itself.
			// The required-path extension alone cannot catch this class
			// of wall; stripReservedAxisWalls is the actual guarantee.
			// It only ever REMOVES walls (never adds), so it can't
			// introduce a new safety violation; a full-dimension sweep
			// at this crypt's exact dimensions found zero unintended
			// notches from this removal (see boss_primary_axis_test.go).
			regionWalls = stripReservedAxisWalls(regionWalls, starts[i], r.Width, doorRow)
		}
		segs = append(segs, regionWalls...)
		regions[i] = RegionData{
			ID:        r.ID,
			Archetype: r.Archetype,
			Hexes:     core.NewHexSet(hexesFromCubes(regionCubes(r.Width, params.Height, starts[i]))...),
		}
		regionObstacles, err := placeRegionObstacles(placeRegionObstaclesParams{
			regionID:      r.ID,
			specs:         r.Obstacles,
			placed:        r.PlacedObstacles,
			reserved:      r.ReservedCells,
			partyReserved: partyStart.seatsByRegion[i],
			width:         r.Width,
			height:        params.Height,
			offsetX:       starts[i],
			doorRow:       doorRow,
			wallCubes:     wallCubeSet(regionWalls),
			seed:          obstacleSeeds[i],
		})
		if err != nil {
			return nil, fmt.Errorf("place region %d (%q) obstacles: %w", i, r.ID, err)
		}
		obstacles = append(obstacles, regionObstacles...)

		if i < n-1 {
			doorX := starts[i] + r.Width
			// rpg-toolkit#848: no longer appends a degenerate (Start == End)
			// entry per non-door cell here -- that made each flanking cell
			// an independently-classified wall hex client-side, and a hex
			// column's flanking cells expose 4 of their 6 sides to the two
			// adjacent regions (not 2, the way a square-grid column would),
			// rendering as an isolated "chunky rubble box" per cell instead
			// of a clean wall. Every column cell (door included) is instead
			// recorded here and converted into boundary-edge segments by
			// connectorBoundaryEdgeWalls below, once every region's own
			// walls are known.
			for y := 0; y < params.Height; y++ {
				cube := spatial.OffsetCoordinateToCubeWithOrientation(
					spatial.Position{X: float64(doorX), Y: float64(y)}, spatial.HexOrientationPointyTop)
				connectorColumnCubes[cube] = struct{}{}
				if y == doorRow {
					continue
				}
				connectorFlankingCubes[cube] = struct{}{}
			}
			doors[i] = spatial.OffsetCoordinateToCubeWithOrientation(
				spatial.Position{X: float64(doorX), Y: float64(doorRow)}, spatial.HexOrientationPointyTop)
		}
	}

	// blocked is every cell that is NOT real floor for boundary-edge
	// purposes: interior pattern-wall/obstacle cells (degenerate Start==End
	// entries already in segs) UNION every connector column cell, door
	// cells included (rpg-toolkit#848) -- a connector's own door cell must
	// never be treated as floor eligible to grow an outward-facing
	// perimeter edge of its own (TestPerimeterEdgeWalls_NeverAtConnectorDoorCells),
	// even though it carries no wall entry in segs itself.
	blocked := wallCubeSet(segs)
	for cube := range connectorColumnCubes {
		blocked[cube] = struct{}{}
	}

	// rpg-toolkit#834: one boundary-edge segment (Start != End) per
	// room-facing perimeter edge of the WHOLE combined space, appended
	// after every region's interior walls and every connector's boundary
	// column above -- see perimeterEdgeWalls' doc for why this never
	// touches interior wall emission or the connector columns, and is a
	// pure, seed-independent function of the floor/wall layout already
	// built above.
	segs = append(segs, perimeterEdgeWalls(blocked, totalWidth, params.Height)...)

	// rpg-toolkit#848: the same boundary-edge technique, extended to every
	// connector's interior boundary column -- see connectorBoundaryEdgeWalls'
	// doc for why the column's flanking (non-door) cells can no longer be
	// left as degenerate Start==End entries.
	segs = append(segs, connectorBoundaryEdgeWalls(blocked, connectorFlankingCubes, totalWidth, params.Height)...)

	return &dungeonLayout{
		walls:               segs,
		width:               totalWidth,
		regions:             regions,
		entrance:            partyStart.anchor,
		doors:               doors,
		obstacles:           obstacles,
		partyStartPositions: partyStart.positions(),
	}, nil
}

// perimeterEdgeWalls computes one boundary-edge WallSegmentData (Start !=
// End, exactly one hex step apart) per room-facing edge of the combined
// dungeon space's outer perimeter (rpg-toolkit#834) -- rooms shipped ZERO
// perimeter wall data before this; blocking came only from grid bounds,
// and the client's wall renderer had nothing to draw a room's outer walls
// with. For every FLOOR hex in [0,width) x [0,height) (i.e. not already in
// blocked -- interior pattern/obstacle walls and every connector column
// cell alike, rpg-toolkit#848) and every one of its 6 hex-grid neighbor
// directions that lands OUTSIDE those bounds, one segment {Start: the
// floor hex, End: the out-of-bounds neighbor} is emitted.
//
// blocked is caller-supplied (generateDungeonLayout unions wallCubeSet(segs)
// with every connector column cube, doors included) rather than derived
// here from a raw wall list, specifically so a connector column's flanking
// cells -- no longer degenerate Start==End entries in segs themselves,
// see connectorBoundaryEdgeWalls -- still count as "not floor" for THIS
// function's purposes. Without that, a flanking cell sitting at the
// space's own y=0/y=height-1 row (common: validateDungeonParams' >=4
// height floor guarantees a connector column always has flanking rows at
// both true edge rows, only doorRow is ever excluded) would be
// misidentified as real floor and grow a bogus outward-facing perimeter
// edge of its own.
//
// A connector's boundary column sits at an interior x (strictly between
// two regions, never the space's x=0 or x=width-1 edge) but spans every y
// INCLUDING the true edge rows y=0/y=height-1 -- by "floor hex" exclusion
// those cells never get a NEW edge segment here, keeping them exactly the
// "interior structure" they already were (this function only ever ADDS
// segments, never touches walls' existing entries). Doors sit at a
// boundary column's doorRow cell specifically, which is never on the
// space's own x=0/x=width-1/y=0/y=height-1 edges either
// (validateDungeonParams' >=4 height/width floor keeps doorRow strictly
// interior) -- so this function needs no separate door-passage exclusion;
// the geometry already keeps every connector passage off the outer
// perimeter (see
// perimeter_edge_walls_test.go's TestPerimeterEdgeWalls_NeverAtConnectorDoorCells
// for the assertion).
//
// A single floor hex can and does emit MORE than one segment (e.g. a true
// corner-of-the-map hex, whose skewed odd-q offset neighbor geometry can
// put several of its 6 sides outside the rectangle) -- "one segment per
// room-facing edge," not per hex, is exactly what rpg-dnd5e-web#566's
// client-side hexDistance==1 renderer wants: each exposed edge draws its
// own clean slab.
//
// Purely a function of blocked/width/height, already fully determined by
// the (possibly seeded) region generation above -- no RNG here, so
// perimeter emission is itself deterministic and seed-independent, unlike
// the per-region interior pattern it follows.
//
// These segments never become a spatial.Room blocker in their own right
// when End is genuinely outside the grid (see rebuildRoomFromData): Start
// is real walkable floor and End is entirely outside the grid, already
// unreachable by construction (every LOS/movement check already runs
// through spatial.HexGrid.IsValidPosition) -- this is the render contract
// (rpg-dnd5e-web#566) catching up to the room's actual shape, not a new
// blocking mechanism, so walkability outcomes are unchanged by this
// function's output. (connectorBoundaryEdgeWalls' segments differ here --
// their End IS a valid in-grid position, and rebuildRoomFromData places a
// blocker there specifically because of that.)
func perimeterEdgeWalls(blocked map[spatial.CubeCoordinate]struct{}, width, height int) []environments.WallSegmentData {
	var out []environments.WallSegmentData
	fw, fh := float64(width), float64(height)
	for x := 0; x < width; x++ {
		for y := 0; y < height; y++ {
			cube := spatial.OffsetCoordinateToCubeWithOrientation(
				spatial.Position{X: float64(x), Y: float64(y)}, spatial.HexOrientationPointyTop)
			if _, isWall := blocked[cube]; isWall {
				continue
			}
			for _, n := range cube.GetNeighbors() {
				npos := n.ToOffsetCoordinateWithOrientation(spatial.HexOrientationPointyTop)
				if npos.X >= 0 && npos.X < fw && npos.Y >= 0 && npos.Y < fh {
					continue // neighbor is inside the space -- not a perimeter edge
				}
				out = append(out, environments.WallSegmentData{
					Start: cube, End: n, BlocksMovement: true, BlocksLoS: true,
				})
			}
		}
	}
	return out
}

// connectorBoundaryEdgeWalls computes one boundary-edge WallSegmentData
// (Start != End, exactly one hex step apart) per room-facing edge where a
// REAL floor hex sits adjacent to a connector column's flanking (non-door)
// cell -- extending #834's boundary-edge technique from the space's outer
// perimeter (perimeterEdgeWalls, above) to these INTERIOR boundary columns
// (rpg-toolkit#848).
//
// Root cause this replaces: a connector column's flanking cells used to be
// individual degenerate (Start == End) WallSegmentData entries. Client-side
// (rpg-dnd5e-web's buildDungeonWallSegments/collectWallHexes), any such
// entry is classified as an independent wall hex and rendered by drawing
// one edge piece per exposed side. Unlike a square grid, a straight
// single-file line of hex cells does NOT collapse to "2 exposed sides": a
// hex has 6 neighbors, and a north/south column only ever shares 2 of them
// with its own same-column neighbors -- the other 4 face diagonally into
// the two ADJACENT REGIONS (at rows skewed by the odd-q offset scheme,
// tools/spatial's OffsetCoordinateToCubeWithOrientation). So even a
// flanking cell in the middle of a long run exposes 4 sides, not 2,
// rendering as an isolated "chunky rubble box" that visually drowns the
// (correctly rendered) door immediately beside it.
//
// The fix mirrors perimeterEdgeWalls exactly, just facing inward instead
// of outward: iterate every REAL floor hex in [0,width) x [0,height) (not
// in blocked, which the caller has already unioned with every connector
// column cell -- door cells included, so a door is never treated as a
// Start here either, matching TestPerimeterEdgeWalls_NeverAtConnectorDoorCells'
// invariant) and every one of its 6 neighbor directions; whenever that
// neighbor is one of THIS dungeon's connector flanking cells, emit one
// {Start: the floor hex, End: the flanking cell} segment. A flanking cell
// has up to 4 "region-facing" neighbor candidates (2 toward each adjacent
// region, matching the "4 exposed sides" diagnosis above) -- one on the
// space's own true edge row (y=0 or y=height-1) has 2 of those candidates
// land outside the whole grid rather than in a region, so it receives
// only 2 edges; every other flanking cell receives all 4. Either way,
// both of a flanking cell's neighboring regions still contribute at
// least one edge, which is what "the corridor's side walls" (the issue's
// phrasing) actually looks like once rendered: clean edges on both
// sides, not a floating block.
//
// Unlike perimeterEdgeWalls' segments, a flanking cell IS a valid in-grid
// position (an interior column, not the space's true edge) -- these
// segments DO need to keep blocking movement/LOS even though they're no
// longer a degenerate entry in segs. See rebuildRoomFromData's companion
// change: it places a blocker at the End of any Start != End segment that
// is itself a valid in-grid position, leaving the true out-of-grid
// perimeter case (#834) an unaffected no-op.
//
// rpg-toolkit#849 gate review finding 1: without the fallback below, a
// flanking cell's blocking would be EMERGENT rather than structural --
// it's only blocked if at least one of its region-facing neighbors is
// itself real, unblocked floor that emits an edge to it. If every one of
// a flanking cell's region-facing neighbors happened to already be an
// interior obstacle-blocked cell, the main loop would emit no edge to it
// at all, and rebuildRoomFromData would place nothing there -- silently
// making that cell both walkable and see-through, a real regression
// versus the old unconditional degenerate block. Never observed across
// hundreds of seeds of the 3-region fixture (interior density has never
// been high enough to surround a flanking cell on every side), but the
// guarantee shouldn't rest on interior density: any flanking cell the
// main loop never covers falls back to the pre-fix degenerate
// (Start == End) shape, so it renders as the old "rubble box" only in
// this fringe case -- never in the common case the main loop already
// covers.
func connectorBoundaryEdgeWalls(
	blocked map[spatial.CubeCoordinate]struct{},
	flanking map[spatial.CubeCoordinate]struct{},
	width, height int,
) []environments.WallSegmentData {
	// Capacity hint only, not exact: each flanking cell has at most 4
	// region-facing edges (see doc above), plus at most one fallback
	// entry apiece.
	out := make([]environments.WallSegmentData, 0, len(flanking)*4)
	covered := make(map[spatial.CubeCoordinate]struct{}, len(flanking))
	for x := 0; x < width; x++ {
		for y := 0; y < height; y++ {
			cube := spatial.OffsetCoordinateToCubeWithOrientation(
				spatial.Position{X: float64(x), Y: float64(y)}, spatial.HexOrientationPointyTop)
			if _, isBlocked := blocked[cube]; isBlocked {
				continue
			}
			for _, n := range cube.GetNeighbors() {
				if _, isFlanking := flanking[n]; !isFlanking {
					continue
				}
				covered[n] = struct{}{}
				out = append(out, environments.WallSegmentData{
					Start: cube, End: n, BlocksMovement: true, BlocksLoS: true,
				})
			}
		}
	}
	for cube := range flanking {
		if _, ok := covered[cube]; ok {
			continue
		}
		out = append(out, environments.WallSegmentData{
			Start: cube, End: cube, BlocksMovement: true, BlocksLoS: true,
		})
	}
	return out
}

// generateRegionWalls generates one region's interior wall pattern,
// reusing the same pattern registry (environments.WallPatterns) and
// default rectangle shape InitRoom's QuickRoom uses internally. Called
// directly against the pattern function — bypassing
// environments.BasicRoomBuilder — because BasicRoomBuilder.generateWalls
// unconditionally overwrites PatternParams.Safety.RequiredPaths with
// paths derived from the shape's own connections, which would silently
// drop the connectivity guarantee this generator depends on. Mirrors
// two_chamber.go's (now retired) generateChamberWalls.
func generateRegionWalls(
	width, height int, pattern string, seed int64, requiredPaths []environments.Path,
) ([]environments.WallSegment, error) {
	patternFunc, ok := environments.WallPatterns[pattern]
	if !ok {
		return nil, fmt.Errorf("unknown wall pattern %q", pattern)
	}
	size := spatial.Dimensions{Width: float64(width), Height: float64(height)}
	shape := environments.ScaleShape(environments.GetDefaultShapes()[environments.ShapeRectangle], size)
	params := environments.PatternParams{
		Density:           0.4,
		DestructibleRatio: 0.7,
		RandomSeed:        seed,
		Material:          "stone",
		WallHeight:        3.0,
		Safety: environments.PathSafetyParams{
			MinPathWidth:      dungeonPathWidth,
			MinOpenSpace:      0.6,
			EntitySize:        1.0,
			RequiredPaths:     requiredPaths,
			EmergencyFallback: true,
		},
	}
	return patternFunc(context.Background(), shape, size, params)
}

// regionWallSegments discretizes one region's raw (continuous-position)
// wall segments into per-hex WallSegmentData in the COMBINED space's
// absolute coordinates, translating by offsetX/offsetY before rounding.
// Mirrors this package's own snapshotWalls (space.go) — same rounding and
// cube-coordinate dedup, because generated walls don't naturally align to
// hex cells (see InitRoom's doc) — but operates directly on raw
// WallSegments via environments.CreateWallEntities rather than a
// spatial.Room, since there's no single-region Room to snapshot here (all
// regions only ever exist merged, in the encounter's one e.room). Mirrors
// two_chamber.go's (now retired) chamberWallSegments.
func regionWallSegments(walls []environments.WallSegment, offsetX, offsetY int) []environments.WallSegmentData {
	entities := environments.CreateWallEntities(walls)
	out := make([]environments.WallSegmentData, 0, len(entities))
	seen := make(map[spatial.CubeCoordinate]int, len(entities))
	for _, ent := range entities {
		we, ok := ent.(*environments.WallEntity)
		if !ok {
			continue
		}
		pos := we.GetPosition()
		rounded := spatial.Position{
			X: math.Round(pos.X) + float64(offsetX),
			Y: math.Round(pos.Y) + float64(offsetY),
		}
		cube := spatial.OffsetCoordinateToCubeWithOrientation(rounded, spatial.HexOrientationPointyTop)
		if i, dup := seen[cube]; dup {
			out[i].BlocksMovement = out[i].BlocksMovement || we.BlocksMovement()
			out[i].BlocksLoS = out[i].BlocksLoS || we.BlocksLineOfSight()
			continue
		}
		seen[cube] = len(out)
		out = append(out, environments.WallSegmentData{
			Start: cube, End: cube,
			BlocksMovement: we.BlocksMovement(),
			BlocksLoS:      we.BlocksLineOfSight(),
		})
	}
	return out
}

// authoredEndpointCubes returns every semantic endpoint of the already
// validated authored-edge collection. The set only protects edge-native
// geometry from legacy wall-cell emission; it does not reserve endpoints from
// ordinary props, actors, starts, or spawns.
func authoredEndpointCubes(edges []AuthoredEdge) map[spatial.CubeCoordinate]struct{} {
	if len(edges) == 0 {
		return nil
	}
	endpoints := make(map[spatial.CubeCoordinate]struct{}, len(edges)*2)
	for _, edge := range edges {
		endpoints[edge.From.ToCube()] = struct{}{}
		endpoints[edge.To.ToCube()] = struct{}{}
	}
	return endpoints
}

// stripAuthoredEndpointWalls removes seed-generated legacy wall cells at
// authored endpoints. It changes neither selected party seats nor ordinary
// prop-placement coordinates.
func stripAuthoredEndpointWalls(
	walls []environments.WallSegmentData, endpoints map[spatial.CubeCoordinate]struct{},
) []environments.WallSegmentData {
	if len(endpoints) == 0 {
		return walls
	}
	out := make([]environments.WallSegmentData, 0, len(walls))
	for _, wall := range walls {
		if _, reserved := endpoints[wall.Start]; reserved {
			continue
		}
		out = append(out, wall)
	}
	return out
}

// wallCubeSet builds a lookup set of every absolute cube coordinate a
// region's wall segments occupy — used by placeRegionObstacles to reject
// wall cells as obstacle candidates. walls is already in absolute
// coordinates (the caller's regionWallSegments output); only Start is
// read, which is correct as long as every entry passed in is degenerate
// (Start == End) — true of both current call sites (regionWalls at :590,
// always degenerate; segs at :631, called BEFORE perimeterEdgeWalls/
// connectorBoundaryEdgeWalls append any boundary-edge entries to it) but
// no longer a property of WallSegmentData in general since rpg-toolkit#834/
// #848 (rpg-toolkit#849 gate review finding 7) — calling this on a wall
// list that already contains boundary-edge entries would silently fold
// their real-floor Start cubes into the result as if they were blocked.
func wallCubeSet(walls []environments.WallSegmentData) map[spatial.CubeCoordinate]struct{} {
	set := make(map[spatial.CubeCoordinate]struct{}, len(walls))
	for _, w := range walls {
		set[w.Start] = struct{}{}
	}
	return set
}

// stripReservedAxisWalls removes any wall segment (already in absolute
// coordinates — the caller's regionWallSegments output) landing on
// doorRow within [offsetX, offsetX+width) — a boss-archetype region's
// primary playable axis, rpg-toolkit#819's hard invariant. This is a
// construction-time discrete safeguard applied as part of building this
// region's layout, BEFORE the result is committed to encounter state —
// not a post-hoc patch to already-committed data, and not a seed retry.
// It exists because tools/environments' upstream RequiredPaths
// declaration alone (see the call site) cannot catch a wall segment
// that shares the required path's own orientation — see rpg-toolkit#827
// (upstream, tools/environments ownership) for the root cause. Only
// ever removes walls — widening the open floor can never violate a
// safety guarantee the pattern generator already established, so no
// re-validation is needed; a full 1..3000-seed sweep at this crypt's
// exact dimensions found zero unintended notches from this removal
// (boss_primary_axis_test.go).
func stripReservedAxisWalls(
	walls []environments.WallSegmentData, offsetX, width, doorRow int,
) []environments.WallSegmentData {
	out := make([]environments.WallSegmentData, 0, len(walls))
	for _, w := range walls {
		pos := w.Start.ToOffsetCoordinateWithOrientation(spatial.HexOrientationPointyTop)
		localX := int(pos.X) - offsetX
		if int(pos.Y) == doorRow && localX >= 0 && localX < width {
			continue
		}
		out = append(out, w)
	}
	return out
}

// placeRegionObstaclesParams bundles placeRegionObstacles' inputs — one
// region's geometry plus its caller-supplied specs — so the function
// signature doesn't grow an eighth positional argument as #819 evolves.
type placeRegionObstaclesParams struct {
	regionID      string
	specs         []ObstacleSpec
	placed        []PlacedObstacleSpec
	reserved      []LocalHex
	partyReserved map[spatial.CubeCoordinate]struct{}
	width         int
	height        int
	offsetX       int
	doorRow       int
	wallCubes     map[spatial.CubeCoordinate]struct{}
	seed          int64
}

// placeRegionObstacles computes the ObstacleData instances for every
// ObstacleSpec in one region — rpg-toolkit#819. Purely geometric and
// content-agnostic: it never reads Ref/BlocksMovement/BlocksLoS, only
// copies them verbatim.
//
// Candidates are every LOCAL floor cell (x in [0,width), y in [0,height))
// that is NOT a wall cell AND NOT on doorRow. Excluding the whole doorRow
// row — not just the narrower required-path segment
// generateDungeonLayout reserves for connectivity — is deliberately the
// SAME reservation for every region regardless of archetype: it is a
// superset of the required path (entrance-to-door / door-to-door /
// door-to-region-center all run along this exact row), and for a boss-
// archetype region it ALSO satisfies #819's additional "primary playable
// axis (>6 hex steps) must stay clear" invariant, because the row spans
// the region's FULL width — always at least the validateDungeonParams
// size floor already enforces at construction time. One rule, no
// archetype-specific branching, provably sufficient for both invariants.
//
// The candidate pool's draw order depends on whether ANY spec in this
// region sets PreferBorder (rpg-toolkit#839/#840):
//
//   - No spec prefers the border (every pre-#839 caller/fixture): ONE
//     flat candidate list in natural (x,y) scan order, ONE seeded
//     Fisher-Yates shuffle — the exact, byte-identical code path this
//     function used before #839 ever existed. Zero behavior change for
//     any caller that never sets PreferBorder.
//   - At least one spec prefers the border: the pool is PARTITIONED into
//     border cells (local x==0, x==width-1, y==0, or y==height-1 —
//     hugging the region's own four walls/corners) and interior cells,
//     each independently shuffled. PreferBorder specs draw border-first
//     (falling back to interior on overflow) in TWO PASSES: every
//     PreferBorder=true spec draws before any PreferBorder=false spec,
//     regardless of each spec's position in p.specs — rpg-toolkit#840's
//     gate finding was that applying the border bias to ALL specs in
//     list order forced focal pieces (always listed first: obelisk;
//     coffin→altar→statues) onto the border ahead of the dressing that
//     actually wanted it, inverting the composition target instead of
//     achieving it. Once every border-preferring spec has drawn, the
//     UNCONSUMED remainder (border leftovers + all interior, mixed) is
//     reshuffled once more so PreferBorder=false specs still draw
//     uniformly over what's left, not a residual border-first order.
//
// Either way, specs draw from their pool in order, each instance
// consuming one candidate so no two specs (or two instances of the same
// spec) can ever collide — a spec whose Count exceeds its pool's
// capacity naturally places as many as fit and drops the rest — #819's
// "skip rather than invalidate the dungeon" requirement — the ROLLED path
// never errors regardless of which draw-order branch runs.
//
// p.placed (PlacedObstacleSpec, design.md §Design delta) is handled FIRST,
// verbatim — see placeVerbatimObstacles — and is NOT best-effort: any
// violation (reserved row, collision with another placed entry, or a wall
// cell) fails this call outright. Placed cells are then excluded from the
// rolled candidate pool (both draw-order branches below), so the two
// mechanisms never collide with each other; placed obstacle IDs are
// numbered first, and rolled instances continue that same region's ID
// sequence via idOffset, so a region with zero PlacedObstacles produces
// byte-identical rolled output to before this field existed.
//
// p.reserved (DungeonRegionParams.ReservedCells) is validated the same way
// as p.placed (see reserveCubes) and excluded from the SAME rolled
// candidate pool, but never emits any ObstacleData of its own — it exists
// purely to keep the rolled draw off a cell something else (a compiled
// place: monster, or a pinned boss.at) already occupies.
//
// p.partyReserved is deliberately separate from p.reserved: it is the
// dungeon-wide party-start envelope selected before wall generation, and its
// anchor may legally be on doorRow where ordinary ReservedCells are invalid.
// Every rolled obstacle skips those cells in both draw-order paths.
//
// Authored-edge endpoints are deliberately absent from this placement policy:
// an ordinary prop may share one and independently block its cell.
func placeRegionObstacles(p placeRegionObstaclesParams) ([]ObstacleData, error) {
	placedData, placedCubes, err := placeVerbatimObstacles(p)
	if err != nil {
		return nil, err
	}
	idOffset := len(placedData)

	reservedCubes, err := reserveCubes(p, placedCubes)
	if err != nil {
		return nil, err
	}
	// excluded is the union of placedCubes and reservedCubes: every cube
	// EITHER rolled draw-order branch below must skip. Built even when
	// p.specs is empty (the early-return just below), since reserveCubes'
	// validation above must run regardless of whether there's anything to
	// roll -- a caller can set ReservedCells with no Obstacles at all.
	excluded := make(map[spatial.CubeCoordinate]string, len(placedCubes)+len(reservedCubes)+len(p.partyReserved))
	for cube, ref := range placedCubes {
		excluded[cube] = ref
	}
	for cube := range reservedCubes {
		if _, already := excluded[cube]; !already {
			excluded[cube] = "" // reserved, not placed -- no obstacle name to report
		}
	}
	for cube := range p.partyReserved {
		if _, already := excluded[cube]; !already {
			excluded[cube] = "" // party-start reservation, no obstacle data
		}
	}

	if len(p.specs) == 0 {
		return placedData, nil
	}

	anyPrefersBorder := false
	for _, spec := range p.specs {
		if spec.PreferBorder {
			anyPrefersBorder = true
			break
		}
	}

	//nolint:gosec // G404: deterministic per-region obstacle seed, not cryptographic
	rng := rand.New(rand.NewSource(p.seed))

	if !anyPrefersBorder {
		candidates := regionObstacleCandidates(p, excluded)
		rng.Shuffle(len(candidates), func(i, j int) {
			candidates[i], candidates[j] = candidates[j], candidates[i]
		})
		return append(placedData, drawObstaclesFrom(p.regionID, p.specs, candidates, idOffset)...), nil
	}

	var border, interior []spatial.CubeCoordinate
	for x := 0; x < p.width; x++ {
		for y := 0; y < p.height; y++ {
			if y == p.doorRow {
				continue // reserved: required path / primary combat axis
			}
			cube := spatial.OffsetCoordinateToCubeWithOrientation(
				spatial.Position{X: float64(x + p.offsetX), Y: float64(y)}, spatial.HexOrientationPointyTop)
			if _, blocked := p.wallCubes[cube]; blocked {
				continue
			}
			if _, taken := excluded[cube]; taken {
				continue
			}
			if x == 0 || x == p.width-1 || y == 0 || y == p.height-1 {
				border = append(border, cube)
			} else {
				interior = append(interior, cube)
			}
		}
	}
	rng.Shuffle(len(border), func(i, j int) {
		border[i], border[j] = border[j], border[i]
	})
	rng.Shuffle(len(interior), func(i, j int) {
		interior[i], interior[j] = interior[j], interior[i]
	})
	preferPool := make([]spatial.CubeCoordinate, 0, len(border)+len(interior))
	preferPool = append(preferPool, border...)
	preferPool = append(preferPool, interior...)

	var preferSpecs, normalSpecs []ObstacleSpec
	for _, spec := range p.specs {
		if spec.PreferBorder {
			preferSpecs = append(preferSpecs, spec)
		} else {
			normalSpecs = append(normalSpecs, spec)
		}
	}

	out := drawObstaclesFrom(p.regionID, preferSpecs, preferPool, idOffset)

	consumed := 0
	for _, spec := range preferSpecs {
		consumed += spec.Count
	}
	if consumed > len(preferPool) {
		consumed = len(preferPool)
	}
	remainder := append([]spatial.CubeCoordinate(nil), preferPool[consumed:]...)
	rng.Shuffle(len(remainder), func(i, j int) {
		remainder[i], remainder[j] = remainder[j], remainder[i]
	})

	out = append(out, drawObstaclesFrom(p.regionID, normalSpecs, remainder, idOffset+len(out))...)
	return append(placedData, out...), nil
}

// placeVerbatimObstacles validates and positions one region's
// PlacedObstacleSpecs (design.md §Design delta) — verbatim placement, not
// a candidate-pool roll. Unlike rolled ObstacleSpecs (best-effort: skip
// whatever doesn't fit), a placed entry is a hard guarantee: any violation
// fails region generation outright rather than silently dropping the
// entry, mirroring InitDungeon's "either the whole dungeon commits, or a
// failure leaves the encounter exactly as it was before the call"
// contract. Returns the placed ObstacleData — IDs 0..len(p.placed)-1 in
// p.regionID's numbering, so rolled instances continue from len(placed)
// via idOffset — and the set of absolute cube coordinates consumed, so
// the rolled candidate pool can exclude them too.
func placeVerbatimObstacles(p placeRegionObstaclesParams) ([]ObstacleData, map[spatial.CubeCoordinate]string, error) {
	// Keyed by the Ref of whichever PlacedObstacleSpec claimed that cell
	// first, so a collision error can name BOTH obstacles involved, not
	// just the later one.
	placedCubes := make(map[spatial.CubeCoordinate]string, len(p.placed))
	out := make([]ObstacleData, 0, len(p.placed))
	for i, spec := range p.placed {
		// Bounds first: an out-of-bounds cell can still coincidentally
		// pass the doorRow/collision/wall checks below (or land in an
		// adjacent region's own span, or the boundary/door column), which
		// would report the wrong problem — same defense-in-depth
		// rationale as the doorRow check below (InitDungeon is a public
		// entry point; a caller other than dungeonspec's own load-time
		// Validate could reach an out-of-bounds At directly).
		if spec.At.Col < 0 || spec.At.Col >= p.width || spec.At.Row < 0 || spec.At.Row >= p.height {
			return nil, nil, fmt.Errorf("placed obstacle %q at %v is out of bounds (width=%d, height=%d)",
				spec.Ref, spec.At, p.width, p.height)
		}
		if spec.At.Row == p.doorRow {
			return nil, nil, fmt.Errorf("placed obstacle %q at %v is on the reserved row (doorRow=%d)",
				spec.Ref, spec.At, p.doorRow)
		}
		// Same conversion idiom as the sibling loops below (regionObstacleCandidates,
		// the border/interior partition): OffsetCoordinateToCubeWithOrientation
		// directly, rather than a HexFromPosition -> ToCube round trip.
		cube := spatial.OffsetCoordinateToCubeWithOrientation(
			spatial.Position{X: float64(p.offsetX + spec.At.Col), Y: float64(spec.At.Row)}, spatial.HexOrientationPointyTop)
		if prevRef, dup := placedCubes[cube]; dup {
			return nil, nil, fmt.Errorf("placed obstacle %q at %v collides with placed obstacle %q",
				spec.Ref, spec.At, prevRef)
		}
		if _, wall := p.wallCubes[cube]; wall {
			return nil, nil, fmt.Errorf("placed obstacle %q at %v is on a wall cell", spec.Ref, spec.At)
		}
		placedCubes[cube] = spec.Ref
		out = append(out, ObstacleData{
			ID:             core.EntityID(fmt.Sprintf("obstacle-%s-%d", p.regionID, i)),
			Ref:            spec.Ref,
			Position:       core.HexFromCube(cube),
			BlocksMovement: spec.BlocksMovement,
			BlocksLoS:      spec.BlocksLoS,
			Facing:         spec.Facing,
		})
	}
	return out, placedCubes, nil
}

// reserveCubes validates and positions one region's ReservedCells
// (DungeonRegionParams.ReservedCells) — cells excluded from the rolled-
// obstacle draw without any ObstacleData ever emitted for them, because
// something OUTSIDE this obstacle mechanism already occupies the cell (a
// compiled place: monster or a pinned boss.at — dungeonspec's own use of
// this field, compile.go). Validated the same way as
// placeVerbatimObstacles' PlacedObstacleSpecs — bounds, doorRow, wall
// cell, and collision with an already-placed obstacle — for the same
// defense-in-depth reason: InitDungeon is a public entry point a caller
// other than dungeonspec's load-time Validate could reach directly with a
// bad cell. Two ReservedCells landing on the SAME cell is not itself
// rejected (harmless — both just mean "this cell is spoken for"), unlike
// two PlacedObstacleSpecs at the same cell (which would silently drop one
// obstacle's data if not rejected).
//
// This function exists to close a real cross-task seam bug (rpg-toolkit#842
// gate finding): compileRoom routes a place: monster (or the boss's pinned
// boss.at) to a SpawnInstruction, never to PlacedObstacles — so without
// this reservation, the rolled-obstacle draw below has no idea that cell
// is spoken for, and can roll a count-based obstacle directly onto it. The
// collision then surfaces much later and far more confusingly, as
// Encounter.SeedMonsters failing to place the monster into a cell an
// obstacle already occupies — on some fraction of seeds, not all,
// depending on whether the shuffle happens to draw that cell.
func reserveCubes(
	p placeRegionObstaclesParams, placedCubes map[spatial.CubeCoordinate]string,
) (map[spatial.CubeCoordinate]struct{}, error) {
	reserved := make(map[spatial.CubeCoordinate]struct{}, len(p.reserved))
	for _, cell := range p.reserved {
		if cell.Col < 0 || cell.Col >= p.width || cell.Row < 0 || cell.Row >= p.height {
			return nil, fmt.Errorf("reserved cell %v is out of bounds (width=%d, height=%d)", cell, p.width, p.height)
		}
		if cell.Row == p.doorRow {
			return nil, fmt.Errorf("reserved cell %v is on the reserved row (doorRow=%d)", cell, p.doorRow)
		}
		cube := spatial.OffsetCoordinateToCubeWithOrientation(
			spatial.Position{X: float64(p.offsetX + cell.Col), Y: float64(cell.Row)}, spatial.HexOrientationPointyTop)
		if _, wall := p.wallCubes[cube]; wall {
			return nil, fmt.Errorf("reserved cell %v is on a wall cell", cell)
		}
		if prevRef, dup := placedCubes[cube]; dup {
			return nil, fmt.Errorf("reserved cell %v collides with placed obstacle %q", cell, prevRef)
		}
		reserved[cube] = struct{}{}
	}
	return reserved, nil
}

// regionObstacleCandidates enumerates every LOCAL floor cell (x in
// [0,width), y in [0,height)) that is NOT a wall cell, NOT on doorRow, and
// NOT already excluded (placed obstacle OR reserved cell, absolute
// coordinates) — the same reservation placeRegionObstacles' doc explains
// (a superset of every required path, and sufficient for the boss
// primary-axis invariant too), in natural (x,y) scan order, unshuffled.
// Extracted so the no-PreferBorder path (placeRegionObstacles) can build
// the exact same candidate list the pre-#839 mechanism always built, now
// also excluding placed and reserved cells.
func regionObstacleCandidates(
	p placeRegionObstaclesParams, excluded map[spatial.CubeCoordinate]string,
) []spatial.CubeCoordinate {
	candidates := make([]spatial.CubeCoordinate, 0, p.width*p.height)
	for x := 0; x < p.width; x++ {
		for y := 0; y < p.height; y++ {
			if y == p.doorRow {
				continue
			}
			cube := spatial.OffsetCoordinateToCubeWithOrientation(
				spatial.Position{X: float64(x + p.offsetX), Y: float64(y)}, spatial.HexOrientationPointyTop)
			if _, blocked := p.wallCubes[cube]; blocked {
				continue
			}
			if _, taken := excluded[cube]; taken {
				continue
			}
			candidates = append(candidates, cube)
		}
	}
	return candidates
}

// drawObstaclesFrom places specs against an already-shuffled candidates
// pool, each instance consuming one candidate in order so no two specs
// (or two instances of the same spec) can collide. idOffset lets a
// second pass over a second pool continue this region's ID numbering
// (obstacle-<regionID>-<n>) rather than restarting at 0 and colliding
// with the first pass's IDs.
func drawObstaclesFrom(
	regionID string, specs []ObstacleSpec, candidates []spatial.CubeCoordinate, idOffset int,
) []ObstacleData {
	var out []ObstacleData
	next := 0
	for _, spec := range specs {
		for n := 0; n < spec.Count && next < len(candidates); n++ {
			out = append(out, ObstacleData{
				ID:             core.EntityID(fmt.Sprintf("obstacle-%s-%d", regionID, idOffset+len(out))),
				Ref:            spec.Ref,
				Position:       core.HexFromCube(candidates[next]),
				BlocksMovement: spec.BlocksMovement,
				BlocksLoS:      spec.BlocksLoS,
			})
			next++
		}
	}
	return out
}

// regionCubes enumerates every hex in a width x height region (offset
// coordinates x in [0,width), y in [0,height), translated by offsetX) as
// absolute cube coordinates — a region tag's full membership, wall and
// floor cells alike (callers needing walkable-only cells filter against
// Walls). Mirrors two_chamber.go's (now retired) chamberCubes.
func regionCubes(width, height, offsetX int) []spatial.CubeCoordinate {
	out := make([]spatial.CubeCoordinate, 0, width*height)
	for x := 0; x < width; x++ {
		for y := 0; y < height; y++ {
			out = append(out, spatial.OffsetCoordinateToCubeWithOrientation(
				spatial.Position{X: float64(x + offsetX), Y: float64(y)}, spatial.HexOrientationPointyTop))
		}
	}
	return out
}

// hexesFromCubes converts a slice of spatial.CubeCoordinate to encounter
// Hexes — the boundary conversion between tools/spatial's coordinate math
// and this package's Hex type (mirrors core.HexFromCube for a single
// value).
func hexesFromCubes(cubes []spatial.CubeCoordinate) []core.Hex {
	out := make([]core.Hex, len(cubes))
	for i, c := range cubes {
		out[i] = core.HexFromCube(c)
	}
	return out
}
