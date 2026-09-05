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
// orientation. [Spec.Scenery] is the floor that belongs to no region
// (rpg-project#360): the same cells, painted with a different brush, that
// nobody stands on. Version 1 is deleted, not supported: a file that says
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

	// Regions are the OWNED floor: their cells' union.
	Regions []RegionSpec `yaml:"regions"`

	// Scenery is floor nobody stands on, as rows of [col,row] pairs — the
	// same encoding and the same nesting-for-diff-readability as
	// [RegionSpec.Cells] (rpg-project#360, wall-geometry design §3.1).
	// Optional; omitted means none. Written after `regions` and before
	// `start`.
	//
	// A CELL BELONGS TO A REGION OR TO THIS LIST, NEVER BOTH. What the two
	// carry is different in kind: a region's cell has an OWNER, which decides
	// visibility, lighting and archetype, and feet may touch it. A scenery
	// cell has neither — it is floor a wall can stand on, a prop can sit on
	// and a sightline crosses, belonging to nobody and standable by nobody.
	// So a cell in both is not a cell with two answers, it is a cell whose
	// author meant two incompatible things, and [Validate] refuses it naming
	// the cell and the region.
	Scenery [][][2]int `yaml:"scenery,omitempty"`

	// Start is where the party comes in, and optionally which way they are
	// looking when they get there. REQUIRED and explicit — version 1 derived
	// it from an archetype, which is the shape of defaulting
	// rpg-toolkit#1033 forbids. See [StartSpec] for the two spellings.
	Start *StartSpec `yaml:"start"`

	// Walls are STRAIGHT LINES between two picked positions, each blocking
	// both movement and sight along everything it stands in the way of
	// (rpg-project#360). The envelope is implied, never written: a crossing
	// from floor into void is a crossing nobody can make, so a wall may run
	// along the edge of the world and cut nothing. See [WallSpec].
	Walls []WallSpec `yaml:"walls"`

	// Doors are one state each at one position on a wall (rpg-toolkit#1123,
	// rpg-project#360). See [DoorSpec].
	Doors []DoorSpec `yaml:"doors"`

	// Place is everything standing on the floor — props, monsters, and the
	// boss alike, routed by the ref's type segment — at ABSOLUTE cells.
	Place []PlaceSpec `yaml:"place"`

	// Intel is the knowledge this dungeon has authored, as records that can
	// be placed in a holder (rpg-project#372, design §2). Optional; omitted
	// means none.
	//
	// A RECORD IS A THING IN THE FILE, like a door. It has an id and it says
	// what it reveals, and it is placed by NAME on whatever carries it
	// ([PlaceSpec.Holds]) — the same shape a door has, declared once and
	// referred to from wherever it is used. That is what makes intel a
	// general builder capability rather than scenario machinery (R3): a
	// scenario binds the nouns its quest needs and never learns the word
	// intel, and a DM may author a record for anything at any time.
	//
	// This replaces `knows`, which named a door directly on a monster.
	// Kirk (R1): "if we don't have a use case for it then it goes. when a
	// use case does arrive for it it will define the shape without any
	// baggage." A record is one indirection and it is the one that buys
	// everything else — a second holder, a second kind of target, a body to
	// read — without a second spelling of knowledge.
	Intel []IntelSpec `yaml:"intel,omitempty"`

	// Factions are the sides this dungeon authored, and Dispositions how
	// they stand to each other (rpg-project#375, the hold-out design §2).
	// Both optional; omitted means the two reserved factions — `party` and
	// `monsters` — and the one default hostility every dungeon had before
	// factions existed. See [FactionSpec] and [DispositionSpec].
	Factions     []FactionSpec     `yaml:"factions,omitempty"`
	Dispositions []DispositionSpec `yaml:"dispositions,omitempty"`

	// Exits are the ways out of this dungeon: an id and a floor cell each
	// (rpg-project#368, design §3.1). Optional; omitted means none.
	//
	// STRUCTURE, NOT SCENARIO. A dungeon has ways out whatever the party is
	// there for, so an exit is authored beside `start` and the two have the
	// same shape. `start` is NOT implicitly an exit: nothing is defaulted
	// (rpg-toolkit#1033), and a dungeon whose entrance is also its way out
	// says so in one line.
	Exits []ExitSpec `yaml:"exits,omitempty"`

	// Scenarios binds this dungeon to the scenarios it is authored for: a
	// map from scenario id to that scenario's bindings, each binding a field
	// key to the id of something in this file (rpg-project#368, design §3.1).
	// Optional; omitted means none.
	//
	// CARRIED OPAQUELY, AND VALIDATED ONLY AS REFERENCES. This package checks
	// exactly one thing about a binding: that its value names a placement id
	// or an exit id that exists in this file. What the keys mean, which are
	// required, and whether the thing named is the right KIND of thing are
	// the scenario package's own refusals — asked at its `New(cfg)`, in
	// form-filler words. Design law C1 is the reason: this package never
	// resolves content, and a scenario is content.
	//
	// A dungeon may bind SEVERAL scenarios, and the run ends when any bound
	// ending fires (design R8).
	Scenarios map[string]map[string]string `yaml:"scenarios,omitempty"`
}

