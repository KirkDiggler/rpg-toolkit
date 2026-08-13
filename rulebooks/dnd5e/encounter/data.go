// Copyright (C) 2026 Kirk Diggler
// SPDX-License-Identifier: GPL-3.0-or-later

package encounter

import (
	"fmt"
	"slices"
	"sort"

	"github.com/KirkDiggler/rpg-toolkit/play/clock"
	"github.com/KirkDiggler/rpg-toolkit/play/intel"
	"github.com/KirkDiggler/rpg-toolkit/play/record"
	"github.com/KirkDiggler/rpg-toolkit/tools/spatial"
)

// EncounterData is the persistent representation of an Encounter.
// Outcome is omitted when the encounter is open; present when closed.
// All leaves (Clock, Intel, Log) embed their Data types verbatim.
// Deciders are NOT persisted; they are re-registered at load.
type EncounterData struct {
	Outcome     *OutcomeData   `json:"outcome,omitempty"`
	Clock       clock.TickData `json:"clock"`
	Intel       intel.Data     `json:"intel"`
	Log         record.LogData `json:"log"`
	Field       FieldData      `json:"field"`
	Members     []MemberData   `json:"members"`
	Endings     []EndingData   `json:"endings"`
	EverMembers []MemberID     `json:"ever_members"`
	// Retention is the story-beat window this encounter was built with (see
	// SetupInput.Retention). Persisted so a reloaded encounter keeps the policy
	// it was constructed with rather than silently adopting the package default.
	// Absent in blobs written before #937, which load as zero and therefore take
	// DefaultRetention.
	Retention int `json:"retention,omitempty"`
}

// OutcomeData is the persistent representation of an Outcome.
type OutcomeData struct {
	Ending  string              `json:"ending"`
	At      uint64              `json:"at,omitempty"`
	Members []MemberOutcomeData `json:"members,omitempty"`
}

// MemberOutcomeData is a member's position when the encounter closed.
type MemberOutcomeData struct {
	ID       MemberID     `json:"id"`
	Room     string       `json:"room"`
	Position PositionData `json:"position"`
}

// FieldData is the persistent representation of the encounter's field.
// Rooms hold the composition's own descriptions (mirroring RoomInput exactly).
// Connections are the declared room links.
type FieldData struct {
	Rooms       []RoomData       `json:"rooms"`
	Connections []ConnectionData `json:"connections,omitempty"`
}

// RoomData mirrors RoomInput exactly to persist construction inputs — true
// again as of #929 T2 (a T1-era comment here briefly said otherwise, while
// Origin was Setup-only). Origin is REQUIRED at load (W5): a nil pointer
// means the field was absent from the blob (distinct from a declared
// zero) and is rejected with ErrInvalidData naming the room — a missing
// anchor must never silently default to (0,0), a legal position that
// would invent placement, mirroring ConnectionData's FromPosition/ToPosition
// precedent. ToData ALWAYS writes Origin, even the zero value (no
// omitempty) — a declared (0,0) persists as an explicit "origin":{"x":0,"y":0},
// never as absence, so presence itself is meaningful at load.
// Grid is persisted as spatial's own GridType string ("hex" — tools/spatial's
// GridTypeHex; "gridless" is a v0.2-era value no longer accepted at load,
// #929 T2), not the GridShape iota: the iota is an in-process enumeration
// order, not a wire contract, and would silently reinterpret old blobs if
// spatial ever reordered it. Grid omits when empty (the square zero value)
// so pre-v0.2 blobs and square-only encounters keep byte-identical output.
type RoomData struct {
	ID         string         `json:"id"`
	Width      int            `json:"width"`
	Height     int            `json:"height"`
	Grid       string         `json:"grid,omitempty"`
	Occluders  []PositionData `json:"occluders,omitempty"`
	Boundaries []BoundaryData `json:"boundaries,omitempty"`
	Origin     *PositionData  `json:"origin"`
}

// PositionData is the persistent representation of spatial.Position.
type PositionData struct {
	X float64 `json:"x"`
	Y float64 `json:"y"`
}

// BoundaryData is the persistent representation of spatial.Boundary.
type BoundaryData struct {
	From              PositionData `json:"from"`
	To                PositionData `json:"to"`
	BlocksMovement    bool         `json:"blocks_movement,omitempty"`
	BlocksLineOfSight bool         `json:"blocks_line_of_sight,omitempty"`
}

// ConnectionData is the persistent representation of a connection.
// FromPosition and ToPosition are required — ToData always populates
// both; a nil pointer at Load means the field was absent from the
// blob and is rejected (a connection without both endpoints has no
// meaning, and a missing endpoint must never silently default to
// (0,0), a legal cell that would invent topology).
type ConnectionData struct {
	ID           string        `json:"id"`
	From         string        `json:"from"`
	To           string        `json:"to"`
	FromPosition *PositionData `json:"from_position"`
	ToPosition   *PositionData `json:"to_position"`
}

