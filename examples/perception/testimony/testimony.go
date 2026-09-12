// Copyright (C) 2026 Kirk Diggler
// SPDX-License-Identifier: GPL-3.0-or-later

// Package testimony holds what each observer's senses have delivered, as an
// immutable append-only log per track.
//
// # What a track is
//
// A track is one channel's continuity handle: "the noise I have been following
// behind that door". It is minted by the projection, it is opaque, and it is
// deliberately NOT derived from any entity identity — if a track id could be
// inverted to an entity, then an observer holding a sight track and a hearing
// track of the same creature could merge them by string comparison, and the
// merge would have happened without anybody deciding it.
//
// A track therefore carries no identity, which is how "something is through
// that door" is held: a track with no name attached is the ordinary case, not a
// special one.
//
// # What this package can never do
//
//   - It cannot read a payload. Payloads are opaque bytes and the emitter
//     supplies the change key, so this package can never tell a lie from the
//     truth, a fresh fact from a stale one, or two disagreeing channels from
//     two agreeing ones.
//
// Change detection splits along exactly that line. This package compares the
// emitter's [Report.ChangeKey] because it cannot read the payload, and it
// compares [Report.Locus] itself because Locus is a field this package defines.
// Leaving the locus to the emitter's key looks tidier and is a trap: a creature
// that moves while looking identical has unchanged content and a changed place,
// and an emitter that forgot to fold the place into its key would leave every
// observer holding a stale position that nothing reported as changed.
//   - It cannot fold two tracks into one answer. There is no Where is it? Two
//     tracks that disagree are returned side by side, forever, and somebody
//     above decides.
//   - It cannot revise an entry. Not by a later percept, not by a caller, not
//     to correct itself. A memory does not quietly become true. New content
//     appends; identical content only extends a watermark.
//   - It cannot report an absence as a negative claim. A track that was never
//     perceived has no entry at all — not a blank one, not a zero one.
//
// # Observed and Confirmed
//
// Every entry carries two stamps, and the pair is the whole answer to "how do I
// know this might be stale". Observed is when this content was first seen.
// Confirmed is when it was last seen to still say the same thing. Staleness is
// the caller's arithmetic over Confirmed; this package offers no opinion,
// because "is it still true" is a question about the world and this package
// cannot see the world.
package testimony

import (
	"cmp"
	"errors"
	"fmt"
	"slices"
)

// Observer is whose senses these are.
type Observer string

// Channel is a source of testimony — sight, hearing, smell, a rumour. The
// vocabulary is open and this package treats every channel identically.
type Channel string

// Sight is the one predeclared channel, for the same reason the shipped store
// predeclares it: enough code needs to say "the visual one" that spelling it as
// a literal in every caller is worse than naming it once. It gets no special
// handling anywhere — a channel is a channel.
const Sight Channel = "sight"

// TrackID is a channel's opaque continuity handle. Never an entity id.
type TrackID string

// Stamp is where a piece of testimony sits in order.
//
// It carries two coordinates because there are two kinds of order and only one
// of them survives a run.
//
// [Stamp.Seq] is the world's order — the append position of the last fact the
// world recorded. It advances when something HAPPENS, not when time passes, so a
// whole dungeon run can sit at one Seq while a great deal goes on inside it. It
// is also the only half that means anything once the run is over, which is why a
// carried belief keeps it.
//
// [Stamp.Tick] orders testimony inside one run, where the world's order stands
// still. It means nothing outside the run that minted it, and comparing ticks
// from two different runs is meaningless — which lexicographic ordering handles
// on its own, since a different run is almost always a different Seq.
//
// This package never does arithmetic on either. It orders and compares, and has
// no opinion about how much time any distance represents.
type Stamp struct {
	Seq  int
	Tick uint64
}

// Before reports whether this stamp is earlier than another.
func (s Stamp) Before(other Stamp) bool {
	if s.Seq != other.Seq {
		return s.Seq < other.Seq
	}

	return s.Tick < other.Tick
}

// After reports whether this stamp is later than another.
func (s Stamp) After(other Stamp) bool {
	return other.Before(s)
}

// Compare orders two stamps, for sorting.
func (s Stamp) Compare(other Stamp) int {
	return cmp.Or(cmp.Compare(s.Seq, other.Seq), cmp.Compare(s.Tick, other.Tick))
}

