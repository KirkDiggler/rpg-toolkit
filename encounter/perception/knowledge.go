package perception

import (
	"encoding/json"
	"sort"

	"github.com/KirkDiggler/rpg-toolkit/encounter/core"
)

// rpg-toolkit#851: this file is the toolkit-owned implementation of the
// per-viewer knowledge seam rpg-api specified against a toolkit gap —
// see rpg-api's internal/handlers/dnd5e/v2/encounter/knowledge.go, written
// as a SPECIFICATION before this existed. The types and semantics here
// mirror that spec exactly (rpg-api will drop its local copies and consume
// these once it bumps its toolkit dependency); naming is toolkit-idiomatic,
// meaning is unchanged.
//
// Contract: rpg-project/ideas/fog-of-war/design.md §"The event layer".
// The hex is the unit of truth, a VISIBLE observation is total, and nothing
// is ever deleted. This file does NOT define any wire/event shape — that is
// rpg-api-protos' job. It defines what a viewer KNOWS; translating that into
// HexKnowledgeChanged envelopes is the API's.

// KnowledgeState is what a viewer knows about one hex right now.
//
// There is no Unseen value, deliberately. A hex the viewer has never
// observed is ABSENT from View.Memory — omission is the third state. A
// value would be a second way to say the same thing, and the two could
// disagree.
//
// There is likewise no Gone/Removed value. See HexObservation.
type KnowledgeState int

const (
	// KnowledgeStateUnspecified means the producer failed to set a state. It
	// is a defect to surface, never a synonym for unseen.
	KnowledgeStateUnspecified KnowledgeState = iota
	// KnowledgeStateVisible is current authorized truth: the viewer observes
	// this hex right now.
	KnowledgeStateVisible
	// KnowledgeStateRemembered is a frozen last observation. What the viewer
	// saw, not what is there now.
	KnowledgeStateRemembered
)

// TerrainKind is reserved for a future terrain source. Nothing in the
// toolkit determines hex terrain today — no generator, no room builder,
// nothing persisted on SpaceData names a per-hex terrain type — so this
// stays TerrainKindUnspecified on every HexObservation this package
// produces. It is named here, not omitted, because the field belongs on the
// contract (a remembered hex must carry terrain AS OBSERVED, not as
// currently true, whenever a source exists) and adding it later must not be
// a JSON migration. Saying "no source" plainly here, rather than deriving a
// placeholder from something adjacent (e.g. RegionData.Archetype), is
// deliberate: a guessed value would be indistinguishable from a real one to
// every consumer.
type TerrainKind int

const (
	// TerrainKindUnspecified means no terrain source exists — the value
	// every observation this package produces carries today. See
	// TerrainKind's own doc for why that's stated plainly rather than
	// derived from something adjacent.
	TerrainKindUnspecified TerrainKind = iota
	// TerrainKindFloor is plain walkable ground. Unused until a terrain
	// source exists — see TerrainKindUnspecified.
	TerrainKindFloor
	// TerrainKindRough costs extra movement but is not difficult terrain.
	// Unused until a terrain source exists — see TerrainKindUnspecified.
	TerrainKindRough
	// TerrainKindDifficult is D&D 5e difficult terrain (double movement
	// cost). Unused until a terrain source exists — see
	// TerrainKindUnspecified.
	TerrainKindDifficult
	// TerrainKindVoid is a pit, chasm, or other non-floor gap. Unused
	// until a terrain source exists — see TerrainKindUnspecified.
	TerrainKindVoid
	// TerrainKindWater is a water hazard. Unused until a terrain source
	// exists — see TerrainKindUnspecified.
	TerrainKindWater
)

