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
// and line of sight, so a sight ray crossed empty canvas freely. That is not a
// decision anybody made; it is what fell out of nobody making one, and it
// decided something anyway: two chambers with a gap between them saw each other
// through the gap unless a complete wall-edge chain had been authored along the
// seam.
//
// So the field says which it is, and Kirk ruled how (2026-08-19): "seems like
// we have some very specific choices and these choices could be configured on
// the canvas." Authored data, required, never defaulted.
//
// # The names say what void DOES, not what it is made of
//
// Kirk again, reviewing this file: "the isrock is very specific do we need to
// call it rock. isBlockedLOS or maybe isblocked." He is right, and the reason
// is the rule this whole composition runs on. Stone, a ship's hull, the air
// over a chasm, the vacuum outside a skyship: those are four fictions and ONE
// mechanic, and the mechanic is all this module can know. Naming the case for
// the material would have been this file holding a fact about a world — the
// same overreach as deciding the default in the first place, one layer up.
//
// So the cases are [VoidOpaque] and [VoidTransparent], for what a sightline does
// when it crosses. The fiction stays in these comments, where it is an
// ILLUSTRATION of a mechanic rather than a definition of one, and in the content
// that authors the field.
//
// He corrected the second name too, and the correction is worth keeping because
// it is a finer point than the first: the pair was briefly VoidOpaque and
// VoidClear, and he said "transparant i think matches opaque better". Both words
// name an effect, so the first rule was already satisfied — what was wrong was
// that "clear" is colloquial where "opaque" is precise, and a sealed set whose
// members are drawn from two registers reads as two separate decisions rather
// than one axis with two ends. Opaque and transparent are the same word's two
// directions.
//
// TWO NAME CORRECTIONS IN ONE REVIEW IS THE PATTERN, not an accident of taste.
// A composition that may not hold fiction has to be read for fiction, and the
// place it hides is in names that feel merely descriptive — which is why both
// of these survived writing, testing and a full mutation battery before anybody
// noticed them.

// CanvasInput is what the field DECLARES about its own canvas — the world facts
// no rule can derive and this module is not allowed to pick.
//
// Two fields, both facts that are true of THIS dungeon and arrive as
// construction data because there is nowhere else they could come from.
// Light did NOT land here: it is a fact about an area, not about the space
// between areas, so it lives on the region ([RegionInput.Lighting],
// rpg-project#256 — closing rpg-toolkit#1113 by relocation).
//
// It is not a place for anything DERIVABLE. The canvas's dimensions and which
// cells are floor: all of those the regions already say, and a field that
// could state them twice would be a field that could state them differently.
type CanvasInput struct {
	// Void is what the space between the regions does to a
	// sightline. REQUIRED — see [Void] for why there is no default to fall
	// back on.
	Void Void

	// Orientation is which way this field's hexes point — REQUIRED. See
	// [Orientation].
	//
	// It sits here rather than on the region because it is a fact about the
	// MAP: every region in one field is painted on one grid, and a field
	// whose regions could disagree about which way its cells point would be
	// a field whose edges do not meet.
	Orientation Orientation
}

// VoidKind names what void does to a sightline, in the form the story and the
// blob carry it. See [Void].
type VoidKind string

const (
	// VoidOpaque stops a sightline: you cannot see across the space between
	// the chambers. Stone is the obvious fiction for it — a tomb cut from a
	// mountain, with what was not cut still there — but a hull, a curtain
	// wall, or a bank of fog are the same mechanic, and this module knows the
	// mechanic.
	VoidOpaque VoidKind = "opaque"

	// VoidTransparent does not: you can see straight across, and still not walk it.
	// An open-air ruin, a ship's deck, a rooftop, the gap over a chasm.
	VoidTransparent VoidKind = "transparent"
)

// Void is what the space between the authored chambers does to a sightline: a
// closed set, sealed the way [DissolveCause] is and for the same reason.
//
// # Why it is declared rather than decided here
//
// "Can you see across the space between two rooms" is not a 5e rule this
// composition could derive. It is a fact about THIS world — a tomb's void is
// opaque, an open-air ruin's or a ship's deck is transparent — and a fact about
// world arrives as construction data. There is no correct default, which is exactly
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
// step into the gap in an open-air ruin any more than into the stone of a tomb —
// [Encounter.Step] and [Encounter.Join] refuse a cell no region owns, and this
// declaration does not touch that. SIGHT is the only thing the declaration
// decides, which is why the cases are named for it.
//
// OPAQUE STOPS A SIGHTLINE EXACTLY AS A WALL DOES, which is the point rather
// than an implementation note: a boundary edge is a hard block on the direct
// canonical ray (tools/spatial's IsLineOfSightBlocked checks boundaries on that
// ray alone, deliberately, and lets occluding ENTITIES be leaned around).
// Opaque void is checked on the same ray, by the same rule, so the edge of the
// floor and a wall somebody drew along it cannot answer differently. That is
// Kirk's ruling on rpg-toolkit#1105 fork 2 — walls are boundary edges — kept
// true for the walls nobody had to draw.
//
// # What it does NOT change
//
// Props and boundaries inside a chamber are untouched: still authored, still
// carrying BlocksMovement and BlocksLineOfSight independently, so a blocker
// somebody placed and a low altar you can see over both stay expressible.
// Kirk's point on the ruling was that both should be deliberate — in his words,
// "if I wanted rock, I could place it and set it to block LoS. I could put an
// obstacle that does not. I can have both but should be deliberate" — and what
// this slice changes is that the DEFAULT is deliberate too.
//
// THAT SENTENCE WAS HALF FALSE WHEN IT WAS WRITTEN, and it is worth saying so
// here rather than quietly correcting it, because of what it took to notice.
// It was true of boundaries, which have carried both flags all along. It was
// false of a chamber's CONTENTS, which were bare cells the module decided for:
// every one blocked sight, none blocked movement, hardcoded. The low altar you
// can see over is the reference tomb's own coffin (`blocks_los: false`), and it
// was precisely the thing that could not be authored — the example the comment
// reached for was the counter-example. Fixed in rpg-toolkit#1128, where a
// chamber's contents became [PropInput] and the promise became true.
//
// The lesson is this file's own, one paragraph up: a composition that may not
// hold fiction has to be READ for it, and what hides is a sentence that
// describes the design correctly while the code does something else. This one
// survived writing, review and a mutation battery — nothing tests a comment.
//
// # Why a sealed set rather than a bool
//
// A bool would answer today's question and could never be grown into the next
// one. A chasm you can see across and FALL INTO is a third case, and it is not
// [VoidTransparent] with a footnote — it is a different thing that happens to share
// one of transparent's two answers while differing on a third nobody has asked
// yet. Sealing the set the way [DissolveCause] is
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

