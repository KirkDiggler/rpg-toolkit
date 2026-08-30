// Copyright (C) 2026 Kirk Diggler
// SPDX-License-Identifier: GPL-3.0-or-later

package quest_test

import (
	"testing"

	"github.com/stretchr/testify/suite"

	"github.com/KirkDiggler/rpg-toolkit/examples/world/graph"
	"github.com/KirkDiggler/rpg-toolkit/examples/world/journal"
	"github.com/KirkDiggler/rpg-toolkit/examples/world/quest"
)

const (
	holdID  journal.EntityID = "hold"
	bandID  journal.EntityID = "band"
	scoutID journal.EntityID = "scout"

	belongsTo graph.Relation = "belongs-to"
	hostileTo graph.Relation = "hostile-to"

	broken graph.Flag = "broken"
)

type QuestSuite struct {
	suite.Suite

	world *graph.World
	log   *journal.Journal
	peace quest.Template
}

func (s *QuestSuite) SetupTest() {
	w, err := graph.New(graph.Config{
		Membership: belongsTo,
		Entities: []graph.Entity{
			{ID: holdID, Kind: "faction"},
			{ID: bandID, Kind: "faction"},
			{ID: scoutID, Kind: "person"},
		},
		Edges: []graph.Edge{
			{From: scoutID, Rel: belongsTo, To: bandID},
			{From: holdID, Rel: hostileTo, To: bandID},
		},
		Reducers: []graph.Reducer{graph.Raise{On: "routed", Flag: broken}},
		Projections: []graph.Projection{
			graph.Retire{OnFlag: broken, Relations: []graph.Relation{hostileTo}},
		},
	})
	s.Require().NoError(err)

	s.world = w
	s.log = journal.New()
	s.peace = quest.Template{
		ID:   "quiet-the-hold",
		Name: "Quiet the Hold",
		Objectives: []quest.Objective{{
			ID:        "no-longer-hostile",
			Observer:  holdID,
			Predicate: quest.NoEdge{From: holdID, Rel: hostileTo, To: bandID},
		}},
	}
}

func TestQuestSuite(t *testing.T) {
	suite.Run(t, new(QuestSuite))
}

func (s *QuestSuite) offer() *quest.Instance {
	i, err := quest.Offer("job-1", s.peace)
	s.Require().NoError(err)

	return i
}

func (s *QuestSuite) rout() {
	_, err := s.log.Append(journal.Fact{
		Kind: "routed", Actor: scoutID, Subject: holdID,
		Audience: journal.Audience{holdID, bandID},
	})
	s.Require().NoError(err)
}

func (s *QuestSuite) TestOfferRefusesContentNobodyCouldFail() {
	s.Run("a template needs an id", func() {
		_, err := quest.Offer("job-1", quest.Template{Objectives: s.peace.Objectives})
		s.Require().ErrorIs(err, quest.ErrNoTemplate)
	})

	s.Run("a template with no objectives would close on sight", func() {
		_, err := quest.Offer("job-1", quest.Template{ID: "empty"})
		s.Require().ErrorIs(err, quest.ErrNoObjectives)
	})
}

func (s *QuestSuite) TestLifecycleRunsOfferedToClaimedToCompleted() {
	instance := s.offer()
	s.Equal(quest.StatusOffered, instance.Status())

	s.Run("the first claim moves the status and emits", func() {
		events, err := instance.Claim(bandID)
		s.Require().NoError(err)
		s.Require().Len(events, 1)
		s.Equal(quest.EventQuestClaimed, events[0].Kind)
		s.Equal(quest.StatusClaimed, instance.Status())
	})

	s.Run("a second claimant joins without a second transition", func() {
		events, err := instance.Claim(scoutID)
		s.Require().NoError(err)
		s.Empty(events)
		s.Equal([]journal.EntityID{bandID, scoutID}, instance.Claimants())
	})

	s.Run("an unmet objective completes nothing", func() {
		report := instance.Observe(s.world, s.log)
		s.False(report.Met["no-longer-hostile"])
		s.Empty(report.Events)
		s.Equal(quest.StatusClaimed, report.Status)
	})

	s.Run("the world moving completes it, once", func() {
		s.rout()

		report := instance.Observe(s.world, s.log)
		s.True(report.Met["no-longer-hostile"])
		s.Require().Len(report.Events, 1)
		s.Equal(quest.EventQuestCompleted, report.Events[0].Kind)
		s.Equal([]journal.EntityID{bandID, scoutID}, report.Events[0].Claimants)

		again := instance.Observe(s.world, s.log)
		s.Empty(again.Events)
		s.Equal(quest.StatusCompleted, again.Status)
	})

	s.Run("a closed instance takes no more claims", func() {
		_, err := instance.Claim(scoutID)
		s.Require().ErrorIs(err, quest.ErrClosed)
	})
}

