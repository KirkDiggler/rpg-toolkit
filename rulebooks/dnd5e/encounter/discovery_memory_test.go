// Copyright (C) 2026 Kirk Diggler
// SPDX-License-Identifier: GPL-3.0-or-later

package encounter_test

import (
	"encoding/json"

	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/encounter"
	"github.com/KirkDiggler/rpg-toolkit/tools/spatial"
)

func discoveryFixture(resolver encounter.CheckResolver) *encounter.SetupInput {
	prop := holdableProp("secret", "test:props:idol", spatial.Position{X: 4})
	prop.Holdable = false
	prop.BlocksMovement = boolPtr(true)
	prop.BlocksLineOfSight = boolPtr(true)
	return &encounter.SetupInput{
		Sight: everyoneSeesTheWholeMap{}, Equipment: encounter.UnobservedEquipment{}, Sheets: zeroSheets{}, Standing: everyoneStanding{},
		Initiative: orderAsGiven{}, TurnDriver: passDriver{}, Striker: passStriker{},
		Mover: quietMover{}, Announcer: quietAnnouncer{}, CheckResolver: resolver, Witness: nobodyPerceives{},
		Field: encounter.FieldInput{
			Canvas: pointyCanvas(), Regions: []encounter.RegionInput{rectRegion("hall", 0, 0, 10, 3)},
			Props: []encounter.PropInput{prop},
			Concealments: []encounter.ConcealmentInput{{
				ID: "secret", Checks: []encounter.CheckApproach{{Ability: "religion", DC: 15}}, Props: []string{"secret"},
			}},
		},
		Members: []encounter.MemberInput{
			{ID: "alice", Kind: encounter.KindPlayer, Position: spatial.Position{}},
			{ID: "bob", Kind: encounter.KindPlayer, Position: spatial.Position{Y: 2}},
		},
		Endings: []encounter.EndingInput{{Key: "exit", Trigger: encounter.TriggerExternal{}}},
	}
}

func (s *AutomaticDiscoverySuite) create(in *encounter.SetupInput) *encounter.Encounter {
	enc, err := encounter.NewEncounter(in)
	s.Require().NoError(err)
	return enc
}

func (s *AutomaticDiscoverySuite) reloadDiscovery(enc *encounter.Encounter, resolver encounter.CheckResolver) *encounter.Encounter {
	raw, err := json.Marshal(enc.ToData())
	s.Require().NoError(err)
	var data encounter.EncounterData
	s.Require().NoError(json.Unmarshal(raw, &data))
	loaded, err := encounter.LoadEncounter(&encounter.LoadEncounterInput{
		Data: data, Sight: everyoneSeesTheWholeMap{}, Equipment: encounter.UnobservedEquipment{}, Sheets: zeroSheets{}, Standing: everyoneStanding{},
		Initiative: orderAsGiven{}, TurnDriver: passDriver{}, Striker: passStriker{},
		Mover: quietMover{}, Announcer: quietAnnouncer{}, CheckResolver: resolver, Witness: nobodyPerceives{},
	})
	s.Require().NoError(err)
	return loaded
}

func (s *AutomaticDiscoverySuite) memory(enc *encounter.Encounter, member encounter.MemberID) map[encounter.ConcealmentID]encounter.DiscoveryMemoryData {
	out, err := enc.DiscoveryMemory(&encounter.DiscoveryMemoryInput{Member: member})
	s.Require().NoError(err)
	return out.Checks
}

func (s *AutomaticDiscoverySuite) walk(enc *encounter.Encounter, xs ...int) {
	for _, x := range xs {
		s.step(enc, x)
	}
}

