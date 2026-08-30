// Copyright (C) 2026 Kirk Diggler
// SPDX-License-Identifier: GPL-3.0-or-later

package goal_test

import (
	"testing"
	"time"

	"github.com/stretchr/testify/suite"

	"github.com/KirkDiggler/rpg-toolkit/examples/world/goal"
	"github.com/KirkDiggler/rpg-toolkit/examples/world/graph"
	"github.com/KirkDiggler/rpg-toolkit/examples/world/journal"
	"github.com/KirkDiggler/rpg-toolkit/examples/world/quest"
	"github.com/KirkDiggler/rpg-toolkit/examples/world/scripted"
)

const (
	holdID   journal.EntityID = "hold"
	bandID   journal.EntityID = "band"
	scoutID  journal.EntityID = "scout"
	firstID  journal.EntityID = "first-captive"
	secondID journal.EntityID = "second-captive"

	belongsTo graph.Relation = "belongs-to"
	hostileTo graph.Relation = "hostile-to"

	broken graph.Flag = "broken"
	freed  graph.Flag = "freed"

	routedKind journal.Kind = "routed"
	freeKind   journal.Kind = "freeing"

	rescueJob   = "free-the-captives"
	heldBucket  = "held"
	freedBucket = "freed"

	goalID   = "pacify"
	goalName = "Pacify"

	person graph.Kind = "person"
)

// deadline is the moment everything in this suite is measured against.
var deadline = time.Date(2026, 9, 4, 17, 0, 0, 0, time.UTC)

// always and never are conditions with no opinion about the world, for
// exercising the clock without dragging a region in.
type always struct{}

func (always) Holds(goal.Reading) bool { return true }
func (always) Describe() string        { return "always" }

type never struct{}

func (never) Holds(goal.Reading) bool { return false }
func (never) Describe() string        { return "never" }

type GoalSuite struct {
	suite.Suite

	world  *graph.World
	log    *journal.Journal
	ledger *quest.Ledger
	clock  *scripted.Clock
}

func TestGoalSuite(t *testing.T) {
	suite.Run(t, new(GoalSuite))
}

func (s *GoalSuite) SetupTest() {
	w, err := graph.New(graph.Config{
		Membership: belongsTo,
		Entities: []graph.Entity{
			{ID: holdID, Kind: "faction"},
			{ID: bandID, Kind: "faction"},
			{ID: scoutID, Kind: person},
			{ID: firstID, Kind: person},
			{ID: secondID, Kind: person},
		},
		Edges: []graph.Edge{
			{From: scoutID, Rel: belongsTo, To: bandID},
			{From: holdID, Rel: hostileTo, To: bandID},
		},
		Reducers: []graph.Reducer{
			graph.Raise{On: routedKind, Flag: broken},
			graph.Raise{On: freeKind, Flag: freed},
		},
		Projections: []graph.Projection{
			graph.Retire{OnFlag: broken, Relations: []graph.Relation{hostileTo}},
		},
	})
	s.Require().NoError(err)
	s.world = w
	s.log = journal.New()

	ledger, err := quest.NewLedger(quest.Template{
		ID:       rescueJob,
		Name:     "Free the Captives",
		Subjects: []journal.EntityID{firstID, secondID},
		Objectives: []quest.Objective{{
			ID:        freedBucket,
			Predicate: quest.Flagged{Flag: freed, Of: quest.InstanceSubject},
		}},
		Buckets: []quest.Bucket{
			{Name: freedBucket, Predicate: quest.Flagged{Flag: freed, Of: quest.InstanceSubject}},
			{Name: heldBucket, Predicate: quest.Anything{}},
		},
	})
	s.Require().NoError(err)
	s.ledger = ledger

	s.clock = scripted.NewClock(deadline.Add(-24 * time.Hour))
}

func (s *GoalSuite) reading() goal.Reading {
	return goal.Reading{Graph: s.world, Log: s.log, Ledger: s.ledger}
}

func (s *GoalSuite) append(kind journal.Kind, subject journal.EntityID) {
	s.T().Helper()

	_, err := s.log.Append(journal.Fact{
		Kind: kind, Actor: scoutID, Subject: subject,
		Audience: journal.Audience{holdID, bandID, subject},
	})
	s.Require().NoError(err)
}

func (s *GoalSuite) tracker(conditions ...goal.Condition) *goal.Tracker {
	s.T().Helper()

	t, err := goal.NewTracker(goal.TrackerConfig{
		Clock: s.clock,
		Goals: []goal.Goal{{
			ID: goalID, Name: goalName, Deadline: deadline, Conditions: conditions,
		}},
	})
	s.Require().NoError(err)

	return t
}

