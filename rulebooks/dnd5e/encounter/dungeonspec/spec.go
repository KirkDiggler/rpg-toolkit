// Copyright (C) 2026 Kirk Diggler
// SPDX-License-Identifier: GPL-3.0-or-later

// Package dungeonspec compiles an authored dungeon into a world the
// composition can run (rpg-toolkit#1127, rpg-project#256).
//
// # Where this lives, and why it is here rather than in the host
//
// The yaml is content and the target is [encounter.FieldInput], so the compiler
// could plausibly sit on either side of that seam. It sits here because the
// toolkit owns geometry: rpg-api's own words for the arrangement it already has
// are that "rpg-api never computes dungeon geometry itself — a key maps to a
// builder, and only the toolkit turns that spec into an actual Space".
//
// # Version 2: the regions are the floor
//
// Version 1 described a dungeon as a chain of rectangular chambers and
// generated the seam walls and doorways between them. Version 2 — the dialect
// the dungeon builder writes (rpg-project#256) — describes the FLOOR directly:
// named regions painted cell by cell, walls and doors as edges between adjacent
// floor cells, everything in absolute [col,row] under one declared
// orientation. Version 1 is deleted, not supported: a file that says
// `version: 1` is refused by name, and the reference tomb is re-authored in
// version 2 and compiles to the identical atlas (golden_test.go).
//
// # What it may not know
//
// This package compiles GEOMETRY and carries everything else. It resolves no
// content: "dnd5e:monsters:skeleton" comes out the far end as the same string
// that went in, and so does "lowest-health", and so does a region's
// `archetype`. The composition may not import a rulebook (design law C1), and
// a compiler that resolved refs here would drag one in. Refs become sheets one
// layer up, where a package may import both.
//
// So the output is deliberately in two halves: a [encounter.FieldInput] that is
// ready to hand to [encounter.NewEncounter] as it stands, and a roster of
// placements that still need somebody who knows what a skeleton is.
//
// # Validation is path-addressed
//
// [Validate] reports EVERY defect it finds, each naming the YAML path of the
// thing that is wrong (`regions[1].cells[0][3]`, `walls[3]`, `place[4].blocks_los`,
// `start`), because the builder draws each one on the canvas at the thing it
// names. A tool that only wants to know whether a file is a dungeon reads the
// length of the list.
package dungeonspec

import (
	"fmt"

	"gopkg.in/yaml.v3"
)

// Spec is the authored dungeon, decoded and not yet checked. Every field is
// exactly what the file said; nothing is defaulted, derived, or repaired.
//
// # Pointers mean "the author may leave this out"
//
// Where a field is a pointer, an omitted value and an explicitly written one
// are different facts and the compiler must be able to tell them apart. The
// coffin is the reason: `blocks_los: false` is an authored exception, and a
// plain bool would make it indistinguishable from a placement that said nothing
// — which is how the reference tomb's one see-over-able prop would silently
// become a wall. `start`, `lighting` and `lighting.intensity` are pointers for
// the same reason: [0,0] and 0 are legal values an omission must not become.
type Spec struct {
	// Version is the dialect this file is written in. Exactly one is
	// understood — [Version] — and any other is refused by name rather than
	// read hopefully, version 1 included.
	Version int `yaml:"version"`

	// Key is the dungeon's identifier: `[a-z0-9-]`, the file's identity, and
	// the prefix of every door this compile mints so two dungeons in one
	// process cannot collide.
	Key string `yaml:"key"`

	// Name is the dungeon's display title. Carried, never read.
	Name string `yaml:"name"`

	// Orientation is which way the hexes point — "pointy" or "flat".
	// REQUIRED: every [col,row] pair in this file is an offset coordinate,
	// and an offset coordinate means nothing until the orientation is known.
	Orientation string `yaml:"orientation"`

	// Void is what the space between the regions does to a sightline —
	// "opaque" or "transparent", the words [encounter.Void] carries.
	// REQUIRED: [encounter.CanvasInput.Void] has no default by ruling.
	Void string `yaml:"void"`

	// Regions are the floor: their cells' union, and nothing else.
	Regions []RegionSpec `yaml:"regions"`

	// Start is where the party comes in: an absolute [col,row] cell that
	// must be floor. REQUIRED and explicit — version 1 derived it from an
	// archetype, which is the shape of defaulting rpg-toolkit#1033 forbids.
	Start *[2]int `yaml:"start"`

	// Walls are edges between adjacent floor cells that block both movement
	// and sight. The envelope is implied, never written: a crossing from
	// floor into void is a crossing nobody can make. Each entry is a bare
	// edge or an object carrying an authored height — see [WallSpec].
	Walls []WallSpec `yaml:"walls"`

	// Doors are one state each over one or more edges (rpg-toolkit#1123).
	Doors []DoorSpec `yaml:"doors"`

	// Place is everything standing on the floor — props, monsters, and the
	// boss alike, routed by the ref's type segment — at ABSOLUTE cells.
	Place []PlaceSpec `yaml:"place"`
}

