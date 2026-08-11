// Copyright (C) 2026 Kirk Diggler
// SPDX-License-Identifier: GPL-3.0-or-later

package encounter

import (
	"encoding/json"
	"fmt"
	"sort"

	"github.com/KirkDiggler/rpg-toolkit/core"
	"github.com/KirkDiggler/rpg-toolkit/play/clock"
	"github.com/KirkDiggler/rpg-toolkit/play/intel"
	"github.com/KirkDiggler/rpg-toolkit/play/record"
	"github.com/KirkDiggler/rpg-toolkit/tools/spatial"
)

// SightPayload is the composition-owned encoding of a sighted member's position —
// the payload of every sight-channel intel report. Hosts decode it to render what
// a player sees; intel itself never interprets it.
type SightPayload struct {
	Room string  `json:"room"`
	X    float64 `json:"x"`
	Y    float64 `json:"y"`
}

// Encounter is the aggregate encounter composition: members, field, clock,
// intel, and record. Construct via NewEncounter; zero value unusable.
// declaredEnding pairs an ending key with its trigger, in Setup order.
type declaredEnding struct {
	key     string
	trigger Trigger
}

type Encounter struct {
	orchestrator *spatial.BasicRoomOrchestrator
	clock        *clock.Tick
	intelLog     *intel.Intel
	story        *record.Log
	members      map[MemberID]*Member
	everMembers  map[MemberID]bool // Track all members who have ever joined (for Story access)
	deciders     map[MemberID]Decider
	// endings holds declared endings in Setup order — evaluation is
	// deterministic, first-declared-wins (law C8).
	endings []declaredEnding
	outcome *Outcome
	// fieldInput and connectionsInput are stored at Setup time for persistence.
	// LoadEncounter uses them to rebuild the field deterministically without
	// re-running surveil or requiring an event bus.
	fieldInput       []RoomInput
	connectionsInput []ConnectionInput
}

// deepCopyRoomInputs snapshots the caller's room descriptions — the
// persistence source must never alias caller-owned slices (T6 review
// M4: a caller editing its SetupInput after construction silently
// corrupted ToData and made the encounter unsavable).
func deepCopyRoomInputs(rooms []RoomInput) []RoomInput {
	out := make([]RoomInput, len(rooms))
	for i, r := range rooms {
		rc := r
		rc.Occluders = append([]spatial.Position(nil), r.Occluders...)
		rc.Boundaries = append([]spatial.Boundary(nil), r.Boundaries...)
		out[i] = rc
	}
	return out
}

// positionInBounds reports whether (x, y) lies within a room of the given
// width and height — the same rule applied to member placement: non-negative
// and strictly less than each dimension. Shared by connection validation at
// both Setup and Load.
func positionInBounds(x, y float64, width, height int) bool {
	return x >= 0 && y >= 0 && x < float64(width) && y < float64(height)
}

// validateConnectionInputs rejects connection defects before construction
// (R5 atomicity — no observable state until Setup succeeds): empty or
// duplicate ID, an unknown or self-referencing room, and an endpoint outside
// its room's bounds or on an occluder position.
func validateConnectionInputs(rooms []RoomInput, connections []ConnectionInput) error {
	roomsByID := make(map[string]RoomInput, len(rooms))
	for _, r := range rooms {
		roomsByID[r.ID] = r
	}

	seenIDs := make(map[string]bool, len(connections))
	for _, c := range connections {
		if c.ID == "" {
			return fmt.Errorf("newencounter: connection has empty id: %w", ErrBadConnection)
		}
		if seenIDs[c.ID] {
			return fmt.Errorf("newencounter: duplicate connection %q: %w", c.ID, ErrBadConnection)
		}
		seenIDs[c.ID] = true

		fromRoom, ok := roomsByID[c.From]
		if !ok {
			return fmt.Errorf("newencounter: connection %q references unknown room %q: %w", c.ID, c.From, ErrBadConnection)
		}
		toRoom, ok := roomsByID[c.To]
		if !ok {
			return fmt.Errorf("newencounter: connection %q references unknown room %q: %w", c.ID, c.To, ErrBadConnection)
		}
		if c.From == c.To {
			return fmt.Errorf("newencounter: connection %q connects room %q to itself: %w", c.ID, c.From, ErrBadConnection)
		}

		if !positionInBounds(c.FromPosition.X, c.FromPosition.Y, fromRoom.Width, fromRoom.Height) {
			return fmt.Errorf("newencounter: connection %q from-position out of bounds: %w", c.ID, ErrBadConnection)
		}
		if !positionInBounds(c.ToPosition.X, c.ToPosition.Y, toRoom.Width, toRoom.Height) {
			return fmt.Errorf("newencounter: connection %q to-position out of bounds: %w", c.ID, ErrBadConnection)
		}

		for _, occ := range fromRoom.Occluders {
			if occ.X == c.FromPosition.X && occ.Y == c.FromPosition.Y {
				return fmt.Errorf("newencounter: connection %q from-position on occluder: %w", c.ID, ErrBadConnection)
			}
		}
		for _, occ := range toRoom.Occluders {
			if occ.X == c.ToPosition.X && occ.Y == c.ToPosition.Y {
				return fmt.Errorf("newencounter: connection %q to-position on occluder: %w", c.ID, ErrBadConnection)
			}
		}
	}
	return nil
}

