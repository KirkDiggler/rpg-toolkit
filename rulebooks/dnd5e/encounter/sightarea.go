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
	ID                 string
	SourceID           string
	Name               string
	Ref                string
	Center             spatial.Position
	RadiusFeet         int
	MembershipRef      string
	MembershipName     string
	MembershipSourceID string
}

type SightAreaData struct {
	ID                 string
	SourceID           string
	Name               string
	Ref                string
	Center             PositionData
	RadiusFeet         int
	MembershipRef      string
	MembershipName     string
	MembershipSourceID string
}
type SightAreaInput struct {
	ID                 string
	SourceID           string
	Name               string
	Ref                string
	Center             spatial.Position
	RadiusFeet         int
	MembershipRef      string
	MembershipName     string
	MembershipSourceID string
}

func (in *SightAreaInput) area() (SightArea, error) {
	if in == nil {
		return SightArea{}, fmt.Errorf("sight area: %w", ErrNilInput)
	}
	if in.ID == "" || in.SourceID == "" || in.RadiusFeet <= 0 || !finite(in.Center.X) || !finite(in.Center.Y) {
		return SightArea{}, fmt.Errorf("sight area: invalid identity or radius: %w", ErrInvalidData)
	}
	if (in.MembershipRef == "") != (in.MembershipName == "") || (in.MembershipRef == "") != (in.MembershipSourceID == "") {
		return SightArea{}, fmt.Errorf("sight area: membership metadata must be all present or absent: %w", ErrInvalidData)
	}
	return SightArea{ID: in.ID, SourceID: in.SourceID, Name: in.Name, Ref: in.Ref, Center: in.Center, RadiusFeet: in.RadiusFeet, MembershipRef: in.MembershipRef, MembershipName: in.MembershipName, MembershipSourceID: in.MembershipSourceID}, nil
}

// AddSightArea opens one runtime area and tells its story at once: every
// member whose placement the area holds is told it entered, with the area's
// own membership label. An area without a membership label obscures sight
// and tells nobody anything.
//
// TELLING IS IMMEDIATE, SO ORDER IS THE CALLER'S. The beats append when the
// area is applied, never held for later, so a save or a step cannot lose or
// reorder them. A host applies an area change after it records the outcome
// that caused it (the cast, the broken concentration), and the story then
// reads cause before membership.
//
// Errors: ErrNilInput; ErrInvalidData for an invalid identity, radius or
// partial membership label, or an id already open; any error reading the
// roster's placement or the sight answer the audience is told by, with the
// area set and the story untouched.
func (e *Encounter) AddSightArea(in *SightAreaInput) error {
	a, err := in.area()
	if err != nil {
		return err
	}
	if _, ok := e.sightAreas[a.ID]; ok {
		return fmt.Errorf("sight area %q exists: %w", a.ID, ErrInvalidData)
	}
	before := e.copySightAreas()
	after := e.copySightAreas()
	after[a.ID] = a
	transitions, err := e.areaChangeTransitions(before, after)
	if err != nil {
		return err
	}
	e.sightAreas = after
	return e.appendSightAreaTransitions(transitions)
}

// RemoveSightArea ends every runtime area the source opened and tells its
// story at once: every member the ended area held is told "area ended". As
// with [Encounter.AddSightArea], a host removes the area after it records
// the outcome that ended it. removed is false when the source opened no area,
// which is an answer rather than an error.
//
// Errors: any error reading the roster's placement or the sight answer the
// audience is told by, with the area set and the story untouched.
func (e *Encounter) RemoveSightArea(sourceID string) (removed bool, err error) {
	before := e.copySightAreas()
	after := e.copySightAreas()
	for id, a := range after {
		if a.SourceID == sourceID {
			delete(after, id)
			removed = true
		}
	}
	if !removed {
		return false, nil
	}
	transitions, err := e.areaChangeTransitions(before, after)
	if err != nil {
		return false, err
	}
	e.sightAreas = after
	if err := e.appendSightAreaTransitions(transitions); err != nil {
		return false, err
	}
	return true, nil
}

func (e *Encounter) copySightAreas() map[string]SightArea {
	out := make(map[string]SightArea, len(e.sightAreas)+1)
	for id, a := range e.sightAreas {
		out[id] = a
	}
	return out
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
		// A member standing inside the area can always see its own footprint;
		// otherwise both supplied sight range and the wall ray bound visibility.
		if e.inSightArea(a, pos) ||
			(e.canvas.GetGrid().Distance(pos, c) <= float64(maxDistance) && !e.canvas.IsLineOfSightBlocked(pos, c)) {
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
		out = append(out, SightAreaData{ID: a.ID, SourceID: a.SourceID, Name: a.Name, Ref: a.Ref, MembershipRef: a.MembershipRef, MembershipName: a.MembershipName, MembershipSourceID: a.MembershipSourceID, Center: PositionData{X: a.Center.X, Y: a.Center.Y}, RadiusFeet: a.RadiusFeet})
	}
	return out
}
func sightAreasFromData(in []SightAreaData) map[string]SightArea {
	out := make(map[string]SightArea, len(in))
	for _, d := range in {
		out[d.ID] = SightArea{ID: d.ID, SourceID: d.SourceID, Name: d.Name, Ref: d.Ref, MembershipRef: d.MembershipRef, MembershipName: d.MembershipName, MembershipSourceID: d.MembershipSourceID, Center: spatial.Position{X: d.Center.X, Y: d.Center.Y}, RadiusFeet: d.RadiusFeet}
	}
	return out
}

func finite(v float64) bool { return !math.IsNaN(v) && !math.IsInf(v, 0) }
func areaCrosses(a SightArea, from, to spatial.Position, grid spatial.Grid) bool {
	if areaHolds(a, from, grid) || areaHolds(a, to, grid) {
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
	return areaHolds(a, spatial.Position{X: x, Y: y}, grid)
}

func validateSightAreasData(in []SightAreaData) error {
	seen := make(map[string]bool, len(in))
	for _, d := range in {
		if d.ID == "" || d.SourceID == "" || d.RadiusFeet <= 0 || !finite(d.Center.X) || !finite(d.Center.Y) {
			return fmt.Errorf("invalid sight area %q: %w", d.ID, ErrInvalidData)
		}
		if seen[d.ID] {
			return fmt.Errorf("duplicate sight area %q: %w", d.ID, ErrInvalidData)
		}
		seen[d.ID] = true
		if (d.MembershipRef == "") != (d.MembershipName == "") || (d.MembershipRef == "") != (d.MembershipSourceID == "") {
			return fmt.Errorf("invalid sight area %q membership metadata: %w", d.ID, ErrInvalidData)
		}
	}
	return nil
}
