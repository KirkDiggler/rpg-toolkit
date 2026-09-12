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
	sheets map[testimony.Observer]Sheet
	selves map[testimony.Observer]Self
	doors  map[string][]string
	fears  map[testimony.Observer][]testimony.TrackID
}

// New builds an empty game.
func New() *Game {
	return &Game{
		p:      perception.NewGame(),
		minds:  make(map[testimony.Observer]Mind),
		sheets: make(map[testimony.Observer]Sheet),
		selves: make(map[testimony.Observer]Self),
		doors:  make(map[string][]string),
		fears:  make(map[testimony.Observer][]testimony.TrackID),
	}
}

// Frighten puts the frightened condition on an actor: it may not willingly
// move toward the source. The condition is a fact about the actor's own sheet
// and arrives here as one, in ledger terms; it is translated into the actor's
// own sight handle of the source so that nothing downstream ever sees an
// entity id. Whether the actor can currently perceive that source is its own
// problem — a fence on something you cannot place forbids nothing, which is
// what being afraid of something you cannot see feels like.
func (g *Game) Frighten(o testimony.Observer, source string) {
	g.fears[o] = append(g.fears[o], projection.Handle(testimony.Sight, source))
}

// Connect declares two regions adjacent. Static topology is construction
// truth and every actor may know it; who stands where is not.
func (g *Game) Connect(a, b string) {
	g.doors[a] = append(g.doors[a], b)
	g.doors[b] = append(g.doors[b], a)
}

// Route is the first step from one region toward another along the dungeon's
// doors, and false when there is no way. Static topology is construction
// truth: a monster may know the way through its own dungeon. It may not know
// who is standing in it.
func (g *Game) Route(from, to string) (string, bool) {
	dist := g.distances(to)

	best, found := "", false

	for _, next := range g.doors[from] {
		if d, reachable := dist[next]; reachable && (!found || d < dist[best]) {
			best, found = next, true
		}
	}

	if !found || dist[best] >= dist[from] {
		return "", false
	}

	return best, true
}

// Farther is one step that puts more of the dungeon between the actor and a
// region, and false when every door leads closer or nowhere — a dead end.
// Fleeing into a corner is not fleeing.
func (g *Game) Farther(from, awayFrom string) (string, bool) {
	dist := g.distances(awayFrom)

	here, placed := dist[from]
	if !placed {
		return "", false
	}

	best, found := "", false

	for _, next := range g.doors[from] {
		d, reachable := dist[next]
		if !reachable || d <= here {
			continue
		}

		if !found || d > dist[best] {
			best, found = next, true
		}
	}

	return best, found
}

// distances is how many doors each region is from one region.
func (g *Game) distances(from string) map[string]int {
	dist := map[string]int{from: 0}
	queue := []string{from}

	for len(queue) > 0 {
		here := queue[0]
		queue = queue[1:]

		for _, next := range g.doors[here] {
			if _, seen := dist[next]; seen {
				continue
			}

			dist[next] = dist[here] + 1
			queue = append(queue, next)
		}
	}

	return dist
}

// Sheet gives an actor what it is armed with. Unset is melee.
func (g *Game) Sheet(o testimony.Observer, sheet Sheet) {
	g.sheets[o] = sheet
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
				g.selves[o] = Self{
					Sheet:    g.sheets[o],
					Where:    p.Where,
					Adjacent: g.doors[p.Where],
					Fences:   g.fears[o],
				}
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
