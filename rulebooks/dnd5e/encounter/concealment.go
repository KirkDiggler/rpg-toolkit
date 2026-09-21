// Copyright (C) 2026 Kirk Diggler
// SPDX-License-Identifier: GPL-3.0-or-later

package encounter

import (
	"fmt"

	"github.com/KirkDiggler/rpg-toolkit/tools/spatial"
)

// concealment.go is ONE NOUN, AND EVERYTHING HIDDEN BELONGS TO IT
// (rpg-project#490, "Concealing: one noun, everything hidden belongs to it",
// ruled 2026-09-21).
//
// Before this file the field hid things in two unrelated ways: a door carried
// `Concealed []CheckApproach` and a region carried `Concealed bool`. Two
// flags, two fact kinds, two reveal writers, two beats — and the pair had to
// agree. When they did not, nothing on the page said why: a room only
// enterable through concealed doors that was not itself concealed, and a
// concealed room anyone could walk into, were both authorable, and the
// authoring compiler grew a whole coherence pass to refuse the combinations
// (dungeonspec's `concealment()`).
//
// A CONCEALMENT IS A THING WITH AN ID, and cells, doors and props are hidden
// BY BELONGING TO IT. There is exactly one authored fact, exactly one
// knowledge fact kind, exactly one reveal writer and exactly one beat. The
// two flags are gone; nothing else in a field says "hidden".
//
// # What belonging means
//
//   - A CELL of a concealment is withheld from an observer who has not found
//     it, with everything standing on it — the never-authored yardstick
//     (projection.go), unchanged, asked of a cell set instead of a region.
//   - A DOOR of a concealment is absent from that observer's door list and
//     its doorways; an EDGE door's crossings are masked as ordinary wall at
//     the neighbouring run's height, and a FOOTPRINT door is withheld like a
//     prop with the cells it stands on treated as the concealment's.
//   - A PROP of a concealment is withheld wherever it stands, which is what
//     lets a bookcase in the middle of a visible room be part of a secret.
//
// # No overlap, and why the refusal names both
//
// A cell, a door or a prop belongs to AT MOST ONE concealment (R4). Nesting —
// a closet inside the vault — is a widening a use case may bring; until one
// does, two concealments claiming one thing is a document whose author meant
// something this build cannot represent, and the refusal names both
// declarations so they can see which two.

// ConcealmentID names one concealment.
//
// An alias rather than a defined type, following [DoorID] and [RegionID]: it
// exists to say which of two strings a signature means, not to make callers
// convert.
type ConcealmentID = string

// ConcealmentInput authors one concealment: what finds it, and what it hides.
//
// # Checks and notice are two lists, deliberately
//
// RAW has one DC, and a character whose passive Perception beats it notices
// the secret door outright. This shape splits the two: [ConcealmentInput.Notice]
// is a passive TELL and [ConcealmentInput.Checks] is what a Search rolls, so
// the rogue gets a "hm" and still has to search. An author who wants RAW sets
// `notice` equal to `checks` (rpg-project#490, "Divergence from RAW, stated").
type ConcealmentInput struct {
	// ID names the concealment. REQUIRED non-empty and unique within the
	// field — the knowledge fact kind is minted from it, an intel record
	// names it, and an ambiguous one has no answer.
	ID ConcealmentID

	// Checks are the ways a Search can find this, each with its own DC —
	// [CheckApproach]'s contract. REQUIRED non-empty: a concealment nobody
	// can ever find is a secret the author started and did not finish, and
	// the zero value would be a check with nothing to beat.
	//
	// ONE ROLL PER CONCEALMENT, never per hidden thing ([Encounter.Search]).
	Checks []CheckApproach

	// Notice is the PASSIVE tell — the same approach list, resolved without
	// dice against an observer's passive score when they first sight a cell
	// touching this (rpg-project#490, R6). Optional; nil means the author
	// declared none, and non-nil with zero length is refused exactly as an
	// empty Checks is.
	//
	// CARRIED AND UNREAD IN THIS SLICE. The passive pass is slice 2 (E6):
	// nothing here resolves it, no fact kind is minted for it, and no beat
	// carries it. It is validated and persisted so an author can write it
	// today and so the slice that reads it changes no authored file.
	Notice []CheckApproach

	// Cells are the floor this hides, as authored offset [col,row] pairs
	// under the field's orientation — [RegionInput.Cells]' frame, converted
	// once at construction through the same [HexCellAt].
	//
	// EVERY CELL MUST BE FLOOR this field has (a region's or scenery's), and
	// no cell may belong to two concealments. Optional: a concealment that
	// hides only a door or a prop lists none — a hidden crossing, or a
	// bookcase that is not what it looks like.
	Cells []spatial.Position

	// Doors are the doors this hides, by id. Every one must be a door this
	// field declares, and no door may belong to two concealments. Optional.
	//
	// EITHER GEOMETRY (rpg-project#485). An edge door's crossings are masked
	// as ordinary wall; a footprint door is withheld like a prop, and the
	// cells its rectangle stands on are treated as this concealment's for
	// masking, so the secret does not mark itself as a hole in the floor.
	Doors []DoorID

	// Props are the things this hides, by id — either kind
	// ([PropInput.ID] or [PlacedPropInput.ID], one shared namespace). Every
	// one must be a prop this field declares, and no prop may belong to two
	// concealments. Optional.
	Props []PropID

	// Cells, Doors and Props may not ALL be empty: a concealment that hides
	// nothing is refused (see [field.compileConcealments]).
}

