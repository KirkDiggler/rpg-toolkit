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

	// Check member IDs: empty or duplicate
	seenIDs := make(map[MemberID]bool)
	for _, m := range in.Members {
		if m.ID == "" {
			return nil, fmt.Errorf("newencounter: %w", ErrNoMember)
		}
		if seenIDs[m.ID] {
			return nil, fmt.Errorf("newencounter: %w", ErrNoMember)
		}
		seenIDs[m.ID] = true
	}

	// After validation passes, construct (R5: no observable state until success)
	e := &Encounter{
		members: make(map[MemberID]*Member),
		endings: make(map[string]Trigger),
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
	}

	// Store endings
	for _, ei := range in.Endings {
		e.endings[ei.Key] = ei.Trigger
	}

	// First light: build sight percepts for each member
	for _, memberID := range memberIDs {
		memberData := in.Members[findMemberIndex(in.Members, memberID)]
		room := roomMap[memberData.Room]

		// Get all other members in the same room
		var percept []intel.Report
		for _, other := range in.Members {
			if other.ID == memberID {
				continue // Skip self
			}
			if other.Room != memberData.Room {
				continue // Different room
			}

			// Check line of sight
			if room.IsLineOfSightBlocked(memberData.Position, other.Position) {
				continue // Blocked by obstacle
			}

			// Add to percept
			pos := SightPayload{
				Room: other.Room,
				X:    other.Position.X,
				Y:    other.Position.Y,
			}
			payload, _ := json.Marshal(pos)
			percept = append(percept, intel.Report{
				Subject: intel.Subject(other.ID),
				Payload: payload,
			})
		}

		// Surveil with all visible members
		_, err = e.intelLog.Surveil(&intel.SurveilInput{
			Observer: memberID,
			Channel:  intel.Sight,
			Percept:  percept,
			At:       0,
		})
		if err != nil {
			return nil, fmt.Errorf("newencounter surveil: %w", err)
		}
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
// Returns a deep copy of the outcome to prevent aliasing.
func (e *Encounter) Status() (*Status, error) {
	if e.outcome != nil {
		// Deep-copy outcome and its Members slice
		members := make([]MemberOutcome, len(e.outcome.Members))
		copy(members, e.outcome.Members)
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

// Helper to find a member in the slice by ID
func findMemberIndex(members []MemberInput, id MemberID) int {
	for i, m := range members {
		if m.ID == id {
			return i
		}
	}
	return -1
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