// NewEncounter constructs and initializes an encounter from SetupInput.
// Validation order (first failure wins, R5 atomicity): nil input, no rooms,
// no endings, reserved ending key, empty member ID, duplicate member IDs,
// connection defects (empty/duplicate ID, unknown room, self-connection,
// endpoint out of bounds or on an occluder), spatial placement errors.
func NewEncounter(in *SetupInput) (*Encounter, error) {
	// Validation order: nil, no rooms, no endings, reserved ending, empty ID, duplicates
	if in == nil {
		return nil, fmt.Errorf("newencounter: %w", ErrNilInput)
	}

	if len(in.Field.Rooms) == 0 {
		return nil, fmt.Errorf("newencounter: %w", ErrNoField)
	}

	if len(in.Endings) == 0 {
		return nil, fmt.Errorf("newencounter: %w", ErrNoEnding)
	}

	// Check ending keys
	for _, ending := range in.Endings {
		if ending.Key == "" || ending.Key == "abandoned" {
			return nil, fmt.Errorf("newencounter: %w", ErrNoEnding)
		}
	}

	// Check member IDs: empty or duplicate; validate deciders
	seenIDs := make(map[MemberID]bool)
	for _, m := range in.Members {
		if m.ID == "" {
			return nil, fmt.Errorf("newencounter: %w", ErrNoMember)
		}
		if seenIDs[m.ID] {
			return nil, fmt.Errorf("newencounter: duplicate member %s: %w", m.ID, ErrNoMember)
		}
		seenIDs[m.ID] = true

		// Players cannot carry deciders (design law C2)
		if m.Kind == KindPlayer && m.Decider != nil {
			return nil, fmt.Errorf("newencounter: player %s cannot carry a decider: %w", m.ID, ErrNoMember)
		}
	}

	// Check connections: unique non-empty IDs, endpoints resolve to distinct
	// declared rooms, endpoints in bounds and off any occluder.
	if err := validateConnectionInputs(in.Field.Rooms, in.Field.Connections); err != nil {
		return nil, err
	}

	// Connections are stored sorted by ID (C8 determinism — order is
	// observable in ToData).
	connectionsInput := append([]ConnectionInput(nil), in.Field.Connections...)
	sort.Slice(connectionsInput, func(i, j int) bool { return connectionsInput[i].ID < connectionsInput[j].ID })

	// After validation passes, construct (R5: no observable state until success)
	e := &Encounter{
		members:          make(map[MemberID]*Member),
		everMembers:      make(map[MemberID]bool),
		deciders:         make(map[MemberID]Decider),
		endings:          nil,
		fieldInput:       deepCopyRoomInputs(in.Field.Rooms),
		connectionsInput: connectionsInput,
	}

	// Build clock and intel
	var err error
	e.clock, err = clock.NewTick()
	if err != nil {
		return nil, fmt.Errorf("newencounter clock: %w", err)
	}

	e.intelLog, err = intel.NewIntel()
	if err != nil {
		return nil, fmt.Errorf("newencounter intel: %w", err)
	}

	e.story, err = record.NewLog()
	if err != nil {
		return nil, fmt.Errorf("newencounter story: %w", err)
	}

	// Build field: orchestrator and rooms
	e.orchestrator = spatial.NewBasicRoomOrchestrator(spatial.BasicRoomOrchestratorConfig{
		ID:     "encounter-orchestrator",
		Type:   "orchestrator",
		Layout: spatial.LayoutTypeOrganic,
	})

	// Create all rooms
	roomMap := make(map[string]*spatial.BasicRoom)
	for _, ri := range in.Field.Rooms {
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
			return nil, fmt.Errorf("newencounter add room: %w", err)
		}
		roomMap[ri.ID] = room

		// Add occluders as blocking entities
		for _, pos := range ri.Occluders {
			occluder := &occluderEntity{id: fmt.Sprintf("occluder-%s-%d-%d", ri.ID, int(pos.X), int(pos.Y))}
			_, err = e.orchestrator.PlaceEntity(&spatial.PlaceEntityInput{
				RoomID:   spatial.RoomID(ri.ID),
				Entity:   occluder,
				Position: pos,
			})
			if err != nil {
				return nil, fmt.Errorf("newencounter occluder placement: %w: %w", ErrBadPlacement, err)
			}
		}

		// Add boundaries
		for _, b := range ri.Boundaries {
			boundaryRoom := roomMap[ri.ID]
			if boundaryRoom != nil {
				if br, ok := interface{}(boundaryRoom).(spatial.BoundaryAwareRoom); ok {
					err = br.RegisterBoundary(b)
					if err != nil {
						return nil, fmt.Errorf("newencounter boundary: %w: %w", ErrBadPlacement, err)
					}
				}
			}
		}
	}

	// Add connections
	for _, ci := range in.Field.Connections {
		door := spatial.CreateDoorConnection(ci.ID, ci.From, ci.To, 1.0)
		err = e.orchestrator.AddConnection(door)
		if err != nil {
			return nil, fmt.Errorf("newencounter add connection: %w", err)
		}
	}

	// Place members and collect them
	memberIDs := make([]MemberID, 0, len(in.Members))
	for _, mi := range in.Members {
		memberIDs = append(memberIDs, mi.ID)

		entity := &memberEntity{
			id:   string(mi.ID),
			kind: mi.Kind,
		}

		_, err = e.orchestrator.PlaceEntity(&spatial.PlaceEntityInput{
			RoomID:   spatial.RoomID(mi.Room),
			Entity:   entity,
			Position: mi.Position,
		})
		if err != nil {
			return nil, fmt.Errorf("newencounter member placement: %w: %w", ErrBadPlacement, err)
		}

		member := &Member{
			ID:   mi.ID,
			Kind: mi.Kind,
			Room: mi.Room,
		}
		e.members[mi.ID] = member
		e.everMembers[mi.ID] = true // Track in everMembers

		// Store decider if present (monsters only, validated above)
		if mi.Decider != nil {
			e.deciders[mi.ID] = mi.Decider
		}
	}

	// Store endings in declaration order (deterministic evaluation, C8)
	for _, ei := range in.Endings {
		e.endings = append(e.endings, declaredEnding{key: ei.Key, trigger: ei.Trigger})
	}

	// First light: build sight percepts for each member using refreshSight
	_, err = e.refreshSight(memberIDs)
	if err != nil {
		return nil, fmt.Errorf("newencounter first light: %w", err)
	}

	// Opening record beat: all members hear "scene-opened"
	beatPayload, _ := json.Marshal(map[string]string{"beat": "scene-opened"})
	_, err = e.story.Append(&record.AppendInput{
		At:       0,
		Audience: memberIDs,
		Tags:     map[string]string{"tag": "scene"},
		Payload:  beatPayload,
	})
	if err != nil {
		return nil, fmt.Errorf("newencounter append beat: %w", err)
	}

	return e, nil
}

