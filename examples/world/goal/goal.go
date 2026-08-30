// Copyright (C) 2026 Kirk Diggler
// SPDX-License-Identifier: GPL-3.0-or-later

package goal

import (
	"errors"
	"fmt"
	"slices"
	"time"
)

// Clock is the one capability a goal needs from outside the world: what time it
// is now.
//
// It is injected and never defaulted. There is no world clock in this module
// and none is wanted — a deadline is arithmetic on a timestamp, not a
// simulation of time passing — so the host supplies whatever it already
// considers authoritative, and this package asks it once per observation.
//
// Implementations must be safe for concurrent use.
type Clock interface {
	// Now reports the current moment.
	Now() time.Time
}

// Goal is one thing a guild is trying to make true about a region, and the
// moment it has to be true by.
type Goal struct {
	// ID names the goal.
	ID string

	// Name is what a goal board would print.
	Name string

	// Deadline is the moment the goal has to be met *before*.
	//
	// Strictly before: meeting the conditions at the very instant of the
	// deadline is a miss, the same way finishing a job as the weekend starts is
	// not finishing it before the weekend. See [Tracker.Observe].
	Deadline time.Time

	// Conditions all have to hold at the same observation.
	Conditions []Condition
}

// Status is where a goal stands.
type Status string

const (
	// StatusOpen means the goal is still live: unmet, and the deadline has not
	// passed.
	StatusOpen Status = "open"

	// StatusMet means the conditions held before the deadline. Terminal.
	StatusMet Status = "met"

	// StatusMissed means the deadline arrived without them. Terminal — and
	// terminal is what stops a late finish from retro-firing the unlock.
	StatusMissed Status = "missed"
)

// Settled reports whether a status is terminal.
func (s Status) Settled() bool {
	return s == StatusMet || s == StatusMissed
}

// EventKind names an emission.
type EventKind string

const (
	// EventGoalMet reports a goal met before its deadline. Fires once.
	EventGoalMet EventKind = "goal.met"

	// EventGoalMissed reports a deadline that arrived first. Fires once.
	EventGoalMissed EventKind = "goal.missed"
)

// Event is what a host subscribes to, returned as a value and never published.
//
// What an unlock *means* is the host's business: a bonus stream, a title, a
// paragraph in a newsletter. This package emits that it happened and never
// learns what anybody made of it.
type Event struct {
	Kind     EventKind
	GoalID   string
	Name     string
	Deadline time.Time

	// At is the moment the clock reported when this fired.
	At time.Time
}

// ConditionStatus is one line of a goal board.
type ConditionStatus struct {
	Description string
	Holds       bool
}

// Standing is where one goal stands at one observation.
type Standing struct {
	GoalID string
	Name   string

	// Conditions is every part and whether it currently holds, derived fresh.
	Conditions []ConditionStatus

	// Holds reports whether every condition holds right now, regardless of the
	// clock. A goal can hold and still be missed.
	Holds bool

	// Status is where the goal stands after this observation.
	Status Status

	// Deadline is the moment it has to be met before.
	Deadline time.Time
}

// Report is one observation of every goal in play.
type Report struct {
	Goals  []Standing
	Events []Event
}

// ErrNoClock reports a tracker built without one.
var ErrNoClock = errors.New(
	"this needs a clock — something that can say what time it is now, so a deadline has anything to be " +
		"checked against")

// ErrNoGoalID reports a goal with no id.
var ErrNoGoalID = errors.New("this goal needs an id — a short name the rest of the content refers to it by")

// ErrNoDeadline reports a goal with no deadline.
var ErrNoDeadline = errors.New(
	"this goal needs a deadline — the moment it has to be finished before. A goal with no time limit is a " +
		"different thing and this is not it")

// ErrNoConditions reports a goal nobody could fail.
var ErrNoConditions = errors.New(
	"this goal needs at least one condition — without one there is nothing for the region to achieve, and " +
		"it would count as done the moment it was written")

// ErrDuplicateGoal reports the same goal declared twice.
var ErrDuplicateGoal = errors.New("this goal is declared twice — each one needs its own id")

// TrackerConfig supplies a tracker's parts. Both are required; neither is
// defaulted.
type TrackerConfig struct {
	// Clock is what the deadlines are checked against.
	Clock Clock

	// Goals are what the guild is trying to do.
	Goals []Goal
}

type tracked struct {
	goal   Goal
	status Status
}

// Tracker watches every goal in play and settles each one exactly once.
type Tracker struct {
	clock Clock
	goals []*tracked
	index map[string]*tracked
}