// IntelSpec is one authored piece of knowledge: an id, and what knowing it
// reveals.
//
//	intel:
//	  - id: vault-map
//	    reveals: { door: vault }
//
// The record is what a monster HOLDS ([PlaceSpec.Holds]) and what Loot moves
// off a body. What it reveals is read when it changes hands, not when it is
// placed, which is why the same record can grow a second kind of target
// without anything that carries it changing.
type IntelSpec struct {
	// ID names the record within this dungeon, and is what a holder names.
	// REQUIRED non-empty and unique — refused on collision naming both
	// lines, the same rule a door id and a placement id follow.
	ID string `yaml:"id"`

	// Reveals is what learning this record tells you. REQUIRED, and exactly
	// one target in this cut. See [RevealsSpec].
	Reveals RevealsSpec `yaml:"reveals"`
}

// RevealsSpec is what an intel record tells whoever learns it.
//
// ONE KEY PER USE CASE, and exactly one key set. Today the only one is a
// door, because the only use case anybody has is the way into the vault. A
// region, a location, a lock's approach, a camp's disposition — each is a
// field here, and each arrives WITH the use case that wants it and never
// ahead of it (R5). That is why this is a struct of optional targets rather
// than an open map: a target this build does not understand is a record
// nothing can apply, and it is refused rather than carried hopefully.
type RevealsSpec struct {
	// Door is the id of the door this record reveals the way to. Refused
	// when no door in this dungeon has that id.
	//
	// A DECLARED BUT UNCONCEALED DOOR IS LEGAL AND INERT: revealing the way
	// to a door anyone can already see tells nobody anything, and refusing
	// it would make this declaration depend on a fact about a different one.
	Door string `yaml:"door,omitempty"`

	// Fact is the id of the fact this record reveals (rpg-project#375, the
	// hold-out design §2) — the second key, arrived with its use case: a
	// letter that says the party saved the Wiseman.
	//
	// A PLAIN STRING, DECLARED BY MENTION. Nothing declares a fact; a
	// disposition's `until: { fact: x }` and a record's `reveals: { fact: x }`
	// simply agree on the word. A record may reveal a fact no disposition
	// waits for, and a disposition may wait for a fact no record reveals —
	// the dungeon allows both (pre-release: show the cost) and the SCENARIO
	// refuses the second, because a hold-out nobody can win is its business
	// (R8).
	Fact string `yaml:"fact,omitempty"`
}

// FactionSpec is one faction the dungeon authored: a name members belong
// to, and the member it knows through.
//
//	factions:
//	  - { id: goblins, mind: chief }
//
// `party` is never declared — it is the players' side, reserved — and
// `monsters` is where every monster with no `faction` already is. Declaring
// `monsters` is legal, and is how the unauthored side is given a mind.
type FactionSpec struct {
	// ID names the faction, and is what a placement's `faction`, a
	// disposition's `between` and a scenario's `convince` name. REQUIRED
	// non-empty and unique; `party` is refused by name.
	ID string `yaml:"id"`

	// Mind is the placement the faction knows through — "the faction knows
	// what its mind knows" (design R3). MUST name a MONSTER placement in
	// this faction. Optional: a faction of one has its member as its mind,
	// and a faction of many that waits for a fact and names no mind is
	// refused ("name a mind, or the faction cannot learn").
	Mind string `yaml:"mind,omitempty"`
}

