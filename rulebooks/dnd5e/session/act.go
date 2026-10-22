// Copyright (C) 2026 Kirk Diggler
// SPDX-License-Identifier: GPL-3.0-or-later

package session

import (
	"context"
	"encoding/json"
	"strings"

	"github.com/KirkDiggler/rpg-toolkit/play/record"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/encounter"
)

// act.go is THE ACT PROJECTION (rpg-project#629, ideas/event-stream R5, R6,
// R16, R17, R19, R24, R27, R30, R31, Laws 6, 7, 8, 10): one recipient's record
// beats, folded into acts — who did what, to whom, how it was decided and what
// changed — and delivered as segments under one act id.
//
// ONE FUNCTION FOR LIVE AND CATCH-UP (#541 R2). [actsAfter] is the only way
// this package builds an [Act]. The commit calls it with the id of the last
// beat the recipient held before the verb; [Manager.Acts] calls it with the
// caller's AfterID. Both fold the recipient's own retained story, so a
// catch-up after a segment's id is the live suffix after it, act for act.
//
// THE FOLD READS BEATS, NEVER v1 BODIES. Every body field is decoded from the
// beat's own payload key (act_bodies.go); nothing here goes through
// [decodeBeat], an [EventKind] or an [EventBody]. The v1alpha1 projection is
// untouched by this file and untouched by it.
//
// AUDIENCES COME FROM THE BEATS. A recipient's acts are folded from that
// recipient's [encounter.Encounter.Story] entries only, so a perception beat
// audienced to one observer splits only that observer's walk, and nobody
// else's acts can carry what they did not perceive.
//
// THE SEGMENT (R19). A segment is a maximal run of one recipient's entries
// that share an act tag and fold into one body. Any other entry delivered to
// that recipient between them closes the run, and so does an entry of the
// same act that folds into a different body (a window in a walk, the
// reaction it answers). Its id is its LAST folded entry's record id (R31), so
// resuming after a segment's id starts exactly at the next entry the
// recipient holds.
//
// A RUN NEVER SPANS A COMMIT. That holds because every act that outlives one
// call is paused by a window, and the window's own beat, under the same act,
// closes the run before the call ends. TestCatchUpEqualsLive is the guard: a
// verb that continued an act across calls without a beat would fold
// differently live than on catch-up, and that test would fail.

// Family is which kind of thing an act entry is (R16). The beat decides it,
// never the verb (act_family.go holds the table).
type Family string

const (
	// FamilyAct is a verb's own deed: a walk, a swing, a cast, a line.
	FamilyAct Family = "act"
	// FamilyReaction is a deed done in answer to another act, under that
	// act's id — an opportunity attack inside the walk that provoked it.
	FamilyReaction Family = "reaction"
	// FamilyTableAnswer is a table roll: a creature's or a prop's answer, a
	// faction's temperament deal.
	FamilyTableAnswer Family = "table_answer"
	// FamilyState is a change of standing an act caused: a fall, a stance, a
	// concentration ended, a condition lifted, experience paid.
	FamilyState Family = "state"
	// FamilyPerception is what one observer newly perceived. It is folded
	// from that observer's own entries only.
	FamilyPerception Family = "perception"
	// FamilyClock is the table's own bookkeeping: ticks, turns, fights,
	// arrivals and departures, windows.
	FamilyClock Family = "clock"
)

