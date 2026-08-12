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
	}
}

// gridShapeToData maps a room's constructed grid shape to its persisted
// string form, reusing spatial's own GridType* constants
// (tools/spatial/data.go) so the wire vocabulary matches spatial's own
// persistence. Square (the zero value) maps to "" so byte-compat
// goldens keep omitting the field entirely.
func gridShapeToData(shape spatial.GridShape) string {
	switch shape {
	case spatial.GridShapeHex:
		return spatial.GridTypeHex
	case spatial.GridShapeGridless:
		return spatial.GridTypeGridless
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
// Validation order (R5 — validate all before constructing): nil-equivalent empty Data,
// no rooms, no endings, empty/reserved ending keys (and kind/reached_position checks),
// undeclared outcome ending, room defects (empty/duplicate ID, unrecognized-or-no-longer-
// supported grid shape, missing origin — W5 presence), then room-list and connection
// defects via the SAME buildValidRoomGrids/validateConnectionInputs Setup uses: W1 (one
// grid family per field), non-positive dimensions, non-representable/non-integral origin
// (every family), W2 (rooms never overlap), and connection defects (empty/duplicate ID,
// missing room, self-connection, missing endpoint, endpoint out of bounds or on an
// occluder, W3 non-kissing doorway) — then duplicate member IDs, member's room not in
// field, member position out of bounds, outcome member room/bounds checks, everMembers
// missing a current member.
//
// #929 T2: this is deliberately the SAME validation Setup runs, not a parallel
// reimplementation — Setup and Load diverging on the W-laws was flagged explicitly in
// T1 review as a drift risk. RoomData/ConnectionData convert to RoomInput/ConnectionInput
// FIRST (resolving the grid string and checking Origin/endpoint presence — concerns that
// only exist at the wire layer, since RoomInput.Grid is already typed and
// RoomInput.Origin is already a value, not a pointer), then the converted slices are
// handed to buildValidRoomGrids/validateConnectionInputs verbatim; every error they
// return is wrapped once more with ErrInvalidData (multi-%w, this module's established
// load-error style — the underlying error already carries ErrNoField/ErrBadConnection).
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

	// Validate ending keys and kinds
	for _, ed := range data.Endings {
		if ed.Key == "" || ed.Key == "abandoned" {
			return nil, fmt.Errorf("load encounter: bad endings: %w: %w", ErrInvalidData, ErrNoEnding)
		}

		if ed.Kind != "reached_position" && ed.Kind != "external" {
			return nil, fmt.Errorf("load encounter: unknown ending kind %q: %w", ed.Kind, ErrInvalidData)
		}

		// A reached_position ending without a position would panic at
		// construction — LoadEncounter is the trust boundary for
		// persisted bytes and must reject, never crash (T6 review M2).
		if ed.Kind == "reached_position" && (ed.Position == nil || ed.Room == "") {
			return nil, fmt.Errorf("load encounter: ending %q reached_position without room/position: %w", ed.Key, ErrInvalidData)
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
			return nil, fmt.Errorf("load encounter: outcome ending %q not declared: %w", data.Outcome.Ending, ErrInvalidData)
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
			return nil, fmt.Errorf("load encounter: empty member id: %w", ErrInvalidData)
		}
		if seenIDs[m.ID] {
			return nil, fmt.Errorf("load encounter: duplicate member %q: %w", m.ID, ErrInvalidData)
		}
		seenIDs[m.ID] = true

		if _, ok := roomGrids[m.Room]; !ok {
			return nil, fmt.Errorf("load encounter: member %q room %q not in field: %w", m.ID, m.Room, ErrInvalidData)
		}

		// Validate position in bounds — grid-deferred: the room's own
		// constructed Grid decides validity (see roomGrids above).
		if !roomGrids[m.Room].IsValidPosition(spatial.Position{X: m.Position.X, Y: m.Position.Y}) {
			return nil, fmt.Errorf("load encounter: member %q position out of bounds: %w", m.ID, ErrInvalidData)
		}

		// Hex rooms require integral axial member positions (interim
		// tools/spatial#926 enforcement — see isIntegralAxialPosition).
		if !isIntegralAxialPosition(roomGrids[m.Room], spatial.Position{X: m.Position.X, Y: m.Position.Y}) {
			return nil, fmt.Errorf("load encounter: member %q position is not an integral axial cell: %w", m.ID, ErrInvalidData)
		}
	}

	// Outcome members must reference rooms that exist with in-bounds
	// positions (design R9 — the outcome list was wholly unvalidated,
	// T6 review M5), and an abandoned outcome with members still
	// present is unreachable (abandonment means the membership emptied).
	if data.Outcome != nil {
		if data.Outcome.Ending == "abandoned" && len(data.Members) > 0 {
			return nil, fmt.Errorf("load encounter: abandoned outcome with members present: %w", ErrInvalidData)
		}
		for _, om := range data.Outcome.Members {
			_, ok := roomGrids[om.Room]
			if !ok {
				return nil, fmt.Errorf("load encounter: outcome member %q room %q not in field: %w", om.ID, om.Room, ErrInvalidData)
			}
			if !roomGrids[om.Room].IsValidPosition(spatial.Position{X: om.Position.X, Y: om.Position.Y}) {
				return nil, fmt.Errorf("load encounter: outcome member %q position out of bounds: %w", om.ID, ErrInvalidData)
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
			return nil, fmt.Errorf("load encounter: member %q missing from ever_members: %w", m.ID, ErrInvalidData)
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
	for _, ri := range roomInputs {
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

		// Add occluders as blocking entities
		for _, pos := range ri.Occluders {
			occluder := &occluderEntity{id: fmt.Sprintf("occluder-%s-%d-%d", ri.ID, int(pos.X), int(pos.Y))}
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

	// Restore declared endings
	for _, ed := range data.Endings {
		var trigger Trigger
		switch ed.Kind {
		case "reached_position":
			trigger = TriggerReachedPosition{
				Room:     ed.Room,
				Position: spatial.Position{X: ed.Position.X, Y: ed.Position.Y},
				Member:   ed.Member,
			}
		case "external":
			trigger = TriggerExternal{}
		}
		e.endings = append(e.endings, declaredEnding{key: ed.Key, trigger: trigger})
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

// convertRoomDataToRoomInput converts RoomData to RoomInput, both for the
// SAME room-list validation Setup uses (buildValidRoomGrids — see
// LoadEncounter's doc comment) and for later storage. This is the ONLY
// place LoadEncounter resolves the wire-only concerns that don't exist on
// RoomInput itself: the Grid string (rejecting an unrecognized value,
// including the no-longer-supported "gridless" — gridDataToShape's doc
// comment) and Origin's W5 presence requirement (a nil pointer means the
// field was absent from the blob, distinct from a declared zero — RoomData's
// doc comment). Both reject with ErrNoField, matching the room-list defect
// vocabulary buildValidRoomGrids itself uses for every OTHER room-list
// defect, so a caller inspecting the error chain sees one consistent
// sentinel regardless of which check fired.
func convertRoomDataToRoomInput(rooms []RoomData) ([]RoomInput, error) {
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
// wire-only endpoint-presence concern that doesn't exist on ConnectionInput
// itself: a nil FromPosition/ToPosition pointer means the field was absent
// from the blob (not merely zero-valued) — ConnectionData's doc comment.
// Rejects with ErrBadConnection, matching validateConnectionInputs' own
// vocabulary for every other connection defect.
func convertConnectionDataToConnectionInput(conns []ConnectionData) ([]ConnectionInput, error) {
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
