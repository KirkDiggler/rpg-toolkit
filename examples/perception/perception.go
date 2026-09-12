// Copyright (C) 2026 Kirk Diggler
// SPDX-License-Identifier: GPL-3.0-or-later

// Package perception is the composition: the one place that reads a truth
// surface, hands it to the projection, lands the percepts as testimony, and
// lets each observer's own mind make what it will of them.
//
// It exists to prove the four layers hold as separate pieces:
//
//	truth surface  ->  projection  ->  testimony  <-  reconciler
//	(values in)        (the lie)       (the memory)   (the intelligence)
//
// The arrows are the whole design. The projection sees the world and can never
// be asked a question. The store sees neither the world nor the meaning of what
// it holds. The reconciler sees only the store, which is the only reason it can
// be wrong. Nothing reads truth to answer a question about an observer.
//
// This is a spike. It owes nothing to the shipped implementation and is not a
// migration of it; it is here to find out whether the shape survives contact
// with the scenarios in scenarios_test.go.
package perception

import (
	"slices"

	"github.com/KirkDiggler/rpg-toolkit/examples/perception/belief"
	"github.com/KirkDiggler/rpg-toolkit/examples/perception/projection"
	"github.com/KirkDiggler/rpg-toolkit/examples/perception/reconcile"
	"github.com/KirkDiggler/rpg-toolkit/examples/perception/testimony"
)

// Landing is what one channel's pass did to one observer's knowledge.
type Landing struct {
	Observer testimony.Observer
	Channel  testimony.Channel
	Delta    testimony.Delta
}

// Game is one region's perception: everyone's testimony, everyone's claims, and
// who has which mind.
type Game struct {
	held  *testimony.Store
	belie *belief.Beliefs
	minds map[testimony.Observer]reconcile.Reconciler
}

// NewGame builds an empty game.
func NewGame() *Game {
	return &Game{
		held:  testimony.New(),
		belie: belief.New(),
		minds: make(map[testimony.Observer]reconcile.Reconciler),
	}
}

// Mind sets how an observer reconciles their own tracks. An observer with no
// mind set makes no claims, which is the same as [reconcile.Oblivious] — the
// default is to hold everything separately and decide nothing.
func (g *Game) Mind(o testimony.Observer, r reconcile.Reconciler) {
	g.minds[o] = r
}

// Tick runs one pass: project, land, then reconcile.
//
// Reconciliation runs once per observer AFTER every channel has landed, because
// a cross-channel judgment cannot be made while half the evidence is still in
// flight.
func (g *Game) Tick(in projection.Input) ([]Landing, error) {
	percepts := projection.Project(in)

	out := make([]Landing, 0, len(percepts))

	var touched []testimony.Observer

	for _, p := range percepts {
		delta, err := g.held.Surveil(p)
		if err != nil {
			return nil, err
		}

		out = append(out, Landing{Observer: p.Observer, Channel: p.Channel, Delta: delta})

		if !slices.Contains(touched, p.Observer) {
			touched = append(touched, p.Observer)
		}
	}

	for _, o := range touched {
		mind, set := g.minds[o]
		if !set {
			continue
		}

		judged := g.unclaimed(o, mind.Judge(reconcile.ViewsOf(g.held.Held(o)), in.At))
		if err := reconcile.Apply(g.belie, o, judged, in.At); err != nil {
			return nil, err
		}
	}

	return out, nil
}

// unclaimed drops any judgment about a pair this observer has already decided.
//
// A mind proposes; it does not overrule its owner. Without this, a reconciler
// re-judges every pair on every pass, so a player who took two tracks apart by
// hand would watch their own instincts put them back together on the next tick,
// and a ghost would be re-merged forever on evidence nobody is still receiving.
//
// It also gives the two ways of coming apart different meanings, which is worth
// having: retracting to [belief.Unrelated] says "I no longer know", and leaves
// the mind free to reach the same conclusion again. Claiming [belief.Distinct]
// says "I have ruled it out", and it sticks until the observer says otherwise.
func (g *Game) unclaimed(o testimony.Observer, judged []reconcile.Judgment) []reconcile.Judgment {
	out := make([]reconcile.Judgment, 0, len(judged))

	for _, j := range judged {
		if rel, _ := g.belie.Relation(o, j.A, j.B); rel != belief.Unrelated {
			continue
		}

		out = append(out, j)
	}

	return out
}

// Held is one observer's whole testimony.
func (g *Game) Held(o testimony.Observer) []testimony.Track {
	return g.held.Held(o)
}

// Track is one held track, and whether it is held at all.
func (g *Game) Track(o testimony.Observer, id testimony.TrackID) (testimony.Track, bool) {
	return g.held.Track(o, id)
}

// Contacts is how this observer currently bundles their own tracks.
func (g *Game) Contacts(o testimony.Observer) []belief.Contact {
	tracks := g.held.Held(o)

	ids := make([]testimony.TrackID, 0, len(tracks))
	for _, t := range tracks {
		ids = append(ids, t.ID)
	}

	return g.belie.Contacts(o, ids)
}

// Relation is what this observer claims about a pair of their tracks.
func (g *Game) Relation(o testimony.Observer, a, b testimony.TrackID) (belief.Relation, testimony.Stamp) {
	return g.belie.Relation(o, a, b)
}

// Identify records what this observer calls a track. The composition satisfies
// carry.Namer through this pair, because deciding what you call a thing is a
// game decision and not a storage one.
func (g *Game) Identify(o testimony.Observer, track testimony.TrackID, as belief.Name, at testimony.Stamp) error {
	return g.belie.Identify(o, track, as, at)
}

// NameOf is what this observer calls a track, if they have a word for it.
func (g *Game) NameOf(o testimony.Observer, track testimony.TrackID) (belief.Name, testimony.Stamp, bool) {
	return g.belie.NameOf(o, track)
}

// Claim records a judgment the observer made themselves, rather than one their
// mind reached. A player merging two contacts by hand comes through here.
func (g *Game) Claim(o testimony.Observer, a, b testimony.TrackID, rel belief.Relation, at testimony.Stamp) error {
	return g.belie.Assert(o, a, b, rel, at)
}
