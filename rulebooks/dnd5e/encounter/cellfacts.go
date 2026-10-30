// Copyright (C) 2026 Kirk Diggler
// SPDX-License-Identifier: GPL-3.0-or-later

package encounter

import (
	"fmt"

	"github.com/KirkDiggler/rpg-toolkit/tools/spatial"
)

// Passage is what a cell allows a mover to do with it — the three-valued
// answer 5e's own movement rules need, not a boolean.
type Passage int

const (
	// PassageBlocked means the mover may not enter the cell at all.
	PassageBlocked Passage = iota

	// PassagePassThrough means the mover may cross the cell but may not stop
	// on it — a nonhostile creature's space (2014 PHB, "Moving Around Other
	// Creatures").
	PassagePassThrough

	// PassageStandable means the mover may enter the cell and end a step
	// there.
	PassageStandable
)

// ContribKind names WHAT KIND of thing contributed a fact about a cell. It is
// how a debug feed or an intel record says why a cell answered the way it did
// without the reader having to re-derive it.
type ContribKind string

const (
	// ContribField is the compiled field itself: a cell no region owns, or
	// one a wall has sealed.
	ContribField ContribKind = "field"

	// ContribProp is a placed prop whose author declared BlocksMovement.
	ContribProp ContribKind = "prop"

	// ContribMember is a creature standing on the cell, folded by its stance
	// to the mover.
	ContribMember ContribKind = "member"

	// ContribDoor is a SHUT DOOR standing as a footprint over the cell
	// (rpg-project#485, R1). Its own kind rather than a prop's, because the
	// difference is the whole point: a prop is a fact about the dungeon and
	// a shut door is a thing with a state, which is the part a caller can do
	// something about (rpg-toolkit#1123's lesson, one noun over). An edge
	// door never appears here — it closes a CROSSING, not a cell, and the
	// crossing fold is where it is answered.
	ContribDoor ContribKind = "door"
)

// ContribRef is one contributor to a cell fact.
//
// Two identifiers because two readers want different ones. ID is WHICH ONE —
// the member's id, or the prop's placement id — and is what a record names
// when it has to point back at a specific thing; it is empty for
// [ContribField], which is the map itself and not a placed thing. Ref is WHAT
// IT IS, content's own identifier (`dnd5e:props:pillar`), carried so a refusal
// can say the word "pillar" to whoever asked; it is empty for everything but a
// prop, because nothing else on a cell has a content ref this module holds.
// A PLACED FOOTPRINT carries its own id in both, honestly: it has no content
// ref beyond the name its author gave the placement (placed_props.go).
type ContribRef struct {
	Kind ContribKind
	ID   string
	Ref  string

	// Blocks is whether THIS contributor is why the cell is closed to the
	// mover. A nonhostile creature contributes a fact about the cell —
	// something is standing there — without being a reason not to enter it,
	// so a refusal that named it would be the story lying about which thing
	// stopped the step.
	Blocks bool
}

// CellAtInput asks what one cell allows one mover.
type CellAtInput struct {
	// Cell is the absolute cell being asked about.
	Cell spatial.Position

	// Mover is whose question it is. A cell answers differently for
	// different movers: a creature is an obstacle or a doorway depending on
	// whether it is an enemy. A mover's own cell never blocks it.
	Mover MemberID
}

// CellFact is the fold: everything standing on a cell, combined by policy.
type CellFact struct {
	// Passage is the combined answer. Blocked beats PassThrough beats
	// Standable — the most restrictive contributor wins, which is the only
	// combination rule that cannot let a mover through something.
	Passage Passage

	// Cost is what entering costs, in cells. Always 1 today: nothing on the
	// map yet declares difficult ground, and a cost nobody can vary would be
	// a knob pretending to be a rule.
	Cost int

	// Contribs is who said so, in no promised order.
	Contribs []ContribRef
}

