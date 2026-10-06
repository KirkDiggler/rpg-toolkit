// Copyright (C) 2026 Kirk Diggler
// SPDX-License-Identifier: GPL-3.0-or-later

package encounter_test

import (
	"encoding/json"
	"errors"

	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/encounter"
	"github.com/KirkDiggler/rpg-toolkit/tools/spatial"
)

func (s *AutomaticDiscoverySuite) TestFootprintDoorSupportStillTriggersDiscoveryWithoutHiddenFloor() {
	r := &discoveryRoller{success: true}
	in := discoveryFixture(r)
	in.Field.Props = nil
	in.Field.Concealments[0].Props = nil
	in.Field.Concealments[0].Doors = []encounter.DoorID{"secret-door"}
	placement := coveredBox(1, centreOf(spatial.Position{X: 4}))
	in.Field.Doors = []encounter.DoorInput{{ID: "secret-door", Placement: &placement, State: encounter.DoorIsClosed()}}
	enc := s.create(in)
	before, err := enc.AtlasFor("alice")
	s.Require().NoError(err)
	full, err := enc.Atlas()
	s.Require().NoError(err)
	s.Equal(full.Cells, before.Cells)
	s.walk(enc, 1, 2)
	s.Zero(r.calls)
	s.step(enc, 3)
	s.Equal(1, r.calls, "selected door geometry still supplies discovery distance")
	s.True(s.memory(enc, "alice")["secret"].Learned)
	after, err := enc.AtlasFor("alice")
	s.Require().NoError(err)
	s.Equal(before.Cells, after.Cells, "discovering an object reveals no additional floor")
}

func (s *AutomaticDiscoverySuite) TestBlockedHexDoesNotGateDiscovery() {
	r := &discoveryRoller{}
	in := discoveryFixture(r)
	in.Field.Walls = []encounter.WallInput{wall(3, 0, 4, 0)}
	enc := s.create(in)
	canvas, err := enc.Canvas()
	s.Require().NoError(err)
	s.True(canvas.IsLineOfSightBlocked(spatial.Position{X: 3}, spatial.Position{X: 4}))
	s.walk(enc, 1, 2, 3)
	s.Equal(1, r.calls, "the discovery range ignores the actual sight/movement obstruction")
	_, err = enc.Step(&encounter.StepInput{Member: "alice", To: spatial.Position{X: 4}})
	s.Error(err, "discovery does not grant movement through the blocker")
	s.Equal(1, r.calls)
}

func (s *AutomaticDiscoverySuite) TestFailedResolverWritesNoAttemptOrResult() {
	for _, nilVerdict := range []bool{false, true} {
		name := "error"
		if nilVerdict {
			name = "invalid nil verdict"
		}
		s.Run(name, func() {
			r := &discoveryRoller{}
			enc := s.create(discoveryFixture(r))
			s.walk(enc, 1, 2)
			persisted := s.reloadDiscovery(enc, r)
			failure := errors.New("resolver unavailable")
			if nilVerdict {
				r.nilVerdict = true
			} else {
				r.err = failure
			}
			out, err := enc.Step(&encounter.StepInput{Member: "alice", To: spatial.Position{X: 3}})
			s.Require().Error(err)
			s.Nil(out)
			if nilVerdict {
				s.ErrorIs(err, encounter.ErrBadConcealment)
			} else {
				s.ErrorIs(err, failure)
			}
			s.NotContains(enc.ToData().Discovery["alice"].Attempts, "secret")
			s.Empty(s.results(enc, "alice"))
			s.Empty(s.results(enc, "bob"))
			// The failed Step may already have moved/appended movement in its
			// in-memory instance. Discard it, as the composition contract says.
			r.err, r.nilVerdict = nil, false
			s.step(persisted, 3)
			s.Equal(1, persisted.ToData().Discovery["alice"].Attempts["secret"].Used)
			s.Len(s.results(persisted, "alice"), 1)
		})
	}
}