func (s *QuestSuite) TestAbandonedInstancesDoNotCompleteLater() {
	instance := s.offer()
	_, err := instance.Claim(bandID)
	s.Require().NoError(err)

	events, err := instance.Abandon()
	s.Require().NoError(err)
	s.Require().Len(events, 1)
	s.Equal(quest.EventQuestAbandoned, events[0].Kind)

	s.rout()
	report := instance.Observe(s.world, s.log)
	s.True(report.Met["no-longer-hostile"])
	s.Equal(quest.StatusAbandoned, report.Status)
	s.Empty(report.Events)
}

func (s *QuestSuite) TestObservingWritesNothingToTheWorld() {
	instance := s.offer()
	s.rout()

	before := s.log.All()
	stateBefore := s.world.StateFor(holdID, s.log)

	instance.Observe(s.world, s.log)

	s.Equal(before, s.log.All())
	s.Equal(stateBefore, s.world.StateFor(holdID, s.log))
}

func (s *QuestSuite) TestObjectivesAreReadInTheNamedObserversView() {
	// The band never witnessed the rout, so in the band's view the hold is
	// still hostile — while in the hold's own view it is not.
	_, err := s.log.Append(journal.Fact{
		Kind: "routed", Actor: scoutID, Subject: holdID,
		Audience: journal.Audience{holdID},
	})
	s.Require().NoError(err)

	s.Run("read in the hold's view, the objective is met", func() {
		instance := s.offer()
		s.True(instance.Observe(s.world, s.log).Met["no-longer-hostile"])
	})

	s.Run("read in the band's view, it is not", func() {
		template := s.peace
		template.Objectives = []quest.Objective{{
			ID:        "no-longer-hostile",
			Observer:  bandID,
			Predicate: quest.NoEdge{From: holdID, Rel: hostileTo, To: bandID},
		}}
		instance, err := quest.Offer("job-2", template)
		s.Require().NoError(err)
		s.False(instance.Observe(s.world, s.log).Met["no-longer-hostile"])
	})
}

func (s *QuestSuite) TestPredicatesDescribeThemselvesForAQuestLog() {
	s.Equal("hold is not hostile-to band", quest.NoEdge{From: holdID, Rel: hostileTo, To: bandID}.Describe())
	s.Equal("hold is hostile-to band", quest.HasEdge{From: holdID, Rel: hostileTo, To: bandID}.Describe())
	s.Equal("hold is broken", quest.Flagged{Flag: broken, Of: holdID}.Describe())
	s.Equal("scout leads hold", quest.Occupies{Who: scoutID, Role: "leads", Of: holdID}.Describe())
	s.Equal("somebody leads hold", quest.Occupies{Role: "leads", Of: holdID}.Describe())
	s.Equal("nothing in particular", quest.All{}.Describe())
}

func (s *QuestSuite) TestAllRequiresEveryPart() {
	s.rout()
	state := s.world.StateFor(holdID, s.log)

	both := quest.All{
		quest.NoEdge{From: holdID, Rel: hostileTo, To: bandID},
		quest.Flagged{Flag: broken, Of: holdID},
	}
	s.True(both.Holds(state))

	withUnmet := append(both, quest.Occupies{Role: "leads", Of: holdID})
	s.False(withUnmet.Holds(state))
	s.True(quest.All{}.Holds(state))
}
