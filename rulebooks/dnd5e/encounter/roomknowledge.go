// Copyright (C) 2026 Kirk Diggler
// SPDX-License-Identifier: GPL-3.0-or-later

package encounter

import (
	"encoding/json"
	"fmt"

	"github.com/KirkDiggler/rpg-toolkit/tools/spatial"
	"github.com/KirkDiggler/rpg-toolkit/world/journal"
)

// legacyRoomKnownPrefix is the retired whole-room discovery kind
// (`known:room:<region>`). A blob written before cell discovery still LOADS
// with these facts in it, and they TEACH NOTHING (rpg-project#586 R1, R6
// superseded): a room is no longer a unit of knowledge, so a fact claiming one
// has no cells to contribute and is carried only so the save stays loadable.
const legacyRoomKnownPrefix = "known:room:"

func legacyRoomKnownKind(id RegionID) journal.Kind { return journal.Kind(legacyRoomKnownPrefix + id) }

// cellsKnownKind is THE CELL IS THE UNIT OF KNOWLEDGE (rpg-project#586 R1):
// one fact per member per learning moment, actor = subject = audience = that
// member alone (R4 — memory is per seat), its [journal.Outcome.Detail] the
// JSON [cellsKnownPayload] naming only the cells that moment newly taught.
// The fold is a union; nothing is ever forgotten within the run.
const cellsKnownKind journal.Kind = "known:cells"

// cellsKnownPayload is a `known:cells` fact's Detail. Cells are absolute
// axial and sorted by [cellBefore]; the writer never lists a cell twice or one
// already known, and the load boundary refuses a cell that is not floor.
type cellsKnownPayload struct {
	Cells []spatial.Position `json:"cells"`
}

// decodeCellsKnown reads one fact's Detail. The load boundary
// ([validateWorldFacts]) is the only caller that can see a malformed one.
func decodeCellsKnown(detail string) ([]spatial.Position, error) {
	var payload cellsKnownPayload
	if err := json.Unmarshal([]byte(detail), &payload); err != nil {
		return nil, err
	}
	if payload.Cells == nil {
		return nil, fmt.Errorf("no cells list")
	}

	return payload.Cells, nil
}

// knownCells folds what a member has learned of the fixed layout — every cell
// of every `known:cells` fact they wrote — independently of mutable subject
// testimony. The journal persists only the fact of discovery; the field owns
// the geometry.
//
// THE ANSWER IS SHARED AND READ-ONLY: callers never write to it. It is the
// running [cellFold], so a later fact can add cells to a map a caller is still
// holding — which is the only change a union can make.
func (w *encounterWorld) knownCells(member MemberID) map[spatial.Position]bool {
	w.cells.catchUp(w.log)
	if known := w.cells.byMember[member]; known != nil {
		return known
	}
	return map[spatial.Position]bool{}
}

// cellFold is THE `known:cells` FOLD, CARRIED FORWARD rather than recomputed
// per question. The journal is append-only and a fold of it is a union, so the
// answer for N facts plus the facts after N is the answer for all of them; the
// fold remembers how far it has read and decodes only what landed since. It is
// derived state of the one truth, never persisted and never a second record —
// it is rebuilt from the journal by whoever holds a fresh world, and it starts
// over if it is ever handed a different journal.
//
// Why it exists: every [Encounter.AtlasFor], every sight pass and every
// [Encounter.VisibleCellsFor] asks it, and re-decoding every fact's cell list
// for each of those cost more than the rest of the read on a keep-sized floor
// (sightpass_scale_test.go).
type cellFold struct {
	log      *journal.Journal
	read     int
	byMember map[MemberID]map[spatial.Position]bool
}

// catchUp folds every fact appended since the last call.
func (f *cellFold) catchUp(log *journal.Journal) {
	if f.log != log {
		*f = cellFold{log: log, byMember: map[MemberID]map[spatial.Position]bool{}}
	}
	if log.Len() == f.read {
		return
	}
	facts := log.All()
	for _, fact := range facts[f.read:] {
		if fact.Kind != cellsKnownKind {
			continue
		}
		cells, err := decodeCellsKnown(fact.Outcome.Detail)
		if err != nil {
			continue // unreachable: the writer and the load boundary agree
		}
		member := MemberID(fact.Actor)
		known := f.byMember[member]
		if known == nil {
			known = make(map[spatial.Position]bool, len(cells))
			f.byMember[member] = known
		}
		for _, cell := range cells {
			known[cell] = true
		}
	}
	f.read = len(facts)
}