// MemberData is the persistent representation of a member's current placement.
type MemberData struct {
	ID       MemberID     `json:"id"`
	Kind     MemberKind   `json:"kind"`
	Room     string       `json:"room"`
	Position PositionData `json:"position"`
}

// EndingData is the persistent representation of a declared ending.
// Kind is one of "reached_position" or "external".
// Room, Position, and Member are only populated for reached_position triggers.
type EndingData struct {
	Key      string        `json:"key"`
	Kind     string        `json:"kind"`
	Room     string        `json:"room,omitempty"`
	Position *PositionData `json:"position,omitempty"`
	Member   MemberID      `json:"member,omitempty"`
}

// ToData returns a persistent snapshot of this Encounter.
// All slices and embedded data are deep-copied; mutating the returned
// EncounterData will not affect this Encounter (snapshot immunity).
func (e *Encounter) ToData() EncounterData {
	// Members in sorted-ID order: an unchanged encounter must produce
	// byte-identical snapshots (T6 review M1 — map iteration here made
	// ToData nondeterministic and the round-trip suite ~16% flaky).
	memberIDs := make([]MemberID, 0, len(e.members))
	for id := range e.members {
		memberIDs = append(memberIDs, id)
	}
	sort.Slice(memberIDs, func(i, j int) bool { return memberIDs[i] < memberIDs[j] })
	membersData := make([]MemberData, 0, len(memberIDs))
	for _, id := range memberIDs {
		m := e.members[id]
		room, ok := e.orchestrator.GetRoom(m.Room)
		if !ok {
			continue // Room not found (shouldn't happen in valid encounter)
		}
		pos, ok := room.GetEntityPosition(string(m.ID))
		if !ok {
			continue // Position not found (shouldn't happen in valid encounter)
		}
		membersData = append(membersData, MemberData{
			ID:       m.ID,
			Kind:     m.Kind,
			Room:     m.Room,
			Position: PositionData{X: pos.X, Y: pos.Y},
		})
	}

	// Deep-copy endings in declaration order
	endingsData := make([]EndingData, len(e.endings))
	for i, de := range e.endings {
		ed := EndingData{
			Key: de.key,
		}

		switch t := de.trigger.(type) {
		case TriggerReachedPosition:
			ed.Kind = "reached_position"
			ed.Room = t.Room
			ed.Position = &PositionData{X: t.Position.X, Y: t.Position.Y}
			ed.Member = t.Member
		case TriggerExternal:
			ed.Kind = "external"
		}

		endingsData[i] = ed
	}

	// Deep-copy everMembers in sorted order for determinism
	everMembersSlice := make([]MemberID, 0, len(e.everMembers))
	for id := range e.everMembers {
		everMembersSlice = append(everMembersSlice, id)
	}
	slices.SortFunc(everMembersSlice, func(a, b MemberID) int {
		if a < b {
			return -1
		}
		if a > b {
			return 1
		}
		return 0
	})

	// Deep-copy field from stored inputs
	fieldData := FieldData{
		Rooms:       make([]RoomData, len(e.fieldInput)),
		Connections: make([]ConnectionData, len(e.connectionsInput)),
	}

	for i, ri := range e.fieldInput {
		rd := RoomData{
			ID:         ri.ID,
			Width:      ri.Width,
			Height:     ri.Height,
			Grid:       gridShapeToData(ri.Grid),
			Occluders:  make([]PositionData, len(ri.Occluders)),
			Boundaries: make([]BoundaryData, len(ri.Boundaries)),
			// Always a fresh pointer, always written — even the zero value
			// (no omitempty, RoomData's own doc comment) — so a declared
			// origin (0,0) round-trips as explicit presence, not absence,
			// and two ToData calls never alias the same PositionData.
			Origin: &PositionData{X: ri.Origin.X, Y: ri.Origin.Y},
		}

		for j, occ := range ri.Occluders {
			rd.Occluders[j] = PositionData{X: occ.X, Y: occ.Y}
		}

		for j, b := range ri.Boundaries {
			rd.Boundaries[j] = BoundaryData{
				From:              PositionData{X: b.From.X, Y: b.From.Y},
				To:                PositionData{X: b.To.X, Y: b.To.Y},
				BlocksMovement:    b.BlocksMovement,
				BlocksLineOfSight: b.BlocksLineOfSight,
			}
		}

		fieldData.Rooms[i] = rd
	}

	for i, ci := range e.connectionsInput {
		fieldData.Connections[i] = ConnectionData{
			ID:           ci.ID,
			From:         ci.From,
			To:           ci.To,
			FromPosition: &PositionData{X: ci.FromPosition.X, Y: ci.FromPosition.Y},
			ToPosition:   &PositionData{X: ci.ToPosition.X, Y: ci.ToPosition.Y},
		}
	}

	// Build outcome if present
	var outcomeData *OutcomeData
	if e.outcome != nil {
		outcomeData = &OutcomeData{
			Ending:  e.outcome.Ending,
			At:      e.outcome.At,
			Members: make([]MemberOutcomeData, len(e.outcome.Members)),
		}
		for i, mo := range e.outcome.Members {
			outcomeData.Members[i] = MemberOutcomeData{
				ID:       mo.ID,
				Room:     mo.Room,
				Position: PositionData{X: mo.Position.X, Y: mo.Position.Y},
			}
		}
	}

	return EncounterData{
		Outcome:     outcomeData,
		Clock:       e.clock.ToData(),
		Intel:       e.intelLog.ToData(),
		Log:         e.story.ToData(),
		Field:       fieldData,
		Members:     membersData,
		Endings:     endingsData,
		EverMembers: everMembersSlice,
		Retention:   e.retention,
	}
}

