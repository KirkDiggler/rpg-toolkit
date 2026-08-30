// Copyright (C) 2026 Kirk Diggler
// SPDX-License-Identifier: GPL-3.0-or-later

package graph_test

import (
	"testing"

	"github.com/stretchr/testify/suite"

	"github.com/KirkDiggler/rpg-toolkit/examples/world/graph"
	"github.com/KirkDiggler/rpg-toolkit/examples/world/journal"
)

const (
	holdID    journal.EntityID = "hold"
	chiefID   journal.EntityID = "chief"
	gruntID   journal.EntityID = "grunt"
	watcherID journal.EntityID = "watcher"
	bandID    journal.EntityID = "band"
	scoutID   journal.EntityID = "scout"

	faction graph.Kind = "faction"
	person  graph.Kind = "person"

	belongsTo  graph.Relation = "belongs-to"
	hostileTo  graph.Relation = "hostile-to"
	alliedWith graph.Relation = "allied-with"

	leads graph.Role = "leads"

	regard graph.Counter = "regard"

	alerted graph.Flag = "alerted"
	broken  graph.Flag = "broken"

	posture graph.LabelName = "posture"

	assaultKind  journal.Kind = "assault"
	commandKind  journal.Kind = "claims-command"
	unmaskKind   journal.Kind = "unmasked"
	persuadeKind journal.Kind = "persuasion"
	routedKind   journal.Kind = "routed"
	rumourKind   journal.Kind = "rumour"
)

type GraphSuite struct {
	suite.Suite

	log *journal.Journal
}

func (s *GraphSuite) SetupTest() {
	s.log = journal.New()
}

func TestGraphSuite(t *testing.T) {
	suite.Run(t, new(GraphSuite))
}

// declaration is the shared fixture: a hold under its chief, hostile to a band
// that a scout belongs to, with a watcher inside the hold at individual grain.
func (s *GraphSuite) declaration() graph.Config {
	return graph.Config{
		Membership: belongsTo,
		Entities: []graph.Entity{
			{ID: holdID, Kind: faction},
			{ID: bandID, Kind: faction},
			{ID: chiefID, Kind: person},
			{ID: gruntID, Kind: person},
			{ID: watcherID, Kind: person, Grain: graph.GrainIndividual},
			{ID: scoutID, Kind: person},
		},
		Edges: []graph.Edge{
			{From: chiefID, Rel: belongsTo, To: holdID},
			{From: gruntID, Rel: belongsTo, To: holdID},
			{From: watcherID, Rel: belongsTo, To: holdID},
			{From: scoutID, Rel: belongsTo, To: bandID},
			{From: holdID, Rel: hostileTo, To: bandID},
			{From: bandID, Rel: alliedWith, To: bandID},
		},
		Slots: []graph.Slot{
			{Role: leads, Of: holdID, Occupant: chiefID},
		},
	}
}

func (s *GraphSuite) world(cfg graph.Config) *graph.World {
	w, err := graph.New(cfg)
	s.Require().NoError(err)

	return w
}

func (s *GraphSuite) append(f journal.Fact) {
	_, err := s.log.Append(f)
	s.Require().NoError(err)
}

func (s *GraphSuite) TestNewRefusesDeclarationsItCannotFold() {
	s.Run("membership relation is required, never defaulted", func() {
		cfg := s.declaration()
		cfg.Membership = ""
		_, err := graph.New(cfg)
		s.Require().ErrorIs(err, graph.ErrNoMembership)
	})

	s.Run("an edge may not name an undeclared entity", func() {
		cfg := s.declaration()
		cfg.Edges = append(cfg.Edges, graph.Edge{From: holdID, Rel: hostileTo, To: "ghost"})
		_, err := graph.New(cfg)
		s.Require().ErrorIs(err, graph.ErrUnknownEntity)
	})

	s.Run("a slot may not name an undeclared occupant", func() {
		cfg := s.declaration()
		cfg.Slots = append(cfg.Slots, graph.Slot{Role: "guards", Of: bandID, Occupant: "ghost"})
		_, err := graph.New(cfg)
		s.Require().ErrorIs(err, graph.ErrUnknownEntity)
	})

	s.Run("an entity may not be declared twice", func() {
		cfg := s.declaration()
		cfg.Entities = append(cfg.Entities, graph.Entity{ID: holdID, Kind: faction})
		_, err := graph.New(cfg)
		s.Require().ErrorIs(err, graph.ErrDuplicateEntity)
	})

	s.Run("a role may not be declared twice for one entity", func() {
		cfg := s.declaration()
		cfg.Slots = append(cfg.Slots, graph.Slot{Role: leads, Of: holdID, Occupant: gruntID})
		_, err := graph.New(cfg)
		s.Require().ErrorIs(err, graph.ErrDuplicateSlot)
	})
}