// Version is the one dialect this build speaks.
//
// A single accepted value rather than a floor, deliberately: "version 2 or
// later is fine" is a promise about files nobody has written yet, and the
// standing precedent for a shape this build does not know is to refuse it by
// name rather than read it hopefully (rpg-toolkit#1053/#1068).
const Version = 2

// RegionSpec is one named set of cells with the per-area facts it carries.
type RegionSpec struct {
	// ID names the region, and survives the compile as the region's ID.
	ID string `yaml:"id"`

	// Name is the region's display name. Carried, never read.
	Name string `yaml:"name"`

	// Archetype is the presentation profile the assets resolve — "crypt" —
	// REQUIRED non-empty, carried unread, and NEVER a mechanic
	// (rpg-project#256 ruling; [encounter.RegionInput.Archetype]).
	Archetype string `yaml:"archetype"`

	// Lighting is the region's light level. REQUIRED, as is its intensity.
	Lighting *LightingSpec `yaml:"lighting"`

	// Cells is the region's floor as rows of [col,row] pairs. The nesting is
	// for diff-readability only — the builder writes one row per line so a
	// repaint diffs as a line change — and the compiler flattens it. A cell
	// in two regions, or twice in one, fails.
	Cells [][][2]int `yaml:"cells"`
}

// LightingSpec is a region's lighting block: one field today, a block so later
// fields land beside it without reshaping the file.
type LightingSpec struct {
	// Intensity is the light level in [0,1]. REQUIRED — a pointer so a
	// written 0 is distinct from nothing written.
	Intensity *float64 `yaml:"intensity"`
}

// EdgeSpec is one crossing between two adjacent floor cells:
// [[col,row],[col,row]]. Undirected.
type EdgeSpec [2][2]int

// WallSpec is one authored wall entry. Two forms are legal (rpg-project#273):
//
//   - [[5,0],[6,0]]                          # bare edge — default height
//   - { between: [[5,1],[6,1]], height: 2 }  # edge with an authored height
//
// The bare pair stays the common case and means default height; the object
// form exists for the edge that carries more facts than its endpoints. The
// EDGE carries the height rather than some named run because runs are
// DERIVED — order-free, re-derived under editing — and giving them stored
// identity would re-open everything the wall engine's order-invariance
// closed. An edge is the one stable unit of authored fact.
type WallSpec struct {
	// Between is the crossing: two adjacent floor cells,
	// [[col,row],[col,row]], exactly the bare form's pair.
	Between EdgeSpec

	// Height is the authored wall-height MULTIPLIER of the standard
	// rendered wall height, REQUIRED in [1, 3] when authored — raise-only
	// by ruling (rpg-project#273: "I am looking to raise the walls not
	// lower them"), so a waist-high wall cannot be authored at all. Nil
	// means not authored: the standard height, exactly what writing 1.0
	// means — the two are the same fact by design. VISUAL ONLY: a wall
	// blocks movement and sight identically at every height.
	Height *float64
}

// UnmarshalYAML reads either wall form. A sequence node is the bare edge; a
// mapping node is the object form. Unknown keys in the object form are
// refused HERE because a custom unmarshaler bypasses the decoder's
// KnownFields, and a typo silently dropped is exactly what Decode's
// strictness exists to prevent.
func (w *WallSpec) UnmarshalYAML(value *yaml.Node) error {
	if value.Kind == yaml.SequenceNode {
		return value.Decode(&w.Between)
	}
	if value.Kind != yaml.MappingNode {
		return fmt.Errorf("line %d: a wall is an edge [[col,row],[col,row]] or an object {between, height}", value.Line)
	}
	for i := 0; i < len(value.Content); i += 2 {
		switch key := value.Content[i].Value; key {
		case "between", "height":
		default:
			return fmt.Errorf("line %d: field %s not found in type dungeonspec.WallSpec", value.Content[i].Line, key)
		}
	}
	var obj struct {
		Between *EdgeSpec `yaml:"between"`
		Height  *float64  `yaml:"height"`
	}
	if err := value.Decode(&obj); err != nil {
		return err
	}
	if obj.Between == nil {
		return fmt.Errorf("line %d: a wall object must name its edge in `between`", value.Line)
	}
	w.Between = *obj.Between
	w.Height = obj.Height
	return nil
}

