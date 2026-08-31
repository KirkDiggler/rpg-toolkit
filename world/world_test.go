// Copyright (C) 2026 Kirk Diggler
// SPDX-License-Identifier: GPL-3.0-or-later

package world_test

import (
	"context"
	"errors"
	"testing"

	"github.com/stretchr/testify/suite"

	"github.com/KirkDiggler/rpg-toolkit/world"
	"github.com/KirkDiggler/rpg-toolkit/world/graph"
	"github.com/KirkDiggler/rpg-toolkit/world/journal"
	"github.com/KirkDiggler/rpg-toolkit/world/quest"
)

const (
	holdID  journal.EntityID = "hold"
	bandID  journal.EntityID = "band"
	scoutID journal.EntityID = "scout"
	watchID journal.EntityID = "watcher"

	belongsTo graph.Relation = "belongs-to"
	hostileTo graph.Relation = "hostile-to"

	routed graph.Flag = "routed"

	shoutKind journal.Kind = "shout"
	greatKind journal.Kind = "great-blow"
	fairKind  journal.Kind = "fair-blow"
	missKind  journal.Kind = "miss"

	strike world.VerbName = "strike"
	shout  world.VerbName = "shout"

	viaMight journal.Approach = "might"
)

// fixedResolver is an ordinary test double. The composer's own tests use one
// rather than a rulebook: what a margin means is exactly what this package
// must not know.
type fixedResolver struct {
	margin int
	err    error
}

func (r fixedResolver) Resolve(context.Context, world.Attempt) (journal.Outcome, error) {
	if r.err != nil {
		return journal.Outcome{}, r.err
	}

	return journal.Outcome{Contested: true, Succeeded: r.margin >= 0, Margin: r.margin}, nil
}

type WorldSuite struct {
	suite.Suite

	ctx context.Context
}

func (s *WorldSuite) SetupTest() {
	s.ctx = context.Background()
}

func TestWorldSuite(t *testing.T) {
	suite.Run(t, new(WorldSuite))
}

func (s *WorldSuite) structure() graph.Config {
	return graph.Config{
		Membership: belongsTo,
		Entities: []graph.Entity{
			{ID: holdID, Kind: "faction"},
			{ID: bandID, Kind: "faction"},
			{ID: scoutID, Kind: "person"},
			{ID: watchID, Kind: "person", Grain: graph.GrainIndividual},
		},
		Edges: []graph.Edge{
			{From: scoutID, Rel: belongsTo, To: bandID},
			{From: watchID, Rel: belongsTo, To: holdID},
			{From: holdID, Rel: hostileTo, To: bandID},
		},
		Reducers: []graph.Reducer{graph.Raise{On: greatKind, Flag: routed}},
		Projections: []graph.Projection{
			graph.Retire{OnFlag: routed, Relations: []graph.Relation{hostileTo}},
		},
	}
}

// graded is a verb with three results, so a margin has somewhere to land.
func (s *WorldSuite) graded() world.Verb {
	return world.Verb{
		Name:       strike,
		Approach:   viaMight,
		Difficulty: 10,
		Outcomes: []world.Band{
			{AtLeast: 5, Emission: world.Emission{Kind: greatKind, Witness: world.WitnessTarget}},
			{AtLeast: 0, Emission: world.Emission{Kind: fairKind, Witness: world.WitnessTarget}},
		},
		Otherwise: world.Emission{Kind: missKind, Witness: world.WitnessTarget},
	}
}

func (s *WorldSuite) job() quest.Template {
	return quest.Template{
		ID:       "quiet-the-hold",
		Name:     "Quiet the Hold",
		Subjects: []journal.EntityID{holdID},
		Objectives: []quest.Objective{{
			ID:        "no-longer-hostile",
			Observer:  quest.InstanceSubject,
			Predicate: quest.NoEdge{From: quest.InstanceSubject, Rel: hostileTo, To: bandID},
		}},
	}
}

func (s *WorldSuite) build(margin int, verbs ...world.Verb) *world.World {
	s.T().Helper()

	built, err := world.New(world.Config{
		Scenario: world.Scenario{
			Graph:  s.structure(),
			Verbs:  verbs,
			Quests: []quest.Template{s.job()},
		},
		Resolver: fixedResolver{margin: margin},
	})
	s.Require().NoError(err)

	return built
}

