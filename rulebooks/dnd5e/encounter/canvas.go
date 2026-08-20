// Copyright (C) 2026 Kirk Diggler
// SPDX-License-Identifier: GPL-3.0-or-later

package encounter

import (
	"fmt"

	"github.com/KirkDiggler/rpg-toolkit/core"
	"github.com/KirkDiggler/rpg-toolkit/tools/spatial"
)

// canvas.go is THE MAP, HANDED OUT TO BE READ (rpg-toolkit#1114).
//
// Since #1106 this composition has held exactly one map and published only
// DESCRIPTIONS of it: [Encounter.Atlas]'s regions and walls, a [Member]'s cell,
// [Encounter.RegionAt]'s name for a cell. Each is correct and none of them is
// the map, so a caller with a question the descriptions do not answer — "is
// there a wall between these two cells" — had to build a map out of them.
//
// That reconstruction is a second implementation of the field's geometry, and
// rpg-toolkit#1090's slice measured what it costs. The one that exists today,
// in `rulebooks/dnd5e/resolution`, carries its own copy of grid construction
// with a comment promising it "tracks encounter.buildRoomGrid rather than
// choosing for itself" — and has no walls and no props in it at all, so the
// first rule to ask it about line of sight would be told nothing blocks while
// this module says otherwise. A promise in a comment is not a mechanism.
//
// So the map is handed out instead of described. What that costs is stated in
// [Encounter.Canvas]'s own doc: it is the LIVE room, so it goes out behind a
// view that refuses every write.

// Canvas returns the map this encounter runs on, to READ.
//
// One spatial room spanning the whole dungeon, in dungeon-absolute cells — the
// canvas the authored rooms compiled into (#1106), with every wall registered
// as an absolute boundary edge and every prop and member placed on it. It
// answers what a room answers: where somebody stands, what the distance between
// two cells is, whether a sightline is blocked, who is within a radius.
//
// # It is the live map, and that is the point
//
// Not a snapshot. A member who steps is at their new cell the next time this
// room is asked, through a value obtained before the step. A copy would be
// cheaper to reason about and would be the wrong thing: the caller this exists
// for installs it as the world an interaction's rules read positions out of,
// and a rule reading a stale world gives a WRONG answer rather than no answer.
//
// This is the one place this module hands out live internal state rather than
// copying out, and it is deliberate rather than an oversight in a module whose
// every other read is copy-out ([Encounter.Atlas]'s "every returned slice is
// freshly allocated per call"). The difference is that those describe the map
// and this IS the map; there is no version of "the same world, copied" that
// stays the same world.
//
// AND A FAITHFUL COPY IS NOT MERELY WORSE, IT IS NOT AVAILABLE. Entities and
// their cells can be read back off a [spatial.Room], but its REGISTERED
// BOUNDARIES cannot: nothing on Room or [spatial.BoundaryAwareRoom] lists them,
// and the only way to ask is GetBoundary, one adjacent pair at a time — so
// recovering a canvas's walls means probing every adjacent pair on it, through
// an interface this deliberately does not return (see below). Any snapshot
// built with what a caller actually has is therefore a room with NO WALLS,
// which is exactly the defect this exists to end, reintroduced one layer down.
// Pinned by TestTheCanvasIsTheLiveMapNotACopy, and measured: the snapshot
// mutant fails that test AND the sight test, and the sight failure is the one
// that matters.
//
// # Which is why it refuses to be written to
//
// Placing, moving or removing an entity here would move a member behind every
// verb's back: no sight refresh, no beat in the story, and a blob that
// disagrees with the world the next load builds. So the returned room refuses
// all three with ErrReadOnly, NAMING the method refused.
//
// Refusing rather than quietly doing nothing is the whole design of it. A no-op
// mutator reports success and changes nothing, which is the defect shape this
// composition has spent several slices deleting — a silent fallback that makes
// broken wiring look like working wiring. A caller that genuinely needs to move
// somebody has [Encounter.Step] and [Encounter.Join], which are the verbs that
// know what else has to happen.
//
// The type system would say this better than a runtime error does, and it
// cannot yet: the consumer seam is `gamectx.WithRoom`, which takes a full
// `spatial.Room`, so anything handed to it must carry the mutators whether or
// not they can be honoured. Narrowing that seam to a read-only interface is a
// change to a third module and is worth doing; until then the refusal is loud,
// immediate, and asserted in canvasread_test.go.
//
// # Boundaries are readable, not registrable
//
// The returned value is a [spatial.Room] and NOT a [spatial.BoundaryAwareRoom],
// which is a decision rather than an omission. That interface's writing half —
// RegisterBoundary, RemoveBoundary — is exactly what a read-only view must not
// offer, and its reading half is already answered here: IsLineOfSightBlocked
// consults the registered boundaries, and a caller that wants the walls
// themselves has [Encounter.Atlas], which reports every one of them in absolute
// space and in construction terms rather than a pair at a time. Adding the
// interface to withhold half of it would be offering a door in order to lock
// it.
//
// # It answers about the void too
//
// IsLineOfSightBlocked here is the CANVAS's answer, which since
// rpg-toolkit#1116 counts the field's declared void: on a field whose void is
// opaque, a sightline crossing the space between two chambers is blocked, and a
// caller reading this map is told so. That is the whole reason the declaration
// lives on the map rather than in the sight loop — see [canvasRoom].
//
// Returns ErrNoField when there is no canvas to hand out. Construction forbids
// that — both seams compile one or fail — so it is reachable only through the
// zero value, and a nil [spatial.Room] is not an absent answer but a wrong one
// that panics at the first read. [Encounter.Grid] refuses on the same grounds.
func (e *Encounter) Canvas() (spatial.Room, error) {
	if e.canvas == nil {
		return nil, fmt.Errorf("canvas: %w", ErrNoField)
	}

	return readOnlyRoom{canvas: e.canvas}, nil
}