func (s *AutomaticDiscoverySuite) TestKnownSecretDoesNotRerollWithAllowanceRemaining() {
	r := &discoveryRoller{success: true}
	enc := s.world(false, 3, r)
	s.walk(enc, 1, 2, 3, 2, 1, 0, 1, 2, 3)
	s.Equal(1, r.calls)
	s.Equal(encounter.DiscoveryMemoryData{Used: 1, Learned: true}, s.memory(enc, "alice")["secret"])
	s.True(s.memory(enc, "bob")["secret"].Learned)
	loaded := s.reloadDiscovery(enc, r)
	s.walk(loaded, 2, 1, 2, 3)
	s.Equal(1, r.calls)
}

func (s *AutomaticDiscoverySuite) TestDefaultAttemptSurvivesDepartureAndReload() {
	r := &discoveryRoller{}
	in := discoveryFixture(r) // no explicit policy: exercise the actual default
	s.Nil(in.Field.Concealments[0].Attempts)
	enc := s.create(in)
	s.walk(enc, 1, 2, 3)
	loaded := s.reloadDiscovery(enc, r)
	s.Equal(1, r.calls, "loading cannot roll")
	s.walk(loaded, 2, 1, 0, 1, 2, 3)
	s.Equal(1, r.calls)
	s.Equal(1, loaded.ToData().Discovery["alice"].Attempts["secret"].Used)
	s.Len(s.results(loaded, "alice"), 1)
}

func (s *AutomaticDiscoverySuite) TestResetDistanceOverrideAndDistanceNotAccumulatedSteps() {
	r := &discoveryRoller{}
	in := discoveryFixture(r)
	in.Field.Concealments[0].Attempts = &encounter.DiscoveryPolicyInput{
		MaxAttempts: discoveryInt(2), ResetHexes: discoveryInt(4),
	}
	enc := s.create(in)
	s.walk(enc, 1, 2, 3)
	for range 3 {
		s.walk(enc, 2, 1, 2, 3) // distance three, many steps, still below authored four
	}
	s.Equal(1, r.calls)
	s.False(enc.ToData().Discovery["alice"].Attempts["secret"].Armed)
	s.walk(enc, 2, 1, 0)
	s.Equal(1, r.calls, "reaching reset distance only re-arms")
	s.True(enc.ToData().Discovery["alice"].Attempts["secret"].Armed)
	s.walk(enc, 1, 2, 3)
	s.Equal(2, r.calls)
	s.walk(enc, 2, 1, 0, 1, 2, 3)
	s.Equal(2, r.calls, "re-arm does not refill the exhausted allowance")
}

