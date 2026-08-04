package encounter

import (
	"fmt"
	"sort"
	"strings"

	"github.com/KirkDiggler/rpg-toolkit/encounter/core"
	"github.com/KirkDiggler/rpg-toolkit/tools/spatial"
)

// AuthoredEdge is one normalized, dungeon-owned physical edge authored in a
// dungeon spec. From and To are distinct adjacent semantic floor cells in the
// pointy-top absolute dungeon grid. DoorID is empty for a solid edge and is
// the stable authored-door identity for a door edge.
//
// Phase 2A persists and projects this record only. It deliberately does not
// register a spatial boundary, alter movement or line of sight, or enable
// authored-door interaction; those are the follow-on behavior phase.
type AuthoredEdge struct {
	From   core.Hex          `json:"from"`
	To     core.Hex          `json:"to"`
	Kind   GeneratedEdgeKind `json:"kind"`
	DoorID core.EntityID     `json:"door_id,omitempty"`
}

// AuthoredDoorID returns the stable identity for a door on an authored
// undirected edge. The dungeon key and lexicographically normalized absolute
// pointy-top endpoint pair are the complete identity, so reversing endpoints
// or reordering YAML entries cannot change the result.
func AuthoredDoorID(dungeonKey string, from, to core.Hex) core.EntityID {
	from, to = normalizeAuthoredEndpoints(from, to)
	return core.EntityID(fmt.Sprintf("%s-authored-door-%d-%d-%d--%d-%d-%d",
		dungeonKey, from.Q, from.R, from.S, to.Q, to.R, to.S))
}

func normalizeAuthoredEndpoints(from, to core.Hex) (core.Hex, core.Hex) {
	if generatedHexLess(to, from) {
		return to, from
	}
	return from, to
}

func normalizeAuthoredEdge(edge AuthoredEdge) AuthoredEdge {
	edge.From, edge.To = normalizeAuthoredEndpoints(edge.From, edge.To)
	return edge
}

func authoredEdgeLess(left, right AuthoredEdge) bool {
	return generatedEdgeLess(GeneratedEdge(left), GeneratedEdge(right))
}

// validateAndNormalizeAuthoredEdges is InitDungeon's defense for callers that
// bypass dungeonspec. It checks the fixed semantic floor footprint before any
// generated layout or encounter data mutates, then returns the exact canonical
// records SpaceData persists.
func validateAndNormalizeAuthoredEdges(params DungeonParams) ([]AuthoredEdge, error) {
	if len(params.AuthoredEdges) == 0 {
		return nil, nil
	}
	if strings.TrimSpace(params.Key) == "" {
		return nil, fmt.Errorf("authored edges require a dungeon key")
	}

	floor := semanticDungeonFloorHexes(params)
	out := make([]AuthoredEdge, 0, len(params.AuthoredEdges))
	seen := make(map[generatedEdgeKey]int, len(params.AuthoredEdges))
	for index, source := range params.AuthoredEdges {
		edge := normalizeAuthoredEdge(source)
		if !edge.From.ToCube().IsValid() || !edge.To.ToCube().IsValid() {
			return nil, fmt.Errorf("authored edge %d: endpoints must be valid pointy-top hex cells", index)
		}
		if edge.From == edge.To {
			return nil, fmt.Errorf("authored edge %d: endpoints must be distinct", index)
		}
		if edge.From.ToCube().Distance(edge.To.ToCube()) != 1 {
			return nil, fmt.Errorf("authored edge %d: endpoints must be adjacent", index)
		}
		if _, ok := floor[edge.From]; !ok {
			return nil, fmt.Errorf("authored edge %d: from %v is not a semantic floor cell", index, edge.From)
		}
		if _, ok := floor[edge.To]; !ok {
			return nil, fmt.Errorf("authored edge %d: to %v is not a semantic floor cell", index, edge.To)
		}
		switch edge.Kind {
		case GeneratedEdgeKindSolid:
			if edge.DoorID != "" {
				return nil, fmt.Errorf("authored edge %d: solid edges must not carry a door id", index)
			}
		case GeneratedEdgeKindDoor:
			expected := AuthoredDoorID(params.Key, edge.From, edge.To)
			if edge.DoorID != expected {
				return nil, fmt.Errorf("authored edge %d: stable authored door id must be %q, got %q", index, expected, edge.DoorID)
			}
		default:
			return nil, fmt.Errorf("authored edge %d: unknown kind %q", index, edge.Kind)
		}

		key := newGeneratedEdgeKey(edge.From, edge.To)
		if first, exists := seen[key]; exists {
			if out[first].Kind == edge.Kind && out[first].DoorID == edge.DoorID {
				return nil, fmt.Errorf("authored edge %d: duplicate of authored edge %d", index, first)
			}
			return nil, fmt.Errorf("authored edge %d: conflicts with authored edge %d", index, first)
		}
		seen[key] = len(out)
		out = append(out, edge)
	}
	sort.Slice(out, func(i, j int) bool { return authoredEdgeLess(out[i], out[j]) })
	return out, nil
}