// DispositionSpec is how two factions stand to each other, and what ends
// it.
//
//	dispositions:
//	  - { between: [goblins, party], stance: hostile, until: { fact: saved-wiseman } }
//
// One per unordered pair; a pair nobody declares has a default
// ([encounter.DefaultStance]): `party` is hostile to every faction that did
// not say otherwise, and every other pair is neutral.
type DispositionSpec struct {
	// Between is the pair, unordered. Both MUST exist: a declared faction,
	// or `party` or `monsters`.
	Between [2]string `yaml:"between"`

	// Stance is one of hostile, neutral, allied. REQUIRED.
	Stance string `yaml:"stance"`

	// Until is the predicate that ends hostility; when it holds the stance
	// becomes neutral (R2). LEGAL ONLY WITH `stance: hostile` — a neutral or
	// allied pair has nothing to stop doing. Optional. See [PredicateSpec].
	Until *PredicateSpec `yaml:"until,omitempty"`
}

// ExitSpec is one authored way out: an id and the floor cell a member stands
// on to leave through it.
//
//	exits:
//	  - { id: entrance, at: [1, 3] }
//
// The shape `start` already has, deliberately (design §3.1) — a way out is
// the same kind of authored fact as a way in.
type ExitSpec struct {
	// ID names the exit within this dungeon, and is what a scenario binding
	// names. REQUIRED non-empty and unique — a binding that named an
	// ambiguous exit would have no answer.
	ID string `yaml:"id"`

	// At is the absolute cell, offset [col,row]. Must be STANDABLE floor:
	// an exit nobody can reach is a liveness hole, which is
	// [encounter.ErrNoEnding]'s own reason applied one layer out.
	At [2]int `yaml:"at"`
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

	// Concealed marks the region as hidden space: the room that "appears to
	// be a wall unless it is found" (rpg-project#351 — the room hides with
	// its door). DECLARED HERE, NEVER CASCADED from a concealed door: the
	// room and its door are separate authored facts, moved together by
	// choice — but not moved apart: [Validate] refuses the incoherent
	// combinations (a room only enterable through concealed doors that is
	// not itself concealed, and a concealed room anyone can walk into).
	// Carried unread; what a non-knower's map withholds is the world
	// layer's business.
	Concealed bool `yaml:"concealed,omitempty"`
}

// LightingSpec is a region's lighting block: one field today, a block so later
// fields land beside it without reshaping the file.
type LightingSpec struct {
	// Intensity is the light level in [0,1]. REQUIRED — a pointer so a
	// written 0 is distinct from nothing written.
	Intensity *float64 `yaml:"intensity"`
}

// PositionSpec is ONE OF THE SEVEN POINTS a wall or a door may stand at: the
// midpoint of one of a hex's six sides, or its centre (design §3.3).
//
// A cell and an offset, because a point on a hex grid has no name of its own
// and the author already thinks in cells. The CELL names the frame; it need
// not be floor, and often is not — a wall along the outside of a room is
// written from the cells it walls. The OFFSET picks the point within it, in
// BOUNDING-BOX FRACTIONS: x in widths, east positive; y in heights, south
// positive, the same unit and the same axes a prop's offset uses.
//
// # The set is closed, and it compares exactly
//
// Seven values per orientation and nothing else (F8). Every one is a dyadic
// rational — halves, quarters and eighths — so the file's literals compare as
// exact floats and recognising a position needs no tolerance. An offset
// outside the set is refused naming the wall and the value, never snapped to
// the nearest member: a wall a hand's breadth from where its author put it is
// a dungeon they did not draw.
//
// # The same point, named twice, is a corner
//
// Two positions that name the same physical point are the same point, whether
// or not they name it from the same cell. That is the whole of what a corner
// is (F5): the designer writes a join by writing the same point at both ends,
// and this package has no corner concept and needs none.
type PositionSpec struct {
	// Cell is the hex the point is named from, as an absolute offset
	// [col,row] pair. REQUIRED.
	Cell [2]int `yaml:"cell"`

	// Offset is the point within that cell, in bounding-box fractions.
	// REQUIRED, and one of the seven (§3.3). Written even for the centre —
	// `[0,0]` is a real position and an author who meant it says so, which
	// is what keeps a forgotten key from quietly becoming a thick wall.
	Offset [2]float64 `yaml:"offset"`
}