func (s *AutomaticDiscoverySuite) TestSetupAndJoinRestoreBeforeFirstSweep() {
	for _, via := range []string{"setup", "join"} {
		for _, tc := range []struct {
			name   string
			max    int
			memory encounter.DiscoveryMemoryData
			rolls  int
			used   int
		}{
			{"spent single try", 1, encounter.DiscoveryMemoryData{Used: 1}, 0, 1},
			{"history exceeds current allowance", 1, encounter.DiscoveryMemoryData{Used: 2}, 0, 2},
			{"repeatable new visit is armed", 3, encounter.DiscoveryMemoryData{Used: 1}, 1, 2},
			{"already learned", 3, encounter.DiscoveryMemoryData{Learned: true}, 0, 0},
		} {
			s.Run(via+"/"+tc.name, func() {
				r := &discoveryRoller{}
				in := discoveryFixture(r)
				in.Field.Concealments[0].Attempts = &encounter.DiscoveryPolicyInput{MaxAttempts: discoveryInt(tc.max)}
				retained := map[encounter.ConcealmentID]encounter.DiscoveryMemoryData{"secret": tc.memory}
				var enc *encounter.Encounter
				member := encounter.MemberID("alice")
				if via == "setup" {
					in.Members[0].Position = spatial.Position{X: 3}
					in.Members[0].PrivateDiscoveries = true
					in.Members[0].RetainedDiscoveries = retained
					enc = s.create(in)
				} else {
					enc = s.create(in)
					member = "incoming"
					_, err := enc.Join(&encounter.JoinInput{
						Member: member, Kind: encounter.KindPlayer, Cell: spatial.Position{X: 3},
						PrivateDiscoveries: true, RetainedDiscoveries: retained,
					})
					s.Require().NoError(err)
				}
				s.Equal(tc.rolls, r.calls)
				s.Equal(tc.used, s.memory(enc, member)["secret"].Used)
				s.Equal(tc.memory.Learned, s.memory(enc, member)["secret"].Learned)
				sharing, err := enc.DiscoverySharing(&encounter.DiscoveryMemoryInput{Member: member})
				s.Require().NoError(err)
				s.False(sharing.Sharing, "the restored preference applies to the first roll")
				s.Empty(s.results(enc, "bob"))
				s.Empty(s.memory(enc, "bob"), "restoring personal memory is not party backfill")
				// The constructor must not retain aliases into the host profile.
				retained["secret"] = encounter.DiscoveryMemoryData{Used: 99}
				s.Equal(tc.used, s.memory(enc, member)["secret"].Used)
			})
		}
	}
}

func (s *AutomaticDiscoverySuite) TestRestoreMergesMonotonicallyWithoutRearmingOrBroadcasting() {
	r := &discoveryRoller{}
	enc := s.world(true, 3, r)
	s.walk(enc, 1, 2, 3)
	s.Equal(1, r.calls)
	s.False(enc.ToData().Discovery["alice"].Attempts["secret"].Armed)
	beforeBob, err := enc.Story(&encounter.StoryInput{Audience: "bob"})
	s.Require().NoError(err)
	_, err = enc.RestoreDiscovery(&encounter.RestoreDiscoveryInput{
		Member: "alice", Private: true,
		Checks: map[encounter.ConcealmentID]encounter.DiscoveryMemoryData{"secret": {}},
	})
	s.Require().NoError(err)
	s.Equal(1, s.memory(enc, "alice")["secret"].Used, "older storage cannot lower the live count")
	s.False(enc.ToData().Discovery["alice"].Attempts["secret"].Armed, "same-visit restoration does not re-arm")
	_, err = enc.RestoreDiscovery(&encounter.RestoreDiscoveryInput{
		Member: "alice", Private: true,
		Checks: map[encounter.ConcealmentID]encounter.DiscoveryMemoryData{"secret": {Used: 2, Learned: true}},
	})
	s.Require().NoError(err)
	_, err = enc.RestoreDiscovery(&encounter.RestoreDiscoveryInput{
		Member: "alice", Private: true,
		Checks: map[encounter.ConcealmentID]encounter.DiscoveryMemoryData{"secret": {Used: 1}},
	})
	s.Require().NoError(err)
	s.Equal(encounter.DiscoveryMemoryData{Used: 2, Learned: true}, s.memory(enc, "alice")["secret"])
	s.Equal(1, r.calls, "restoration never rolls")
	afterBob, err := enc.Story(&encounter.StoryInput{Audience: "bob"})
	s.Require().NoError(err)
	s.Equal(beforeBob, afterBob)
	s.Empty(s.memory(enc, "bob"))
	copy := s.memory(enc, "alice")
	delete(copy, "secret")
	s.True(s.memory(enc, "alice")["secret"].Learned, "exported memory is detached")
}

func (s *AutomaticDiscoverySuite) TestNegativeRetainedCountsRefuseAtSetupJoinAndRestore() {
	negative := map[encounter.ConcealmentID]encounter.DiscoveryMemoryData{"secret": {Used: -1}}
	for _, via := range []string{"setup", "join", "restore"} {
		s.Run(via, func() {
			r := &discoveryRoller{}
			in := discoveryFixture(r)
			if via == "setup" {
				in.Members[0].RetainedDiscoveries = negative
				enc, err := encounter.NewEncounter(in)
				s.ErrorIs(err, encounter.ErrInvalidData)
				s.Nil(enc)
			} else {
				enc := s.create(in)
				if via == "join" {
					out, err := enc.Join(&encounter.JoinInput{
						Member: "incoming", Kind: encounter.KindPlayer, Cell: spatial.Position{X: 3}, RetainedDiscoveries: negative,
					})
					s.ErrorIs(err, encounter.ErrInvalidData)
					s.Nil(out)
				} else {
					out, err := enc.RestoreDiscovery(&encounter.RestoreDiscoveryInput{Member: "alice", Checks: negative})
					s.ErrorIs(err, encounter.ErrInvalidData)
					s.Nil(out)
				}
				// Mutation-phase errors require discarding the instance, not reuse.
			}
			s.Zero(r.calls)
		})
	}
}

