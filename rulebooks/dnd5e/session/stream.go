// Copyright (C) 2026 Kirk Diggler
// SPDX-License-Identifier: GPL-3.0-or-later

package session

import (
	"fmt"

	"github.com/KirkDiggler/rpg-toolkit/play/record"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/encounter"
)

// stream.go is PER-RECIPIENT DENSE NUMBERING (rpg-toolkit#1375, ruled on
// rpg-project#351 "the gap oracle"): each member's delivered stream numbers
// its own events densely at this seam, so no member ever observes a hole, and
// the record's global sequence numbers stay internal.
//
// # Why the record's own numbers stopped being deliverable
//
// The record numbers every beat globally — monotonic, gapless, one counter
// for the whole table. While every beat went to everybody (full data until
// v1.0), a member's delivered stream WAS the global stream and the numbers
// doubled as gap detection: a client that saw 7 after 5 knew it missed 6.
// Concealment ends that: a search's reveal beat is audienced to the searcher
// alone, a moved beat inside a hidden room stops at the frontier — so under
// global numbering every OTHER member would see a hole exactly where a secret
// beat happened. The hole itself is an oracle ("something hidden just
// occurred, and roughly how much"), the same side channel the probe law and
// the masquerade wall close elsewhere. Killing it while keeping the wire's
// gapless contract means each recipient gets their own dense numbering, and
// the global seq never crosses the seam.
//
// # The mechanism: a persisted cursor per member
//
// A member's number for a beat is "how many beats have ever been delivered to
// me, up to and including this one". That count cannot be recomputed from the
// record alone, because the record TRIMS (DefaultRetention, 32 beats): once a
// member's early beats age out, counting the survivors would renumber
// everything after them. So the session persists, per member, a
// [StreamCursor] — the count of that member's beats up to a global watermark
// — and every write verb extends it over the beats the verb appended, in the
// same save (S9's ordering keeps it honest: the cursor rides SessionData,
// written in the same persist as the world). Numbering therefore SURVIVES
// LOAD: the cursor is data, the surviving entries are data, and the mapping
// is arithmetic over the two.
//
// # Fail closed, and the one legal seeding
//
// A cursor whose watermark has fallen below the record's retained floor
// cannot be extended — beats between the watermark and the floor were
// trimmed uncounted, and numbers already handed out under this cursor would
// go wrong silently. That is refused loudly (ErrInvalidWorld: the stream
// cannot be renumbered), never papered over. The ABSENT cursor is different:
// no number was ever issued under it, so seeding from what the record still
// holds is exact for a new member (their beats cannot predate their join,
// which is this verb) and is the one-time migration for a session stored
// before cursors existed — a restart that restarts nothing, because no
// per-recipient number was ever delivered for that session.
//
// # What this makes true on a plain dungeon
//
// Nothing here consults concealment: the numbering law is the seam's, not the
// world's. For a member present from the first beat of an all-shared story,
// the mapping is the identity — their count and the global counter advance in
// step — so a plain dungeon's founding member sees exactly the numbers they
// always saw. A late joiner's stream starts at 1 instead of at the global
// value of their join beat: dense from their own point of view, which is the
// contract, and pinned as such rather than as identity.

// StreamCursor is one member's delivered-stream position: how many beats had
// been delivered to them as of a global watermark. Persisted on
// [SessionData.Streams]; see this file's doc for why it must be.
type StreamCursor struct {
	// UpTo is the global sequence (inclusive) through which Count is exact.
	UpTo uint64 `json:"up_to"`

	// Count is how many beats this member has been audience to, among all
	// beats with global sequence at or below UpTo — over the session's whole
	// life, trimmed ones included.
	Count uint64 `json:"count"`
}

// streamNumbers is one verb's computed numbering: for each member, the map
// from a retained entry's global sequence to that member's own delivered
// number. Built once per verb by [buildStreamNumbers] and read by event
// projection and by verb outputs.
type streamNumbers struct {
	perMember map[string]map[uint64]uint64
}

// deliveredSeq answers one member's own number for one global sequence, or
// zero for a beat that was never delivered to them — zero telling the truth:
// there is no position in their stream for it.
func (n *streamNumbers) deliveredSeq(member string, global uint64) uint64 {
	if n == nil {
		return 0
	}
	return n.perMember[member][global]
}