// View returns the member's current intel holdings.
// Returns ErrNotMember if the member is not part of this encounter.
func (e *Encounter) View(in *ViewInput) ([]intel.Holding, error) {
	if in == nil {
		return nil, fmt.Errorf("view: %w", ErrNilInput)
	}

	if _, ok := e.members[in.Member]; !ok {
		return nil, fmt.Errorf("view: %w", ErrNotMember)
	}

	holdings, err := e.intelLog.HeldBy(&intel.HeldByInput{Observer: in.Member})
	if err != nil {
		return nil, fmt.Errorf("view: %w", err)
	}

	return holdings, nil
}

// Members returns the current member roster in stable order.

// buildMemberOutcomes snapshots every current member's placement in
// sorted-ID order — deterministic output for outcomes and persistence
// (map iteration here was a latent nondeterminism, T6 review M1).
func (e *Encounter) buildMemberOutcomes() []MemberOutcome {
	ids := make([]MemberID, 0, len(e.members))
	for id := range e.members {
		ids = append(ids, id)
	}
	sort.Slice(ids, func(i, j int) bool { return ids[i] < ids[j] })
	outcomes := make([]MemberOutcome, 0, len(ids))
	for _, id := range ids {
		m := e.members[id]
		mRoom, ok := e.orchestrator.GetRoom(m.Room)
		if !ok {
			continue
		}
		mPos, ok := mRoom.GetEntityPosition(string(m.ID))
		if !ok {
			continue
		}
		outcomes = append(outcomes, MemberOutcome{ID: m.ID, Room: m.Room, Position: mPos})
	}
	return outcomes
}

func (e *Encounter) Members() ([]Member, error) {
	// Sort by ID for stability
	ids := make([]MemberID, 0, len(e.members))
	for id := range e.members {
		ids = append(ids, id)
	}
	sort.Slice(ids, func(i, j int) bool {
		return ids[i] < ids[j]
	})

	members := make([]Member, 0, len(ids))
	for _, id := range ids {
		members = append(members, *e.members[id])
	}
	return members, nil
}

// Status returns the encounter's current state (Open or Closed with Outcome).
// Returns a deep copy of the outcome to prevent aliasing (MUTATION-PROOF).
func (e *Encounter) Status() (*Status, error) {
	if e.outcome != nil {
		// Deep-copy outcome and its Members slice to prevent aliasing
		// (mutation-proof: modifying returned outcome does not affect internal state)
		members := make([]MemberOutcome, len(e.outcome.Members))
		for i, m := range e.outcome.Members {
			members[i] = MemberOutcome{
				ID:       m.ID,
				Room:     m.Room,
				Position: m.Position,
			}
		}
		return &Status{
			Open: false,
			Outcome: &Outcome{
				Ending:  e.outcome.Ending,
				At:      e.outcome.At,
				Members: members,
			},
		}, nil
	}
	return &Status{Open: true}, nil
}

// Story returns the story entries for a member after the given sequence number.
// Allows both current members and members who have exited (everMembers).
// Returns ErrNilInput if the input is nil, ErrNoMember if the member never joined.
// Copy-out follows record's own conventions (returned entries are already copies
// per record's implementation).
func (e *Encounter) Story(in *StoryInput) ([]record.Entry, error) {
	if in == nil {
		return nil, fmt.Errorf("story: %w", ErrNilInput)
	}

	if _, ok := e.everMembers[in.Audience]; !ok {
		return nil, fmt.Errorf("story: %w", ErrNoMember)
	}

	entries, err := e.story.SliceFor(&record.SliceForInput{
		Viewer:  in.Audience,
		FromSeq: in.AfterSeq,
	})
	if err != nil {
		return nil, fmt.Errorf("story: %w", err)
	}

	return entries, nil
}

// moveMember executes a spatial move for a member and returns the old position if successful,
// or an error if the spatial move was rejected. This is the shared managed seam for both
// player moves (Move verb) and monster moves (Pump). The member must exist and be in an
// open encounter; spatial rejection does not abort the operation (handled by caller).
func (e *Encounter) moveMember(member *Member, to spatial.Position) (spatial.Position, error) {
	// Get the room and current position
	room, ok := e.orchestrator.GetRoom(member.Room)
	if !ok {
		return spatial.Position{}, fmt.Errorf("movemember: %w", ErrBadPlacement)
	}

	currentPos, ok := room.GetEntityPosition(string(member.ID))
	if !ok {
		return spatial.Position{}, fmt.Errorf("movemember: %w", ErrBadPlacement)
	}

	// Attempt the spatial move via managed seam using MoveEntity
	_, err := e.orchestrator.MoveEntity(&spatial.MoveEntityInput{
		RoomID:   spatial.RoomID(member.Room),
		EntityID: core.EntityID(member.ID),
		To:       to,
	})
	if err != nil {
		return spatial.Position{}, fmt.Errorf("movemember: %w: %w", ErrBadPlacement, err)
	}

	return currentPos, nil
}