// StartSpec is where the party comes in: a cell, and optionally the direction
// they are facing when they arrive.
//
// # Two spellings, one meaning
//
// The bare pair is the whole of what a start was before facings existed, and
// it stays legal forever:
//
//	start: [4, 7]
//	start: { at: [4, 7], facing: e }
//
// A dungeon that says nothing about facing is not a dungeon facing north — it
// is a dungeon whose author did not say, and the empty string is that fact
// (zero values tell the truth). Every reference fixture keeps the bare pair,
// and the golden files are byte-unchanged by this addition.
//
// Both spellings are read by hand rather than by a struct tag because a
// scalar and a mapping cannot both satisfy one Go type through the decoder,
// and because a custom unmarshaler bypasses KnownFields — so an unknown key
// has to be refused here, by name, exactly as [PositionSpec] does one type
// over and for that type's stated reason.
type StartSpec struct {
	// At is the absolute [col,row] cell the party arrives on. REQUIRED in
	// both spellings, and it must be floor nobody is standing on — the
	// refusals are unchanged by facing (see validate.go).
	At [2]int

	// Facing is the direction the party is looking on arrival, one of the
	// eight true-compass names props already speak — n|ne|e|se|s|sw|w|nw
	// (rpg-project#272). Empty means the author stated none.
	//
	// PRESENTATION, and the compiler treats it as such: it says which way a
	// camera opens, and nothing anywhere reads it to decide what a member
	// may see, reach or do. A word outside the eight is refused BY NAME
	// rather than snapped to the nearest — the vocabulary rule every
	// authored word in this dialect follows.
	Facing string
}

// UnmarshalYAML reads a start in either spelling — the bare cell pair, or the
// object with an explicit `at` and an optional `facing`.
//
// An unknown key is refused by name for [PositionSpec.UnmarshalYAML]'s
// reason: a custom unmarshaler anywhere above this in the tree bypasses the
// decoder's KnownFields, so a typo would be silently dropped by the very
// strictness Decode exists to provide.
func (s *StartSpec) UnmarshalYAML(value *yaml.Node) error {
	// The bare pair, and the ONLY spelling that existed before facings. A
	// sequence here is unambiguous: an object start is a mapping.
	if value.Kind == yaml.SequenceNode {
		var at [2]int
		if err := value.Decode(&at); err != nil {
			return err
		}
		s.At, s.Facing = at, ""
		return nil
	}
	if value.Kind != yaml.MappingNode {
		return fmt.Errorf(
			"line %d: a start is [col,row] or { at: [col,row], facing: n|ne|e|se|s|sw|w|nw }",
			value.Line)
	}

	var sawAt bool
	for i := 0; i < len(value.Content); i += 2 {
		switch key := value.Content[i].Value; key {
		case "at":
			sawAt = true
		case "facing":
		default:
			return fmt.Errorf("line %d: field %s not found in type dungeonspec.StartSpec",
				value.Content[i].Line, key)
		}
	}
	if !sawAt {
		return fmt.Errorf("line %d: a start does not say which cell the party arrives on", value.Line)
	}

	var obj struct {
		At     [2]int `yaml:"at"`
		Facing string `yaml:"facing"`
	}
	if err := value.Decode(&obj); err != nil {
		return err
	}
	s.At, s.Facing = obj.At, obj.Facing
	return nil
}

