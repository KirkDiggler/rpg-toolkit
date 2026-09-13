// Copyright (C) 2026 Kirk Diggler
// SPDX-License-Identifier: GPL-3.0-or-later

package behavior

import (
	"slices"

	"github.com/KirkDiggler/rpg-toolkit/core"
	"github.com/KirkDiggler/rpg-toolkit/mind/perception"
)

// Reader is how a payload becomes something a mind may know. It is the
// caller's, the way perception's Reach is: behaviour never decodes a payload
// it did not write. The one channel it did write is the deeds channel, which
// it reads itself and never offers here (R2).
//
// A payload the reader has no word for reads as the zero Reading — unplaced,
// not a creature — and is not an error. A mind may still decode it.
type Reader interface {
	Read(h perception.Holding) (*Reading, error)
}

// Reading is what behaviour needs from a payload and nothing more.
type Reading struct {
	// Where is the region the testimony places its subject in. "" is known
	// to be there, not known where.
	Where string
	// Creature is whether a live thing is there: something that can be
	// struck and can strike. A noise, a mark, a banner are not.
	Creature bool
}

// Holding is one thing an actor holds, and what behaviour read from it.
type Holding struct {
	perception.Holding
	Reading
}

// Name is what an actor calls a contact. It is the only thing an intent may
// name a target by (R5).
type Name string

// Contact is one thing the actor believes is there: the holdings its own
// mind bundled together, and what it calls them.
//
// A contact is folded fresh from the mind's claims every time a situation is
// built. It is never stored, so it is only ever as merged as the mind has
// decided it is (R3).
type Contact struct {
	// Holdings is every holding this contact bundles, by subject. Never
	// empty.
	Holdings []Holding
	// Name is what the actor calls this contact. Named is false when it has
	// no word for it yet, which is common and is not a defect.
	Name  Name
	Named bool
	// Bearer is the one subject the name is recorded on (R4). A contact is
	// folded fresh every time, so a name needs a stable handle to persist
	// on, and the bearer is it: the next situation that bundles this
	// subject finds the word already there.
	Bearer core.EntityID
}

// Holds reports whether this contact bundles a subject.
func (c Contact) Holds(subject core.EntityID) bool {
	return slices.ContainsFunc(c.Holdings, func(h Holding) bool { return h.Subject == subject })
}

// Current reports whether any channel is delivering this contact right now.
// False is a ghost: something remembered and not currently perceived. A deed
// is current on nothing by construction, so a contact made only of deeds is
// never current either.
func (c Contact) Current() bool {
	return slices.ContainsFunc(c.Holdings, func(h Holding) bool { return len(h.CurrentVia) > 0 })
}

// Creature reports whether some current holding reads as a creature.
func (c Contact) Creature() bool {
	return slices.ContainsFunc(c.Holdings, func(h Holding) bool {
		return len(h.CurrentVia) > 0 && h.Creature
	})
}

// Where is the place the actor believes this contact is: the freshest placed
// reading across its holdings. For a live contact that is where a channel
// puts it now; for a ghost it is where it was last seen. "" means known to be
// there, not known where.
//
// There is one rule and not two, on purpose (R6). A first draft read only
// current holdings, and a ghost then had no place at all, so the ladder could
// never walk toward a memory. A current holding's latest confirmation IS its
// placement, so the memory rule already answers for a live contact.
func (c Contact) Where() string {
	var (
		where string
		at    uint64
		found bool
	)

	for _, h := range c.Holdings {
		if h.Where != "" && (!found || at < h.Confirmed) {
			where, at, found = h.Where, h.Confirmed, true
		}
	}

	return where
}

// LastConfirmed is the latest moment any holding in this contact was
// perceived to still hold. For a ghost, that is how long ago it was last
// seen; the arithmetic is the mind's, because the store has no opinion about
// whether a memory still holds.
func (c Contact) LastConfirmed() uint64 {
	last := c.Holdings[0].Confirmed

	for _, h := range c.Holdings[1:] {
		last = max(last, h.Confirmed)
	}

	return last
}

// FirstObserved is the earliest moment any holding in this contact was first
// perceived: when the actor first became aware of it.
func (c Contact) FirstObserved() uint64 {
	first := c.Holdings[0].Observed

	for _, h := range c.Holdings[1:] {
		first = min(first, h.Observed)
	}

	return first
}

// Sheet is what an actor knows about itself that does not change turn to
// turn: what it is armed with.
type Sheet struct {
	// Reach is how many regions away this actor can strike. Zero is melee —
	// the same region — and one is a bow. This module's whole geometry is
	// region grain.
	Reach int
}

