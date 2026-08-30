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
	holdID   journal.EntityID = "hold"
	bandID   journal.EntityID = "band"
	scoutID  journal.EntityID = "scout"
	firstID  journal.EntityID = "first-captive"
	secondID journal.EntityID = "second-captive"
	thirdID  journal.EntityID = "third-captive"

	belongsTo graph.Relation = "belongs-to"
	hostileTo graph.Relation = "hostile-to"

	broken graph.Flag = "broken"
	freed  graph.Flag = "freed"
	lost   graph.Flag = "lost"

	leads graph.Role = "leads"

	routedKind journal.Kind = "routed"
	freeKind   journal.Kind = "freeing"
	lostKind   journal.Kind = "losing"

	peaceJob  = "quiet-the-hold"
	rescueJob = "free-the-captives"
)

type QuestSuite struct {
	suite.Suite

	world *graph.World
	log   *journal.Journal
}

func (s *QuestSuite) SetupTest() {
	w, err := graph.New(graph.Config{
		Membership: belongsTo,
		Entities: []graph.Entity{
			{ID: holdID, Kind: "faction"},
			{ID: bandID, Kind: "faction"},
			{ID: scoutID, Kind: "person"},
			{ID: firstID, Kind: "person"},
			{ID: secondID, Kind: "person"},
			{ID: thirdID, Kind: "person"},
		},
		Edges: []graph.Edge{
			{From: scoutID, Rel: belongsTo, To: bandID},
			{From: holdID, Rel: hostileTo, To: bandID},
		},
		Reducers: []graph.Reducer{
			graph.Raise{On: routedKind, Flag: broken},
			graph.Raise{On: freeKind, Flag: freed},
			graph.Raise{On: lostKind, Flag: lost},
		},
		Projections: []graph.Projection{
			graph.Retire{OnFlag: broken, Relations: []graph.Relation{hostileTo}},
		},
	})
	s.Require().NoError(err)

	s.world = w
	s.log = journal.New()
}

func TestQuestSuite(t *testing.T) {
	suite.Run(t, new(QuestSuite))
}

// peace is the one-subject shape UC-1 uses: a job about a place.
func (s *QuestSuite) peace() quest.Template {
	return quest.Template{
		ID:       peaceJob,
		Name:     "Quiet the Hold",
		Subjects: []journal.EntityID{holdID},
		Objectives: []quest.Objective{{
			ID:        "no-longer-hostile",
			Observer:  quest.InstanceSubject,
			Predicate: quest.NoEdge{From: quest.InstanceSubject, Rel: hostileTo, To: bandID},
		}},
	}
}

// rescue is the population shape: one job, three captives, one each.
func (s *QuestSuite) rescue() quest.Template {
	return quest.Template{
		ID:       rescueJob,
		Name:     "Free the Captives",
		Subjects: []journal.EntityID{firstID, secondID, thirdID},
		Objectives: []quest.Objective{{
			ID:        "freed",
			Predicate: quest.Flagged{Flag: freed, Of: quest.InstanceSubject},
		}},
		Failure: &quest.Objective{
			ID:        "lost",
			Predicate: quest.Flagged{Flag: lost, Of: quest.InstanceSubject},
		},
		Buckets: []quest.Bucket{
			{Name: "freed", Predicate: quest.Flagged{Flag: freed, Of: quest.InstanceSubject}},
			{Name: "lost", Predicate: quest.Flagged{Flag: lost, Of: quest.InstanceSubject}},
			{Name: "held", Predicate: quest.Anything{}},
		},
	}
}

func (s *QuestSuite) board(t quest.Template) *quest.Board {
	s.T().Helper()

	b, err := quest.NewBoard(t)
	s.Require().NoError(err)

	return b
}

func (s *QuestSuite) append(kind journal.Kind, subject journal.EntityID) {
	s.T().Helper()

	_, err := s.log.Append(journal.Fact{
		Kind: kind, Actor: scoutID, Subject: subject,
		Audience: journal.Audience{holdID, bandID, subject},
	})
	s.Require().NoError(err)
}

