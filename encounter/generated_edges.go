package encounter

import (
	"fmt"
	"sort"

	"github.com/KirkDiggler/rpg-toolkit/encounter/core"
)

// GeneratedEdgeKind identifies the runtime barrier represented by a
// GeneratedEdge.
type GeneratedEdgeKind string

const (
	// GeneratedEdgeKindSolid is a generated, non-door barrier. Solid edges
	// never carry an entity identity.
	GeneratedEdgeKindSolid GeneratedEdgeKind = "solid"
	// GeneratedEdgeKindDoor is a connector door barrier. DoorID identifies
	// the existing DoorData record that owns its lifecycle.
	GeneratedEdgeKindDoor GeneratedEdgeKind = "door"
)

// GeneratedEdge is one canonical physical edge in an initialized encounter's
// current generated geometry. From and To are always distinct and retain the
// runtime canonical orientation: solid edges use SpaceData.Walls' Start/End
// and doors use their threshold cell plus the designated passage neighbor.
// DoorID is populated only when Kind is GeneratedEdgeKindDoor.
type GeneratedEdge struct {
	From   core.Hex
	To     core.Hex
	Kind   GeneratedEdgeKind
	DoorID core.EntityID
}

// DescribeGeneratedEdgesInput reserves an explicit input boundary for the
// generated-edge describe operation. The current encounter already owns the
// initialized geometry, so no regeneration or caller-provided geometry is
// accepted.
type DescribeGeneratedEdgesInput struct{}

// DescribeGeneratedEdgesOutput contains one record for every undirected
// physical generated barrier edge in the encounter's current geometry.
type DescribeGeneratedEdgesOutput struct {
	Edges []GeneratedEdge
}

// DescribeGeneratedEdges exposes the canonical generated wall and connector
// door geometry for an already-initialized encounter. It deduplicates
// equivalent reversed solid records, preserves their canonical source
// endpoints, and rejects conflicting records for the same physical edge.
//
// This is an authoring-safe read seam: callers project its result directly and
// must not inspect SpaceData.Walls or Doors to derive competing geometry.
func (e *Encounter) DescribeGeneratedEdges(_ DescribeGeneratedEdgesInput) (DescribeGeneratedEdgesOutput, error) {
	records, err := e.canonicalGeneratedEdgeRecords()
	if err != nil {
		return DescribeGeneratedEdgesOutput{}, err
	}
	output := DescribeGeneratedEdgesOutput{Edges: make([]GeneratedEdge, 0, len(records))}
	for _, record := range records {
		// Degenerate source records are cell blockers for viewer knowledge,
		// not physical edges for authoring/proto projection.
		if record.edge.From == record.edge.To {
			continue
		}
		output.Edges = append(output.Edges, record.edge)
	}
	return output, nil
}

// DescribeEdgesInput reserves an explicit input boundary for the combined
// generated-plus-authored edge projection. Geometry is already initialized on
// the encounter; callers cannot provide a competing edge collection here.
type DescribeEdgesInput struct{}

// DescribeEdgesOutput contains the sorted effective physical edge projection.
// It intentionally reuses GeneratedEdge's established wire-neutral endpoint,
// kind, and DoorID fields so existing API consumers need no parallel model.
type DescribeEdgesOutput struct {
	Edges []GeneratedEdge
}

// DescribeEdges exposes the effective generated-plus-authored canonical edge
// projection. An authored edge replaces a colliding generated non-connector
// physical edge; connector-derived collisions are rejected during InitDungeon
// and defensively here. DescribeGeneratedEdges remains generated-only for
// source compatibility.
func (e *Encounter) DescribeEdges(_ DescribeEdgesInput) (DescribeEdgesOutput, error) {
	records, err := e.canonicalEffectiveEdgeRecords()
	if err != nil {
		return DescribeEdgesOutput{}, err
	}
	output := DescribeEdgesOutput{Edges: make([]GeneratedEdge, 0, len(records))}
	for _, record := range records {
		if record.edge.From == record.edge.To {
			continue
		}
		output.Edges = append(output.Edges, record.edge)
	}
	return output, nil
}

type generatedEdgeRecord struct {
	edge                 GeneratedEdge
	blocksMovement       bool
	blocksLoS            bool
	observeBothEndpoints bool
}

type generatedEdgeKey struct {
	first  core.Hex
	second core.Hex
}

// canonicalEffectiveEdgeRecords is the one effective dungeon-edge source for
// runtime DescribeEdges and viewer observations. It overlays authored edges
// onto generated physical edges, preserving generated cell-blocker records for
// knowledge while giving authored edges one canonical normalized identity and
// live door-state blocking flags.
func (e *Encounter) canonicalEffectiveEdgeRecords() ([]generatedEdgeRecord, error) {
	if err := validatePersistedAuthoredEdges(e.data.Space, e.data.Doors); err != nil {
		return nil, fmt.Errorf("validate authored edges: %w", err)
	}
	generated, err := e.canonicalGeneratedEdgeRecords()
	if err != nil {
		return nil, err
	}

	var authored []AuthoredEdge
	if e.data.Space != nil {
		authored = e.data.Space.AuthoredEdges
	}
	authoredByKey := authoredEdgesByKey(e.data.Space)
	records := make([]generatedEdgeRecord, 0, len(generated)+len(authored))
	for _, record := range generated {
		if record.edge.From != record.edge.To {
			if _, replaces := authoredByKey[newGeneratedEdgeKey(record.edge.From, record.edge.To)]; replaces {
				if record.edge.Kind == GeneratedEdgeKindDoor {
					return nil, fmt.Errorf("authored edge collides with connector-derived edge %v to %v",
						record.edge.From, record.edge.To)
				}
				continue
			}
			record.edge.From, record.edge.To = normalizeAuthoredEndpoints(record.edge.From, record.edge.To)
		}
		records = append(records, record)
	}
	for _, edge := range authored {
		record := generatedEdgeRecord{
			edge:                 GeneratedEdge(edge),
			observeBothEndpoints: true,
		}
		if edge.Kind == GeneratedEdgeKindDoor {
			door := e.data.Doors[edge.DoorID]
			record.blocksMovement = !door.Open
			record.blocksLoS = !door.Open
		} else {
			record.blocksMovement = true
			record.blocksLoS = true
		}
		records = append(records, record)
	}
	sort.Slice(records, func(i, j int) bool {
		return generatedEdgeLess(records[i].edge, records[j].edge)
	})
	return records, nil
}

