// Copyright (C) 2026 Kirk Diggler
// SPDX-License-Identifier: GPL-3.0-or-later

package behavior_test

import (
	"cmp"
	"slices"

	"github.com/KirkDiggler/rpg-toolkit/core"
	"github.com/KirkDiggler/rpg-toolkit/mind/behavior"
	"github.com/KirkDiggler/rpg-toolkit/mind/behavior/deed"
)

// This file is the worked examples: the types a behaviour author copies.
//
// Each mind is a few judgments and no state. The difference between a zombie
// and a captain is entirely in what each makes of the same testimony — the
// bytes are identical, and the minds are not. They live in the tests because
// they speak a content vocabulary behaviour does not own.

const (
	// chanting is what hearing says about somebody casting.
	chanting = "chanting"
	// robed is what sight says about somebody dressed like a caster.
	robed = "robed"
	// heal is the deed a witness saw when somebody mended somebody.
	heal = "heal"
	// patience is how many ticks old a ghost may be before the captain
	// stops walking after it. It is a feel number, and the captain's alone.
	patience uint64 = 2
)

// age is how many ticks since a contact was last confirmed.
func age(c behavior.Contact, at uint64) uint64 {
	last := c.LastConfirmed()
	if last > at {
		return 0
	}

	return at - last
}

// did reports whether any holding in the contact is a deed with the verb.
// Deeds are never current, so this reads every holding.
func did(c behavior.Contact, verb string) bool {
	for _, h := range c.Holdings {
		if d, err := deed.Decode(h.Payload); err == nil && d.Verb == verb {
			return true
		}
	}

	return false
}

// byFirstSeen orders contacts by when the actor first became aware of them,
// ties broken by the first subject so the order is stable.
func byFirstSeen(contacts []behavior.Contact) []behavior.Contact {
	out := slices.Clone(contacts)

	slices.SortStableFunc(out, func(a, b behavior.Contact) int {
		if c := cmp.Compare(a.FirstObserved(), b.FirstObserved()); c != 0 {
			return c
		}

		return cmp.Compare(a.Holdings[0].Subject, b.Holdings[0].Subject)
	})

	return out
}

// reflexive is the name a mind with no words gives everything: "thing" and
// a subject. It has a word for everything and it is the same word.
func reflexive(c behavior.Contact) *behavior.NameOutput {
	return &behavior.NameOutput{Name: behavior.Name("thing " + string(c.Holdings[0].Subject)), Named: true}
}

// zombieMind never merges, names everything reflexively, and goes for
// whatever it noticed first.
//
// It is not stupid because it merges wrongly. It is stupid because it can
// never merge at all: the chant and the figure chanting are two things to
// it, forever, and it will swing at the figure and wonder where the noise is
// coming from.
type zombieMind struct{}

// Judge claims nothing, ever.
func (zombieMind) Judge(*behavior.JudgeInput) (*behavior.JudgeOutput, error) {
	return &behavior.JudgeOutput{}, nil
}

// Name calls a contact "thing" and its first subject.
func (zombieMind) Name(in *behavior.NameInput) (*behavior.NameOutput, error) {
	return reflexive(in.Contact), nil
}

// Rank prefers whatever it noticed first, and never lets go: a ghost from an
// hour ago ranks exactly as it did when it was fresh. A post means nothing
// to it, so it never goes anywhere on purpose.
func (zombieMind) Rank(in *behavior.RankInput) (*behavior.RankOutput, error) {
	out := make([]behavior.Contact, 0, len(in.Situation.Contacts))

	for _, c := range in.Situation.Contacts {
		if kindOf(c) != post {
			out = append(out, c)
		}
	}

	return &behavior.RankOutput{Ranked: byFirstSeen(out)}, nil
}

// Keep is nothing. A zombie lets everything get as close as it likes.
func (zombieMind) Keep(*behavior.KeepInput) (*behavior.KeepOutput, error) {
	return &behavior.KeepOutput{Regions: 0}, nil
}

// captainMind merges a chant into the one robed figure standing where the
// chant is, attaches a deed to who it saw do it, names what it sees by how
// it looks, and goes for the healer first, then the caster.
//
// Every one of those is a judgment it can be wrong about. The chant might be
// coming from the armoured one. Nothing the captain holds could tell it so,
// and that is the point: a mind that could not be wrong would be reading
// the world.
type captainMind struct{}

