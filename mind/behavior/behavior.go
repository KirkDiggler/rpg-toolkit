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
	// Where is the place the testimony puts its subject. What a place is —
	// a room, a cell — is the caller's; behaviour only compares them and asks
	// the caller's Space how far apart they are. "" is known to be there, not
	// known where.
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
	// subject finds the word already there. It is never a deeds handle
	// while the contact holds anything else, so a caller may match a real
	// figure on it.
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
	// Reach is how far away this actor can strike, in whatever unit the
	// caller's Space measures. Zero is the same place — melee, at room grain.
	Reach int
}

// Self is the part of a situation that is not perception: the actor's own
// sheet, where it stands, and what it may not approach. The knowledge-only
// contract is about OTHERS; your own position is yours, and it arrives as a
// value (R12).
type Self struct {
	Sheet
	// Where is the place the actor stands.
	Where string
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

// DistanceInput is two places.
type DistanceInput struct {
	From, To string
}

// DistanceOutput is how far apart they are, in the Space's own unit. Known
// is false when the Space cannot say — no way between them, or a place it
// has never heard of — and the ladder treats what it cannot measure as
// neither near nor in reach.
type DistanceOutput struct {
	Steps int
	Known bool
}

// TowardInput is where an actor is and where it wants to be.
type TowardInput struct {
	From, To string
}

// TowardOutput is where one step toward it lands, if there is a way.
type TowardOutput struct {
	Next  string
	Found bool
}

// AwayInput is where an actor is and what it wants more distance from.
type AwayInput struct {
	From, AwayFrom string
}

// AwayOutput is where one step away lands. Found is false when every way
// leads closer or nowhere — a dead end. Fleeing into a corner is not
// fleeing, and it is the Space that knows where the corners are.
type AwayOutput struct {
	Next  string
	Found bool
}

// Space is the caller's geometry: how far apart two places are, and where
// one step toward or away from a place lands. It is the third caller-owned
// seam after perception's Reach and this package's Reader, and for the same
// reason: a monster may know the way through its own dungeon, and how the
// dungeon is measured is the dungeon's business, not the mind's (R11).
type Space interface {
	Distance(in *DistanceInput) (*DistanceOutput, error)
	Toward(in *TowardInput) (*TowardOutput, error)
	Away(in *AwayInput) (*AwayOutput, error)
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
	// Toward steps one step closer to where the actor believes a named
	// contact is. A walk toward a ghost goes where the ghost was last placed.
	Toward
	// Away steps one step further from where the actor believes a named
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

	// Contact is the creature the question is about. ASKED PER CREATURE,
	// not once per turn (rpg-project#454): a mind may keep its distance
	// from one figure and none at all from another, which is what fear is
	// — the frightened goblin runs from the fighter who threatened it and
	// walks past the wizard beside them.
	//
	// It is always a contact rung 0 is measuring; there is no "keep, in
	// general" question and no zero Contact to answer. A mind with one
	// answer for everybody ignores this field, which every mind that
	// predates it already does.
	Contact Contact
}

// KeepOutput is how close this mind lets THIS live creature get before it
// would rather step away, in the space's own unit: 0 means it stands and
// fights, 1 means it keeps a step between them.
type KeepOutput struct {
	Steps int
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

// DecideInput is one actor's situation, the mind that reads it, and the
// space it stands in.
type DecideInput struct {
	Situation Situation
	Mind      Mind
	Space     Space
}

// DecideOutput is what the actor means to do.
type DecideOutput struct {
	Intent Intent
}

// Decide is the ladder (R7). It is fixed, and the mind is consulted only
// where the ladder cannot answer alone: what it prefers, and how close it
// lets things get. The space is consulted for what only a map can know: how
// far, and whether there is anywhere to step.
//
//  0. a live named creature is nearer than the mind keeps FROM THAT
//     CREATURE, and the space finds a step away → Away
//  1. a live named creature is within reach → Attack
//  2. a ranked named contact, live or ghost, is placed, not here, and not
//     fenced → Toward
//  3. a fenced live creature is placed → Away
//  4. nothing to act on → Pass
//
// Live beats remembered: a ghost is never attacked and never fled, however
// the mind ranks it — you cannot hit a memory and it cannot hit you. Rung 2
// is where the mind's ranking decides between a live target ahead and a
// ghost behind, and the ladder does not second-guess it.
//
// Every rung skips a contact with no place, and rungs 0 and 1 skip one the
// space cannot measure. Known to be there, not known where, is not nearer
// than anything and not within reach of anything.
//
// Rung 0 asks the mind once PER CREATURE rather than once per turn
// (rpg-project#454), so "I keep my distance from her and nobody else" is a
// thing a mind can say. The ladder is unchanged by it: the rungs, their
// order, and what each one skips are exactly what they were, and a mind with
// one answer for everybody behaves identically.
//
// The two flights differ on purpose. Keeping range (rung 0) is a preference:
// an archer that cannot step away stands and shoots. Fear (rung 3) is not:
// a frightened creature with nowhere to go still means to flee, the stage
// finds it nowhere, and it stays where it is — which is what cornered means.
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

	for _, c := range ranked.Ranked {
		if !c.Named || !c.Creature() || c.Where() == "" {
			continue
		}

		d, err := in.Space.Distance(&DistanceInput{From: s.Self.Where, To: c.Where()})
		if err != nil {
			return nil, err
		}

		if !d.Known {
			continue
		}

		// ASKED ABOUT THIS CREATURE, inside the loop. It used to be one
		// question per turn, and a mind that keeps different distances
		// from different figures was inexpressible — which is what a
		// frightened monster is (rpg-project#454). A mind with one answer
		// for everybody is unaffected: the same number comes back every
		// time it is asked.
		keep, err := in.Mind.Keep(&KeepInput{Situation: s, Contact: c})
		if err != nil {
			return nil, err
		}

		if d.Steps >= keep.Steps {
			continue
		}

		away, err := in.Space.Away(&AwayInput{From: s.Self.Where, AwayFrom: c.Where()})
		if err != nil {
			return nil, err
		}

		if away.Found {
			return &DecideOutput{Intent: Intent{Verb: Away, Target: c.Name}}, nil
		}
	}

	for _, c := range ranked.Ranked {
		if !c.Named || !c.Creature() || c.Where() == "" {
			continue
		}

		d, err := in.Space.Distance(&DistanceInput{From: s.Self.Where, To: c.Where()})
		if err != nil {
			return nil, err
		}

		if d.Known && d.Steps <= s.Self.Reach {
			return &DecideOutput{Intent: Intent{Verb: Attack, Target: c.Name}}, nil
		}
	}

	for _, c := range ranked.Ranked {
		if !c.Named || c.Where() == "" || c.Where() == s.Self.Where || s.Self.Fenced(c) {
			continue
		}

		return &DecideOutput{Intent: Intent{Verb: Toward, Target: c.Name}}, nil
	}

	for _, c := range ranked.Ranked {
		if !c.Named || !c.Creature() || c.Where() == "" || !s.Self.Fenced(c) {
			continue
		}

		return &DecideOutput{Intent: Intent{Verb: Away, Target: c.Name}}, nil
	}

	return &DecideOutput{Intent: Intent{Verb: Pass}}, nil
}
