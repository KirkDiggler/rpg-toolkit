// Copyright (C) 2026 Kirk Diggler
// SPDX-License-Identifier: GPL-3.0-or-later

package behavior

import (
	"cmp"
	"fmt"
	"slices"

	"github.com/KirkDiggler/rpg-toolkit/core"
	"github.com/KirkDiggler/rpg-toolkit/mind/behavior/deed"
	"github.com/KirkDiggler/rpg-toolkit/mind/perception"
)

// Game is the composition: a perception, a reader, a space, and who has
// which mind and stands where. It is the one place that holds a perception,
// and it writes to it only through perception's own doors (R1).
type Game struct {
	p      *perception.Perception
	reader Reader
	space  Space
	minds  map[core.EntityID]Mind
	sheets map[core.EntityID]Sheet
	places map[core.EntityID]string
	fears  map[core.EntityID][]core.EntityID
	names  map[core.EntityID]map[core.EntityID]Name
}

// NewInput is what a game is built from: how to read a payload, and how to
// measure the world.
type NewInput struct {
	Reader Reader
	Space  Space
}

// New builds an empty game. ErrNoReader without a reader, ErrNoSpace
// without a space.
func New(in *NewInput) (*Game, error) {
	if in == nil || in.Reader == nil {
		return nil, fmt.Errorf("new: %w", ErrNoReader)
	}

	if in.Space == nil {
		return nil, fmt.Errorf("new: %w", ErrNoSpace)
	}

	p, err := perception.New()
	if err != nil {
		return nil, fmt.Errorf("new: %w", err)
	}

	return &Game{
		p:      p,
		reader: in.Reader,
		space:  in.Space,
		minds:  make(map[core.EntityID]Mind),
		sheets: make(map[core.EntityID]Sheet),
		places: make(map[core.EntityID]string),
		fears:  make(map[core.EntityID][]core.EntityID),
		names:  make(map[core.EntityID]map[core.EntityID]Name),
	}, nil
}

// SheetInput is an actor and what it is armed with.
type SheetInput struct {
	Actor core.EntityID
	Sheet Sheet
}

// Sheet gives an actor what it is armed with. Unset is melee.
func (g *Game) Sheet(in *SheetInput) {
	g.sheets[in.Actor] = in.Sheet
}

// MindInput is an actor and the mind it gets.
type MindInput struct {
	Actor core.EntityID
	Mind  Mind
}

// Mind gives an actor a mind.
func (g *Game) Mind(in *MindInput) {
	g.minds[in.Actor] = in.Mind
}

// PlaceInput is an actor and the place it stands.
type PlaceInput struct {
	Actor core.EntityID
	Where string
}

// Place records where an actor stands. It is the actor's own position — a
// fact about its own sheet, not about anyone else — and it arrives as a
// value because nothing in behaviour reads the world (R12).
func (g *Game) Place(in *PlaceInput) {
	g.places[in.Actor] = in.Where
}

// FrightenInput is an actor and the subject that frightened it.
type FrightenInput struct {
	Actor  core.EntityID
	Source core.EntityID
}

// Frighten puts the frightened condition on an actor: it may not willingly
// move toward the source. Whether the actor can currently perceive that
// subject is its own problem — a fence on something you cannot place forbids
// nothing, which is what being afraid of something you cannot see feels like.
func (g *Game) Frighten(in *FrightenInput) {
	g.fears[in.Actor] = append(g.fears[in.Actor], in.Source)
}

// Observe runs one perception pass. It is perception's own door, passed
// through; behaviour adds nothing to what may be perceived.
func (g *Game) Observe(pass perception.Pass) error {
	if _, err := g.p.Observe(pass); err != nil {
		return err
	}

	return nil
}

// Report lands discrete testimony on one actor — the stage uses it to tell a
// witness what they saw somebody do. It is perception's own door, passed
// through; behaviour adds nothing to what may be said.
func (g *Game) Report(in perception.ReportInput) error {
	if _, err := g.p.Report(in); err != nil {
		return err
	}

	return nil
}

// Held is everything one actor holds, as perception holds it. The stage
// reads it to say a deed in the witness's own terms.
func (g *Game) Held(observer core.EntityID) ([]perception.Holding, error) {
	return g.p.Held(observer)
}

// SituationInput is whose situation, and when.
type SituationInput struct {
	Actor core.EntityID
	At    uint64
}