// Move executes a continuous movement within the same room for ANY
// member — players move themselves; Pump routes monster intents through
// the same path. Unfiltered ReachedPosition endings fire only for
// player members; member-filtered endings fire only for the named
// member.
// Validation order (R5 atomicity): nil input → empty member → closed →
// not a member → spatial move rejection. On success, refreshes sight for all members,
// records beat, and evaluates ReachedPosition endings.
func (e *Encounter) Move(in *MoveInput) (*MoveOutput, error) {
	// Validation order
	if in == nil {
		return nil, fmt.Errorf("move: %w", ErrNilInput)
	}

	if in.Member == "" {
		return nil, fmt.Errorf("move: %w", ErrNoMember)
	}

	if e.outcome != nil {
		return nil, fmt.Errorf("move: %w", ErrClosed)
	}

	member, ok := e.members[in.Member]
	if !ok {
		return nil, fmt.Errorf("move: %w", ErrNotMember)
	}

	// Execute the move through the managed seam
	currentPos, err := e.moveMember(member, in.To)
	if err != nil {
		return nil, fmt.Errorf("move: %w", err)
	}

	// Get all member IDs for the refresh scope (v1: refresh everyone)
	memberIDs := make([]MemberID, 0, len(e.members))
	for id := range e.members {
		memberIDs = append(memberIDs, id)
	}
	sort.Slice(memberIDs, func(i, j int) bool { return memberIDs[i] < memberIDs[j] })

	// Refresh sight for all members
	intelDeltas, err := e.refreshSight(memberIDs)
	if err != nil {
		return nil, fmt.Errorf("move refresh sight: %w", err)
	}

	// Record the movement beat
	clockReadingInt := e.clock.ToData().HighWater
	clockReadingForBeat := uint64(clockReadingInt)
	beatPayload := map[string]interface{}{
		"beat":     "moved",
		"member":   string(in.Member),
		"position": in.To,
	}
	beatBytes, _ := json.Marshal(beatPayload)

	appendOut, err := e.story.Append(&record.AppendInput{
		At:       clockReadingForBeat,
		Audience: memberIDs,
		Tags:     map[string]string{"tag": "movement"},
		Payload:  beatBytes,
	})
	if err != nil {
		return nil, fmt.Errorf("move append beat: %w", err)
	}

	seqNum := appendOut.Seq

	// Evaluate ReachedPosition endings
	var firedOutcome *Outcome
	for _, de := range e.endings {
		endingKey, trigger := de.key, de.trigger
		reachedPosTrigger, ok := trigger.(TriggerReachedPosition)
		if !ok {
			continue // Not a ReachedPosition trigger
		}

		// Check if the ending fires
		if reachedPosTrigger.Room != member.Room {
			continue // Different room
		}

		if reachedPosTrigger.Position.X != in.To.X || reachedPosTrigger.Position.Y != in.To.Y {
			continue // Different position
		}

		// Check member filter: empty = any player member; non-empty = specific member
		if reachedPosTrigger.Member != "" && reachedPosTrigger.Member != in.Member {
			continue // Member filter doesn't match
		}

		// Member filter passes (empty or matches) but we need to check kind
		if reachedPosTrigger.Member == "" && member.Kind != KindPlayer {
			continue // Empty filter means players only, but moved member is not player
		}

		// Ending fires! Build the outcome with all members' current positions
		memberOutcomes := e.buildMemberOutcomes()

		e.outcome = &Outcome{
			Ending:  endingKey,
			At:      clockReadingForBeat,
			Members: memberOutcomes,
		}
		// Return a deep copy of the outcome (mutation-proof)
		outcomeMembers := make([]MemberOutcome, len(e.outcome.Members))
		for i, m := range e.outcome.Members {
			outcomeMembers[i] = MemberOutcome{
				ID:       m.ID,
				Room:     m.Room,
				Position: m.Position,
			}
		}
		firedOutcome = &Outcome{
			Ending:  e.outcome.Ending,
			At:      e.outcome.At,
			Members: outcomeMembers,
		}
		break
	}

	return &MoveOutput{
		Moved: struct {
			Member MemberID
			From   spatial.Position
			To     spatial.Position
		}{
			Member: in.Member,
			From:   currentPos,
			To:     in.To,
		},
		IntelDeltas: intelDeltas,
		Seq:         seqNum,
		Outcome:     firedOutcome,
	}, nil
}