// gridShapeToData maps a room's constructed grid shape to its persisted
// string form, reusing spatial's own GridType* constants
// (tools/spatial/data.go) so the wire vocabulary matches spatial's own
// persistence. Square (the zero value) maps to "" so byte-compat
// goldens keep omitting the field entirely. No GridShapeGridless case
// (#929 T2 second review round: deleted — write-only wire value at this
// point, since no RoomInput this function is ever called with can hold
// GridShapeGridless. At Setup, buildValidRoomGrids' shape-legality switch
// rejects it explicitly (encounter.go's doc comment on that switch); at
// Load it never even reaches that switch — gridDataToShape below rejects
// a stored "gridless" string at the wire layer, before a RoomInput ever
// exists, closing the load-side hole T1 left open — see gridDataToShape's
// own doc comment. Either way, a room is never stored in fieldInput and
// later handed to ToData while holding GridShapeGridless). The default
// case already returns "" for it, same as any other unreachable value.
func gridShapeToData(shape spatial.GridShape) string {
	switch shape {
	case spatial.GridShapeHex:
		return spatial.GridTypeHex
	default:
		return ""
	}
}

// gridDataToShape is gridShapeToData's inverse, used at Load. Empty
// string and the literal "square" both mean the zero-value shape —
// ToData never emits "square" (it omits the field), but Load accepts
// it defensively for hand-authored fixtures. An unrecognized string
// returns ok=false so the caller can reject with a fragment naming
// the bad value, rather than silently defaulting to square.
//
// "gridless" is deliberately NOT recognized (#929 T2): it was a v0.2
// value, dropped from the shape-legality-valid set in T1 (RoomInput.Grid's
// doc comment) — Setup has rejected it since T1, and a stored "gridless"
// string now falls through to the same ok=false rejection any other
// unrecognized string gets, closing the load-side hole T1 left open
// (a stored "gridless" blob loaded successfully until this change).
func gridDataToShape(s string) (shape spatial.GridShape, ok bool) {
	switch s {
	case "", spatial.GridTypeSquare:
		return spatial.GridShapeSquare, true
	case spatial.GridTypeHex:
		return spatial.GridShapeHex, true
	default:
		return spatial.GridShapeSquare, false
	}
}

