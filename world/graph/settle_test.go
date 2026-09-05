// Copyright (C) 2026 Kirk Diggler
// SPDX-License-Identifier: GPL-3.0-or-later

package graph_test

import (
	"github.com/KirkDiggler/rpg-toolkit/world/graph"
	"github.com/KirkDiggler/rpg-toolkit/world/journal"
)

// The pair fixture: the hold and the band are hostile in both directions, and
// the hold's chief is the one whose knowledge settles it. A rival faction
// stands beside them so the tests can show what a settle leaves alone.
const (
	rivalID journal.EntityID = "rival"

	learnedKind  journal.Kind = "learned"
	betrayalKind journal.Kind = "betrayal"
	turningKind  journal.Kind = "turning"

	learned  graph.Flag = "learned"
	betrayed graph.Flag = "betrayed"
	turned   graph.Flag = "turned"
)

// paired extends the shared fixture with the return direction of the hold's
// hostility, a rival the hold is also hostile to, and the reducer that raises
// learned on whoever a learned fact is about.
func (s *GraphSuite) paired() graph.Config {
	cfg := s.declaration()
	cfg.Entities = append(cfg.Entities, graph.Entity{ID: rivalID, Kind: faction})
	cfg.Edges = append(cfg.Edges,
		graph.Edge{From: bandID, Rel: hostileTo, To: holdID},
		graph.Edge{From: holdID, Rel: hostileTo, To: rivalID},
		graph.Edge{From: rivalID, Rel: hostileTo, To: bandID},
	)
	cfg.Reducers = []graph.Reducer{graph.Raise{On: learnedKind, Flag: learned}}

	return cfg
}

// settle is the hold-out's declaration: while the chief knows, the hold and
// the band are no longer hostile — they are `to`, in both directions.
func (s *GraphSuite) settle(to graph.Relation) graph.Settle {
	return graph.Settle{
		OnFlag:    learned,
		Of:        chiefID,
		Between:   [2]journal.EntityID{holdID, bandID},
		Relations: []graph.Relation{hostileTo},
		To:        to,
	}
}

// learn records that subject came to know the thing, witnessed by audience.
func (s *GraphSuite) learn(subject journal.EntityID, audience ...journal.EntityID) {
	s.append(journal.Fact{Kind: learnedKind, Actor: scoutID, Subject: subject, Audience: audience})
}

func (s *GraphSuite) TestSettleRefusesWhatItCannotFold() {
	cases := []struct {
		name   string
		change func(*graph.Settle)
		want   error
	}{
		{
			name:   "a pair may not name an undeclared entity",
			change: func(p *graph.Settle) { p.Between[1] = ghostID },
			want:   graph.ErrUnknownEntity,
		},
		{
			name:   "the knower may not be an undeclared entity",
			change: func(p *graph.Settle) { p.Of = ghostID },
			want:   graph.ErrUnknownEntity,
		},
		{
			name:   "the zero value names nobody to watch",
			change: func(p *graph.Settle) { p.Of = "" },
			want:   graph.ErrNoKnower,
		},
		{
			name:   "a pair with an empty side is no pair",
			change: func(p *graph.Settle) { p.Between = [2]journal.EntityID{holdID, ""} },
			want:   graph.ErrNoPair,
		},
		{
			name:   "a pair of one is not a pair",
			change: func(p *graph.Settle) { p.Between = [2]journal.EntityID{holdID, holdID} },
			want:   graph.ErrPairOfOne,
		},
		{
			name:   "a settle that replaces no relations could never change anything",
			change: func(p *graph.Settle) { p.Relations = nil },
			want:   graph.ErrNoRelations,
		},
		{
			name:   "membership is not a stance to replace",
			change: func(p *graph.Settle) { p.Relations = []graph.Relation{belongsTo} },
			want:   graph.ErrSettlesMembership,
		},
		{
			name:   "membership is not a stance to settle to",
			change: func(p *graph.Settle) { p.To = belongsTo },
			want:   graph.ErrSettlesMembership,
		},
	}

	// A projection declared by pointer satisfies the interface too, so every
	// refusal is asserted in both spellings: a &Settle{} that walked past
	// New would delete membership itself, silently, once its flag went up.
	for _, tc := range cases {
		s.Run(tc.name, func() {
			settle := s.settle(alliedWith)
			tc.change(&settle)
			cfg := s.paired()
			cfg.Projections = []graph.Projection{settle}

			_, err := graph.New(cfg)
			s.Require().ErrorIs(err, tc.want)
		})

		s.Run(tc.name+", declared by pointer", func() {
			settle := s.settle(alliedWith)
			tc.change(&settle)
			cfg := s.paired()
			cfg.Projections = []graph.Projection{&settle}

			_, err := graph.New(cfg)
			s.Require().ErrorIs(err, tc.want)
		})
	}

	s.Run("the declaration those were mutations of is accepted", func() {
		cfg := s.paired()
		cfg.Projections = []graph.Projection{s.settle(alliedWith)}
		_, err := graph.New(cfg)
		s.Require().NoError(err)
	})

	s.Run("and declared by pointer it is accepted and folds the same", func() {
		settle := s.settle(alliedWith)
		cfg := s.paired()
		cfg.Projections = []graph.Projection{&settle}
		w := s.world(cfg)

		s.learn(chiefID, chiefID)
		state := w.StateFor(chiefID, s.log)
		s.False(state.HasEdge(holdID, hostileTo, bandID))
		s.True(state.HasEdge(bandID, alliedWith, holdID))
	})
}

