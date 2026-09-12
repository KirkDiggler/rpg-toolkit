// Copyright (C) 2026 Kirk Diggler
// SPDX-License-Identifier: GPL-3.0-or-later

// Package behavior is a spike: what a monster's mind has to be.
//
// Perception is a tool behaviour holds. This package imports the perception
// example and reads what an actor holds; perception never learns an intent
// exists. That is the whole of the layering, and the arrow points one way.
//
// # The nouns
//
// A [Situation] is everything one actor has to go on: its [Contact] values as
// folded by its own mind, its [Self], and a stamp. Nothing about anyone else
// that did not arrive through a channel.
//
// A [Mind] is three judgments and no state of its own. Judge is the reconciler
// perception already has — which tracks are one thing. Name is what the actor
// calls a contact. Rank is which contact it would rather deal with first. A
// behaviour author writes one type.
//
// [Decide] is the ladder, and it is not the mind's to change: attack a live
// named contact in reach, else pass. Rungs arrive with the fixtures that pay
// for them; see README.md.
//
// # You cannot aim at what you have not named
//
// [Intent.Target] is a [belief.Name], never an entity id and never a track
// handle. A target exists only once the actor has perceived something and
// worked out what to call it, which is why naming is a judgment of the mind and
// not a step somebody performs on the monster's behalf.
package behavior

import (
	"slices"

	"github.com/KirkDiggler/rpg-toolkit/examples/perception/belief"
	"github.com/KirkDiggler/rpg-toolkit/examples/perception/content"
	"github.com/KirkDiggler/rpg-toolkit/examples/perception/reconcile"
	"github.com/KirkDiggler/rpg-toolkit/examples/perception/testimony"
)

// Contact is one thing the actor believes is there: the tracks its own mind
// bundled together, and what it calls them.
//
// A contact is folded fresh from the actor's claims every time a situation is
// built. It is never stored, so it is only ever as merged as the mind has
// decided it is.
type Contact struct {
	// Tracks is every held track this contact bundles. Never empty.
	Tracks []reconcile.TrackView
	// Name is what the actor calls this contact. Named is false when it has
	// no word for it yet, which is common and is not a defect.
	Name  belief.Name
	Named bool
	// Bearer is the one track the name is recorded on. A name attaches to a
	// track because tracks are the only stable handles — and it matters
	// which one: a swing at "the robed human" goes at the figure that was
	// named, not at every track the actor has since bundled with it. When a
	// contact is a wrong merge, this is what keeps the swing honest.
	Bearer testimony.TrackID
}

// Current reports whether any channel is delivering this contact right now.
// False is a ghost: something remembered and not currently perceived.
func (c Contact) Current() bool {
	return slices.ContainsFunc(c.Tracks, func(v reconcile.TrackView) bool { return v.Current })
}

// Creature reports whether some current track says a creature is there. It
// decodes the payload, which a mind may do: opacity is the store's property,
// and this package lives where the rulebook lives.
func (c Contact) Creature() bool {
	for _, v := range c.Tracks {
		if !v.Current {
			continue
		}

		if p, err := content.Decode(v.Payload); err == nil && p.Kind == content.Creature {
			return true
		}
	}

	return false
}

// Where is the place the actor currently puts this contact, or "" when no
// current channel could place it — known to be there, not known where.
func (c Contact) Where() string {
	for _, v := range c.Tracks {
		if v.Current && v.Locus.Where != "" {
			return v.Locus.Where
		}
	}

	return ""
}

// FirstObserved is the earliest moment any track in this contact was first
// perceived: when the actor first became aware of it.
func (c Contact) FirstObserved() testimony.Stamp {
	first := c.Tracks[0].Observed

	for _, v := range c.Tracks[1:] {
		if v.Observed.Before(first) {
			first = v.Observed
		}
	}

	return first
}

// Self is the part of a situation that is not perception: the actor's own
// sheet. The knowledge-only contract is about OTHERS; your own position is
// yours to read.
type Self struct {
	// Where is the region the actor stands in.
	Where string
}

// Situation is everything one actor has to go on.
type Situation struct {
	Actor    testimony.Observer
	Contacts []Contact
	Self     Self
	At       testimony.Stamp
}

// Verb is what kind of thing an intent is. The set is sealed.
type Verb uint8

const (
	// Pass does nothing, and is a decision.
	Pass Verb = iota
	// Attack strikes a named contact the actor can currently perceive in reach.
	Attack
)

// String names a verb for a test's benefit.
func (v Verb) String() string {
	switch v {
	case Pass:
		return "pass"
	case Attack:
		return "attack"
	default:
		return "verb(?)"
	}
}

// Intent is what an actor means to do, expressed entirely in what it holds.
type Intent struct {
	Verb   Verb
	Target belief.Name
}

// Mind is what makes one monster different from another. It embeds the
// perception spike's reconciler because judging which tracks are one thing IS
// the first act of a mind, and adds the two questions a decision needs.
type Mind interface {
	reconcile.Reconciler

	// Name is what this mind would call a contact it has no word for yet.
	// Returning false means it still has no word, and the contact cannot be
	// aimed at.
	Name(c Contact) (belief.Name, bool)

	// Rank orders the situation's contacts by preference, most preferred
	// first. It may drop contacts it would never act on.
	Rank(s Situation) []Contact
}

// Decide is the ladder. It is fixed, and the mind is consulted only where the
// ladder cannot answer alone.
//
//  1. a live named contact is a creature in reach → Attack
//  2. nothing to act on → Pass
//
// Live beats remembered: a ghost is never attacked, however the mind ranks it.
// In reach is the same region, this spike's whole geometry.
func Decide(s Situation, m Mind) Intent {
	for _, c := range m.Rank(s) {
		if !c.Named || !c.Current() || !c.Creature() {
			continue
		}

		if c.Where() != s.Self.Where {
			continue
		}

		return Intent{Verb: Attack, Target: c.Name}
	}

	return Intent{Verb: Pass}
}