// UnmarshalYAML reads a position, refusing an unknown key and a missing one by
// name. Written by hand because a custom unmarshaler anywhere above this in
// the tree bypasses the decoder's KnownFields, and a typo silently dropped is
// exactly what Decode's strictness exists to prevent.
func (p *PositionSpec) UnmarshalYAML(value *yaml.Node) error {
	if value.Kind != yaml.MappingNode {
		return fmt.Errorf("line %d: a position is { cell: [col,row], offset: [x,y] }", value.Line)
	}
	var sawCell, sawOffset bool
	for i := 0; i < len(value.Content); i += 2 {
		switch key := value.Content[i].Value; key {
		case "cell":
			sawCell = true
		case "offset":
			sawOffset = true
		default:
			return fmt.Errorf("line %d: field %s not found in type dungeonspec.PositionSpec",
				value.Content[i].Line, key)
		}
	}
	if !sawCell {
		return fmt.Errorf("line %d: a position does not say which cell it is named from", value.Line)
	}
	if !sawOffset {
		return fmt.Errorf(
			"line %d: a position does not say its offset within the cell (the centre is [0,0], written out)",
			value.Line)
	}
	var obj struct {
		Cell   [2]int     `yaml:"cell"`
		Offset [2]float64 `yaml:"offset"`
	}
	if err := value.Decode(&obj); err != nil {
		return err
	}
	p.Cell, p.Offset = obj.Cell, obj.Offset

	return nil
}

// WallSpec is one authored wall: A STRAIGHT LINE BETWEEN TWO POSITIONS, and
// the file holds nothing else (design §1.5, F3).
//
//	walls:
//	  - start: { cell: [1, 2],  offset: [-0.25, -0.375] }
//	    end:   { cell: [1, 10], offset: [-0.25,  0.375] }
//	    height: 2
//	    name: the long seam
//
// The crossings it blocks, the cells it passes through and which of those it
// leaves too little of to stand on are ALL DERIVED (§4.2) and never written.
// Kirk: "edges just need the starting and ending hex coordinate… not only be
// easy to read it would be easy to edit."
//
// # The pair form is deleted, not deprecated
//
// The old dialect wrote a wall as the list of crossings it blocked —
// `[[5,0],[6,0]]`, `{between: …}`, `{edges: […]}` — which made a wall a
// bookkeeping exercise in what a line already says, and made a wall standing
// against the void unsayable. A file that uses it is refused by name (F4),
// with no migration: legacy dungeons are deleted and re-authored (C17).
//
// # Thin and thick are not in here
//
// A line along a row of side midpoints shaves a twenty-fourth off its
// neighbours; the one through their centres seals them. Those are COSTS, shown
// by the designer at pick time, not kinds of wall (F16a). This struct knows
// seven positions and twelve directions, and the compiler knows the area rule.
// There is no lower-level decision to relax.
type WallSpec struct {
	// Start and End are the wall's two ends. REQUIRED, both.
	Start PositionSpec `yaml:"start"`
	End   PositionSpec `yaml:"end"`

	// Height is the authored wall-height MULTIPLIER of the standard rendered
	// wall height, REQUIRED in [1, 3] when authored — raise-only by ruling
	// (rpg-project#273: "I am looking to raise the walls not lower them"), so
	// a waist-high wall cannot be authored at all. Nil means not authored:
	// the standard height, exactly what writing 1.0 means. VISUAL ONLY: a
	// wall blocks movement and sight identically at every height.
	Height *float64 `yaml:"height"`

	// Name is the wall's display name, for the human reading the file and the
	// errors about it: "north wall" beats "walls[7]" for the streamers this
	// dialect is authored by. Carried, and read by nothing but a refusal.
	Name string `yaml:"name"`
}