// CellAt folds the sources a step has always consulted — the compiled field's
// sealed and unowned cells, the props placed on it, and the members standing
// on it by their stance to the mover — into ONE answer.
//
// THE POINT IS THAT THERE IS ONE (rpg-toolkit#1652). Before this, [Encounter.Step]
// asked the field about standability while the monster route asked only about
// region ownership and wall edges, so the route could hand back a path through
// a pillar that the step then refused, and the monster stood still. A route and
// a step that read the same fold cannot disagree; that is the whole fix, and
// the reason this is a fold rather than a blocked-set parameter threaded into
// one searcher.
//
// DERIVED ON EVERY CALL AND NEVER STORED. A cell's facts are made of live
// placement, and a cached copy would be a second answer of exactly the kind
// this exists to remove.
//
// The creature rows are the 2014 rules as written: you may move through a
// nonhostile creature's space but not stop there, and a hostile creature's
// space is closed to you while it is up. A downed monster permits standing.
// Participation failures are returned, never converted to a passage value.
// (The size-difference exception waits for size to
// exist on a member — [memberEntity.GetSize] is a hardcoded 1.)
func (e *Encounter) CellAt(in CellAtInput) (CellFact, error) {
	q, err := e.cellQuery()
	if err != nil {
		return CellFact{}, fmt.Errorf("cell at: %w", err)
	}
	return q.cellAt(in)
}

// cellQuery is a bounded read. Placement remains live; participation is assessed
// once for this read only. Never retain a query across a step or reaction.
//
// FLOODS ARE MEMOIZED FOR THE READ (rpg-toolkit#2097). Every input a flood
// reads (positions, doors, placements, who is down, each walker's picture)
// changes only between reads, never inside one, so a flood asked twice from
// the same origin for the same mover, walker and limit is the same field. A
// monster's view asks it once per creature it sees; the read answers the
// first ask and hands the rest the same field. The memo dies with the query,
// which is why the rule above (never retain one across a step) is also the
// memo's: a step moves creatures, even within one Step, as world pace drives
// them, so a memo that outlived its read would route on stale ground.
type cellQuery struct {
	*Encounter
	down map[MemberID]bool

	// floods is this read's memo of goal-free floods, keyed by everything a
	// flood without a forgiveness predicate reads from its caller.
	floods map[floodKey]spatial.FieldOutput

	// locksYield makes a LOCKED door one a creature's walk would open, in
	// this read only. It exists for one question,
	// [Encounter.lockedDoorInTheWay] — "would the way be open but for a
	// lock?" — and no walk is ever planned or taken on a read that sets it.
	// A fresh read is made for that question, so its floods are never shared
	// with one that routes.
	locksYield bool

	// cells and crossings are this read's memo of the two answers every
	// flood edge asks — what a cell allows a mover ([cellQuery.cellAt]) and
	// what closes the crossing between two cells ([cellQuery.crossingBlocked])
	// — under the same promise the flood memo rests on: nothing moves inside
	// a read. A flood asks a cell once from each neighbour and a crossing
	// once from each end, and a creature's view floods the same floor more
	// than once, so a route through every room of a castle (rpg-project#715)
	// pays for each answer once.
	cells     map[CellAtInput]CellFact
	crossings map[[2]spatial.Position]crossingAnswer
}

// crossingAnswer is one memoized crossing fold: what closes it, if anything.
type crossingAnswer struct {
	prop    PropID
	blocked bool
}

// crossingBlocked is [Encounter.crossingBlocked] memoized for the read. A
// crossing is undirected — a boundary is, and so is the segment a footprint
// is traced across — so one answer serves both directions. An error is not
// memoized; it fails the read that met it.
func (e *cellQuery) crossingBlocked(from, to spatial.Position) (PropID, bool, error) {
	key := [2]spatial.Position{from, to}
	if to.X < from.X || (to.X == from.X && to.Y < from.Y) {
		key = [2]spatial.Position{to, from}
	}
	if a, ok := e.crossings[key]; ok {
		return a.prop, a.blocked, nil
	}
	prop, blocked, err := e.Encounter.crossingBlocked(from, to)
	if err != nil {
		return prop, blocked, err
	}
	if e.crossings == nil {
		e.crossings = map[[2]spatial.Position]crossingAnswer{}
	}
	e.crossings[key] = crossingAnswer{prop: prop, blocked: blocked}

	return prop, blocked, nil
}

// floodKey is everything a goal-free flood reads from its caller.
type floodKey struct {
	mover, walker MemberID
	from          spatial.Position
	limit         int
	opens         bool
}

