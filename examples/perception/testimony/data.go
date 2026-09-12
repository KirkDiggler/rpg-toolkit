// Copyright (C) 2026 Kirk Diggler
// SPDX-License-Identifier: GPL-3.0-or-later

package testimony

import (
	"fmt"
	"slices"
)

// Data is the persistent shape of everyone's testimony.
//
// An observer who holds nothing has no entry here, exactly as they have no
// entry in memory — the common case costs nothing to store and cannot be
// counted.
type Data struct {
	Observers map[Observer]ObserverData `json:"observers,omitempty"`
}

// ObserverData is one observer's tracks.
type ObserverData struct {
	Tracks map[TrackID]TrackData `json:"tracks,omitempty"`
}

// TrackData is one channel's log for one observer.
type TrackData struct {
	Channel Channel     `json:"channel,omitempty"`
	Entries []EntryData `json:"entries,omitempty"`
	Current bool        `json:"current,omitempty"`
}

// EntryData is one immutable observation.
type EntryData struct {
	Payload   []byte    `json:"payload,omitempty"`
	ChangeKey string    `json:"change_key,omitempty"`
	Where     string    `json:"where,omitempty"`
	Observed  StampData `json:"observed"`
	Confirmed StampData `json:"confirmed"`
}

// StampData is a stamp's two coordinates.
type StampData struct {
	Seq  int    `json:"seq,omitempty"`
	Tick uint64 `json:"tick,omitempty"`
}

// ToData returns a persistent snapshot. Everything is deep-copied, so mutating
// the result cannot reach back into the store.
func (s *Store) ToData() Data {
	if len(s.tracks) == 0 {
		return Data{}
	}

	out := Data{Observers: make(map[Observer]ObserverData, len(s.tracks))}

	for o, held := range s.tracks {
		tracks := make(map[TrackID]TrackData, len(held))

		for id, t := range held {
			entries := make([]EntryData, 0, len(t.entries))
			for _, e := range t.entries {
				entries = append(entries, EntryData{
					Payload:   bytesCopy(e.Payload),
					ChangeKey: e.ChangeKey,
					Where:     e.Locus.Where,
					Observed:  StampData{Seq: e.Observed.Seq, Tick: e.Observed.Tick},
					Confirmed: StampData{Seq: e.Confirmed.Seq, Tick: e.Confirmed.Tick},
				})
			}

			tracks[id] = TrackData{Channel: t.channel, Entries: entries, Current: t.current}
		}

		out.Observers[o] = ObserverData{Tracks: tracks}
	}

	return out
}

// Load rebuilds a store, refusing anything this package could not have written.
//
// The checks are not defensive tidiness. Every one of them names a state the
// store cannot reach on its own, so encountering it means the data came from
// somewhere else — a hand edit, a partial write, a version that did not agree
// with this one — and continuing would mean trusting testimony nobody gave.
func Load(d Data) (*Store, error) {
	s := New()

	for o, od := range d.Observers {
		if o == "" {
			return nil, fmt.Errorf("load: %w: an observer with no name", ErrInvalidData)
		}

		// An observer holds something or has no entry. An empty one would make
		// "I have never perceived anything" and "I looked and there was
		// nothing" indistinguishable in storage, and they are not the same.
		if len(od.Tracks) == 0 {
			return nil, fmt.Errorf("load %s: %w: an observer holding nothing", o, ErrInvalidData)
		}

		held := make(map[TrackID]*track, len(od.Tracks))

		for id, td := range od.Tracks {
			t, err := loadTrack(o, id, td)
			if err != nil {
				return nil, err
			}

			held[id] = t
		}

		s.tracks[o] = held
	}

	return s, nil
}

func loadTrack(o Observer, id TrackID, td TrackData) (*track, error) {
	if id == "" {
		return nil, fmt.Errorf("load %s: %w: a track with no handle", o, ErrInvalidData)
	}

	if td.Channel == "" {
		return nil, fmt.Errorf("load %s/%s: %w: a track with no channel", o, id, ErrInvalidData)
	}

	// A track IS its testimony. One with no entries is not an empty memory, it
	// is a memory of nothing, and every reader of Latest assumes it cannot
	// happen.
	if len(td.Entries) == 0 {
		return nil, fmt.Errorf("load %s/%s: %w: a track with no testimony", o, id, ErrInvalidData)
	}

	entries := make([]Entry, 0, len(td.Entries))

	for i, ed := range td.Entries {
		if ed.ChangeKey == "" {
			return nil, fmt.Errorf("load %s/%s entry %d: %w: no change key", o, id, i, ErrInvalidData)
		}

		entry := Entry{
			Payload:   bytesCopy(ed.Payload),
			ChangeKey: ed.ChangeKey,
			Locus:     Locus{Where: ed.Where},
			Observed:  Stamp{Seq: ed.Observed.Seq, Tick: ed.Observed.Tick},
			Confirmed: Stamp{Seq: ed.Confirmed.Seq, Tick: ed.Confirmed.Tick},
		}

		if entry.Confirmed.Before(entry.Observed) {
			return nil, fmt.Errorf("load %s/%s entry %d: %w: confirmed before it was observed",
				o, id, i, ErrInvalidData)
		}

		if i > 0 {
			if err := checkAppendOnly(o, id, i, entries[i-1], entry); err != nil {
				return nil, err
			}
		}

		entries = append(entries, entry)
	}

	return &track{channel: td.Channel, entries: entries, current: td.Current}, nil
}

// checkAppendOnly refuses a log that could not have been appended in this order.
func checkAppendOnly(o Observer, id TrackID, i int, prev, entry Entry) error {
	if entry.Observed.Before(prev.Confirmed) {
		return fmt.Errorf("load %s/%s entry %d: %w: observed before the entry it follows was last confirmed",
			o, id, i, ErrInvalidData)
	}

	// Two entries running that say the same thing in the same place is the one
	// shape Surveil and Report can never produce: identical content extends a
	// watermark, it does not append. Reading it back would turn one belief that
	// held for a while into two, and the difference between "still true" and
	// "true again" is the whole reason these are two stamps.
	if entry.ChangeKey == prev.ChangeKey && entry.Locus == prev.Locus {
		return fmt.Errorf("load %s/%s entry %d: %w: repeats the entry before it instead of extending it",
			o, id, i, ErrInvalidData)
	}

	return nil
}

// Held is every observer with testimony, sorted. Persistence needs a way to
// enumerate, and a read that walks the store cannot be the one that leaks the
// ability to ask about somebody else.
func (d Data) Held() []Observer {
	out := make([]Observer, 0, len(d.Observers))
	for o := range d.Observers {
		out = append(out, o)
	}

	slices.Sort(out)

	return out
}