// discoverCells is THE SIGHT PASS'S LEARNING HALF: every floor cell this
// observer does not already know and that [sightReach.seesCell] answers true
// for — reachesCell, the one sight test seen-now asks too (rpg-project#586
// R5), plus the face of a cover standing on a cell's centre — learned in ONE
// fact for the pass.
//
// A CONCEALED CELL IS LEARNED LIKE ANY OTHER, and concealment still withholds
// it. The two masks stay separate ("unknown is not secret"): knowing a cell
// says the observer has looked at that spot, so an unlisted prop standing there
// is presented exactly as it was when knowledge was by room; the concealment
// alone decides that the floor reads as the never-authored twin until it is
// found. Skipping concealed cells here would fold the secret into the
// unexplored mask and hide every unlisted thing on secret floor.
//
// THE CELL THE OBSERVER STANDS ON IS ALWAYS LEARNED, sight or no sight: a
// member inside an obscuring area or with no sight at all still knows the
// floor under their feet, the same way presence pierces a concealment. It is
// what keeps [Encounter.VisibleCellsFor] non-empty for a placed member.
//
// The learned fact makes no claim about occupants, props, or a door's
// current state.
//
// AN OBSERVER WHOSE SWEEP KEY IS UNCHANGED SKIPS (sightsweep.go): opacity is
// the pass's [Encounter.sightOpacity], and the key is stored only once the
// sweep's cells are learned, so a failed sweep is retried.
func (e *Encounter) discoverCells(observer MemberID, reach sightReach, opacity string) error {
	own, placed := reach.positions[observer]
	if !placed {
		return nil
	}
	hidden := e.hiddenFrom(observer)
	key := sweepKeyOf(own, reach.cells[observer], opacity, hidden)
	if e.sightSweeps[observer] == string(key) {
		return nil
	}
	if e.sightSweepsRun == nil {
		e.sightSweepsRun = make(map[MemberID]int)
	}
	e.sightSweepsRun[observer]++
	known := e.world.knownCells(observer)
	learned := make([]spatial.Position, 0)
	for _, cell := range e.field.cells {
		if known[cell] {
			continue
		}
		if cell == own || reach.seesCell(observer, cell, hidden) {
			learned = append(learned, cell)
		}
	}
	if err := e.learnCells(observer, learned, known); err != nil {
		return err
	}
	if e.sightSweeps == nil {
		e.sightSweeps = make(map[MemberID]string)
	}
	e.sightSweeps[observer] = string(key)

	return nil
}

// learnCells is the layout-only writer. Sight, opening a door and presence
// share it; none of them records mutable contents as a consequence of
// learning floor. known is the member's fold read by the caller, so a cell
// already learned is never written twice. One fact and at most one
// cells-revealed beat per call.
func (e *Encounter) learnCells(member MemberID, cells []spatial.Position, known map[spatial.Position]bool) error {
	fresh := make([]spatial.Position, 0, len(cells))
	seen := make(map[spatial.Position]bool, len(cells))
	for _, cell := range cells {
		if known[cell] || seen[cell] {
			continue
		}
		seen[cell] = true
		fresh = append(fresh, cell)
	}
	if len(fresh) == 0 {
		return nil
	}
	sortCells(fresh)
	next, err := e.NextStorySeq()
	if err != nil {
		return err
	}
	var before Atlas
	// Initial discovery precedes scene-opened and is restored by the snapshot.
	// Later discovery is a recipient-scoped addition on the existing story.
	if next > 1 {
		before, err = e.AtlasFor(member)
		if err != nil {
			return err
		}
	}
	detail, err := json.Marshal(cellsKnownPayload{Cells: fresh})
	if err != nil {
		return fmt.Errorf("discover cells: %w", err)
	}
	_, err = e.world.log.Append(journal.Fact{
		Kind: cellsKnownKind, Actor: journal.EntityID(member),
		Subject: journal.EntityID(member), Audience: journal.Audience{journal.EntityID(member)},
		Outcome: journal.Outcome{Detail: string(detail)},
	})
	if err != nil {
		return fmt.Errorf("discover cells: %w", err)
	}
	if next > 1 {
		return e.appendCellsRevealedBeat(member, before)
	}
	return nil
}

