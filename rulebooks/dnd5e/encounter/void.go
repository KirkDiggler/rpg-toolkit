// Copyright (C) 2026 Kirk Diggler
// SPDX-License-Identifier: GPL-3.0-or-later

package encounter

import (
	"fmt"

	"github.com/KirkDiggler/rpg-toolkit/tools/spatial"
)

// void.go is THE CANVAS DECLARES WHAT VOID IS (rpg-toolkit#1116).
//
// The canvas spans the field's whole bounding BOX, and the authored chambers
// are only the part of it somebody drew. Everything else — the gap between two
// chambers, the strip beside a short one, the margin outside them all — is
// void: on the map, and owned by nobody.
//
// Until this file, void was UNWALKABLE BUT TRANSPARENT. [Encounter.Step]
// refused a cell no region owns while percept building checked only distance
// and line of sight, so a sight ray crossed empty canvas as if it were open
// air. That is neither rock nor sky. It is what fell out of nobody deciding,
// and it decided something: two chambers with a gap between them saw each other
// through solid nothing unless a complete wall-edge chain had been authored
// along the seam.
//
// So the field says which it is, and Kirk ruled how (2026-08-19): "seems like
// we have some very specific choices and these choices could be configured on
// the canvas." Authored data, required, never defaulted.

// CanvasInput is what the field DECLARES about its own canvas — the world facts
// no rule can derive and this module is not allowed to pick.
//
// A struct with one field today, and a struct on purpose. "Is the space between
// the chambers stone or sky" is the first of a species: facts that are true of
// THIS dungeon, that arrive as construction data because there is nowhere else
// they could come from. AMBIENT LIGHT IS THE NEXT ONE (rpg-toolkit#1113) —
// "this dungeon is dark" is the same kind of sentence as "this dungeon is cut
// out of rock", and it belongs beside it rather than in a second mechanism
// invented for it. That is the slot this type exists to hold open.
//
// It is not a place for anything DERIVABLE. The canvas's dimensions, its grid
// family, which cells are floor: all of those the authored rooms already say,
// and a field that could state them twice would be a field that could state
// them differently.
type CanvasInput struct {
	// Void is what the space between the authored chambers is made of.
	// REQUIRED — see [Void] for why there is no default to fall back on.
	Void Void
}

// VoidKind names what void is made of, in the form the story and the blob carry
// it. See [Void].
type VoidKind string

const (
	// VoidRock is stone: the chambers were cut out of a mountain, and what
	// was not cut is still there.
	VoidRock VoidKind = "rock"

	// VoidOpen is sky: the chambers stand in the open, and what is not floor
	// is the air above a courtyard, a deck, or a rooftop.
	VoidOpen VoidKind = "open"
)

// Void is what the space between the authored chambers is made of: a closed
// set, sealed the way [DissolveCause] is and for the same reason.
//
// # Why it is declared rather than decided here
//
// "Is the space between two rooms stone or sky" is not a 5e rule this
// composition could derive. It is a fact about THIS world — a tomb's void is
// rock, an open-air ruin's or a ship's deck is sky — and a fact about the world
// arrives as construction data. There is no correct default, which is exactly
// what rpg-toolkit#1033's capabilities-supplied-never-defaulted law is about:
// a default here would be this module quietly deciding what a dungeon is made
// of, in a field the author never wrote. So [CanvasInput.Void] is required, and
// a field without one is refused at construction (ErrNoField) rather than
// silently given the answer that happened to be cheapest.
//
// # What each answer does, and what neither does
//
// Both are UNWALKABLE, and that half does not vary: void is not floor under any
// declaration, because floor is what the authored chambers own. A member cannot
// step into the gap in an open-air ruin any more than into the rock of a tomb —
// [Encounter.Step] and [Encounter.Join] refuse a cell no region owns, and this
// declaration does not touch that. What varies is what SIGHT does with it:
// [VoidRock] stops a sightline, [VoidOpen] does not.
//
// Rock stops it EXACTLY AS A WALL DOES, which is the point rather than an
// implementation note: a boundary edge is a hard block on the direct canonical
// ray (tools/spatial's IsLineOfSightBlocked checks boundaries on that ray alone,
// deliberately, and lets occluding ENTITIES be leaned around). Rock void is
// checked on the same ray, by the same rule, so "the rock face at the edge of
// the room" and "a wall somebody drew there" cannot answer differently. That is
// Kirk's ruling on rpg-toolkit#1105 fork 2 — walls are boundary edges — kept
// true for the walls nobody had to draw.
//
// # What it does NOT change
//
// Occluders and boundaries inside a chamber are untouched: still authored,
// still carrying BlocksMovement and BlocksLineOfSight independently, so "rock I
// placed" and "a low altar you can see over" both stay expressible. Kirk's
// point on the ruling was that both should be deliberate — "if I wanted rock, I
// could place it and set it to block LoS. I could put an obstacle that does
// not. I can have both but should be deliberate" — and what this slice changes
// is that the DEFAULT is deliberate too.
//
// # Why a sealed set rather than a bool
//
// A bool would answer today's question and could never be grown into the next
// one. A chasm you can see across and fall into is a third case, and it is not
// "transparent" with a footnote — it is a different thing that happens to share
// one of transparency's answers. Sealing the set the way [DissolveCause] is
// sealed makes that structural: the unexported method means a third case cannot
// be declared outside this package, so adding one means editing this file, and
// editing this file means having the caller that forces it in hand. It also
// makes the zero value USELESS rather than plausible — a nil Void is obviously
// absent, where a false bool is a legal-looking answer nobody wrote.
//
// The set is deliberately not asked whether void is WALKABLE. Both cases answer
// the same way today, so a predicate for it would be a branch no field could
// take and no test could pin — it lands with the case that needs it.
type Void interface {
	// Kind names which void this is: the word the blob carries, and the word
	// an error quotes back.
	Kind() VoidKind

	// blocksSight reports whether a sightline crossing void is stopped by it.
	//
	// Unexported, which SEALS the set — see the type's godoc. It is the only
	// question this module asks a Void, so the sealing method is a real one
	// rather than a marker: a third case cannot be added without answering
	// it.
	blocksSight() bool
}