func (s *GraphSuite) TestAudienceReachesMembersThroughTheirGroups() {
	w := s.world(s.declaration())

	s.ElementsMatch([]journal.EntityID{gruntID, holdID}, w.AudienceOf(gruntID))
	s.ElementsMatch([]journal.EntityID{holdID}, w.AudienceOf(holdID))
	s.Empty(w.AudienceOf(""))
}

func (s *GraphSuite) TestRaiseFlagsTheSubjectForWhoeverWitnessedIt() {
	cfg := s.declaration()
	cfg.Reducers = []graph.Reducer{graph.Raise{On: assaultKind, Flag: alerted}}
	cfg.Projections = []graph.Projection{
		graph.Label{Name: posture, Of: faction, WhenFlag: alerted, Then: "formed-up", Else: "surprised"},
	}
	w := s.world(cfg)

	s.append(journal.Fact{Kind: assaultKind, Actor: scoutID, Subject: holdID, Audience: journal.Audience{holdID}})

	s.Run("the hold folds what it saw", func() {
		state := w.StateFor(holdID, s.log)
		s.True(state.Flagged(alerted, holdID))
		s.Equal("formed-up", state.Label(posture, holdID))
	})

	s.Run("a member folds it too, through the group", func() {
		s.True(w.StateFor(gruntID, s.log).Flagged(alerted, holdID))
	})

	s.Run("the band never saw it", func() {
		state := w.StateFor(bandID, s.log)
		s.False(state.Flagged(alerted, holdID))
		s.Equal("surprised", state.Label(posture, holdID))
	})
}

func (s *GraphSuite) TestAnUnwitnessedFactChangesNobodysPresent() {
	cfg := s.declaration()
	cfg.Reducers = []graph.Reducer{graph.Raise{On: assaultKind, Flag: alerted}}
	w := s.world(cfg)

	s.append(journal.Fact{Kind: assaultKind, Actor: scoutID, Subject: holdID})

	s.False(w.StateFor(holdID, s.log).Flagged(alerted, holdID))
	s.False(w.StateFor(gruntID, s.log).Flagged(alerted, holdID))

	// It still happened. Truth is where a game master looks, and nowhere else.
	s.True(w.Truth(s.log).Flagged(alerted, holdID))
}

func (s *GraphSuite) TestFollowSlotMakesAllegianceTrackWhoeverHoldsTheRole() {
	cfg := s.declaration()
	cfg.Reducers = []graph.Reducer{
		graph.Occupy{On: commandKind, When: graph.Succeeded, Role: leads},
		graph.Vacate{On: unmaskKind, Role: leads},
	}
	cfg.Projections = []graph.Projection{
		graph.FollowSlot{Role: leads, Relations: []graph.Relation{hostileTo, alliedWith}},
	}
	w := s.world(cfg)

	s.Run("under its own chief the hold keeps its declared stance", func() {
		state := w.StateFor(holdID, s.log)
		s.Equal(chiefID, state.Occupant(leads, holdID))
		s.True(state.HasEdge(holdID, hostileTo, bandID))
	})

	s.append(journal.Fact{
		Kind: commandKind, Actor: scoutID, Subject: holdID,
		Audience: journal.Audience{holdID, watcherID},
		Outcome:  journal.Outcome{Contested: true, Succeeded: true},
	})

	s.Run("an outsider in the slot lends the hold their faction's stance", func() {
		state := w.StateFor(holdID, s.log)
		s.Equal(scoutID, state.Occupant(leads, holdID))
		s.False(state.HasEdge(holdID, hostileTo, bandID))
		s.True(state.HasEdge(holdID, alliedWith, bandID))
	})

	s.append(journal.Fact{
		Kind: unmaskKind, Actor: watcherID, Subject: scoutID,
		Audience: journal.Audience{watcherID},
	})

	s.Run("vacating the slot puts the declared stance back, for whoever saw it", func() {
		seen := w.StateFor(watcherID, s.log)
		s.Empty(seen.Occupant(leads, holdID))
		s.True(seen.HasEdge(holdID, hostileTo, bandID))

		unseen := w.StateFor(holdID, s.log)
		s.Equal(scoutID, unseen.Occupant(leads, holdID))
		s.True(unseen.HasEdge(holdID, alliedWith, bandID))
	})
}