// UnmarshalYAML reads a wall, refusing the deleted pair form by name.
//
// Unknown keys are refused HERE because a custom unmarshaler bypasses the
// decoder's KnownFields, and a typo silently dropped is exactly what Decode's
// strictness exists to prevent. `edges` and `between` are called out
// separately from any other unknown key: a file carrying them is not a typo,
// it is last dialect's dungeon, and saying so is the difference between an
// author who knows what to do and one who does not.
func (w *WallSpec) UnmarshalYAML(value *yaml.Node) error {
	if value.Kind == yaml.SequenceNode {
		return fmt.Errorf(
			"line %d: %s", value.Line, pairFormRefusal)
	}
	if value.Kind != yaml.MappingNode {
		return fmt.Errorf(
			"line %d: a wall is { start: {cell, offset}, end: {cell, offset} }, with an optional height and name",
			value.Line)
	}
	var sawStart, sawEnd bool
	for i := 0; i < len(value.Content); i += 2 {
		switch key := value.Content[i].Value; key {
		case "edges", "between":
			return fmt.Errorf("line %d: %s", value.Content[i].Line, pairFormRefusal)
		case "start":
			sawStart = true
		case "end":
			sawEnd = true
		case "height", "name":
		default:
			return fmt.Errorf("line %d: field %s not found in type dungeonspec.WallSpec",
				value.Content[i].Line, key)
		}
	}
	if !sawStart || !sawEnd {
		return fmt.Errorf("line %d: a wall runs from `start` to `end`, and this one does not say both", value.Line)
	}
	var obj struct {
		Start  PositionSpec `yaml:"start"`
		End    PositionSpec `yaml:"end"`
		Height *float64     `yaml:"height"`
		Name   string       `yaml:"name"`
	}
	if err := value.Decode(&obj); err != nil {
		return err
	}
	w.Start, w.End, w.Height, w.Name = obj.Start, obj.End, obj.Height, obj.Name

	return nil
}

// pairFormRefusal is the one sentence a last-dialect file gets, wherever it is
// met (F4, F12). It names the form, says what replaced it, and stops — there
// is no migration to offer.
const pairFormRefusal = "`edges` is the deleted pair form: a wall is now a line, " +
	"`start` and `end`, each a cell and one of the seven offsets, and a door is `at` one position on it"

// DoorSpec is one door: A POSITION ON A WALL, plus the state it is in.
//
//	doors:
//	  - id: crypt-door
//	    at: { cell: [1, 6], offset: [-0.25, 0.375] }
//	    closed: true
//
// The door opens the ONE crossing of the side it is the midpoint of (F11), and
// exactly one wall must pass through it (F10). A wider doorway is two doors.
type DoorSpec struct {
	// ID names the door within this dungeon; the compiled door is
	// `<key>/<id>`.
	ID string `yaml:"id"`

	// At is where the door stands: one position, which must be a SIDE
	// MIDPOINT rather than a centre — a door in the middle of a hex opens no
	// crossing — and must have exactly one wall through it. REQUIRED.
	At PositionSpec `yaml:"at"`

	// Locked, when present, makes the door locked behind the check it
	// carries. Nil with Closed false is an open doorway.
	//
	// NIL, NOT LEN 0, IS "NOT LOCKED" — [PlaceSpec.Offset]'s law: yaml.v3
	// leaves this nil when the key is absent but decodes `locked: []` to a
	// non-nil, zero-length list, and the second is an authored lock that
	// forgot to say how it is beaten — refused at validate rather than
	// silently read as an open doorway.
	Locked CheckSpec `yaml:"locked,omitempty"`

	// Closed makes the door shut but not locked. Ignored when Locked is
	// set — a locked door is shut by definition.
	Closed bool `yaml:"closed,omitempty"`

	// Concealed, when present, hides the door behind the find check it
	// carries — e.g. Perception 15 or Investigation 12. It COMPOSES with
	// plain, closed, or locked underneath: what a door is doing and whether
	// anyone knows it is there are two separate authored facts
	// (rpg-project#350). Nil vs empty is Locked's law: `concealed: []` is a
	// door hidden with no way to ever find it, refused at validate by name.
	Concealed CheckSpec `yaml:"concealed,omitempty"`
}