// Sentinel errors. Every returned error wraps exactly one.
var (
	// ErrNoObserver reports an empty observer.
	ErrNoObserver = errors.New("empty observer")
	// ErrNoChannel reports an empty channel.
	ErrNoChannel = errors.New("empty channel")
	// ErrNoTrack reports a report with an empty track handle.
	ErrNoTrack = errors.New("empty track")
	// ErrNoChangeKey reports a report with no change key. The emitter must
	// declare what counts as a change; this package cannot work it out.
	ErrNoChangeKey = errors.New("empty change key")
	// ErrChannelMismatch reports a percept landing on a track another channel
	// owns. Tracks belong to exactly one channel for the life of the handle.
	ErrChannelMismatch = errors.New("track belongs to another channel")
	// ErrInvalidData reports persisted state this package could not have
	// written. Every check that returns it names a state the store cannot
	// reach, so meeting one means the data came from somewhere else.
	ErrInvalidData = errors.New("invalid testimony data")
	// ErrStampRegress reports a percept older than testimony already held on
	// that track. Time only moves one way, and silently accepting a regress
	// would let a stale percept overwrite a fresh one.
	ErrStampRegress = errors.New("percept older than held testimony")
)

// Locus is a correspondence hint: roughly where and through what. It exists so
// a reconciler OUTSIDE this package has something to be wrong with, and this
// package never reads it for any purpose of its own.
type Locus struct {
	// Where is an opaque place key. Empty means the channel delivered the
	// track without placing it — known to be there, not known where.
	Where string
}

// Entry is one immutable observation. Nothing ever writes an Entry twice
// except to extend Confirmed.
type Entry struct {
	// Payload is what the channel said, opaque to this package.
	Payload []byte
	// ChangeKey is the emitter's declaration of this content's identity. Two
	// entries with the same key say the same thing.
	ChangeKey string
	// Locus is the correspondence hint carried with this observation.
	Locus Locus
	// Observed is when this content was first perceived.
	Observed Stamp
	// Confirmed is when this content was last perceived to still hold.
	Confirmed Stamp
}

// Report is one track's testimony within a percept.
type Report struct {
	Track     TrackID
	Payload   []byte
	ChangeKey string
	Locus     Locus
}

// Percept is one channel's COMPLETE delivery for one observer at one moment.
// Complete is the whole contract: a track this channel sustains and does not
// name here has stopped being current, which is how looking and finding
// nothing is a write.
type Percept struct {
	Observer Observer
	Channel  Channel
	Reports  []Report
	At       Stamp
}

// Track is the read-side value: one channel's whole log for one observer.
type Track struct {
	ID      TrackID
	Channel Channel
	// Entries is oldest-first and append-only.
	Entries []Entry
	// Current reports whether the owning channel sustained this track on its
	// last pass. False is a ghost: still held, no longer delivered.
	Current bool
}

// Latest is the most recent entry. A Track always has at least one.
func (t Track) Latest() Entry {
	return t.Entries[len(t.Entries)-1]
}

// Delta is what a percept changed about this observer's own knowledge, and
// nothing else. It names only tracks the observer holds, so a caller relaying
// it cannot leak knowledge the observer does not have.
type Delta struct {
	// FirstContact is tracks this observer had nothing for at all.
	FirstContact []TrackID
	// Changed is tracks whose latest content differs from what was held. This
	// is the moment "my knowledge changed" — and it is not the same as being
	// re-delivered.
	Changed []TrackID
	// Confirmed is tracks re-delivered saying exactly what they already said,
	// in the same place. Nothing changed; only the watermark moved. This is the
	// distinction a one-slot-per-subject store cannot draw, which is why it
	// overwrites and then needs a correction pass to undo the damage.
	Confirmed []TrackID
	// Faded is tracks the owning channel stopped sustaining. The log survives.
	Faded []TrackID
	// Reacquired is tracks that were ghosts when this pass began and are
	// current again. It refines FirstContact's opposite, not Changed.
	Reacquired []TrackID
}

type track struct {
	channel Channel
	entries []Entry
	current bool
}

// Store is every observer's testimony. Not safe for concurrent use.
type Store struct {
	tracks map[Observer]map[TrackID]*track
}

// New builds an empty store.
func New() *Store {
	return &Store{tracks: make(map[Observer]map[TrackID]*track)}
}