func (e *Encounter) cellQuery() (*cellQuery, error) {
	down, err := e.downNow()
	if err != nil {
		return nil, &passageReadError{cause: err}
	}
	return &cellQuery{Encounter: e, down: down}, nil
}

func (e *cellQuery) cellAt(in CellAtInput) (CellFact, error) {
	if fact, ok := e.cells[in]; ok {
		return fact, nil
	}
	fact, err := e.computeCellAt(in)
	if err != nil {
		return fact, err
	}
	if e.cells == nil {
		e.cells = map[CellAtInput]CellFact{}
	}
	e.cells[in] = fact

	return fact, nil
}

// computeCellAt is [cellQuery.cellAt] without the read's memo.
func (e *cellQuery) computeCellAt(in CellAtInput) (CellFact, error) {
	fact := CellFact{Passage: PassageStandable, Cost: 1}

	if !e.field.isStandable(in.Cell) {
		fact.Passage = PassageBlocked
		fact.Contribs = append(fact.Contribs, ContribRef{Kind: ContribField, Blocks: true})
	}

	// PLACED FOOTPRINTS, CENTRE-COVERED (issue #1753). A movement-blocking
	// placement closes every cell whose centre it covers — stationary
	// [spatial.TraceFootprint] contact, boundary included — no matter where
	// its rectangle reaches. Every covering contributor is reported: two
	// overlapping tables do not erase each other's fact.
	if len(e.field.placed) > 0 {
		centre := e.field.plane.CellCentre(in.Cell)
		now := e.field.placedNow()
		for i := range e.field.placed {
			p := &e.field.placed[i]
			if !p.blocksMovement {
				continue
			}
			// ONE ANSWER WITH [field.standingBlocks] (rpg-toolkit#1854): a
			// placement in reserve or in somebody's hands closes no cell, and
			// the fold that says so is the same one the step's own refusal
			// reads — a router that saw a table nobody can walk into any more
			// would hand back a path around nothing.
			placement, standing := now.stands(p)
			if !standing || !p.reach.mayTouch(placement.Origin, centre, centre) {
				continue
			}
			contact, err := spatial.TraceFootprint(spatial.FootprintTraceInput{
				Placement: placement, From: centre, To: centre,
			})
			if err != nil {
				return CellFact{}, fmt.Errorf("cell footprint %q: %w", p.id, err)
			}
			if contact.Contact {
				fact.Passage = PassageBlocked
				fact.Contribs = append(fact.Contribs, ContribRef{
					Kind: ContribProp, ID: string(p.id), Ref: string(p.id), Blocks: true,
				})
			}
		}
	}

	// A DOOR THAT STANDS AS A FOOTPRINT (rpg-project#485, R1), by the same
	// centre-covered rule and reported as the DOOR it is. It is here rather
	// than only in the step's own refusal because this fold is what a route
	// reads: a router that could not see a shut door would hand back a path
	// through it and the step would refuse — rpg-toolkit#1652's defect, in
	// the one noun that can change mid-scene.
	//
	// An OPEN door contributes nothing at all, not a non-blocking row: what
	// the map reports about a doorway somebody walked through is what it
	// reported before the door existed.
	if door := e.field.doorStandingOn(in.Cell); door != nil {
		fact.Passage = PassageBlocked
		fact.Contribs = append(fact.Contribs, ContribRef{
			Kind: ContribDoor, ID: string(door.id), Ref: string(door.id), Blocks: true,
		})
	}

	for _, ent := range e.canvas.GetEntitiesAt(in.Cell) {
		switch v := ent.(type) {
		case *propEntity:
			if !v.BlocksMovement() {
				continue
			}
			fact.Passage = PassageBlocked
			fact.Contribs = append(fact.Contribs, ContribRef{
				Kind: ContribProp, ID: v.GetID(), Ref: v.ref, Blocks: true,
			})
		case *memberEntity:
			if MemberID(v.id) == in.Mover {
				continue // a mover is not its own obstacle
			}
			passage := occupantPassage(occupantFacts{
				kind: v.kind, down: e.down[MemberID(v.id)],
				blocks: v.BlocksMovement(), hostile: e.opposed(in.Mover, MemberID(v.id)),
			})
			blocks := passage == PassageBlocked
			if passage < fact.Passage {
				fact.Passage = passage
			}

			fact.Contribs = append(fact.Contribs, ContribRef{
				Kind: ContribMember, ID: v.id, Blocks: blocks,
			})
		}
	}

	return fact, nil
}