// UnmarshalYAML reads a door, refusing the deleted pair form by name (F12) and
// any unknown key, for [WallSpec.UnmarshalYAML]'s reason.
func (d *DoorSpec) UnmarshalYAML(value *yaml.Node) error {
	if value.Kind != yaml.MappingNode {
		return fmt.Errorf("line %d: a door is { id, at: {cell, offset} } with an optional state", value.Line)
	}
	var sawAt bool
	for i := 0; i < len(value.Content); i += 2 {
		switch key := value.Content[i].Value; key {
		case "edges", "between":
			return fmt.Errorf("line %d: %s", value.Content[i].Line, pairFormRefusal)
		case "at":
			sawAt = true
		case "id", "locked", "closed", "concealed":
		default:
			return fmt.Errorf("line %d: field %s not found in type dungeonspec.DoorSpec",
				value.Content[i].Line, key)
		}
	}
	if !sawAt {
		return fmt.Errorf("line %d: a door does not say where it stands (`at`, one position on a wall)", value.Line)
	}
	var obj struct {
		ID        string       `yaml:"id"`
		At        PositionSpec `yaml:"at"`
		Locked    CheckSpec    `yaml:"locked"`
		Closed    bool         `yaml:"closed"`
		Concealed CheckSpec    `yaml:"concealed"`
	}
	if err := value.Decode(&obj); err != nil {
		return err
	}
	d.ID, d.At, d.Locked, d.Closed, d.Concealed = obj.ID, obj.At, obj.Locked, obj.Closed, obj.Concealed

	return nil
}

// knowsRefusal is the deleted `knows` key's sentence, pointing at what
// replaced it (R1). The same shape [pairFormRefusal] takes for the deleted
// wall pair form: a key this build once understood is refused BY NAME with
// the migration in the message, never as a bare unknown field, because the
// author who wrote it had a meaning and deserves to be told where it went.
const knowsRefusal = "`knows` is gone: a monster holds an intel RECORD now — declare it under " +
	"`intel:` with an id and what it reveals, then put its id in this placement's `holds:`"

// UnmarshalYAML reads a placement, refusing the deleted `knows` key by name
// (R1) and every unknown key, for [WallSpec.UnmarshalYAML]'s reason.
func (pl *PlaceSpec) UnmarshalYAML(value *yaml.Node) error {
	if value.Kind != yaml.MappingNode {
		return fmt.Errorf("line %d: a placement is { ref, at } plus what it declares", value.Line)
	}
	for i := 0; i < len(value.Content); i += 2 {
		switch key := value.Content[i].Value; key {
		case "knows":
			return fmt.Errorf("line %d: %s", value.Content[i].Line, knowsRefusal)
		case "id", "ref", "at", "blocks_movement", "blocks_los", "facing",
			"offset", "targeting", "boss", "holds", "holdable", "faction":
		default:
			return fmt.Errorf("line %d: field %s not found in type dungeonspec.PlaceSpec",
				value.Content[i].Line, key)
		}
	}
	// A plain alias so decoding the body does not re-enter this method.
	type placeBody PlaceSpec
	var obj placeBody
	if err := value.Decode(&obj); err != nil {
		return err
	}
	*pl = PlaceSpec(obj)

	return nil
}

// CheckSpec is one authored check: the accepted approaches through it, AT
// LEAST ONE, success by any listed one (ruled on rpg-project#350 — a locked
// door is forced with Strength or picked with Dexterity and tools; a
// concealed door is spotted with Perception or reasoned out with
// Investigation). The whole list is carried to the composition opaquely, for
// the reason the single-approach lock was: "does a DEX check of 12 succeed"
// is a rule, and this package never learns what "perception" means either.
//
// A bare list rather than a wrapping object, deliberately: the builder
// authors a check as approach rows, and the YAML reads as the rows it is.
//
// This is the generalized [DoorSpec] lock shape — LockSpec's single
// dc/ability/tool became one entry in this list, in place, free under the
// pin system pre-adoption.
type CheckSpec []ApproachSpec