// Placement is an occupant of a hex, as observed.
//
// Facing rides the placement rather than the entity because an observation
// belongs to one viewer: someone who saw a skeleton facing north and lost
// sight of it must keep facing north after the skeleton turns and another
// viewer sees it face south. On the entity, one viewer's sighting would
// rewrite every other viewer's memory.
//
// Facing is absent for players, monsters, and rolled obstacles. An authored
// floor prop may carry a canonical E,NE,NW,W,SW,SE = 0..5 override, whether
// placed room-locally or at an absolute canvas coordinate. Pointer presence
// distinguishes absent from explicit E = 0 and persists with this observation
// for remembered placement rendering.
type Placement struct {
	EntityID core.EntityID
	// Facing is an optional hex-direction index in canonical 0-5 order.
	Facing *uint32
	// Offset is optional presentation-only [x,y,z] metadata in canonical
	// game-world axes, frozen with this viewer's observation.
	Offset *core.PlacementOffset
}

type placementWire struct {
	EntityID core.EntityID         `json:"EntityID"`
	Facing   *uint32               `json:"facing,omitempty"`
	Offset   *core.PlacementOffset `json:"offset,omitempty"`
}

// MarshalJSON writes optional facing under a lowercase key so presence is
// representable without colliding with legacy Placement JSON. Before facing
// became optional, every placement serialized an uppercase "Facing":0 even
// though no authored override existed.
func (p Placement) MarshalJSON() ([]byte, error) {
	return json.Marshal(placementWire(p))
}

// UnmarshalJSON restores a current optional facing field and deliberately
// ignores legacy uppercase "Facing":0. That historical zero was emitted for
// every placement by the old non-optional field, so treating it as explicit E
// would fabricate authored orientation for persisted viewer memory.
func (p *Placement) UnmarshalJSON(data []byte) error {
	var fields map[string]json.RawMessage
	if err := json.Unmarshal(data, &fields); err != nil {
		return err
	}

	var entityID core.EntityID
	if rawEntityID, ok := fields["EntityID"]; ok {
		if err := json.Unmarshal(rawEntityID, &entityID); err != nil {
			return err
		}
	}
	var facing *uint32
	if rawFacing, ok := fields["facing"]; ok {
		if err := json.Unmarshal(rawFacing, &facing); err != nil {
			return err
		}
	}
	var offset *core.PlacementOffset
	if rawOffset, ok := fields["offset"]; ok {
		if err := json.Unmarshal(rawOffset, &offset); err != nil {
			return err
		}
	}
	p.EntityID = entityID
	p.Facing = facing
	p.Offset = offset
	return nil
}

// Edge is a wall or door on one hex's boundary, AS OBSERVED by this viewer.
//
// Edges hang off the hex rather than living in one global wall list because
// a remembered hex has to be drawable from its own record — the same
// closed leak rpg-api's Space.walls/Space.doors used to have (every
// viewer's projection carried every wall/door regardless of what that
// viewer had actually seen).
//
// The fields are toolkit primitives rather than a wire enum: the toolkit
// owns what a barrier IS; a consumer maps this onto its own wire
// representation at its boundary.
type Edge struct {
	From core.Hex
	To   core.Hex

	BlocksMovement bool
	BlocksLoS      bool

	// DoorID is the door's entity id, empty when this edge is not a door.
	DoorID string
	// DoorOpen and DoorLocked are the door's state AS OBSERVED. A viewer who
	// watched a door close and then walked away must keep "closed" even
	// after someone else opens it, so these travel with the observation
	// rather than being read from the live door.
	DoorOpen   bool
	DoorLocked bool
}

// HexObservation is one viewer's complete authorized truth for one hex, at
// the moment it was observed.
//
// A Visible observation is TOTAL: an empty Contents is a positive claim
// that the hex is empty, never "contents not computed". That totality is
// what lets a remembered occupant vanish on re-sight with no forget
// message — the arriving observation simply does not list it. A producer
// that leaves Contents empty because it did not bother to look is stating
// something false, and the viewer will believe it.
//
// A Remembered observation carries its frozen state IN FULL — terrain,
// edges and contents as last seen — rather than expecting the consumer to
// freeze what it already holds. That is what makes reconnect hydration and
// a live diff the same operation instead of two paths that can drift
// apart.
//
// NOTHING IS EVER DELETED. There is no removal observation and no
// tombstone. A witnessed removal is a Visible observation that no longer
// lists the thing; a hidden removal is no observation at all, leaving
// memory stale on purpose. Deletion is the one operation a later
// observation cannot correct.
type HexObservation struct {
	Position core.Hex
	State    KnowledgeState
	Terrain  TerrainKind
	ZoneID   string
	Edges    []Edge
	Contents []Placement
}