// Surveil lands one channel's complete percept for one observer.
//
// Validation happens before any mutation, so a rejected percept leaves the
// store untouched.
func (s *Store) Surveil(p Percept) (Delta, error) {
	if err := s.check("surveil", p.Observer, p.Channel, p.Reports, p.At); err != nil {
		return Delta{}, err
	}

	held := s.tracks[p.Observer]

	var out Delta

	named := make(map[TrackID]struct{}, len(p.Reports))
	for _, r := range p.Reports {
		named[r.Track] = struct{}{}
	}

	// Fade pass, read from pre-mutation state: a track this channel sustains
	// and this percept does not name is no longer current.
	var faded []TrackID

	for id, t := range held {
		if t.channel != p.Channel || !t.current {
			continue
		}

		if _, still := named[id]; !still {
			t.current = false

			faded = append(faded, id)
		}
	}

	slices.Sort(faded)
	out.Faded = faded

	if len(p.Reports) == 0 {
		return out, nil
	}

	if s.tracks[p.Observer] == nil {
		s.tracks[p.Observer] = make(map[TrackID]*track)
		held = s.tracks[p.Observer]
	}

	// Land pass, in percept order.
	for _, r := range p.Reports {
		t, exists := held[r.Track]
		if !exists {
			held[r.Track] = &track{
				channel: p.Channel,
				entries: []Entry{{
					Payload:   bytesCopy(r.Payload),
					ChangeKey: r.ChangeKey,
					Locus:     r.Locus,
					Observed:  p.At,
					Confirmed: p.At,
				}},
				current: true,
			}
			out.FirstContact = append(out.FirstContact, r.Track)

			continue
		}

		// Read before the write: a track current via nothing is a ghost, and
		// this is the one instant that is visible.
		wasGhost := !t.current
		t.current = true

		last := len(t.entries) - 1
		if t.entries[last].ChangeKey == r.ChangeKey && t.entries[last].Locus == r.Locus {
			// Same content. The entry is not rewritten; its watermark moves.
			t.entries[last].Confirmed = p.At
			out.Confirmed = append(out.Confirmed, r.Track)
		} else {
			t.entries = append(t.entries, Entry{
				Payload:   bytesCopy(r.Payload),
				ChangeKey: r.ChangeKey,
				Locus:     r.Locus,
				Observed:  p.At,
				Confirmed: p.At,
			})
			out.Changed = append(out.Changed, r.Track)
		}

		if wasGhost {
			out.Reacquired = append(out.Reacquired, r.Track)
		}
	}

	return out, nil
}

// check validates a landing before anything mutates, so a rejected delivery
// leaves the store exactly as it was.
func (s *Store) check(verb string, o Observer, channel Channel, reports []Report, at Stamp) error {
	if o == "" {
		return fmt.Errorf("%s: %w", verb, ErrNoObserver)
	}

	if channel == "" {
		return fmt.Errorf("%s: %w", verb, ErrNoChannel)
	}

	held := s.tracks[o]

	for _, r := range reports {
		if r.Track == "" {
			return fmt.Errorf("%s: %w", verb, ErrNoTrack)
		}

		if r.ChangeKey == "" {
			return fmt.Errorf("%s %s: %w", verb, r.Track, ErrNoChangeKey)
		}

		existing, ok := held[r.Track]
		if !ok {
			continue
		}

		if existing.channel != channel {
			return fmt.Errorf("%s %s: %w: %s", verb, r.Track, ErrChannelMismatch, existing.channel)
		}

		if latest := existing.entries[len(existing.entries)-1]; at.Before(latest.Confirmed) {
			return fmt.Errorf("%s %s at %v: %w: %v", verb, r.Track, at, ErrStampRegress, latest.Confirmed)
		}
	}

	return nil
}

// Recollection is discrete testimony: what somebody says they know, rather than
// what a channel is currently delivering.
type Recollection struct {
	Observer Observer
	Channel  Channel
	Reports  []Report
	At       Stamp
}