// VoidIsRock declares that the space between the authored chambers is stone:
// opaque, and not floor. The reference tomb's answer, and every dungeon cut out
// of a mountain.
//
// A function rather than a package-level variable so nothing can reassign what
// it means at runtime — [ByDecision]'s reasoning, and the save gate's before
// it.
func VoidIsRock() Void { return voidRock{} }

type voidRock struct{}

func (voidRock) Kind() VoidKind    { return VoidRock }
func (voidRock) blocksSight() bool { return true }

// VoidIsOpen declares that the space between the authored chambers is sky:
// transparent, and still not floor. An open-air ruin, a ship's deck, a rooftop —
// somewhere you can see clear across the gap and still cannot walk out over it.
func VoidIsOpen() Void { return voidOpen{} }

type voidOpen struct{}

func (voidOpen) Kind() VoidKind    { return VoidOpen }
func (voidOpen) blocksSight() bool { return false }

// voidFromData resolves the persisted word back to the declaration it names.
//
// Two refusals, both by name and both loud, per the standing precedent
// (rpg-toolkit#1053/#1068: fail loudly, no migration). An ABSENT word is a blob
// written before the field declared anything, and loading it under a guess
// would put a party in a dungeon whose walls the host never authored. An
// UNKNOWN word is a blob from a dialect this build does not speak — a kind
// somebody added and this binary has never heard of — and the honest answer to
// that is not to pick the nearest one.
func voidFromData(name string) (Void, error) {
	switch VoidKind(name) {
	case VoidRock:
		return VoidIsRock(), nil
	case VoidOpen:
		return VoidIsOpen(), nil
	case "":
		return nil, fmt.Errorf("field does not say what its void is (canvas.void): %w", ErrNoField)
	default:
		return nil, fmt.Errorf("field declares void %q, which this build does not know (canvas.void): %w", name, ErrNoField)
	}
}

// canvasRoom is the canvas, plus what the field declared its void to be.
//
// The declaration lives ON THE MAP rather than in the sight loop, and that is
// the load-bearing choice in this slice. rpg-toolkit#1118 hands the canvas out
// to be READ — a rule installed on it asks it about line of sight directly — so
// a void rule that lived in [Encounter.rebuildPercepts] would tell that caller
// nothing stands between two cells while this module refuses to let them see
// each other. Two answers to one question is the defect #1118 exists to end,
// and it would have been reintroduced one layer down. Pinned by
// TestTheHandedOutCanvasAnswersTheSameWay.
//
// It EMBEDS the room rather than reimplementing it: every other read, every
// mutator, and the grid itself are spatial's, unchanged. One method is
// overridden, and it is the one the declaration is about.
type canvasRoom struct {
	*spatial.BasicRoom

	void Void

	// rooms and grids are the SAME slices and maps the encounter holds, so
	// asking this room what is floor and asking [Encounter.RegionAt] are one
	// question — see IsLineOfSightBlocked.
	rooms []RoomInput
	grids map[string]spatial.Grid

	// hasVoid is whether this field has any cell no chamber owns — see
	// fieldHasVoid. Purely a cost decision: where there is no void, rock and
	// open MEAN THE SAME THING, and the scan below can only ever run to the
	// end of the ray and find nothing.
	hasVoid bool
}

