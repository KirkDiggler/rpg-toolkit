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
// A [Mind] is a few judgments and no state of its own. Judge is the reconciler
// perception already has — which tracks are one thing. Name is what the actor
// calls a contact. Rank is which contact it would rather deal with first. Keep
// is how close it lets things get. A behaviour author writes one type.
//
// [Decide] is the ladder, and it is not the mind's to change: step away from
// what is too close, attack a live named creature in reach, walk toward what
// the mind ranks first, else pass. Rungs arrived with the use cases that paid
// for them; see README.md.
//
// # You cannot aim at what you have not named
//
// [Intent.Target] is a [belief.Name], never an entity id and never a track
// handle. A target exists only once the actor has perceived something and
// worked out what to call it, which is why naming is a judgment of the mind and
// not a step somebody performs on the monster's behalf.
//
// # Inputs and outputs
//
// Every function that takes more than one thing takes one Input; every function
// that answers more than one thing answers one Output, and an error. That is
// the toolkit's convention and it is kept here so the example reads like the
// engine it is an example for.
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

// Where is the place the actor believes this contact is: the freshest placed
// testimony across its tracks. For a live contact that is where a channel
// puts it now; for a ghost it is where it was last seen. "" means known to be
// there, not known where.
//
// There is one rule and not two, on purpose. A first draft read only current
// tracks, and use case 5 found that a ghost then had no place at all, so the
// ladder could never walk toward a memory. A current track's latest entry IS
// its placement, so the memory rule already answers for a live contact.
func (c Contact) Where() string {
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

	return where
}

// LastConfirmed is the latest moment any track in this contact was perceived
// to still hold. For a ghost, that is how long ago it was last seen; the
// arithmetic is the caller's, because the store has no opinion about whether
// a memory still holds.
func (c Contact) LastConfirmed() testimony.Stamp {
	last := c.Tracks[0].Confirmed

	for _, v := range c.Tracks[1:] {
		if last.Before(v.Confirmed) {
			last = v.Confirmed
		}
	}

	return last
}