func (s *GraphSuite) TestSettleReplacesBothDirectionsWhileTheFlagIsUp() {
	cfg := s.paired()
	cfg.Projections = []graph.Projection{s.settle(alliedWith)}
	w := s.world(cfg)

	s.Run("declared, the pair is hostile both ways", func() {
		state := w.StateFor(chiefID, s.log)
		s.True(state.HasEdge(holdID, hostileTo, bandID))
		s.True(state.HasEdge(bandID, hostileTo, holdID))
		s.False(state.HasEdge(holdID, alliedWith, bandID))
		s.False(state.HasEdge(bandID, alliedWith, holdID))
	})

	s.learn(chiefID, chiefID)

	s.Run("once the chief knows, hostility is gone in both directions", func() {
		state := w.StateFor(chiefID, s.log)
		s.False(state.HasEdge(holdID, hostileTo, bandID))
		s.False(state.HasEdge(bandID, hostileTo, holdID))
	})

	s.Run("and the settled relation stands in both directions", func() {
		state := w.StateFor(chiefID, s.log)
		s.True(state.HasEdge(holdID, alliedWith, bandID))
		s.True(state.HasEdge(bandID, alliedWith, holdID))
	})

	s.Run("a third party's edges are not the pair's business", func() {
		state := w.StateFor(chiefID, s.log)
		s.True(state.HasEdge(holdID, hostileTo, rivalID), "the hold is still hostile to the rival")
		s.True(state.HasEdge(rivalID, hostileTo, bandID), "the rival is still hostile to the band")
		s.True(state.HasEdge(chiefID, belongsTo, holdID), "and nobody changed sides")
	})
}

func (s *GraphSuite) TestSettleToNothingIsTheHonestNeutral() {
	cfg := s.paired()
	cfg.Projections = []graph.Projection{s.settle("")}
	w := s.world(cfg)

	before := len(w.StateFor(chiefID, s.log).Edges())
	s.learn(chiefID, chiefID)
	state := w.StateFor(chiefID, s.log)

	s.False(state.HasEdge(holdID, hostileTo, bandID))
	s.False(state.HasEdge(bandID, hostileTo, holdID))
	s.False(state.HasEdge(holdID, alliedWith, bandID), "neutral is not alliance")
	s.False(state.HasEdge(bandID, alliedWith, holdID))
	s.Len(state.Edges(), before-2, "two edges left and nothing arrived: absence is the whole of neutral")
	s.Empty(state.Refusals())
}

func (s *GraphSuite) TestSettleFoldsNothingUntilItsOwnEntityCarriesTheFlag() {
	cfg := s.paired()
	cfg.Projections = []graph.Projection{s.settle(alliedWith)}
	w := s.world(cfg)

	hostileBothWays := func(state *graph.State) {
		s.True(state.HasEdge(holdID, hostileTo, bandID))
		s.True(state.HasEdge(bandID, hostileTo, holdID))
		s.False(state.HasEdge(holdID, alliedWith, bandID))
	}

	s.Run("nobody knows: the declared edges stand", func() {
		hostileBothWays(w.StateFor(chiefID, s.log))
	})

	s.learn(scoutID, chiefID)

	s.Run("the scout knowing it is not the chief knowing it", func() {
		state := w.StateFor(chiefID, s.log)
		s.True(state.Flagged(learned, scoutID), "the chief saw the scout learn it")
		s.False(state.Flagged(learned, chiefID))
		hostileBothWays(state)
	})

	s.learn(chiefID, chiefID)

	s.Run("the chief knowing it settles the pair", func() {
		s.False(w.StateFor(chiefID, s.log).HasEdge(holdID, hostileTo, bandID))
	})

	s.Run("rolling the journal back puts the declared edges back", func() {
		hostileBothWays(w.StateFor(chiefID, journal.New()))
	})
}

