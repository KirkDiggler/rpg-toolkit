// Copyright (C) 2026 Kirk Diggler
// SPDX-License-Identifier: GPL-3.0-or-later

package world

import (
	"context"
	"errors"
	"fmt"

	"github.com/KirkDiggler/rpg-toolkit/world/goal"
	"github.com/KirkDiggler/rpg-toolkit/world/graph"
	"github.com/KirkDiggler/rpg-toolkit/world/journal"
	"github.com/KirkDiggler/rpg-toolkit/world/quest"
)

// Attempt is one actor trying something difficult.
//
// This is the whole of what the composer says about resolution: who, by what
// approach, against whom, at what difficulty. What a difficulty of 15 means,
// and how "stealth" turns into a number, belong to the injected [Resolver].
type Attempt struct {
	Actor      journal.EntityID
	Approach   journal.Approach
	Target     journal.EntityID
	Difficulty int
}

// Resolver turns an [Attempt] into a [journal.Outcome]. It is the one seam a
// host fills in, and the only place randomness or rulebook mechanics may enter.
//
// Implementations must be safe for concurrent use.
type Resolver interface {
	// Resolve decides the attempt. Returning an error means the attempt could
	// not be judged at all — an unknown actor, an approach the rulebook does
	// not have — which is a wiring fault, not a failed attempt.
	Resolve(ctx context.Context, a Attempt) (journal.Outcome, error)
}

// Scenario is everything a content package declares, and nothing it injects.
//
// This is the contract between content and composer, and its shape is the
// argument: a scenario is data. It says who exists and how they stand to each
// other, what anybody may try, and what the jobs are. It does not say who
// resolves an attempt, because that is the rulebook's to supply and the same
// camp should run under a different one unchanged.
type Scenario struct {
	// Graph is the declared structure and the folds over it.
	Graph graph.Config

	// Verbs are what anyone may attempt.
	Verbs []Verb

	// Quests are the jobs on the board at the start.
	Quests []quest.Template
}

// Config is a scenario plus the things it deliberately withholds.
//
// The split is the boundary. Content declares who exists and what may be tried;
// the host injects the rulebook that judges an attempt and the clock that
// deadlines are checked against. Goals sit on the host's side too, and that is
// a statement rather than a convenience: a guild goal spans whatever content
// the region was composed from, so no single piece of content is in a position
// to declare one.
type Config struct {
	// Scenario is the declared content. Compose several with [Compose].
	Scenario Scenario

	// Resolver is the injected rulebook.
	Resolver Resolver

	// Goals are what the guild is trying to make true of the whole region.
	Goals []goal.Goal

	// Clock is what their deadlines are checked against. Required whenever
	// there are goals, and never defaulted — a deadline nobody can check is
	// worse than no deadline at all.
	Clock goal.Clock
}

// ErrNoResolver reports a world built without one.
var ErrNoResolver = errors.New(
	"this world needs a resolver — something to decide whether an attempt works, and this is the one " +
		"place the rules of your game plug in")

// ErrNoVerbs reports a world nobody could do anything in.
var ErrNoVerbs = errors.New("this world needs at least one action — without one, nobody in it can do anything")

// Result is what one act did.
type Result struct {
	// Fact is what got written down.
	Fact journal.Fact

	// Quests is what the jobs made of it.
	Quests quest.LedgerReport

	// Goals is what the guild's needles made of it. Empty when the world was
	// built without any.
	Goals goal.Report
}

// World is the composer: the assembly of a scenario with a resolver, the one
// door that writes, and the one door that reads.
//
// It is small on purpose. It owns the act loop — look the verb up, ask the
// resolver, pick the outcome, work out who saw it, write it down, let the jobs
// look — and nothing else. Structure belongs to graph, memory to journal, goals
// to quest, and meaning to the scenario.
type World struct {
	graph    *graph.World
	verbs    map[VerbName]Verb
	ledger   *quest.Ledger
	goals    *goal.Tracker
	resolver Resolver
	log      *journal.Journal
}

// New assembles a world from declared content and an injected resolver.
//
// Everything is checked here rather than at first use, so a content package
// that is missing something is told at startup. Returns [ErrNoResolver],
// [ErrNoVerbs], one of the verb errors, or whatever graph and quest say about
// the declarations they were handed.
func New(cfg Config) (*World, error) {
	if cfg.Resolver == nil {
		return nil, ErrNoResolver
	}
	if len(cfg.Scenario.Verbs) == 0 {
		return nil, ErrNoVerbs
	}

	structure, err := graph.New(cfg.Scenario.Graph)
	if err != nil {
		return nil, err
	}

	ledger, err := quest.NewLedger(cfg.Scenario.Quests...)
	if err != nil {
		return nil, err
	}

	tracker, err := newTracker(cfg)
	if err != nil {
		return nil, err
	}

	w := &World{
		graph:    structure,
		verbs:    make(map[VerbName]Verb, len(cfg.Scenario.Verbs)),
		ledger:   ledger,
		goals:    tracker,
		resolver: cfg.Resolver,
		log:      journal.New(),
	}
	for _, v := range cfg.Scenario.Verbs {
		if err := validateVerb(v); err != nil {
			return nil, err
		}
		if _, seen := w.verbs[v.Name]; seen {
			return nil, fmt.Errorf("%q: %w", v.Name, ErrDuplicateVerb)
		}
		w.verbs[v.Name] = v
	}

	return w, nil
}

