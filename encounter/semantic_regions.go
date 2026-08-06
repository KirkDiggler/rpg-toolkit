package encounter

import (
	"fmt"
	"sort"

	"github.com/KirkDiggler/rpg-toolkit/encounter/core"
)

// SemanticRegionParams is authored semantic scope input for a canvas dungeon.
// It does not affect structural geometry, edges, content, or traversal.
type SemanticRegionParams struct {
	ID        string
	Name      *string
	Archetype *RegionArchetype
	Cells     []core.Hex
}

// SemanticRegionData is the durable authored fact set for one semantic scope.
// Parent and innermost lookup are intentionally derived, never serialized.
type SemanticRegionData struct {
	ID        string           `json:"id"`
	Name      *string          `json:"name,omitempty"`
	Archetype *RegionArchetype `json:"archetype,omitempty"`
	Cells     core.HexSet      `json:"cells"`
}

// Zone is fog-authorized metadata for a semantic scope. It deliberately has no
// cells or membership index; callers receive only disclosed zones and ancestors.
type Zone struct {
	ID        string
	Name      *string
	Archetype *RegionArchetype
	ParentID  *string
}

// SemanticRegionAt returns the deterministic innermost declared scope at h.
// Root/unpainted cells return an empty ID and false.
func (sd *SpaceData) SemanticRegionAt(h core.Hex) (string, bool) {
	if sd == nil {
		return "", false
	}
	regions := sd.semanticRegions()
	owner := -1
	for i := range regions {
		if !regions[i].Cells.Has(h) {
			continue
		}
		if owner == -1 || len(regions[i].Cells) < len(regions[owner].Cells) ||
			(len(regions[i].Cells) == len(regions[owner].Cells) && regions[i].ID < regions[owner].ID) {
			owner = i
		}
	}
	if owner == -1 {
		return "", false
	}
	return regions[owner].ID, true
}

// SemanticRegionParent returns the smallest strict-superset parent. Empty
// scopes intentionally have root as parent.
func (sd *SpaceData) SemanticRegionParent(id string) (string, bool) {
	regions := sd.semanticRegions()
	index := semanticRegionIndex(regions, id)
	if index < 0 || len(regions[index].Cells) == 0 {
		return "", false
	}
	parent := -1
	for i := range regions {
		if i == index || len(regions[i].Cells) <= len(regions[index].Cells) ||
			!hexSetContains(regions[i].Cells, regions[index].Cells) {
			continue
		}
		if parent == -1 || len(regions[i].Cells) < len(regions[parent].Cells) ||
			(len(regions[i].Cells) == len(regions[parent].Cells) && regions[i].ID < regions[parent].ID) {
			parent = i
		}
	}
	if parent < 0 {
		return "", false
	}
	return regions[parent].ID, true
}

// ZoneAt resolves a cell's innermost scope metadata, inheriting archetype
// innermost-to-root. Root cells return an empty Zone.
func (sd *SpaceData) ZoneAt(h core.Hex) Zone {
	id, ok := sd.SemanticRegionAt(h)
	if !ok {
		return Zone{}
	}
	zone, _ := sd.zoneByID(id)
	return zone
}

// AuthorizedZones returns only scopes named by this viewer's persisted known
// observations and their ancestors, in deterministic ID order. It never
// re-resolves observation coordinates against current membership: an empty
// observed ZoneID remains root, even if that coordinate is painted later.
//
// LoadFromData validates that every non-empty observed ID resolves in this
// immutable SpaceData snapshot. The current API returns no error, so a
// hand-built invalid in-memory aggregate still omits an unresolvable ID safely
// rather than guessing from coordinates.
func (e *Encounter) AuthorizedZones(playerID core.PlayerID) []Zone {
	if e == nil || e.data == nil || e.data.Space == nil {
		return nil
	}
	regions := e.data.Space.semanticRegions()
	known := e.KnownHexes(playerID)
	needed := make(map[string]struct{})
	for _, observation := range known {
		id := observation.ZoneID
		if id == "" || semanticRegionIndex(regions, id) < 0 {
			continue
		}
		for id != "" {
			needed[id] = struct{}{}
			parent, hasParent := e.data.Space.SemanticRegionParent(id)
			if !hasParent {
				break
			}
			id = parent
		}
	}
	out := make([]Zone, 0, len(needed))
	for id := range needed {
		zone, ok := e.data.Space.zoneByID(id)
		if ok {
			out = append(out, zone)
		}
	}
	sort.Slice(out, func(i, j int) bool { return out[i].ID < out[j].ID })
	return out
}

