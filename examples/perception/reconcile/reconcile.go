// Copyright (C) 2026 Kirk Diggler
// SPDX-License-Identifier: GPL-3.0-or-later

// Package reconcile judges which of an observer's tracks are the same thing.
// It is where intelligence enters perception.
//
// # It cannot see the world
//
// A [Reconciler] is handed [TrackView] values built from held testimony and
// nothing else. It has no access to the truth surface, which is the only reason
// it can be WRONG — a reconciler that could read reality would merge correctly
// every time, and the whole knob would be dead.
//
// It does read payloads. Opacity is a property of the store, not of everyone:
// this package lives where the rulebook lives, so it may decode what a percept
// meant. A payload it cannot decode is a track it cannot judge, which is the
// honest outcome rather than an error.
//
// # The knob
//
// Three reconcilers, three points on one axis:
//
//   - [Oblivious] never claims anything. The zombie. It is not confused because
//     it merges wrongly; it is confused because it can never rule anything OUT,
//     so every new track stays a separate possible threat forever.
//   - [Credulous] merges whatever shares a place. The panicked commoner. It
//     puts the footprints and the goblins together and is confidently wrong.
//   - [Woodwise] merges across channels by co-location, recognises a memory in
//     what it is now looking at, and rules out by content: those are not goblin
//     tracks, so they are not these goblins.
//
// [Woodwise] can rule out and cannot rule in, which is how tracking actually
// works. A trace that matches gets no claim at all — the sign is consistent,
// and consistent is not the same as identical.
package reconcile

import (
	"slices"

	"github.com/KirkDiggler/rpg-toolkit/examples/perception/belief"
	"github.com/KirkDiggler/rpg-toolkit/examples/perception/content"
	"github.com/KirkDiggler/rpg-toolkit/examples/perception/testimony"
)

// TrackView is what a reconciler gets to look at: one held track's latest
// testimony, with no way back to the world that produced it.
type TrackView struct {
	ID        testimony.TrackID
	Channel   testimony.Channel
	Payload   []byte
	Locus     testimony.Locus
	Observed  testimony.Stamp
	Confirmed testimony.Stamp
	// Current says whether a channel is delivering this right now. False is a
	// memory — a ghost, or a belief carried in from somewhere else — and the
	// distinction changes what a reconciler may conclude. See [Woodwise].
	Current bool
}

// Judgment is one claim a reconciler wants recorded.
type Judgment struct {
	A, B testimony.TrackID
	Rel  belief.Relation
}

// Reconciler decides what an observer makes of their own tracks.
type Reconciler interface {
	Judge(views []TrackView, at testimony.Stamp) []Judgment
}

// ViewsOf builds reconciler input from the head of each track. It is the only
// bridge between the store and a reconciler, and it deliberately carries
// nothing the store did not already hold.
//
// It takes heads rather than whole tracks because a reconciler compares what
// tracks say NOW. The trail is not withheld from it as a matter of policy — it
// simply is not the question being asked, and asking the cheaper question is
// free.
func ViewsOf(heads []testimony.Head) []TrackView {
	out := make([]TrackView, 0, len(heads))

	for _, h := range heads {
		out = append(out, TrackView{
			ID:        h.ID,
			Channel:   h.Channel,
			Payload:   h.Entry.Payload,
			Locus:     h.Entry.Locus,
			Observed:  h.Entry.Observed,
			Confirmed: h.Entry.Confirmed,
			Current:   h.Current,
		})
	}

	return out
}

// Apply records a reconciler's judgments as that observer's own claims.
func Apply(b *belief.Beliefs, o testimony.Observer, js []Judgment, at testimony.Stamp) error {
	for _, j := range js {
		if err := b.Assert(o, j.A, j.B, j.Rel, at); err != nil {
			return err
		}
	}

	return nil
}

// Oblivious judges nothing, ever.
type Oblivious struct{}

// Judge returns no claims.
func (Oblivious) Judge([]TrackView, testimony.Stamp) []Judgment { return nil }

// Credulous merges anything that shares a place, on any channel, of any kind.
type Credulous struct{}

// Judge claims Same for every co-located pair.
func (Credulous) Judge(views []TrackView, _ testimony.Stamp) []Judgment {
	var out []Judgment

	for i := range views {
		for j := i + 1; j < len(views); j++ {
			if views[i].Locus.Where == "" || views[i].Locus.Where != views[j].Locus.Where {
				continue
			}

			out = append(out, Judgment{A: views[i].ID, B: views[j].ID, Rel: belief.Same})
		}
	}

	return out
}

// Woodwise merges across channels by co-location and rules out by sign.
type Woodwise struct{}

// Judge claims Same for a present thing delivered by two different channels in
// one place, and Distinct for a trace whose sign contradicts a creature.
//
// A matching trace gets no claim: the ranger can rule out and cannot rule in.
func (Woodwise) Judge(views []TrackView, _ testimony.Stamp) []Judgment {
	read := make([]content.Percept, len(views))
	ok := make([]bool, len(views))

	for i, v := range views {
		p, err := content.Decode(v.Payload)
		if err != nil {
			continue
		}

		read[i], ok[i] = p, true
	}

	var out []Judgment

	for i := range views {
		for j := i + 1; j < len(views); j++ {
			if !ok[i] || !ok[j] {
				continue
			}

			rel, judged := woodwiseJudge(views[i], read[i], views[j], read[j])
			if !judged {
				continue
			}

			out = append(out, Judgment{A: views[i].ID, B: views[j].ID, Rel: rel})
		}
	}

	return out
}

func woodwiseJudge(a TrackView, pa content.Percept, b TrackView, pb content.Percept) (belief.Relation, bool) {
	// The sign rule holds wherever the trace was found: what left a mark is
	// not the thing standing in front of you if the mark is the wrong shape.
	if rel, judged := signRule(pa, pb); judged {
		return rel, true
	}

	if a.Locus.Where == "" || a.Locus.Where != b.Locus.Where {
		return belief.Unrelated, false
	}

	if !present(pa.Kind) || !present(pb.Kind) {
		return belief.Unrelated, false
	}

	if a.Channel != b.Channel {
		return belief.Same, true
	}

	// Same channel. Two things you can perceive at once, in one place, are two
	// things — you would be looking straight at both of them.
	//
	// A memory is not something you are perceiving. So a recollection and a live
	// percept on one channel may well be the same thing, and recognising that is
	// the whole of walking back into a room you have been in before. It is also
	// exactly as available to be wrong as every other claim here.
	if a.Current != b.Current {
		return belief.Same, true
	}

	return belief.Unrelated, false
}

func signRule(a, b content.Percept) (belief.Relation, bool) {
	trace, creature, paired := traceAndCreature(a, b)
	if !paired {
		return belief.Unrelated, false
	}

	if trace.Of == "" || creature.Of == "" {
		return belief.Unrelated, false
	}

	if trace.Of != creature.Of {
		return belief.Distinct, true
	}

	// Consistent sign is not an identity. No claim.
	return belief.Unrelated, false
}

func traceAndCreature(a, b content.Percept) (trace, creature content.Percept, ok bool) {
	switch {
	case a.Kind == content.Trace && b.Kind == content.Creature:
		return a, b, true
	case b.Kind == content.Trace && a.Kind == content.Creature:
		return b, a, true
	default:
		return content.Percept{}, content.Percept{}, false
	}
}

func present(k content.Kind) bool {
	return slices.Contains([]content.Kind{content.Creature, content.Noise}, k)
}
