// Copyright (C) 2026 Kirk Diggler
// SPDX-License-Identifier: GPL-3.0-or-later

package resolution

import (
	"fmt"
	"math"

	"github.com/KirkDiggler/rpg-toolkit/core"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/encounter"
	"github.com/KirkDiggler/rpg-toolkit/tools/spatial"
)

// world.go is ONE WORLD, ALWAYS INSTALLED (rpg-toolkit#1114, closing #1090).
//
// There used to be a question here — WHICH room describes this interaction —
// and an answer this package gave when it could not decide: none. That answer
// was the bug. A cast is the whole encounter roster, deliberately, because
// "deciding they are irrelevant would be this package deciding a rule"
// (ADR-0038); so the moment one party member wandered off, the roster spanned
// two rooms, no room was installed, and every predicate that reads positions
// went quiet. A prone target in the attacker's own cell conferred nothing. In
// the reference tomb — a party spread across a dungeon — that was the NORMAL
// state, not an edge case.
//
// The question is gone rather than answered better. Encounter v0.18.0 made the
// field ONE canvas: rooms are how a dungeon is AUTHORED, and what they compile
// into is a single map in a single absolute frame (rpg-toolkit#1105, Kirk's
// rulings 1 and 3). There is one world, so there is nothing to choose between,
// and this installs it every time.

// placedMember is a member as the spatial room needs it: an ID at a position.
// The room stores entities, and nothing here reads anything else off them.
type placedMember struct {
	id string
}

func (m placedMember) GetID() string            { return m.id }
func (m placedMember) GetType() core.EntityType { return "member" }

// blocker is an occluder as the spatial room needs it: a cell that stops a
// sightline. It carries no identity a rule can read — [placedMember] is what a
// predicate looks somebody up by, and these are map furniture.
//
// IT STOPS SIGHT AND NOT MOVEMENT, which is encounter's occluderEntity exactly
// and is not the obvious pair of answers. Spatial refuses to place an entity on
// a cell held by a movement blocker, so an occluder that blocked movement here
// would make this package REFUSE a world the encounter loaded happily — the
// moment somebody stood on one, which the encounter permits and which
// [encounter.Encounter.RegionAt] treats as an ordinary cell of its region.
// Occlusion is about sightlines, not walkability (#929 T3 ruling 1).
type blocker struct {
	id string
}

func (b blocker) GetID() string            { return b.id }
func (b blocker) GetType() core.EntityType { return "occluder" }
func (b blocker) GetSize() int             { return 1 }
func (b blocker) BlocksLineOfSight() bool  { return true }
func (b blocker) BlocksMovement() bool     { return false }

