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

// RoomData mirrors RoomInput exactly to persist construction inputs.
type RoomData struct {
	ID         string         `json:"id"`
	Width      int            `json:"width"`
	Height     int            `json:"height"`
	Occluders  []PositionData `json:"occluders,omitempty"`
	Boundaries []BoundaryData `json:"boundaries,omitempty"`
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
type ConnectionData struct {
	ID   string `json:"id"`
	From string `json:"from"`
	To   string `json:"to"`
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
			Occluders:  make([]PositionData, len(ri.Occluders)),
			Boundaries: make([]BoundaryData, len(ri.Boundaries)),
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
		fieldData.Connections[i] = ConnectionData(ci)
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

// LoadEncounter reconstructs an Encounter from persistent data and re-attached deciders.
// Validation order (R5 — validate all before constructing): nil-equivalent empty Data,
// no rooms, no endings, duplicate member IDs, member's room not in field, member position
// out of bounds, empty/reserved ending keys, undeclared outcome ending, connection
// referencing missing room, everMembers missing a current member.
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

	// Build room map for validation
	roomMap := make(map[string]bool)
	roomsByID := make(map[string]RoomData)
	for _, r := range data.Field.Rooms {
		roomMap[r.ID] = true
		roomsByID[r.ID] = r
	}

	// Validate connections reference existing rooms
	for _, c := range data.Field.Connections {
		if !roomMap[c.From] || !roomMap[c.To] {
			return nil, fmt.Errorf("load encounter: connection %q references missing room: %w", c.ID, ErrInvalidData)
		}
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

		if !roomMap[m.Room] {
			return nil, fmt.Errorf("load encounter: member %q room %q not in field: %w", m.ID, m.Room, ErrInvalidData)
		}

		// Validate position in bounds
		if m.Position.X < 0 || m.Position.Y < 0 {
			return nil, fmt.Errorf("load encounter: member %q position out of bounds: %w", m.ID, ErrInvalidData)
		}

		// Find room and check bounds
		var roomHeight, roomWidth int
		for _, r := range data.Field.Rooms {
			if r.ID == m.Room {
				roomWidth = r.Width
				roomHeight = r.Height
				break
			}
		}

		if m.Position.X >= float64(roomWidth) || m.Position.Y >= float64(roomHeight) {
			return nil, fmt.Errorf("load encounter: member %q position out of bounds: %w", m.ID, ErrInvalidData)
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
			r, ok := roomsByID[om.Room]
			if !ok {
				return nil, fmt.Errorf("load encounter: outcome member %q room %q not in field: %w", om.ID, om.Room, ErrInvalidData)
			}
			if om.Position.X < 0 || om.Position.Y < 0 || om.Position.X >= float64(r.Width) || om.Position.Y >= float64(r.Height) {
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

	// Create all rooms
	spatialRoomMap := make(map[string]*spatial.BasicRoom)
	for _, ri := range data.Field.Rooms {
		grid := spatial.NewSquareGrid(spatial.SquareGridConfig{
			Width:  float64(ri.Width),
			Height: float64(ri.Height),
		})
		room := spatial.NewBasicRoom(spatial.BasicRoomConfig{
			ID:   ri.ID,
			Type: "room",
			Grid: grid,
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
				Position: spatial.Position{X: pos.X, Y: pos.Y},
			})
			if err != nil {
				return nil, fmt.Errorf("load encounter occluder placement: %w: %w: %w", ErrInvalidData, ErrBadPlacement, err)
			}
		}

		// Add boundaries
		for _, b := range ri.Boundaries {
			boundary := spatial.Boundary{
				From:              spatial.Position{X: b.From.X, Y: b.From.Y},
				To:                spatial.Position{X: b.To.X, Y: b.To.Y},
				BlocksMovement:    b.BlocksMovement,
				BlocksLineOfSight: b.BlocksLineOfSight,
			}
			boundaryRoom := spatialRoomMap[ri.ID]
			if boundaryRoom != nil {
				if br, ok := interface{}(boundaryRoom).(spatial.BoundaryAwareRoom); ok {
					err = br.RegisterBoundary(boundary)
					if err != nil {
						return nil, fmt.Errorf("load encounter boundary: %w: %w: %w", ErrInvalidData, ErrBadPlacement, err)
					}
				}
			}
		}
	}

	// Add connections
	for _, ci := range data.Field.Connections {
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

		// Re-attach decider if present — players cannot carry deciders
		// (C2, enforced at all three seams: Setup, Join, and load).
		if d, ok := deciders[m.ID]; ok {
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

	// Store field and connections inputs for future ToData calls
	e.fieldInput = convertRoomDataToRoomInput(data.Field.Rooms)
	e.connectionsInput = convertConnectionDataToConnectionInput(data.Field.Connections)

	return e, nil
}

// convertRoomDataToRoomInput converts RoomData back to RoomInput for storage.
func convertRoomDataToRoomInput(rooms []RoomData) []RoomInput {
	result := make([]RoomInput, len(rooms))
	for i, rd := range rooms {
		ri := RoomInput{
			ID:         rd.ID,
			Width:      rd.Width,
			Height:     rd.Height,
			Occluders:  make([]spatial.Position, len(rd.Occluders)),
			Boundaries: make([]spatial.Boundary, len(rd.Boundaries)),
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
	return result
}

// convertConnectionDataToConnectionInput converts ConnectionData back to ConnectionInput for storage.
func convertConnectionDataToConnectionInput(conns []ConnectionData) []ConnectionInput {
	result := make([]ConnectionInput, len(conns))
	for i, cd := range conns {
		result[i] = ConnectionInput(cd)
	}
	return result
}
