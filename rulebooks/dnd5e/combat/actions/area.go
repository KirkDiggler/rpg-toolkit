// Copyright (C) 2026 Kirk Diggler
// SPDX-License-Identifier: GPL-3.0-or-later

package actions

import "fmt"

// AreaShape is the kind of region a footprint covers.
//
// Two values, and each arrived with the spell that needed it — Thunderclap's
// burst and Thunderwave's cube. A cone is still not declared here in advance:
// nothing has asked for one, so an enum value for it would be an affordance
// with nothing behind it. (rpg-toolkit#1626, the geometry that would answer a
// cone, is answered by coverage now; the missing half is a customer.)
type AreaShape string

const (
	// AreaRadius is a round footprint measured from its origin outward.
	AreaRadius AreaShape = "radius"

	// AreaBox is a rectangular footprint, as deep along the direction it is
	// aimed as it is wide across it — a cube on the table, whose height
	// nothing in this stack measures.
	//
	// SizeFeet is that single edge, so a 15-foot cube is one number. A box
	// with unequal sides has not been asked for, and splitting the field for
	// one would be the affordance with nothing behind it.
	AreaBox AreaShape = "box"
)

// AreaOrigin says where a footprint is anchored.
//
// Both values today anchor on the CASTER, and that is what keeps them cheap: a
// range check measured from the caster still bounds every cell either one can
// reach. A footprint anchored at a chosen point — Fireball's "a point you
// choose within range" — is still the next one, and still a bigger change than
// an enum value: it needs a point to arrive from the client, and it breaks
// that caster-centred check. Declared when that work is done, not before.
type AreaOrigin string

const (
	// AreaOriginCaster anchors the footprint on whoever cast. The caster
	// stands at its centre.
	AreaOriginCaster AreaOrigin = "caster"

	// AreaOriginCasterEdge anchors the footprint's NEAR edge on the caster's
	// own boundary, along the direction the cast is aimed.
	//
	// The caster is therefore never under the shape — which is the difference
	// that matters, and the reason it is an origin rather than a projection.
	// [AreaCatchesOthers] spares the caster from a shape they stand in;
	// this says they never stood in it. Thunderwave's cube "originates from
	// you" and extends away, so a caster excluded by projection alone would
	// still have the first five feet of the cube standing on their square.
	AreaOriginCasterEdge AreaOrigin = "caster-edge"
)

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
	// radius for [AreaRadius], one edge for [AreaBox]. Feet rather than cells
	// because this is a declaration, and the grid it will be measured against
	// is not in scope where a profile is written.
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
	case AreaRadius, AreaBox:
	default:
		return fmt.Errorf("unknown area shape %q", f.Shape)
	}
	switch f.Origin {
	case AreaOriginCaster, AreaOriginCasterEdge:
	default:
		return fmt.Errorf("unknown area origin %q", f.Origin)
	}
	// The two closed switches say each half is known; this says the pair makes
	// sense. A box centred on the caster would cover the caster, which no
	// spell says, and a radius has no near edge to sit on a boundary, so the
	// edge anchor would be silently ignored. Both are refused rather than
	// reinterpreted: a shape drawn in the wrong place with nothing saying why
	// is the failure worth paying two lines to prevent.
	if f.Shape == AreaBox && f.Origin != AreaOriginCasterEdge {
		return fmt.Errorf("a box must be anchored on the caster's edge, got origin %q", f.Origin)
	}
	if f.Shape != AreaBox && f.Origin == AreaOriginCasterEdge {
		return fmt.Errorf("only a box may be anchored on the caster's edge, got shape %q", f.Shape)
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
