package encounter

import (
	"fmt"
	"math"
	"sort"

	"github.com/KirkDiggler/rpg-toolkit/tools/spatial"
)

// SightArea is a runtime, sight-only obscuring volume. SourceID is opaque to
// encounter and lets its owner remove the area when the effect ends.
type SightArea struct {
	ID         string
	SourceID   string
	Name       string
	Ref        string
	Center     spatial.Position
	RadiusFeet int
}

type SightAreaData struct {
	ID         string
	SourceID   string
	Name       string
	Ref        string
	Center     PositionData
	RadiusFeet int
}
type SightAreaInput struct {
	ID         string
	SourceID   string
	Name       string
	Ref        string
	Center     spatial.Position
	RadiusFeet int
}

func (in *SightAreaInput) area() (SightArea, error) {
	if in == nil {
		return SightArea{}, fmt.Errorf("sight area: %w", ErrNilInput)
	}
	if in.ID == "" || in.SourceID == "" || in.RadiusFeet <= 0 || !finite(in.Center.X) || !finite(in.Center.Y) {
		return SightArea{}, fmt.Errorf("sight area: invalid identity or radius: %w", ErrInvalidData)
	}
	return SightArea{ID: in.ID, SourceID: in.SourceID, Name: in.Name, Ref: in.Ref, Center: in.Center, RadiusFeet: in.RadiusFeet}, nil
}
func (e *Encounter) AddSightArea(in *SightAreaInput) error {
	a, err := in.area()
	if err != nil {
		return err
	}
	if _, ok := e.sightAreas[a.ID]; ok {
		return fmt.Errorf("sight area %q exists: %w", a.ID, ErrInvalidData)
	}
	if e.sightAreas == nil {
		e.sightAreas = make(map[string]SightArea)
	}
	e.sightAreas[a.ID] = a
	return nil
}
func (e *Encounter) ReplaceSightAreas(data []SightAreaData) error {
	if err := validateSightAreasData(data); err != nil {
		return err
	}
	next := sightAreasFromData(data)
	e.sightAreas = next
	return nil
}

func (e *Encounter) RemoveSightArea(sourceID string) bool {
	removed := false
	for id, a := range e.sightAreas {
		if a.SourceID == sourceID {
			delete(e.sightAreas, id)
			removed = true
		}
	}
	return removed
}

// SightAreasFor returns areas whose visible footprint can be shown to member.
func (e *Encounter) SightAreasFor(member MemberID) []SightArea {
	_, ok := e.members[member]
	if !ok {
		return nil
	}
	pos, ok := e.canvas.GetEntityPosition(string(member))
	if !ok {
		return nil
	}
	reach, err := e.sight.Sight([]MemberID{member})
	if err != nil {
		return nil
	}
	maxDistance, ok := reach[member]
	if !ok || maxDistance < 0 {
		return nil
	}
	out := make([]SightArea, 0)
	for _, a := range e.sightAreas {
		c := spatial.Position{X: a.Center.X, Y: a.Center.Y}
		r := float64(a.RadiusFeet) / float64(FeetPerCell)
		distance := e.canvas.GetGrid().Distance(pos, c)
		// A member standing inside the area can always see its own footprint;
		// otherwise both supplied sight range and the wall ray bound visibility.
		if distance <= r || (distance <= float64(maxDistance) && !e.canvas.IsLineOfSightBlocked(pos, c)) {
			out = append(out, a)
		}
	}
	sort.Slice(out, func(i, j int) bool { return out[i].ID < out[j].ID })
	return out
}
func sightAreasDataFrom(in map[string]SightArea) []SightAreaData {
	if len(in) == 0 {
		return nil
	}
	ids := make([]string, 0, len(in))
	for id := range in {
		ids = append(ids, id)
	}
	sort.Strings(ids)
	out := make([]SightAreaData, 0, len(ids))
	for _, id := range ids {
		a := in[id]
		out = append(out, SightAreaData{ID: a.ID, SourceID: a.SourceID, Name: a.Name, Ref: a.Ref, Center: PositionData{X: a.Center.X, Y: a.Center.Y}, RadiusFeet: a.RadiusFeet})
	}
	return out
}
func sightAreasFromData(in []SightAreaData) map[string]SightArea {
	out := make(map[string]SightArea, len(in))
	for _, d := range in {
		out[d.ID] = SightArea{ID: d.ID, SourceID: d.SourceID, Name: d.Name, Ref: d.Ref, Center: spatial.Position{X: d.Center.X, Y: d.Center.Y}, RadiusFeet: d.RadiusFeet}
	}
	return out
}

// SightAreaContains reports whether a point lies inside the runtime sight area.
// The caller supplies the encounter canvas grid so membership uses the same
// hex distance and feet-to-cell conversion as sight reach.
func SightAreaContains(area SightAreaData, point spatial.Position, grid spatial.Grid) bool {
	if area.RadiusFeet <= 0 {
		return false
	}
	center := spatial.Position{X: area.Center.X, Y: area.Center.Y}
	return grid.Distance(center, point) <= float64(area.RadiusFeet)/float64(FeetPerCell)
}

func finite(v float64) bool { return !math.IsNaN(v) && !math.IsInf(v, 0) }
func areaCrosses(a SightArea, from, to spatial.Position, grid spatial.Grid) bool {
	r := float64(a.RadiusFeet) / float64(FeetPerCell)
	if grid.Distance(from, a.Center) <= r || grid.Distance(to, a.Center) <= r {
		return true
	}
	dx, dy := to.X-from.X, to.Y-from.Y
	l2 := dx*dx + dy*dy
	if l2 == 0 {
		return false
	}
	t := ((a.Center.X-from.X)*dx + (a.Center.Y-from.Y)*dy) / l2
	if t < 0 {
		t = 0
	}
	if t > 1 {
		t = 1
	}
	x, y := from.X+t*dx, from.Y+t*dy
	return grid.Distance(spatial.Position{X: x, Y: y}, a.Center) <= r
}

func validateSightAreasData(in []SightAreaData) error {
	seen := make(map[string]bool, len(in))
	sources := make(map[string]bool, len(in))
	for _, d := range in {
		if d.ID == "" || d.SourceID == "" || d.RadiusFeet <= 0 || !finite(d.Center.X) || !finite(d.Center.Y) {
			return fmt.Errorf("invalid sight area %q: %w", d.ID, ErrInvalidData)
		}
		if seen[d.ID] {
			return fmt.Errorf("duplicate sight area %q: %w", d.ID, ErrInvalidData)
		}
		seen[d.ID] = true
		_ = sources
	}
	return nil
}
