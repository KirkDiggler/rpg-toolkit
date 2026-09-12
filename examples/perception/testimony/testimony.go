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
	"errors"
	"fmt"
	"slices"
)

// Observer is whose senses these are.
type Observer string

// Channel is a source of testimony — sight, hearing, smell, a rumour. The
// vocabulary is open and this package treats every channel identically.
type Channel string

// TrackID is a channel's opaque continuity handle. Never an entity id.
type TrackID string

// Stamp is a coordinate on the world clock. It orders testimony and nothing
// else; this package never does arithmetic on it beyond comparison.
type Stamp uint64

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
	if p.Observer == "" {
		return Delta{}, fmt.Errorf("surveil: %w", ErrNoObserver)
	}

	if p.Channel == "" {
		return Delta{}, fmt.Errorf("surveil: %w", ErrNoChannel)
	}

	held := s.tracks[p.Observer]

	for _, r := range p.Reports {
		if r.Track == "" {
			return Delta{}, fmt.Errorf("surveil: %w", ErrNoTrack)
		}

		if r.ChangeKey == "" {
			return Delta{}, fmt.Errorf("surveil %s: %w", r.Track, ErrNoChangeKey)
		}

		existing, ok := held[r.Track]
		if !ok {
			continue
		}

		if existing.channel != p.Channel {
			return Delta{}, fmt.Errorf("surveil %s: %w: %s", r.Track, ErrChannelMismatch, existing.channel)
		}

		if latest := existing.entries[len(existing.entries)-1]; p.At < latest.Confirmed {
			return Delta{}, fmt.Errorf("surveil %s at %d: %w: %d", r.Track, p.At, ErrStampRegress, latest.Confirmed)
		}
	}

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

// Held is every track this observer holds, sorted by handle, deep-copied. An
// observer who has never perceived anything holds nothing — an empty slice, not
// a slice of empty tracks.
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
