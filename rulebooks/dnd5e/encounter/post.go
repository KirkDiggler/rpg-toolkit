// Copyright (C) 2026 Kirk Diggler
// SPDX-License-Identifier: GPL-3.0-or-later

package encounter

import (
	"fmt"
	"slices"
	"sort"

	"github.com/KirkDiggler/rpg-toolkit/mind/machine"
	"github.com/KirkDiggler/rpg-toolkit/mind/perception"
	"github.com/KirkDiggler/rpg-toolkit/tools/spatial"
)

// post.go is A POST HOLDS A PASSAGE (rpg-project#852, S1b of the guard
// holds the gate; ideas/the-guard-holds-the-gate R5).
//
// A body that blocks holds one cell. The castle gate is a two-cell opening
// with a double door, and the party stepped through beside the posted guard.
// So a state may name a POST: the door whose opening it holds, and the cells
// a holder stands on to hold it. While a member is in that state, up and on
// one of those cells, the door's crossings are closed to every mover not
// allied with him, open or shut, at any width. His body still blocks its own
// cell, as [MemberInput.BlockingStates] says.
//
// # The hold is a state at a place
//
// Nothing about a hold is stored beside the state or the cell. It is read on
// every call ([Encounter.memberHolds]): any leaving empties it at once,
// whatever moved him (a voice he walks to, a clang, a pursuit, a dash, a
// push), and he holds again the step he stands there. A downed member holds
// nothing. A stranger on his only post cell keeps him from holding until it
// is free; a post with two cells avoids it.
//
// # The holder is the post
//
// Several members may hold one opening, and it is held while any of them
// does. Killing one guard does not open the gate.
//
// # Per mover, never in the crossing memo
//
// The crossing fold ([cellQuery.crossingBlocked]) is one answer every mover
// shares. A hold is an answer about a pair, the mover and the holder, so it
// is read beside the masquerade ([Encounter.masqueradeBlocks]) and before
// the door pass: a walk never opens a held door ([cellQuery.doorToOpen]).
//
// # The opening is the door's own
//
// An edge door's edges; a footprint door's crossings, read from its
// rectangle ([field.doorCrossings]). Both are geometry, computed once per
// door ([Encounter.doorOpening]).

// Post is what a state holds: the door whose opening it bars, and the
// dungeon-absolute cells a holder stands on to bar it (the frame
// [JoinInput.Cell] and a table's [Selector.At] use). Empty Cells means the
// cell the member is placed at, resolved when it is admitted.
type Post struct {
	// Door is the door whose opening the post holds.
	Door DoorID

	// Cells is where a holder stands to hold it.
	Cells []spatial.Position
}

// PostData is a [Post] as the record stores it.
type PostData struct {
	// Door is [Post.Door].
	Door DoorID `json:"door"`

	// Cells is [Post.Cells], resolved: never empty in a stored record.
	Cells []spatial.Position `json:"cells"`
}

// postsDataFrom is a member's posts as the record stores them, nil for none.
func postsDataFrom(posts map[string]Post) map[string]PostData {
	if len(posts) == 0 {
		return nil
	}
	out := make(map[string]PostData, len(posts))
	for state, p := range posts {
		out[state] = PostData{Door: p.Door, Cells: slices.Clone(p.Cells)}
	}

	return out
}

// postsFromData is a stored record's posts, refused (an error the caller
// wraps in [ErrInvalidData]) when a post names no cell: admission resolves
// the default, so a stored post always carries its cells.
func postsFromData(posts map[string]PostData) (map[string]Post, error) {
	if len(posts) == 0 {
		return nil, nil
	}
	out := make(map[string]Post, len(posts))
	for state, p := range posts {
		if len(p.Cells) == 0 {
			return nil, fmt.Errorf("post %q names no cell: a stored post carries its cells", state)
		}
		out[state] = Post{Door: p.Door, Cells: slices.Clone(p.Cells)}
	}

	return out, nil
}

