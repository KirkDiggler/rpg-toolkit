// Copyright (C) 2026 Kirk Diggler
// SPDX-License-Identifier: GPL-3.0-or-later

package goal

import (
	"fmt"

	"github.com/KirkDiggler/rpg-toolkit/world/graph"
	"github.com/KirkDiggler/rpg-toolkit/world/journal"
	"github.com/KirkDiggler/rpg-toolkit/world/quest"
)

// Reading is the region a goal is asked about: the present as anyone in it
// holds it, and the census of every job on the board.
//
// Both halves are folds. Nothing here is a stored score, a progress bar, or a
// tally of who did what — ask the same question of the same journal twice and
// it answers the same, and roll the journal back and the answer rolls back with
// it.
type Reading struct {
	// Graph is the declared structure the present is derived from.
	Graph *graph.World

	// Log is everything that has happened, across every party.
	Log *journal.Journal

	// Ledger is the jobs in play, for asking about populations.
	Ledger *quest.Ledger
}

// View returns the present as one observer holds it.
//
// Pass the empty id for the truth view, which is usually right for a guild
// goal: a region is not a creature and does not act on beliefs.
func (r Reading) View(observer journal.EntityID) *graph.State {
	if observer == "" {
		return r.Graph.Truth(r.Log)
	}

	return r.Graph.StateFor(observer, r.Log)
}

// Census counts a job's whole population, and reports whether that job is on
// the board at all.
func (r Reading) Census(job string) (quest.Tally, bool) {
	board, ok := r.Ledger.Board(job)
	if !ok {
		return quest.Tally{}, false
	}

	return board.Tally(r.Graph, r.Log), true
}

// Condition is one thing that has to be true of the region.
//
// Open, like [quest.Predicate] and for the same reason: a condition only reads,
// so a guild with a question this package does not have may ask its own. It
// cannot write, cannot act, and is handed no way to find out who is
// responsible for anything.
type Condition interface {
	// Holds answers the question against the region as it currently stands.
	Holds(r Reading) bool

	// Describe renders the question for a goal board. Prose for humans.
	Describe() string
}

// Present holds when a question about derived state holds, in a named
// observer's view.
//
// Leave Observer empty for the truth view.
//
// The predicate is a [quest.Predicate], reused deliberately — the question
// "is this camp still hostile" does not change shape because a guild is asking
// it rather than a contract. A goal has no subject, though, so a predicate that
// names [quest.InstanceSubject] has nothing to resolve to; see the package
// findings.
type Present struct {
	Observer  journal.EntityID
	Predicate quest.Predicate
}

// Holds asks the predicate against the observer's present.
func (c Present) Holds(r Reading) bool {
	if c.Predicate == nil {
		return false
	}

	return c.Predicate.Holds(quest.Bindings{State: r.View(c.Observer)})
}

// Describe renders the question.
func (c Present) Describe() string {
	if c.Predicate == nil {
		return "nothing"
	}
	if c.Observer == "" {
		return c.Predicate.Describe()
	}

	return fmt.Sprintf("%s (as %s sees it)", c.Predicate.Describe(), c.Observer)
}

// Population holds when a job's whole population has settled into a shape.
//
// This is how a goal spans a crowd: not "did this company free their hostage"
// but "is anybody still in that cell", which is the same question no matter how
// many companies took the job or which of them managed what.
//
// A job that is not on the board does not hold. A goal waiting on a population
// that does not exist is a content mistake, and answering "true, vacuously" for
// it would hide the mistake behind a met goal.
type Population struct {
	// Job is the template id whose population is counted.
	Job string

	// Shape is what that population has to look like.
	Shape quest.Distribution
}

// Holds asks the distribution against the job's census.
func (c Population) Holds(r Reading) bool {
	if c.Shape == nil {
		return false
	}
	census, ok := r.Census(c.Job)
	if !ok {
		return false
	}

	return c.Shape.Holds(census)
}

// Describe renders the question.
func (c Population) Describe() string {
	if c.Shape == nil {
		return fmt.Sprintf("nothing about %s", c.Job)
	}

	return fmt.Sprintf("on %s, %s", c.Job, c.Shape.Describe())
}

// Everything holds when all of its parts hold. An empty Everything does not
// hold.
//
// Empty is false here, unlike [quest.All], and the difference is deliberate: an
// empty conjunction is a goal an author has not finished writing, and a goal
// that meets itself the moment it is declared would fire its unlock before
// anybody played. The constructor refuses one outright; this is the second
// guard.
type Everything []Condition

// Holds reports whether every part holds.
func (c Everything) Holds(r Reading) bool {
	if len(c) == 0 {
		return false
	}
	for _, part := range c {
		if !part.Holds(r) {
			return false
		}
	}

	return true
}

// Describe renders the question.
func (c Everything) Describe() string {
	out := ""
	for i, part := range c {
		if i > 0 {
			out += " and "
		}
		out += part.Describe()
	}
	if out == "" {
		return "nothing at all"
	}

	return out
}

// Either holds when at least one part holds. An empty Either does not hold.
type Either []Condition

// Holds reports whether any part holds.
func (c Either) Holds(r Reading) bool {
	for _, part := range c {
		if part.Holds(r) {
			return true
		}
	}

	return false
}

// Describe renders the question.
func (c Either) Describe() string {
	out := ""
	for i, part := range c {
		if i > 0 {
			out += " or "
		}
		out += part.Describe()
	}
	if out == "" {
		return "nothing at all"
	}

	return out
}
