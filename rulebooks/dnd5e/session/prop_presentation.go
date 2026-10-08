// Copyright (C) 2026 Kirk Diggler
// SPDX-License-Identifier: GPL-3.0-or-later

package session

import (
	"encoding/json"
	"math"
	"regexp"

	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/encounter"
)

// PropPresentation is the SDK-owned permitted renderer input. Its PropID shares
// the mechanical/observation namespace, but none of these fields decides rules.
// Distances are feet, facing canonical degrees, and HeightScale dimensionless.
type PropPresentation struct {
	ID            string          `json:"id"`
	Ref           string          `json:"ref"`
	Origin        FootprintPoint  `json:"origin"`
	Elevation     float64         `json:"elevation"`
	FacingDegrees float64         `json:"facing_degrees"`
	HeightScale   float64         `json:"height_scale"`
	PointLight    *PropPointLight `json:"point_light,omitempty"`
	DoorID        string          `json:"door_id,omitempty"`
	Label         string          `json:"label"`
}

// PropPointLight is optional visual emission, not mechanical illumination.
type PropPointLight struct {
	Enabled         bool           `json:"enabled"`
	Offset          FootprintPoint `json:"offset"`
	OffsetElevation float64        `json:"offset_elevation"`
	Color           string         `json:"color"`
	Intensity       float64        `json:"intensity"`
	Range           float64        `json:"range"`
}

func projectPropPresentation(p encounter.PropPresentation) PropPresentation {
	out := PropPresentation{ID: p.ID, Ref: p.Ref, Origin: FootprintPoint{X: p.Origin.X, Y: p.Origin.Y}, Elevation: p.Elevation, FacingDegrees: p.FacingDegrees, HeightScale: p.HeightScale, DoorID: p.DoorID, Label: p.Label}
	if l := p.PointLight; l != nil {
		out.PointLight = &PropPointLight{Enabled: l.Enabled, Offset: FootprintPoint{X: l.Offset.X, Y: l.Offset.Y}, OffsetElevation: l.OffsetElevation, Color: l.Color, Intensity: l.Intensity, Range: l.Range}
	}
	return out
}

var presentationColor = regexp.MustCompile(`^#[0-9a-fA-F]{6}$`)

func validPropPresentation(p PropPresentation) bool {
	if p.ID == "" || p.Ref == "" || p.HeightScale <= 0 {
		return false
	}
	numbers := []float64{p.Origin.X, p.Origin.Y, p.Elevation, p.FacingDegrees, p.HeightScale}
	if l := p.PointLight; l != nil {
		if !presentationColor.MatchString(l.Color) || l.Intensity < 0 || l.Range <= 0 {
			return false
		}
		numbers = append(numbers, l.Offset.X, l.Offset.Y, l.OffsetElevation, l.Intensity, l.Range)
	}
	for _, n := range numbers {
		if math.IsNaN(n) || math.IsInf(n, 0) {
			return false
		}
	}
	return true
}

// propPresentationsFromPayload validates the whole optional collection before
// returning it. Presence is not guessed from zero-valued origin coordinates.
func propPresentationsFromPayload(payload []byte) ([]PropPresentation, bool) {
	var body struct {
		Rows  []json.RawMessage     `json:"prop_presentations"`
		Doors []AtlasStructuralDoor `json:"structural_doors"`
	}
	if json.Unmarshal(payload, &body) != nil {
		return nil, false
	}
	seen := make(map[string]bool)
	doors := make(map[string]bool)
	for _, d := range body.Doors {
		doors[d.ID] = true
	}
	var out []PropPresentation
	for _, raw := range body.Rows {
		var row struct {
			PropPresentation
			Origin *FootprintPoint `json:"origin"`
			Light  *struct {
				PropPointLight
				Offset *FootprintPoint `json:"offset"`
			} `json:"point_light"`
		}
		if json.Unmarshal(raw, &row) != nil || row.Origin == nil {
			return nil, false
		}
		p := row.PropPresentation
		p.Origin = *row.Origin
		if row.Light != nil {
			if row.Light.Offset == nil {
				return nil, false
			}
			l := row.Light.PropPointLight
			l.Offset = *row.Light.Offset
			p.PointLight = &l
		}
		if !validPropPresentation(p) || seen[p.ID] || (p.DoorID != "" && doors[p.DoorID]) {
			return nil, false
		}
		seen[p.ID] = true
		if p.DoorID != "" {
			doors[p.DoorID] = true
		}
		out = append(out, p)
	}
	return out, true
}