// Kind is what the actor would say this contact is: the kind of the first
// track whose payload is a percept. A contact made only of deeds has no kind.
func (c Contact) Kind() content.Kind {
	for _, v := range c.Tracks {
		if p, err := content.Decode(v.Payload); err == nil && p.Kind != "" {
			return p.Kind
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

// Sheet is what an actor knows about itself that does not change turn to
// turn: what it is armed with.
type Sheet struct {
	// Reach is how many regions away this actor can strike. Zero is melee —
	// the same region — and one is a bow. This spike's whole geometry is
	// region grain.
	Reach int
}

// Self is the part of a situation that is not perception: the actor's own
// sheet, where it stands, and where it could step. The knowledge-only contract
// is about OTHERS; your own position and your own dungeon's static topology
// are yours to read.
type Self struct {
	Sheet
	// Where is the region the actor stands in.
	Where string
	// Adjacent is every region one step from Where. Static topology, known
	// at construction — a monster knows its own dungeon's doors. It does not
	// know who is behind them.
	Adjacent []string
	// Fences is the tracks this actor may not willingly move toward: the
	// frightened condition, spoken in the actor's own handles. A fence is a
	// RULE on the sheet, not a belief — the ladder honours it and the mind is
	// never offered it as a choice. Whether the mind also knows who frightened
	// it is a separate matter, and arrives as a deed like any other.
	Fences []testimony.TrackID
}

// Fenced reports whether this contact holds a track the actor may not
// willingly approach.
func (s Self) Fenced(c Contact) bool {
	for _, v := range c.Tracks {
		if slices.Contains(s.Fences, v.ID) {
			return true
		}
	}

	return false
}

// Beyond is the distance this spike cannot measure: not here, not next door.
const Beyond = 2

// Distance is how many steps a place is from the actor: 0 here, 1 next door,
// [Beyond] otherwise. An unplaced contact ("" — known to be there, not known
// where) is Beyond.
func (s Self) Distance(where string) int {
	switch {
	case where == "":
		return Beyond
	case where == s.Where:
		return 0
	case slices.Contains(s.Adjacent, where):
		return 1
	default:
		return Beyond
	}
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
	// Toward steps one region closer to where the actor believes a named
	// contact is. A walk toward a ghost goes where the ghost was last placed.
	Toward
	// Away steps one region further from where the actor believes a named
	// contact is. It is the whole of keeping range, and of fleeing.
	Away
)

// String names a verb for a test's benefit.
func (v Verb) String() string {
	switch v {
	case Pass:
		return "pass"
	case Attack:
		return "attack"
	case Toward:
		return "toward"
	case Away:
		return "away"
	default:
		return "verb(?)"
	}
}

// Intent is what an actor means to do, expressed entirely in what it holds.
type Intent struct {
	Verb   Verb
	Target belief.Name
}

// NameInput is a contact the mind has no word for yet.
type NameInput struct {
	Contact Contact
}

// NameOutput is what the mind calls it. Named is false when it still has no
// word, and the contact cannot be aimed at.
type NameOutput struct {
	Name  belief.Name
	Named bool
}

// RankInput is the situation to rank.
type RankInput struct {
	Situation Situation
}

// RankOutput is the situation's contacts by preference, most preferred first.
// A mind may drop contacts it would never act on.
type RankOutput struct {
	Ranked []Contact
}

// KeepInput is the situation to judge distance in.
type KeepInput struct {
	Situation Situation
}

// KeepOutput is how close this mind lets a live creature get before it would
// rather step away: 0 means it stands and fights, 1 means it keeps a region
// between them.
type KeepOutput struct {
	Regions int
}

// Mind is what makes one monster different from another. It embeds the
// perception spike's reconciler because judging which tracks are one thing IS
// the first act of a mind, and adds the questions a decision needs.
//
// Keep is a judgment, not a sheet fact — a cornered archer may decide to keep
// nothing.
type Mind interface {
	reconcile.Reconciler

	Name(in *NameInput) (*NameOutput, error)
	Rank(in *RankInput) (*RankOutput, error)
	Keep(in *KeepInput) (*KeepOutput, error)
}

// DecideInput is one actor's situation and the mind that reads it.
type DecideInput struct {
	Situation Situation
	Mind      Mind
}

// DecideOutput is what the actor means to do.
type DecideOutput struct {
	Intent Intent
}

// Decide is the ladder. It is fixed, and the mind is consulted only where the
// ladder cannot answer alone: what it prefers, and how close it lets things
// get.
//
//  0. a live named creature is nearer than the mind keeps, and there is
//     somewhere to step → Away
//  1. a live named creature is within reach → Attack
//  2. a ranked named contact, live or ghost, is placed, not here, and not
//     fenced → Toward
//  3. a fenced live creature is placed and there is somewhere to step → Away
//  4. nothing to act on → Pass
//
// Live beats remembered: a ghost is never attacked and never fled, however the
// mind ranks it — you cannot hit a memory and it cannot hit you. Rung 2 is
// where the mind's ranking decides between a live target ahead and a ghost
// behind, and the ladder does not second-guess it.
//
// A fence forbids only approach. A frightened archer with the source in reach
// still shoots (rung 1); one that cannot reach it will not walk closer (rung
// 2) and flees instead (rung 3). That is the frightened condition's rule, and
// it lives here because a rule about which intents are open is the ladder's,
// not the mind's.
func Decide(in *DecideInput) (*DecideOutput, error) {
	s := in.Situation

	ranked, err := in.Mind.Rank(&RankInput{Situation: s})
	if err != nil {
		return nil, err
	}

	keep, err := in.Mind.Keep(&KeepInput{Situation: s})
	if err != nil {
		return nil, err
	}

	canStep := len(s.Self.Adjacent) > 0

	if canStep {
		for _, c := range ranked.Ranked {
			if !c.Named || !c.Current() || !c.Creature() {
				continue
			}

			if s.Self.Distance(c.Where()) < keep.Regions {
				return &DecideOutput{Intent: Intent{Verb: Away, Target: c.Name}}, nil
			}
		}
	}

	for _, c := range ranked.Ranked {
		if !c.Named || !c.Current() || !c.Creature() {
			continue
		}

		if s.Self.Distance(c.Where()) <= s.Self.Reach {
			return &DecideOutput{Intent: Intent{Verb: Attack, Target: c.Name}}, nil
		}
	}

	for _, c := range ranked.Ranked {
		if !c.Named || c.Where() == "" || c.Where() == s.Self.Where || s.Self.Fenced(c) {
			continue
		}

		return &DecideOutput{Intent: Intent{Verb: Toward, Target: c.Name}}, nil
	}

	if canStep {
		for _, c := range ranked.Ranked {
			if !c.Named || !c.Current() || !c.Creature() || c.Where() == "" || !s.Self.Fenced(c) {
				continue
			}

			return &DecideOutput{Intent: Intent{Verb: Away, Target: c.Name}}, nil
		}
	}

	return &DecideOutput{Intent: Intent{Verb: Pass}}, nil
}
