// Copyright (C) 2026 Kirk Diggler
// SPDX-License-Identifier: GPL-3.0-or-later

// Package stage is where a deed meets the people who saw it.
//
// One thing lives here: [Land], which tells every witness what they saw, in
// their own terms. A deed happens once and is held by several people, each of
// whom may know the figures in it differently or not at all, and this is the
// only code that holds both the deed and the store to write it into.
//
// # What used to live here
//
// This package was the other half of the mind ladder: Aim resolved a name
// against the truth for a swing, Recall resolved one against the actor's own
// belief for a walk, and Step turned an intent into a place. The ladder is
// retired and so are they — what a creature does is its authored table now
// (rpg-project#465), and the table is evaluated where the world is, against
// the world's own geometry, rather than against a name a mind coined.
//
// The landing stays because it was never part of deciding. A deed being
// witnessed is a fact about perception, and it is what every table's `when`
// condition reads.
package stage

import (
	"errors"
	"fmt"

	"github.com/KirkDiggler/rpg-toolkit/core"
	"github.com/KirkDiggler/rpg-toolkit/mind/behavior/deed"
	"github.com/KirkDiggler/rpg-toolkit/mind/perception"
)

// ErrNoActor reports a deed with nobody doing it. The actor keys the subject
// the deed is held under; without one every actorless deed would land on the
// same subject and overwrite the last. A deed nobody did is a wiring fault.
var ErrNoActor = errors.New("stage: a deed has no actor")

// ErrNoVerb reports a deed that does not say what was done. The verb keys the
// subject beside the actor ([deed.Subject]), so a verbless deed would collapse
// every verbless deed of one actor onto a single subject and overwrite the
// last — the same fault as a missing actor, arriving the same way, and refused
// for the same reason. A `when` condition names a verb, so a deed without one
// is also a condition nothing could ever satisfy.
var ErrNoVerb = errors.New("stage: a deed has no verb")

// Store is what Land needs of a perception: exactly the two methods
// [perception.Perception] has, so the caller hands its own store and
// nothing wraps it. The store belongs to whoever runs the passes; the stage
// only tells it what somebody saw.
type Store interface {
	Held(observer core.EntityID) ([]perception.Holding, error)
	Report(in perception.ReportInput) (*perception.ReportOutput, error)
}

// LandInput is a deed, the store whose observers witnessed it, who did, and
// when. Who witnessed it is the caller's: a deed happens at a place, and
// whose senses reached that place is the physics perception also leaves to
// the caller.
type LandInput struct {
	Store     Store
	Deed      deed.Deed
	Witnesses []core.EntityID
	At        uint64
}

// Land tells every witness what they saw, in their own terms (R9).
// ErrNoActor if the deed has no actor, ErrNoVerb if it does not say what was
// done — both key the subject it is filed under.
//
// The deed's actor and target are named to each witness only if the witness
// currently holds them on sight, or is them: a witness who could not see the
// healer learns that a heal happened and not who did it, and a witness that
// was the target knows it was. The deed lands on the
// deeds channel, one qualified subject per figure AND KIND, through
// perception's Report door — so it is held and never current, it is judged by the
// witness's mind like everything else, and the store cannot tell it from a
// lie. Nothing here writes to anybody's sight holding.
func Land(in *LandInput) error {
	if in.Deed.Actor == "" {
		return fmt.Errorf("land: %w", ErrNoActor)
	}
	if in.Deed.Verb == "" {
		return fmt.Errorf("land: %w", ErrNoVerb)
	}

	for _, witness := range in.Witnesses {
		held, err := in.Store.Held(witness)
		if err != nil {
			return err
		}

		saw := in.Deed
		saw.Actor = seen(held, witness, in.Deed.Actor)
		saw.Target = seen(held, witness, in.Deed.Target)

		_, err = in.Store.Report(perception.ReportInput{
			Observer: witness,
			Channel:  deed.Channel,
			Reports: []perception.Presence{{
				ID:      deed.Subject(in.Deed.Actor, in.Deed.Verb),
				Payload: deed.Encode(saw),
			}},
			At: in.At,
		})
		if err != nil {
			return err
		}
	}

	return nil
}

// seen is the subject if the witness currently holds it on sight, else
// nothing — except the witness itself, which it always knows. An observer
// never perceives itself, so a witness holds no sight of itself, and
// without this a deed done TO the witness would name nobody. You know when
// you have been shot at, and you know when you did the shooting.
func seen(held []perception.Holding, witness, subject core.EntityID) core.EntityID {
	if subject == witness {
		return subject
	}

	for _, h := range held {
		if h.Subject == subject && h.CurrentOn(perception.Sight) {
			return subject
		}
	}

	return ""
}