func (s *GoalSuite) TestNewTrackerRefusesGoalsAnAuthorHasNotFinished() {
	sound := goal.Goal{ID: goalID, Name: goalName, Deadline: deadline, Conditions: []goal.Condition{always{}}}

	s.Run("a tracker with no clock is told what a clock is for", func() {
		_, err := goal.NewTracker(goal.TrackerConfig{Goals: []goal.Goal{sound}})
		s.Require().ErrorIs(err, goal.ErrNoClock)
		s.Contains(err.Error(), "what time it is now")
	})

	s.Run("a goal needs an id", func() {
		bad := sound
		bad.ID = ""
		_, err := goal.NewTracker(goal.TrackerConfig{Clock: s.clock, Goals: []goal.Goal{bad}})
		s.Require().ErrorIs(err, goal.ErrNoGoalID)
	})

	s.Run("a goal needs a deadline, and is told why a missing one is not zero", func() {
		bad := sound
		bad.Deadline = time.Time{}
		_, err := goal.NewTracker(goal.TrackerConfig{Clock: s.clock, Goals: []goal.Goal{bad}})
		s.Require().ErrorIs(err, goal.ErrNoDeadline)
		s.Contains(err.Error(), "has to be finished before")
	})

	s.Run("a goal with no conditions would be done the moment it was written", func() {
		bad := sound
		bad.Conditions = nil
		_, err := goal.NewTracker(goal.TrackerConfig{Clock: s.clock, Goals: []goal.Goal{bad}})
		s.Require().ErrorIs(err, goal.ErrNoConditions)
		s.Contains(err.Error(), "nothing for the region to achieve")
	})

	s.Run("a goal may not be declared twice", func() {
		_, err := goal.NewTracker(goal.TrackerConfig{Clock: s.clock, Goals: []goal.Goal{sound, sound}})
		s.Require().ErrorIs(err, goal.ErrDuplicateGoal)
	})
}

// TestTheDeadlineBoundary is where off-by-one lives, so it is a table.
func (s *GoalSuite) TestTheDeadlineBoundary() {
	cases := []struct {
		name    string
		now     time.Time
		holds   bool
		want    goal.Status
		emits   goal.EventKind
		emitted bool
	}{
		{
			name: "met a tick before the deadline",
			now:  deadline.Add(-time.Nanosecond), holds: true,
			want: goal.StatusMet, emits: goal.EventGoalMet, emitted: true,
		},
		{
			name: "met at the deadline instant is a miss — before means before",
			now:  deadline, holds: true,
			want: goal.StatusMissed, emits: goal.EventGoalMissed, emitted: true,
		},
		{
			name: "met a tick after the deadline is a miss",
			now:  deadline.Add(time.Nanosecond), holds: true,
			want: goal.StatusMissed, emits: goal.EventGoalMissed, emitted: true,
		},
		{
			name: "unmet a tick before the deadline is still open",
			now:  deadline.Add(-time.Nanosecond), holds: false,
			want: goal.StatusOpen, emitted: false,
		},
		{
			name: "unmet at the deadline instant is a miss",
			now:  deadline, holds: false,
			want: goal.StatusMissed, emits: goal.EventGoalMissed, emitted: true,
		},
	}

	for _, tc := range cases {
		s.Run(tc.name, func() {
			var condition goal.Condition = never{}
			if tc.holds {
				condition = always{}
			}
			s.clock.Set(tc.now)

			tracker := s.tracker(condition)
			report := tracker.Observe(s.reading())

			s.Require().Len(report.Goals, 1)
			s.Equal(tc.want, report.Goals[0].Status)
			s.Equal(tc.holds, report.Goals[0].Holds)

			if !tc.emitted {
				s.Empty(report.Events)

				return
			}
			s.Require().Len(report.Events, 1)
			s.Equal(tc.emits, report.Events[0].Kind)
			s.Equal(tc.now, report.Events[0].At)
			s.Equal(deadline, report.Events[0].Deadline)
		})
	}
}