// concealment is one compiled concealment: the authored facts deep-copied in
// the AUTHORED frame — what ToData writes back out — beside the absolute
// cells every read asks for.
type concealment struct {
	id     ConcealmentID
	checks []CheckApproach
	notice []CheckApproach

	// authoredCells is the input's own list, in the authored offset frame,
	// so ToData writes back what the author wrote.
	authoredCells []spatial.Position

	// cells is the same list, absolute axial, converted once — what the
	// projection, the sweep and Search all read.
	cells []spatial.Position

	doors []DoorID
	props []PropID
}

// compileConcealments validates the authored concealments and builds the
// compiled set, refusing every defect by name.
//
// DOOR EXISTENCE IS NOT ASKED HERE, and that is the intel table's own split:
// the door list reaches this module beside the field rather than inside it
// (validateDoorInputs' two callers), so "this names a door this field
// declares" is [validateConcealmentDoors]' question, run by both construction
// seams once they hold the doors. Everything a concealment can get wrong
// without the doors — ids, checks, cells, props, and all three kinds of
// overlap — is refused here, at the one door both seams share.
func (f *field) compileConcealments(in []ConcealmentInput) error {
	if len(in) == 0 {
		return nil
	}
	f.concealments = make([]concealment, 0, len(in))
	f.concealmentIndex = make(map[ConcealmentID]int, len(in))
	f.concealmentOfCell = make(map[spatial.Position]ConcealmentID)
	f.concealmentOfDoor = make(map[DoorID]ConcealmentID)
	f.concealmentOfProp = make(map[PropID]ConcealmentID)

	for i, c := range in {
		if c.ID == "" {
			return fmt.Errorf("concealments[%d] has no id: %w", i, ErrBadConcealment)
		}
		if _, dup := f.concealmentIndex[c.ID]; dup {
			return fmt.Errorf("duplicate concealment %q: %w", c.ID, ErrBadConcealment)
		}
		if err := validateConcealmentCheck(c.ID, "lists no way to find it", c.Checks); err != nil {
			return err
		}
		// NIL IS NOT EMPTY. A concealment with no `notice` is one whose
		// author declared no passive tell; one with an empty list is an
		// author who said there IS a tell and did not say what beats it —
		// [DoorInput.Concealed]'s own nil-vs-empty law, on the other list.
		if c.Notice != nil {
			if err := validateConcealmentCheck(c.ID, "declares a notice with no way through it", c.Notice); err != nil {
				return err
			}
		}
		if len(c.Cells) == 0 && len(c.Doors) == 0 && len(c.Props) == 0 {
			return fmt.Errorf("concealment %q hides nothing: %w", c.ID, ErrBadConcealment)
		}

		compiled := concealment{
			id:            c.ID,
			checks:        append([]CheckApproach(nil), c.Checks...),
			notice:        copyApproaches(c.Notice),
			authoredCells: append([]spatial.Position(nil), c.Cells...),
			doors:         append([]DoorID(nil), c.Doors...),
			props:         append([]PropID(nil), c.Props...),
		}

		for j, at := range c.Cells {
			if !isAuthoredCell(at) {
				return fmt.Errorf("concealment %q cells[%d] (%g,%g) is not a representable integral cell: %w",
					c.ID, j, at.X, at.Y, ErrBadConcealment)
			}
			cell := f.cellAt(at)
			// FLOOR, NOT A REGION — a concealment may hide a strip of
			// scenery the way a wall may stand on one. What is refused is a
			// cell of the VOID, which hides nothing because there is nothing
			// there ([DoorInput]'s own off-floor argument, one noun over).
			if !f.isFloor(cell) {
				return fmt.Errorf("concealment %q cells[%d] [%g,%g] is not floor this field has: %w",
					c.ID, j, at.X, at.Y, ErrBadConcealment)
			}
			if owner, taken := f.concealmentOfCell[cell]; taken {
				return fmt.Errorf(
					"concealments %q and %q both hide the cell [%g,%g], and a cell belongs to one concealment: %w",
					owner, c.ID, at.X, at.Y, ErrBadConcealment)
			}
			f.concealmentOfCell[cell] = c.ID
			compiled.cells = append(compiled.cells, cell)
		}
		sortCells(compiled.cells)

		for _, id := range c.Doors {
			if id == "" {
				return fmt.Errorf("concealment %q hides a door with no id: %w", c.ID, ErrBadConcealment)
			}
			if owner, taken := f.concealmentOfDoor[id]; taken {
				return fmt.Errorf(
					"concealments %q and %q both hide door %q, and a door belongs to one concealment: %w",
					owner, c.ID, id, ErrBadConcealment)
			}
			f.concealmentOfDoor[id] = c.ID
		}

		for _, id := range c.Props {
			if id == "" {
				return fmt.Errorf("concealment %q hides a prop with no id: %w", c.ID, ErrBadConcealment)
			}
			if owner, taken := f.concealmentOfProp[id]; taken {
				return fmt.Errorf(
					"concealments %q and %q both hide prop %q, and a prop belongs to one concealment: %w",
					owner, c.ID, id, ErrBadConcealment)
			}
			// BOTH KINDS OF PROP, one shared namespace (rpg-toolkit#1854):
			// a legacy cell prop and a placed footprint are the same
			// question to a concealment, and compilePlaced refuses a
			// collision between the two lists, which is what makes one
			// lookup safe.
			if f.propIndexOf(id) < 0 && f.placedIndexOf(id) < 0 {
				return fmt.Errorf("concealment %q hides prop %q, which this field does not declare: %w",
					c.ID, id, ErrBadConcealment)
			}
			f.concealmentOfProp[id] = c.ID
		}

		f.concealmentIndex[c.ID] = i
		f.concealments = append(f.concealments, compiled)
	}

	return nil
}