// fieldHasVoid reports whether any cell of the canvas belongs to no chamber.
//
// ARITHMETIC, NOT ENUMERATION, and that distinction is the whole reason this is
// allowed to exist. W2 makes the chambers disjoint and W6 makes them fit, so
// the union of their footprints has exactly the cell count their sum does, and
// the canvas is void-free precisely when that sum fills it. Both grid families
// count the same way: an AxialHexGrid of Width x Height holds Width*Height
// integer (Q,R) pairs, exactly as a square grid holds Width*Height cells.
//
// THIS IS NOT THE FLOOR MASK, which Kirk's ruling (4) on rpg-toolkit#1105
// deferred until a caller forces one. A mask is a per-cell structure with two
// unanswered questions riding on it — computed per load or persisted, and
// whether it belongs in tools/spatial — and this is one derived bit, computed
// from numbers already in hand, never stored and never persisted. The measured
// case for it: on a twenty-chamber field whose rooms tile their canvas exactly,
// a sight refresh under rock cost 109 ms against open's 26 ms, all of it spent
// proving there was no void to find. With this it is open's number, because
// there is nothing to look for.
func fieldHasVoid(rooms []RoomInput, width, height int) bool {
	var floor int64
	for _, r := range rooms {
		floor += int64(r.Width) * int64(r.Height)
	}

	return int64(width)*int64(height) != floor
}

// IsLineOfSightBlocked reports whether sight between two cells is blocked,
// counting rock void as the wall it is.
//
// THE ROCK IS CHECKED FIRST AND CHECKED HARD. tools/spatial's own rule treats a
// boundary edge as an unconditional block on the direct canonical ray while
// letting an occluding ENTITY be leaned around, because a boundary is a wall
// drawn on an edge with no extent to lean past. A rock face is that same thing,
// so it is asked the same way: [spatial.CanonicalBoundaryRay] — the very
// rasterization spatial checks boundaries along, not a second one — and any
// cell on it that no region owns stops the line. Sharing the ray is what makes
// "the rock at the edge of the chamber" and "a wall somebody drew there"
// evaluate over identical cells rather than merely similar ones.
//
// That is a guarantee about the CODE, and no fixture found here can stand in
// for it — stated because it was measured rather than assumed. Over eight void
// layouts (a gap column, offset and ragged rooms, an L, a checkerboard, a
// staircase, comb teeth, and a hex pair; 340 floor cells) the raw
// GetLineOfSight visits a DIFFERENT SET of cells depending on which end it is
// asked from in roughly a quarter of ordered pairs — Bresenham's known
// direction dependence, alive on square grids and absent on hex — and yet the
// "does this ray cross void" answer came out the same both ways for every
// single pair. So swapping the canonical ray for the raw one is a mutant no
// test in this package kills. It is still wrong: the canonicalization is what
// keeps this rule pinned to the boundary rule as spatial's rasterization
// changes, rather than pinned to it by coincidence today.
//
// Floor is asked through [regionAt], the one function in this package that
// turns an absolute cell into an owner. Not a copy of the rule and not a mask
// beside it: a second answer to "is this floor" is exactly what region.go
// exists to prevent, and rpg-toolkit#1105's ruling (4) deferred the floor mask
// until a caller forces it — this one does not, because the question already
// has an implementation.
//
// Under [VoidOpen] this is spatial's answer unchanged, which is the honest
// shape of "the declaration decides": there is nothing to add to a sightline
// crossing open sky. Under rock on a field with NO void it is spatial's answer
// unchanged too, and for a better reason than speed: a field whose chambers
// tile their canvas exactly has nothing for either declaration to be about, so
// the two mean the same thing and cost the same (fieldHasVoid, pinned by
// TestRockCostsNothingWhereThereIsNoVoid).
//
// WHERE THERE IS VOID, THIS IS USUALLY CHEAPER THAN NOT DOING IT — measured,
// because the shape of the cost is not the shape it looks like. Scanning the
// ray is arithmetic, and the spatial call it can skip rasterizes, walks
// boundaries, walks blocking entities and then walks a lean lane per neighbour.
// So finding rock EARLY returns before any of that: on a twenty-chamber field
// with gaps, a refresh cost 17 ms under rock against 28 ms under open, and the
// reference tomb's shape 46 us against 77 us. The price is paid by rays that
// never leave the floor, which run the scan in full and then delegate anyway:
// one forty-by-forty chamber with a void margin and forty members measured
// 3.7 ms against 2.7 ms. That is the honest worst case, and it is the one to
// beat if a caller ever forces the floor mask.
func (c *canvasRoom) IsLineOfSightBlocked(from, to spatial.Position) bool {
	if c.hasVoid && c.void.blocksSight() {
		for _, cell := range spatial.CanonicalBoundaryRay(c.GetGrid(), from, to) {
			if _, floor := regionAt(c.rooms, c.grids, cell); !floor {
				return true
			}
		}
	}

	return c.BasicRoom.IsLineOfSightBlocked(from, to)
}