// Pump advances the world by one tick: the exploration clock advances,
// each monster member (in deterministic order) acts on its own intel via Decider,
// the complete sight refresh happens once, and the story accrues tick and move beats.
// Errors from a decider abort the pump atomically (R5): no clock advance, no moves,
// no record entries.
//
// Semantics:
//   - Tick advances by exactly 1 (via clock.Advance with displacement 1).
//   - Monsters act in deterministic order (stable Members() order, filtered to KindMonster).
//   - Each decider receives exactly its own holdings (anti-wall-hack contract C2).
//   - IntentHold means do nothing; IntentMoveTo executes via managed seam.
//   - A spatial rejection of a monster's move does NOT abort the pump; the monster
//     simply fails to move. Only a decider error aborts.
//   - After all monster actions: ONE refreshSight for all members, ONE tick beat
//     (stamped with the new clock reading), then move beats in order.
//   - Ending evaluation fires ReachedPosition triggers (only if the filter matches;
//     empty filter = players only, not monsters).
//   - Returns PumpOutput with the new Tick reading, successful moves, deltas, and beats.
func (e *Encounter) Pump(in *PumpInput) (*PumpOutput, error) {
	// Validation
	if in == nil {
		return nil, fmt.Errorf("pump: %w", ErrNilInput)
	}

	if e.outcome != nil {
		return nil, fmt.Errorf("pump: %w", ErrClosed)
	}

	// PHASE 1 — decide. Every decider is consulted BEFORE anything
	// mutates: a decider error aborts here with zero state touched
	// (R5 — no clock advance, no moves, no beats). This also means a
	// later monster's decider error cannot leave an earlier monster's
	// move half-applied.
	allMembers, err := e.Members()
	if err != nil {
		return nil, fmt.Errorf("pump members: %w", err)
	}

	type plannedMove struct {
		member Member
		to     spatial.Position
	}
	var planned []plannedMove

	for _, m := range allMembers {
		if m.Kind != KindMonster {
			continue
		}
		decider, hasDecider := e.deciders[m.ID]
		if !hasDecider {
			continue // no decider = hold
		}

		// The monster's own holdings and nothing else (C2). HeldBy's
		// copy-out is intel's documented contract (pinned in play/intel)
		// — no redundant defensive copy here; the mutating-decider
		// integration test pins the composed guarantee.
		ownHoldings, err := e.intelLog.HeldBy(&intel.HeldByInput{Observer: m.ID})
		if err != nil {
			return nil, fmt.Errorf("pump held_by: %w", err)
		}

		intent, err := decider.Decide(ownHoldings)
		if err != nil {
			return nil, fmt.Errorf("pump decide: %w", err)
		}

		if mv, ok := intent.(IntentMoveTo); ok {
			planned = append(planned, plannedMove{member: m, to: mv.To})
		}
	}

	// PHASE 2 — execute. Nothing below returns a decider-shaped error;
	// the world now advances.
	_, err = e.clock.Advance(&clock.AdvanceInput{
		Driver:       core.EntityID("world"),
		Displacement: 1,
	})
	if err != nil {
		return nil, fmt.Errorf("pump advance: %w", err)
	}

	newTickReading := uint64(e.clock.ToData().HighWater)

	type monsterMove struct {
		member *Member
		from   spatial.Position
		to     spatial.Position
	}
	var successfulMoves []monsterMove

	for i := range planned {
		p := planned[i]
		// Spatial rejection does not abort the pump; the monster simply
		// fails to move and no beat is recorded for it.
		fromPos, moveErr := e.moveMember(&p.member, p.to)
		if moveErr == nil {
			successfulMoves = append(successfulMoves, monsterMove{
				member: &planned[i].member,
				from:   fromPos,
				to:     p.to,
			})
		}
	}

	// Single refreshSight for all members after all monster actions
	memberIDs := make([]MemberID, 0, len(allMembers))
	for _, m := range allMembers {
		memberIDs = append(memberIDs, m.ID)
	}
	sort.Slice(memberIDs, func(i, j int) bool { return memberIDs[i] < memberIDs[j] })

	intelDeltas, err := e.refreshSight(memberIDs)
	if err != nil {
		return nil, fmt.Errorf("pump refresh sight: %w", err)
	}

	// Record the tick beat first (the frame)
	tickBeatPayload := map[string]interface{}{
		"beat": "tick",
		"tick": newTickReading,
	}
	tickBeatBytes, _ := json.Marshal(tickBeatPayload)

	tickAppendOut, err := e.story.Append(&record.AppendInput{
		At:       newTickReading,
		Audience: memberIDs,
		Tags:     map[string]string{"tag": "clock"},
		Payload:  tickBeatBytes,
	})
	if err != nil {
		return nil, fmt.Errorf("pump append tick beat: %w", err)
	}

	seqs := []uint64{tickAppendOut.Seq}

	// Then record move beats for each successful move (in order)
	for _, move := range successfulMoves {
		moveBeatPayload := map[string]interface{}{
			"beat":     "moved",
			"member":   string(move.member.ID),
			"position": move.to,
		}
		moveBeatBytes, _ := json.Marshal(moveBeatPayload)

		moveAppendOut, err := e.story.Append(&record.AppendInput{
			At:       newTickReading,
			Audience: memberIDs,
			Tags:     map[string]string{"tag": "movement"},
			Payload:  moveBeatBytes,
		})
		if err != nil {
			return nil, fmt.Errorf("pump append move beat: %w", err)
		}

		seqs = append(seqs, moveAppendOut.Seq)
	}

	// Evaluate ReachedPosition endings
	var firedOutcome *Outcome
	for _, move := range successfulMoves {
		// Check all endings for this position
		for _, de := range e.endings {
			endingKey, trigger := de.key, de.trigger
			reachedPosTrigger, ok := trigger.(TriggerReachedPosition)
			if !ok {
				continue // Not a ReachedPosition trigger
			}

			if reachedPosTrigger.Room != move.member.Room {
				continue // Different room
			}

			if reachedPosTrigger.Position.X != move.to.X || reachedPosTrigger.Position.Y != move.to.Y {
				continue // Different position
			}

			// Check member filter: non-empty = specific member; empty = players only
			// Monsters should never trigger an unfiltered (empty-filter) ending
			if reachedPosTrigger.Member == "" {
				continue // Empty filter means players only, but this moved member is a monster
			}

			if reachedPosTrigger.Member != move.member.ID {
				continue // Member filter doesn't match
			}

			// Ending fires! Build the outcome with all members' current positions
			memberOutcomes := e.buildMemberOutcomes()

			e.outcome = &Outcome{
				Ending:  endingKey,
				At:      newTickReading,
				Members: memberOutcomes,
			}
			// Return a deep copy of the outcome (mutation-proof)
			outcomeMembers := make([]MemberOutcome, len(e.outcome.Members))
			for i, m := range e.outcome.Members {
				outcomeMembers[i] = MemberOutcome{
					ID:       m.ID,
					Room:     m.Room,
					Position: m.Position,
				}
			}
			firedOutcome = &Outcome{
				Ending:  e.outcome.Ending,
				At:      e.outcome.At,
				Members: outcomeMembers,
			}
			break
		}
		if firedOutcome != nil {
			break
		}
	}

	// Build the output with successful moves
	outputMoves := make([]struct {
		Member MemberID
		From   spatial.Position
		To     spatial.Position
	}, len(successfulMoves))
	for i, m := range successfulMoves {
		outputMoves[i].Member = m.member.ID
		outputMoves[i].From = m.from
		outputMoves[i].To = m.to
	}

	return &PumpOutput{
		Tick:         newTickReading,
		MonsterMoves: outputMoves,
		IntelDeltas:  intelDeltas,
		Seqs:         seqs,
		Outcome:      firedOutcome,
	}, nil
}