// TestTheLaterDeclarationWinsTheEdge pins the one precedence rule, stated on
// [graph.Projection]: declared order, and the last to touch an edge wins. An
// AdoptStance on the hold and a Settle on the hold–band pair both speak about
// hold→band; which one is heard is decided by nothing but their order.
func (s *GraphSuite) TestTheLaterDeclarationWinsTheEdge() {
	// The rival is hostile to the band, so a hold that turns rival adopts
	// hold→band hostility — the very edge the settle removes.
	adopt := graph.AdoptStance{OnFlag: turned, From: rivalID, Relations: []graph.Relation{hostileTo}}

	build := func(projections ...graph.Projection) *graph.World {
		cfg := s.paired()
		cfg.Reducers = append(cfg.Reducers, graph.Raise{On: turningKind, Flag: turned})
		cfg.Projections = projections

		return s.world(cfg)
	}

	s.learn(chiefID, chiefID)
	s.append(journal.Fact{Kind: turningKind, Actor: scoutID, Subject: holdID, Audience: journal.Audience{chiefID}})

	s.Run("settle first, adopt last: the adopted hostility is what stands", func() {
		state := build(s.settle(""), adopt).StateFor(chiefID, s.log)
		s.True(state.HasEdge(holdID, hostileTo, bandID))
		s.False(state.HasEdge(bandID, hostileTo, holdID), "the adopt spoke about the hold's edges only")
	})

	s.Run("adopt first, settle last: the pair is settled", func() {
		state := build(adopt, s.settle("")).StateFor(chiefID, s.log)
		s.False(state.HasEdge(holdID, hostileTo, bandID))
		s.False(state.HasEdge(bandID, hostileTo, holdID))
	})

	s.Run("the rival's own edge is untouched either way", func() {
		s.True(build(s.settle(""), adopt).StateFor(chiefID, s.log).HasEdge(rivalID, hostileTo, bandID))
		s.True(build(adopt, s.settle("")).StateFor(chiefID, s.log).HasEdge(rivalID, hostileTo, bandID))
	})
}

func (s *GraphSuite) TestSettleAnswersEachObserverHonestly() {
	cfg := s.paired()
	cfg.Projections = []graph.Projection{s.settle("")}
	w := s.world(cfg)

	// The chief alone witnesses the learning. Nobody else in the hold was in
	// the room, and the band was never told.
	s.learn(chiefID, chiefID)

	s.Run("folded as the chief, the pair is settled", func() {
		s.False(w.StateFor(chiefID, s.log).HasEdge(holdID, hostileTo, bandID))
	})

	s.Run("folded as a hold-mate who was not there, it is not", func() {
		s.True(w.StateFor(gruntID, s.log).HasEdge(holdID, hostileTo, bandID))
	})

	s.Run("folded as the hold itself, it is not — the fact was audienced to the chief alone", func() {
		s.True(w.StateFor(holdID, s.log).HasEdge(holdID, hostileTo, bandID))
	})

	s.Run("folded as a bystander from the band, it is not", func() {
		s.True(w.StateFor(scoutID, s.log).HasEdge(bandID, hostileTo, holdID))
	})

	s.Run("truth is audience-blind and sees it settled", func() {
		truth := w.Truth(s.log)
		s.False(truth.HasEdge(holdID, hostileTo, bandID))
		s.False(truth.HasEdge(bandID, hostileTo, holdID))
	})
}

func (s *GraphSuite) TestSettleIsReversedOnlyByALaterSettle() {
	// Flags never clear. The hold's way back to war is a betrayal raising a
	// second flag on a second Settle declared after the first; the learned
	// flag stays up and is overruled.
	cfg := s.paired()
	cfg.Reducers = append(cfg.Reducers, graph.Raise{On: betrayalKind, Flag: betrayed})
	cfg.Projections = []graph.Projection{
		s.settle(alliedWith),
		graph.Settle{
			OnFlag:    betrayed,
			Of:        holdID,
			Between:   [2]journal.EntityID{holdID, bandID},
			Relations: []graph.Relation{hostileTo, alliedWith},
			To:        hostileTo,
		},
	}
	w := s.world(cfg)

	s.learn(chiefID, chiefID)
	s.True(w.StateFor(chiefID, s.log).HasEdge(holdID, alliedWith, bandID), "settled")

	s.append(journal.Fact{Kind: betrayalKind, Actor: scoutID, Subject: holdID, Audience: journal.Audience{holdID}})

	state := w.StateFor(chiefID, s.log)
	s.True(state.Flagged(learned, chiefID), "the chief still knows")
	s.True(state.HasEdge(holdID, hostileTo, bandID))
	s.True(state.HasEdge(bandID, hostileTo, holdID))
	s.False(state.HasEdge(holdID, alliedWith, bandID))
	s.False(state.HasEdge(bandID, alliedWith, holdID))
}