func (s *WorldSuite) TestNewRefusesWiringItCannotRun() {
	s.Run("a world with no resolver is told what a resolver is for", func() {
		_, err := world.New(world.Config{
			Scenario: world.Scenario{Graph: s.structure(), Verbs: []world.Verb{s.graded()}},
		})
		s.Require().ErrorIs(err, world.ErrNoResolver)
		s.Contains(err.Error(), "the rules of your game plug in")
	})

	s.Run("a world with no verbs is a world nobody can do anything in", func() {
		_, err := world.New(world.Config{
			Scenario: world.Scenario{Graph: s.structure()},
			Resolver: fixedResolver{},
		})
		s.Require().ErrorIs(err, world.ErrNoVerbs)
	})

	s.Run("a broken graph declaration surfaces from underneath", func() {
		broken := s.structure()
		broken.Membership = ""
		_, err := world.New(world.Config{
			Scenario: world.Scenario{Graph: broken, Verbs: []world.Verb{s.graded()}},
			Resolver: fixedResolver{},
		})
		s.Require().ErrorIs(err, graph.ErrNoMembership)
	})

	s.Run("so does a broken job", func() {
		bad := s.job()
		bad.Subjects = nil
		_, err := world.New(world.Config{
			Scenario: world.Scenario{
				Graph: s.structure(), Verbs: []world.Verb{s.graded()},
				Quests: []quest.Template{bad},
			},
			Resolver: fixedResolver{},
		})
		s.Require().ErrorIs(err, quest.ErrNoSubjects)
	})
}

func (s *WorldSuite) TestNewRefusesVerbsAnAuthorHasNotFinished() {
	cases := []struct {
		name string
		verb world.Verb
		want error
	}{
		{
			name: "no name",
			verb: world.Verb{Otherwise: world.Emission{Kind: missKind}},
			want: world.ErrNoVerbName,
		},
		{
			name: "nothing to write when it goes badly",
			verb: world.Verb{Name: strike, Approach: viaMight},
			want: world.ErrNoOtherwise,
		},
		{
			name: "graded but never rolled for",
			verb: world.Verb{
				Name:      shout,
				Outcomes:  []world.Band{{Emission: world.Emission{Kind: greatKind}}},
				Otherwise: world.Emission{Kind: missKind},
			},
			want: world.ErrGradedButUncontested,
		},
		{
			name: "an outcome that writes nothing",
			verb: world.Verb{
				Name: strike, Approach: viaMight,
				Outcomes:  []world.Band{{AtLeast: 5}},
				Otherwise: world.Emission{Kind: missKind},
			},
			want: world.ErrEmptyBand,
		},
		{
			name: "outcomes that are not best-first",
			verb: world.Verb{
				Name: strike, Approach: viaMight,
				Outcomes: []world.Band{
					{AtLeast: 0, Emission: world.Emission{Kind: fairKind}},
					{AtLeast: 5, Emission: world.Emission{Kind: greatKind}},
				},
				Otherwise: world.Emission{Kind: missKind},
			},
			want: world.ErrBandsOutOfOrder,
		},
	}

	for _, tc := range cases {
		s.Run(tc.name, func() {
			_, err := world.New(world.Config{
				Scenario: world.Scenario{Graph: s.structure(), Verbs: []world.Verb{tc.verb}},
				Resolver: fixedResolver{},
			})
			s.Require().ErrorIs(err, tc.want)
		})
	}

	s.Run("the same verb declared twice", func() {
		_, err := world.New(world.Config{
			Scenario: world.Scenario{
				Graph: s.structure(), Verbs: []world.Verb{s.graded(), s.graded()},
			},
			Resolver: fixedResolver{},
		})
		s.Require().ErrorIs(err, world.ErrDuplicateVerb)
	})
}

func (s *WorldSuite) TestTheMarginPicksTheOutcome() {
	cases := []struct {
		margin int
		want   journal.Kind
	}{
		{margin: 9, want: greatKind},
		{margin: 5, want: greatKind},
		{margin: 4, want: fairKind},
		{margin: 0, want: fairKind},
		{margin: -1, want: missKind},
	}

	for _, tc := range cases {
		w := s.build(tc.margin, s.graded())
		result, err := w.Act(s.ctx, world.Act{Verb: strike, Actor: scoutID, Target: holdID})
		s.Require().NoError(err)
		s.Equalf(tc.want, result.Fact.Kind, "margin %d", tc.margin)
	}
}

