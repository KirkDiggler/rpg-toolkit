// Copyright (C) 2026 Kirk Diggler
// SPDX-License-Identifier: GPL-3.0-or-later

// Package deed is the payload vocabulary for the deeds channel: what an
// observer would say they saw somebody DO.
//
// A deed is testimony like any other. It lands on its own channel, one track
// per figure the witness saw act, and it refers to actor and target by the
// witness's OWN tracks of them — never by an entity id. It is always in the
// past the moment it exists, so a deeds track is never current.
//
// That is the rule the whole behaviour example rests on: a deed on its own
// channel, and only a mind's Judge can attach it to the figure standing there.
// The zombie never does, and so cannot know who healed whom. The easy road —
// stamping the deed onto the actor's sight track so every observer knows
// automatically — was refused because it forecloses the dumb monster and the
// illusion in one move.
package deed

import (
	"encoding/json"
	"errors"

	"github.com/KirkDiggler/rpg-toolkit/examples/perception/testimony"
)

// Channel is the channel deeds land on.
const Channel testimony.Channel = "deeds"

// kind is what a deed payload says it is, so a decoder for percepts reads it
// as an unknown kind rather than a creature, and this decoder refuses
// anything else.
const kind = "deed"

// ErrNotADeed reports a payload this package did not write.
var ErrNotADeed = errors.New("deed: payload is not a deed")

// Deed is what one witness saw one figure do.
type Deed struct {
	// Verb is what was done: "heal", "attack", "intimidate".
	Verb string
	// Actor is the witness's own sight track of who did it. Empty means the
	// witness saw the deed and not the doer — a heal from nowhere.
	Actor testimony.TrackID
	// Target is the witness's own sight track of who it was done to. Empty
	// means nobody, or nobody the witness could see.
	Target testimony.TrackID
}

type wire struct {
	Kind   string            `json:"kind"`
	Verb   string            `json:"verb"`
	Actor  testimony.TrackID `json:"actor,omitempty"`
	Target testimony.TrackID `json:"target,omitempty"`
}

// Encode renders a deed as an opaque payload.
func Encode(d Deed) []byte {
	out, err := json.Marshal(wire{Kind: kind, Verb: d.Verb, Actor: d.Actor, Target: d.Target})
	if err != nil {
		// The struct is closed and every field marshals; a failure here would
		// be a defect in this package, not a runtime condition.
		panic("deed: encode: " + err.Error())
	}

	return out
}

// Decode reads a deed back. A payload that is not a deed is not an error the
// store could have caught; only this package can tell.
func Decode(payload []byte) (Deed, error) {
	var w wire
	if err := json.Unmarshal(payload, &w); err != nil {
		return Deed{}, err
	}

	if w.Kind != kind {
		return Deed{}, ErrNotADeed
	}

	return Deed{Verb: w.Verb, Actor: w.Actor, Target: w.Target}, nil
}

// Key is the change key a deed lands under. Two deeds with the same key say
// the same thing; a second heal of the same target by the same figure extends
// the watermark rather than appending, which is right — what the witness knows
// has not changed, only how recently it was confirmed.
func Key(d Deed) string {
	return kind + "|" + d.Verb + "|" + string(d.Actor) + "|" + string(d.Target)
}
