// Copyright (C) 2026 Kirk Diggler
// SPDX-License-Identifier: GPL-3.0-or-later

package encounter

import (
	"github.com/KirkDiggler/rpg-toolkit/core"
	"github.com/KirkDiggler/rpg-toolkit/mind/perception"
)

// sightread.go is TESTIMONY IS DECODED ONCE (rpg-project#879 S9, F17): every
// look asks each observer's held sightings where their subjects stood
// ([Encounter.addOutranged], [Encounter.headingsSeenBy]), and a garrison of a
// hundred observers re-decoded every one of those payloads from JSON on every
// step of a walk.
//
// INSTANCE STATE, SO ON THE INSTANCE (R4): what an observer holds changes
// with the game, so the decoded form lives on the [Encounter], never on the
// field. It is filled on first read after a load and is never persisted.
//
// THE STORED BYTES ARE THE KEY'S OTHER HALF. Testimony is written by
// mind/perception, which holds the payload and tells this module nothing
// about which slot a pass rewrote; a memo that waited to be told could serve
// a sighting the pass replaced. So each slot keeps the payload it decoded and
// answers only while the held payload is still those bytes: a comparison of a
// few hundred bytes where a decode was three parses. A write is seen at the
// next read, by whichever path wrote it — a look, a report, a load.

// sightSlot is one observer's holding of one subject on one channel, both
// ids qualified as perception holds them.
type sightSlot struct {
	observer, subject core.EntityID
	channel           perception.Channel
}

// sightRead is one slot's decoded testimony and the payload it was decoded
// from. ok is [DecodeSightTestimony]'s answer, so a payload that does not
// decode is not decoded twice either.
type sightRead struct {
	raw       string
	testimony SightTestimony
	ok        bool
}

// heldSight is [DecodeSightTestimony] of the payload of held, a holding
// observer holds as the store returned it (its subject and channel name the
// slot), read from [Encounter.sightReads] while that payload is unchanged.
//
// THE ANSWER IS SHARED WITH LATER READS of the same slot: a caller reads it
// and copies what it changes, and never writes through its pointers.
func (e *Encounter) heldSight(observer core.EntityID, held perception.Holding) (SightTestimony, bool) {
	payload := held.Payload
	slot := sightSlot{observer: observer, subject: held.Subject, channel: held.Channel}
	if read, ok := e.sightReads[slot]; ok && read.raw == string(payload) {
		return read.testimony, read.ok
	}
	testimony, ok := DecodeSightTestimony(payload)
	if e.sightReads == nil {
		e.sightReads = make(map[sightSlot]sightRead)
	}
	e.sightReads[slot] = sightRead{raw: string(payload), testimony: testimony, ok: ok}

	return testimony, ok
}
