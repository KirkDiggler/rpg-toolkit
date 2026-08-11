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
type Encounter struct {
	orchestrator *spatial.BasicRoomOrchestrator
	clock        *clock.Tick
	intelLog     *intel.Intel
	story        *record.Log
	members      map[MemberID]*Member
	deciders     map[MemberID]Decider
	endings      map[string]Trigger
	outcome      *Outcome
}

// NewEncounter constructs and initializes an encounter from SetupInput.
// Validation order (first failure wins, R5 atomicity): nil input, no rooms,
// no endings, reserved ending key, empty member ID, duplicate member IDs,
// spatial placement errors.
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
			return nil, fmt.Errorf("newencounter: %w", ErrNoMember)
		}
		seenIDs[m.ID] = true

		// Players cannot carry deciders (design law C2)
		if m.Kind == KindPlayer && m.Decider != nil {
			return nil, fmt.Errorf("newencounter: player %s cannot carry a decider: %w", m.ID, ErrNilInput)
		}
	}

	// After validation passes, construct (R5: no observable state until success)
	e := &Encounter{
		members:  make(map[MemberID]*Member),
		deciders: make(map[MemberID]Decider),
		endings:  make(map[string]Trigger),
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

		// Store decider if present (monsters only, validated above)
		if mi.Decider != nil {
			e.deciders[mi.ID] = mi.Decider
		}
	}

	// Store endings
	for _, ei := range in.Endings {
		e.endings[ei.Key] = ei.Trigger
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
// Returns ErrNilInput if the input is nil, ErrNoMember if the member is not in
// the encounter. Copy-out follows record's own conventions (returned entries are
// already copies per record's implementation).
func (e *Encounter) Story(in *StoryInput) ([]record.Entry, error) {
	if in == nil {
		return nil, fmt.Errorf("story: %w", ErrNilInput)
	}

	if _, ok := e.members[in.Audience]; !ok {
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

// Move executes a continuous player movement within the same room.
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
	for endingKey, trigger := range e.endings {
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
		memberOutcomes := make([]MemberOutcome, 0, len(e.members))
		for _, m := range e.members {
			mRoom, ok := e.orchestrator.GetRoom(m.Room)
			if !ok {
				continue
			}
			mPos, ok := mRoom.GetEntityPosition(string(m.ID))
			if !ok {
				continue
			}
			memberOutcomes = append(memberOutcomes, MemberOutcome{
				ID:       m.ID,
				Room:     m.Room,
				Position: mPos,
			})
		}

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

	// Advance the exploration clock by 1 tick (the driver is the "world" itself)
	_, err := e.clock.Advance(&clock.AdvanceInput{
		Driver:       core.EntityID("world"),
		Displacement: 1,
	})
	if err != nil {
		return nil, fmt.Errorf("pump advance: %w", err)
	}

	newTickReading := uint64(e.clock.ToData().HighWater)

	// Collect all members in deterministic order and filter to monsters
	allMembers, err := e.Members()
	if err != nil {
		return nil, fmt.Errorf("pump members: %w", err)
	}

	var monsterMembers []Member
	for _, m := range allMembers {
		if m.Kind == KindMonster {
			monsterMembers = append(monsterMembers, m)
		}
	}

	// Track successful monster moves and any decider error
	type monsterMove struct {
		member *Member
		from   spatial.Position
		to     spatial.Position
	}
	var successfulMoves []monsterMove

	// Execute each monster's decision
	for _, monsterMember := range monsterMembers {
		m := monsterMember // Copy for pointer
		decider, hasDecider := e.deciders[m.ID]

		// No decider = hold (do nothing)
		if !hasDecider {
			continue
		}

		// Get the monster's own holdings (anti-wall-hack contract)
		ownHoldings, err := e.intelLog.HeldBy(&intel.HeldByInput{Observer: m.ID})
		if err != nil {
			return nil, fmt.Errorf("pump held_by: %w", err)
		}

		// Copy the holdings slice so the decider cannot mutate encounter state
		holdingsCopy := make([]intel.Holding, len(ownHoldings))
		copy(holdingsCopy, ownHoldings)

		// Ask the decider what to do
		intent, err := decider.Decide(holdingsCopy)
		if err != nil {
			return nil, fmt.Errorf("pump decide: %w", err)
		}

		// Execute the intent
		switch i := intent.(type) {
		case IntentHold:
			// Do nothing
		case IntentMoveTo:
			// Try to move; spatial rejection does not abort (just skip the move)
			fromPos, moveErr := e.moveMember(&m, i.To)
			if moveErr == nil {
				// Move succeeded; record it
				successfulMoves = append(successfulMoves, monsterMove{
					member: &m,
					from:   fromPos,
					to:     i.To,
				})
			}
			// If moveErr != nil, the monster simply fails to move; we continue
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
		for endingKey, trigger := range e.endings {
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
			memberOutcomes := make([]MemberOutcome, 0, len(e.members))
			for _, m := range e.members {
				mRoom, ok := e.orchestrator.GetRoom(m.Room)
				if !ok {
					continue
				}
				mPos, ok := mRoom.GetEntityPosition(string(m.ID))
				if !ok {
					continue
				}
				memberOutcomes = append(memberOutcomes, MemberOutcome{
					ID:       m.ID,
					Room:     m.Room,
					Position: mPos,
				})
			}

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
