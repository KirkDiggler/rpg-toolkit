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
	"github.com/KirkDiggler/rpg-toolkit/core"
	"github.com/KirkDiggler/rpg-toolkit/mind/behavior"
	"github.com/KirkDiggler/rpg-toolkit/mind/behavior/deed"
	"github.com/KirkDiggler/rpg-toolkit/mind/perception"
)

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
// truth whether the subject that BEARS the name is present. Only that
// subject: a contact is the actor's claim that several subjects are one
// thing, and the claim can be wrong. The swing goes at what was named. The
// first use case found this the hard way — resolving through any subject in
// the bundle sent a swing at a chant, and the chant was coming from someone
// else.
func Aim(in *AimInput) (*AimOutput, error) {
	for _, c := range in.Situation.Contacts {
		if !c.Named || c.Name != in.Intent.Target {
			continue
		}

		if in.Truth.Present(c.Bearer) {
			return &AimOutput{Source: c.Bearer}, nil
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
// game whose doors it walks through.
type StepInput struct {
	Game      *behavior.Game
	Situation behavior.Situation
	Intent    behavior.Intent
}

// StepOutput is where the actor ends up this turn. Moved is false when the
// intent cannot be walked from here, and the actor stays where it is.
type StepOutput struct {
	To    string
	Moved bool
}

// Step is where a walking intent takes the actor, at region grain: one step,
// this turn, along the dungeon's doors. Toward takes the first step of the
// way to the recalled region. Away takes the door that puts the most dungeon
// between them, and refuses a dead end — fleeing into a corner is not
// fleeing (R11).
//
// The route is the game's, because static topology is construction truth.
// The destination is the actor's, because where you choose to walk is a
// matter of what you believe.
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
		route, err := in.Game.Route(&behavior.RouteInput{From: in.Situation.Self.Where, To: recalled.Where})
		if err != nil {
			return nil, err
		}

		return &StepOutput{To: route.Next, Moved: route.Found}, nil
	case behavior.Away:
		far, err := in.Game.Farther(&behavior.FartherInput{From: in.Situation.Self.Where, AwayFrom: recalled.Where})
		if err != nil {
			return nil, err
		}

		return &StepOutput{To: far.Next, Moved: far.Found}, nil
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
//
// The deed's actor and target are named to each witness only if the witness
// currently holds them on sight: a witness who could not see the healer
// learns that a heal happened and not who did it. The deed lands on the
// deeds channel, one qualified subject per figure, through perception's
// Report door — so it is held and never current, it is judged by the
// witness's mind like everything else, and the store cannot tell it from a
// lie. Nothing here writes to anybody's sight holding.
func Land(in *LandInput) error {
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