// LoadEncounter reconstructs an Encounter from persistent data and re-attached deciders.
// Validation order (R5 — validate all before constructing): no rooms, no endings,
// empty/reserved ending keys, duplicate ending keys (#929 hardening round E, mirroring
// NewEncounter's identical check), kind/reached_position checks, undeclared outcome
// ending; THEN, per room list, a wire-only ID pre-pass (empty/duplicate room ID — #929 T2
// second review round, so a later presence error can never misname an empty/ambiguous
// ID), grid-shape resolution (unrecognized-or-no-longer-supported string) and origin
// presence (W5) per room; THEN room-list defects via the SAME buildValidRoomGrids Setup
// uses: shape legality, non-integral or duplicate occluder position in any family (#929
// T3 Opus round F2; duplicate — #929 hardening round D), W1 (one grid family per field),
// room legality (non-positive/oversized/over-cell-
// budget dimensions, out-of-bounds origin — maxRoomSpan/maxAnchorCoord/maxRoomCells/
// maxFieldCells), origin legality (non-representable origin, every family), W2 (rooms
// never overlap); THEN, per connection list, the SAME wire-only ID pre-pass and endpoint
// presence, then connection defects via validateConnectionInputs: unknown or
// self-referencing room, endpoint out of bounds, non-integral (hex), or on an occluder,
// W3 (non-kissing doorway); THEN empty or duplicate member IDs, member's room not in
// field, member position out of bounds or non-integral (hex), ending trigger validity
// (unknown room or unreachable position on a TriggerReachedPosition — #929 T3 Opus round
// F5, the SAME validateEndingTriggers Setup uses), an abandoned outcome with members
// still present, outcome member room/bounds checks, everMembers missing a current
// member.
//
// One check runs OUTSIDE this up-front pass, later, during member re-placement and
// decider re-attachment (construction has already begun by then): a player member
// naming a Decider in the reattachment map (design law C2, enforced identically at
// Setup, Join, and here). This is not an R5 violation — nothing external is mutated,
// and the partially-built Encounter is discarded on this error like any other — but it
// is a real ordering asymmetry against the up-front list above; a future cleanup could
// hoist it into the member validation loop instead.
//
// #929 T2: the room-list and connection validation is deliberately the SAME Setup
// runs, not a parallel reimplementation — Setup and Load diverging on the W-laws was
// flagged explicitly in T1 review as a drift risk. RoomData/ConnectionData convert to
// RoomInput/ConnectionInput FIRST (the ID pre-pass, grid-string resolution, and
// Origin/endpoint presence checks above — concerns that only exist at the wire layer,
// since RoomInput.Grid is already typed and RoomInput.Origin is already a value, not a
// pointer), then the converted slices are handed to
// buildValidRoomGrids/validateConnectionInputs verbatim; every error they return is
// wrapped once more with ErrInvalidData (multi-%w, this module's established load-error
// style — the underlying error already carries ErrNoField/ErrBadConnection). Neither
// shared validator's own error messages carry a verb prefix (#929 T2 second review
// round — buildValidRoomGrids' doc comment), so this wrap is the ONLY place "load
// encounter:" enters those messages — NewEncounter wraps the identical unprefixed
// errors with "newencounter:" instead, at its own call sites.
//
// Leaf loaders (clock, intel, record) are called and their rejections are wrapped.
// On success, the field is rebuilt via the same path Setup uses (no re-surveil),
// and members are re-placed at persisted positions.
func LoadEncounter(data EncounterData, deciders map[MemberID]Decider) (*Encounter, error) {
	// R5: Validate everything before constructing
	// No rooms
	if len(data.Field.Rooms) == 0 {
		return nil, fmt.Errorf("load encounter: no rooms: %w: %w", ErrInvalidData, ErrNoField)
	}

	// No endings
	if len(data.Endings) == 0 {
		return nil, fmt.Errorf("load encounter: bad endings: %w: %w", ErrInvalidData, ErrNoEnding)
	}

	// Validate ending keys and kinds: empty/reserved, duplicate (#929
	// hardening round E — the SAME liveness hole NewEncounter's
	// identical check closes, mirrored at Load), and kind.
	seenEndingKeys := make(map[string]bool, len(data.Endings))
	for _, ed := range data.Endings {
		if ed.Key == "" || ed.Key == "abandoned" {
			return nil, fmt.Errorf("load encounter: bad endings: %w: %w", ErrInvalidData, ErrNoEnding)
		}
		if seenEndingKeys[ed.Key] {
			return nil, fmt.Errorf("load encounter: duplicate ending %q: %w: %w", ed.Key, ErrInvalidData, ErrNoEnding)
		}
		seenEndingKeys[ed.Key] = true

		if ed.Kind != "reached_position" && ed.Kind != "external" {
			return nil, fmt.Errorf("load encounter: unknown ending kind %q: %w: %w", ed.Kind, ErrInvalidData, ErrNoEnding)
		}

		// A reached_position ending without a position would panic at
		// construction — LoadEncounter is the trust boundary for
		// persisted bytes and must reject, never crash (T6 review M2).
		if ed.Kind == "reached_position" && (ed.Position == nil || ed.Room == "") {
			return nil, fmt.Errorf("load encounter: ending %q reached_position without room/position: %w: %w", ed.Key, ErrInvalidData, ErrNoEnding)
		}
	}

	// Validate outcome (if present) references a declared ending
	if data.Outcome != nil {
		found := false
		if data.Outcome.Ending == "abandoned" {
			found = true // Reserved ending is valid
		} else {
			for _, ed := range data.Endings {
				if ed.Key == data.Outcome.Ending {
					found = true
					break
				}
			}
		}
		if !found {
			return nil, fmt.Errorf("load encounter: outcome ending %q not declared: %w: %w", data.Outcome.Ending, ErrInvalidData, ErrNoEnding)
		}
	}

	// Convert the wire representation to RoomInput/ConnectionInput — the
	// ONLY load-specific pre-validation left (grid-string resolution, W5
	// origin presence, connection endpoint presence: concerns that exist
	// only because the wire form uses strings and pointers where the
	// construction form uses typed values) — then hand off to the SAME
	// room-list and connection validation Setup uses (see this function's
	// doc comment). Every remaining room-list/connection defect class
	// (shape legality, W1, room legality, origin legality, W2, W3, and the
	// existing bounds/occluder/self-connection checks) is enforced by
	// buildValidRoomGrids/validateConnectionInputs, not duplicated here.
	roomInputs, err := convertRoomDataToRoomInput(data.Field.Rooms)
	if err != nil {
		return nil, fmt.Errorf("load encounter: %w: %w", ErrInvalidData, err)
	}
	roomGrids, err := buildValidRoomGrids(roomInputs)
	if err != nil {
		return nil, fmt.Errorf("load encounter: %w: %w", ErrInvalidData, err)
	}

	connectionInputs, err := convertConnectionDataToConnectionInput(data.Field.Connections)
	if err != nil {
		return nil, fmt.Errorf("load encounter: %w: %w", ErrInvalidData, err)
	}
	if err = validateConnectionInputs(roomInputs, roomGrids, connectionInputs); err != nil {
		return nil, fmt.Errorf("load encounter: %w: %w", ErrInvalidData, err)
	}

	// Validate members: no duplicates, rooms exist, positions in bounds
	seenIDs := make(map[MemberID]bool)
	for _, m := range data.Members {
		// Empty member IDs are unreachable (Setup and Join both reject).
		if m.ID == "" {
			return nil, fmt.Errorf("load encounter: empty member id: %w: %w", ErrInvalidData, ErrNoMember)
		}
		if seenIDs[m.ID] {
			return nil, fmt.Errorf("load encounter: duplicate member %q: %w: %w", m.ID, ErrInvalidData, ErrNoMember)
		}
		seenIDs[m.ID] = true

		if _, ok := roomGrids[m.Room]; !ok {
			return nil, fmt.Errorf("load encounter: member %q room %q not in field: %w: %w", m.ID, m.Room, ErrInvalidData, ErrBadPlacement)
		}

		// Validate position in bounds — grid-deferred: the room's own
		// constructed Grid decides validity (see roomGrids above).
		if !roomGrids[m.Room].IsValidPosition(spatial.Position{X: m.Position.X, Y: m.Position.Y}) {
			return nil, fmt.Errorf("load encounter: member %q position out of bounds: %w: %w", m.ID, ErrInvalidData, ErrBadPlacement)
		}

		// Hex rooms require integral axial member positions (interim
		// tools/spatial#926 enforcement — see isIntegralAxialPosition).
		if !isIntegralAxialPosition(roomGrids[m.Room], spatial.Position{X: m.Position.X, Y: m.Position.Y}) {
			return nil, fmt.Errorf("load encounter: member %q position is not an integral axial cell: %w: %w", m.ID, ErrInvalidData, ErrBadPlacement)
		}
	}

	// A TriggerReachedPosition ending must name a real room and reachable
	// position — the SAME shared validator Setup uses (#929 T3 Opus round
	// F5; validateEndingTriggers' doc comment), fed the wire endings
	// resolved to their runtime Trigger via endingTriggerFromData — the
	// SAME conversion the "Restore declared endings" construction below
	// reuses, so the switch on ed.Kind exists exactly once, not twice.
	endingInputsForValidation := make([]EndingInput, len(data.Endings))
	for i, ed := range data.Endings {
		endingInputsForValidation[i] = EndingInput{Key: ed.Key, Trigger: endingTriggerFromData(ed)}
	}
	if err := validateEndingTriggers(endingInputsForValidation, roomGrids); err != nil {
		return nil, fmt.Errorf("load encounter: %w: %w", ErrInvalidData, err)
	}

	// Outcome members must reference rooms that exist with in-bounds
	// positions (design R9 — the outcome list was wholly unvalidated,
	// T6 review M5), and an abandoned outcome with members still
	// present is unreachable (abandonment means the membership emptied).
	if data.Outcome != nil {
		if data.Outcome.Ending == "abandoned" && len(data.Members) > 0 {
			return nil, fmt.Errorf("load encounter: abandoned outcome with members present: %w: %w", ErrInvalidData, ErrNoMember)
		}
		for _, om := range data.Outcome.Members {
			_, ok := roomGrids[om.Room]
			if !ok {
				return nil, fmt.Errorf("load encounter: outcome member %q room %q not in field: %w: %w", om.ID, om.Room, ErrInvalidData, ErrBadPlacement)
			}
			if !roomGrids[om.Room].IsValidPosition(spatial.Position{X: om.Position.X, Y: om.Position.Y}) {
				return nil, fmt.Errorf("load encounter: outcome member %q position out of bounds: %w: %w", om.ID, ErrInvalidData, ErrBadPlacement)
			}
		}
	}

	// Validate everMembers contains all current members
	for _, m := range data.Members {
		found := false
		for _, em := range data.EverMembers {
			if em == m.ID {
				found = true
				break
			}
		}
		if !found {
			return nil, fmt.Errorf("load encounter: member %q missing from ever_members: %w: %w", m.ID, ErrInvalidData, ErrNoMember)
		}
	}

	// Leaf validation via loaders (delegated — they own their rules).
	// Load ONCE and keep the results: a second call with a discarded
	// error was a needless crash path (T6 review, item 8).
	loadedClock, err := clock.LoadTick(data.Clock)
	if err != nil {
		return nil, fmt.Errorf("load encounter clock: %w: %w", ErrInvalidData, err)
	}

	loadedIntel, err := intel.LoadIntel(data.Intel)
	if err != nil {
		return nil, fmt.Errorf("load encounter intel: %w: %w", ErrInvalidData, err)
	}

	loadedLog, err := record.LoadLog(data.Log)
	if err != nil {
		return nil, fmt.Errorf("load encounter log: %w: %w", ErrInvalidData, err)
	}

	// All validated; now reconstruct via the same path Setup uses
	e := &Encounter{
		members:     make(map[MemberID]*Member),
		everMembers: make(map[MemberID]bool),
		deciders:    make(map[MemberID]Decider),
		endings:     nil,
		retention:   normalizeRetention(data.Retention),
		logFloor:    logFloorOf(data.Log),
	}

	// Rebuild field via setup path (no bus, no surveil)
	e.orchestrator = spatial.NewBasicRoomOrchestrator(spatial.BasicRoomOrchestratorConfig{
		ID:     "encounter-orchestrator",
		Type:   "orchestrator",
		Layout: spatial.LayoutTypeOrganic,
	})

	// Create all rooms, reusing each room's already-constructed Grid
	// (roomGrids, built above) so validation and placement agree exactly.
	// Iterates roomInputs/connectionInputs (already validated, already
	// spatial-typed) rather than re-deriving from data.Field — the same
	// construction shape NewEncounter's own room-construction loop uses.
	spatialRoomMap := make(map[string]*spatial.BasicRoom)
	for roomIdx, ri := range roomInputs {
		room := spatial.NewBasicRoom(spatial.BasicRoomConfig{
			ID:   ri.ID,
			Type: "room",
			Grid: roomGrids[ri.ID],
		})
		err = e.orchestrator.AddRoom(room)
		if err != nil {
			return nil, fmt.Errorf("load encounter add room: %w: %w: %w", ErrInvalidData, ErrBadPlacement, err)
		}
		spatialRoomMap[ri.ID] = room

		// Add occluders as blocking entities. Index-based ID, not
		// room-ID-plus-coordinate concatenation (#929 hardening round
		// C — see NewEncounter's identical loop for the collision this
		// avoids; this is the SAME fix, mirrored at the Load seam).
		for occIdx, pos := range ri.Occluders {
			occluder := &occluderEntity{id: fmt.Sprintf("occluder-%d-%d", roomIdx, occIdx)}
			_, err = e.orchestrator.PlaceEntity(&spatial.PlaceEntityInput{
				RoomID:   spatial.RoomID(ri.ID),
				Entity:   occluder,
				Position: pos,
			})
			if err != nil {
				return nil, fmt.Errorf("load encounter occluder placement: %w: %w: %w", ErrInvalidData, ErrBadPlacement, err)
			}
		}

		// Add boundaries
		for _, b := range ri.Boundaries {
			boundaryRoom := spatialRoomMap[ri.ID]
			if boundaryRoom != nil {
				if br, ok := interface{}(boundaryRoom).(spatial.BoundaryAwareRoom); ok {
					err = br.RegisterBoundary(b)
					if err != nil {
						return nil, fmt.Errorf("load encounter boundary: %w: %w: %w", ErrInvalidData, ErrBadPlacement, err)
					}
				}
			}
		}
	}

	// Add connections
	for _, ci := range connectionInputs {
		door := spatial.CreateDoorConnection(ci.ID, ci.From, ci.To, 1.0)
		err = e.orchestrator.AddConnection(door)
		if err != nil {
			return nil, fmt.Errorf("load encounter add connection: %w: %w", ErrInvalidData, err)
		}
	}

	// Load leaf state (constructors always succeed after validation)
	e.clock = loadedClock
	e.intelLog = loadedIntel
	e.story = loadedLog

	// Re-place members at persisted positions (no surveil here — outcomes already in intel)
	for _, m := range data.Members {
		entity := &memberEntity{
			id:   string(m.ID),
			kind: m.Kind,
		}

		_, err = e.orchestrator.PlaceEntity(&spatial.PlaceEntityInput{
			RoomID:   spatial.RoomID(m.Room),
			Entity:   entity,
			Position: spatial.Position{X: m.Position.X, Y: m.Position.Y},
		})
		if err != nil {
			return nil, fmt.Errorf("load encounter member placement: %w: %w: %w", ErrInvalidData, ErrBadPlacement, err)
		}

		member := &Member{
			ID:   m.ID,
			Kind: m.Kind,
			Room: m.Room,
		}
		e.members[m.ID] = member
		e.everMembers[m.ID] = true

		// Re-attach decider if present and non-nil — a literal nil entry
		// in the reattachment map is equivalent to an ABSENT one: a
		// monster without a decider is legal and simply holds (Setup and
		// Join already treat a nil MemberInput.Decider this way). Storing
		// a nil Decider interface here would panic Pump's first Decide
		// call on that monster (reject-never-crash: LoadEncounter is the
		// trust boundary for the caller-supplied reattachment map too,
		// not just the persisted bytes). Players cannot carry deciders
		// regardless (C2, enforced at all three seams: Setup, Join, load).
		if d, ok := deciders[m.ID]; ok && d != nil {
			if m.Kind == KindPlayer {
				return nil, fmt.Errorf("load encounter: player %s cannot carry a decider: %w: %w", m.ID, ErrInvalidData, ErrNoMember)
			}
			e.deciders[m.ID] = d
		}
	}

	// Restore the full ever-members set (exited members keep Story access)
	for _, em := range data.EverMembers {
		e.everMembers[em] = true
	}

	// Restore declared endings — endingTriggerFromData is the SAME
	// conversion the ending-trigger validation above already ran.
	for _, ed := range data.Endings {
		e.endings = append(e.endings, declaredEnding{key: ed.Key, trigger: endingTriggerFromData(ed)})
	}

	// Restore outcome if present
	if data.Outcome != nil {
		outcome := &Outcome{
			Ending:  data.Outcome.Ending,
			At:      data.Outcome.At,
			Members: make([]MemberOutcome, len(data.Outcome.Members)),
		}
		for i, m := range data.Outcome.Members {
			outcome.Members[i] = MemberOutcome{
				ID:       m.ID,
				Room:     m.Room,
				Position: spatial.Position{X: m.Position.X, Y: m.Position.Y},
			}
		}
		e.outcome = outcome
	}

	// Store field and connections inputs for future ToData calls — the
	// SAME roomInputs/connectionInputs already built (and validated)
	// above, not re-converted: both were freshly allocated from data by
	// convertRoomDataToRoomInput/convertConnectionDataToConnectionInput,
	// never aliasing the caller's data (T6 review M4's alias-immunity
	// requirement, pinned at this seam by TestAliasImmunityLoadEncounter).
	// Connections are kept sorted by ID (C8 determinism — order is
	// observable in ToData).
	e.fieldInput = roomInputs
	e.connectionsInput = connectionInputs
	sort.Slice(e.connectionsInput, func(i, j int) bool { return e.connectionsInput[i].ID < e.connectionsInput[j].ID })

	return e, nil
}