// Judge makes two kinds of claim.
//
// A chant and a robed figure in one place are the same thing — but only
// when exactly one robed figure stands there. Two would be a guess, and a
// guess on a tie is worse than no claim.
//
// A deed and the figure the witness saw do it are the same thing. That one
// is not a guess: the deed already names the actor in the captain's own
// terms. It is still a claim, because attaching what you saw done to who
// you saw do it is the mind's act — a zombie holds the same deed and never
// makes it.
func (captainMind) Judge(in *behavior.JudgeInput) (*behavior.JudgeOutput, error) {
	held := func(id core.EntityID) bool {
		return slices.ContainsFunc(in.Holdings, func(h behavior.Holding) bool { return h.Subject == id })
	}

	var same []behavior.Pair

	for _, h := range in.Holdings {
		if h.Channel != deed.Channel {
			continue
		}

		if d, err := deed.Decode(h.Payload); err == nil && d.Actor != "" && held(d.Actor) {
			same = append(same, behavior.Pair{A: h.Subject, B: d.Actor})
		}
	}

	for _, chant := range in.Holdings {
		p, ok := decode(chant.Payload)
		if !ok || len(chant.CurrentVia) == 0 || p.Kind != noise || p.Note != chanting || p.Where == "" {
			continue
		}

		var candidates []core.EntityID

		for _, figure := range in.Holdings {
			q, ok := decode(figure.Payload)
			if !ok || len(figure.CurrentVia) == 0 || q.Kind != creature || q.Note != robed || q.Where != p.Where {
				continue
			}

			candidates = append(candidates, figure.Subject)
		}

		if len(candidates) == 1 {
			same = append(same, behavior.Pair{A: chant.Subject, B: candidates[0]})
		}
	}

	return &behavior.JudgeOutput{Same: same}, nil
}

// Name calls a creature by how it looks, a noise by what it sounds like,
// and its post "my post". A contact it cannot read gets no word: it is not
// something the captain will act on, so it stays unnamed.
func (captainMind) Name(in *behavior.NameInput) (*behavior.NameOutput, error) {
	var heard percept

	for _, h := range in.Contact.Holdings {
		p, ok := decode(h.Payload)
		if !ok {
			continue
		}

		switch p.Kind {
		case creature:
			return &behavior.NameOutput{Name: behavior.Name("the " + p.Note + " " + p.Of), Named: true}, nil
		case noise:
			heard = p
		case post:
			return &behavior.NameOutput{Name: "my post", Named: true}, nil
		}
	}

	if heard.Kind == noise {
		return &behavior.NameOutput{Name: behavior.Name("the " + heard.Note), Named: true}, nil
	}

	return &behavior.NameOutput{}, nil
}

// Rank puts whoever it has seen heal first, then whoever is chanting, then
// whoever it noticed first, and its own post last of all. A healer it has
// not SEEN heal is just another figure: the ranking reads deeds the captain
// witnessed, never the sheet.
//
// A ghost older than patience is dropped. The captain will walk to where it
// last saw somebody, but not to where it saw somebody an age ago — and with
// nothing left worth pursuing, the post is what remains, so it goes back.
func (captainMind) Rank(in *behavior.RankInput) (*behavior.RankOutput, error) {
	s := in.Situation
	kept := make([]behavior.Contact, 0, len(s.Contacts))

	for _, c := range s.Contacts {
		if !c.Current() && kindOf(c) != post && age(c, s.At) > patience {
			continue
		}

		kept = append(kept, c)
	}

	ordered := byFirstSeen(kept)

	slices.SortStableFunc(ordered, func(a, b behavior.Contact) int {
		return preference(b) - preference(a)
	})

	return &behavior.RankOutput{Ranked: ordered}, nil
}

// Keep is nothing. A captain stands and fights.
func (captainMind) Keep(*behavior.KeepInput) (*behavior.KeepOutput, error) {
	return &behavior.KeepOutput{Regions: 0}, nil
}

func preference(c behavior.Contact) int {
	switch {
	case kindOf(c) == post:
		return -1
	case did(c, heal):
		return 2
	case saysNow(c, noise, chanting):
		return 1
	default:
		return 0
	}
}

// archerMind never merges, names reflexively, goes for whatever it noticed
// first — and keeps a region between itself and anything alive.
//
// It is a zombie with a bow and one preference. That is deliberate: kiting
// is not cleverness, it is a single number the ladder reads. The archer's
// whole difference from the zombie is Keep returning 1, and what it is
// armed with lives on its sheet, not in its mind.
type archerMind struct{ zombieMind }

// Keep is one region. Anything that closes to the archer's own region is
// something to step away from before it is something to shoot.
func (archerMind) Keep(*behavior.KeepInput) (*behavior.KeepOutput, error) {
	return &behavior.KeepOutput{Regions: 1}, nil
}

// keeps is any mind with a different Keep: the one knob the ladder reads
// that the worked minds set to 0 or 1.
type keeps struct {
	behavior.Mind
	regions int
}

// Keep is whatever the test says.
func (k keeps) Keep(*behavior.KeepInput) (*behavior.KeepOutput, error) {
	return &behavior.KeepOutput{Regions: k.regions}, nil
}