// discoverOpenedCells gives the opener the cells on the opened door's edges,
// both sides and nothing beyond them (rpg-project#586 R1): opening a door is
// not seeing the room behind it. The sight pass that follows the open teaches
// whatever the opener can actually see. A world-side open with no actor
// teaches nobody; other observers learn only through their own pass.
func (e *Encounter) discoverOpenedCells(member MemberID, door *doorRecord) error {
	if member == "" {
		return nil
	}
	cells := make([]spatial.Position, 0, 2*len(door.edges))
	for _, edge := range door.edges {
		for _, cell := range []spatial.Position{edge.From, edge.To} {
			if e.field.isFloor(cell) {
				cells = append(cells, cell)
			}
		}
	}
	return e.learnCells(member, cells, e.world.knownCells(member))
}

// undiscoveredFrom adds ordinary unexplored floor to concealment's projection
// mask. This is a delivery mask only: unknown ordinary floor is not a secret
// wall and must not acquire concealment's movement/probe rules.
func (e *Encounter) undiscoveredFrom(member MemberID) hiddenView {
	hidden := e.hiddenFrom(member)
	hidden.unexploredCells = make(map[spatial.Position]bool)
	known := e.world.knownCells(member)
	// Existing explicit concealment discovery teaches its cells without the
	// sight pass seeing them. Preserve that knowledge; ordinary cell
	// discovery is not an authority to forget separately learned geometry.
	revealed := make(map[spatial.Position]bool)
	for i := range e.field.concealments {
		c := &e.field.concealments[i]
		if e.world.knowsConcealment(member, c.id) {
			for _, cell := range e.hiddenCellsOf(c) {
				revealed[cell] = true
			}
		}
	}
	for _, cell := range e.field.cells {
		if !known[cell] && !revealed[cell] {
			hidden.cells[cell] = true
			hidden.unexploredCells[cell] = true
		}
	}
	// Unowned scenery directly bordering learned floor belongs to its visible
	// boundary. A presented wall adds no footing of its own (fog-of-war R9).
	for _, cell := range e.field.cells {
		if _, owned := e.field.regionOf(cell); owned || !hidden.unexploredCells[cell] {
			continue
		}
		for _, neighbor := range adjacencyGrid.GetNeighbors(cell) {
			_, owned := e.field.regionOf(neighbor)
			if owned && known[neighbor] && !hidden.cells[neighbor] {
				// Knowing a boundary's ordinary footing does not beat an
				// explicit concealment on that floor. Keep the two masks
				// independent so unlisted props do not inherit floor secrecy.
				if _, concealed := e.field.concealmentOfCell[cell]; !concealed || revealed[cell] {
					delete(hidden.cells, cell)
				}
				delete(hidden.unexploredCells, cell)
				break
			}
		}
	}
	// An authored boundary brings its unowned footing, but not the adjoining
	// room's interior. Read the pre-boundary set so long chains do not flood.
	footing := make(map[spatial.Position]bool)
	for _, wall := range e.field.walls {
		from, to := e.field.cellAt(wall.From), e.field.cellAt(wall.To)
		if hidden.cells[from] && hidden.cells[to] {
			continue
		}
		for _, cell := range []spatial.Position{from, to} {
			if _, owned := e.field.regionOf(cell); !owned {
				footing[cell] = true
			}
		}
	}
	for cell := range footing {
		delete(hidden.cells, cell)
		delete(hidden.unexploredCells, cell)
	}
	for _, door := range e.doors {
		if hidden.doors[door.id] {
			continue
		}
		if id, concealed := e.hiddenDoorConcealment(door.id); concealed && e.world.knowsConcealment(member, id) {
			continue
		}
		visible := false
		for _, edge := range door.edges {
			if !hidden.cells[edge.From] || !hidden.cells[edge.To] {
				visible = true
				break
			}
		}
		// Footprint doors are projected with their fixed footprint rather than edges.
		if len(door.edges) > 0 && !visible {
			hidden.doors[door.id] = true
		}
	}
	return hidden
}