// validateConcealmentCheck refuses a malformed approach list on a
// concealment: empty, or an approach with nothing to beat.
// [validateCheck]'s rules, with a concealment's own noun in the sentence —
// a door's version says "door %q" and an author fixing one of these is
// looking at the other thing.
func validateConcealmentCheck(id ConcealmentID, empty string, approaches []CheckApproach) error {
	if len(approaches) == 0 {
		return fmt.Errorf("concealment %q %s: %w", id, empty, ErrBadConcealment)
	}
	for _, a := range approaches {
		if a.DC < 1 {
			return fmt.Errorf("concealment %q lists an approach at DC %d, which nothing has to beat: %w",
				id, a.DC, ErrBadConcealment)
		}
	}

	return nil
}

// validateConcealmentDoors refuses a concealment naming a door this field
// does not declare — [validateIntelTargets]' shape, for the other kind of
// dead reference, and run by both construction seams at the same point for
// the same reason: the door list arrives beside the field, not inside it.
func validateConcealmentDoors(f *field, doors []DoorInput) error {
	if len(f.concealments) == 0 {
		return nil
	}
	declared := make(map[DoorID]bool, len(doors))
	for _, d := range doors {
		declared[d.ID] = true
	}
	for i := range f.concealments {
		c := &f.concealments[i]
		for _, id := range c.doors {
			if !declared[id] {
				return fmt.Errorf("concealment %q hides door %q, which this field does not declare: %w",
					c.id, id, ErrNoDoor)
			}
		}
	}

	return nil
}

// concealmentOf is the compiled concealment with this id, or nil.
func (f *field) concealmentOf(id ConcealmentID) *concealment {
	i, ok := f.concealmentIndex[id]
	if !ok {
		return nil
	}

	return &f.concealments[i]
}

// hiddenCellsOf is EVERY CELL A CONCEALMENT HIDES: the cells its author
// listed, plus the cells any FOOTPRINT door of it stands on.
//
// THE SECOND HALF IS WHAT MAKES A FOOTPRINT DOOR HIDEABLE AT ALL. A
// rectangle in the middle of a room has no crossing to mask, so the only way
// its absence does not mark itself is for the floor it occupies to be
// withheld with it — the same answer the never-authored yardstick gives for
// every other hidden thing. An EDGE door contributes nothing here: its
// crossing is masked as wall (projection.go), which is what a wall is.
//
// Derived per call rather than compiled, because a footprint door's cells are
// a question about the FIELD's cell list and the doors arrive beside it.
// Callers that ask repeatedly build the union once (see [Encounter.hiddenFrom]).
func (e *Encounter) hiddenCellsOf(c *concealment) []spatial.Position {
	out := append([]spatial.Position(nil), c.cells...)
	for _, id := range c.doors {
		d, ok := e.doorsByID[id]
		if !ok || d.placement == nil {
			continue
		}
		out = append(out, e.field.placedCells(*d.placement)...)
	}

	return out
}