func (s *QuestSuite) TestNewBoardRefusesContentAnAuthorHasNotFinished() {
	s.Run("a job needs an id", func() {
		t := s.peace()
		t.ID = ""
		_, err := quest.NewBoard(t)
		s.Require().ErrorIs(err, quest.ErrNoTemplateID)
	})

	s.Run("a job needs somebody to be about", func() {
		t := s.peace()
		t.Subjects = nil
		_, err := quest.NewBoard(t)
		s.Require().ErrorIs(err, quest.ErrNoSubjects)
		s.Contains(err.Error(), "one name per copy")
	})

	s.Run("a job with no objectives would close on sight", func() {
		t := s.peace()
		t.Objectives = nil
		_, err := quest.NewBoard(t)
		s.Require().ErrorIs(err, quest.ErrNoObjectives)
	})

	s.Run("a subject may not be listed twice", func() {
		t := s.rescue()
		t.Subjects = []journal.EntityID{firstID, secondID, firstID}
		_, err := quest.NewBoard(t)
		s.Require().ErrorIs(err, quest.ErrDuplicateSubject)
	})

	s.Run("a follow-up needs buckets to count toward", func() {
		t := s.rescue()
		t.Buckets = nil
		t.Successors = []quest.Successor{{Opens: s.peace(), When: quest.NoneIn{Bucket: "held"}}}
		_, err := quest.NewBoard(t)
		s.Require().ErrorIs(err, quest.ErrNoBuckets)
	})

	s.Run("a follow-up may not name its own subjects", func() {
		t := s.rescue()
		t.Successors = []quest.Successor{{
			Opens: s.peace(), When: quest.NoneIn{Bucket: "held"}, SubjectsFrom: "lost",
		}}
		_, err := quest.NewBoard(t)
		s.Require().ErrorIs(err, quest.ErrSuccessorHasSubjects)
	})

	s.Run("a follow-up may not target a bucket that does not exist", func() {
		t := s.rescue()
		opens := s.peace()
		opens.Subjects = nil
		t.Successors = []quest.Successor{{
			Opens: opens, When: quest.NoneIn{Bucket: "held"}, SubjectsFrom: "vanished",
		}}
		_, err := quest.NewBoard(t)
		s.Require().ErrorIs(err, quest.ErrUnknownBucket)
	})
}

func (s *QuestSuite) TestAClaimTakesOneSubjectAndNothingPutsItBack() {
	board := s.board(s.rescue())
	s.Equal(3, board.Available())

	first, events, err := board.Claim("party-a")
	s.Require().NoError(err)
	s.Require().Len(events, 1)
	s.Equal(quest.EventQuestClaimed, events[0].Kind)

	second, _, err := board.Claim("party-b")
	s.Require().NoError(err)
	third, _, err := board.Claim("party-c")
	s.Require().NoError(err)

	s.Run("each claimant got a different captive", func() {
		s.Equal(firstID, first.Subject())
		s.Equal(secondID, second.Subject())
		s.Equal(thirdID, third.Subject())
		s.Equal(journal.EntityID("party-a"), first.Claimant())
		s.Zero(board.Available())
	})

	s.Run("a fourth claimant is told the job is taken, in words they can act on", func() {
		_, _, err := board.Claim("party-d")
		s.Require().ErrorIs(err, quest.ErrBoardExhausted)
		s.Contains(err.Error(), "add more names")
	})

	s.Run("finishing and failing put nobody back on the board", func() {
		s.append(freeKind, firstID)
		s.append(lostKind, secondID)
		board.Observe(s.world, s.log)

		s.Equal(quest.StatusCompleted, first.Status())
		s.Equal(quest.StatusFailed, second.Status())
		s.Zero(board.Available())
	})
}

func (s *QuestSuite) TestInstancesAreIsolatedByTheirOwnSubject() {
	board := s.board(s.rescue())
	first, _, err := board.Claim("party-a")
	s.Require().NoError(err)
	second, _, err := board.Claim("party-b")
	s.Require().NoError(err)

	s.append(lostKind, firstID)
	board.Observe(s.world, s.log)

	s.Equal(quest.StatusFailed, first.Status())
	s.Equal(quest.StatusClaimed, second.Status())
}

