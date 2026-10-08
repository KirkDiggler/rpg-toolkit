// Copyright (C) 2026 Kirk Diggler
// SPDX-License-Identifier: GPL-3.0-or-later

package encounter

import (
	"fmt"
	"reflect"
	"regexp"
	"sort"

	"github.com/KirkDiggler/rpg-toolkit/tools/spatial"
)

// PropPresentation is a permitted render record in the shared PropID namespace.
// FieldInput supplies immutable source definitions; Atlas and PropSighting return
// detached fixed or captured records. Distances are canonical feet, facing is
// canonical planar degrees, and HeightScale is a positive visual-only Y scale.
// No field here controls collision, sight range or an object's mutable location.
type PropPresentation struct {
	ID            PropID
	Ref           string
	Origin        spatial.Point
	Elevation     float64
	FacingDegrees float64
	HeightScale   float64
	PointLight    *PropPointLight
	DoorID        DoorID
	Label         string
}

// PropPointLight is explicitly authored visual emission relative to a prop.
// Offsets/range are feet. It is not a declaration of mechanical illumination.
type PropPointLight struct {
	Enabled         bool
	Offset          spatial.Point
	OffsetElevation float64
	Color           string
	Intensity       float64
	Range           float64
}

// PropPresentationData persists one source definition or captured observation.
// No editor document, parent association or mutable door state is embedded.
type PropPresentationData struct {
	ID            string              `json:"id"`
	Ref           string              `json:"ref"`
	Origin        spatial.Point       `json:"origin"`
	Elevation     float64             `json:"elevation"`
	FacingDegrees float64             `json:"facing_degrees"`
	HeightScale   float64             `json:"height_scale"`
	PointLight    *PropPointLightData `json:"point_light,omitempty"`
	DoorID        string              `json:"door_id,omitempty"`
	Label         string              `json:"label"`
}

// PropPointLightData is the portable visual-light definition, never a rule fact.
type PropPointLightData struct {
	Enabled         bool          `json:"enabled"`
	Offset          spatial.Point `json:"offset"`
	OffsetElevation float64       `json:"offset_elevation"`
	Color           string        `json:"color"`
	Intensity       float64       `json:"intensity"`
	Range           float64       `json:"range"`
}

var propLightColor = regexp.MustCompile(`^#[0-9a-fA-F]{6}$`)

func validatePropPresentation(p PropPresentation) error {
	if p.ID == "" || p.Ref == "" {
		return fmt.Errorf("prop presentation needs a nonempty id and ref: %w", ErrNoField)
	}
	for _, n := range []float64{p.Origin.X, p.Origin.Y, p.Elevation, p.FacingDegrees, p.HeightScale} {
		if !structuralRepresentable(n) {
			return fmt.Errorf("prop presentation %q has a nonfinite or unrepresentable number: %w", p.ID, ErrNoField)
		}
	}
	if p.HeightScale <= 0 {
		return fmt.Errorf("prop presentation %q height scale must be positive: %w", p.ID, ErrNoField)
	}
	if l := p.PointLight; l != nil {
		for _, n := range []float64{l.Offset.X, l.Offset.Y, l.OffsetElevation, l.Intensity, l.Range} {
			if !structuralRepresentable(n) {
				return fmt.Errorf("prop presentation %q light has an invalid number: %w", p.ID, ErrNoField)
			}
		}
		if !propLightColor.MatchString(l.Color) || l.Intensity < 0 || l.Range <= 0 {
			return fmt.Errorf("prop presentation %q light needs RGB color, nonnegative intensity and positive range: %w", p.ID, ErrNoField)
		}
	}
	return nil
}

func clonePropPresentation(p PropPresentation) PropPresentation {
	if p.PointLight != nil {
		l := *p.PointLight
		p.PointLight = &l
	}
	return p
}

func propPresentationData(p PropPresentation) PropPresentationData {
	d := PropPresentationData{ID: p.ID, Ref: p.Ref, Origin: p.Origin, Elevation: p.Elevation, FacingDegrees: p.FacingDegrees, HeightScale: p.HeightScale, DoorID: p.DoorID, Label: p.Label}
	if l := p.PointLight; l != nil {
		d.PointLight = &PropPointLightData{Enabled: l.Enabled, Offset: l.Offset, OffsetElevation: l.OffsetElevation, Color: l.Color, Intensity: l.Intensity, Range: l.Range}
	}
	return d
}

func propPresentationFromData(d PropPresentationData) PropPresentation {
	p := PropPresentation{ID: d.ID, Ref: d.Ref, Origin: d.Origin, Elevation: d.Elevation, FacingDegrees: d.FacingDegrees, HeightScale: d.HeightScale, DoorID: d.DoorID, Label: d.Label}
	if l := d.PointLight; l != nil {
		p.PointLight = &PropPointLight{Enabled: l.Enabled, Offset: l.Offset, OffsetElevation: l.OffsetElevation, Color: l.Color, Intensity: l.Intensity, Range: l.Range}
	}
	return p
}