// endingTriggerFromData converts one persisted ending's Kind/Room/Position/
// Member into its runtime Trigger. Shared by two call sites in
// LoadEncounter above — the ending-trigger validation (which needs it
// early, right after roomGrids exists, so validateEndingTriggers has
// something to check) and the "Restore declared endings" construction
// (which needs it again, once construction is safe to begin, R5) — ONE
// conversion, not two copies of the same switch (#929 T3 Opus round F5).
// ed.Kind is already guaranteed to be "reached_position" or "external" by
// the key/kind checks earlier in LoadEncounter, and a "reached_position"
// ed.Position is already guaranteed non-nil there too — both preconditions
// checked before this is ever called, so no error return is needed here.
func endingTriggerFromData(ed EndingData) Trigger {
	switch ed.Kind {
	case "reached_position":
		return TriggerReachedPosition{
			Room:     ed.Room,
			Position: spatial.Position{X: ed.Position.X, Y: ed.Position.Y},
			Member:   ed.Member,
		}
	case "external":
		return TriggerExternal{}
	}
	return nil
}

// convertRoomDataToRoomInput converts RoomData to RoomInput, both for the
// SAME room-list validation Setup uses (buildValidRoomGrids — see
// LoadEncounter's doc comment) and for later storage. This is the ONLY
// place LoadEncounter resolves the wire-only concerns that don't exist on
// RoomInput itself: ID presence/uniqueness (a cheap pre-pass, #929 T2
// second review round — see below), the Grid string (rejecting an
// unrecognized value, including the no-longer-supported "gridless" —
// gridDataToShape's doc comment), and Origin's W5 presence requirement (a
// nil pointer means the field was absent from the blob, distinct from a
// declared zero — RoomData's doc comment). All reject with ErrNoField,
// matching the room-list defect vocabulary buildValidRoomGrids itself uses
// for every OTHER room-list defect, so a caller inspecting the error chain
// sees one consistent sentinel regardless of which check fired.
//
// The ID pre-pass runs as its OWN first pass over the raw wire IDs, before
// any other conversion: without it, an empty or duplicate room ID could
// reach the Grid/Origin checks below and produce a message naming that
// same empty or ambiguous ID — e.g. `room "" missing origin` — instead of
// the actual defect (the ID itself). buildValidRoomGrids ALSO checks
// ID empty/duplicate, but only after every room in the list has already
// survived conversion — too late to prevent THIS symptom. The pre-pass is
// intentionally redundant with that later check for a room list that
// passes it: its only job is to guarantee an ID-defective room never
// reaches a presence error that would misname it.
func convertRoomDataToRoomInput(rooms []RoomData) ([]RoomInput, error) {
	seenIDs := make(map[string]bool, len(rooms))
	for _, rd := range rooms {
		if rd.ID == "" {
			return nil, fmt.Errorf("room has empty id: %w", ErrNoField)
		}
		if seenIDs[rd.ID] {
			return nil, fmt.Errorf("duplicate room %q: %w", rd.ID, ErrNoField)
		}
		seenIDs[rd.ID] = true
	}

	result := make([]RoomInput, len(rooms))
	for i, rd := range rooms {
		shape, ok := gridDataToShape(rd.Grid)
		if !ok {
			return nil, fmt.Errorf("room %q has unknown grid shape %q: %w", rd.ID, rd.Grid, ErrNoField)
		}
		if rd.Origin == nil {
			return nil, fmt.Errorf("room %q missing origin: %w", rd.ID, ErrNoField)
		}

		ri := RoomInput{
			ID:         rd.ID,
			Width:      rd.Width,
			Height:     rd.Height,
			Grid:       shape,
			Occluders:  make([]spatial.Position, len(rd.Occluders)),
			Boundaries: make([]spatial.Boundary, len(rd.Boundaries)),
			Origin:     spatial.Position{X: rd.Origin.X, Y: rd.Origin.Y},
		}

		for j, pd := range rd.Occluders {
			ri.Occluders[j] = spatial.Position{X: pd.X, Y: pd.Y}
		}

		for j, bd := range rd.Boundaries {
			ri.Boundaries[j] = spatial.Boundary{
				From:              spatial.Position{X: bd.From.X, Y: bd.From.Y},
				To:                spatial.Position{X: bd.To.X, Y: bd.To.Y},
				BlocksMovement:    bd.BlocksMovement,
				BlocksLineOfSight: bd.BlocksLineOfSight,
			}
		}

		result[i] = ri
	}
	return result, nil
}