// readOnlyRoom is the live canvas with its three mutators refusing.
//
// Every read delegates to the room itself rather than to a copy of it, which is
// what makes [Encounter.Canvas] live. Two things were checked before passing
// them straight through, rather than assumed: spatial's own GetAllEntities and
// GetEntitiesAt build fresh containers, so neither hands back a map or slice
// that writes through to the room's own; and every method on [spatial.Grid] is
// a query (GetShape, IsValidPosition, GetDimensions, Distance, GetNeighbors,
// IsAdjacent, GetLineOfSight, GetPositionsInRange), so handing out the grid
// hands out no way to change it.
//
// The pass-throughs behave EXACTLY as the room does, which is the invariant
// worth stating because it also decides what not to do. CanPlaceEntity takes an
// entity and spatial dereferences it without a nil check — measured:
// BasicRoom.CanPlaceEntity(nil, an occupied cell) panics, and on an empty cell
// it returns true because the loop never runs. That is spatial's behaviour, not
// this view's, and guarding it HERE would make the view answer differently from
// the room it stands in front of. The mutators are the only place this type
// decides anything, and they are the only place it guards anything.
type readOnlyRoom struct {
	canvas *canvasRoom
}

// readOnlyRoom must be a full spatial.Room: gamectx.WithRoom takes one.
var _ spatial.Room = readOnlyRoom{}

// PlaceEntity refuses. Use [Encounter.Join], which also refreshes sight and
// writes the beat.
//
// A NIL ENTITY IS REFUSED RATHER THAN DEREFERENCED. [spatial.BasicRoom] answers
// a nil entity with "entity cannot be nil", so a view that panicked on one
// would be strictly worse than the room it stands in front of — and a view of a
// read-only thing has no business being the harder of the two to call. What is
// refused here does not depend on the entity, so the answer is the same
// refusal, with the nil named rather than dereferenced.
//
// This is the ONLY mutator that can meet a nil: MoveEntity and RemoveEntity
// take an entity ID by string, so there is nothing to dereference in either.
// Checked rather than fixed here and left to be discovered there.
func (r readOnlyRoom) PlaceEntity(entity core.Entity, pos spatial.Position) error {
	id := "<nil>"
	if entity != nil {
		id = entity.GetID()
	}

	return fmt.Errorf("canvas: PlaceEntity(%q, (%g,%g)): %w", id, pos.X, pos.Y, ErrReadOnly)
}

// MoveEntity refuses. Use [Encounter.Step], which decides what a step is
// allowed to cross and reports what happened.
func (r readOnlyRoom) MoveEntity(entityID string, newPos spatial.Position) error {
	return fmt.Errorf("canvas: MoveEntity(%q, (%g,%g)): %w", entityID, newPos.X, newPos.Y, ErrReadOnly)
}

// RemoveEntity refuses. Use [Encounter.Exit], which records where they stood on
// the way out.
func (r readOnlyRoom) RemoveEntity(entityID string) error {
	return fmt.Errorf("canvas: RemoveEntity(%q): %w", entityID, ErrReadOnly)
}

func (r readOnlyRoom) GetID() string            { return r.canvas.GetID() }
func (r readOnlyRoom) GetType() core.EntityType { return r.canvas.GetType() }
func (r readOnlyRoom) GetGrid() spatial.Grid    { return r.canvas.GetGrid() }

func (r readOnlyRoom) GetEntitiesAt(pos spatial.Position) []core.Entity {
	return r.canvas.GetEntitiesAt(pos)
}

func (r readOnlyRoom) GetEntityPosition(entityID string) (spatial.Position, bool) {
	return r.canvas.GetEntityPosition(entityID)
}

func (r readOnlyRoom) GetAllEntities() map[string]core.Entity {
	return r.canvas.GetAllEntities()
}

func (r readOnlyRoom) GetEntitiesInRange(center spatial.Position, radius float64) []core.Entity {
	return r.canvas.GetEntitiesInRange(center, radius)
}

func (r readOnlyRoom) IsPositionOccupied(pos spatial.Position) bool {
	return r.canvas.IsPositionOccupied(pos)
}

func (r readOnlyRoom) CanPlaceEntity(entity core.Entity, pos spatial.Position) bool {
	return r.canvas.CanPlaceEntity(entity, pos)
}

func (r readOnlyRoom) GetPositionsInRange(center spatial.Position, radius float64) []spatial.Position {
	return r.canvas.GetPositionsInRange(center, radius)
}

func (r readOnlyRoom) GetLineOfSight(from, to spatial.Position) []spatial.Position {
	return r.canvas.GetLineOfSight(from, to)
}

func (r readOnlyRoom) IsLineOfSightBlocked(from, to spatial.Position) bool {
	return r.canvas.IsLineOfSightBlocked(from, to)
}