// validatePostStates refuses posts nothing can name: any on a member with
// no authored recipe, and a state the recipe does not declare. The door and
// the cells are the field's question, asked by [Encounter.usePosts].
func validatePostStates(r *machine.Recipe, posts map[string]Post) error {
	if len(posts) == 0 {
		return nil
	}
	if r == nil {
		return fmt.Errorf("posts name states of no recipe: a post needs authored states")
	}
	for _, state := range sortedPostStates(posts) {
		if _, ok := r.States[state]; !ok {
			return fmt.Errorf("post state %q is not a state of its recipe", state)
		}
	}

	return nil
}

// usePosts gives a member its own copy of its posts, each refused when it
// names a state its recipe lacks, a door this field lacks, or a cell that is
// not floor, and each with no cells resolved to `at`, the cell it is placed
// at.
//
// Errors: the refusal, wrapped in [ErrBadMember].
func (e *Encounter) usePosts(m *memberRecord, posts map[string]Post, at spatial.Position) error {
	if err := validatePostStates(m.Recipe, posts); err != nil {
		return fmt.Errorf("member %s: %w: %w", m.ID, err, ErrBadMember)
	}
	m.Posts = nil
	if len(posts) == 0 {
		return nil
	}
	out := make(map[string]Post, len(posts))
	for _, state := range sortedPostStates(posts) {
		p := posts[state]
		if _, ok := e.doorsByID[p.Door]; !ok {
			return fmt.Errorf("member %s: post %q holds door %q, which this field does not have: %w",
				m.ID, state, p.Door, ErrBadMember)
		}
		cells := slices.Clone(p.Cells)
		if len(cells) == 0 {
			cells = []spatial.Position{at}
		}
		for _, cell := range cells {
			if !e.field.isFloor(cell) {
				return fmt.Errorf("member %s: post %q stands on %v, which is not floor: %w", m.ID, state, cell, ErrBadMember)
			}
		}
		out[state] = Post{Door: p.Door, Cells: cells}
	}
	m.Posts = out
	if e.posted == nil {
		e.posted = map[MemberID]bool{}
	}
	e.posted[m.ID] = true

	return nil
}

// sortedPostStates is a posts map's states in name order (C8).
func sortedPostStates(posts map[string]Post) []string {
	out := make([]string, 0, len(posts))
	for state := range posts {
		out = append(out, state)
	}
	sort.Strings(out)

	return out
}

// postIn is the post a member's state names, the empty state being its
// recipe's initial one.
func (m *memberRecord) postIn(state string) (Post, bool) {
	if len(m.Posts) == 0 {
		return Post{}, false
	}
	if state == "" && m.Recipe != nil {
		state = m.Recipe.Initial
	}
	p, ok := m.Posts[state]

	return p, ok
}

// samePost reports whether two states name the same post.
func samePost(a Post, okA bool, b Post, okB bool) bool {
	return okA == okB && a.Door == b.Door && slices.Equal(a.Cells, b.Cells)
}

// memberHolds is the door a member holds now and whether it holds one: its
// current state names a post, it is up, and it stands on one of the post's
// cells (rpg-project#852, H1). Read on every call, never stored, so leaving
// the cell empties the hold at once and standing there again restores it.
//
// `down` is the participation read the caller already holds; nil reads it
// here. A read that fails holds: a world that cannot say who is down does
// not open a gate on a guess.
func (e *Encounter) memberHolds(id MemberID, down map[MemberID]bool) (DoorID, bool) {
	m, ok := e.members[id]
	if !ok {
		return "", false
	}
	post, ok := m.postIn(m.Machine.State)
	if !ok {
		return "", false
	}
	cell, placed := e.canvas.GetEntityPosition(string(id))
	if !placed || !slices.Contains(post.Cells, cell) {
		return "", false
	}
	if down == nil {
		read, err := e.downNow()
		if err != nil {
			return post.Door, true
		}
		down = read
	}
	if down[id] {
		return "", false
	}

	return post.Door, true
}