// Act is the one door that writes.
//
// Look the verb up, ask the resolver if there is anything to ask, pick the
// outcome the margin reached, work out who saw it, append the fact, and let the
// jobs look at what the world became. Nothing is checked about the actor beyond
// the verb existing, because there is nothing to check.
//
// Returns [ErrUnknownVerb], or whatever the resolver returned — a resolver
// error means the attempt could not be judged, which is a wiring fault and
// leaves the journal untouched.
func (w *World) Act(ctx context.Context, act Act) (Result, error) {
	verb, ok := w.verbs[act.Verb]
	if !ok {
		return Result{}, fmt.Errorf("%w: %q", ErrUnknownVerb, act.Verb)
	}

	outcome, err := w.resolve(ctx, verb, act)
	if err != nil {
		return Result{}, fmt.Errorf("resolving %q: %w", act.Verb, err)
	}

	emission := verb.emissionFor(outcome.Margin)
	fact, err := w.log.Append(journal.Fact{
		Kind:     emission.Kind,
		Actor:    act.Actor,
		Subject:  subjectOf(emission, act),
		Audience: audienceOf(emission, act),
		Outcome:  outcome,
	})
	if err != nil {
		return Result{}, fmt.Errorf("recording %q: %w", act.Verb, err)
	}

	return Result{
		Fact:   fact,
		Quests: w.ledger.Observe(w.graph, w.log),
		Goals:  w.ObserveGoals(),
	}, nil
}

// newTracker builds the goal tracker, or nothing at all for a world with no
// goals. A clock without goals is harmless; goals without a clock are refused
// by [goal.NewTracker], which is where that message belongs.
func newTracker(cfg Config) (*goal.Tracker, error) {
	if len(cfg.Goals) == 0 {
		return nil, nil
	}

	return goal.NewTracker(goal.TrackerConfig{Clock: cfg.Clock, Goals: cfg.Goals})
}

func (w *World) resolve(ctx context.Context, verb Verb, act Act) (journal.Outcome, error) {
	if verb.Approach == "" {
		return journal.Outcome{Detail: string(verb.Name) + ": uncontested"}, nil
	}

	return w.resolver.Resolve(ctx, Attempt{
		Actor:      act.Actor,
		Approach:   verb.Approach,
		Target:     act.Target,
		Difficulty: verb.Difficulty,
	})
}

// View is the one door that reads: the present as one observer holds it.
func (w *World) View(observer journal.EntityID) *graph.State {
	return w.graph.StateFor(observer, w.log)
}

// Truth folds every fact, audience ignored. It is the game master's view and
// the test's view, and never how anyone in the world behaves.
func (w *World) Truth() *graph.State {
	return w.graph.Truth(w.log)
}

// Claim takes a subject off a job and mints the claimant's own instance.
func (w *World) Claim(templateID string, by journal.EntityID) (*quest.Instance, []quest.Event, error) {
	return w.ledger.Claim(templateID, by)
}

// Observe asks the jobs where they stand without anybody having acted.
func (w *World) Observe() quest.LedgerReport {
	return w.ledger.Observe(w.graph, w.log)
}

// ObserveGoals asks the guild's needles where they stand, and settles whatever
// the clock says is settled.
//
// [World.Act] calls it after every act, so an unlock fires on the act that
// earned it. Calling it directly is how a host notices a deadline passing when
// nobody is doing anything — which is the only way a miss can be noticed at
// all, since missing a deadline is precisely the absence of an act.
func (w *World) ObserveGoals() goal.Report {
	if w.goals == nil {
		return goal.Report{}
	}

	return w.goals.Observe(goal.Reading{Graph: w.graph, Log: w.log, Ledger: w.ledger})
}

// GoalStatus returns where a goal stands, and whether it is being watched.
func (w *World) GoalStatus(id string) (goal.Status, bool) {
	if w.goals == nil {
		return "", false
	}

	return w.goals.Status(id)
}

// Ledger is the jobs in play, for a quest log.
func (w *World) Ledger() *quest.Ledger {
	return w.ledger
}

// Journal is what has happened, in order. It is append-only and hands out
// copies, so reading it cannot change anything.
func (w *World) Journal() *journal.Journal {
	return w.log
}

// Graph is the declared structure, for deriving a state against a journal other
// than this world's — replaying a prefix, say.
func (w *World) Graph() *graph.World {
	return w.graph
}