func (s *AutomaticDiscoverySuite) TestRunAndCharacterLifetimes() {
	for _, lifetime := range []encounter.DiscoveryLifetime{encounter.DiscoveryLifetimeCharacter, encounter.DiscoveryLifetimeRun} {
		s.Run(string(lifetime), func() {
			r := &discoveryRoller{}
			in := discoveryFixture(r)
			in.Field.Concealments[0].Attempts = &encounter.DiscoveryPolicyInput{Lifetime: &lifetime}
			in.Members[0].Position = spatial.Position{X: 3}
			in.Members[0].RetainedDiscoveries = map[encounter.ConcealmentID]encounter.DiscoveryMemoryData{
				"secret": {Used: 1, Learned: true},
			}
			enc := s.create(in)
			if lifetime == encounter.DiscoveryLifetimeCharacter {
				s.Zero(r.calls)
				s.Equal(encounter.DiscoveryMemoryData{Used: 1, Learned: true}, s.memory(enc, "alice")["secret"])
			} else {
				s.Equal(1, r.calls, "a new run does not inherit an old run's allowance or discovery")
				s.Equal(1, enc.ToData().Discovery["alice"].Attempts["secret"].Used)
				s.Empty(s.memory(enc, "alice"), "run-only attempts are not exported to the character profile")
				loaded := s.reloadDiscovery(enc, r)
				s.walk(loaded, 2, 1, 2, 3)
				s.Equal(1, r.calls, "reloading this run does not reset its spent attempt")
				fresh := discoveryFixture(r)
				fresh.Field.Concealments[0].Attempts = &encounter.DiscoveryPolicyInput{Lifetime: &lifetime}
				fresh.Members[0].Position = spatial.Position{X: 3}
				fresh.Members[0].RetainedDiscoveries = s.memory(loaded, "alice")
				_ = s.create(fresh)
				s.Equal(2, r.calls, "a genuinely new run receives its own allowance")
			}
		})
	}
}

func (s *AutomaticDiscoverySuite) TestOnlyPlayersCanExportOrRestoreCharacterMemory() {
	in := discoveryFixture(&discoveryRoller{})
	in.Field.Props = nil
	in.Field.Concealments[0].Props = nil
	in.Field.Concealments[0].Cells = []spatial.Position{{X: 4}}
	in.Field.Dispositions = []encounter.DispositionInput{{
		Between: [2]encounter.FactionID{encounter.FactionParty, encounter.FactionMonsters}, Stance: encounter.StanceNeutral,
	}}
	in.Members = append(in.Members,
		encounter.MemberInput{ID: "monster", Kind: encounter.KindMonster, Position: spatial.Position{X: 4}},
		encounter.MemberInput{ID: "vendor", Kind: encounter.KindWorld, Position: spatial.Position{X: 6}},
	)
	enc := s.create(in)
	atlas, err := enc.AtlasFor("monster")
	s.Require().NoError(err)
	s.Contains(atlas.Cells, spatial.Position{X: 4}, "the monster really pierced the hidden floor by occupancy")
	for _, member := range []encounter.MemberID{"monster", "vendor"} {
		out, err := enc.DiscoveryMemory(&encounter.DiscoveryMemoryInput{Member: member})
		s.ErrorIs(err, encounter.ErrNotMember)
		s.Nil(out)
		out, err = enc.RestoreDiscovery(&encounter.RestoreDiscoveryInput{Member: member})
		s.ErrorIs(err, encounter.ErrNotMember)
		s.Nil(out)
	}
	out, err := enc.DiscoveryMemory(nil)
	s.ErrorIs(err, encounter.ErrNilInput)
	s.Nil(out)
	out, err = enc.RestoreDiscovery(nil)
	s.ErrorIs(err, encounter.ErrNilInput)
	s.Nil(out)
	out, err = enc.RestoreDiscovery(&encounter.RestoreDiscoveryInput{Member: "missing"})
	s.ErrorIs(err, encounter.ErrNotMember)
	s.Nil(out)
}