// Self is the part of a situation that is not perception: the actor's own
// sheet, where it stands, and where it could step. The knowledge-only
// contract is about OTHERS; your own position and your own dungeon's static
// topology are yours, and they arrive as values (R12).
type Self struct {
	Sheet
	// Where is the region the actor stands in.
	Where string
	// Adjacent is every region one step from Where. Static topology, known
	// at construction — a monster knows its own dungeon's doors. It does not
	// know who is behind them.
	Adjacent []string
	// Fences is the subjects this actor may not willingly move toward: the
	// frightened condition. A fence is a RULE on the sheet, not a belief —
	// the ladder honours it and the mind is never offered it as a choice
	// (R8). Whether the mind also knows who frightened it is a separate
	// matter, and arrives as a deed like any other.
	Fences []core.EntityID
}

// Fenced reports whether this contact holds a subject the actor may not
// willingly approach.
func (s Self) Fenced(c Contact) bool {
	return slices.ContainsFunc(c.Holdings, func(h Holding) bool { return slices.Contains(s.Fences, h.Subject) })
}

// Beyond is the distance region grain cannot measure: not here, not next
// door.
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
	Actor    core.EntityID
	Contacts []Contact
	Self     Self
	At       uint64
}

// Verb is what kind of thing an intent is. The set is sealed.
type Verb uint8

const (
	// Pass does nothing, and is a decision.
	Pass Verb = iota
	// Attack strikes a named contact the actor can currently perceive in
	// reach.
	Attack
	// Toward steps one region closer to where the actor believes a named
	// contact is. A walk toward a ghost goes where the ghost was last placed.
	Toward
	// Away steps one region further from where the actor believes a named
	// contact is. It is the whole of keeping range, and of fleeing.
	Away
)

// String names a verb.
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
	Target Name
}

// Pair is two subjects a mind claims are one thing.
type Pair struct {
	A, B core.EntityID
}

// JudgeInput is everything the actor holds, and when.
type JudgeInput struct {
	Holdings []Holding
	At       uint64
}

// JudgeOutput is the mind's claims. Same is every pair it believes to be one
// thing; the claims are folded by union, so A~B and B~C is one contact of
// three. A mind that claims nothing holds one contact per subject.
type JudgeOutput struct {
	Same []Pair
}

// NameInput is a contact the mind has no word for yet.
type NameInput struct {
	Contact Contact
}

// NameOutput is what the mind calls it. Named is false when it still has no
// word, and the contact cannot be aimed at.
type NameOutput struct {
	Name  Name
	Named bool
}

// RankInput is the situation to rank.
type RankInput struct {
	Situation Situation
}

// RankOutput is the situation's contacts by preference, most preferred
// first. A mind may drop contacts it would never act on.
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

// Mind is what makes one monster different from another: four judgments and
// no state. Judging which holdings are one thing is the first act of a mind;
// the other three are the questions a decision needs.
//
// Keep is a judgment, not a sheet fact — a cornered archer may decide to keep
// nothing.
type Mind interface {
	Judge(in *JudgeInput) (*JudgeOutput, error)
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

// Decide is the ladder (R7). It is fixed, and the mind is consulted only
// where the ladder cannot answer alone: what it prefers, and how close it
// lets things get.
//
//  0. a live named creature is nearer than the mind keeps, and there is
//     somewhere to step → Away
//  1. a live named creature is within reach → Attack
//  2. a ranked named contact, live or ghost, is placed, not here, and not
//     fenced → Toward
//  3. a fenced live creature is placed and there is somewhere to step → Away
//  4. nothing to act on → Pass
//
// Live beats remembered: a ghost is never attacked and never fled, however
// the mind ranks it — you cannot hit a memory and it cannot hit you. Rung 2
// is where the mind's ranking decides between a live target ahead and a
// ghost behind, and the ladder does not second-guess it.
//
// Every rung skips a contact with no place. Known to be there, not known
// where, is not nearer than anything and not within reach of anything, and
// an intent to step away from it could not be walked: a flee the stage
// refuses would spend the turn on nothing while a placed enemy in reach went
// unanswered.
//
// A fence forbids only approach (R8). A frightened archer with the source in
// reach still shoots (rung 1); one that cannot reach it will not walk closer
// (rung 2) and flees instead (rung 3). That is the frightened condition's
// rule, and it lives here because a rule about which intents are open is the
// ladder's, not the mind's.
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
			if !c.Named || !c.Creature() || c.Where() == "" {
				continue
			}

			if s.Self.Distance(c.Where()) < keep.Regions {
				return &DecideOutput{Intent: Intent{Verb: Away, Target: c.Name}}, nil
			}
		}
	}

	for _, c := range ranked.Ranked {
		if !c.Named || !c.Creature() || c.Where() == "" {
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
			if !c.Named || !c.Creature() || c.Where() == "" || !s.Self.Fenced(c) {
				continue
			}

			return &DecideOutput{Intent: Intent{Verb: Away, Target: c.Name}}, nil
		}
	}

	return &DecideOutput{Intent: Intent{Verb: Pass}}, nil
}
