// Copyright (C) 2026 Kirk Diggler
// SPDX-License-Identifier: GPL-3.0-or-later

// Package deed is the payload vocabulary for the deeds channel: what an
// observer would say they saw somebody DO.
//
// A deed is testimony like any other. It lands on its own channel through
// perception's Report door, one subject per figure the witness saw act, and
// it names actor and target only if the witness could see them. It is
// always in the past the moment it exists, so a deed is never current.
//
// That is the rule the whole module rests on (R9): a deed on its own
// channel, and only a mind's Judge can attach it to the figure standing
// there. The zombie never does, and so cannot know who healed whom. The easy
// road — stamping the deed onto the actor's sight holding so every observer
// knows automatically — was refused because it forecloses the dumb monster
// and the illusion in one move.
package deed

import (
	"encoding/json"
	"errors"

	"github.com/KirkDiggler/rpg-toolkit/core"
	"github.com/KirkDiggler/rpg-toolkit/mind/perception"
)

// Channel is the channel deeds land on.
const Channel perception.Channel = "deeds"

// kind is what a deed payload says it is, so a reader for percepts reads it
// as an unknown kind rather than a creature, and this decoder refuses
// anything else.
const kind = "deed"

// ErrNotADeed reports a payload this package did not write.
var ErrNotADeed = errors.New("deed: payload is not a deed")

// Subject is the subject a figure's deeds are held under: the actor's id
// qualified by channel, so it can never collide with the witness's sight
// holding of the same figure. Perception's rule 11 says to qualify; this is
// where behaviour does.
func Subject(actor core.EntityID) core.EntityID {
	return core.EntityID(string(Channel) + "|" + string(actor))
}

// Deed is what one witness saw one figure do.
type Deed struct {
	// Verb is what was done: "heal", "attack", "intimidate".
	Verb string
	// Actor is who did it, as the witness holds them. Empty means the
	// witness saw the deed and not the doer — a heal from nowhere.
	Actor core.EntityID
	// Target is who it was done to, as the witness holds them. Empty means
	// nobody, or nobody the witness could see.
	Target core.EntityID
	// Where is the region it happened in.
	Where string
}

type wire struct {
	Kind   string        `json:"kind"`
	Verb   string        `json:"verb"`
	Actor  core.EntityID `json:"actor,omitempty"`
	Target core.EntityID `json:"target,omitempty"`
	Where  string        `json:"where,omitempty"`
}

// Encode renders a deed as an opaque payload.
func Encode(d Deed) []byte {
	out, err := json.Marshal(wire{Kind: kind, Verb: d.Verb, Actor: d.Actor, Target: d.Target, Where: d.Where})
	if err != nil {
		// The struct is closed and every field marshals; a failure here
		// would be a defect in this package, not a runtime condition.
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

	return Deed{Verb: w.Verb, Actor: w.Actor, Target: w.Target, Where: w.Where}, nil
}