// interactionRoom builds THE CANVAS — the one map this interaction happens on —
// and places the cast on it.
//
// ALWAYS. There is no case in which this returns no room: a saving throw with
// nobody on the map gets an empty canvas rather than a nil one, because "no
// geometry was needed" and "no geometry was available" are different sentences
// and only the second one is a defect. The predicates that read this ask about
// named members and answer "nobody knows where these two are standing" when a
// member is not placed (conditions.ProneCondition's attackerIsWithinReach), so
// an empty canvas says exactly that, and a spread party no longer says it about
// two creatures nose to nose.
//
// # It is built from what the encounter itself reports, not from the blob
//
// Every coordinate here comes back out of the loaded encounter in DUNGEON-
// ABSOLUTE space, already projected through its authored rooms' anchors by the
// same arithmetic the encounter enforces with: [encounter.Encounter.Atlas]
// reports each region's absolute footprint, each occluder's absolute cell and
// each wall's two absolute endpoints, and [encounter.Encounter.Members] reports
// each member's absolute cell. So the projection is not duplicated — it is
// CONSUMED. What is left for this file to decide is the canvas's span, which is
// arithmetic over values that are already absolute, and which nothing reads:
// the predicates measure distances and ask about walls, and neither depends on
// where the grid's bounds fall.
//
// That is a deliberate step back from what this used to do. It used to
// reconstruct a room out of EncounterData — "the same bytes the encounter
// itself loads from, rather than asking the encounter for a runtime object it
// does not expose" — and carry its own copy of the encounter's grid
// construction to do it, with a comment promising the two would be kept in
// step. A promise in a comment is not a mechanism; the encounter now publishes
// the projected geometry, so the copy is down to the span and the family, and
// TestTheInstalledWorldAgreesWithTheEncounter checks even that against the
// encounter's own answers rather than asserting it.
//
// # Positions are as-of-load
//
// Nothing moves during an interaction — movement is the walk machine's, and it
// is a different interaction — so a canvas built once at the start stays true
// for the whole call. A live-truth projection, for when an interaction does
// move somebody, is #964's question.
func interactionRoom(enc *encounter.Encounter, participants []Participant) (spatial.Room, error) {
	atlas, err := enc.Atlas()
	if err != nil {
		return nil, fmt.Errorf("%w: %w", ErrBadWorld, err)
	}

	shape, err := enc.Grid()
	if err != nil {
		return nil, fmt.Errorf("%w: %w", ErrBadWorld, err)
	}

	canvas, err := compileCanvas(shape, atlas)
	if err != nil {
		return nil, err
	}

	wanted := make(map[string]struct{}, len(participants))
	for _, p := range participants {
		if id := p.ID(); id != "" {
			wanted[id] = struct{}{}
		}
	}

	members, err := enc.Members()
	if err != nil {
		return nil, fmt.Errorf("%w: %w", ErrBadWorld, err)
	}

	for _, member := range members {
		if _, ok := wanted[string(member.ID)]; !ok {
			continue
		}

		if err := canvas.PlaceEntity(placedMember{id: string(member.ID)}, member.Position); err != nil {
			return nil, fmt.Errorf("%w: place %q: %w", ErrBadWorld, member.ID, err)
		}
	}

	return canvas, nil
}

// compileCanvas draws the atlas's absolute geometry onto one spatial room.
//
// The grid spans the field's whole absolute footprint, which is the span the
// encounter's own canvas has — deliberately, and it is the one number here that
// is not simply read off the atlas. A SMALLER grid would answer line of sight
// differently at its edges, because spatial filters a sightline's rasterized
// cells through IsValidPosition, so a ray that leaves the bounds is not the ray
// the encounter traced. Matching the span is what makes "the same walls" mean
// "the same answers".
//
// Occluders arrive as cells and walls as edges, both already absolute
// ([encounter.AtlasRegion]'s doc comment: the boundaries it reports are
// projected "by the SAME projection compileCanvas registers them by, so what a
// host draws and what the encounter enforces are the same edges"). An endpoint
// may belong to the region next door — that is how a wall between two chambers
// is expressible at all — which is a second reason the canvas must span the
// whole field rather than any one region.
func compileCanvas(shape spatial.GridShape, atlas encounter.Atlas) (*spatial.BasicRoom, error) {
	if len(atlas.Regions) == 0 {
		return nil, fmt.Errorf("%w: the field has no regions", ErrBadWorld)
	}

	width, height, err := canvasSpan(shape, atlas.Regions)
	if err != nil {
		return nil, err
	}

	canvas := spatial.NewBasicRoom(spatial.BasicRoomConfig{
		ID:   "canvas",
		Type: "encounter",
		Grid: buildGrid(shape, width, height),
	})

	for regionIdx, region := range atlas.Regions {
		// Occluder IDs are index-based, exactly as the encounter's are: a
		// region ID is an arbitrary string and a cell-derived key collides on
		// legal fields, but a pair of slice indices cannot.
		for occIdx, cell := range region.Occluders {
			id := fmt.Sprintf("occluder-%d-%d", regionIdx, occIdx)
			if perr := canvas.PlaceEntity(blocker{id: id}, cell); perr != nil {
				return nil, fmt.Errorf("%w: occluder %s: %w", ErrBadWorld, id, perr)
			}
		}

		for _, wall := range region.Boundaries {
			if berr := canvas.RegisterBoundary(spatial.Boundary{
				From:              wall.From,
				To:                wall.To,
				BlocksMovement:    wall.BlocksMovement,
				BlocksLineOfSight: wall.BlocksLineOfSight,
			}); berr != nil {
				return nil, fmt.Errorf("%w: wall (%g,%g)-(%g,%g): %w",
					ErrBadWorld, wall.From.X, wall.From.Y, wall.To.X, wall.To.Y, berr)
			}
		}
	}

	return canvas, nil
}