func (s *WorldSuite) TestAnUncontestedVerbIsNeverRolledFor() {
	verb := world.Verb{Name: shout, Otherwise: world.Emission{Kind: shoutKind, Witness: world.WitnessTarget}}
	w := s.build(0, verb)

	result, err := w.Act(s.ctx, world.Act{Verb: shout, Actor: scoutID, Target: holdID})
	s.Require().NoError(err)

	s.Equal(shoutKind, result.Fact.Kind)
	s.False(result.Fact.Outcome.Contested)
	s.Contains(result.Fact.Outcome.Detail, "uncontested")
}

func (s *WorldSuite) TestActRefusesWhatItCannotRun() {
	s.Run("a verb nobody declared", func() {
		w := s.build(0, s.graded())
		_, err := w.Act(s.ctx, world.Act{Verb: "sing", Actor: scoutID, Target: holdID})
		s.Require().ErrorIs(err, world.ErrUnknownVerb)
	})

	s.Run("a resolver that could not judge leaves the journal alone", func() {
		boom := errors.New("no sheet for that actor")
		built, err := world.New(world.Config{
			Scenario: world.Scenario{
				Graph: s.structure(), Verbs: []world.Verb{s.graded()},
				Quests: []quest.Template{s.job()},
			},
			Resolver: fixedResolver{err: boom},
		})
		s.Require().NoError(err)

		_, err = built.Act(s.ctx, world.Act{Verb: strike, Actor: scoutID, Target: holdID})
		s.Require().ErrorIs(err, boom)
		s.Zero(built.Journal().Len())
	})
}

func (s *WorldSuite) TestAnActorAlwaysWitnessesTheirOwnAct() {
	verb := s.graded()
	verb.Outcomes[0].Emission.Witness = world.WitnessNobody
	w := s.build(9, verb)

	result, err := w.Act(s.ctx, world.Act{
		Verb: strike, Actor: scoutID, Target: holdID,
		Bystanders: []journal.EntityID{watchID},
	})
	s.Require().NoError(err)

	s.Equal(journal.Audience{scoutID}, result.Fact.Audience)
}

func (s *WorldSuite) TestBystandersAndTargetCanBothBeToldAtOnce() {
	verb := s.graded()
	verb.Outcomes[0].Emission.Witness = world.WitnessTargetAndBystanders
	w := s.build(9, verb)

	result, err := w.Act(s.ctx, world.Act{
		Verb: strike, Actor: scoutID, Target: holdID,
		Bystanders: []journal.EntityID{watchID, holdID},
	})
	s.Require().NoError(err)

	s.Equal(journal.Audience{scoutID, holdID, watchID}, result.Fact.Audience,
		"the target listed twice is still one witness")
}

func (s *WorldSuite) TestActIsTheOnlyDoorAndTheJobsLookThroughIt() {
	w := s.build(9, s.graded())

	instance, events, err := w.Claim("quiet-the-hold", bandID)
	s.Require().NoError(err)
	s.Require().Len(events, 1)
	s.Equal(quest.EventQuestClaimed, events[0].Kind)

	s.Run("before anybody acts, nothing has happened", func() {
		s.Zero(w.Journal().Len())
		s.True(w.View(holdID).HasEdge(holdID, hostileTo, bandID))
		s.Equal(quest.StatusClaimed, instance.Status())
	})

	s.Run("one act writes the fact, moves the fold, and closes the job", func() {
		result, err := w.Act(s.ctx, world.Act{Verb: strike, Actor: scoutID, Target: holdID})
		s.Require().NoError(err)

		s.Equal(1, w.Journal().Len())
		s.Equal(greatKind, result.Fact.Kind)
		s.False(w.View(holdID).HasEdge(holdID, hostileTo, bandID))
		s.Require().Len(result.Quests.Events, 1)
		s.Equal(quest.EventQuestCompleted, result.Quests.Events[0].Kind)
		s.Equal(quest.StatusCompleted, instance.Status())
	})

	s.Run("reading twice does not write anything", func() {
		before := w.Journal().All()
		w.Observe()
		s.Equal(before, w.Journal().All())
	})
}