// refreshSight rebuilds the complete percept for all given observers,
// surveils each, and returns a map of member IDs to their SurveilOutput deltas.
// This is shared between Setup's first-light and Move's percept refresh.
// The current clock reading is stamped on each Surveil call.
func (e *Encounter) refreshSight(observers []MemberID) (map[MemberID]*intel.SurveilOutput, error) {
	// Get current clock reading
	clockReadingInt := e.clock.ToData().HighWater
	clockReading := uint64(clockReadingInt)
	deltas := make(map[MemberID]*intel.SurveilOutput)

	for _, observerID := range observers {
		member, ok := e.members[observerID]
		if !ok {
			continue // Skip if not found
		}

		// Get the room
		room, ok := e.orchestrator.GetRoom(member.Room)
		if !ok {
			continue // Room not found
		}

		// Get observer's position
		observerPos, ok := room.GetEntityPosition(string(observerID))
		if !ok {
			continue // Observer position not found
		}

		// List all OTHER members in the same room and check line of sight
		var percept []intel.Report
		for _, otherMember := range e.members {
			if otherMember.ID == observerID {
				continue // Skip self
			}
			if otherMember.Room != member.Room {
				continue // Different room
			}

			// Get the other member's position
			otherPos, ok := room.GetEntityPosition(string(otherMember.ID))
			if !ok {
				continue // Position not found
			}

			// Check line of sight
			if room.IsLineOfSightBlocked(observerPos, otherPos) {
				continue // Blocked by obstacle
			}

			// Add to percept
			pos := SightPayload{
				Room: otherMember.Room,
				X:    otherPos.X,
				Y:    otherPos.Y,
			}
			payload, _ := json.Marshal(pos)
			percept = append(percept, intel.Report{
				Subject: intel.Subject(otherMember.ID),
				Payload: payload,
			})
		}

		// Surveil with the complete percept and current clock reading
		out, err := e.intelLog.Surveil(&intel.SurveilInput{
			Observer: observerID,
			Channel:  intel.Sight,
			Percept:  percept,
			At:       clockReading,
		})
		if err != nil {
			return nil, fmt.Errorf("refreshsight surveil: %w", err)
		}
		deltas[observerID] = out
	}

	return deltas, nil
}

// occluderEntity is an internal entity for blocking line of sight
type occluderEntity struct {
	id string
}

// GetID returns the occluder's ID
func (o *occluderEntity) GetID() string {
	return o.id
}

// GetType returns "occluder"
func (o *occluderEntity) GetType() core.EntityType {
	return core.EntityType("occluder")
}

// GetSize returns 1 (single-cell entity)
func (o *occluderEntity) GetSize() int {
	return 1
}

// BlocksLineOfSight returns true for occluders
func (o *occluderEntity) BlocksLineOfSight() bool {
	return true
}

// BlocksMovement returns false for occluders
func (o *occluderEntity) BlocksMovement() bool {
	return false
}

