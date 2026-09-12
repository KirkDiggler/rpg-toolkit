// Copyright (C) 2026 Kirk Diggler
// SPDX-License-Identifier: GPL-3.0-or-later

// Package act is the seam where a belief becomes a deed.
//
// A [Decider] is handed one actor's own holdings and nothing else — no world,
// no roster, no truth surface, not even the other actors' beliefs. It returns an
// [Intent]. That constraint is the whole point: a decider that could see the
// world would act on the world, and every wrong belief in the system beneath it
// would stop mattering.
//
// This package deliberately does not import world. A monster's behaviour is
// about what the monster thinks, and expressing it should not require knowing
// what a verb or a journal is. The composition maps an intent onto whatever
// acting means where it is being played.
//
// # You can only aim at what you can name
//
// [Intent.Target] is a [belief.Name], never an entity id — because an actor that
// could name an entity id would be reaching past its own senses to do it. The
// consequence is the one worth having: a target is only available once the actor
// has perceived something AND worked out what to call it, and a name that turns
// out to have been an illusion binds to nothing when the composition resolves
// it. Acting on a lie reaches empty air, and no rule had to say so.
package act

import (
	"github.com/KirkDiggler/rpg-toolkit/examples/perception/belief"
	"github.com/KirkDiggler/rpg-toolkit/examples/perception/reconcile"
	"github.com/KirkDiggler/rpg-toolkit/examples/perception/testimony"
)

// The verbs the worked deciders in this file use. They are an EXAMPLE
// vocabulary and deliberately unexported: [Intent.Verb] is a plain string
// precisely so a rulebook can name its own actions without this package ever
// learning them, and a behaviour author should name their own rather than reach
// for these.
const (
	attack   = "attack"
	approach = "approach"
	flee     = "flee"
)

// Held is one track as its holder sees it: the testimony, and what they call it
// if they have worked that out.
type Held struct {
	reconcile.TrackView

	// Name is what this actor calls the track. Named is false when they have no
	// word for it — which is common, and is not a defect.
	Name  belief.Name
	Named bool
}

// Situation is everything an actor has to go on.
type Situation struct {
	Actor    testimony.Observer
	Holds    []Held
	Contacts []belief.Contact
	At       testimony.Stamp
}

// Named is the holdings this actor has a word for, which is the only part of a
// situation that can be aimed at.
func (s Situation) Named() []Held {
	out := make([]Held, 0, len(s.Holds))

	for _, h := range s.Holds {
		if h.Named {
			out = append(out, h)
		}
	}

	return out
}

// Current is the holdings a channel is delivering right now, as opposed to the
// ones being remembered.
func (s Situation) Current() []Held {
	out := make([]Held, 0, len(s.Holds))

	for _, h := range s.Holds {
		if h.Current {
			out = append(out, h)
		}
	}

	return out
}

// Intent is what an actor means to do, expressed entirely in what they hold.
type Intent struct {
	Verb   string
	Target belief.Name
}

// Decider chooses an actor's intent. Returning false is a decision too: it
// means this actor does nothing, which is what most things do most of the time.
type Decider interface {
	Decide(s Situation) (Intent, bool)
}

// DeciderFunc adapts a function to [Decider].
type DeciderFunc func(s Situation) (Intent, bool)

// Decide calls f.
func (f DeciderFunc) Decide(s Situation) (Intent, bool) { return f(s) }

// Idle never acts.
type Idle struct{}

// Decide returns no intent, ever.
func (Idle) Decide(Situation) (Intent, bool) { return Intent{}, false }

// Cautious will not commit to something it has only heard. It is the same
// situation as [Aggressive] reaching a different answer, and the difference is
// one field: which channel the testimony arrived on.
type Cautious struct{}

// Decide attacks the first named thing it can currently SEE.
func (Cautious) Decide(s Situation) (Intent, bool) {
	for _, h := range s.Named() {
		if h.Channel != testimony.Sight || !h.Current {
			continue
		}

		return Intent{Verb: attack, Target: h.Name}, true
	}

	return Intent{}, false
}

// Wary tells a memory from a sighting and does something different with each:
// it attacks what is in front of it, and goes to look at what it only
// remembers.
//
// Nothing here asks whether the memory is still true, because nothing an actor
// holds could answer that. Going to look IS the answer, and it is a decision
// rather than a correction.
type Wary struct{}

// Decide attacks what it can see, or approaches what it remembers.
func (Wary) Decide(s Situation) (Intent, bool) {
	var remembered Intent

	for _, h := range s.Named() {
		if h.Current {
			return Intent{Verb: attack, Target: h.Name}, true
		}

		if remembered.Target == "" {
			remembered = Intent{Verb: approach, Target: h.Name}
		}
	}

	return remembered, remembered.Target != ""
}

// Cowardly runs when it can currently perceive more named things than it can
// stand. Tolerates is how many it will put up with before bolting.
//
// The intent it returns has no target, because fleeing is not aimed at
// anything — which is the honest shape, not a gap.
type Cowardly struct {
	Tolerates int
}

// Decide bolts when outnumbered by what it can perceive right now.
func (c Cowardly) Decide(s Situation) (Intent, bool) {
	facing := 0

	for _, h := range s.Named() {
		if h.Current {
			facing++
		}
	}

	if facing > c.Tolerates {
		return Intent{Verb: flee}, true
	}

	return Intent{}, false
}

// Aggressive attacks the first thing it can currently perceive and name.
//
// It is deliberately credulous: it does not ask whether what it is looking at
// is really there, because nothing an actor holds could tell it.
type Aggressive struct{}

// Decide picks a target from what is in front of the actor right now.
func (Aggressive) Decide(s Situation) (Intent, bool) {
	for _, h := range s.Holds {
		if !h.Named || !h.Current {
			continue
		}

		return Intent{Verb: attack, Target: h.Name}, true
	}

	return Intent{}, false
}
