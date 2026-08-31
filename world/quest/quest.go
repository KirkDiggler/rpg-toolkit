// Copyright (C) 2026 Kirk Diggler
// SPDX-License-Identifier: GPL-3.0-or-later

package quest

import (
	"errors"
	"fmt"
	"slices"

	"github.com/KirkDiggler/rpg-toolkit/world/graph"
	"github.com/KirkDiggler/rpg-toolkit/world/journal"
)

// Objective is one condition a quest wants met.
type Objective struct {
	// ID names the objective within its template.
	ID string

	// Observer is whose derived present the predicate is read against.
	//
	// For an objective about how somebody behaves, name them: a camp that
	// follows an impostor is not hostile in its own view, and its own view is
	// what it acts on. Name [InstanceSubject] to read it in the view of
	// whoever this instance is about. Leave it empty to read the truth view,
	// which is right for bookkeeping the world cannot be mistaken about and
	// wrong for anything a creature decides.
	Observer journal.EntityID

	// Predicate is the question.
	Predicate Predicate
}

// Template is authored quest content: what the job is, who it can be taken
// about, what closes it, and what it turns into.
type Template struct {
	// ID names the template.
	ID string

	// Name is what a quest board would print.
	Name string

	// Subjects is the population: the individuals or places this job can be
	// taken about, one instance each.
	//
	// The list IS the population size. A count and a list of names could
	// disagree with each other, and then one of them would be a lie; the names
	// are the half that instances actually need.
	Subjects []journal.EntityID

	// Objectives all have to hold for an instance to be finished.
	Objectives []Objective

	// Failure closes an instance the other way when it holds. Leave it nil for
	// a job that can only be finished or given up.
	//
	// It is checked before the objectives, so an instance that is somehow both
	// won and lost is lost: the conditions that end a quest badly tend to be
	// the ones that cannot be taken back.
	Failure *Objective

	// Buckets classify an instance by the state of its subject, for counting a
	// whole population. Tried in declared order, first match wins — so put the
	// most recent meaning first, because flags only ever go up and a redeemed
	// hostage is still carrying the flag that says they turned.
	Buckets []Bucket

	// Successors are the jobs this population turns into once it settles into
	// a declared shape.
	Successors []Successor
}

// Status is where an instance is in its lifecycle.
type Status string

const (
	// StatusClaimed means somebody has taken the job and it is unfinished. An
	// instance is born claimed: it did not exist before somebody took it.
	StatusClaimed Status = "claimed"

	// StatusCompleted means every objective held at an observation.
	StatusCompleted Status = "completed"

	// StatusFailed means the failure condition held. Terminal.
	StatusFailed Status = "failed"

	// StatusAbandoned means it was given up. Terminal.
	StatusAbandoned Status = "abandoned"
)

// Settled reports whether a status is terminal.
func (s Status) Settled() bool {
	return s == StatusCompleted || s == StatusFailed || s == StatusAbandoned
}

// EventKind names a lifecycle emission.
type EventKind string

const (
	// EventQuestClaimed reports an instance minted by a claim.
	EventQuestClaimed EventKind = "quest.claimed"

	// EventQuestCompleted reports that every objective held.
	EventQuestCompleted EventKind = "quest.completed"

	// EventQuestFailed reports that the failure condition held.
	EventQuestFailed EventKind = "quest.failed"

	// EventQuestAbandoned reports that the instance was given up.
	EventQuestAbandoned EventKind = "quest.abandoned"

	// EventBoardOpened reports a successor coming onto the board.
	EventBoardOpened EventKind = "quest.board-opened"
)

// Event is what a host subscribes to. It is returned as a value and never
// published: this package has no bus and wants none.
//
// The host decides what an event means. Three hundred experience points is a
// rulebook's opinion, not a quest's.
type Event struct {
	Kind       EventKind
	InstanceID string
	TemplateID string
	Subject    journal.EntityID
	Claimant   journal.EntityID
}

// Report is one observation of an instance.
type Report struct {
	InstanceID string

	// Met is every objective id and whether it holds right now. It is derived
	// on each observation and never remembered.
	Met map[string]bool

	// Status is where the instance stands after this observation.
	Status Status

	// Events are the lifecycle transitions this observation caused.
	Events []Event
}

// ErrNoTemplateID reports a template with no id.
var ErrNoTemplateID = errors.New("this job needs an id — a short name the rest of the content refers to it by")

