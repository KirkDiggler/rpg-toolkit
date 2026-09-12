// Copyright (C) 2026 Kirk Diggler
// SPDX-License-Identifier: GPL-3.0-or-later

// Package belief holds how an observer ORGANISES their own testimony: claims
// that two tracks are one thing, or are not.
//
// # The three states, and why the third one is the interesting one
//
// Two tracks stand in one of three relations for a given observer:
//
//   - [Unrelated] — no claim. The default, and the resting state of the whole
//     system. "I have not thought about it." Nothing merges by accident, so
//     "never merge two things the person hasn't merged" is not a rule anybody
//     enforces; it is what happens when nobody does anything.
//   - [Same] — merged. "The noise behind the door is that goblin."
//   - [Distinct] — ruled out. "Those are not goblin tracks, so they are not
//     these goblins."
//
// Distinct is not the absence of Same, and that difference is where expertise
// lives. A dumb observer sits in Unrelated forever: every new track is another
// possible threat, because it can never rule anything out. A woodwise observer
// can say Distinct, and that is what makes it hard to confuse.
//
// A wrong Distinct is as available as a wrong Same, which is the point.
//
// # Claims are revisable; testimony is not
//
// A later claim replaces an earlier claim on the same pair, because judging is
// an act an observer performs and may perform again. Testimony has no such
// door: an entry in a log is never revised by anything. Getting a merge wrong
// therefore costs an observer nothing but the claim — their memories were never
// at risk from their own inference.
//
// # Two claims, not one
//
// "These tracks are one thing" and "that thing is the goblin chief" are
// different claims, and a [Contact] still has no id and no name of its own.
// Naming is [Beliefs.Identify], and it attaches to a TRACK rather than a
// contact, because tracks are the only stable handles an observer has — a
// contact is folded fresh from claims every time it is asked for.
//
// That has a consequence worth wanting: merging two differently-named tracks
// leaves an observer holding one thing under two names. It is a contradiction,
// it is theirs, and nothing resolves it for them.
//
// # Why naming exists at all
//
// It is what makes a belief portable. A track handle is minted inside one run
// and means nothing outside it, so a belief can only leave a run if it has been
// re-keyed onto something that means something elsewhere. You can carry out
// "there are goblins in the eastern tunnels"; you cannot carry out "something
// is through that door", because there is nothing to file it under.
package belief

import (
	"errors"
	"fmt"
	"slices"

	"github.com/KirkDiggler/rpg-toolkit/examples/perception/testimony"
)

// Relation is what an observer claims about a pair of tracks.
type Relation uint8

const (
	// Unrelated is the absence of a claim: the zero value, and the default for
	// every pair of tracks anybody has ever perceived.
	Unrelated Relation = iota
	// Same claims the two tracks are one thing.
	Same
	// Distinct claims the two tracks are different things.
	Distinct
)

// String names a relation.
func (r Relation) String() string {
	switch r {
	case Same:
		return "same"
	case Distinct:
		return "distinct"
	case Unrelated:
		return "unrelated"
	default:
		return "unknown"
	}
}

// Sentinel errors.
var (
	// ErrNoObserver reports an empty observer.
	ErrNoObserver = errors.New("empty observer")
	// ErrNoTrack reports an empty track handle.
	ErrNoTrack = errors.New("empty track")
	// ErrSameTrack reports a claim about a track and itself.
	ErrSameTrack = errors.New("a track is not a pair")
)

// Claim is one observer's recorded judgment about a pair, and when they made
// it. It is attributed and timestamped for the same reason testimony is: a
// claim is a thing somebody did, not a fact about the world.
type Claim struct {
	A, B testimony.TrackID
	Rel  Relation
	At   testimony.Stamp
}

// Contact is a bundle of tracks one observer holds as a single thing. A track
// nobody has claimed anything about is a contact of one — the null bundle.
type Contact struct {
	// Tracks is sorted, and never empty.
	Tracks []testimony.TrackID
}

type pair struct {
	a, b testimony.TrackID
}

// Beliefs is every observer's claims. Not safe for concurrent use.
type Beliefs struct {
	claims map[testimony.Observer]map[pair]Claim
	names  map[testimony.Observer]map[testimony.TrackID]Identification
}

// New builds an empty set of beliefs.
func New() *Beliefs {
	return &Beliefs{
		claims: make(map[testimony.Observer]map[pair]Claim),
		names:  make(map[testimony.Observer]map[testimony.TrackID]Identification),
	}
}

// Assert records an observer's judgment about a pair of tracks, replacing any
// earlier judgment on that pair.
//
// Asserting [Unrelated] retracts a claim: "I no longer think anything about
// these two", which is a different act from claiming [Distinct].
func (b *Beliefs) Assert(o testimony.Observer, a, c testimony.TrackID, rel Relation, at testimony.Stamp) error {
	if o == "" {
		return fmt.Errorf("assert: %w", ErrNoObserver)
	}

	if a == "" || c == "" {
		return fmt.Errorf("assert: %w", ErrNoTrack)
	}

	if a == c {
		return fmt.Errorf("assert %s: %w", a, ErrSameTrack)
	}

	key := keyOf(a, c)

	if b.claims[o] == nil {
		b.claims[o] = make(map[pair]Claim)
	}

	if rel == Unrelated {
		delete(b.claims[o], key)

		return nil
	}

	b.claims[o][key] = Claim{A: key.a, B: key.b, Rel: rel, At: at}

	return nil
}