// DoorSpec is one door: one state over one or more edges.
type DoorSpec struct {
	// ID names the door within this dungeon; the compiled door is
	// `<key>/<id>`.
	ID string `yaml:"id"`

	// Edges are the crossings the door stands in — at least one. An edge
	// need not sit on a region seam: a door inside a region is legal.
	Edges []EdgeSpec `yaml:"edges"`

	// Locked, when present, makes the door locked behind the check it
	// carries. Nil with Closed false is an open doorway.
	Locked *LockSpec `yaml:"locked,omitempty"`

	// Closed makes the door shut but not locked. Ignored when Locked is
	// set — a locked door is shut by definition.
	Closed bool `yaml:"closed,omitempty"`
}

// LockSpec is the check that opens a locked door. Both fields are carried to
// the composition opaquely — [encounter.Lock] never inspects an ability
// either, because "does a DEX check of 12 succeed" is a rule.
type LockSpec struct {
	DC      int    `yaml:"dc"`
	Ability string `yaml:"ability"`
	Tool    string `yaml:"tool,omitempty"`
}

// PlaceSpec is one authored placement at an ABSOLUTE cell.
type PlaceSpec struct {
	// Ref is content's identifier, "module:type:id". Its TYPE segment routes
	// the placement: "props" becomes a prop, "monsters" becomes a member.
	// Nothing else about it is read.
	Ref string `yaml:"ref"`

	// At is the absolute cell, offset [col,row]. Must be floor, and no two
	// placements may share one.
	At [2]int `yaml:"at"`

	// BlocksMovement is whether a prop stops somebody standing there.
	// REQUIRED on every prop, and REFUSED on anything else — a blocker
	// nobody declared is a wall nobody drew ([encounter.PropInput]).
	BlocksMovement *bool `yaml:"blocks_movement,omitempty"`

	// BlocksLoS is whether a prop stops a sightline. REQUIRED on every prop
	// and refused on anything else, for BlocksMovement's reason.
	BlocksLoS *bool `yaml:"blocks_los,omitempty"`

	// Facing is the prop's authored direction, one of the EIGHT true-compass
	// names — n|ne|e|se|s|sw|w|nw — the SAME eight under BOTH orientations
	// (rpg-project#272, superseding #261's orientation-scoped six-name
	// sets: compass directions live in world space, and a prop must be able
	// to stand squarely against an axis-true wall whatever way the hexes
	// point). A name outside the set is refused by name, never silently
	// snapped to the nearest valid one. Optional: omitted means the asset's
	// own default facing. REFUSED on monsters, for BlocksMovement's reason —
	// a monster faces dynamically in play; authored spawn facing is a shelf
	// item for the Monster AI journey, not smuggled in here.
	Facing string `yaml:"facing,omitempty"`

	// Offset is an authored VISUAL displacement: [x,y] within-cell nudge
	// fractions of the cell size, each REQUIRED in [-0.5, 0.5] when
	// authored (Kirk's ruling: "offset is visual only, agreed" — the prop
	// still occupies its whole cell for movement and LOS), plus an OPTIONAL
	// third component: height above the floor in the same cell-size unit,
	// REQUIRED in [0, 3] when authored and deliberately NOT bound to the
	// planar clamp (rpg-project#272: "height should be able to gun higher
	// than the 5 ticks we allow on x and y"). Omitted means centered on the
	// floor, which [0,0] and [0,0,0] also mean — all the same fact by
	// design. A SLICE, not a fixed-size array, so a list of the wrong
	// length is a validate-level defect naming place[i].offset rather than
	// a decode-time line number. REFUSED on monsters, for Facing's reason.
	//
	// NIL, NOT LEN 0, IS "OMITTED": yaml.v3 leaves this nil when the key is
	// absent but decodes `offset: []` to a non-nil, zero-length slice, and
	// the two are different authored facts — the second is a malformed list
	// somebody wrote, refused the same as any other wrong-length one. Every
	// presence check in validate.go tests the pointer-ness (`!= nil`), never
	// the length, for exactly this reason.
	Offset []float64 `yaml:"offset,omitempty"`

	// Targeting is the author's word for how a monster picks a target.
	// Monsters only. Carried opaquely and never interpreted here.
	Targeting *string `yaml:"targeting,omitempty"`

	// Boss marks the monster whose death ends things. Monsters only, and at
	// most one per region.
	Boss bool `yaml:"boss,omitempty"`
}
