// Copyright (C) 2026 Kirk Diggler
// SPDX-License-Identifier: GPL-3.0-or-later

// Package minds is the worked examples: the types a behaviour author copies.
//
// Each mind is three judgments and no state. The difference between a zombie
// and a captain is entirely in what each makes of the same testimony — the
// bytes are identical, and the minds are not.
//
// # The words a mind knows
//
// A mind reads percept content, so it has a vocabulary: the notes a channel
// might say that it knows how to act on. Those are the constants below. A note
// a mind has no word for is simply not something it can reason about, which is
// honest rather than an error.
package minds

import (
	"slices"

	"github.com/KirkDiggler/rpg-toolkit/examples/behavior"
	"github.com/KirkDiggler/rpg-toolkit/examples/behavior/deed"
	"github.com/KirkDiggler/rpg-toolkit/examples/perception/belief"
	"github.com/KirkDiggler/rpg-toolkit/examples/perception/content"
	"github.com/KirkDiggler/rpg-toolkit/examples/perception/reconcile"
	"github.com/KirkDiggler/rpg-toolkit/examples/perception/testimony"
)

const (
	// Chanting is what hearing says about somebody casting.
	Chanting = "chanting"
	// Robed is what sight says about somebody dressed like a caster.
	Robed = "robed"
	// Heal is the deed a witness saw when somebody mended somebody.
	Heal = "heal"

	// PostKind is what sight says about a place a guard was told to stand: a
	// banner, a doorway, a mark on the floor. A post is perceived like
	// anything else, so returning to it is Toward a name and needs no verb.
	PostKind content.Kind = "post"

	// Patience is how many ticks old a ghost may be before the captain stops
	// walking after it. It is a feel number, and the captain's alone.
	Patience uint64 = 2
)

// age is how many ticks since a contact was last confirmed, within one run.
func age(c behavior.Contact, at testimony.Stamp) uint64 {
	last := c.LastConfirmed()
	if last.Seq != at.Seq || last.Tick > at.Tick {
		return 0
	}

	return at.Tick - last.Tick
}

// did reports whether any track in the contact is a deed with the given verb.
// Deeds are never current, so this reads every track.
func did(c behavior.Contact, verb string) bool {
	for _, v := range c.Tracks {
		if d, err := deed.Decode(v.Payload); err == nil && d.Verb == verb {
			return true
		}
	}

	return false
}

// byFirstSeen orders contacts by when the actor first became aware of them,
// ties broken by the first track's handle so the order is stable.
func byFirstSeen(contacts []behavior.Contact) []behavior.Contact {
	out := slices.Clone(contacts)

	slices.SortStableFunc(out, func(a, b behavior.Contact) int {
		if c := a.FirstObserved().Compare(b.FirstObserved()); c != 0 {
			return c
		}

		return compareID(a.Tracks[0].ID, b.Tracks[0].ID)
	})

	return out
}

func compareID(a, b testimony.TrackID) int {
	switch {
	case a < b:
		return -1
	case a > b:
		return 1
	default:
		return 0
	}
}

// says reports whether any current track in the contact carries a percept of
// the given kind with the given note.
func says(c behavior.Contact, kind content.Kind, note string) bool {
	for _, v := range c.Tracks {
		if !v.Current {
			continue
		}

		if p, err := content.Decode(v.Payload); err == nil && p.Kind == kind && p.Note == note {
			return true
		}
	}

	return false
}

// Zombie never merges, names everything reflexively, and goes for whatever it
// noticed first.
//
// It is not stupid because it merges wrongly. It is stupid because it can never
// merge at all: the chant and the figure chanting are two things to it, forever,
// and it will swing at the figure and wonder where the noise is coming from.
type Zombie struct{}

// Judge claims nothing, ever.
func (Zombie) Judge([]reconcile.TrackView, testimony.Stamp) []reconcile.Judgment { return nil }

// Name calls a contact "thing" and its first handle. A zombie has a word for
// everything and it is the same word.
func (Zombie) Name(c behavior.Contact) (belief.Name, bool) {
	return belief.Name("thing " + string(c.Tracks[0].ID)), true
}

// Rank prefers whatever it noticed first, and never lets go: a ghost from an
// hour ago ranks exactly as it did when it was fresh. A post means nothing to
// it, so it never goes anywhere on purpose.
func (Zombie) Rank(s behavior.Situation) []behavior.Contact {
	out := make([]behavior.Contact, 0, len(s.Contacts))

	for _, c := range s.Contacts {
		if c.Kind() != PostKind {
			out = append(out, c)
		}
	}

	return byFirstSeen(out)
}

// Keep is nothing. A zombie lets everything get as close as it likes.
func (Zombie) Keep(behavior.Situation) int { return 0 }

// Captain merges a chant into the one robed figure standing where the chant
// is, names what it sees by how it looks, and goes for the caster first.
//
// Every one of those is a judgment it can be wrong about. The chant might be
// coming from the armoured one. Nothing the captain holds could tell it so, and
// that is the point: a mind that could not be wrong would be reading the world.
type Captain struct{}