// Report lands discrete testimony as HELD and never as current.
//
// It is the verb for knowledge that arrives without a channel sustaining it: a
// rumour, a warning, a map somebody drew — and a belief carried out of a run
// that is over. All of those are things you know and are not currently
// perceiving, which is precisely a ghost, so they arrive as one. No pass is
// implied, so nothing fades: reporting one thing says nothing about any other.
func (s *Store) Report(in Recollection) (Delta, error) {
	if err := s.check("report", in.Observer, in.Channel, in.Reports, in.At); err != nil {
		return Delta{}, err
	}

	if len(in.Reports) == 0 {
		return Delta{}, nil
	}

	if s.tracks[in.Observer] == nil {
		s.tracks[in.Observer] = make(map[TrackID]*track)
	}

	held := s.tracks[in.Observer]

	var out Delta

	for _, r := range in.Reports {
		t, exists := held[r.Track]
		if !exists {
			held[r.Track] = &track{
				channel: in.Channel,
				entries: []Entry{{
					Payload:   bytesCopy(r.Payload),
					ChangeKey: r.ChangeKey,
					Locus:     r.Locus,
					Observed:  in.At,
					Confirmed: in.At,
				}},
				current: false,
			}
			out.FirstContact = append(out.FirstContact, r.Track)

			continue
		}

		// Currency is left exactly as it was: being told about something does
		// not mean you can see it, and it does not mean you have stopped.
		last := len(t.entries) - 1
		if t.entries[last].ChangeKey == r.ChangeKey && t.entries[last].Locus == r.Locus {
			t.entries[last].Confirmed = in.At
			out.Confirmed = append(out.Confirmed, r.Track)

			continue
		}

		t.entries = append(t.entries, Entry{
			Payload:   bytesCopy(r.Payload),
			ChangeKey: r.ChangeKey,
			Locus:     r.Locus,
			Observed:  in.At,
			Confirmed: in.At,
		})
		out.Changed = append(out.Changed, r.Track)
	}

	return out, nil
}

// Head is one track and the last thing it said.
//
// It exists because almost nothing wants the log. A decider asks what an actor
// believes NOW, and a reconciler compares the latest testimony of one track
// against another's — neither needs the trail, and copying it to reach the tip
// is the difference between a pass that costs what it should and one that pays
// for every tick that ever happened.
//
// The trail is still there and still immutable. This is a cheaper question, not
// a smaller memory.
type Head struct {
	ID      TrackID
	Channel Channel
	Entry   Entry
	Current bool
}

// Heads is the last thing each of this observer's tracks said, sorted by
// handle. Use it for anything that does not need history; use [Store.Held] when
// the trail itself is the point.
func (s *Store) Heads(o Observer) []Head {
	held := s.tracks[o]
	if len(held) == 0 {
		return nil
	}

	out := make([]Head, 0, len(held))

	for id, t := range held {
		latest := t.entries[len(t.entries)-1]
		latest.Payload = bytesCopy(latest.Payload)

		out = append(out, Head{ID: id, Channel: t.channel, Entry: latest, Current: t.current})
	}

	slices.SortFunc(out, func(a, b Head) int {
		return cmpID(a.ID, b.ID)
	})

	return out
}

// Held is every track this observer holds, sorted by handle, deep-copied,
// trail and all. An observer who has never perceived anything holds nothing —
// an empty slice, not a slice of empty tracks.
func (s *Store) Held(o Observer) []Track {
	held := s.tracks[o]
	if len(held) == 0 {
		return nil
	}

	out := make([]Track, 0, len(held))
	for id, t := range held {
		out = append(out, t.copyOut(id))
	}

	slices.SortFunc(out, func(a, b Track) int {
		return cmpID(a.ID, b.ID)
	})

	return out
}

// Track is one held track. The second return is false when this observer holds
// nothing on that handle — which means "I do not know", never "there is
// nothing there".
func (s *Store) Track(o Observer, id TrackID) (Track, bool) {
	t, ok := s.tracks[o][id]
	if !ok {
		return Track{}, false
	}

	return t.copyOut(id), true
}

func (t *track) copyOut(id TrackID) Track {
	entries := make([]Entry, len(t.entries))
	for i, e := range t.entries {
		e.Payload = bytesCopy(e.Payload)
		entries[i] = e
	}

	return Track{ID: id, Channel: t.channel, Entries: entries, Current: t.current}
}

func bytesCopy(in []byte) []byte {
	if in == nil {
		return nil
	}

	out := make([]byte, len(in))
	copy(out, in)

	return out
}

func cmpID(a, b TrackID) int {
	switch {
	case a < b:
		return -1
	case a > b:
		return 1
	default:
		return 0
	}
}
