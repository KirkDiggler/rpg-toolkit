// Copyright (C) 2026 Kirk Diggler
// SPDX-License-Identifier: GPL-3.0-or-later

package behavior

import (
	"errors"
	"fmt"

	"github.com/KirkDiggler/rpg-toolkit/examples/perception"
	"github.com/KirkDiggler/rpg-toolkit/examples/perception/content"
	"github.com/KirkDiggler/rpg-toolkit/examples/perception/projection"
	"github.com/KirkDiggler/rpg-toolkit/examples/perception/reconcile"
	"github.com/KirkDiggler/rpg-toolkit/examples/perception/testimony"
)

// ErrNoSelf reports a turn for an actor the truth surface has nowhere. It
// fails loudly: an actor with no position would find nothing in reach and pass
// forever, which would look like a very cautious monster rather than a wiring
// fault.
var ErrNoSelf = errors.New("behavior: the actor is not on the truth surface")

// Game is the composition: a perception game, plus who has which mind and
// where each of them stands.
//
// It is the one place that reads the truth surface, and it reads exactly one
// thing from it about an actor — that actor's OWN position, which is its own
// sheet and not a fact about anyone else.
type Game struct {
	p      *perception.Game
	minds  map[testimony.Observer]Mind
	selves map[testimony.Observer]Self
}

// New builds an empty game.
func New() *Game {
	return &Game{
		p:      perception.NewGame(),
		minds:  make(map[testimony.Observer]Mind),
		selves: make(map[testimony.Observer]Self),
	}
}

// Mind gives an observer a mind. Its Judge becomes that observer's reconciler
// in the perception game, so a mind's first judgment lands where every other
// claim does.
func (g *Game) Mind(o testimony.Observer, m Mind) {
	g.minds[o] = m
	g.p.Mind(o, m)
}

// Tick runs one perception pass and notes where each minded actor stands.
func (g *Game) Tick(in projection.Input) error {
	if _, err := g.p.Tick(in); err != nil {
		return err
	}

	for o := range g.minds {
		for _, p := range in.Presences {
			if p.Source == string(o) {
				g.selves[o] = Self{Where: p.Where}
			}
		}
	}

	return nil
}

// Situation assembles everything one actor has to go on, naming as it goes.
//
// A contact the actor has no word for is offered to its mind. A name the mind
// gives is recorded as that actor's own identification, on one track of the
// contact, so it persists the way every claim does and the next situation
// finds it already there.
func (g *Game) Situation(o testimony.Observer, at testimony.Stamp) (Situation, error) {
	mind, minded := g.minds[o]
	if !minded {
		return Situation{}, fmt.Errorf("behavior: %s has no mind", o)
	}

	self, placed := g.selves[o]
	if !placed {
		return Situation{}, fmt.Errorf("%w: %s", ErrNoSelf, o)
	}

	views := make(map[testimony.TrackID]reconcile.TrackView)
	for _, v := range reconcile.ViewsOf(g.p.Held(o)) {
		views[v.ID] = v
	}

	bundles := g.p.Contacts(o)
	contacts := make([]Contact, 0, len(bundles))

	for _, b := range bundles {
		c := Contact{Tracks: make([]reconcile.TrackView, 0, len(b.Tracks))}
		for _, id := range b.Tracks {
			c.Tracks = append(c.Tracks, views[id])
		}

		for _, id := range b.Tracks {
			if name, _, named := g.p.NameOf(o, id); named {
				c.Name, c.Named, c.Bearer = name, true, id

				break
			}
		}

		if !c.Named {
			if name, ok := mind.Name(c); ok {
				id := bearer(c)
				if err := g.p.Identify(o, id, name, at); err != nil {
					return Situation{}, err
				}

				c.Name, c.Named, c.Bearer = name, true, id
			}
		}

		contacts = append(contacts, c)
	}

	return Situation{Actor: o, Contacts: contacts, Self: self, At: at}, nil
}

// bearer is the track a contact's name is recorded on: a current creature if
// there is one, else the first. A name attaches to a track because tracks are
// the only stable handles; a contact is folded fresh every time.
func bearer(c Contact) testimony.TrackID {
	for _, v := range c.Tracks {
		if !v.Current {
			continue
		}

		if p, err := content.Decode(v.Payload); err == nil && p.Kind == content.Creature {
			return v.ID
		}
	}

	return c.Tracks[0].ID
}

// Report lands discrete testimony on one actor — the stage uses it to tell a
// witness what they saw somebody do. It is perception's own door, passed
// through; behaviour adds nothing to what may be said.
func (g *Game) Report(in testimony.Recollection) (testimony.Delta, error) {
	return g.p.Report(in)
}

// Held is one actor's whole testimony. The stage reads it to say a deed in
// the witness's own terms: which of THEIR tracks the actor and target are.
func (g *Game) Held(o testimony.Observer) []testimony.Track {
	return g.p.Held(o)
}

// Turn is one actor's decision: build the situation, climb the ladder.
func (g *Game) Turn(o testimony.Observer, at testimony.Stamp) (Intent, Situation, error) {
	s, err := g.Situation(o, at)
	if err != nil {
		return Intent{}, Situation{}, err
	}

	return Decide(s, g.minds[o]), s, nil
}
