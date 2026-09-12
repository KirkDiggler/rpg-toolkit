// Copyright (C) 2026 Kirk Diggler
// SPDX-License-Identifier: GPL-3.0-or-later

// Package stage is where an intent meets the world.
//
// A mind speaks in names. The stage is the only code that holds both a
// situation and the truth surface, so it is the only place a name can be
// resolved against what is really there — and it is deliberately a separate
// package, so nothing that decides can import it.
//
// There are two resolutions here, and they read different things. Aim binds a
// name to the truth surface, for a swing: an illusion binds to nothing. Recall
// binds a name to the actor's own remembered locus, for a walk: a walk toward
// a ghost never consults truth, so a monster searching the wrong room is
// correct behaviour and never leaks a position.
package stage

import (
	"slices"

	"github.com/KirkDiggler/rpg-toolkit/examples/behavior"
	"github.com/KirkDiggler/rpg-toolkit/examples/behavior/deed"
	"github.com/KirkDiggler/rpg-toolkit/examples/perception/belief"
	"github.com/KirkDiggler/rpg-toolkit/examples/perception/projection"
	"github.com/KirkDiggler/rpg-toolkit/examples/perception/testimony"
)

// Deed is something that happened on the truth surface: who did what to
// whom, and where. It is spoken in ledger handles because it is a fact, and
// facts are the one thing that may be.
type Deed struct {
	Actor, Target string
	Verb          string
	Where         string
}

// WitnessesInput is the truth surface and the place something happened.
type WitnessesInput struct {
	Truth projection.Input
	Where string
}

// WitnessesOutput is everyone whose senses reached that place.
type WitnessesOutput struct {
	Observers []testimony.Observer
}

// Witnesses is everyone whose senses reached the place a deed happened, on
// any channel — decided by the same senses the projection runs on, so who
// perceived a place and who witnessed what happened there cannot disagree.
func Witnesses(in *WitnessesInput) (*WitnessesOutput, error) {
	out := make([]testimony.Observer, 0, len(in.Truth.Senses))

	for _, sense := range in.Truth.Senses {
		if !slices.Contains(sense.Reach, in.Where) || slices.Contains(out, sense.Observer) {
			continue
		}

		out = append(out, sense.Observer)
	}

	return &WitnessesOutput{Observers: out}, nil
}

// LandInput is a deed, the truth surface it happened on, the game whose
// actors witnessed it, and when.
type LandInput struct {
	Game  *behavior.Game
	Truth projection.Input
	Deed  Deed
	At    testimony.Stamp
}

// Land tells every witness what they saw, in their own terms.
//
// The deed's actor and target become each witness's OWN sight tracks of them,
// and only if the witness currently holds such a track: a witness who could
// not see the healer learns that a heal happened and not who did it. The deed
// lands on the deeds channel, one track per figure, held and never current,
// through perception's Report door — so it is testimony, it is judged by the
// witness's mind like everything else, and the store cannot tell it from a
// lie. Nothing here writes to anybody's sight track.
func Land(in *LandInput) error {
	witnesses, err := Witnesses(&WitnessesInput{Truth: in.Truth, Where: in.Deed.Where})
	if err != nil {
		return err
	}

	for _, witness := range witnesses.Observers {
		held := in.Game.Held(witness)

		saw := deed.Deed{
			Verb:   in.Deed.Verb,
			Actor:  seen(held, projection.Handle(testimony.Sight, in.Deed.Actor)),
			Target: seen(held, projection.Handle(testimony.Sight, in.Deed.Target)),
		}

		_, err := in.Game.Report(testimony.Recollection{
			Observer: witness,
			Channel:  deed.Channel,
			Reports: []testimony.Report{{
				Track:     projection.Handle(deed.Channel, in.Deed.Actor),
				Payload:   deed.Encode(saw),
				ChangeKey: deed.Key(saw),
				Locus:     testimony.Locus{Where: in.Deed.Where},
			}},
			At: in.At,
		})
		if err != nil {
			return err
		}
	}

	return nil
}

// seen is the handle if the witness currently holds it, else nothing.
func seen(held []testimony.Track, handle testimony.TrackID) testimony.TrackID {
	for _, t := range held {
		if t.ID == handle && t.Current {
			return handle
		}
	}

	return ""
}

// AimInput is an intent, the situation it was formed in, and the truth
// surface it is aimed at.
type AimInput struct {
	Truth     projection.Input
	Situation behavior.Situation
	Intent    behavior.Intent
}

// AimOutput is the ledger's own handle for what was really there, or "" —
// and "" is not an error. It is what acting on an illusion looks like: the
// belief was real, the swing was real, and there was nothing there.
type AimOutput struct {
	Source string
}

// Aim resolves what an actor meant to hit against what is really there.
//
// It finds the contact the actor holds under the intent's name and asks the
// truth surface what, if anything, is behind the track that BEARS the name.
// Only that track: a contact is the actor's claim that several tracks are one
// thing, and the claim can be wrong. The swing goes at what was named. The
// first use case found this the hard way — resolving through any track in the
// bundle sent a swing at a chant, and the chant was coming from someone else.
func Aim(in *AimInput) (*AimOutput, error) {
	bound := projection.Bind(in.Truth)

	for _, c := range in.Situation.Contacts {
		if !c.Named || c.Name != in.Intent.Target {
			continue
		}

		return &AimOutput{Source: bound[c.Bearer]}, nil
	}

	return &AimOutput{}, nil
}

// RecallInput is a situation and a name in it.
type RecallInput struct {
	Situation behavior.Situation
	Name      belief.Name
}

// RecallOutput is where the actor believes that name is. Placed is false
// when it has no idea: known to be there, not known where.
type RecallOutput struct {
	Where  string
	Placed bool
}

// Recall is where the actor believes a named contact is. It reads the
// situation and nothing else — [behavior.Contact.Where], which is the
// freshest placed testimony the actor holds, live or remembered.
//
// A walk resolves through Recall and never through Aim. That asymmetry is the
// whole point: a swing has to meet what is really there, but where you choose
// to walk is entirely a matter of what you believe.
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

// StepInput is a walking intent, the situation it was formed in, and the game
// whose doors it walks through.
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
// fleeing.
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
