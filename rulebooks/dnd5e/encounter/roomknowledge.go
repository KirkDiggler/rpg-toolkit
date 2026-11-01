// Copyright (C) 2026 Kirk Diggler
// SPDX-License-Identifier: GPL-3.0-or-later

package encounter

import (
	"encoding/json"
	"fmt"
	"math"

	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/encounter/internal/rowruns"
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

// cellsKnownPayload is a `known:cells` fact's Detail, in exactly one of two
// spellings of the same cell set.
//
// ROWS IS WHAT THIS MODULE WRITES (rpg-project#753, "Knowledge is written
// once, compactly"): the cells as canonical axial row runs
// ([rowruns.Row]: q spans per r), the one spelling a site's regions use
// too. A sight pass learns a disk cut by walls, which is a few dozen spans
// rather than hundreds of cells.
//
// CELLS IS THE FORM A RUN SAVED BEFORE THAT CARRIES: one absolute axial cell
// per entry, sorted by [cellBefore]. It is read, never written, and never
// migrated — a stored fact keeps its bytes, and both spellings fold to the
// same known set.
//
// Either way the writer never lists a cell twice or one already known, and
// the load boundary refuses a cell that is not floor.
type cellsKnownPayload struct {
	Cells []spatial.Position `json:"cells,omitempty"`
	Rows  []rowruns.Row      `json:"rows,omitempty"`
}

// encodeCellsKnown is a learning moment's Detail: its cells as row runs.
// Every floor cell is a whole axial pair ([field.cellAt] converts each
// authored integral cell), so a cell that is not — or one beyond the range a
// float64 holds every integer of — is refused rather than rounded into a
// different one.
func encodeCellsKnown(cells []spatial.Position) (string, error) {
	in := make([]rowruns.Cell, len(cells))
	for i, cell := range cells {
		if !wholeCell(cell) || math.Abs(cell.X) > 1<<53 || math.Abs(cell.Y) > 1<<53 {
			return "", fmt.Errorf("cell %v is not a whole axial cell", cell)
		}
		in[i] = rowruns.Cell{Q: int(cell.X), R: int(cell.Y)}
	}
	detail, err := json.Marshal(cellsKnownPayload{Rows: rowruns.Encode(in)})
	if err != nil {
		return "", err
	}
	return string(detail), nil
}

// decodeCellsKnown reads one fact's Detail in either spelling, refusing one
// that names both or neither, runs that are not canonical, and runs naming
// more than limit cells (counted before a cell is listed). The load boundary
// ([validateWorldFacts]) is the only caller that can see a malformed one.
func decodeCellsKnown(detail string, limit int) ([]spatial.Position, error) {
	var payload cellsKnownPayload
	if err := json.Unmarshal([]byte(detail), &payload); err != nil {
		return nil, err
	}
	switch {
	case payload.Cells != nil && payload.Rows != nil:
		return nil, fmt.Errorf("both a cells list and row runs")
	case payload.Cells != nil:
		return payload.Cells, nil
	case payload.Rows == nil:
		return nil, fmt.Errorf("no cells list")
	}
	cells, err := rowruns.Expand(payload.Rows, limit)
	if err != nil {
		return nil, err
	}
	out := make([]spatial.Position, len(cells))
	for i, c := range cells {
		out[i] = spatial.Position{X: float64(c.Q), Y: float64(c.R)}
	}
	return out, nil
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
	w.cells.catchUp(w.log, member)
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
//
// FOLDED PER MEMBER, ON ASKING (rpg-project#753, S3). A member's facts are
// audienced to that member alone, so one member's answer reads only the
// facts they witnessed, and a member nobody asks about is never decoded: a
// fresh load whose first read is one seat's reach decodes that seat's
// knowledge, not the garrison's.
type cellFold struct {
	log *journal.Journal
	// seen is the journal's length at each member's last catch-up, and read
	// how many of the facts that member witnessed have been folded: the
	// witnessed list only ever grows at its end, so the next catch-up starts
	// there.
	seen     map[MemberID]int
	read     map[MemberID]int
	byMember map[MemberID]map[spatial.Position]bool
}

// catchUp folds every fact of member's appended since the member's last
// catch-up.
func (f *cellFold) catchUp(log *journal.Journal, member MemberID) {
	if f.log != log || f.byMember == nil {
		*f = cellFold{
			log: log, seen: map[MemberID]int{}, read: map[MemberID]int{},
			byMember: map[MemberID]map[spatial.Position]bool{},
		}
	}
	if log.Len() == f.seen[member] {
		return
	}
	witnessed := log.WitnessedBy(journal.EntityID(member))
	for _, fact := range witnessed[f.read[member]:] {
		if fact.Kind != cellsKnownKind || MemberID(fact.Actor) != member {
			continue
		}
		cells, err := decodeCellsKnown(fact.Outcome.Detail, maxFieldCells)
		if err != nil {
			continue // unreachable: the writer and the load boundary agree
		}
		known := f.byMember[member]
		if known == nil {
			known = make(map[spatial.Position]bool, len(cells))
			f.byMember[member] = known
		}
		for _, cell := range cells {
			known[cell] = true
		}
	}
	f.read[member] = len(witnessed)
	f.seen[member] = log.Len()
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
// THE SWEEP COVERS WHAT CAN BE SEEN (rpg-project#753, S3): the floor within
// the observer's sight this pass plus one ring, [field.sweepCells], never the
// whole field. Every cell beyond that ring is one [sightReach.seesCell]
// answers false for by its own first two tests — a ray reaches no farther
// than the sight range, and a cover's face is asked of a cell at most one
// further — so the bounded sweep learns exactly what the whole-floor sweep
// learned, at the cost of the ring.
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
	for _, cell := range e.field.sweepCells(own, reach.cells[observer]) {
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

// sweepCells is every floor cell within sight + 1 of own by the grid's
// distance: the cells [sightReach.seesCell] can answer true for, which is
// the sight range for a ray ([sightReach.reachesCell]) and one more for a
// cover's face. Walked as the axial hex disk around own, so it costs the
// disk, not the field; a disk larger than the floor — a sight that reaches
// past every wall of the map — walks the floor instead, which is the same
// set at the floor's cost. Negative sight is refused upstream ([Sight]); a
// blind observer's disk is its own cell and its neighbours.
//
// The disk is axial (q, r) with s = -q-r, the encounter's one grid
// ([Encounter.Grid]); a cell is in it when max(|dq|, |dr|, |dq+dr|) is at
// most the radius, which is the cube distance [spatial.AxialHexGrid]
// measures.
func (f *field) sweepCells(own spatial.Position, sight int) []spatial.Position {
	radius := sight + 1
	if sight < 0 || radius <= 0 || disk(radius) >= len(f.cells) || !wholeCell(own) {
		return f.cells
	}
	q0, r0 := int(own.X), int(own.Y)
	out := make([]spatial.Position, 0, disk(radius))
	for dq := -radius; dq <= radius; dq++ {
		lo, hi := max(-radius, -dq-radius), min(radius, -dq+radius)
		for dr := lo; dr <= hi; dr++ {
			cell := spatial.Position{X: float64(q0 + dq), Y: float64(r0 + dr)}
			if f.isFloor(cell) {
				out = append(out, cell)
			}
		}
	}

	return out
}

// wholeCell reports whether a position is a cell's own integral axial pair,
// the only kind the disk can be walked around. A member always stands on one;
// anything else walks the floor.
func wholeCell(p spatial.Position) bool { return p.X == math.Trunc(p.X) && p.Y == math.Trunc(p.Y) }

// disk is how many cells a hex disk of this radius holds: 3r(r+1)+1. A
// radius too large to count is larger than any floor.
func disk(radius int) int {
	if radius > 1<<20 {
		return int(^uint(0) >> 1)
	}
	return 3*radius*(radius+1) + 1
}

// learnCells is the layout-only writer. Sight, opening a door and presence
// share it; none of them records mutable contents as a consequence of
// learning floor. known is the member's fold read by the caller, so a cell
// already learned is never written twice. One fact and at most one
// cells-revealed beat per call, and the beat only for a seat.
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
	// Later discovery is a recipient-scoped addition on the existing story,
	// for a seat alone ([Encounter.revealsTo]).
	reveal := next > 1 && e.revealsTo(member)
	if reveal {
		before, err = e.AtlasFor(member)
		if err != nil {
			return err
		}
	}
	detail, err := encodeCellsKnown(fresh)
	if err != nil {
		return fmt.Errorf("discover cells: %w", err)
	}
	_, err = e.world.log.Append(journal.Fact{
		Kind: cellsKnownKind, Actor: journal.EntityID(member),
		Subject: journal.EntityID(member), Audience: journal.Audience{journal.EntityID(member)},
		Outcome: journal.Outcome{Detail: detail},
	})
	if err != nil {
		return fmt.Errorf("discover cells: %w", err)
	}
	if reveal {
		return e.appendCellsRevealedBeat(member, before)
	}
	return nil
}

// revealsTo reports whether a member's learning is delivered as a
// cells-revealed beat: a SEAT's — a player's, whose client draws the atlas
// the beat is a delta of. A monster or a world NPC learns exactly as a seat
// does, into the one `known:cells` fact its sweep writes, and its knowledge
// is read from that fact by everything that asks (its sight, its memory, a
// reload); the beat is the client's, and it has no client (rpg-project#753,
// "A sweep covers what can be seen"; design O2: no reader of a monster's
// beat is known, and one that appears reopens this).
func (e *Encounter) revealsTo(member MemberID) bool {
	m, ok := e.members[member]
	return ok && m.Kind == KindPlayer
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
//
// It is [Encounter.undiscoveredMask] MATERIALIZED over the floor: the one
// rule, asked of every cell, for the projection that walks every cell anyway.
// A read that asks a few cells asks the mask itself (rpg-project#753).
func (e *Encounter) undiscoveredFrom(member MemberID) hiddenView {
	mask := e.undiscoveredMask(member)
	hidden := e.hiddenFrom(member)
	hidden.unexploredCells = make(map[spatial.Position]bool)
	// Footing beats even concealment: an authored boundary's unowned footing
	// is on the map wherever its wall is.
	for cell := range mask.footing {
		delete(hidden.cells, cell)
	}
	for _, cell := range e.field.cells {
		if mask.hides(cell) {
			hidden.cells[cell] = true
		}
		if mask.unexplored(cell) {
			hidden.unexploredCells[cell] = true
		}
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