// Join adds a new member to the encounter. The ambient field is always there
// to join. Validation order (R5 atomicity): nil input → closed → empty member ID →
// already a member → player-with-decider rejected → spatial placement rejection.
// On success, the joiner is placed, all members' sight is refreshed (the joiner's
// first percepts AND incumbents now seeing them), and a beat is recorded.
// ReachedPosition endings are evaluated (a player could join ON the stairs — fires YES).
func (e *Encounter) Join(in *JoinInput) (*JoinOutput, error) {
	// Validation
	if in == nil {
		return nil, fmt.Errorf("join: %w", ErrNilInput)
	}

	if e.outcome != nil {
		return nil, fmt.Errorf("join: %w", ErrClosed)
	}

	if in.Member.ID == "" {
		return nil, fmt.Errorf("join: %w", ErrNoMember)
	}

	// Check if already a member
	if _, exists := e.members[in.Member.ID]; exists {
		return nil, fmt.Errorf("join: member %s is already in the encounter: %w", in.Member.ID, ErrNoMember)
	}

	// Players cannot carry deciders (design law C2)
	if in.Member.Kind == KindPlayer && in.Member.Decider != nil {
		return nil, fmt.Errorf("join: player %s cannot carry a decider: %w", in.Member.ID, ErrNoMember)
	}

	// Place the new member via managed seam
	entity := &memberEntity{
		id:   string(in.Member.ID),
		kind: in.Member.Kind,
	}

	_, err := e.orchestrator.PlaceEntity(&spatial.PlaceEntityInput{
		RoomID:   spatial.RoomID(in.Member.Room),
		Entity:   entity,
		Position: in.Member.Position,
	})
	if err != nil {
		return nil, fmt.Errorf("join placement: %w: %w", ErrBadPlacement, err)
	}

	// Register the member
	member := &Member{
		ID:   in.Member.ID,
		Kind: in.Member.Kind,
		Room: in.Member.Room,
	}
	e.members[in.Member.ID] = member
	e.everMembers[in.Member.ID] = true // Track in everMembers

	// Store decider if present (monsters only, validated above)
	if in.Member.Decider != nil {
		e.deciders[in.Member.ID] = in.Member.Decider
	}

	// refreshSight for all members: the joiner sees incumbents, incumbents see the joiner
	memberIDs := make([]MemberID, 0, len(e.members))
	for id := range e.members {
		memberIDs = append(memberIDs, id)
	}
	sort.Slice(memberIDs, func(i, j int) bool { return memberIDs[i] < memberIDs[j] })

	intelDeltas, err := e.refreshSight(memberIDs)
	if err != nil {
		return nil, fmt.Errorf("join refresh sight: %w", err)
	}

	// Record the join beat (audience = all members including the joiner)
	clockReadingInt := e.clock.ToData().HighWater
	clockReadingForBeat := uint64(clockReadingInt)
	beatPayload := map[string]interface{}{
		"beat":   "joined",
		"member": string(in.Member.ID),
	}
	beatBytes, _ := json.Marshal(beatPayload)

	appendOut, err := e.story.Append(&record.AppendInput{
		At:       clockReadingForBeat,
		Audience: memberIDs,
		Tags:     map[string]string{"tag": "membership"},
		Payload:  beatBytes,
	})
	if err != nil {
		return nil, fmt.Errorf("join append beat: %w", err)
	}

	seqNum := appendOut.Seq

	// Evaluate ReachedPosition endings (a player could join ON the stairs)
	var firedOutcome *Outcome
	for _, de := range e.endings {
		endingKey, trigger := de.key, de.trigger
		reachedPosTrigger, ok := trigger.(TriggerReachedPosition)
		if !ok {
			continue // Not a ReachedPosition trigger
		}

		// Check if the ending fires
		if reachedPosTrigger.Room != member.Room {
			continue // Different room
		}

		if reachedPosTrigger.Position.X != in.Member.Position.X || reachedPosTrigger.Position.Y != in.Member.Position.Y {
			continue // Different position
		}

		// Check member filter: empty = any player member; non-empty = specific member
		if reachedPosTrigger.Member != "" && reachedPosTrigger.Member != in.Member.ID {
			continue // Member filter doesn't match
		}

		// Member filter passes but check kind
		if reachedPosTrigger.Member == "" && member.Kind != KindPlayer {
			continue // Empty filter means players only
		}

		// Ending fires! Build the outcome with all members' current positions
		memberOutcomes := e.buildMemberOutcomes()

		e.outcome = &Outcome{
			Ending:  endingKey,
			At:      clockReadingForBeat,
			Members: memberOutcomes,
		}
		// Return a deep copy of the outcome (mutation-proof)
		outcomeMembers := make([]MemberOutcome, len(e.outcome.Members))
		for i, m := range e.outcome.Members {
			outcomeMembers[i] = MemberOutcome{
				ID:       m.ID,
				Room:     m.Room,
				Position: m.Position,
			}
		}
		firedOutcome = &Outcome{
			Ending:  e.outcome.Ending,
			At:      e.outcome.At,
			Members: outcomeMembers,
		}
		break
	}

	return &JoinOutput{
		Member:      *member,
		IntelDeltas: intelDeltas,
		Seq:         seqNum,
		Outcome:     firedOutcome,
	}, nil
}

// Exit removes a member from the encounter. The member leaves with carry-forward:
// they are removed from the field, their final MemberOutcome is returned, and their
// intel holdings are copied out for the campaign. The encounter auto-closes with the
// reserved ending "abandoned" if the last member exits and no declared ending has fired.
// After exit, remaining members' views fade the departed naturally (their entity left,
// so next refreshSight removes them from new percepts; old holdings ghost).
func (e *Encounter) Exit(in *ExitInput) (*ExitOutput, error) {
	// Validation
	if in == nil {
		return nil, fmt.Errorf("exit: %w", ErrNilInput)
	}

	if in.Member == "" {
		return nil, fmt.Errorf("exit: %w", ErrNoMember)
	}

	if e.outcome != nil {
		return nil, fmt.Errorf("exit: %w", ErrClosed)
	}

	member, ok := e.members[in.Member]
	if !ok {
		return nil, fmt.Errorf("exit: %w", ErrNotMember)
	}

	// Get the exiting member's final position
	room, ok := e.orchestrator.GetRoom(member.Room)
	if !ok {
		return nil, fmt.Errorf("exit: %w", ErrBadPlacement)
	}

	finalPos, ok := room.GetEntityPosition(string(in.Member))
	if !ok {
		return nil, fmt.Errorf("exit: %w", ErrBadPlacement)
	}

	// Capture the exiting member's holdings (carry-forward)
	carry, err := e.intelLog.HeldBy(&intel.HeldByInput{Observer: in.Member})
	if err != nil {
		return nil, fmt.Errorf("exit held_by: %w", err)
	}

	// Remove from the field via managed seam
	_, err = e.orchestrator.RemoveEntity(&spatial.RemoveEntityInput{
		RoomID:   spatial.RoomID(member.Room),
		EntityID: core.EntityID(in.Member),
	})
	if err != nil {
		return nil, fmt.Errorf("exit remove entity: %w: %w", ErrBadPlacement, err)
	}

	// Remove from member set (and deciders if present)
	delete(e.members, in.Member)
	delete(e.deciders, in.Member)

	// Get remaining member IDs for story and refresh
	memberIDs := make([]MemberID, 0, len(e.members))
	for id := range e.members {
		memberIDs = append(memberIDs, id)
	}
	sort.Slice(memberIDs, func(i, j int) bool { return memberIDs[i] < memberIDs[j] })

	// The exit beat's audience is every member INCLUDING the exiter —
	// they witness their own departure (and can re-read it via Story).
	allMemberIDs := make([]MemberID, 0, len(e.members)+1)
	allMemberIDs = append(allMemberIDs, in.Member) // Add the exiter
	for id := range e.members {
		allMemberIDs = append(allMemberIDs, id)
	}
	sort.Slice(allMemberIDs, func(i, j int) bool { return allMemberIDs[i] < allMemberIDs[j] })

	clockReadingInt := e.clock.ToData().HighWater
	clockReadingForBeat := uint64(clockReadingInt)
	beatPayload := map[string]interface{}{
		"beat":   "exited",
		"member": string(in.Member),
	}
	beatBytes, _ := json.Marshal(beatPayload)

	appendOut, err := e.story.Append(&record.AppendInput{
		At:       clockReadingForBeat,
		Audience: allMemberIDs,
		Tags:     map[string]string{"tag": "membership"},
		Payload:  beatBytes,
	})
	if err != nil {
		return nil, fmt.Errorf("exit append beat: %w", err)
	}

	seqNum := appendOut.Seq

	// refreshSight for REMAINING members only (the exiter's holdings remain in intel archive)
	if len(memberIDs) > 0 {
		_, err := e.refreshSight(memberIDs)
		if err != nil {
			return nil, fmt.Errorf("exit refresh sight: %w", err)
		}
	}

	// Check if we need to auto-close (last member exited and no ending has fired)
	var closedOutcome *Outcome
	if len(e.members) == 0 && e.outcome == nil {
		e.outcome = &Outcome{
			Ending:  "abandoned",
			At:      clockReadingForBeat,
			Members: []MemberOutcome{}, // No members remain
		}
		closedOutcome = &Outcome{
			Ending:  "abandoned",
			At:      clockReadingForBeat,
			Members: []MemberOutcome{},
		}
	}

	return &ExitOutput{
		Outcome: MemberOutcome{
			ID:       in.Member,
			Room:     member.Room,
			Position: finalPos,
		},
		Carry:  carry,
		Seq:    seqNum,
		Closed: closedOutcome,
	}, nil
}