// ErrNoSubjects = errors for the form-filler, not the debugger.
var ErrNoSubjects = errors.New(
	"this job needs at least one subject — the person or place each copy of it is about, " +
		"one name per copy (three hostages means three names here)")

// ErrNoObjectives reports a template nobody could fail to finish.
var ErrNoObjectives = errors.New(
	"this job needs at least one objective — without one it is finished the moment anybody takes it")

// ErrNoBuckets reports a template that declares successors but no way to count
// its population.
var ErrNoBuckets = errors.New(
	"this job needs buckets before it can have a follow-up — the follow-up opens when the population " +
		"reaches a shape, and buckets are how a copy of the job is counted toward that shape")

// ErrDuplicateSubject reports the same subject listed twice.
var ErrDuplicateSubject = errors.New(
	"this job lists the same subject twice — each copy is about a different person, so each name appears once")

// ErrClosed reports an action on an instance that has already settled.
var ErrClosed = errors.New("this copy of the job is already finished")

// Instance is one claim on one subject: the job somebody actually took.
//
// The stored state here is exactly the state that is not derivable. Who took a
// job, which person it was about, and whether it was given up are not
// consequences of anything in the world. Whether the objectives hold is, and so
// it is not stored.
type Instance struct {
	id       string
	template Template
	subject  journal.EntityID
	claimant journal.EntityID
	status   Status
}

// ID names this instance.
func (i *Instance) ID() string {
	return i.id
}

// TemplateID names the content this instance came from.
func (i *Instance) TemplateID() string {
	return i.template.ID
}

// Subject is the individual this instance is about — the claimant's own
// hostage, and nobody else's.
func (i *Instance) Subject() journal.EntityID {
	return i.subject
}

// Claimant is who took the job.
func (i *Instance) Claimant() journal.EntityID {
	return i.claimant
}

// Status returns where the instance stands.
func (i *Instance) Status() Status {
	return i.status
}

// Objectives returns the template's objectives, for a quest log.
func (i *Instance) Objectives() []Objective {
	return slices.Clone(i.template.Objectives)
}

// Abandon gives the job up and returns the events it caused.
//
// Returns [ErrClosed] for an instance that has already settled.
func (i *Instance) Abandon() ([]Event, error) {
	if i.status.Settled() {
		return nil, fmt.Errorf("%w: %q is %s", ErrClosed, i.id, i.status)
	}
	i.status = StatusAbandoned

	return []Event{i.event(EventQuestAbandoned)}, nil
}

// Observe derives each objective's observer view, evaluates its predicate, and
// reports.
//
// It writes nothing to the world: no facts, no edges, no resolver call. The
// only thing it may change is this instance's own lifecycle, and only into a
// terminal state — an instance that finished stays finished even if the world
// later moves on.
func (i *Instance) Observe(w *graph.World, log *journal.Journal) Report {
	report := Report{
		InstanceID: i.id,
		Met:        make(map[string]bool, len(i.template.Objectives)),
	}

	for _, objective := range i.template.Objectives {
		report.Met[objective.ID] = i.holds(w, log, objective)
	}

	if !i.status.Settled() {
		report.Events = i.settle(w, log, report.Met)
	}
	report.Status = i.status

	return report
}

// settle moves a live instance into a terminal state if the world says so.
// Failure is weighed before success; see [Template.Failure].
func (i *Instance) settle(w *graph.World, log *journal.Journal, met map[string]bool) []Event {
	if i.template.Failure != nil && i.holds(w, log, *i.template.Failure) {
		i.status = StatusFailed

		return []Event{i.event(EventQuestFailed)}
	}

	for _, held := range met {
		if !held {
			return nil
		}
	}
	i.status = StatusCompleted

	return []Event{i.event(EventQuestCompleted)}
}

func (i *Instance) holds(w *graph.World, log *journal.Journal, o Objective) bool {
	return i.holdsPredicate(w, log, o.Observer, o.Predicate)
}

func (i *Instance) holdsPredicate(
	w *graph.World, log *journal.Journal, observer journal.EntityID, p Predicate,
) bool {
	if p == nil {
		return false
	}

	return p.Holds(Bindings{
		State:   view(w, log, observer, i.subject),
		Subject: i.subject,
	})
}

func (i *Instance) event(kind EventKind) Event {
	return Event{
		Kind:       kind,
		InstanceID: i.id,
		TemplateID: i.template.ID,
		Subject:    i.subject,
		Claimant:   i.claimant,
	}
}