func (s *GoalSuite) TestSettlingIsTerminalInBothDirections() {
	s.Run("met fires once and a later look is silent", func() {
		tracker := s.tracker(always{})

		first := tracker.Observe(s.reading())
		s.Require().Len(first.Events, 1)
		s.Equal(goal.EventGoalMet, first.Events[0].Kind)

		second := tracker.Observe(s.reading())
		s.Empty(second.Events)
		s.Equal(goal.StatusMet, second.Goals[0].Status)
	})

	s.Run("missed fires once, and finishing afterwards does not retro-fire", func() {
		condition := &switchable{}
		tracker, err := goal.NewTracker(goal.TrackerConfig{
			Clock: s.clock,
			Goals: []goal.Goal{{ID: goalID, Name: goalName, Deadline: deadline,
				Conditions: []goal.Condition{condition}}},
		})
		s.Require().NoError(err)

		s.clock.Set(deadline)
		missed := tracker.Observe(s.reading())
		s.Require().Len(missed.Events, 1)
		s.Equal(goal.EventGoalMissed, missed.Events[0].Kind)

		// The region gets pacified anyway, an hour late.
		condition.holds = true
		s.clock.Advance(time.Hour)

		late := tracker.Observe(s.reading())
		s.Empty(late.Events, "a late finish must not unlock anything")
		s.Equal(goal.StatusMissed, late.Goals[0].Status)
		s.True(late.Goals[0].Holds, "the region really is pacified — it is just too late")
	})
}

// switchable is a condition a test can flip after the deadline has passed.
type switchable struct{ holds bool }

func (c *switchable) Holds(goal.Reading) bool { return c.holds }
func (c *switchable) Describe() string        { return "the region is pacified" }

func (s *GoalSuite) TestConditionsReadTheWorldAndTheCensus() {
	tracker := s.tracker(
		goal.Present{Observer: holdID, Predicate: quest.NoEdge{From: holdID, Rel: hostileTo, To: bandID}},
		goal.Population{Job: rescueJob, Shape: quest.NoneIn{Bucket: heldBucket}},
	)

	s.Run("neither half holds at the start", func() {
		report := tracker.Observe(s.reading())
		s.False(report.Goals[0].Holds)
		s.False(report.Goals[0].Conditions[0].Holds)
		s.False(report.Goals[0].Conditions[1].Holds)
	})

	s.Run("one half moving is not enough", func() {
		s.append(routedKind, holdID)
		report := tracker.Observe(s.reading())
		s.True(report.Goals[0].Conditions[0].Holds)
		s.False(report.Goals[0].Conditions[1].Holds)
		s.False(report.Goals[0].Holds)
		s.Equal(goal.StatusOpen, report.Goals[0].Status)
	})

	s.Run("the census closing the other half meets the goal", func() {
		s.append(freeKind, firstID)
		s.append(freeKind, secondID)

		report := tracker.Observe(s.reading())
		s.True(report.Goals[0].Holds)
		s.Equal(goal.StatusMet, report.Goals[0].Status)
	})
}

func (s *GoalSuite) TestAGoalWaitingOnAJobNobodyOfferedDoesNotHold() {
	// Answering "true, vacuously" would hide a content mistake behind a met
	// goal, which is the worst place to hide one.
	tracker := s.tracker(goal.Population{Job: "no-such-job", Shape: quest.NoneIn{Bucket: heldBucket}})

	report := tracker.Observe(s.reading())
	s.False(report.Goals[0].Holds)
}

func (s *GoalSuite) TestEmptyConditionSetsDoNotHold() {
	s.False(goal.Everything{}.Holds(s.reading()), "an unfinished goal must not meet itself")
	s.False(goal.Either{}.Holds(s.reading()))
	s.True(goal.Everything{always{}}.Holds(s.reading()))
	s.True(goal.Either{never{}, always{}}.Holds(s.reading()))
	s.False(goal.Either{never{}, never{}}.Holds(s.reading()))
}

func (s *GoalSuite) TestObservingWritesNothing() {
	s.append(routedKind, holdID)
	tracker := s.tracker(always{})

	before := s.log.All()
	stateBefore := s.world.StateFor(holdID, s.log)

	tracker.Observe(s.reading())

	s.Equal(before, s.log.All())
	s.Equal(stateBefore, s.world.StateFor(holdID, s.log))
	s.Equal(2, s.ledger.Boards()[0].Available(), "nobody claimed anything by being looked at")
}

func (s *GoalSuite) TestGoalsDescribeThemselvesForAGoalBoard() {
	g := goal.Goal{
		ID: goalID, Name: "Pacify the Region", Deadline: deadline,
		Conditions: []goal.Condition{
			goal.Present{Observer: holdID, Predicate: quest.NoEdge{From: holdID, Rel: hostileTo, To: bandID}},
			goal.Population{Job: rescueJob, Shape: quest.NoneIn{Bucket: heldBucket}},
		},
	}

	s.Equal("Pacify the Region: hold is not hostile-to band (as hold sees it) "+
		"and on free-the-captives, none are held", g.Describe())
}
