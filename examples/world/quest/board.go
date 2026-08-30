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

// Bucket names one state a subject can be in, and how to recognise it.
//
// Buckets are how a population is counted. They are tried in declared order and
// the first match wins, which makes the list a priority order rather than a
// partition — necessary, because graph's flags only ever go up: a redeemed
// hostage still carries the flag that says they turned, so "redeemed" has to be
// asked first or it will never be the answer.
type Bucket struct {
	// Name is what this bucket is called in a [Distribution].
	Name string

	// Observer is whose view the subject is judged in. Empty means the truth
	// view, which is usually right for counting a population — a census is
	// bookkeeping, not behaviour.
	Observer journal.EntityID

	// Predicate is how a subject is recognised as being in this bucket. Use
	// [InstanceSubject] to name the subject being classified.
	Predicate Predicate
}

// Tally is the census of a population: how many of a job's subjects currently
// sit in each declared bucket.
//
// It is a fold over the world, not over the quest ledger. Nothing about who
// claimed what enters into it — three hostages are three hostages whether or
// not anybody took the job — which is why a distribution can never disagree
// with the state of the world.
type Tally struct {
	total  int
	counts map[string]int
}

// Total is the size of the population.
func (t Tally) Total() int {
	return t.total
}

// Count is how many subjects fall into a bucket. A bucket nobody is in is zero,
// including a bucket that was never declared.
func (t Tally) Count(bucket string) int {
	return t.counts[bucket]
}

// Buckets returns the occupied bucket names in a stable order, for a
// transcript.
func (t Tally) Buckets() []string {
	out := make([]string, 0, len(t.counts))
	for name := range t.counts {
		out = append(out, name)
	}
	slices.Sort(out)

	return out
}

// Distribution is a question asked of a whole population rather than of one
// subject.
//
// This is the shape a follow-up job waits on: not "did this party win" but
// "how did the whole thing come out".
type Distribution interface {
	// Holds answers the question against a census.
	Holds(t Tally) bool

	// Describe renders the question for a transcript.
	Describe() string
}

// NoneIn holds when a bucket is empty.
type NoneIn struct{ Bucket string }

// Holds reports whether the bucket is empty.
func (d NoneIn) Holds(t Tally) bool { return t.Count(d.Bucket) == 0 }

// Describe renders the question.
func (d NoneIn) Describe() string { return fmt.Sprintf("none are %s", d.Bucket) }

// AllIn holds when every subject is in a bucket.
//
// An empty population never satisfies it: "all of nothing" is a true statement
// about arithmetic and a false one about the world, and a follow-up job that
// opened because a population was empty would be nonsense.
type AllIn struct{ Bucket string }

// Holds reports whether the whole population is in the bucket.
func (d AllIn) Holds(t Tally) bool { return t.Total() > 0 && t.Count(d.Bucket) == t.Total() }

// Describe renders the question.
func (d AllIn) Describe() string { return fmt.Sprintf("all are %s", d.Bucket) }

// AtLeastIn holds when a bucket has reached a count.
type AtLeastIn struct {
	Bucket string
	Count  int
}

// Holds reports whether the bucket has reached the count.
func (d AtLeastIn) Holds(t Tally) bool { return t.Count(d.Bucket) >= d.Count }

// Describe renders the question.
func (d AtLeastIn) Describe() string {
	return fmt.Sprintf("at least %d are %s", d.Count, d.Bucket)
}

// Every holds when all of its parts hold. An empty Every holds.
type Every []Distribution

// Holds reports whether every part holds.
func (d Every) Holds(t Tally) bool {
	for _, part := range d {
		if !part.Holds(t) {
			return false
		}
	}

	return true
}

// Describe renders the question.
func (d Every) Describe() string {
	out := ""
	for i, part := range d {
		if i > 0 {
			out += " and "
		}
		out += part.Describe()
	}
	if out == "" {
		return "anything"
	}

	return out
}