// doorOpening is a door's crossings, normalized: an edge door's edges, a
// footprint door's crossings ([field.doorCrossings]). Geometry, computed
// once per door, for the reason [Encounter.links] is.
func (e *Encounter) doorOpening(id DoorID) []DoorEdge {
	d, ok := e.doorsByID[id]
	if !ok {
		return nil
	}

	return e.openingOf(d)
}

// openingOf is [Encounter.doorOpening] for a door record in hand.
func (e *Encounter) openingOf(d *doorRecord) []DoorEdge {
	if out, ok := e.openings[d.id]; ok {
		return out
	}
	out := make([]DoorEdge, 0, len(d.edges))
	for _, edge := range d.edges {
		out = append(out, normalizeDoorEdge(edge))
	}
	for _, crossing := range e.field.doorCrossings(d) {
		out = append(out, normalizeDoorEdge(DoorEdge{From: crossing[0], To: crossing[1]}))
	}
	if e.openings == nil {
		e.openings = map[DoorID][]DoorEdge{}
	}
	e.openings[d.id] = out

	return out
}

// heldCrossings is every crossing held now and its holders, in id order:
// the hold of every posted member that holds, whoever it holds against.
// Nil when nobody holds. A member removed since it was posted is asked and
// holds nothing.
func (e *Encounter) heldCrossings(down map[MemberID]bool) map[DoorEdge][]postHolder {
	if len(e.posted) == 0 {
		return nil
	}
	ids := make([]MemberID, 0, len(e.posted))
	for id := range e.posted {
		ids = append(ids, id)
	}
	slices.Sort(ids)
	var out map[DoorEdge][]postHolder
	for _, id := range ids {
		door, holds := e.memberHolds(id, down)
		if !holds {
			continue
		}
		for _, edge := range e.doorOpening(door) {
			if out == nil {
				out = map[DoorEdge][]postHolder{}
			}
			out[edge] = append(out[edge], postHolder{holder: id, door: door})
		}
	}

	return out
}

// postHolder is one holder of a crossing and the door it holds there.
type postHolder struct {
	holder MemberID
	door   DoorID
}

// heldAgainst is the members holding the crossing between two adjacent
// cells against a mover, nearest the mover's side first, then by id: every
// holder not allied with it, of a door the mover can see as a door (one it
// has not found is wall to it, and the masquerade answers it). Empty when
// the crossing is not held against it.
func (e *Encounter) heldAgainst(held map[DoorEdge][]postHolder, mover MemberID, from, to spatial.Position) []MemberID {
	if len(held) == 0 || from == to {
		return nil
	}
	holders := held[normalizeDoorEdge(DoorEdge{From: from, To: to})]
	var out []MemberID
	for _, h := range holders {
		if h.holder == mover || e.hiddenDoorTo(mover, h.door) {
			continue
		}
		if allied, _ := e.IsAllied(mover, h.holder); allied {
			continue
		}
		out = append(out, h.holder)
	}
	sort.SliceStable(out, func(i, j int) bool {
		a, _ := e.canvas.GetEntityPosition(string(out[i]))
		b, _ := e.canvas.GetEntityPosition(string(out[j]))
		return e.Distance(from, a) < e.Distance(from, b)
	})

	return out
}

// heldOnTheWay is the holders of the first held crossing on a step's way
// from one cell to another, read fresh: the ray the canvas refuses a step
// along, crossing by crossing. Empty when nothing on the way is held
// against the mover.
//
// Errors: the participation read, which a held step cannot judge without.
func (e *Encounter) heldOnTheWay(mover MemberID, from, to spatial.Position) ([]MemberID, error) {
	if len(e.posted) == 0 {
		return nil, nil
	}
	down, err := e.downNow()
	if err != nil {
		return nil, &passageReadError{cause: err}
	}
	held := e.heldCrossings(down)
	if len(held) == 0 {
		return nil, nil
	}
	ray := spatial.CanonicalBoundaryRay(e.canvas.GetGrid(), from, to)
	for i := 1; i < len(ray); i++ {
		if holders := e.heldAgainst(held, mover, ray[i-1], ray[i]); len(holders) > 0 {
			return holders, nil
		}
	}

	return nil, nil
}

