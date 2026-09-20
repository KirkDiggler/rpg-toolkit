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
// space is closed to you. (The size-difference exception waits for size to
// exist on a member — [memberEntity.GetSize] is a hardcoded 1.)
func (e *Encounter) CellAt(in CellAtInput) CellFact {
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
			if !standing {
				continue
			}
			contact, err := spatial.TraceFootprint(spatial.FootprintTraceInput{
				Placement: placement, From: centre, To: centre,
			})
			if err != nil || contact.Contact {
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
			blocks := v.BlocksMovement() || e.opposed(in.Mover, MemberID(v.id))
			if blocks {
				fact.Passage = PassageBlocked
			} else if fact.Passage == PassageStandable {
				fact.Passage = PassagePassThrough
			}
			fact.Contribs = append(fact.Contribs, ContribRef{
				Kind: ContribMember, ID: v.id, Blocks: blocks,
			})
		}
	}

	return fact
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
func ValidateStaticPlacements(in FieldInput, cells []spatial.Position) error {
	f, err := compileField(in)
	if err != nil {
		return err
	}
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
		// The doors are read from the INPUT rather than from compiled
		// records, because this seam compiles the field alone; a door whose
		// geometry is unmeasurable is refused by validateDoorInputs at
		// construction, and is treated here as occupying the cell — fail
		// closed, the rule the placed loop above already applies.
		for _, d := range in.Doors {
			if d.Placement == nil || d.Placement.Footprint.Box == nil || d.State == nil || !d.State.blocks() {
				// A door with no box is a field construction refuses
				// outright (validateDoorInputs), so there is nothing
				// measurable here and nothing to be closed about.
				continue
			}
			contact, traceErr := spatial.TraceFootprint(spatial.FootprintTraceInput{Placement: *d.Placement, From: centre, To: centre})
			if traceErr != nil || contact.Contact {
				return &StaticPlacementError{
					Index: i, At: authored,
					Reason: fmt.Sprintf("is occupied by door %q", d.ID), Cause: ErrBadPlacement,
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
