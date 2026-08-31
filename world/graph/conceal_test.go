// Copyright (C) 2026 Kirk Diggler
// SPDX-License-Identifier: GPL-3.0-or-later

package graph_test

import (
	"github.com/KirkDiggler/rpg-toolkit/world/graph"
	"github.com/KirkDiggler/rpg-toolkit/world/journal"
)

// The concealed fixture: a cache hidden in the hold, behind a passage
// nobody knows about until they find it or somebody opens it in front of
// them.
const (
	secretID journal.EntityID = "secret-cache"

	leadsTo graph.Relation = "leads-to"

	foundKind  journal.Kind = "found"
	openedKind journal.Kind = "opened"
)

// concealed extends the shared fixture with one concealed entity and the one
// concealed edge that leads to it — a found fact pierces both for its own
// audience, and an opened fact reveals both for good.
func (s *GraphSuite) concealed() graph.Config {
	cfg := s.declaration()
	cfg.Entities = append(cfg.Entities, graph.Entity{ID: secretID, Kind: person, Concealed: true})
	cfg.Edges = append(cfg.Edges, graph.Edge{From: holdID, Rel: leadsTo, To: secretID, Concealed: true})
	cfg.Pierces = []graph.Pierce{{
		On:       foundKind,
		Entities: []journal.EntityID{secretID},
		Edges:    []graph.Edge{{From: holdID, Rel: leadsTo, To: secretID}},
	}}
	cfg.Reveals = []graph.Reveal{{
		On:       openedKind,
		Entities: []journal.EntityID{secretID},
		Edges:    []graph.Edge{{From: holdID, Rel: leadsTo, To: secretID}},
	}}

	return cfg
}

func (s *GraphSuite) TestConcealedStructureIsHiddenFromAStrangerAndVisibleInTruth() {
	w := s.world(s.concealed())

	stranger := w.StateFor(scoutID, s.log)
	s.False(stranger.Visible(secretID), "the entity is hidden")
	s.False(stranger.HasEdge(holdID, leadsTo, secretID), "so is the edge that leads to it")

	truth := w.Truth(s.log)
	s.True(truth.Visible(secretID), "the game master sees everything")
	s.True(truth.HasEdge(holdID, leadsTo, secretID))
}

func (s *GraphSuite) TestPiercingHoldsForTheFinderAloneNotTheirFactionmates() {
	cfg := s.concealed()
	w := s.world(cfg)

	s.append(journal.Fact{
		Kind: foundKind, Actor: gruntID, Subject: secretID,
		Audience: journal.Audience{gruntID}, // the finder alone, same as UC-4's search
	})

	s.Run("the finder sees both the entity and the edge", func() {
		found := w.StateFor(gruntID, s.log)
		s.True(found.Visible(secretID))
		s.True(found.HasEdge(holdID, leadsTo, secretID))
	})

	s.Run("a faction-mate who was not in the audience sees neither", func() {
		// chiefID belongs to the same hold as gruntID, proving this is
		// audience discipline and not group-grain leaking the pierce upward.
		chief := w.StateFor(chiefID, s.log)
		s.False(chief.Visible(secretID))
		s.False(chief.HasEdge(holdID, leadsTo, secretID))
	})

	s.Run("truth was never concealed in the first place", func() {
		truth := w.Truth(s.log)
		s.True(truth.Visible(secretID))
		s.True(truth.HasEdge(holdID, leadsTo, secretID))
	})
}

func (s *GraphSuite) TestARevealHoldsForAnObserverWithNoWitnessedFactsAtAll() {
	w := s.world(s.concealed())

	s.append(journal.Fact{
		Kind: openedKind, Actor: gruntID, Subject: secretID,
		Audience: journal.Audience{gruntID}, // narrow audience on purpose
	})

	// bandID witnessed nothing about the cache — not the opening fact, not
	// any prior found fact — and is not even in the hold. Perceiving present
	// state is not witnessing past events, so a late arrival still sees it.
	late := w.StateFor(bandID, s.log)
	s.True(late.Visible(secretID), "a reveal folds on the truth grain, not the audience that witnessed it")
	s.True(late.HasEdge(holdID, leadsTo, secretID))
}

func (s *GraphSuite) TestAWorldWithNothingConcealedBehavesExactlyAsBefore() {
	// The shared fixture declares nothing concealed. Every existing
	// assertion in this suite already exercises that path; this test states
	// the invariant directly, once, so a regression here fails loudly rather
	// than as a diff scattered across a dozen unrelated tests.
	w := s.world(s.declaration())

	for _, observer := range []journal.EntityID{holdID, gruntID, chiefID, bandID, scoutID, watcherID} {
		state := w.StateFor(observer, s.log)
		s.Truef(state.Visible(holdID), "%s should see an entity nobody concealed", observer)
	}
}

func (s *GraphSuite) TestConcealmentDeclarationsRefuseWhatTheyCannotReveal() {
	cases := []struct {
		name   string
		change func(*graph.Config)
	}{
		{
			name: "a reveal naming an entity nobody declared",
			change: func(cfg *graph.Config) {
				cfg.Reveals[0].Entities = []journal.EntityID{ghostID}
			},
		},
		{
			name: "a reveal naming an entity declared but never concealed",
			change: func(cfg *graph.Config) {
				cfg.Reveals[0].Entities = []journal.EntityID{holdID}
			},
		},
		{
			name: "a reveal naming an edge nobody declared",
			change: func(cfg *graph.Config) {
				cfg.Reveals[0].Edges = []graph.Edge{{From: holdID, Rel: leadsTo, To: ghostID}}
			},
		},
		{
			name: "a reveal naming an edge declared but never concealed",
			change: func(cfg *graph.Config) {
				cfg.Reveals[0].Edges = []graph.Edge{{From: holdID, Rel: hostileTo, To: bandID}}
			},
		},
	}

	for _, tc := range cases {
		s.Run(tc.name, func() {
			cfg := s.concealed()
			tc.change(&cfg)
			_, err := graph.New(cfg)
			s.Require().Error(err)
		})
	}

	s.Run("an entity reference is ErrUnknownConcealedEntity specifically", func() {
		cfg := s.concealed()
		cfg.Reveals[0].Entities = []journal.EntityID{ghostID}
		_, err := graph.New(cfg)
		s.Require().ErrorIs(err, graph.ErrUnknownConcealedEntity)
	})

	s.Run("an edge reference is ErrUnknownConcealedEdge specifically", func() {
		cfg := s.concealed()
		cfg.Reveals[0].Edges = []graph.Edge{{From: holdID, Rel: leadsTo, To: ghostID}}
		_, err := graph.New(cfg)
		s.Require().ErrorIs(err, graph.ErrUnknownConcealedEdge)
	})

	s.Run("the same law applies to pierces, worded for the form-filler", func() {
		cfg := s.concealed()
		cfg.Pierces[0].Entities = []journal.EntityID{ghostID}
		_, err := graph.New(cfg)
		s.Require().ErrorIs(err, graph.ErrUnknownConcealedEntity)
		s.Contains(err.Error(), "mark the entity concealed")
	})
}