// semanticDungeonFloorHexes produces only the declared room footprint. It
// deliberately excludes connector columns, exterior cells, generated walls,
// and obstacles: authored edge endpoints identify durable semantic floor cells,
// not a seed-dependent open-cell sample.
func semanticDungeonFloorHexes(params DungeonParams) map[core.Hex]struct{} {
	capacity := 0
	for _, region := range params.Regions {
		capacity += region.Width * params.Height
	}
	floor := make(map[core.Hex]struct{}, capacity)
	offsetX := 0
	for index, region := range params.Regions {
		for column := 0; column < region.Width; column++ {
			for row := 0; row < params.Height; row++ {
				hex := core.HexFromPosition(spatial.Position{X: float64(offsetX + column), Y: float64(row)})
				floor[hex] = struct{}{}
			}
		}
		offsetX += region.Width
		if index < len(params.Regions)-1 {
			offsetX++ // connector column: deliberately not semantic floor
		}
	}
	return floor
}

// validatePersistedAuthoredEdges checks the authored records that arrive
// through LoadFromData. They are encounter-owned persistence, not generic
// spatial persistence, so stale or hand-built snapshots fail before a room is
// reconstructed with potentially contradictory data.
func validatePersistedAuthoredEdges(space *SpaceData, doors map[core.EntityID]*DoorData) error {
	if space == nil || len(space.AuthoredEdges) == 0 {
		return nil
	}
	if strings.TrimSpace(space.DungeonKey) == "" {
		return fmt.Errorf("persisted authored edges require a dungeon key")
	}
	floor := make(map[core.Hex]struct{})
	for _, region := range space.Regions {
		for hex := range region.Hexes {
			floor[hex] = struct{}{}
		}
	}
	seen := make(map[generatedEdgeKey]int, len(space.AuthoredEdges))
	var previous AuthoredEdge
	for index, source := range space.AuthoredEdges {
		edge := normalizeAuthoredEdge(source)
		if index > 0 && authoredEdgeLess(edge, previous) {
			return fmt.Errorf("authored edge %d: persisted collection is not canonical-sorted", index)
		}
		if edge != source {
			return fmt.Errorf("authored edge %d: persisted endpoints are not normalized", index)
		}
		if edge.From == edge.To || edge.From.ToCube().Distance(edge.To.ToCube()) != 1 {
			return fmt.Errorf("authored edge %d: persisted endpoints must be distinct adjacent cells", index)
		}
		if _, ok := floor[edge.From]; !ok {
			return fmt.Errorf("authored edge %d: persisted from endpoint is not a semantic floor cell", index)
		}
		if _, ok := floor[edge.To]; !ok {
			return fmt.Errorf("authored edge %d: persisted to endpoint is not a semantic floor cell", index)
		}
		switch edge.Kind {
		case GeneratedEdgeKindSolid:
			if edge.DoorID != "" {
				return fmt.Errorf("authored edge %d: persisted solid edge carries door id", index)
			}
		case GeneratedEdgeKindDoor:
			door := doors[edge.DoorID]
			if edge.DoorID == "" || door == nil || door.ID != edge.DoorID {
				return fmt.Errorf("authored edge %d: persisted door data missing for %q", index, edge.DoorID)
			}
			if door.Position != edge.From {
				return fmt.Errorf("authored edge %d: persisted door %q does not match its normalized endpoint", index, edge.DoorID)
			}
			if edge.DoorID != AuthoredDoorID(space.DungeonKey, edge.From, edge.To) {
				return fmt.Errorf("authored edge %d: persisted door id %q is not stable for dungeon key", index, edge.DoorID)
			}
			if door.Open || door.Locked {
				return fmt.Errorf("authored edge %d: Phase 2A door %q must remain closed and unlocked", index, edge.DoorID)
			}
		default:
			return fmt.Errorf("authored edge %d: persisted unknown kind %q", index, edge.Kind)
		}
		key := newGeneratedEdgeKey(edge.From, edge.To)
		if first, exists := seen[key]; exists {
			return fmt.Errorf("authored edge %d: persisted duplicate/conflict with authored edge %d", index, first)
		}
		seen[key] = index
		previous = edge
	}
	return nil
}

func authoredDoorIDs(space *SpaceData) map[core.EntityID]struct{} {
	if space == nil || len(space.AuthoredEdges) == 0 {
		return nil
	}
	ids := make(map[core.EntityID]struct{})
	for _, edge := range space.AuthoredEdges {
		if edge.Kind == GeneratedEdgeKindDoor {
			ids[edge.DoorID] = struct{}{}
		}
	}
	return ids
}

func (e *Encounter) isAuthoredDoor(id core.EntityID) bool {
	_, ok := authoredDoorIDs(e.data.Space)[id]
	return ok
}

// validateAuthoredEdgeOverlay rejects connector collisions and leaves the
// non-connector replacement rule to DescribeEdges. The public invariant that
// authored endpoints are both semantic floor cells already rejects every
// connector-column/flanking edge before this point; connector door records are
// checked here as a second defense for future generator shapes.
func validateAuthoredEdgeOverlay(generated []generatedEdgeRecord, authored []AuthoredEdge) error {
	if len(authored) == 0 {
		return nil
	}
	byKey := make(map[generatedEdgeKey]generatedEdgeRecord, len(generated))
	for _, record := range generated {
		if record.edge.From == record.edge.To {
			continue
		}
		byKey[newGeneratedEdgeKey(record.edge.From, record.edge.To)] = record
	}
	for index, edge := range authored {
		record, collides := byKey[newGeneratedEdgeKey(edge.From, edge.To)]
		if !collides {
			continue
		}
		if record.edge.Kind == GeneratedEdgeKindDoor {
			return fmt.Errorf("authored edge %d collides with connector-derived edge %v to %v",
				index, record.edge.From, record.edge.To)
		}
	}
	return nil
}