// canvasSpan returns the Width/Height a grid of this family needs to hold every
// region's absolute footprint — the field's own bounding box.
//
// THE TWO FAMILIES ANCHOR DIFFERENTLY, and that difference is the whole rule. A
// square grid is the half-open rectangle [0,Width) x [0,Height): it starts at
// the origin and cannot be moved, so a field reaching a negative cell has no
// square grid to draw it on. A hex grid is origin-CENTERED, its span is
// [ceil(-dim/2), ceil(dim/2)-1], and widening it always reaches further both
// ways — so a hex field always fits and this never rejects one.
//
// A field that cannot be drawn is refused rather than shrunk to fit, and it can
// only be reached by a world the encounter itself refused to build (encounter's
// W6 runs the same check at both of its construction seams), so this is a
// defect report about the caller's data rather than a case play can produce.
func canvasSpan(shape spatial.GridShape, regions []encounter.AtlasRegion) (width, height int, err error) {
	qMin, qMax, rMin, rMax := fieldBounds(shape, regions)

	if shape == spatial.GridShapeHex {
		// half = dim/2, min = ceil(-half), max = ceil(half)-1. dim = 2*m gives
		// min = -m and max = m-1, so m must cover both ends.
		return 2 * max(-qMin, qMax+1), 2 * max(-rMin, rMax+1), nil
	}

	if qMin < 0 || rMin < 0 {
		return 0, 0, fmt.Errorf(
			"%w: the field's absolute footprint reaches cell (%d,%d), which no square grid can hold",
			ErrBadWorld, qMin, rMin)
	}

	return qMax + 1, rMax + 1, nil
}

// fieldBounds is the union of every region's absolute footprint. Requires at
// least one region, which compileCanvas checks before calling.
func fieldBounds(shape spatial.GridShape, regions []encounter.AtlasRegion) (qMin, qMax, rMin, rMax int) {
	qMin, qMax, rMin, rMax = regionBounds(shape, regions[0])
	for _, region := range regions[1:] {
		q0, q1, r0, r1 := regionBounds(shape, region)
		qMin, qMax = min(qMin, q0), max(qMax, q1)
		rMin, rMax = min(rMin, r0), max(rMax, r1)
	}

	return qMin, qMax, rMin, rMax
}

// regionBounds is one region's absolute bounding box: its local cell bounds
// offset by the absolute anchor the atlas reports. Origins are integral by the
// encounter's own construction check, so the truncation is exact.
func regionBounds(shape spatial.GridShape, region encounter.AtlasRegion) (qMin, qMax, rMin, rMax int) {
	localQMin, localQMax := axisBounds(shape, region.Width)
	localRMin, localRMax := axisBounds(shape, region.Height)
	oq, or := int(region.Origin.X), int(region.Origin.Y)

	return localQMin + oq, localQMax + oq, localRMin + or, localRMax + or
}

// axisBounds is one axis's cell interval for a span of dim, in the family's own
// frame: square counts up from zero, hex is centered on it.
func axisBounds(shape spatial.GridShape, dim int) (minCell, maxCell int) {
	if shape == spatial.GridShapeHex {
		half := float64(dim) / 2

		return int(math.Ceil(-half)), int(math.Ceil(half)) - 1
	}

	return 0, dim - 1
}

// buildGrid makes a grid of the field's own family. Square is the default for
// the same reason the encounter's is: hex is the family that has to be asked
// for.
func buildGrid(shape spatial.GridShape, width, height int) spatial.Grid {
	if shape == spatial.GridShapeHex {
		return spatial.NewAxialHexGrid(spatial.AxialHexGridConfig{
			SpanWidth:  float64(width),
			SpanHeight: float64(height),
		})
	}

	return spatial.NewSquareGrid(spatial.SquareGridConfig{
		Width:  float64(width),
		Height: float64(height),
	})
}