func (sd *SpaceData) zoneByID(id string) (Zone, bool) {
	regions := sd.semanticRegions()
	index := semanticRegionIndex(regions, id)
	if index < 0 {
		return Zone{}, false
	}
	region := regions[index]
	zone := Zone{ID: region.ID, Name: cloneString(region.Name)}
	if region.Archetype != nil {
		value := *region.Archetype
		zone.Archetype = &value
	}
	if parent, ok := sd.SemanticRegionParent(id); ok {
		zone.ParentID = &parent
	}
	for current := index; current >= 0 && zone.Archetype == nil; {
		parent, ok := sd.SemanticRegionParent(regions[current].ID)
		if !ok {
			break
		}
		current = semanticRegionIndex(regions, parent)
		if current >= 0 && regions[current].Archetype != nil {
			value := *regions[current].Archetype
			zone.Archetype = &value
		}
	}
	return zone, true
}

func (sd *SpaceData) semanticRegions() []SemanticRegionData {
	if sd == nil {
		return nil
	}
	return sd.SemanticRegions
}

func semanticRegionIndex(regions []SemanticRegionData, id string) int {
	for i := range regions {
		if regions[i].ID == id {
			return i
		}
	}
	return -1
}

func hexSetContains(outer, inner core.HexSet) bool {
	for h := range inner {
		if !outer.Has(h) {
			return false
		}
	}
	return true
}

func cloneString(value *string) *string {
	if value == nil {
		return nil
	}
	clone := *value
	return &clone
}

func validateSemanticRegionParams(
	regions []SemanticRegionParams, floor map[core.Hex]struct{},
) ([]SemanticRegionData, error) {
	out := make([]SemanticRegionData, len(regions))
	seen := make(map[string]int, len(regions))
	for i, region := range regions {
		if region.ID == "" {
			return nil, fmt.Errorf("semantic region %d: id required", i)
		}
		if first, exists := seen[region.ID]; exists {
			return nil, fmt.Errorf("semantic region %d (%q): duplicate id (already used by %d)", i, region.ID, first)
		}
		seen[region.ID] = i
		if region.Archetype != nil {
			switch *region.Archetype {
			case ArchetypeEntrance, ArchetypeChamber, ArchetypeCorridor, ArchetypeBoss:
			default:
				return nil, fmt.Errorf("semantic region %q: unsupported archetype %q", region.ID, *region.Archetype)
			}
		}
		cells := make(core.HexSet, len(region.Cells))
		for _, h := range region.Cells {
			if _, ok := floor[h]; !ok {
				return nil, fmt.Errorf("semantic region %q cell %v is outside structural floor", region.ID, h)
			}
			cells[h] = struct{}{}
		}
		out[i] = SemanticRegionData{ID: region.ID, Name: cloneString(region.Name), Archetype: region.Archetype, Cells: cells}
	}
	for i := range out {
		for j := i + 1; j < len(out); j++ {
			if len(out[i].Cells) == 0 || len(out[j].Cells) == 0 {
				continue
			}
			intersects := false
			for h := range out[i].Cells {
				if out[j].Cells.Has(h) {
					intersects = true
					break
				}
			}
			if !intersects {
				continue
			}
			leftInside := len(out[i].Cells) < len(out[j].Cells) && hexSetContains(out[j].Cells, out[i].Cells)
			rightInside := len(out[j].Cells) < len(out[i].Cells) && hexSetContains(out[i].Cells, out[j].Cells)
			if !leftInside && !rightInside {
				return nil, fmt.Errorf("semantic regions %q and %q have equal or partial overlap", out[i].ID, out[j].ID)
			}
		}
	}
	return out, nil
}

// validateObservedZoneIDs keeps persisted viewer memory and immutable canvas
// scope facts in one snapshot-consistency domain. It intentionally applies only
// to canvas semantic regions: legacy room-chain observations use RegionData IDs.
func validateObservedZoneIDs(players map[core.PlayerID]*PlayerData, space *SpaceData) error {
	regions := space.semanticRegions()
	for playerID, player := range players {
		if player == nil || player.View == nil {
			continue
		}
		for hex, observation := range player.View.Memory {
			if observation.ZoneID == "" {
				continue
			}
			if semanticRegionIndex(regions, observation.ZoneID) < 0 {
				return fmt.Errorf(
					"player %q known hex %v references missing semantic zone %q",
					playerID, hex, observation.ZoneID,
				)
			}
		}
	}
	return nil
}

func validateSemanticRegionData(regions []SemanticRegionData, floor map[core.Hex]struct{}) error {
	params := make([]SemanticRegionParams, len(regions))
	for i, region := range regions {
		params[i] = SemanticRegionParams{ID: region.ID, Name: region.Name, Archetype: region.Archetype}
		for h := range region.Cells {
			params[i].Cells = append(params[i].Cells, h)
		}
	}
	_, err := validateSemanticRegionParams(params, floor)
	return err
}
