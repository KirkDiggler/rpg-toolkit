// Copyright (C) 2026 Kirk Diggler
// SPDX-License-Identifier: GPL-3.0-or-later

package journal_test

import (
	"testing"

	"github.com/stretchr/testify/suite"

	"github.com/KirkDiggler/rpg-toolkit/examples/world/journal"
)

const (
	camp       journal.EntityID = "bandit-camp"
	lieutenant journal.EntityID = "lieutenant-vek"
	rogue      journal.EntityID = "shadow"
)

type JournalSuite struct {
	suite.Suite

	log *journal.Journal
}

func (s *JournalSuite) SetupTest() {
	s.log = journal.New()
}

func TestJournalSuite(t *testing.T) {
	suite.Run(t, new(JournalSuite))
}

func (s *JournalSuite) TestAppendAssignsSequenceFromOne() {
	first, err := s.log.Append(journal.Fact{Kind: "assault", Actor: rogue, Subject: camp})
	s.Require().NoError(err)
	second, err := s.log.Append(journal.Fact{Kind: "assault", Actor: rogue, Subject: camp})
	s.Require().NoError(err)

	s.Equal(1, first.Seq)
	s.Equal(2, second.Seq)
	s.Equal(2, s.log.Len())
}

func (s *JournalSuite) TestAppendRefusesFactsNoFoldCouldUse() {
	s.Run("no kind", func() {
		_, err := s.log.Append(journal.Fact{Actor: rogue})
		s.Require().ErrorIs(err, journal.ErrEmptyKind)
		s.Zero(s.log.Len())
	})

	s.Run("no actor", func() {
		_, err := s.log.Append(journal.Fact{Kind: "assault"})
		s.Require().ErrorIs(err, journal.ErrEmptyActor)
		s.Zero(s.log.Len())
	})
}

func (s *JournalSuite) TestEmptyAudienceIsRecordedAndWitnessedByNobody() {
	stored, err := s.log.Append(journal.Fact{Kind: "kill", Actor: rogue, Subject: lieutenant})
	s.Require().NoError(err)

	// The quiet kill: the log holds it, and no fold will ever reach it.
	s.Equal(1, s.log.Len())
	s.Empty(stored.Audience)
	s.Empty(s.log.WitnessedBy(camp, lieutenant, rogue))
}

func (s *JournalSuite) TestWitnessedByMatchesAnyGivenID() {
	_, err := s.log.Append(journal.Fact{
		Kind: "unmasked", Actor: lieutenant, Subject: rogue,
		Audience: journal.Audience{lieutenant},
	})
	s.Require().NoError(err)
	_, err = s.log.Append(journal.Fact{
		Kind: "assault", Actor: rogue, Subject: camp,
		Audience: journal.Audience{camp},
	})
	s.Require().NoError(err)

	s.Run("the camp sees only the camp-audienced fact", func() {
		seen := s.log.WitnessedBy(camp)
		s.Require().Len(seen, 1)
		s.Equal(journal.Kind("assault"), seen[0].Kind)
	})

	s.Run("a member folded with its group sees both", func() {
		seen := s.log.WitnessedBy(lieutenant, camp)
		s.Require().Len(seen, 2)
		s.Equal(1, seen[0].Seq)
		s.Equal(2, seen[1].Seq)
	})

	s.Run("an observer that is nobody witnessed nothing", func() {
		s.Empty(s.log.WitnessedBy())
	})
}

func (s *JournalSuite) TestLogIsAppendOnlyThroughItsHandouts() {
	stored, err := s.log.Append(journal.Fact{
		Kind: "assault", Actor: rogue, Subject: camp,
		Audience: journal.Audience{camp},
	})
	s.Require().NoError(err)

	s.Run("mutating the appended copy does not reach the log", func() {
		stored.Audience[0] = lieutenant
		s.True(s.log.All()[0].Audience.Includes(camp))
	})

	s.Run("mutating a handed-out copy does not reach the log", func() {
		handed := s.log.All()
		handed[0].Audience[0] = lieutenant
		handed[0].Kind = "nonsense"
		s.True(s.log.All()[0].Audience.Includes(camp))
		s.Equal(journal.Kind("assault"), s.log.All()[0].Kind)
	})

	s.Run("mutating the caller's original audience does not reach the log", func() {
		aud := journal.Audience{camp}
		_, err := s.log.Append(journal.Fact{Kind: "parley", Actor: rogue, Subject: camp, Audience: aud})
		s.Require().NoError(err)
		aud[0] = lieutenant
		s.True(s.log.All()[1].Audience.Includes(camp))
	})
}

func (s *JournalSuite) TestOutcomeDistinguishesUncontestedFromFailed() {
	declared, err := s.log.Append(journal.Fact{Kind: "belongs", Actor: lieutenant, Subject: camp})
	s.Require().NoError(err)
	failed, err := s.log.Append(journal.Fact{
		Kind: "sneak", Actor: rogue, Subject: camp,
		Outcome: journal.Outcome{Contested: true, Succeeded: false, Margin: -4},
	})
	s.Require().NoError(err)

	// A declared fact is not a failed attempt; the zero value has to say so.
	s.False(declared.Outcome.Contested)
	s.True(failed.Outcome.Contested)
	s.False(failed.Outcome.Succeeded)
	s.Equal(-4, failed.Outcome.Margin)
}