// convertConnectionDataToConnectionInput converts ConnectionData to
// ConnectionInput, both for the SAME connection validation Setup uses
// (validateConnectionInputs — see LoadEncounter's doc comment) and for
// later storage. This is the ONLY place LoadEncounter resolves the
// wire-only concerns that don't exist on ConnectionInput itself: ID
// presence/uniqueness (a cheap pre-pass, #929 T2 second review round — same
// reasoning as convertRoomDataToRoomInput's own pre-pass, so an endpoint-
// presence error can never misname an empty/ambiguous connection ID) and a
// nil FromPosition/ToPosition pointer, meaning the field was absent from
// the blob, not merely zero-valued (ConnectionData's doc comment). Rejects
// with ErrBadConnection, matching validateConnectionInputs' own vocabulary
// for every other connection defect.
func convertConnectionDataToConnectionInput(conns []ConnectionData) ([]ConnectionInput, error) {
	seenIDs := make(map[string]bool, len(conns))
	for _, cd := range conns {
		if cd.ID == "" {
			return nil, fmt.Errorf("connection has empty id: %w", ErrBadConnection)
		}
		if seenIDs[cd.ID] {
			return nil, fmt.Errorf("duplicate connection %q: %w", cd.ID, ErrBadConnection)
		}
		seenIDs[cd.ID] = true
	}

	result := make([]ConnectionInput, len(conns))
	for i, cd := range conns {
		if cd.FromPosition == nil {
			return nil, fmt.Errorf("connection %q missing from_position: %w", cd.ID, ErrBadConnection)
		}
		if cd.ToPosition == nil {
			return nil, fmt.Errorf("connection %q missing to_position: %w", cd.ID, ErrBadConnection)
		}
		result[i] = ConnectionInput{
			ID:           cd.ID,
			From:         cd.From,
			To:           cd.To,
			FromPosition: spatial.Position{X: cd.FromPosition.X, Y: cd.FromPosition.Y},
			ToPosition:   spatial.Position{X: cd.ToPosition.X, Y: cd.ToPosition.Y},
		}
	}
	return result, nil
}