func (s *AutomaticDiscoverySuite) TestAudienceExcludesExitedReservedAndNonPlayerMembers() {
	r := &discoveryRoller{success: true}
	in := discoveryFixture(r)
	in.Field.Dispositions = []encounter.DispositionInput{{
		Between: [2]encounter.FactionID{encounter.FactionParty, encounter.FactionMonsters}, Stance: encounter.StanceNeutral,
	}}
	in.Members = append(in.Members,
		encounter.MemberInput{ID: "monster", Kind: encounter.KindMonster, Position: cellAt(8, 2)},
		encounter.MemberInput{ID: "reserved", Kind: encounter.KindMonster, Position: cellAt(7, 2), Arrives: encounter.TriggerRound{Round: 99}},
		encounter.MemberInput{ID: "vendor", Kind: encounter.KindWorld, Position: cellAt(6, 2)},
		encounter.MemberInput{ID: "present", Kind: encounter.KindPlayer, Position: cellAt(1, 2)},
	)
	enc := s.create(in)
	s.Require().Len(enc.ToData().Reserve, 1, "there really is an unplaced reserved member")
	_, err := enc.Exit(&encounter.ExitInput{Member: "bob"})
	s.Require().NoError(err)
	s.walk(enc, 1, 2, 3)
	s.Equal(1, r.calls)
	s.Len(s.results(enc, "alice"), 1)
	s.Len(s.results(enc, "present"), 1)
	s.Empty(s.results(enc, "bob"))
	s.Empty(s.results(enc, "monster"))
	s.Empty(s.results(enc, "vendor"))
	s.True(s.memory(enc, "present")["secret"].Learned)
	for _, entry := range enc.ToData().Log.Entries {
		var body map[string]any
		s.Require().NoError(json.Unmarshal(entry.Payload, &body))
		if body["beat"] == encounter.BeatDiscoveryChecked {
			s.Equal([]encounter.MemberID{"alice", "present"}, entry.Audience)
		}
	}
	// Players cannot be unplaced reserve members in a valid encounter.
	_, err = enc.Join(&encounter.JoinInput{
		Member: "unplaced-player", Kind: encounter.KindPlayer, Cell: cellAt(2, 2),
		Arrives: encounter.TriggerRound{Round: 99},
	})
	s.ErrorIs(err, encounter.ErrNoMember)
	_, err = enc.Join(&encounter.JoinInput{Member: "late", Kind: encounter.KindPlayer, Cell: cellAt(0, 1)})
	s.Require().NoError(err)
	s.Empty(s.results(enc, "late"), "a later arrival is not added to the old audience")
	s.Empty(s.memory(enc, "late"), "no historical discovery backfill")
}

func (s *AutomaticDiscoverySuite) TestMultipleMembersAndApproachesDoNotMultiplyAttempts() {
	r := &discoveryRoller{}
	in := discoveryFixture(r)
	part := holdableProp("other-part", "test:props:other", spatial.Position{X: 4, Y: 1})
	part.Holdable = false
	in.Field.Props = append(in.Field.Props, part)
	in.Field.Concealments[0].Props = append(in.Field.Concealments[0].Props, part.ID)
	in.Field.Concealments[0].Checks = append(in.Field.Concealments[0].Checks, encounter.CheckApproach{Ability: "investigation", DC: 12})
	enc := s.create(in)
	for _, cell := range []spatial.Position{{Y: 1}, {X: 1, Y: 1}, {X: 2, Y: 1}, {X: 3, Y: 1}} {
		_, err := enc.Step(&encounter.StepInput{Member: "alice", To: cell})
		s.Require().NoError(err)
	}
	s.Equal(1, r.calls, "two physical members and two approaches still describe one check")
	s.Equal(1, enc.ToData().Discovery["alice"].Attempts["secret"].Used)
}

func (s *AutomaticDiscoverySuite) TestMultipleChecksUseSortedPlayerAndCheckOrder() {
	r := &discoveryRoller{}
	in := discoveryFixture(r)
	first := holdableProp("first-prop", "test:props:first", spatial.Position{X: 4})
	last := holdableProp("last-prop", "test:props:last", spatial.Position{X: 3, Y: 1})
	first.Holdable, last.Holdable = false, false
	in.Field.Props = []encounter.PropInput{last, first}
	in.Field.Concealments = []encounter.ConcealmentInput{
		{ID: "z-last", Checks: []encounter.CheckApproach{{Ability: "religion", DC: 22}}, Props: []string{"last-prop"}},
		{ID: "a-first", Checks: []encounter.CheckApproach{{Ability: "religion", DC: 11}}, Props: []string{"first-prop"}},
	}
	in.Members = []encounter.MemberInput{
		{ID: "bob", Kind: encounter.KindPlayer, Position: cellAt(4, 1)},
		{ID: "alice", Kind: encounter.KindPlayer, Position: cellAt(3, 0)},
	}
	enc := s.create(in)
	s.Equal([]discoveryConsult{{"alice", 11}, {"alice", 22}, {"bob", 11}, {"bob", 22}}, r.order)
	s.Len(s.results(enc, "alice"), 4)
	s.Len(s.results(enc, "bob"), 4)
}