// Situation assembles everything one actor has to go on: reads every
// holding, asks the mind which are one thing, folds them, and names as it
// goes. Everything in it is the caller's to keep: the situation copies what
// it reports, so nothing a consumer does to it reaches the game's own
// sheets.
//
// A contact the actor has no word for is offered to its mind. A name the
// mind gives is recorded on the contact's bearer and the next situation
// finds it already there (R4).
func (g *Game) Situation(in *SituationInput) (*Situation, error) {
	mind, minded := g.minds[in.Actor]
	if !minded {
		return nil, fmt.Errorf("situation: %w: %s", ErrNoMind, in.Actor)
	}

	where, placed := g.places[in.Actor]
	if !placed {
		return nil, fmt.Errorf("situation: %w: %s", ErrNoSelf, in.Actor)
	}

	holdings, err := g.read(in.Actor)
	if err != nil {
		return nil, err
	}

	judged, err := mind.Judge(&JudgeInput{Holdings: holdings, At: in.At})
	if err != nil {
		return nil, err
	}

	contacts := fold(holdings, judged.Same)

	for i := range contacts {
		if err := g.name(in.Actor, mind, &contacts[i]); err != nil {
			return nil, err
		}
	}

	return &Situation{
		Actor:    in.Actor,
		Contacts: contacts,
		Self: Self{
			Sheet:  g.sheets[in.Actor],
			Where:  where,
			Fences: slices.Clone(g.fears[in.Actor]),
		},
		At: in.At,
	}, nil
}

// read is everything one actor holds, with what behaviour could read from
// each: its own deeds channel itself, everything else through the reader
// (R2).
func (g *Game) read(actor core.EntityID) ([]Holding, error) {
	held, err := g.p.Held(actor)
	if err != nil {
		return nil, err
	}

	out := make([]Holding, 0, len(held))

	for _, h := range held {
		var reading Reading

		if h.Channel == deed.Channel {
			if d, err := deed.Decode(h.Payload); err == nil {
				reading = Reading{Where: d.Where}
			}
		} else {
			r, err := g.reader.Read(h)
			if err != nil {
				return nil, fmt.Errorf("situation: read %s: %w", h.Subject, err)
			}

			if r != nil {
				reading = *r
			}
		}

		out = append(out, Holding{Holding: h, Reading: reading})
	}

	return out, nil
}

// name records what the actor calls a contact, asking the mind only when
// the actor has no word for it yet.
func (g *Game) name(actor core.EntityID, mind Mind, c *Contact) error {
	for _, h := range c.Holdings {
		if n, ok := g.names[actor][h.Subject]; ok {
			c.Name, c.Named, c.Bearer = n, true, h.Subject

			return nil
		}
	}

	named, err := mind.Name(&NameInput{Contact: *c})
	if err != nil {
		return err
	}

	if !named.Named {
		return nil
	}

	if g.names[actor] == nil {
		g.names[actor] = make(map[core.EntityID]Name)
	}

	c.Name, c.Named, c.Bearer = named.Name, true, bearer(*c)
	g.names[actor][c.Bearer] = named.Name

	return nil
}

// bearer is the subject a contact's name is recorded on: a current creature
// if there is one, else the first. A name attaches to a subject because
// subjects are the only stable handles; a contact is folded fresh every
// time.
func bearer(c Contact) core.EntityID {
	for _, h := range c.Holdings {
		if len(h.CurrentVia) > 0 && h.Creature {
			return h.Subject
		}
	}

	return c.Holdings[0].Subject
}

// fold bundles holdings into contacts by the mind's Same claims, unioned:
// A~B and B~C is one contact of three. Contacts are ordered by their
// smallest subject and holdings within one by subject, so two identical
// situations fold identically (R3).
func fold(holdings []Holding, same []Pair) []Contact {
	index := make(map[core.EntityID]int, len(holdings))
	for i, h := range holdings {
		index[h.Subject] = i
	}

	parent := make([]int, len(holdings))
	for i := range parent {
		parent[i] = i
	}

	find := func(i int) int {
		for parent[i] != i {
			parent[i] = parent[parent[i]]
			i = parent[i]
		}

		return i
	}

	for _, p := range same {
		a, okA := index[p.A]
		b, okB := index[p.B]

		if !okA || !okB {
			continue
		}

		ra, rb := find(a), find(b)
		if ra != rb {
			parent[max(ra, rb)] = min(ra, rb)
		}
	}

	groups := make(map[int][]Holding)
	for i, h := range holdings {
		r := find(i)
		groups[r] = append(groups[r], h)
	}

	out := make([]Contact, 0, len(groups))
	for _, hs := range groups {
		slices.SortFunc(hs, func(a, b Holding) int { return cmp.Compare(a.Subject, b.Subject) })
		out = append(out, Contact{Holdings: hs})
	}

	slices.SortFunc(out, func(a, b Contact) int {
		return cmp.Compare(a.Holdings[0].Subject, b.Holdings[0].Subject)
	})

	return out
}

// TurnInput is whose turn, and when.
type TurnInput struct {
	Actor core.EntityID
	At    uint64
}

// TurnOutput is what the actor decided, and the situation it decided it in —
// the stage needs both to resolve the one against the other.
type TurnOutput struct {
	Intent    Intent
	Situation Situation
}

// Turn is one actor's decision: build the situation, climb the ladder.
func (g *Game) Turn(in *TurnInput) (*TurnOutput, error) {
	s, err := g.Situation(&SituationInput{Actor: in.Actor, At: in.At})
	if err != nil {
		return nil, err
	}

	decided, err := Decide(&DecideInput{Situation: *s, Mind: g.minds[in.Actor], Space: g.space})
	if err != nil {
		return nil, err
	}

	return &TurnOutput{Intent: decided.Intent, Situation: *s}, nil
}