// ApproachSpec is one accepted route through a check: an ability or skill,
// maybe a tool, and the DC that route must beat. Every field is carried
// uninterpreted; the DC is priced PER APPROACH, not per check — forcing the
// door and picking its lock need not cost the same.
type ApproachSpec struct {
	// Ability is the opaque rulebook ref this approach rolls — "str",
	// "dex", "perception", "investigation". REQUIRED non-empty, and never
	// inspected here.
	Ability string `yaml:"ability"`

	// Tool is the opaque item ref for a named tool, e.g.
	// "dnd5e:item:thieves-tools". Optional: empty means the approach names
	// none — the reference tomb's lock does not.
	Tool string `yaml:"tool,omitempty"`

	// DC is what this route must beat. REQUIRED at least 1 — a check with
	// dc 0 is what an undeclared one would look like.
	DC int `yaml:"dc"`
}

// PlaceSpec is one authored placement at an ABSOLUTE cell.
type PlaceSpec struct {
	// ID is the author's name for this placement — the third id in this
	// file after a region's and a door's (rpg-project#368, design P2).
	// Optional: a placement nothing binds to needs no name.
	//
	// REQUIRED BY WHATEVER BINDS TO IT, and by nothing else. A knowledge
	// link's SUBJECT needs none — `knows` names doors, and a monster that
	// knows one is just a monster. A scenario binding needs one, and a
	// holdable prop needs one because both the binding and the `held` beat
	// have to be able to say WHICH thing. Refused on collision, naming both
	// lines.
	ID string `yaml:"id,omitempty"`

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
	//
	// UNTOUCHED BY THIS SLICE and deliberately so (design R8): endings come
	// from scenarios, and the named follow-up converts the reference tomb to
	// the kill-the-captain scenario and deletes this flag then. A dungeon
	// that binds a scenario AND flags a boss has two endings; that is the
	// author's business and it is visible on the form.
	Boss bool `yaml:"boss,omitempty"`

	// Holds is the intel records this placement carries, by record id
	// (rpg-project#372, design §2). Optional; legal on MONSTERS AND PROPS
	// (R6), refused only when the id names no record.
	//
	// A MONSTER CARRIES IT FROM SPAWN. The captain is not a role: it is a
	// monster holding a record, and nothing in the game needs the word
	// captain.
	//
	// A PROP CARRIES IT UNTIL SOMEBODY PICKS IT UP. Kirk, walking the
	// stack: "tech could get intel by holding something too. if we want to
	// test the intel we need to be able to place it on a few things — not
	// the hardest monster to kill in the game." A scroll on a table is the
	// obvious shape, and it is also the whole hold-out's-letter idea with
	// nothing extra: the record stays ON the prop, so whoever holds the prop
	// next is taught in turn.
	//
	// Refused by name when no record has that id.
	//
	// THE SAME RECORD MAY BE HELD BY SEVERAL MONSTERS, because intel copies
	// rather than moving (holdings.go): two guards may both know the way in,
	// and looting either teaches it. That is the difference between a record
	// and a physical thing, and it is why this is not refused as a duplicate.
	Holds []string `yaml:"holds,omitempty"`

	// Holdable is whether a member can pick this prop up (design §5).
	// Optional; PROPS ONLY, and refused on a monster.
	//
	// A POINTER, for [PlaceSpec.BlocksMovement]'s reason one type over: an
	// omitted `holdable` and an authored `holdable: false` are the same fact
	// here — a thing nobody declared holdable stays scenery — but the
	// pointer is what lets the refusal below distinguish "said nothing" from
	// "said false" on a monster, so a monster that wrote `holdable: false`
	// is told it cannot declare that at all rather than silently accepted.
	//
	// A HOLDABLE PROP MUST HAVE AN ID, refused otherwise: the scenario
	// binding names it and the `held` beat names it, and neither can name a
	// thing with no name.
	Holdable *bool `yaml:"holdable,omitempty"`

	// Faction is the faction this monster belongs to (rpg-project#375, the
	// hold-out design §2). MONSTERS ONLY, refused on a prop; MUST name a
	// declared faction (or `monsters`, which it is anyway); absent means the
	// reserved `monsters` faction. `party` is refused — that is the players'
	// side. Carried to the host as [MonsterPlacement.Faction], which hands
	// it to [encounter.MemberInput.Faction] when it spawns the sheet.
	Faction string `yaml:"faction,omitempty"`
}