// Relation reports what this observer claims about a pair, and when. An
// observer who has made no claim gets [Unrelated] — which says they have not
// decided, never that the tracks are different.
func (b *Beliefs) Relation(o testimony.Observer, a, c testimony.TrackID) (Relation, testimony.Stamp) {
	claim, ok := b.claims[o][keyOf(a, c)]
	if !ok {
		return Unrelated, testimony.Stamp{}
	}

	return claim.Rel, claim.At
}

// Claims is every judgment this observer has made, ordered by pair.
func (b *Beliefs) Claims(o testimony.Observer) []Claim {
	held := b.claims[o]
	if len(held) == 0 {
		return nil
	}

	out := make([]Claim, 0, len(held))
	for _, claim := range held {
		out = append(out, claim)
	}

	slices.SortFunc(out, func(x, y Claim) int {
		if x.A != y.A {
			return cmpID(x.A, y.A)
		}

		return cmpID(x.B, y.B)
	})

	return out
}

// Contacts folds this observer's claims over the tracks they hold. It is
// derived, never stored: only the claims are state, so the null bundle costs
// nothing and a retraction cannot leave a stale contact behind.
//
// [Distinct] claims are recorded but do NOT drive this fold. Ruling two tracks
// out is a held belief in its own right, not an instruction to regroup — an
// observer who has never claimed Same has nothing to take apart.
func (b *Beliefs) Contacts(o testimony.Observer, tracks []testimony.TrackID) []Contact {
	if len(tracks) == 0 {
		return nil
	}

	known := make(map[testimony.TrackID]struct{}, len(tracks))
	for _, id := range tracks {
		known[id] = struct{}{}
	}

	adjacent := make(map[testimony.TrackID][]testimony.TrackID)

	for _, claim := range b.claims[o] {
		if claim.Rel != Same {
			continue
		}

		// A claim about a track this observer no longer holds joins nothing.
		if _, ok := known[claim.A]; !ok {
			continue
		}

		if _, ok := known[claim.B]; !ok {
			continue
		}

		adjacent[claim.A] = append(adjacent[claim.A], claim.B)
		adjacent[claim.B] = append(adjacent[claim.B], claim.A)
	}

	ordered := slices.Clone(tracks)
	slices.SortFunc(ordered, cmpID)

	seen := make(map[testimony.TrackID]struct{}, len(ordered))

	out := make([]Contact, 0, len(ordered))

	for _, start := range ordered {
		if _, done := seen[start]; done {
			continue
		}

		bundle := []testimony.TrackID{start}
		seen[start] = struct{}{}
		queue := []testimony.TrackID{start}

		for len(queue) > 0 {
			at := queue[0]
			queue = queue[1:]

			for _, next := range adjacent[at] {
				if _, done := seen[next]; done {
					continue
				}

				seen[next] = struct{}{}
				bundle = append(bundle, next)
				queue = append(queue, next)
			}
		}

		slices.SortFunc(bundle, cmpID)
		out = append(out, Contact{Tracks: bundle})
	}

	return out
}

func keyOf(a, c testimony.TrackID) pair {
	if a > c {
		return pair{a: c, b: a}
	}

	return pair{a: a, b: c}
}

func cmpID(a, b testimony.TrackID) int {
	switch {
	case a < b:
		return -1
	case a > b:
		return 1
	default:
		return 0
	}
}

// Name is a handle that means something OUTSIDE the run that produced it: a
// place, a faction, a creature somebody could speak about later. It is the
// observer's own word for a thing and carries no guarantee of being right.
type Name string

// Identification is one observer's claim that a track is of a named thing.
type Identification struct {
	Track testimony.TrackID
	Name  Name
	At    testimony.Stamp
}

// Identify records what this observer calls a track, replacing whatever they
// called it before. An empty name retracts the claim: they no longer have a word
// for it, which is different from having decided it is nothing.
func (b *Beliefs) Identify(o testimony.Observer, track testimony.TrackID, as Name, at testimony.Stamp) error {
	if o == "" {
		return fmt.Errorf("identify: %w", ErrNoObserver)
	}

	if track == "" {
		return fmt.Errorf("identify: %w", ErrNoTrack)
	}

	if b.names[o] == nil {
		b.names[o] = make(map[testimony.TrackID]Identification)
	}

	if as == "" {
		delete(b.names[o], track)

		return nil
	}

	b.names[o][track] = Identification{Track: track, Name: as, At: at}

	return nil
}

// NameOf is what this observer calls a track, and when they decided. The final
// return is false when they have no word for it — which says nothing about
// whether the thing has one.
func (b *Beliefs) NameOf(o testimony.Observer, track testimony.TrackID) (Name, testimony.Stamp, bool) {
	named, ok := b.names[o][track]
	if !ok {
		return "", testimony.Stamp{}, false
	}

	return named.Name, named.At, true
}

// Named is every identification this observer has made, ordered by track.
func (b *Beliefs) Named(o testimony.Observer) []Identification {
	held := b.names[o]
	if len(held) == 0 {
		return nil
	}

	out := make([]Identification, 0, len(held))
	for _, named := range held {
		out = append(out, named)
	}

	slices.SortFunc(out, func(x, y Identification) int {
		return cmpID(x.Track, y.Track)
	})

	return out
}
