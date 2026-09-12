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

// Witnesses is everyone whose senses reached the place a deed happened, on
// any channel — decided by the same senses the projection runs on, so who
// perceived a place and who witnessed what happened there cannot disagree.
func Witnesses(in projection.Input, where string) []testimony.Observer {
	var out []testimony.Observer

	for _, sense := range in.Senses {
		if !slices.Contains(sense.Reach, where) || slices.Contains(out, sense.Observer) {
			continue
		}

		out = append(out, sense.Observer)
	}

	return out
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
func Land(g *behavior.Game, in projection.Input, d Deed, at testimony.Stamp) error {
	for _, witness := range Witnesses(in, d.Where) {
		held := g.Held(witness)

		saw := deed.Deed{
			Verb:   d.Verb,
			Actor:  seen(held, projection.Handle(testimony.Sight, d.Actor)),
			Target: seen(held, projection.Handle(testimony.Sight, d.Target)),
		}

		_, err := g.Report(testimony.Recollection{
			Observer: witness,
			Channel:  deed.Channel,
			Reports: []testimony.Report{{
				Track:     projection.Handle(deed.Channel, d.Actor),
				Payload:   deed.Encode(saw),
				ChangeKey: deed.Key(saw),
				Locus:     testimony.Locus{Where: d.Where},
			}},
			At: at,
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

// Aim resolves what an actor meant to hit against what is really there.
//
// It finds the contact the actor holds under the intent's name and asks the
// truth surface what, if anything, is behind the track that BEARS the name.
// Only that track: a contact is the actor's claim that several tracks are one
// thing, and the claim can be wrong. The swing goes at what was named. The
// first fixture found this the hard way — resolving through any track in the
// bundle sent a swing at a chant, and the chant was coming from someone else.
//
// The answer is the ledger's own handle for the thing, or "" — and "" is not
// an error. It is what acting on an illusion looks like: the belief was real,
// the swing was real, and there was nothing there.
func Aim(in projection.Input, s behavior.Situation, intent behavior.Intent) string {
	bound := projection.Bind(in)

	for _, c := range s.Contacts {
		if !c.Named || c.Name != intent.Target {
			continue
		}

		return bound[c.Bearer]
	}

	return ""
}

// Recall is where the actor believes a named contact is: the freshest placed
// testimony across the contact's tracks. It reads the situation and nothing
// else. False means the actor has no idea — known to be there, not known
// where.
//
// There is deliberately no separate rule for a live contact. A current track's
// latest entry IS its placement, so "where a channel puts it now" and "where
// it was last placed" are one question with one answer; a first draft had two
// branches and a mutant proved them equivalent. A ghost is not a special case
// of recall, only an older one.
//
// A walk resolves through Recall and never through Aim. That asymmetry is the
// whole point: a swing has to meet what is really there, but where you choose
// to walk is entirely a matter of what you believe.
func Recall(s behavior.Situation, name belief.Name) (string, bool) {
	for _, c := range s.Contacts {
		if !c.Named || c.Name != name {
			continue
		}

		var (
			where string
			at    testimony.Stamp
			found bool
		)

		for _, v := range c.Tracks {
			if v.Locus.Where != "" && (!found || at.Before(v.Confirmed)) {
				where, at, found = v.Locus.Where, v.Confirmed, true
			}
		}

		return where, found
	}

	return "", false
}

// Step is where a walking intent takes the actor, at region grain: one step,
// this turn, along the dungeon's doors. Toward takes the first step of the
// way to the recalled region. Away takes the door that puts the most dungeon
// between them, and refuses a dead end — fleeing into a corner is not
// fleeing. False means the intent cannot be walked from here, and the actor
// stays where it is.
//
// The route is the game's, because static topology is construction truth.
// The destination is the actor's, because where you choose to walk is a
// matter of what you believe.
func Step(g *behavior.Game, s behavior.Situation, intent behavior.Intent) (string, bool) {
	where, recalled := Recall(s, intent.Target)
	if !recalled {
		return "", false
	}

	switch intent.Verb {
	case behavior.Toward:
		return g.Route(s.Self.Where, where)
	case behavior.Away:
		return g.Farther(s.Self.Where, where)
	case behavior.Attack, behavior.Pass:
		return "", false
	default:
		return "", false
	}
}