// Successor is a job that comes onto the board once a population settles into a
// declared shape.
//
// The rescue that failed everywhere becomes the job of undoing it, and nobody
// wrote "if all three are turned, offer this" as code — the shape is data and
// the opening is a fold.
type Successor struct {
	// Opens is the job that appears. Leave its Subjects empty: they are filled
	// from the bucket this successor targets, because the whole point is that
	// it is about exactly the people it found there.
	Opens Template

	// When is the shape of the population that opens it.
	When Distribution

	// SubjectsFrom names the bucket whose members become the new job's
	// subjects.
	SubjectsFrom string
}

// ErrBoardExhausted reports a claim on a job nobody is left to take.
var ErrBoardExhausted = errors.New(
	"every subject on this job has already been taken — add more names to it if more parties should be able to")

// ErrUnknownBucket reports a successor targeting a bucket its job never
// declared.
var ErrUnknownBucket = errors.New(
	"this follow-up targets a bucket the job does not have — the bucket names here must match the ones above")

// ErrSuccessorHasSubjects reports a successor that tried to name its own
// subjects.
var ErrSuccessorHasSubjects = errors.New(
	"leave a follow-up's subjects empty — it is about whoever ended up in the bucket it targets, " +
		"and those names are not known until then")

// Board is one job's population: the subjects nobody has taken yet, and the
// instances that claims have minted.
type Board struct {
	template  Template
	unclaimed []journal.EntityID
	instances []*Instance
	opened    map[string]bool
}

// BoardReport is one observation of a whole job.
type BoardReport struct {
	// TemplateID names the job.
	TemplateID string

	// Tally is the census of its population.
	Tally Tally

	// Instances is one report per claimed instance.
	Instances []Report

	// Opens are the follow-up jobs this observation activated, with their
	// subjects already filled in from the buckets they target.
	Opens []Template

	// Events are the transitions this observation caused.
	Events []Event
}

// NewBoard puts a job's population on the board.
//
// Returns one of the template errors, written for whoever is filling the form:
// [ErrNoTemplateID], [ErrNoSubjects], [ErrNoObjectives], [ErrDuplicateSubject],
// [ErrNoBuckets], [ErrUnknownBucket], or [ErrSuccessorHasSubjects].
func NewBoard(t Template) (*Board, error) {
	if err := validate(t); err != nil {
		return nil, err
	}

	return &Board{
		template:  t,
		unclaimed: slices.Clone(t.Subjects),
		opened:    make(map[string]bool),
	}, nil
}

func validate(t Template) error {
	if t.ID == "" {
		return ErrNoTemplateID
	}
	if len(t.Subjects) == 0 {
		return fmt.Errorf("%q: %w", t.ID, ErrNoSubjects)
	}
	if len(t.Objectives) == 0 {
		return fmt.Errorf("%q: %w", t.ID, ErrNoObjectives)
	}
	for i, subject := range t.Subjects {
		if slices.Contains(t.Subjects[:i], subject) {
			return fmt.Errorf("%q lists %q twice: %w", t.ID, subject, ErrDuplicateSubject)
		}
	}

	return validateSuccessors(t)
}

func validateSuccessors(t Template) error {
	if len(t.Successors) > 0 && len(t.Buckets) == 0 {
		return fmt.Errorf("%q: %w", t.ID, ErrNoBuckets)
	}

	for _, s := range t.Successors {
		if len(s.Opens.Subjects) > 0 {
			return fmt.Errorf("%q's follow-up %q: %w", t.ID, s.Opens.ID, ErrSuccessorHasSubjects)
		}
		if !slices.ContainsFunc(t.Buckets, func(b Bucket) bool { return b.Name == s.SubjectsFrom }) {
			return fmt.Errorf("%q's follow-up %q wants bucket %q: %w",
				t.ID, s.Opens.ID, s.SubjectsFrom, ErrUnknownBucket)
		}
	}

	return nil
}

// TemplateID names the job this board holds.
func (b *Board) TemplateID() string {
	return b.template.ID
}

// Available is how many subjects nobody has taken yet.
func (b *Board) Available() int {
	return len(b.unclaimed)
}

// Instances returns the instances claims have minted, in claim order.
func (b *Board) Instances() []*Instance {
	return slices.Clone(b.instances)
}

