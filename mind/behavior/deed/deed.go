// Copyright (C) 2026 Kirk Diggler
// SPDX-License-Identifier: GPL-3.0-or-later

// Package deed is the payload vocabulary for the deeds channel: what an
// observer would say they saw somebody DO.
//
// A deed is testimony like any other. It lands on its own channel through
// perception's Report door, one subject per figure the witness saw act AND
// per kind of thing they were seen to do, and it names actor and target only
// if the witness could see them. It is always in the past the moment it
// exists, so a deed is never current.
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

// kindSeparator joins the two halves of a deed subject: whose deed it is, and
// which deed of theirs. It is deed's own and NOT perception's qualifier —
// that one keeps a channel's subjects from colliding with a sight holding's,
// and this is the grammar inside deed's half of it.
//
// The result is a HANDLE and nothing parses it back apart, which is why one
// separator is enough: a mind that read a subject to find out what happened
// would be reading the filing system, and the worked minds read the payload.
const kindSeparator = "#"

// Subject is the subject one of a figure's deeds is held under: the actor's
// id and the deed's verb, qualified by channel so it can never collide with
// the witness's sight holding of the same figure. Perception's rule 11 says
// to qualify and its Qualify owns that separator; this is where behaviour
// asks.
//
// ONE SUBJECT PER ACTOR AND KIND, not per actor (rpg-project#465). The store
// keeps one payload per subject, so a subject keyed on the actor alone meant
// an actor's newest witnessed deed EVICTED every older one of theirs: a
// goblin that fled the barbarian lost its own flight the moment it watched
// that same barbarian intimidate somebody else, and its table's
// `when: { fled: { within: 3 } }` stopped holding a round into the three the
// author wrote. Private memory overwritten by public testimony, from the same
// actor, is not a thing any author can reason about.
//
// A second deed of the SAME kind by the same actor still replaces the first,
// which is the shape that was always wanted: what a creature holds about
// somebody is the freshest of each thing they were seen to do.
//
// The subject carries the actor's identity on purpose: attaching a deed to a
// figure is a claim that deeds|X and X are one thing, and a mind cannot make
// that claim without the X. What the payload says about the actor is what
// the witness could vouch for; what the subject says is the handle the store
// files it under. A mind that reads handles as testimony is reading the
// filing system, and the worked minds do not.
func Subject(actor core.EntityID, verb string) core.EntityID {
	return perception.Qualify(Channel, core.EntityID(string(actor)+kindSeparator+verb))
}

// Deed is one figure doing one thing to another, somewhere.
//
// The same struct is spoken twice. As stage.Land's input it is the fact, in
// the caller's terms: Actor is who really did it and keys the subject the
// deed is held under, so it is never empty. As the payload each witness
// holds it is the fact as that witness would say it: Actor and Target are
// kept only if the witness currently holds them on sight, and empty means
// the witness saw the deed and not the doer, or nobody it could see.
//
// One subject per actor AND VERB (see [Subject]), and perception keeps one
// payload per subject, so a witness holds an actor's latest deed OF EACH KIND
// and nothing before it. A creature that was fled from and then talked past
// holds both; a creature attacked twice holds the second blow. The narrower
// shape — one subject per actor, the actor's latest deed and nothing else —
// is what shipped first, and the price came due the first time a table read a
// private deed across rounds of public ones (rpg-project#465).
type Deed struct {
	// Verb is what was done: "heal", "attack", "intimidate".
	Verb string
	// Actor is who did it.
	Actor core.EntityID
	// Target is who it was done to. Empty means nobody.
	Target core.EntityID
	// Where is the place it happened.
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
