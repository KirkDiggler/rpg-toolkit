// Copyright (C) 2026 Kirk Diggler
// SPDX-License-Identifier: GPL-3.0-or-later

// Package stage is where an intent meets the world.
//
// A mind speaks in names. The stage is the only code that holds both a
// situation and a question for the truth, so it is the only place a name can
// be resolved against what is really there — and it is deliberately a
// separate package, so nothing that decides can import it.
//
// There are two resolutions here, and they read different things (R10). Aim
// asks the truth, for a swing: an illusion binds to nothing. Recall reads the
// actor's own belief, for a walk: a walk toward a ghost never consults truth,
// so a monster searching the wrong room is correct behaviour and never leaks
// a position.
package stage

import (
	"errors"
	"fmt"

	"github.com/KirkDiggler/rpg-toolkit/core"
	"github.com/KirkDiggler/rpg-toolkit/mind/behavior"
	"github.com/KirkDiggler/rpg-toolkit/mind/behavior/deed"
	"github.com/KirkDiggler/rpg-toolkit/mind/perception"
)

// ErrNoActor reports a deed with nobody doing it. The actor keys the subject
// the deed is held under; without one every actorless deed would land on the
// same subject and overwrite the last. A deed nobody did is a wiring fault.
var ErrNoActor = errors.New("stage: a deed has no actor")

// Truth is the one question a swing may ask of the world: is that subject
// really there. It is the caller's, the way perception's Reach is — the
// stage supplies no world of its own.
type Truth interface {
	Present(subject core.EntityID) bool
}

// AimInput is an intent, the situation it was formed in, and the truth it
// is aimed at.
type AimInput struct {
	Truth     Truth
	Situation behavior.Situation
	Intent    behavior.Intent
}

// AimOutput is the subject the swing really lands on, or "" — and "" is not
// an error. It is what acting on an illusion looks like: the belief was
// real, the swing was real, and there was nothing there.
type AimOutput struct {
	Source core.EntityID
}

// Aim resolves what an actor meant to hit against what is really there.
//
// It finds the contact the actor holds under the intent's name and asks the
// truth which of its subjects is present. A contact is the actor's claim
// that several subjects are one thing, and most of them cannot be hit: a
// chant is held under a subject qualified by channel, a deed under the deeds
// channel, and the truth knows neither — so a swing at "the chanting" lands
// on nothing, and a swing at the robed figure the chant was bundled into
// lands on the figure. The wrong merge costs the captain a claim; it never
// costs it a swing at a noise.
//
// What this does not yet decide is a contact holding two LIVE figures — a
// mind that believes two people are one. No use case has paid for that; when
// one does, the name's bearer (R4) is the subject to prefer.
func Aim(in *AimInput) (*AimOutput, error) {
	for _, c := range in.Situation.Contacts {
		if !c.Named || c.Name != in.Intent.Target {
			continue
		}

		for _, h := range c.Holdings {
			if in.Truth.Present(h.Subject) {
				return &AimOutput{Source: h.Subject}, nil
			}
		}

		return &AimOutput{}, nil
	}

	return &AimOutput{}, nil
}

// RecallInput is a situation and a name in it.
type RecallInput struct {
	Situation behavior.Situation
	Name      behavior.Name
}

// RecallOutput is where the actor believes that name is. Placed is false
// when it has no idea: known to be there, not known where.
type RecallOutput struct {
	Where  string
	Placed bool
}

// Recall is where the actor believes a named contact is. It reads the
// situation and nothing else — [behavior.Contact.Where], the freshest placed
// reading the actor holds, live or remembered.
//
// A walk resolves through Recall and never through Aim. That asymmetry is
// the whole point: a swing has to meet what is really there, but where you
// choose to walk is entirely a matter of what you believe.
func Recall(in *RecallInput) (*RecallOutput, error) {
	for _, c := range in.Situation.Contacts {
		if !c.Named || c.Name != in.Name {
			continue
		}

		where := c.Where()

		return &RecallOutput{Where: where, Placed: where != ""}, nil
	}

	return &RecallOutput{}, nil
}

// StepInput is a walking intent, the situation it was formed in, and the
// space it walks through.
type StepInput struct {
	Space     behavior.Space
	Situation behavior.Situation
	Intent    behavior.Intent
}

// StepOutput is where the actor ends up this turn. Moved is false when the
// intent cannot be walked from here, and the actor stays where it is.
type StepOutput struct {
	To    string
	Moved bool
}

// Step is where a walking intent takes the actor: one step, this turn, in
// the caller's space. Toward is the space's first step to the recalled
// place. Away is the space's step that puts the most of the world between
// them, and the space refuses a dead end — fleeing into a corner is not
// fleeing (R11).
//
// The step is the space's, because the map is the caller's. The destination
// is the actor's, because where you choose to walk is a matter of what you
// believe.
func Step(in *StepInput) (*StepOutput, error) {
	recalled, err := Recall(&RecallInput{Situation: in.Situation, Name: in.Intent.Target})
	if err != nil {
		return nil, err
	}

	if !recalled.Placed {
		return &StepOutput{}, nil
	}

	switch in.Intent.Verb {
	case behavior.Toward:
		toward, err := in.Space.Toward(&behavior.TowardInput{From: in.Situation.Self.Where, To: recalled.Where})
		if err != nil {
			return nil, err
		}

		return &StepOutput{To: toward.Next, Moved: toward.Found}, nil
	case behavior.Away:
		away, err := in.Space.Away(&behavior.AwayInput{From: in.Situation.Self.Where, AwayFrom: recalled.Where})
		if err != nil {
			return nil, err
		}

		return &StepOutput{To: away.Next, Moved: away.Found}, nil
	case behavior.Attack, behavior.Pass:
		return &StepOutput{}, nil
	default:
		return &StepOutput{}, nil
	}
}

// LandInput is a deed, the game whose actors witnessed it, who did, and
// when. Who witnessed it is the caller's: a deed happens at a place, and
// whose senses reached that place is the physics perception also leaves to
// the caller.
type LandInput struct {
	Game      *behavior.Game
	Deed      deed.Deed
	Witnesses []core.EntityID
	At        uint64
}

// Land tells every witness what they saw, in their own terms (R9).
// ErrNoActor if the deed has no actor.
//
// The deed's actor and target are named to each witness only if the witness
// currently holds them on sight: a witness who could not see the healer
// learns that a heal happened and not who did it. The deed lands on the
// deeds channel, one qualified subject per figure, through perception's
// Report door — so it is held and never current, it is judged by the
// witness's mind like everything else, and the store cannot tell it from a
// lie. Nothing here writes to anybody's sight holding.
func Land(in *LandInput) error {
	if in.Deed.Actor == "" {
		return fmt.Errorf("land: %w", ErrNoActor)
	}

	for _, witness := range in.Witnesses {
		held, err := in.Game.Held(witness)
		if err != nil {
			return err
		}

		saw := in.Deed
		saw.Actor = seen(held, in.Deed.Actor)
		saw.Target = seen(held, in.Deed.Target)

		err = in.Game.Report(perception.ReportInput{
			Observer: witness,
			Channel:  deed.Channel,
			Reports:  []perception.Presence{{ID: deed.Subject(in.Deed.Actor), Payload: deed.Encode(saw)}},
			At:       in.At,
		})
		if err != nil {
			return err
		}
	}

	return nil
}

// seen is the subject if the witness currently holds it on sight, else
// nothing.
func seen(held []perception.Holding, subject core.EntityID) core.EntityID {
	for _, h := range held {
		if h.Subject == subject && h.CurrentOn(perception.Sight) {
			return subject
		}
	}

	return ""
}