func (s *QuestSuite) TestFailureIsWeighedBeforeSuccess() {
	board := s.board(s.rescue())
	instance, _, err := board.Claim("party-a")
	s.Require().NoError(err)

	s.append(freeKind, firstID)
	s.append(lostKind, firstID)
	board.Observe(s.world, s.log)

	// Both hold. The one that cannot be taken back is the one that counts.
	s.Equal(quest.StatusFailed, instance.Status())
}

func (s *QuestSuite) TestTallyCountsThePopulationNotTheClaims() {
	board := s.board(s.rescue())

	s.Run("nobody has claimed anything and the census is already three", func() {
		tally := board.Tally(s.world, s.log)
		s.Equal(3, tally.Total())
		s.Equal(3, tally.Count("held"))
	})

	s.Run("the world moving moves the census, claims or no claims", func() {
		s.append(freeKind, firstID)
		s.append(lostKind, secondID)

		tally := board.Tally(s.world, s.log)
		s.Equal(3, tally.Total())
		s.Equal(1, tally.Count("freed"))
		s.Equal(1, tally.Count("lost"))
		s.Equal(1, tally.Count("held"))
		s.Equal([]string{"freed", "held", "lost"}, tally.Buckets())
	})
}

func (s *QuestSuite) TestBucketsAreAPriorityListNotAPartition() {
	// Flags only go up, so a captive who was lost and then freed still carries
	// "lost". Whichever bucket is asked first is the answer.
	s.append(lostKind, firstID)
	s.append(freeKind, firstID)

	s.Run("freed asked first wins", func() {
		s.Equal(1, s.board(s.rescue()).Tally(s.world, s.log).Count("freed"))
	})

	s.Run("lost asked first wins instead", func() {
		t := s.rescue()
		t.Buckets = []quest.Bucket{
			{Name: "lost", Predicate: quest.Flagged{Flag: lost, Of: quest.InstanceSubject}},
			{Name: "freed", Predicate: quest.Flagged{Flag: freed, Of: quest.InstanceSubject}},
			{Name: "held", Predicate: quest.Anything{}},
		}
		s.Equal(1, s.board(t).Tally(s.world, s.log).Count("lost"))
	})
}

func (s *QuestSuite) TestDistributionsAskAboutTheWholePopulation() {
	board := s.board(s.rescue())
	s.append(lostKind, firstID)
	s.append(lostKind, secondID)

	partway := board.Tally(s.world, s.log)
	s.False(quest.AllIn{Bucket: "lost"}.Holds(partway))
	s.False(quest.NoneIn{Bucket: "held"}.Holds(partway))
	s.True(quest.AtLeastIn{Bucket: "lost", Count: 2}.Holds(partway))

	s.append(lostKind, thirdID)
	settled := board.Tally(s.world, s.log)
	s.True(quest.AllIn{Bucket: "lost"}.Holds(settled))
	s.True(quest.NoneIn{Bucket: "held"}.Holds(settled))
	s.True(quest.Every{
		quest.NoneIn{Bucket: "held"},
		quest.AllIn{Bucket: "lost"},
	}.Holds(settled))
	s.True(quest.Every{}.Holds(settled))
}

func (s *QuestSuite) TestAllInIsFalseForAnEmptyPopulation() {
	// "All of nothing" is true about arithmetic and false about the world. A
	// follow-up that opened because a population was empty would be nonsense.
	empty := quest.Tally{}
	s.False(quest.AllIn{Bucket: "lost"}.Holds(empty))
	s.True(quest.NoneIn{Bucket: "lost"}.Holds(empty))
}

func (s *QuestSuite) TestAbandonedInstancesDoNotCompleteLater() {
	board := s.board(s.rescue())
	instance, _, err := board.Claim("party-a")
	s.Require().NoError(err)

	events, err := instance.Abandon()
	s.Require().NoError(err)
	s.Require().Len(events, 1)
	s.Equal(quest.EventQuestAbandoned, events[0].Kind)

	s.append(freeKind, firstID)
	report := instance.Observe(s.world, s.log)
	s.True(report.Met["freed"])
	s.Equal(quest.StatusAbandoned, report.Status)
	s.Empty(report.Events)

	_, err = instance.Abandon()
	s.Require().ErrorIs(err, quest.ErrClosed)
}