// Act is one segment of one act, as one recipient receives it.
//
// IT CARRIES NO SEQ (R30). The record's sequence is dense and global, so a
// recipient holding two of them could count what it was not sent. The ids
// here are the record's wire id and the act tag, and nothing else.
type Act struct {
	// Session is the session the act happened in — what a host's stream
	// routes by, as [RecordEvent.Session] is.
	Session string

	// ID is the record id of the last entry this segment folds (R31). It is
	// the resume point: [Manager.Acts] after it returns what follows.
	//
	// ACTS SHARING AN ID ARE ONE DELIVERY UNIT. One record entry can fold
	// into several acts — a rest and the concentrations and conditions it
	// ended, a departure or an ending and the conditions it lifted — and
	// each carries that entry's id. They are always adjacent, a transport
	// never splits them, and resuming after the id skips them all. A broker
	// closes a lagging subscriber, and a paged read ends a page, only
	// between two ids, never inside a same-id run; cutting inside one would
	// lose the rest of it with no signal (R28).
	ID string

	// Act is the act id, the [encounter.ActTag] value `<verb>:<id>` (R27).
	// "" only for an entry outside any act, such as a tick or a join.
	Act string

	// Verb is the verb half of Act, "" when Act is "".
	Verb string

	// Recipient is the member this segment was folded for.
	Recipient string

	// At is the world-clock reading of the last folded entry.
	At uint64

	// Segment numbers this recipient's FamilyAct segments of one act, from
	// 1: a walk interrupted by a reaction is segments 1 and 2. Every other
	// family is 0 — a reaction, a state, a table answer or a perception is
	// one entry, not a part of the act's own body.
	Segment int

	// Family is what kind of entry this is.
	Family Family

	// Body is the family's typed body: one of the act bodies (WalkAct,
	// AttackAct, ...) for FamilyAct and FamilyReaction, [TableAnswer],
	// [StateChange], [Perception] or [ClockEntry]. Nil when Undecoded is set.
	Body ActBody

	// Undecoded is set when the entry's payload could not be read into its
	// family's body: the entry still takes its place in the stream, under
	// its family, with this reason and a nil Body (R22's spirit — one bad
	// beat fails that entry, not the fold).
	Undecoded string
}

// ActBody is one of the act stream's typed bodies.
type ActBody interface{ isActBody() }

// PartyKind is what a [Party] names.
type PartyKind string

const (
	// PartyMember is an encounter member — a character or a creature.
	PartyMember PartyKind = "member"
	// PartyProp is a prop.
	PartyProp PartyKind = "prop"
	// PartyFaction is a faction.
	PartyFaction PartyKind = "faction"
	// PartyWorld is the world itself — a change with no party behind it.
	PartyWorld PartyKind = "world"
)

// Party is who did something, or had it done to them (R17).
type Party struct {
	Kind PartyKind
	ID   string
}

// member is a member party, or the zero party for an empty id.
func member(id string) Party {
	if id == "" {
		return Party{}
	}
	return Party{Kind: PartyMember, ID: id}
}

// prop is a prop party, or the zero party for an empty id.
func prop(id string) Party {
	if id == "" {
		return Party{}
	}
	return Party{Kind: PartyProp, ID: id}
}

// ActStream is the live channel for the act stream. Required on [Config].
//
// Publishing is best effort, exactly as [EventStream]'s and [RecordStream]'s
// is (S10): it runs after the save has landed, a failure does not fail the
// verb, and a recipient that missed acts catches up with [Manager.Acts]
// after the last act id it holds.
type ActStream interface {
	// PublishActs delivers one commit's acts, each recipient's in record
	// order.
	PublishActs(ctx context.Context, acts []Act) error
}

// DiscardActs is an ActStream that delivers nothing — a stated "no act
// stream" for tests and headless runs, as [DiscardEvents] is for events.
type DiscardActs struct{}

// PublishActs discards the batch and reports success.
func (DiscardActs) PublishActs(_ context.Context, _ []Act) error { return nil }

// actsAfter is THE ONE FOLD: a recipient's acts whose id is after afterID,
// folded from that recipient's retained story entries (record order).
//
// It folds from the first entry of any act that has an entry after afterID,
// not from afterID itself, so a segment number and a walk's continued path
// read what came before them in the same act exactly as a fold from the
// start would — and so live (afterID = the last entry held before the verb)
// and catch-up (afterID = the caller's) fold the same entries the same way.
func actsAfter(session, recipient string, entries []record.Entry, afterID string) []Act {
	first := len(entries)
	for i, e := range entries {
		if e.ID > afterID {
			first = i
			break
		}
	}
	if first == len(entries) {
		return nil
	}
	tags := map[string]bool{}
	for _, e := range entries[first:] {
		if tag := e.Tags[encounter.ActTag]; tag != "" {
			tags[tag] = true
		}
	}
	start := first
	for i, e := range entries[:first] {
		if tags[e.Tags[encounter.ActTag]] {
			start = i
			break
		}
	}
	var out []Act
	for _, a := range foldActs(session, recipient, seedView(recipient, entries[:start]), entries[start:]) {
		if a.ID > afterID {
			out = append(out, a)
		}
	}
	return out
}

