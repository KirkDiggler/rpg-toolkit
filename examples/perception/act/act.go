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

		return Intent{Verb: "attack", Target: h.Name}, true
	}

	return Intent{}, false
}