// buildStreamNumbers computes every ever-member's numbering from the persisted
// cursors and the retained record, and returns the advanced cursors for this
// verb's save.
//
// Deterministic by construction: the answer is a function of persisted data
// (cursors, entries) alone, so the same state numbers the same way on every
// load — which is the whole survives-load argument.
func buildStreamNumbers(
	enc *encounter.Encounter, world *encounter.EncounterData, cursors map[string]StreamCursor,
) (*streamNumbers, map[string]StreamCursor, error) {
	numbers := &streamNumbers{perMember: make(map[string]map[uint64]uint64, len(world.EverMembers))}
	advanced := make(map[string]StreamCursor, len(world.EverMembers))

	watermark := uint64(0)
	if world.Log.NextSeq > 0 {
		watermark = world.Log.NextSeq - 1
	}
	floor := retainedFloor(world.Log)

	for _, member := range world.EverMembers {
		entries, err := enc.Story(&encounter.StoryInput{Audience: member, AfterSeq: 0})
		if err != nil {
			return nil, nil, fmt.Errorf("stream for %q: %w", member, translate(err))
		}

		cursor := cursors[string(member)]
		seqs, count, err := numberEntries(string(member), entries, cursor, floor)
		if err != nil {
			return nil, nil, err
		}

		numbers.perMember[string(member)] = seqs
		advanced[string(member)] = StreamCursor{UpTo: watermark, Count: count}
	}

	return numbers, advanced, nil
}

// numberEntries assigns one member's own numbers to their surviving entries
// and reports their total delivered count. The zero cursor means "no number
// was ever issued" and seeds from the surviving entries — see the file doc
// for why that is exact for a new member and legal for a pre-cursor blob.
func numberEntries(
	member string, entries []record.Entry, cursor StreamCursor, floor uint64,
) (map[uint64]uint64, uint64, error) {
	// A cursor that has fallen behind the trim cannot be extended: beats in
	// (UpTo, floor) are gone uncounted. Refused loudly — numbers handed out
	// under this cursor would silently go wrong. The zero cursor is exempt:
	// nothing was ever issued under it, so there is nothing to go wrong.
	if (cursor != StreamCursor{}) && cursor.UpTo+1 < floor {
		return nil, 0, fmt.Errorf(
			"stream for %q: beats above cursor %d were trimmed below floor %d, "+
				"the stream cannot be renumbered: %w",
			member, cursor.UpTo, floor, ErrInvalidWorld)
	}

	settled := uint64(0)
	for _, e := range entries {
		if e.Seq <= cursor.UpTo {
			settled++
		}
	}
	if settled > cursor.Count {
		return nil, 0, fmt.Errorf(
			"stream for %q: cursor counts %d delivered up to seq %d but %d survive there: %w",
			member, cursor.Count, cursor.UpTo, settled, ErrInvalidWorld)
	}

	seqs := make(map[uint64]uint64, len(entries))
	before := cursor.Count - settled
	news := uint64(0)
	for i, e := range entries {
		if e.Seq <= cursor.UpTo {
			seqs[e.Seq] = before + uint64(i) + 1
			continue
		}
		news++
		seqs[e.Seq] = cursor.Count + news
	}

	return seqs, cursor.Count + news, nil
}

// retainedFloor is the oldest global sequence the record can still testify
// about: the first surviving entry's, or one past the end for a trimmed-empty
// log, or 1 for a fresh one (record's own fresh-log convention, NextSeq 0).
func retainedFloor(log record.LogData) uint64 {
	if len(log.Entries) > 0 {
		return log.Entries[0].Seq
	}
	if log.NextSeq > 1 {
		return log.NextSeq
	}
	return 1
}

// cursorsEqual reports whether a verb's advanced cursors match what the
// session already stores, so a verb that appended nothing does not write the
// session aggregate for a no-op.
func cursorsEqual(a, b map[string]StreamCursor) bool {
	if len(a) != len(b) {
		return false
	}
	for k, v := range a {
		if b[k] != v {
			return false
		}
	}
	return true
}