// Judge makes two kinds of claim.
//
// A chant and a robed figure in one place are the same thing — but only when
// exactly one robed figure stands there. Two would be a guess, and a guess on
// a tie is worse than no claim.
//
// A deed and the figure the witness saw do it are the same thing. That one is
// not a guess: the deed already names the actor in the captain's own terms.
// It is still a claim, because attaching what you saw done to who you saw do
// it is the mind's act — a zombie holds the same deed and never makes it.
func (Captain) Judge(views []reconcile.TrackView, _ testimony.Stamp) []reconcile.Judgment {
	read := make([]content.Percept, len(views))
	ok := make([]bool, len(views))
	out := make([]reconcile.Judgment, 0, len(views))

	for i, v := range views {
		if d, err := deed.Decode(v.Payload); err == nil {
			if d.Actor != "" && slices.ContainsFunc(views, func(w reconcile.TrackView) bool { return w.ID == d.Actor }) {
				out = append(out, reconcile.Judgment{A: v.ID, B: d.Actor, Rel: belief.Same})
			}

			continue
		}

		p, err := content.Decode(v.Payload)
		if err != nil || !v.Current {
			continue
		}

		read[i], ok[i] = p, true
	}

	for i, chant := range views {
		if !ok[i] || read[i].Kind != content.Noise || read[i].Note != Chanting {
			continue
		}

		var robed []int

		for j := range views {
			if !ok[j] || j == i || read[j].Kind != content.Creature || read[j].Note != Robed {
				continue
			}

			if views[j].Locus.Where == "" || views[j].Locus.Where != chant.Locus.Where {
				continue
			}

			robed = append(robed, j)
		}

		if len(robed) != 1 {
			continue
		}

		out = append(out, reconcile.Judgment{A: chant.ID, B: views[robed[0]].ID, Rel: belief.Same})
	}

	return out
}

// Name calls a creature by how it looks, and a noise by what it sounds like.
func (Captain) Name(c behavior.Contact) (belief.Name, bool) {
	var noise content.Percept

	for _, v := range c.Tracks {
		p, err := content.Decode(v.Payload)
		if err != nil {
			continue
		}

		switch p.Kind {
		case content.Creature:
			return belief.Name("the " + p.Note + " " + p.Of), true
		case content.Noise:
			noise = p
		case PostKind:
			return "my post", true
		case content.Trace:
			// A captain has no word for a mark on the floor. It is not
			// something it will act on, so it stays unnamed.
		}
	}

	if noise.Kind == content.Noise {
		return belief.Name("the " + noise.Note), true
	}

	return "", false
}

// Rank puts whoever it has seen heal first, then whoever is chanting, then
// whoever it noticed first, and its own post last of all. A healer it has not
// SEEN heal is just another figure: the ranking reads deeds the captain
// witnessed, never the sheet.
//
// A ghost older than [Patience] is dropped. The captain will walk to where it
// last saw somebody, but not to where it saw somebody an age ago — and with
// nothing left worth pursuing, the post is what remains, so it goes back.
func (Captain) Rank(s behavior.Situation) []behavior.Contact {
	kept := make([]behavior.Contact, 0, len(s.Contacts))

	for _, c := range s.Contacts {
		if !c.Current() && c.Kind() != PostKind && age(c, s.At) > Patience {
			continue
		}

		kept = append(kept, c)
	}

	ordered := byFirstSeen(kept)

	slices.SortStableFunc(ordered, func(a, b behavior.Contact) int {
		return preference(b) - preference(a)
	})

	return ordered
}

// Keep is nothing. A captain stands and fights.
func (Captain) Keep(behavior.Situation) int { return 0 }

func preference(c behavior.Contact) int {
	switch {
	case c.Kind() == PostKind:
		return -1
	case did(c, Heal):
		return 2
	case says(c, content.Noise, Chanting):
		return 1
	default:
		return 0
	}
}

// Archer never merges, names reflexively, goes for whatever it noticed first —
// and keeps a region between itself and anything alive.
//
// It is a zombie with a bow and one preference. That is deliberate: kiting is
// not cleverness, it is a single number the ladder reads. The archer's whole
// difference from the zombie is Keep returning 1, and what it is armed with
// lives on its sheet, not in its mind.
type Archer struct{}

// Judge claims nothing.
func (Archer) Judge([]reconcile.TrackView, testimony.Stamp) []reconcile.Judgment { return nil }

// Name calls a contact "thing" and its first handle.
func (Archer) Name(c behavior.Contact) (belief.Name, bool) { return Zombie{}.Name(c) }

// Rank prefers whatever it noticed first.
func (Archer) Rank(s behavior.Situation) []behavior.Contact { return byFirstSeen(s.Contacts) }

// Keep is one region. Anything that closes to the archer's own region is
// something to step away from before it is something to shoot.
func (Archer) Keep(behavior.Situation) int { return 1 }
