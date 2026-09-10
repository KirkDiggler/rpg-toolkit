// Copyright (C) 2026 Kirk Diggler
// SPDX-License-Identifier: GPL-3.0-or-later

package actions

import "fmt"

// AreaShape is the kind of region a footprint covers.
//
// One value today, and a second arrives with the spell that needs it. A cone
// is not declared here in advance: the geometry that would answer it is wrong
// (rpg-toolkit#1626) and nothing has asked, so an enum value for it would be an
// affordance with nothing behind it.
type AreaShape string

// AreaRadius is a round footprint measured from its origin outward.
const AreaRadius AreaShape = "radius"

// AreaOrigin says where a footprint is anchored.
//
// Also one value today. A footprint anchored at a chosen point — Fireball's
// "a point you choose within range" — is the next one, and it is a bigger
// change than an enum value: it needs a point to arrive from the client, and
// it breaks the caster-centred range check resolution currently applies per
// target. Declared when that work is done, not before.
type AreaOrigin string

// AreaOriginCaster anchors the footprint on whoever cast.
const AreaOriginCaster AreaOrigin = "caster"

// Footprint is a shape in space. It says WHERE. It never says WHO.
//
// SEPARATE FROM [CastArea] ON PURPOSE, and this is the one place this slice
// builds a hair more structure than it strictly needs. A cast uses a footprint
// once. A persistent effect — the burning floor a fireball leaves — is the same
// shape, held, and re-projected every time somebody moves. The two differ in
// who they catch, not in what shape they are: the floor catches EVERYONE
// including the caster, and a thunderclap catches everyone but.
//
// Fold the exclusion into the shape and a stored footprint carries a field that
// is meaningless for half its intended uses, which is the zero value that lies
// — the same defect [CastConcentration]'s "a pointer rather than a bool"
// rationale names. Keeping them apart is what leaves the lingering area
// buildable later without re-cutting this type.
type Footprint struct {
	// Shape is the kind of region covered.
	Shape AreaShape `json:"shape"`

	// SizeFeet is the footprint's extent in FEET, as content authors it — a
	// radius for [AreaRadius]. Feet rather than cells because this is a
	// declaration, and the grid it will be measured against is not in scope
	// where a profile is written.
	SizeFeet int `json:"size_feet"`

	// Origin is what the footprint is anchored to.
	Origin AreaOrigin `json:"origin"`
}

// Validate reports whether the footprint declares a known shape, a known
// origin, and a positive extent.
//
// It does NOT check that the extent survives conversion to cells. A footprint
// of one to four feet floors to zero cells and can then only ever catch
// something on the origin's own cell — real, and worth refusing — but the
// conversion lives in a module this one cannot import (rpg-toolkit#1625), so
// that refusal is made by the layer that can measure. Stated here so the
// absence reads as a known cost rather than an oversight.
func (f Footprint) Validate() error {
	switch f.Shape {
	case AreaRadius:
	default:
		return fmt.Errorf("unknown area shape %q", f.Shape)
	}
	switch f.Origin {
	case AreaOriginCaster:
	default:
		return fmt.Errorf("unknown area origin %q", f.Origin)
	}
	if f.SizeFeet <= 0 {
		return fmt.Errorf("area must declare a positive size in feet")
	}
	return nil
}

// AreaCatches is the projection one cast takes of its footprint.
type AreaCatches string

const (
	// AreaCatchesOthers is "every creature other than you" — the caster is
	// inside the shape and is not affected by it.
	AreaCatchesOthers AreaCatches = "others"

	// AreaCatchesEveryone spares nobody standing in the shape, the caster
	// included.
	AreaCatchesEveryone AreaCatches = "everyone"
)

// CastArea is one cast's use of a footprint: the shape, and which of the
// creatures standing in it this particular spell affects.
//
// The engine derives the members; content never names them. That is the whole
// capability — everything the toolkit could hit before this, it hit because
// somebody named it.
type CastArea struct {
	// Footprint is the shape in space.
	Footprint Footprint `json:"footprint"`

	// Catches is which creatures standing in the footprint this cast affects.
	Catches AreaCatches `json:"catches"`
}

// Validate reports whether the area declares a legal footprint and a known
// projection.
func (a CastArea) Validate() error {
	if err := a.Footprint.Validate(); err != nil {
		return err
	}
	switch a.Catches {
	case AreaCatchesOthers, AreaCatchesEveryone:
	default:
		return fmt.Errorf("unknown area projection %q", a.Catches)
	}
	return nil
}