// NewTracker validates the goals and returns the tracker that watches them.
//
// Everything is checked up front rather than at the first observation, so a
// guild that has half-written a goal is told before anybody plays. Returns
// [ErrNoClock], [ErrNoGoalID], [ErrNoDeadline], [ErrNoConditions], or
// [ErrDuplicateGoal].
func NewTracker(cfg TrackerConfig) (*Tracker, error) {
	if cfg.Clock == nil {
		return nil, ErrNoClock
	}

	t := &Tracker{clock: cfg.Clock, index: make(map[string]*tracked, len(cfg.Goals))}
	for _, g := range cfg.Goals {
		if err := validate(g); err != nil {
			return nil, err
		}
		if _, seen := t.index[g.ID]; seen {
			return nil, fmt.Errorf("%q: %w", g.ID, ErrDuplicateGoal)
		}
		entry := &tracked{goal: g, status: StatusOpen}
		t.goals = append(t.goals, entry)
		t.index[g.ID] = entry
	}

	return t, nil
}

func validate(g Goal) error {
	if g.ID == "" {
		return ErrNoGoalID
	}
	if g.Deadline.IsZero() {
		return fmt.Errorf("%q: %w", g.ID, ErrNoDeadline)
	}
	if len(g.Conditions) == 0 {
		return fmt.Errorf("%q: %w", g.ID, ErrNoConditions)
	}

	return nil
}

// Status returns where a goal stands, and whether it is being watched at all.
func (t *Tracker) Status(id string) (Status, bool) {
	entry, ok := t.index[id]
	if !ok {
		return "", false
	}

	return entry.status, true
}

// Goals returns the declared goals, in the order they were given.
func (t *Tracker) Goals() []Goal {
	out := make([]Goal, 0, len(t.goals))
	for _, entry := range t.goals {
		out = append(out, entry.goal)
	}

	return out
}

// Observe reads the region, settles whatever the clock says is settled, and
// reports.
//
// It writes nothing: no facts, no edges, no claims. The only thing it changes
// is a goal's own status, and only from open into a terminal state.
//
// The order of the two questions is the whole of the deadline's honesty:
//
//   - Met, but only if the clock is strictly before the deadline. Meeting the
//     conditions at the deadline instant itself is not meeting them before it.
//   - Otherwise, if the clock has reached the deadline, the goal is missed —
//     and missed is terminal, so finishing afterwards changes the world and not
//     the unlock.
//
// A goal that is neither stays open, and an already settled goal is reported
// unchanged and emits nothing however many times it is observed.
func (t *Tracker) Observe(r Reading) Report {
	now := t.clock.Now()
	report := Report{Goals: make([]Standing, 0, len(t.goals))}

	for _, entry := range t.goals {
		one, events := entry.observe(r, now)
		report.Goals = append(report.Goals, one)
		report.Events = append(report.Events, events...)
	}

	return report
}

func (e *tracked) observe(r Reading, now time.Time) (Standing, []Event) {
	report := Standing{
		GoalID:     e.goal.ID,
		Name:       e.goal.Name,
		Conditions: make([]ConditionStatus, 0, len(e.goal.Conditions)),
		Deadline:   e.goal.Deadline,
	}

	holds := true
	for _, condition := range e.goal.Conditions {
		held := condition != nil && condition.Holds(r)
		report.Conditions = append(report.Conditions, ConditionStatus{
			Description: describe(condition),
			Holds:       held,
		})
		holds = holds && held
	}
	// NewTracker refuses a goal with no conditions, so an empty set cannot
	// reach here and meet itself vacuously.
	report.Holds = holds

	events := e.settle(report.Holds, now)
	report.Status = e.status

	return report, events
}

// settle moves an open goal into a terminal state. A settled goal is never
// looked at again — which is what "fires once" and "no retro-unlock" both mean.
func (e *tracked) settle(holds bool, now time.Time) []Event {
	if e.status.Settled() {
		return nil
	}

	switch {
	case holds && now.Before(e.goal.Deadline):
		e.status = StatusMet

		return []Event{e.event(EventGoalMet, now)}
	case !now.Before(e.goal.Deadline):
		e.status = StatusMissed

		return []Event{e.event(EventGoalMissed, now)}
	default:
		return nil
	}
}

func (e *tracked) event(kind EventKind, now time.Time) Event {
	return Event{
		Kind:     kind,
		GoalID:   e.goal.ID,
		Name:     e.goal.Name,
		Deadline: e.goal.Deadline,
		At:       now,
	}
}

func describe(c Condition) string {
	if c == nil {
		return "nothing"
	}

	return c.Describe()
}

// Describe renders a goal for a goal board.
func (g Goal) Describe() string {
	parts := make([]string, 0, len(g.Conditions))
	for _, c := range g.Conditions {
		parts = append(parts, describe(c))
	}
	if len(parts) == 0 {
		return g.Name
	}

	return fmt.Sprintf("%s: %s", g.Name, joinAnd(parts))
}

func joinAnd(parts []string) string {
	switch len(parts) {
	case 0:
		return ""
	case 1:
		return parts[0]
	default:
		return fmt.Sprintf("%s and %s",
			joinAll(parts[:len(parts)-1]), parts[len(parts)-1])
	}
}

func joinAll(parts []string) string {
	out := ""
	for i, p := range slices.Clone(parts) {
		if i > 0 {
			out += ", "
		}
		out += p
	}

	return out
}
