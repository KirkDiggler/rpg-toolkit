// Copyright (C) 2026 Kirk Diggler
// SPDX-License-Identifier: GPL-3.0-or-later

package quest

import (
	"errors"
	"fmt"
	"slices"

	"github.com/KirkDiggler/rpg-toolkit/examples/world/graph"
	"github.com/KirkDiggler/rpg-toolkit/examples/world/journal"
)

// Objective is one condition a quest wants met.
type Objective struct {
	// ID names the objective within its template.
	ID string

	// Observer is whose derived present the predicate is read against.
	//
	// For an objective about how somebody behaves, name them: a camp that
	// follows an impostor is not hostile in its own view, and its own view is
	// what it acts on. Leave it empty to read the truth view, which is
	// appropriate for bookkeeping the world cannot be mistaken about and wrong
	// for anything a creature decides.
	Observer journal.EntityID

	// Predicate is the question.
	Predicate Predicate
}

// Template is authored quest content: a name and the objectives that close it.
type Template struct {
	ID         string
	Name       string
	Objectives []Objective
}

// Status is where an instance is in its lifecycle.
type Status string

const (
	// StatusOffered means the quest exists and nobody has taken it.
	StatusOffered Status = "offered"

	// StatusClaimed means at least one party has taken it and it is unfinished.
	StatusClaimed Status = "claimed"

	// StatusCompleted means every objective held at an observation.
	StatusCompleted Status = "completed"

	// StatusAbandoned means it was given up. Terminal: an abandoned instance is
	// never completed by a later observation.
	StatusAbandoned Status = "abandoned"
)

// Claim records that somebody took the quest.
type Claim struct {
	By journal.EntityID
}

// EventKind names a lifecycle emission.
type EventKind string

const (
	// EventQuestClaimed reports the first claim on an offered instance.
	EventQuestClaimed EventKind = "quest.claimed"

	// EventQuestCompleted reports that every objective held.
	EventQuestCompleted EventKind = "quest.completed"

	// EventQuestAbandoned reports that the instance was given up.
	EventQuestAbandoned EventKind = "quest.abandoned"
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
	Claimants  []journal.EntityID
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

// ErrNoTemplate reports an instance offered without a template.
var ErrNoTemplate = errors.New("quest: template is required")

// ErrNoObjectives reports a template with nothing to satisfy. Such a quest
// would complete on its first observation, which is never what an author meant.
var ErrNoObjectives = errors.New("quest: template has no objectives")

// ErrClosed reports a claim or abandonment on an instance that has finished.
var ErrClosed = errors.New("quest: instance is closed")

// Instance is one offering of a template: its claims and its lifecycle.
//
// The stored state here is exactly the state that is not derivable. Who took a
// quest is not a consequence of anything in the world, and neither is giving it
// up; whether the objectives hold is, and so it is not stored.
type Instance struct {
	id       string
	template Template
	status   Status
	claims   []Claim
}

// Offer returns an instance of a template, ready to be claimed.
//
// Returns [ErrNoTemplate] or [ErrNoObjectives].
func Offer(id string, t Template) (*Instance, error) {
	if t.ID == "" {
		return nil, ErrNoTemplate
	}
	if len(t.Objectives) == 0 {
		return nil, fmt.Errorf("%w: %q", ErrNoObjectives, t.ID)
	}

	return &Instance{
		id:       id,
		template: t,
		status:   StatusOffered,
	}, nil
}

// ID names this instance.
func (i *Instance) ID() string {
	return i.id
}

// TemplateID names the content this instance came from.
func (i *Instance) TemplateID() string {
	return i.template.ID
}

// Status returns where the instance stands.
func (i *Instance) Status() Status {
	return i.status
}

// Claimants returns who has taken this quest, in claim order.
func (i *Instance) Claimants() []journal.EntityID {
	out := make([]journal.EntityID, 0, len(i.claims))
	for _, c := range i.claims {
		out = append(out, c.By)
	}

	return out
}

// Claim records that somebody took the quest, and returns the events it caused.
//
// Returns [ErrClosed] for a completed or abandoned instance. Claiming twice is
// not an error — a second party joining a hunt is a normal thing — but only the
// first claim moves the status.
func (i *Instance) Claim(by journal.EntityID) ([]Event, error) {
	if i.status == StatusCompleted || i.status == StatusAbandoned {
		return nil, fmt.Errorf("%w: %q is %s", ErrClosed, i.id, i.status)
	}

	first := i.status == StatusOffered
	i.claims = append(i.claims, Claim{By: by})
	if !first {
		return nil, nil
	}
	i.status = StatusClaimed

	return []Event{i.event(EventQuestClaimed)}, nil
}

// Abandon gives the quest up and returns the events it caused.
//
// Returns [ErrClosed] for an instance that has already finished.
func (i *Instance) Abandon() ([]Event, error) {
	if i.status == StatusCompleted || i.status == StatusAbandoned {
		return nil, fmt.Errorf("%w: %q is %s", ErrClosed, i.id, i.status)
	}
	i.status = StatusAbandoned

	return []Event{i.event(EventQuestAbandoned)}, nil
}

// Observe derives each objective's observer view, evaluates its predicate, and
// reports.
//
// It writes nothing to the world: no facts, no edges, no resolver call. The
// only thing it may change is this instance's own lifecycle, and only in the
// one direction — an instance whose objectives all hold becomes completed and
// stays completed even if the world later moves on.
func (i *Instance) Observe(w *graph.World, log *journal.Journal) Report {
	report := Report{
		InstanceID: i.id,
		Met:        make(map[string]bool, len(i.template.Objectives)),
	}

	all := true
	for _, objective := range i.template.Objectives {
		state := i.view(w, log, objective.Observer)
		held := objective.Predicate != nil && objective.Predicate.Holds(state)
		report.Met[objective.ID] = held
		all = all && held
	}

	if all && (i.status == StatusOffered || i.status == StatusClaimed) {
		i.status = StatusCompleted
		report.Events = append(report.Events, i.event(EventQuestCompleted))
	}
	report.Status = i.status

	return report
}

// Objectives returns the template's objectives, for a quest log.
func (i *Instance) Objectives() []Objective {
	return slices.Clone(i.template.Objectives)
}

func (i *Instance) view(w *graph.World, log *journal.Journal, observer journal.EntityID) *graph.State {
	if observer == "" {
		return w.Truth(log)
	}

	return w.StateFor(observer, log)
}

func (i *Instance) event(kind EventKind) Event {
	return Event{
		Kind:       kind,
		InstanceID: i.id,
		TemplateID: i.template.ID,
		Claimants:  i.Claimants(),
	}
}