// VoidIsOpaque declares that you cannot see across the space between the
// authored chambers, and cannot walk it either. The reference tomb's answer,
// where the fiction is the stone the chambers were cut from — and equally a
// hull, a curtain wall, or a fog bank, which is why the name is the effect.
//
// A function rather than a package-level variable so nothing can reassign what
// it means at runtime — [ByDecision]'s reasoning, and the save gate's before
// it.
func VoidIsOpaque() Void { return voidOpaque{} }

type voidOpaque struct{}

func (voidOpaque) Kind() VoidKind    { return VoidOpaque }
func (voidOpaque) blocksSight() bool { return true }

// VoidIsTransparent declares that you can see straight across the space between the
// authored chambers, and still cannot walk it. An open-air ruin, a ship's deck,
// a rooftop — somewhere the gap is a drop rather than a barrier.
func VoidIsTransparent() Void { return voidTransparent{} }

type voidTransparent struct{}

func (voidTransparent) Kind() VoidKind    { return VoidTransparent }
func (voidTransparent) blocksSight() bool { return false }

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
	case VoidOpaque:
		return VoidIsOpaque(), nil
	case VoidTransparent:
		return VoidIsTransparent(), nil
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

	// field is the compiled field this canvas was built from — the SAME
	// owner map [Encounter.RegionAt] reads, so asking this room what is
	// floor and asking the encounter are one question — see
	// IsLineOfSightBlocked.
	field *field
}

// IsLineOfSightBlocked reports whether sight between two cells is blocked,
// counting opaque void as the wall it is.
//
// THE VOID IS CHECKED FIRST AND CHECKED HARD. tools/spatial's own rule treats a
// boundary edge as an unconditional block on the direct canonical ray while
// letting an occluding ENTITY be leaned around, because a boundary is a wall
// drawn on an edge with no extent to lean past. Opaque void is that same thing,
// so it is asked the same way: [spatial.CanonicalBoundaryRay] — the very
// rasterization spatial checks boundaries along, not a second one — and any
// cell on it that no region owns stops the line. Sharing the ray is what makes
// the edge of the floor and a wall somebody drew along it evaluate over
// identical cells rather than merely similar ones.
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
// Floor is asked through the compiled field's owner map, the one answer in
// this package to which region holds a cell. Not a copy of the rule and not a
// mask beside it: a second answer to "is this floor" is exactly what region.go
// exists to prevent.
//
// Under [VoidTransparent] this is spatial's answer unchanged, which is the honest
// shape of "the declaration decides": there is nothing to add to a sightline
// crossing a transparent gap. Under opaque on a field with NO void it is spatial's
// answer unchanged too, and for a better reason than speed: a field whose chambers
// tile their canvas exactly has nothing for either declaration to be about, so
// the two mean the same thing and cost the same (fieldHasVoid, pinned by
// TestOpaqueCostsNothingWhereThereIsNoVoid).
//
// WHERE VOID LIES BETWEEN THE CHAMBERS, THIS IS CHEAPER THAN NOT DOING IT —
// measured, because the shape of the cost is not the shape it looks like.
// Scanning the ray is arithmetic, and the spatial call it can skip rasterizes,
// walks boundaries, walks blocking entities and then walks a lean lane per
// neighbour. So finding void EARLY returns before any of that: a twenty-chamber
// square field with gaps refreshed sight 1.7x FASTER under opaque than under
// transparent, and a three-chamber square field 1.2-1.4x faster.
//
// THE PRICE IS PAID BY RAYS THAT NEVER LEAVE THE FLOOR, which run the scan in
// full and then delegate anyway — and since rpg-toolkit#1127 the worst case is
// no longer a contrived fixture. The reference tomb is HEX, and a hex canvas
// holding sheared rectangles is 90.5% void pointy-top, yet opaque measures
// 1.34-1.40x SLOWER on it: a party of eight in three chambers looks mostly at
// people in the same chamber, and none of those rays ever reach the void. That
// is the number to beat if this is ever asked to go faster. (The old worst
// case, one forty-by-forty square chamber with a void margin, sits at
// 1.35-1.43x — the same place, for the same reason.)
//
// Ratios rather than times because the times move with the machine and the
// ratios did not; both are in voidcost_internal_test.go, which is where they
// are re-runnable.
func (c *canvasRoom) IsLineOfSightBlocked(from, to spatial.Position) bool {
	if c.field.hasVoid() && c.field.void.blocksSight() {
		for _, cell := range spatial.CanonicalBoundaryRay(c.GetGrid(), from, to) {
			if _, floor := c.field.regionOf(cell); !floor {
				return true
			}
		}
	}

	return c.BasicRoom.IsLineOfSightBlocked(from, to)
}