// foldActs folds one recipient's entries, in record order, into act
// segments, keeping only what the recipient perceives (act_filter.go) from
// the view they begin with. Every entry yields its pieces (act_family.go); a
// delivered piece either extends the segment the previous one left open or
// starts its own.
func foldActs(session, recipient string, view *viewState, entries []record.Entry) []Act {
	f := &fold{
		session: session, recipient: recipient, view: view,
		segments: map[string]int{}, last: map[string]ActBody{}, own: map[string]bool{}, opened: map[string]bool{},
	}
	for _, e := range entries {
		f.entry(e)
	}
	return f.out
}

// fold is foldActs's running state.
type fold struct {
	session   string
	recipient string
	out       []Act
	// open is true while the last Act in out can still absorb a piece.
	open bool
	// segments counts each act tag's FamilyAct segments.
	segments map[string]int
	// last is each act tag's most recent FamilyAct body, which a new segment
	// of the same act continues from.
	last map[string]ActBody

	// view is what the recipient perceives at the current entry.
	view *viewState
	// opened marks each act tag whose first entry the fold has read; own
	// marks those whose first entry was the recipient's — the acts they
	// opened.
	opened, own map[string]bool
	// held are pieces of members not yet in view, waiting for a sighting
	// later in their act (act_filter.go, walking into view).
	held []heldPiece
}

// heldPiece is one piece waiting to learn whether its subject comes into view.
type heldPiece struct {
	entry   record.Entry
	tag     string
	piece   piece
	subject string
	// prop is true for a prop arriving, which no sighting gains by id.
	prop bool
}

// entry filters and folds one entry.
func (f *fold) entry(e record.Entry) {
	tag := e.Tags[encounter.ActTag]
	pieces := piecesOf(f.recipient, tag, e.Payload)
	f.settle(tag, pieces)
	for _, p := range pieces {
		if tag != "" && !f.opened[tag] {
			f.opened[tag] = true
			s := Party{}
			if p.body != nil {
				s = subjectOf(p.body)
			}
			f.own[tag] = s.Kind == PartyMember && s.ID == f.recipient
		}
		if f.absorb(e, tag, p) {
			continue // part of an act the recipient is already receiving
		}
		v := f.view.judge(f.recipient, f.own[tag], p)
		if v == deliver {
			f.start(e, tag, p)
			continue
		}
		// AN UNDELIVERED PART BREAKS WHAT THE RECIPIENT HOLDS OF THE ACT: a
		// walk seen again after steps they did not see starts where they see
		// it, never from the last cell they saw.
		if p.family == FamilyAct {
			delete(f.last, tag)
		}
		if v == hold && tag != "" {
			f.held = append(f.held, heldPiece{entry: e, tag: tag, piece: p, subject: heldSubject(p.body), prop: arrivingProp(p.body)})
		}
	}
	f.view.observe(f.recipient, e.Payload)
	f.release(e, tag)
}

// settle drops the held pieces this entry ends the wait for: it is outside
// their act, or it is their subject's next step.
func (f *fold) settle(tag string, pieces []piece) {
	if len(f.held) == 0 {
		return
	}
	stepper := ""
	for _, p := range pieces {
		if w, ok := p.body.(WalkAct); ok {
			stepper = w.Mover.ID
		}
	}
	kept := f.held[:0]
	for _, h := range f.held {
		if h.tag == tag && h.subject != stepper {
			kept = append(kept, h)
		}
	}
	f.held = kept
}