// heldEdge is [Encounter.heldAgainst] for a read: the read's holds, taken
// once from its own participation read, since nothing moves inside a read.
func (q *cellQuery) heldEdge(mover MemberID, from, to spatial.Position) bool {
	if len(q.posted) == 0 {
		return false
	}
	if !q.heldRead {
		q.held, q.heldRead = q.heldCrossings(q.down), true
	}

	return len(q.heldAgainst(q.held, mover, from, to)) > 0
}

// holdRefusal is the refusal of a step across a held crossing (H4): a
// [bodyRefusalError] whose public sentence names the nearest holder the
// mover sees now, "<name> bars the way", and "someone bars the way" when it
// sees none. The barrier it names inside is that holder, else the nearest.
func (e *Encounter) holdRefusal(mover MemberID, holders []MemberID, cell spatial.Position) error {
	barrier, who := holders[0], "someone"
	for _, h := range holders {
		if sees, err := e.currentlySees(mover, h); err == nil && sees {
			barrier, who = h, string(h)
			if m, ok := e.members[h]; ok && m.Name != "" {
				who = m.Name
			}
			break
		}
	}

	return &bodyRefusalError{barrier: barrier, at: cell, public: who + " bars the way"}
}

// ObservedHolds answers, for a viewer, the door each body it sees now holds
// against it: from its current sight testimony, so a hold the viewer has not
// seen is not told, and a body seen to step off its post is seen to hold
// nothing (H5). A body the viewer believes allied is omitted, as is a door
// hidden to the viewer. Memories and unknown locations are absent.
//
// Errors: [ErrNilInput]; a sight testimony that does not decode; the
// viewer's view or belief read.
func (e *Encounter) ObservedHolds(in *ViewInput) (map[MemberID]DoorID, error) {
	if in == nil {
		return nil, ErrNilInput
	}
	// THE STORED VIEW, because the hold is not delivered in a sighting's
	// payload ([deliveredSightPayload]): this read is its one way out, with
	// allies and hidden doors already left out.
	holdings, err := e.storedView(in)
	if err != nil {
		return nil, err
	}
	out := make(map[MemberID]DoorID)
	for _, holding := range holdings {
		if !holding.CurrentOn(perception.Sight) || holding.Subject == in.Member {
			continue
		}
		seen, ok := DecodeSightTestimony(holding.Payload)
		if !ok {
			return nil, fmt.Errorf("holds for %q: invalid sight testimony", holding.Subject)
		}
		if seen.State == LocationUnknown || seen.HoldsDoor == "" || e.hiddenDoorTo(in.Member, seen.HoldsDoor) {
			continue
		}
		stance, err := e.BelievedStance(in.Member, holding.Subject)
		if err != nil {
			return nil, fmt.Errorf("holds for %q: %w", holding.Subject, err)
		}
		if stance == StanceAllied {
			continue
		}
		out[holding.Subject] = seen.HoldsDoor
	}

	return out, nil
}

// loadPosts is [Encounter.usePosts] for a stored record, whose posts carry
// their cells: one with none is refused rather than resolved, because the
// cell it was placed at is not the cell it stands on now.
func (e *Encounter) loadPosts(m *memberRecord, stored map[string]PostData) error {
	posts, err := postsFromData(stored)
	if err != nil {
		return fmt.Errorf("member %s: %w", m.ID, err)
	}

	return e.usePosts(m, posts, spatial.Position{})
}