func (f *field) compilePropPresentations(in []PropPresentation, doors []DoorInput) error {
	reserved := make(map[PropID]bool)
	structuralDoors := make(map[DoorID]bool)
	for _, w := range f.structuralWalls {
		reserved[w.id] = true
		for _, o := range w.openings {
			if o.door != nil {
				reserved[o.door.placedID] = true
				structuralDoors[o.door.doorID] = true
			}
		}
	}
	seen := make(map[PropID]bool, len(in))
	boundDoors := make(map[DoorID]bool)
	out := make([]PropPresentation, 0, len(in))
	for _, p := range in {
		if err := validatePropPresentation(p); err != nil {
			return err
		}
		if seen[p.ID] || reserved[p.ID] {
			return fmt.Errorf("duplicate/structural prop presentation %q: %w", p.ID, ErrNoField)
		}
		seen[p.ID] = true
		if i := f.placedIndexOf(p.ID); i >= 0 {
			placement := f.placed[i].placement
			if p.Origin != placement.Origin || p.FacingDegrees != placement.Facing {
				return fmt.Errorf("prop presentation %q conflicts with its placed pose: %w", p.ID, ErrNoField)
			}
		}
		if p.DoorID != "" {
			if structuralDoors[p.DoorID] || boundDoors[p.DoorID] {
				return fmt.Errorf("duplicate structural/prop door appearance %q: %w", p.DoorID, ErrNoField)
			}
			found := false
			placedIndex := f.placedIndexOf(p.ID)
			if placedIndex >= 0 && f.placed[placedIndex].mutable() {
				return fmt.Errorf("door presentation %q needs a fixed placed identity: %w", p.ID, ErrNoField)
			}
			for _, door := range doors {
				if door.ID == p.DoorID && placedIndex >= 0 && door.Placement != nil {
					found = reflect.DeepEqual(*door.Placement, f.placed[placedIndex].placement)
					break
				}
			}
			if !found {
				return fmt.Errorf("prop presentation %q has no matching placed door %q: %w", p.ID, p.DoorID, ErrNoField)
			}
			boundDoors[p.DoorID] = true
		}
		out = append(out, clonePropPresentation(p))
	}
	sort.Slice(out, func(i, j int) bool { return out[i].ID < out[j].ID })
	f.propPresentations = out
	return nil
}

func (f *field) presentationIndexOf(id PropID) int {
	i := sort.Search(len(f.propPresentations), func(i int) bool { return f.propPresentations[i].ID >= id })
	if i < len(f.propPresentations) && f.propPresentations[i].ID == id {
		return i
	}
	return -1
}

// A decorative point has no fabricated rectangle. Its disclosure support is
// the nearest declared floor cell, with the field's canonical tie-break. Bound
// props retain their existing mechanical support and permission instead.
func (f *field) presentationCell(origin spatial.Point) spatial.Position {
	var nearest spatial.Position
	var distance float64
	for i, cell := range f.cells {
		c := f.plane.CellCentre(cell)
		dx, dy := c.X-origin.X, c.Y-origin.Y
		d := dx*dx + dy*dy
		if i == 0 || d < distance || (d == distance && cellBefore(cell, nearest)) {
			nearest, distance = cell, d
		}
	}
	return nearest
}

// presentPropDefinitions projects current world truth ONLY for the full Atlas.
// Player reads filter these rows; mutable sightings capture them during the
// observation pass and never look them up again while returning memory.
func (f *field) presentPropDefinitions(out *Atlas) {
	placed := make(map[PropID]AtlasPlacedProp, len(out.Placed))
	props := make(map[PropID]AtlasProp, len(out.Props))
	for _, p := range out.Placed {
		placed[p.ID] = p
	}
	for _, p := range out.Props {
		props[p.ID] = p
	}
	var states map[PropID]propPlacement
	if f.holdings != nil && len(f.propPresentations) > 0 {
		states = f.holdings.propPlacements()
	}
	for _, definition := range f.propPresentations {
		p := clonePropPresentation(definition)
		if f.placedIndexOf(p.ID) >= 0 {
			current, exists := placed[p.ID]
			if !exists {
				continue
			}
			p.Origin = current.Placement.Origin
			p.FacingDegrees = current.Placement.Facing
		} else if f.propIndexOf(p.ID) >= 0 {
			current, exists := props[p.ID]
			if !exists {
				continue
			}
			if states[p.ID].moved {
				p.Origin = f.plane.CellCentre(current.At)
			}
		}
		if states[p.ID].dropped {
			p.Elevation = 0
		}
		out.PropPresentations = append(out.PropPresentations, p)
	}
}

func (e *Encounter) projectPropPresentations(out *Atlas, full Atlas, hidden hiddenView) {
	allowed := make(map[PropID]bool, len(out.Placed)+len(out.Props))
	for _, p := range out.Placed {
		allowed[p.ID] = true
	}
	for _, p := range out.Props {
		allowed[p.ID] = true
	}
	for _, p := range full.PropPresentations {
		if hidden.props[p.ID] || hidden.doors[p.ID] || (p.DoorID != "" && hidden.doors[p.DoorID]) {
			continue
		}
		bound := e.field.placedIndexOf(p.ID) >= 0 || e.field.propIndexOf(p.ID) >= 0
		if bound && !allowed[p.ID] {
			continue
		}
		if !bound && hidden.unexploredCells[e.field.presentationCell(p.Origin)] {
			continue
		}
		out.PropPresentations = append(out.PropPresentations, clonePropPresentation(p))
	}
}

func addPropPresentationReveal(payload map[string]interface{}, before, after Atlas) {
	had := make(map[PropID]bool, len(before.PropPresentations))
	for _, p := range before.PropPresentations {
		had[p.ID] = true
	}
	var added []PropPresentationData
	for _, p := range after.PropPresentations {
		if !had[p.ID] {
			added = append(added, propPresentationData(p))
		}
	}
	if len(added) > 0 {
		payload["prop_presentations"] = added
	}
}