func (s *GraphSuite) TestCountPointsAtTheActorsFactionAndThresholdConverts() {
	cfg := s.declaration()
	cfg.Reducers = []graph.Reducer{
		graph.Count{On: persuadeKind, When: graph.Succeeded, Into: regard, By: 1},
		graph.Count{On: persuadeKind, When: graph.Failed, Into: regard, By: -1},
	}
	cfg.Projections = []graph.Projection{
		graph.Threshold{Counter: regard, At: 2, From: hostileTo, To: alliedWith},
	}
	w := s.world(cfg)

	persuade := func(ok bool) {
		s.append(journal.Fact{
			Kind: persuadeKind, Actor: scoutID, Subject: holdID,
			Audience: journal.Audience{holdID},
			Outcome:  journal.Outcome{Contested: true, Succeeded: ok},
		})
	}

	s.Run("regard accrues to the band, not to the scout personally", func() {
		persuade(true)
		state := w.StateFor(holdID, s.log)
		s.Equal(1, state.Count(regard, holdID, bandID))
		s.Zero(state.Count(regard, holdID, scoutID))
	})

	s.Run("below the threshold nothing has changed", func() {
		state := w.StateFor(holdID, s.log)
		s.True(state.HasEdge(holdID, hostileTo, bandID))
		s.False(state.HasEdge(holdID, alliedWith, bandID))
	})

	s.Run("a botched parley moves it back", func() {
		persuade(false)
		s.Zero(w.StateFor(holdID, s.log).Count(regard, holdID, bandID))
	})

	s.Run("crossing the threshold converts hostility to alliance", func() {
		persuade(true)
		persuade(true)
		state := w.StateFor(holdID, s.log)
		s.Equal(2, state.Count(regard, holdID, bandID))
		s.False(state.HasEdge(holdID, hostileTo, bandID))
		s.True(state.HasEdge(holdID, alliedWith, bandID))
	})
}

func (s *GraphSuite) TestRetireStripsTheEdgesOfAFlaggedEntity() {
	cfg := s.declaration()
	cfg.Reducers = []graph.Reducer{graph.Raise{On: routedKind, Flag: broken}}
	cfg.Projections = []graph.Projection{
		graph.Retire{OnFlag: broken, Relations: []graph.Relation{hostileTo}},
	}
	w := s.world(cfg)

	s.append(journal.Fact{
		Kind: routedKind, Actor: scoutID, Subject: holdID,
		Audience: journal.Audience{holdID, bandID},
	})

	state := w.StateFor(holdID, s.log)
	s.True(state.Flagged(broken, holdID))
	s.False(state.HasEdge(holdID, hostileTo, bandID))
	// Membership was not in Relations, so the hold is still a hold.
	s.True(state.HasEdge(gruntID, belongsTo, holdID))
}

func (s *GraphSuite) TestAFoldThatCannotApplyARuleSaysSoOutLoud() {
	cfg := s.declaration()
	cfg.Reducers = []graph.Reducer{
		graph.Occupy{On: commandKind, Role: "quartermasters"},
		graph.Raise{On: rumourKind, Flag: alerted},
	}
	w := s.world(cfg)

	s.append(journal.Fact{
		Kind: commandKind, Actor: scoutID, Subject: holdID,
		Audience: journal.Audience{holdID},
	})
	s.append(journal.Fact{Kind: rumourKind, Actor: scoutID, Audience: journal.Audience{holdID}})

	refusals := w.StateFor(holdID, s.log).Refusals()
	s.Require().Len(refusals, 2)
	s.Contains(refusals[0], "quartermasters")
	s.Contains(refusals[1], "no subject")
}

func (s *GraphSuite) TestPresentStateIsAPureFunctionOfDeclarationAndFacts() {
	cfg := s.declaration()
	cfg.Reducers = []graph.Reducer{graph.Raise{On: assaultKind, Flag: alerted}}
	cfg.Projections = []graph.Projection{
		graph.Label{Name: posture, Of: faction, WhenFlag: alerted, Then: "formed-up", Else: "surprised"},
	}
	w := s.world(cfg)

	before := w.StateFor(holdID, s.log)

	s.append(journal.Fact{Kind: assaultKind, Actor: scoutID, Subject: holdID, Audience: journal.Audience{holdID}})
	after := w.StateFor(holdID, s.log)

	s.Run("two folds over the same facts agree", func() {
		s.Equal(after, w.StateFor(holdID, s.log))
	})

	s.Run("the earlier state was not mutated by the later fold", func() {
		s.False(before.Flagged(alerted, holdID))
		s.Equal("surprised", before.Label(posture, holdID))
		s.True(after.Flagged(alerted, holdID))
	})

	s.Run("rolling the journal back rolls the present back with it", func() {
		rewound := journal.New()
		s.Equal(before, w.StateFor(holdID, rewound))
	})

	s.Run("a handed-out edge list cannot reach the state", func() {
		edges := after.Edges()
		edges[0] = graph.Edge{From: "nonsense", Rel: hostileTo, To: "nonsense"}
		s.True(after.HasEdge(chiefID, belongsTo, holdID))
	})
}