// release delivers, after sighting e, the held pieces of its act whose
// subject it gains — or, for a prop arriving, whose sighting says the
// recipient's view of things changed — under e's id, since they now follow
// it.
func (f *fold) release(e record.Entry, tag string) {
	gained, knowledge := sighting(e.Payload)
	if (len(gained) == 0 && !knowledge) || len(f.held) == 0 {
		return
	}
	kept := f.held[:0]
	for _, h := range f.held {
		seen := contains(gained, h.subject) || (h.prop && knowledge)
		if h.tag != tag || !seen {
			kept = append(kept, h)
			continue
		}
		at := h.entry
		at.ID, at.At = e.ID, e.At
		if !f.absorb(at, h.tag, h.piece) {
			f.start(at, h.tag, h.piece)
		}
	}
	f.held = kept
}

// heldSubject is the member a held piece waits to see.
func heldSubject(body ActBody) string {
	switch b := body.(type) {
	case WalkAct:
		return b.Mover.ID
	case ClockEntry:
		switch {
		case b.Arrival != nil:
			return b.Arrival.ID
		case b.Departure != nil:
			return b.Departure.Member.ID
		default:
			return b.Member.ID
		}
	default:
		return ""
	}
}

// arrivingProp is whether a held piece is a prop arriving.
func arrivingProp(body ActBody) bool {
	c, ok := body.(ClockEntry)
	return ok && c.Arrival != nil && c.Arrival.Kind == encounter.ArrivedProp
}

// absorb folds piece p of entry e into the open segment when it extends it,
// and reports whether it did.
func (f *fold) absorb(e record.Entry, tag string, p piece) bool {
	if !f.open || tag == "" || len(f.out) == 0 {
		return false
	}
	prev := &f.out[len(f.out)-1]
	into := p.family
	if p.into != "" {
		into = p.into
	}
	if prev.Act != tag || prev.Family != into || p.absorb == nil || prev.Body == nil {
		return false
	}
	body, ok := p.absorb(prev.Body)
	if !ok {
		return false
	}
	prev.Body = body
	prev.ID, prev.At = e.ID, e.At
	f.last[tag] = body
	return true
}

// start places piece p of entry e as an Act of its own.
func (f *fold) start(e record.Entry, tag string, p piece) {
	verb := ""
	if i := strings.IndexByte(tag, ':'); i > 0 {
		verb = tag[:i]
	}
	a := Act{
		Session: f.session, ID: e.ID, Act: tag, Verb: verb, Recipient: f.recipient,
		At: e.At, Family: p.family, Body: p.body, Undecoded: p.undecoded,
	}
	if p.family == FamilyAct && p.body != nil {
		if prior, ok := f.last[tag]; ok && tag != "" && p.continues != nil {
			a.Body = p.continues(prior)
		}
	}
	if p.family == FamilyAct {
		f.segments[tag]++
		a.Segment = f.segments[tag]
		if tag == "" {
			a.Segment = 1
		}
		if a.Body != nil {
			f.last[tag] = a.Body
		}
	}
	f.out = append(f.out, a)
	f.open = p.absorb != nil || p.opens
}

// piece is one Act an entry contributes, and how it joins its neighbours.
type piece struct {
	family    Family
	body      ActBody
	undecoded string

	// absorb, when set, tries to fold this piece into the open segment's
	// body, returning the merged body and true, or false to start a new
	// segment. Nil for a piece that never joins one.
	absorb func(open ActBody) (ActBody, bool)

	// into is the family of the segment absorb folds into, when it is not
	// the piece's own: a speak's answer is a table answer that folds into
	// the line's act.
	into Family

	// opens is true when a later piece may fold into this one even though
	// this one never folds into another (a cast, a line, an activation).
	opens bool

	// continues, for a FamilyAct piece, restates the act's own earlier
	// segment in this one — a walk's next segment starts where the last one
	// left the mover. Nil when a segment stands alone.
	continues func(prior ActBody) ActBody
}

// peekBeat is the payload's declared beat name, "" when it names none.
func peekBeat(payload []byte) string {
	var peek struct {
		Beat string `json:"beat"`
	}
	_ = json.Unmarshal(payload, &peek) // an unreadable payload names no beat
	return peek.Beat
}
