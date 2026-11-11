// Copyright (C) 2026 Kirk Diggler
// SPDX-License-Identifier: GPL-3.0-or-later

package encounter

import (
	"errors"
	"fmt"
	"reflect"
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
//
// # Both ways, unless the setting says a direction (rpg-project#857, R6)
//
// A post bars its opening both ways by default. Its setting may name one
// direction ([Post.Against]): entering bars a crossing that starts on the
// post's side, the region its cells stand in, and leaving bars the reverse.
// Regions do not follow walls, so a directed post is refused unless every
// crossing of its door has exactly one cell on that side. The direction is
// decided by the cell a crossing starts on, so the undirected crossing memo
// never holds it.
//
// # A pass walks through
//
// A post's setting may name a pass ([Post.Pass]): requirements in the same
// grammar an option's `requires` uses, judged with the WALKER as "me" and
// the post's HOLDER as "the other party" (rpg-project#928). A walker the
// pass holds for crosses the held opening and works its door as an ally
// does. Two forms carry the gate:
//
//   - `{ fact: <id> }` — the walker knows a fact, read from the journal: a
//     password;
//   - `{ recognized: <topic> }` — the holder holds the topic about the walker
//     ([TriggerRecognized], recognition.go): the guard recognises them.
//
// The pass is per pair, so with several holders on one door the door is
// held while ANY holder it does not hold for holds it. Answered once per
// walker and holder in a read ([passReads]).

// Post is what a state holds: the door whose opening it bars, and the
// dungeon-absolute cells a holder stands on to bar it (the frame
// [JoinInput.Cell] and a table's [Selector.At] use). Empty Cells means the
// cell the member is placed at, resolved when it is admitted.
type Post struct {
	// Door is the door whose opening the post holds.
	Door DoorID

	// Cells is where a holder stands to hold it.
	Cells []spatial.Position

	// Against is which crossings of the opening the post bars: empty or
	// [PostBothWays] bars both, [PostEntering] a crossing that starts on the
	// post's side, [PostLeaving] one that ends there.
	Against PostDirection

	// Pass is what lets a mover through, as an ally walks (rpg-project#928):
	// requirements in [Option.Requires]' grammar, EVERY ONE of them Met, with
	// the mover as "me" and this post's holder as "the other party". Each is
	// a [TriggerFact] (the mover knows it), a [TriggerHolds] (the mover
	// carries it), a [TriggerRecognized] (the holder holds the topic about
	// the mover), or a `not` of one. An interaction's state is refused here:
	// it is the run's, not the pair's, and a pass on it would open the door
	// to everyone (#916 R6). Nil: no pass, nobody but allies walks through.
	Pass []Trigger
}

// PostDirection is which way a post bars its opening (rpg-project#857, R6).
// The post's side is the region its cells stand in.
type PostDirection string

const (
	// PostBothWays bars every crossing of the opening. The default: an empty
	// direction reads as this one.
	PostBothWays PostDirection = "both"

	// PostEntering bars a crossing that starts on the post's side and ends
	// on the far side.
	PostEntering PostDirection = "entering"

	// PostLeaving bars a crossing that starts on the far side and ends on
	// the post's side.
	PostLeaving PostDirection = "leaving"
)

// directed reports whether a direction bars one way only.
func (d PostDirection) directed() bool { return d == PostEntering || d == PostLeaving }

// known reports whether a direction is one of the three words, or empty.
func (d PostDirection) known() bool { return d == "" || d == PostBothWays || d.directed() }

// PostData is a [Post] as the record stores it.
type PostData struct {
	// Door is [Post.Door].
	Door DoorID `json:"door"`

	// Cells is [Post.Cells], resolved: never empty in a stored record.
	Cells []spatial.Position `json:"cells"`

	// Against is [Post.Against]. Absent in a record from before it existed,
	// which loads as both ways.
	Against PostDirection `json:"against,omitempty"`

	// Pass is [Post.Pass], each requirement in the trigger grammar's
	// persisted form. Absent: no pass.
	Pass []TriggerData `json:"pass,omitempty"`
}

// postsDataFrom is a member's posts as the record stores them, nil for none.
func postsDataFrom(posts map[string]Post) map[string]PostData {
	if len(posts) == 0 {
		return nil
	}
	out := make(map[string]PostData, len(posts))
	for state, p := range posts {
		data := PostData{Door: p.Door, Cells: slices.Clone(p.Cells), Against: p.Against}
		for _, t := range p.Pass {
			data.Pass = append(data.Pass, triggerDataFrom(t))
		}
		out[state] = data
	}

	return out
}

// postsFromData is a stored record's posts, empty when it stores none,
// refused (an error the caller wraps in [ErrInvalidData]) when a post names
// no cell: admission resolves the default, so a stored post always carries
// its cells.
func postsFromData(posts map[string]PostData) (map[string]Post, error) {
	if len(posts) == 0 {
		return map[string]Post{}, nil
	}
	out := make(map[string]Post, len(posts))
	for state, p := range posts {
		if len(p.Cells) == 0 {
			return nil, fmt.Errorf("post %q names no cell: a stored post carries its cells", state)
		}
		post := Post{Door: p.Door, Cells: slices.Clone(p.Cells), Against: p.Against}
		for i, td := range p.Pass {
			t, err := triggerFromData(td)
			if err != nil {
				return nil, fmt.Errorf("post %q pass[%d]: %w", state, i, err)
			}
			post.Pass = append(post.Pass, t)
		}
		out[state] = post
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
// names a state its recipe lacks, a door this field lacks, a cell that is
// not floor, or a direction that is not one of the three words, or names a
// direction its door cannot have ([Encounter.postSide]); and each with no
// cells resolved to `at`, the cell it is placed at.
//
// Errors: the refusal, wrapped in [ErrBadMember].
func (e *Encounter) usePosts(m *memberRecord, posts map[string]Post, at spatial.Position) error {
	if err := validatePostStates(m.behaviors.bodyRecipe(), posts); err != nil {
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
		post := Post{Door: p.Door, Cells: cells, Against: p.Against, Pass: cloneRequirements(p.Pass)}
		if err := e.validatePass(post.Pass); err != nil {
			return fmt.Errorf("member %s: post %q: %w: %w", m.ID, state, err, ErrBadMember)
		}
		if !post.Against.known() {
			return fmt.Errorf("member %s: post %q holds against %q: a post holds both, entering or leaving: %w",
				m.ID, state, post.Against, ErrBadMember)
		}
		if post.Against.directed() {
			if _, err := e.postSide(post); err != nil {
				return fmt.Errorf("member %s: post %q holds %s: %w: %w", m.ID, state, post.Against, err, ErrBadMember)
			}
		}
		out[state] = post
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
	p, ok := m.Posts[state]

	return p, ok
}

// samePost reports whether two states name the same post.
func samePost(a Post, okA bool, b Post, okB bool) bool {
	return okA == okB && a.Door == b.Door && slices.Equal(a.Cells, b.Cells) &&
		a.direction() == b.direction() && reflect.DeepEqual(a.Pass, b.Pass)
}

// direction is the post's direction, an empty one read as both ways.
func (p Post) direction() PostDirection {
	if p.Against == "" {
		return PostBothWays
	}

	return p.Against
}

// postSide is the crossing cells of a post's door that stand on the post's
// side, the region its cells stand in, in cell order with none twice: one
// per crossing. Refused unless the cells stand in one region and every
// crossing of the door has exactly one cell in it: regions do not follow
// walls (region.go), so a crossing with both cells, or neither, on the
// post's side would make a direction mean both ways, or no way.
func (e *Encounter) postSide(p Post) ([]spatial.Position, error) {
	if len(p.Cells) == 0 {
		return nil, errors.New("a post with a direction stands somewhere")
	}
	side, ok := e.field.regionOf(p.Cells[0])
	if !ok {
		return nil, fmt.Errorf("its cell %v stands in no region, so it has no side", p.Cells[0])
	}
	for _, cell := range p.Cells[1:] {
		if r, ok := e.field.regionOf(cell); !ok || r != side {
			return nil, fmt.Errorf("its cells %v and %v stand in two regions, so it has no one side", p.Cells[0], cell)
		}
	}
	var out []spatial.Position
	for _, edge := range e.doorOpening(p.Door) {
		from, _ := e.field.regionOf(edge.From)
		to, _ := e.field.regionOf(edge.To)
		switch onFrom, onTo := from == side, to == side; {
		case onFrom && onTo:
			return nil, fmt.Errorf("its door's crossing %v-%v stands wholly on its side %q", edge.From, edge.To, side)
		case !onFrom && !onTo:
			return nil, fmt.Errorf("its door's crossing %v-%v does not touch its side %q", edge.From, edge.To, side)
		case onFrom:
			out = append(out, edge.From)
		default:
			out = append(out, edge.To)
		}
	}
	sortCells(out)
	out = slices.Compact(out)

	return out, nil
}

// barsFrom reports whether a post's direction bars a crossing that starts
// at `from`: both ways bars every crossing; entering, one that starts on the
// post's side; leaving, one that starts off it. Usable only on a post its
// door admitted ([Encounter.usePosts] refuses a direction with no one side).
func (e *Encounter) barsFrom(p Post, from spatial.Position) bool {
	if !p.Against.directed() {
		return true
	}
	side, _ := e.field.regionOf(p.Cells[0])
	here, ok := e.field.regionOf(from)
	onSide := ok && here == side
	if p.Against == PostEntering {
		return onSide
	}

	return !onSide
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
	post, ok := e.heldPost(id, down)

	return post.Door, ok
}

// heldPost is [Encounter.memberHolds] answering the whole post held, its
// settings with it.
func (e *Encounter) heldPost(id MemberID, down map[MemberID]bool) (Post, bool) {
	m, ok := e.members[id]
	if !ok {
		return Post{}, false
	}
	post, ok := m.postIn(m.bodyState())
	if !ok {
		return Post{}, false
	}
	cell, placed := e.canvas.GetEntityPosition(string(id))
	if !placed || !slices.Contains(post.Cells, cell) {
		return Post{}, false
	}
	if down == nil {
		read, err := e.downNow()
		if err != nil {
			return post, true
		}
		down = read
	}
	if down[id] {
		return Post{}, false
	}

	return post, true
}

// doorOpening is a door's crossings, normalized: an edge door's edges, a
// footprint door's crossings ([field.doorCrossings]). Site data, held on the
// field ([field.doorOpening]), for the reason [Encounter.links] is.
func (e *Encounter) doorOpening(id DoorID) []DoorEdge {
	d, ok := e.doorsByID[id]
	if !ok {
		return nil
	}

	return e.openingOf(d)
}

// openingOf is [Encounter.doorOpening] for a door record in hand.
func (e *Encounter) openingOf(d *doorRecord) []DoorEdge {
	return e.field.doorOpening(d)
}

// heldCrossings is every crossing held now and its holders, in id order:
// the hold of every posted member that holds, whoever it holds against.
// Nil when nobody holds. A member removed since it was posted is asked and
// holds nothing.
func (e *Encounter) heldCrossings(down map[MemberID]bool) map[DoorEdge][]postHolder {
	if len(e.posted) == 0 {
		return nil
	}
	var out map[DoorEdge][]postHolder
	for _, id := range sortedPosted(e.posted) {
		post, holds := e.heldPost(id, down)
		if !holds {
			continue
		}
		for _, edge := range e.doorOpening(post.Door) {
			if out == nil {
				out = map[DoorEdge][]postHolder{}
			}
			out[edge] = append(out[edge], postHolder{holder: id, post: post})
		}
	}

	return out
}

// postHolder is one holder of a crossing and the post it holds there.
type postHolder struct {
	holder MemberID
	post   Post
}

// passReads is one read's answers to "does this holder's pass hold for this
// mover": judged once per mover and holder, under the promise every read
// memo rests on, that nothing moves inside a read — a holder's post, the
// mover's facts and the holder's holdings all stay put.
type passReads map[passKey]bool

// passKey is one mover and one holder.
type passKey struct {
	mover  MemberID
	holder MemberID
}

// lets is whether the holder's post lets the mover through, judged the
// first time this read asks it.
func (r passReads) lets(e *Encounter, mover, holder MemberID, p Post) bool {
	key := passKey{mover: mover, holder: holder}
	if lets, ok := r[key]; ok {
		return lets
	}
	lets := e.passLets(mover, holder, p)
	r[key] = lets

	return lets
}

// passLets is the one judgment of a post's pass (rpg-project#928): a pass
// is named and every requirement of it holds with the mover as "me" and the
// holder as "the other party" ([Encounter.requirementHolds]). No pass lets
// nobody through.
func (e *Encounter) passLets(mover, holder MemberID, p Post) bool {
	if len(p.Pass) == 0 {
		return false
	}
	for _, t := range p.Pass {
		if !e.requirementHolds(mover, holder, t) {
			return false
		}
	}

	return true
}

// validatePass refuses a pass this composition cannot judge (rpg-project#928):
// an item [validateOptionRequirement] refuses, an interaction's state (the
// run's, never the pair's: #916 R6 refused the door opened for everyone), a
// holding of a prop nobody can pick up, and a recognition on a topic no
// interaction of the run grants — each a pass nobody could ever meet.
func (e *Encounter) validatePass(pass []Trigger) error {
	granted := grantedTopics(e.interactions)
	for i, t := range pass {
		what := fmt.Sprintf("pass[%d]", i)
		if err := validateOptionRequirement(what, t); err != nil {
			return err
		}
		switch inner, _ := unwrapNot(t); r := inner.(type) {
		case TriggerInteraction:
			return fmt.Errorf("%s reads interaction %q: an interaction's state is the run's, and a pass is the holder's "+
				"and the walker's", what, r.Interaction)
		case TriggerHolds:
			if !e.field.holdable[r.Prop] {
				return fmt.Errorf("%s requires holding %q, which nobody can pick up", what, r.Prop)
			}
		case TriggerRecognized:
			if !granted[r.Topic] {
				return fmt.Errorf("%s requires recognition on topic %q, and no interaction's outcome grants it", what, r.Topic)
			}
		}
	}

	return nil
}

// bars reports whether a post bars a mover crossing from `from`, the post's
// holder and its allies already set aside: its direction bars a crossing
// starting there ([Encounter.barsFrom]), and its pass does not let the mover
// through. The direction is asked first, so a crossing it lets through
// judges no pass.
func (e *Encounter) bars(p Post, holder, mover MemberID, from spatial.Position, passes passReads) bool {
	if !e.barsFrom(p, from) {
		return false
	}

	return !passes.lets(e, mover, holder, p)
}

// heldAgainst is the members holding the crossing from one adjacent cell to
// another against a mover, nearest the mover's side first, then by id:
// every holder not allied with it, of a door the mover can see as a door
// (one it has not found is wall to it, and the masquerade answers it),
// whose post bars this mover this way ([Encounter.bars]). Empty when the
// crossing is not held against it. `passes` is the caller's read of the
// mover's pass facts, never nil.
func (e *Encounter) heldAgainst(
	held map[DoorEdge][]postHolder, mover MemberID, from, to spatial.Position, passes passReads,
) []MemberID {
	if len(held) == 0 || from == to {
		return nil
	}
	holders := held[normalizeDoorEdge(DoorEdge{From: from, To: to})]
	var out []MemberID
	for _, h := range holders {
		if h.holder == mover || e.hiddenDoorTo(mover, h.post.Door) {
			continue
		}
		if allied, _ := e.IsAllied(mover, h.holder); allied {
			continue
		}
		if !e.bars(h.post, h.holder, mover, from, passes) {
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
	passes := passReads{}
	for i := 1; i < len(ray); i++ {
		if holders := e.heldAgainst(held, mover, ray[i-1], ray[i], passes); len(holders) > 0 {
			return holders, nil
		}
	}

	return nil, nil
}

// heldEdge is [Encounter.heldAgainst] for a read, the crossing from `from`
// to `to` in that order: the read's holds, taken once from its own
// participation read, and its pass answers, read once per mover and fact,
// since nothing moves inside a read. The direction is the crossing's own and
// is decided here, never in the undirected crossing memo.
func (q *cellQuery) heldEdge(mover MemberID, from, to spatial.Position) bool {
	if len(q.posted) == 0 {
		return false
	}
	if !q.heldRead {
		q.held, q.heldRead = q.heldCrossings(q.down), true
		q.passes = passReads{}
	}

	return len(q.heldAgainst(q.held, mover, from, to, q.passes)) > 0
}

// holdRefusal is the refusal of a step across a held crossing (H4): a
// [bodyRefusalError] whose public sentence names the nearest holder the
// mover sees now, "<name> bars the way", and "someone bars the way" when it
// sees none. The barrier it names inside is that holder, else the nearest.
func (e *Encounter) holdRefusal(mover MemberID, holders []MemberID, cell spatial.Position) error {
	barrier, who := e.seenHolder(mover, holders)

	return &bodyRefusalError{barrier: barrier, at: cell, public: who + " bars the way"}
}

// HeldDoor is a door a body the viewer sees holds against it
// ([Encounter.ObservedHolds]), with the post's settings read at the time of
// asking: enough for a viewer anywhere to preview which crossings it is
// barred from.
type HeldDoor struct {
	// Door is the door whose opening is held.
	Door DoorID

	// Against is the post's direction, always one of the three words: an
	// unset one is [PostBothWays].
	Against PostDirection

	// Side is the door's crossing cells on the post's side, the region its
	// cells stand in, in cell order: one per crossing, a cell two crossings
	// share named once. A crossing starts on the post's side when it leaves
	// one of these cells. Nil when the door has no one side for the post
	// (a both-ways post whose crossings straddle no single region edge).
	Side []spatial.Position
}

// ObservedHolds answers, for a viewer, the door each body it sees now holds
// against it: from its current sight testimony, so a hold the viewer has not
// seen is not told, and a body seen to step off its post is seen to hold
// nothing (H5). The settings are read from the holder's live post for that
// door, never from the testimony: they are the post's, not something the
// viewer saw. A hold is told to a viewer in any region, whatever way it
// faces, because a direction bars the viewer some way (rpg-project#857); a
// body the viewer believes allied is omitted, as is a door hidden to the
// viewer and a post whose pass fact the viewer knows. Memories and unknown
// locations are absent.
//
// Errors: [ErrNilInput]; a sight testimony that does not decode; the
// viewer's view or belief read.
func (e *Encounter) ObservedHolds(in *ViewInput) (map[MemberID]HeldDoor, error) {
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
	out := make(map[MemberID]HeldDoor)
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
		post := e.livePost(MemberID(holding.Subject), seen.HoldsDoor)
		if e.passLets(in.Member, MemberID(holding.Subject), post) {
			continue
		}
		stance, err := e.BelievedStance(in.Member, holding.Subject)
		if err != nil {
			return nil, fmt.Errorf("holds for %q: %w", holding.Subject, err)
		}
		if stance == StanceAllied {
			continue
		}
		side, err := e.postSide(post)
		if err != nil {
			side = nil
		}
		out[holding.Subject] = HeldDoor{Door: seen.HoldsDoor, Against: post.direction(), Side: side}
	}

	return out, nil
}

// livePost is the post a member holds a door by now: its current state's,
// when that state's post names the door, else the first of its states in
// name order whose post does, else a post of the door with no settings and
// no cells. A sighting names the door; the settings are read here, at the
// time of asking.
//
// THE FALLBACK IS FOR THE GAP BETWEEN A STATE CHANGE AND THE NEXT LOOK: the
// viewer's sighting still names the door while his new state posts none
// (cowed, not yet seen to stand aside). Without it the viewer would be told
// a bare both-ways hold with no side, a direction the author never wrote.
func (e *Encounter) livePost(id MemberID, door DoorID) Post {
	m, ok := e.members[id]
	if !ok {
		return Post{Door: door}
	}
	if p, ok := m.postIn(m.bodyState()); ok && p.Door == door {
		return p
	}
	for _, state := range sortedPostStates(m.Posts) {
		if p := m.Posts[state]; p.Door == door {
			return p
		}
	}

	return Post{Door: door}
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

// DoorHeldError is the hold the door verbs read (rpg-project#835 R7): a door
// a non-allied holder holds against the actor. It is [ErrDoorHeld] and
// nothing else. [Encounter.DoorHeldFor] answers from it; the verbs answer it
// with a [DoorRefusal] on their output (rpg-project#842).
type DoorHeldError struct {
	// Door is the door refused.
	Door DoorID

	// Holder is the nearest holder the actor sees, else the nearest holder.
	Holder MemberID

	public string
}

// Error names the door and the holder, for logs.
func (e *DoorHeldError) Error() string {
	return fmt.Sprintf("door %q is held by %s: %s", e.Door, e.Holder, ErrDoorHeld)
}

// Unwrap answers [ErrDoorHeld].
func (e *DoorHeldError) Unwrap() error { return ErrDoorHeld }

// PublicReason is the sentence the actor is told: "<name> bars the door"
// for the nearest holder it sees now, "someone bars the door" when it sees
// none, so a refusal never confirms who stands where the actor cannot see.
func (e *DoorHeldError) PublicReason() string { return e.public }

// DoorHeldFor answers whether a post holds a door against an actor, by the
// hold read the door verbs make: held and the [DoorHeldError.PublicReason]
// the verb would refuse with ("<name> bars the door", or "someone bars the
// door"), or false and "". A host asks it BEFORE rolling anything for a
// verb on the door — a lock's check, a pause that check may pose — so a
// held door is answered by the holder, not the lock. The verbs still refuse
// with the typed error on their own.
//
// The door and the actor are asked as the verbs ask them, in their order:
// the probe law first, so an unknown door and a door hidden to the actor
// both answer [ErrNoDoor], byte for byte; then the actor; then reach, so a
// door out of reach answers [ErrOutOfRange] before any hold is read. An empty actor is nobody's hands and is never
// held against.
//
// Errors: [ErrNoDoor]; [ErrNotMember]; [ErrOutOfRange] and the reach
// refusals the verbs give; the participation read.
func (e *Encounter) DoorHeldFor(actor MemberID, door DoorID) (held bool, reason string, err error) {
	d, err := e.doorOf(door)
	if err != nil {
		return false, "", fmt.Errorf("door held: %w", err)
	}
	if err := e.probeDoor(d, actor); err != nil {
		return false, "", fmt.Errorf("door held: %w", err)
	}
	if err := e.doorActorOf(actor); err != nil {
		return false, "", fmt.Errorf("door held %q: %w", d.id, err)
	}
	// REACH BEFORE THE HOLD, as every verb asks it: a door out of reach is
	// answered by the distance, never by whoever holds it.
	if err := e.refuseOutOfDoorReach("door held", d, actor); err != nil {
		return false, "", err
	}
	refusal := e.refuseHeldDoor(d, actor)
	var heldErr *DoorHeldError
	switch {
	case refusal == nil:
		return false, "", nil
	case errors.As(refusal, &heldErr):
		return true, heldErr.PublicReason(), nil
	default:
		return false, "", fmt.Errorf("door held %q: %w", d.id, refusal)
	}
}

// refuseHeldDoor refuses a door verb on a door a post holds against the
// actor (R7): the same hold read the crossing makes — a holder in a holding
// state, up and on a post cell — and the same alliance rule, so the holder
// himself and his allies work the door as before. The post's settings are
// read as a crossing reads them, with the actor's own cell as the side it
// acts from: an entering post bars an actor on its side, a leaving post one
// off it, and a mover who knows the pass works the door (rpg-project#857).
// A walk never reaches this: it never opens a held door
// ([cellQuery.doorToOpen]).
func (e *Encounter) refuseHeldDoor(d *doorRecord, actor MemberID) error {
	if actor == "" || len(e.posted) == 0 {
		return nil
	}
	down, err := e.downNow()
	if err != nil {
		return &passageReadError{cause: err}
	}
	from, _ := e.canvas.GetEntityPosition(string(actor))
	passes := passReads{}
	var holders []MemberID
	for _, id := range sortedPosted(e.posted) {
		post, holds := e.heldPost(id, down)
		if !holds || post.Door != d.id || id == actor {
			continue
		}
		if allied, _ := e.IsAllied(actor, id); allied {
			continue
		}
		if !e.bars(post, id, actor, from, passes) {
			continue
		}
		holders = append(holders, id)
	}
	if len(holders) == 0 {
		return nil
	}
	sort.SliceStable(holders, func(i, j int) bool {
		a, _ := e.canvas.GetEntityPosition(string(holders[i]))
		b, _ := e.canvas.GetEntityPosition(string(holders[j]))
		return e.Distance(from, a) < e.Distance(from, b)
	})
	holder, who := e.seenHolder(actor, holders)

	return &DoorHeldError{Door: d.id, Holder: holder, public: who + " bars the door"}
}

// seenHolder is the nearest holder the actor sees now and its name, else
// the nearest holder and "someone".
func (e *Encounter) seenHolder(actor MemberID, holders []MemberID) (MemberID, string) {
	for _, h := range holders {
		if sees, err := e.currentlySees(actor, h); err == nil && sees {
			if m, ok := e.members[h]; ok && m.Name != "" {
				return h, m.Name
			}
			return h, string(h)
		}
	}

	return holders[0], "someone"
}

// sortedPosted is the posted members in id order (C8).
func sortedPosted(posted map[MemberID]bool) []MemberID {
	ids := make([]MemberID, 0, len(posted))
	for id := range posted {
		ids = append(ids, id)
	}
	slices.Sort(ids)

	return ids
}