// hexObservationWire is the JSON wire shape for one Memory entry — Position
// promoted to a top-level key so Memory round-trips as a slice (see
// Memory's doc for why a map keyed by core.Hex cannot marshal directly).
type hexObservationWire struct {
	Position core.Hex       `json:"position"`
	State    KnowledgeState `json:"state"`
	Terrain  TerrainKind    `json:"terrain,omitempty"`
	ZoneID   string         `json:"zone_id,omitempty"`
	Edges    []Edge         `json:"edges,omitempty"`
	Contents []Placement    `json:"contents,omitempty"`
}

// Memory is a viewer's complete personal knowledge: every hex ever
// observed, keyed by position, holding the last AUTHORIZED observation.
// Whether that observation is currently visible or remembered is a
// property of the observation's own State field, not of Memory's shape —
// Memory holds every known hex regardless of current visibility. A hex
// never observed is simply absent (see HexObservation's doc; there is no
// Unseen value).
//
// JSON: Memory marshals as a stable-ordered slice of {position + the rest
// of the observation}, mirroring core.HexSet's pattern one level up — Go's
// encoding/json cannot encode struct keys in a map directly, and without
// custom marshaling Memory would silently round-trip as an empty object.
type Memory map[core.Hex]HexObservation

// NewMemory builds an empty Memory.
func NewMemory() Memory { return make(Memory) }

// Observe replaces the observation at obs.Position wholesale. Never a
// field-level merge — merging would resurrect contents or edges the new
// observation deliberately omits (see HexObservation's "Visible is TOTAL"
// doc). Applying the identical observation twice leaves Memory identical
// (idempotent): this is a plain map assignment, and HexObservation carries
// no hidden non-deterministic state as long as the caller's Edges/Contents
// are already in a stable order — see the encounter package's
// refreshObservations, the caller responsible for that ordering.
func (m Memory) Observe(obs HexObservation) {
	m[obs.Position] = obs
}

// Knows reports whether h has ever been observed — the third-state-by-
// omission check HexObservation's doc describes. Safe on a nil Memory.
func (m Memory) Knows(h core.Hex) bool {
	_, ok := m[h]
	return ok
}

// MarshalJSON encodes Memory as a slice sorted by (Q,R,S) for stable,
// deterministic wire/storage output — mirrors core.HexSet.MarshalJSON.
func (m Memory) MarshalJSON() ([]byte, error) {
	out := make([]hexObservationWire, 0, len(m))
	for _, obs := range m {
		out = append(out, hexObservationWire(obs))
	}
	sort.Slice(out, func(i, j int) bool {
		a, b := out[i].Position, out[j].Position
		if a.Q != b.Q {
			return a.Q < b.Q
		}
		if a.R != b.R {
			return a.R < b.R
		}
		return a.S < b.S
	})
	return json.Marshal(out)
}

// UnmarshalJSON decodes a slice back into a Memory. Accepts JSON null and
// the empty array as the empty map — mirrors core.HexSet.UnmarshalJSON.
func (m *Memory) UnmarshalJSON(b []byte) error {
	if string(b) == "null" {
		*m = make(Memory)
		return nil
	}
	var slice []hexObservationWire
	if err := json.Unmarshal(b, &slice); err != nil {
		return err
	}
	out := make(Memory, len(slice))
	for _, w := range slice {
		out[w.Position] = HexObservation(w)
	}
	*m = out
	return nil
}