// canonicalGeneratedEdgeRecords is the generated-only source for
// DescribeGeneratedEdges. It retains every degenerate cell-blocker record for
// later effective knowledge projection. Only distinct-endpoint physical edges
// receive undirected deduplication and conflict checks. GeneratedEdge retains
// the source's runtime orientation, while the private record retains blocking
// semantics the effective source must preserve.
func (e *Encounter) canonicalGeneratedEdgeRecords() ([]generatedEdgeRecord, error) {
	records := make([]generatedEdgeRecord, 0)
	seen := make(map[generatedEdgeKey]int)
	add := func(record generatedEdgeRecord) error {
		// Start==End is a valid cell-blocker/self-loop representation. It is
		// not a physical edge, so a co-located wall and door must both reach
		// viewer memory rather than being deduplicated or treated as a conflict.
		if record.edge.From == record.edge.To {
			records = append(records, record)
			return nil
		}

		key := newGeneratedEdgeKey(record.edge.From, record.edge.To)
		if index, exists := seen[key]; exists {
			existing := records[index]
			if existing.edge.Kind == record.edge.Kind &&
				existing.edge.DoorID == record.edge.DoorID &&
				existing.blocksMovement == record.blocksMovement &&
				existing.blocksLoS == record.blocksLoS {
				return nil
			}
			return fmt.Errorf("conflicting generated edges at %v to %v: %s/%q conflicts with %s/%q",
				record.edge.From, record.edge.To, existing.edge.Kind, existing.edge.DoorID,
				record.edge.Kind, record.edge.DoorID)
		}
		seen[key] = len(records)
		records = append(records, record)
		return nil
	}

	if e.data.Space != nil {
		for _, wall := range e.data.Space.Walls {
			if !wall.BlocksMovement && !wall.BlocksLoS {
				continue
			}
			if err := add(generatedEdgeRecord{
				edge: GeneratedEdge{
					From: core.HexFromCube(wall.Start),
					To:   core.HexFromCube(wall.End),
					Kind: GeneratedEdgeKindSolid,
				},
				blocksMovement: wall.BlocksMovement,
				blocksLoS:      wall.BlocksLoS,
			}); err != nil {
				return nil, err
			}
		}
	}

	authoredDoors := authoredDoorIDs(e.data.Space)
	doorIDs := make([]core.EntityID, 0, len(e.data.Doors))
	for id, door := range e.data.Doors {
		if door != nil {
			if _, authored := authoredDoors[id]; authored {
				// Authored doors are edge-native effective records below, never
				// generated connector cell geometry.
				continue
			}
			doorIDs = append(doorIDs, id)
		}
	}
	sort.Slice(doorIDs, func(i, j int) bool { return doorIDs[i] < doorIDs[j] })
	for _, id := range doorIDs {
		door := e.data.Doors[id]
		if err := add(generatedEdgeRecord{
			edge: GeneratedEdge{
				From:   door.Position,
				To:     e.doorPassageNeighbor(door),
				Kind:   GeneratedEdgeKindDoor,
				DoorID: id,
			},
			blocksMovement: !door.Open,
			blocksLoS:      !door.Open,
		}); err != nil {
			return nil, err
		}
	}

	sort.Slice(records, func(i, j int) bool {
		return generatedEdgeLess(records[i].edge, records[j].edge)
	})
	return records, nil
}

// newGeneratedEdgeKey gives a distinct-endpoint undirected physical edge a
// comparable key without changing the endpoint orientation exposed to callers.
func newGeneratedEdgeKey(from, to core.Hex) generatedEdgeKey {
	if generatedHexLess(to, from) {
		return generatedEdgeKey{first: to, second: from}
	}
	return generatedEdgeKey{first: from, second: to}
}

func generatedEdgeLess(left, right GeneratedEdge) bool {
	if generatedHexLess(left.From, right.From) {
		return true
	}
	if generatedHexLess(right.From, left.From) {
		return false
	}
	if generatedHexLess(left.To, right.To) {
		return true
	}
	if generatedHexLess(right.To, left.To) {
		return false
	}
	if left.Kind != right.Kind {
		return left.Kind < right.Kind
	}
	return left.DoorID < right.DoorID
}

func generatedHexLess(left, right core.Hex) bool {
	if left.Q != right.Q {
		return left.Q < right.Q
	}
	if left.R != right.R {
		return left.R < right.R
	}
	return left.S < right.S
}