func (s *QuestSuite) TestObservingWritesNothingToTheWorld() {
	board := s.board(s.peace())
	_, _, err := board.Claim("party-a")
	s.Require().NoError(err)
	s.append(routedKind, holdID)

	before := s.log.All()
	stateBefore := s.world.StateFor(holdID, s.log)

	board.Observe(s.world, s.log)

	s.Equal(before, s.log.All())
	s.Equal(stateBefore, s.world.StateFor(holdID, s.log))
}

func (s *QuestSuite) TestObjectivesAreReadInTheNamedObserversView() {
	// The band never witnessed the rout, so in the band's view the hold is
	// still hostile — while in the hold's own view it is not.
	_, err := s.log.Append(journal.Fact{
		Kind: routedKind, Actor: scoutID, Subject: holdID,
		Audience: journal.Audience{holdID},
	})
	s.Require().NoError(err)

	s.Run("read in the hold's own view, the objective is met", func() {
		instance, _, err := s.board(s.peace()).Claim("party-a")
		s.Require().NoError(err)
		s.True(instance.Observe(s.world, s.log).Met["no-longer-hostile"])
	})

	s.Run("read in the band's view, it is not", func() {
		t := s.peace()
		t.Objectives[0].Observer = bandID
		instance, _, err := s.board(t).Claim("party-a")
		s.Require().NoError(err)
		s.False(instance.Observe(s.world, s.log).Met["no-longer-hostile"])
	})
}

func (s *QuestSuite) TestPredicatesDescribeThemselvesForAQuestLog() {
	s.Equal("hold is not hostile-to band", quest.NoEdge{From: holdID, Rel: hostileTo, To: bandID}.Describe())
	s.Equal("hold is hostile-to band", quest.HasEdge{From: holdID, Rel: hostileTo, To: bandID}.Describe())
	s.Equal("hold is broken", quest.Flagged{Flag: broken, Of: holdID}.Describe())
	s.Equal("scout leads hold", quest.Occupies{Who: scoutID, Role: leads, Of: holdID}.Describe())
	s.Equal("somebody leads hold", quest.Occupies{Role: leads, Of: holdID}.Describe())
	s.Equal("nothing in particular", quest.All{}.Describe())
	s.Equal("anything", quest.Anything{}.Describe())
	s.Equal("none are held", quest.NoneIn{Bucket: "held"}.Describe())
	s.Equal("all are lost", quest.AllIn{Bucket: "lost"}.Describe())
	s.Equal("at least 2 are lost", quest.AtLeastIn{Bucket: "lost", Count: 2}.Describe())
	s.Equal("none are held and all are lost", quest.Every{
		quest.NoneIn{Bucket: "held"}, quest.AllIn{Bucket: "lost"},
	}.Describe())
}

func (s *QuestSuite) TestAllRequiresEveryPart() {
	s.append(routedKind, holdID)
	bindings := quest.Bindings{State: s.world.StateFor(holdID, s.log), Subject: holdID}

	both := quest.All{
		quest.NoEdge{From: holdID, Rel: hostileTo, To: bandID},
		quest.Flagged{Flag: broken, Of: holdID},
	}
	s.True(both.Holds(bindings))

	withUnmet := quest.All{both[0], both[1], quest.Occupies{Role: leads, Of: holdID}}
	s.False(withUnmet.Holds(bindings))
	s.True(quest.All{}.Holds(bindings))
}

func (s *QuestSuite) TestInstanceSubjectSubstitutesTheClaimantsOwnSubject() {
	bindings := quest.Bindings{State: s.world.StateFor(holdID, s.log), Subject: firstID}
	s.Equal(firstID, bindings.Resolve(quest.InstanceSubject))
	s.Equal(holdID, bindings.Resolve(holdID))
}