// StaticPlacementError identifies a placement rejected by static field facts.
// At remains in the encounter field's coordinate frame; callers that retain
// an authoring frame should use the source coordinate for presentation.
type StaticPlacementError struct {
	Index  int
	At     spatial.Position
	Reason string
	Cause  error
}

// Error returns the detailed placement refusal in the field coordinate frame.
func (e *StaticPlacementError) Error() string {
	return fmt.Sprintf("placement[%d] at [%g,%g] %s: %v", e.Index, e.At.X, e.At.Y, e.Reason, e.Cause)
}

// Unwrap exposes the underlying placement sentinel.
func (e *StaticPlacementError) Unwrap() error { return e.Cause }

// ValidateStaticPlacements validates authored placement cells against the
// compiled field and static contributors without inventing a live member.
//
// It compiles the field on every call. A caller asking about many cells of
// one field — a seat search over a whole floor — compiles once with
// [CompileStaticPlacements] and asks [StaticPlacements.Validate] per cell
// instead; the answers are the same ones, by construction, because this is
// that pair (rpg-toolkit#2047).
func ValidateStaticPlacements(in FieldInput, cells []spatial.Position) error {
	s, err := CompileStaticPlacements(in)
	if err != nil {
		return err
	}
	return s.Validate(cells)
}

// StaticPlacements is a field compiled ONCE for the static placement
// question [ValidateStaticPlacements] asks, so the question can be put to as
// many cells as a caller has without recompiling the field for each one
// (rpg-toolkit#2047: a seat search over an open floor recompiled it per
// candidate cell, which made a compile quadratic in floor hexes).
//
// It is a snapshot of the input it was compiled from. The field is compiled
// into its own storage, and the shut footprint doors — read from the INPUT,
// for the reason [StaticPlacements.Validate] gives — are copied out at
// compile time, so a caller editing its [FieldInput] afterwards changes
// nothing here. A changed input is a new compile.
type StaticPlacements struct {
	f         *field
	shutDoors []staticDoor
}

// staticDoor is one door that occupies its cells in its authored state, held
// by value so the snapshot owns it.
type staticDoor struct {
	id        DoorID
	placement spatial.FootprintPlacement
}

// CompileStaticPlacements compiles a field for repeated static placement
// questions. It refuses exactly what [ValidateStaticPlacements] refuses
// before it reaches a cell: whatever the field compile refuses.
func CompileStaticPlacements(in FieldInput) (*StaticPlacements, error) {
	f, err := compileField(in)
	if err != nil {
		return nil, err
	}
	s := &StaticPlacements{f: f}
	for _, d := range in.Doors {
		if d.Placement == nil || d.Placement.Footprint.Box == nil || d.State == nil || !d.State.blocks() {
			// A door with no box is a field construction refuses
			// outright (validateDoorInputs), so there is nothing
			// measurable here and nothing to be closed about.
			continue
		}
		s.shutDoors = append(s.shutDoors, staticDoor{id: d.ID, placement: *copyPlacement(d.Placement)})
	}
	return s, nil
}

