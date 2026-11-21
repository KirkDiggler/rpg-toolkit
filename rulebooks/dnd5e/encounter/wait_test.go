// Copyright (C) 2026 Kirk Diggler
// SPDX-License-Identifier: GPL-3.0-or-later

package encounter_test

import (
	"errors"

	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/encounter"
	"github.com/KirkDiggler/rpg-toolkit/tools/spatial"
)

// wait_test.go pins WAIT (rpg-project#1079): one act; it costs that character
// one round on the world clock and does nothing else.

func (s *WorldTimeSuite) waits(enc *encounter.Encounter, who encounter.MemberID) {
	s.T().Helper()
	_, err := enc.Wait(&encounter.WaitInput{Member: who})
	s.Require().NoError(err)
}

func (s *WorldTimeSuite) onePlayerHall() *encounter.Encounter {
	return s.hall(encounter.MemberInput{
		ID: alice, Kind: encounter.KindPlayer, Position: spatial.Position{X: 1, Y: 1},
	})
}

func (s *WorldTimeSuite) TestAWaitIsARoundForItsActor() {
	enc := s.onePlayerHall()
	s.waits(enc, alice)
	s.Equal(1, s.reading(enc))
	s.waits(enc, alice)
	s.Equal(2, s.reading(enc))
}

func (s *WorldTimeSuite) TestAWaitSaysSo() {
	enc := s.onePlayerHall()
	s.waits(enc, alice)

	beat := s.beatOf(enc, encounter.BeatStayed, alice)
	s.Require().NotNil(beat, "a wait is a round that moved nobody, and it must be visible")
	s.Equal("encounter:intent:wait", beat["cause"])
	s.Equal("", beat["why"])
	s.Equal(1, s.ticks(enc))
}

func (s *WorldTimeSuite) TestTwoPlayersWaitingTogetherAreOneRound() {
	enc := s.hall(
		encounter.MemberInput{ID: alice, Kind: encounter.KindPlayer, Position: spatial.Position{X: 1, Y: 1}},
		encounter.MemberInput{ID: billy, Kind: encounter.KindPlayer, Position: spatial.Position{X: 2, Y: 1}},
	)
	s.waits(enc, alice)
	s.Equal(1, s.reading(enc))
	s.waits(enc, billy)
	s.Equal(1, s.reading(enc), "billy only reaches the high-water alice set")
	s.waits(enc, alice)
	s.Equal(2, s.reading(enc))
}

func (s *WorldTimeSuite) TestAHeldPlayerWaits() {
	enc := s.hallSheets(
		sheetFacts{alice: {SpeedFeet: 30, CannotMove: []string{"Held"}}}, nil,
		encounter.MemberInput{ID: alice, Kind: encounter.KindPlayer, Position: spatial.Position{X: 1, Y: 1}},
	)
	_, err := enc.Step(&encounter.StepInput{Member: alice, To: cellAt(2, 1)})
	s.Require().ErrorIs(err, encounter.ErrCannotMove, "precondition: she is held")

	s.waits(enc, alice)
	s.Equal(1, s.reading(enc))
}

func (s *WorldTimeSuite) TestAWaitWakesWhoIsDue() {
	enc := s.hallWith(
		[]encounter.DispositionInput{{
			Between: [2]encounter.FactionID{"vendors", encounter.FactionParty},
			Stance:  encounter.StanceNeutral,
		}},
		encounter.MemberInput{ID: alice, Kind: encounter.KindPlayer, Position: spatial.Position{X: 1, Y: 1}},
		encounter.MemberInput{ID: goblin, Kind: encounter.KindMonster, Faction: "vendors",
			Position: spatial.Position{X: 25, Y: 6}, Behaviors: ordinaryBehavior(walksTo(cellAt(20, 6)))},
	)
	started := whereIs(s.T(), enc, goblin)
	s.waits(enc, alice)
	s.NotEqual(started, whereIs(s.T(), enc, goblin), "the round a wait pays wakes the creature")
}

func (s *WorldTimeSuite) TestAWaitInsideAFightIsRefused() {
	enc := s.hall(
		encounter.MemberInput{ID: alice, Kind: encounter.KindPlayer, Position: spatial.Position{X: 1, Y: 1}},
		encounter.MemberInput{ID: goblin, Kind: encounter.KindMonster, Position: spatial.Position{X: 3, Y: 1}},
	)
	on, err := enc.ClockOf(&encounter.ClockOfInput{Member: alice})
	s.Require().NoError(err)
	s.Require().Equal(encounter.ClockTurn, on.Kind, "precondition: a fight")
	before, ticks := s.reading(enc), s.ticks(enc)

	_, err = enc.Wait(&encounter.WaitInput{Member: alice})
	s.Require().ErrorIs(err, encounter.ErrInBubble)
	s.Equal(before, s.reading(enc))
	s.Equal(ticks, s.ticks(enc))
	s.Nil(s.beatOf(enc, encounter.BeatStayed, alice))
}

func (s *WorldTimeSuite) TestWaitRefusesWhatItCannotPrice() {
	build := func() *encounter.Encounter {
		return s.hall(
			encounter.MemberInput{ID: alice, Kind: encounter.KindPlayer, Position: spatial.Position{X: 1, Y: 1}},
			encounter.MemberInput{ID: "vendor", Kind: encounter.KindWorld, Position: spatial.Position{X: 5, Y: 1}},
		)
	}
	cases := []struct {
		name string
		in   *encounter.WaitInput
		want error
		end  bool
	}{
		{"nil input", nil, encounter.ErrNilInput, false},
		{"empty member", &encounter.WaitInput{}, encounter.ErrNoMember, false},
		{"not a member", &encounter.WaitInput{Member: "nobody"}, encounter.ErrNotMember, false},
		{"not a player", &encounter.WaitInput{Member: "vendor"}, encounter.ErrNotMember, false},
		{"run ended", &encounter.WaitInput{Member: alice}, encounter.ErrClosed, true},
	}
	for _, c := range cases {
		s.Run(c.name, func() {
			enc := build()
			if c.end {
				_, err := enc.End(&encounter.EndInput{Ending: "withdrawn"})
				s.Require().NoError(err)
			}
			before := s.reading(enc)
			_, err := enc.Wait(c.in)
			s.Require().Error(err)
			s.True(errors.Is(err, c.want), "got %v", err)
			s.Equal(before, s.reading(enc))
		})
	}
}