// Claim takes the next subject off the board and mints the claimant's own
// instance of the job.
//
// Nothing ever puts a subject back. Finishing, failing, and giving up all leave
// the board where they found it, because the person the job was about is not
// available again just because somebody stopped trying.
//
// Returns [ErrBoardExhausted] when every subject has been taken.
func (b *Board) Claim(by journal.EntityID) (*Instance, []Event, error) {
	if len(b.unclaimed) == 0 {
		return nil, nil, fmt.Errorf("%q: %w", b.template.ID, ErrBoardExhausted)
	}

	subject := b.unclaimed[0]
	b.unclaimed = b.unclaimed[1:]

	instance := &Instance{
		id:       fmt.Sprintf("%s#%s", b.template.ID, subject),
		template: b.template,
		subject:  subject,
		claimant: by,
		status:   StatusClaimed,
	}
	b.instances = append(b.instances, instance)

	return instance, []Event{instance.event(EventQuestClaimed)}, nil
}

// Tally counts the whole population, claimed or not, by the declared buckets.
func (b *Board) Tally(w *graph.World, log *journal.Journal) Tally {
	tally := Tally{total: len(b.template.Subjects), counts: make(map[string]int)}
	for _, subject := range b.template.Subjects {
		if name := b.classify(w, log, subject); name != "" {
			tally.counts[name]++
		}
	}

	return tally
}

func (b *Board) classify(w *graph.World, log *journal.Journal, subject journal.EntityID) string {
	for _, bucket := range b.template.Buckets {
		if bucket.Predicate == nil {
			continue
		}
		bindings := Bindings{State: view(w, log, bucket.Observer, subject), Subject: subject}
		if bucket.Predicate.Holds(bindings) {
			return bucket.Name
		}
	}

	return ""
}

// membersOf returns the population's subjects currently in a bucket, in
// declared order.
func (b *Board) membersOf(w *graph.World, log *journal.Journal, bucket string) []journal.EntityID {
	var out []journal.EntityID
	for _, subject := range b.template.Subjects {
		if b.classify(w, log, subject) == bucket {
			out = append(out, subject)
		}
	}

	return out
}

// Observe reports every instance, takes the census, and activates any follow-up
// whose shape the population has reached.
//
// A follow-up opens once. Having opened, it is remembered as opened, so a
// population that drifts back across the line does not offer the same job
// twice.
func (b *Board) Observe(w *graph.World, log *journal.Journal) BoardReport {
	report := BoardReport{TemplateID: b.template.ID, Tally: b.Tally(w, log)}

	for _, instance := range b.instances {
		one := instance.Observe(w, log)
		report.Instances = append(report.Instances, one)
		report.Events = append(report.Events, one.Events...)
	}

	for _, successor := range b.template.Successors {
		opened := b.activate(w, log, successor, report.Tally)
		if opened == nil {
			continue
		}
		report.Opens = append(report.Opens, *opened)
		report.Events = append(report.Events, Event{
			Kind:       EventBoardOpened,
			TemplateID: opened.ID,
		})
	}

	return report
}

// activate fills a successor's subjects from the bucket it targets, once.
// A successor whose shape holds but whose bucket is empty is left unopened:
// there is nobody for the job to be about, and it may yet fill.
func (b *Board) activate(
	w *graph.World, log *journal.Journal, s Successor, tally Tally,
) *Template {
	if b.opened[s.Opens.ID] || s.When == nil || !s.When.Holds(tally) {
		return nil
	}

	subjects := b.membersOf(w, log, s.SubjectsFrom)
	if len(subjects) == 0 {
		return nil
	}

	opened := s.Opens
	opened.Subjects = subjects
	b.opened[s.Opens.ID] = true

	return &opened
}

// view is where an observer's present comes from, for objectives and buckets
// alike.
func view(w *graph.World, log *journal.Journal, observer, subject journal.EntityID) *graph.State {
	switch observer {
	case "":
		return w.Truth(log)
	case InstanceSubject:
		return w.StateFor(subject, log)
	default:
		return w.StateFor(observer, log)
	}
}