// End fires an externally-triggered ending. Validates the key was declared and
// has an External trigger, then closes the encounter with that Outcome.
func (e *Encounter) End(in *EndInput) (*EndOutput, error) {
	// Validation
	if in == nil {
		return nil, fmt.Errorf("end: %w", ErrNilInput)
	}

	if e.outcome != nil {
		return nil, fmt.Errorf("end: %w", ErrClosed)
	}

	if in.Ending == "" {
		return nil, fmt.Errorf("end: %w", ErrNoEnding)
	}

	// Find and validate the ending
	var foundEnding *declaredEnding
	for i := range e.endings {
		if e.endings[i].key == in.Ending {
			foundEnding = &e.endings[i]
			break
		}
	}

	if foundEnding == nil {
		return nil, fmt.Errorf("end: ending %s not declared: %w", in.Ending, ErrNoEnding)
	}

	// Verify it's an External trigger
	_, isExternal := foundEnding.trigger.(TriggerExternal)
	if !isExternal {
		return nil, fmt.Errorf("end: ending %s is not External: %w", in.Ending, ErrNoEnding)
	}

	// Build outcome with all current members' positions
	memberOutcomes := e.buildMemberOutcomes()

	clockReadingInt := e.clock.ToData().HighWater
	clockReadingForBeat := uint64(clockReadingInt)

	e.outcome = &Outcome{
		Ending:  in.Ending,
		At:      clockReadingForBeat,
		Members: memberOutcomes,
	}

	// Record the end beat
	allMemberIDs := make([]MemberID, 0, len(e.members))
	for id := range e.members {
		allMemberIDs = append(allMemberIDs, id)
	}
	sort.Slice(allMemberIDs, func(i, j int) bool { return allMemberIDs[i] < allMemberIDs[j] })

	beatPayload := map[string]interface{}{
		"beat":   "ended",
		"ending": in.Ending,
	}
	beatBytes, _ := json.Marshal(beatPayload)

	_, err := e.story.Append(&record.AppendInput{
		At:       clockReadingForBeat,
		Audience: allMemberIDs,
		Tags:     map[string]string{"tag": "scene"},
		Payload:  beatBytes,
	})
	if err != nil {
		return nil, fmt.Errorf("end append beat: %w", err)
	}

	// Return a deep copy of the outcome (mutation-proof)
	outcomeMembers := make([]MemberOutcome, len(e.outcome.Members))
	for i, m := range e.outcome.Members {
		outcomeMembers[i] = MemberOutcome{
			ID:       m.ID,
			Room:     m.Room,
			Position: m.Position,
		}
	}

	return &EndOutput{
		Outcome: Outcome{
			Ending:  e.outcome.Ending,
			At:      e.outcome.At,
			Members: outcomeMembers,
		},
	}, nil
}

// memberEntity is an internal entity for members
type memberEntity struct {
	id   string
	kind MemberKind
}

// GetID returns the member's ID
func (m *memberEntity) GetID() string {
	return m.id
}

// GetType returns the kind of member as an EntityType
func (m *memberEntity) GetType() core.EntityType {
	return core.EntityType(m.kind)
}

// GetSize returns 1 (single-cell entity)
func (m *memberEntity) GetSize() int {
	return 1
}

// BlocksLineOfSight returns false for members
func (m *memberEntity) BlocksLineOfSight() bool {
	return false
}

// BlocksMovement returns false for members
func (m *memberEntity) BlocksMovement() bool {
	return false
}