// Validate validates authored placement cells against the compiled field and
// static contributors without inventing a live member. The answer for each
// cell is the one [ValidateStaticPlacements] gives for the same input; the
// first refused cell is reported with its index in cells.
func (s *StaticPlacements) Validate(cells []spatial.Position) error {
	f := s.f
	for i, authored := range cells {
		cell := f.cellAt(authored)
		if !f.isStandable(cell) {
			return &StaticPlacementError{Index: i, At: authored, Reason: "is not standable", Cause: ErrBadPlacement}
		}
		for _, p := range f.props {
			if p.BlocksMovement != nil && *p.BlocksMovement && f.cellAt(p.At) == cell {
				return &StaticPlacementError{Index: i, At: authored, Reason: fmt.Sprintf("is occupied by prop %q", p.Ref), Cause: ErrBadPlacement}
			}
		}
		centre := f.plane.CellCentre(cell)
		for _, p := range f.placed {
			if !p.blocksMovement {
				continue
			}
			contact, traceErr := spatial.TraceFootprint(spatial.FootprintTraceInput{Placement: p.placement, From: centre, To: centre})
			if traceErr != nil || contact.Contact {
				return &StaticPlacementError{Index: i, At: authored, Reason: fmt.Sprintf("is occupied by footprint %q", p.id), Cause: ErrBadPlacement}
			}
		}
		// A SHUT FOOTPRINT DOOR OCCUPIES ITS CELLS TOO (rpg-project#485, R1),
		// in its AUTHORED state — which is what this question is about: a
		// party seat or a monster placed inside a closed door is an author's
		// defect, not something the run works out later. An open one occupies
		// nothing, exactly as an open door's crossing blocks nothing.
		//
		// The doors are the shut-door copy [CompileStaticPlacements] took
		// from the input's door list, not compiled door records, because this
		// seam compiles the field alone. A door whose geometry cannot be
		// measured is refused by validateDoorInputs at construction, and is
		// treated here as occupying the cell — fail closed, the rule the
		// placed loop above already applies.
		for _, d := range s.shutDoors {
			contact, traceErr := spatial.TraceFootprint(spatial.FootprintTraceInput{Placement: d.placement, From: centre, To: centre})
			if traceErr != nil || contact.Contact {
				return &StaticPlacementError{
					Index: i, At: authored,
					Reason: fmt.Sprintf("is occupied by door %q", d.id), Cause: ErrBadPlacement,
				}
			}
		}
	}
	return nil
}

// blockedBy is WHY a cell is closed to a mover, as a phrase to drop into a
// refusal — the same shape [field.notStandable] has always had, extended to the
// contributors the field itself knows nothing about.
//
// NAMES THE THING, not the category. "Cannot place entity" is true and useless;
// a caller can do something about "is blocked by dnd5e:props:pillar" and about
// the id of the creature in the way. This is the door refusal's lesson
// (rpg-toolkit#1123) applied to the two contributors that had no sentence.
//
// The field speaks first when it has something to say, because a cell no region
// owns is not a cell with a pillar on it — it is not a cell at all, and naming
// whatever happens to be standing in the void would be the more confusing of
// two true answers. Only contributors that actually [ContribRef.Blocks] are
// eligible: an ally sharing a blocked cell did not block it.
func (e *Encounter) blockedBy(fact CellFact, cell spatial.Position) string {
	var door, prop, member string
	for _, c := range fact.Contribs {
		if !c.Blocks {
			continue
		}
		switch c.Kind {
		case ContribField:
			return e.field.notStandable(cell)
		case ContribDoor:
			// THE DOOR SPEAKS FIRST among the placed things, because it is
			// the one a caller can do something about: "there is a table
			// here" ends the conversation and "door X is closed" starts
			// the next verb.
			if door == "" {
				door = fmt.Sprintf("is blocked by door %s", c.ID)
			}
		case ContribProp:
			if prop == "" {
				prop = fmt.Sprintf("is blocked by %s", c.Ref)
			}
		case ContribMember:
			if member == "" {
				member = fmt.Sprintf("is occupied by %s", c.ID)
			}
		}
	}
	switch {
	case door != "":
		return door
	case prop != "":
		return prop
	case member != "":
		return member
	default:
		return e.field.notStandable(cell)
	}
}

// passageReadError marks a failed assessment rather than a movement refusal.
// Preserve its cause even if a dependency uses one of our refusal sentinels.
type passageReadError struct{ cause error }

func (e *passageReadError) Error() string { return e.cause.Error() }
func (e *passageReadError) Unwrap() error { return e.cause }

// occupantFacts supplies either current truth or observed facts to one policy.
type occupantFacts struct {
	kind    MemberKind
	down    bool
	blocks  bool
	hostile bool
}

func occupantPassage(in occupantFacts) Passage {
	if in.kind == KindMonster && in.down {
		return PassageStandable
	}
	if in.blocks || in.hostile {
		return PassageBlocked
	}
	return PassagePassThrough
}
