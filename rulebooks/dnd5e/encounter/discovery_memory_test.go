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
		Capabilities: encounter.Capabilities{
			Sight:         everyoneSeesTheWholeMap{},
			Equipment:     encounter.UnobservedEquipment{},
			Conditions:    encounter.UnobservedEquipment{},
			Sheets:        zeroSheets{},
			Participation: everyoneStanding{},
			Initiative:    orderAsGiven{},
			Driver:        passDriver{},
			CheckResolver: resolver,
			Witness:       nobodyPerceives{},
			Actors: encounter.Actors{
				Striker:   passStriker{},
				Mover:     quietMover{},
				Announcer: quietAnnouncer{},
			},
		},
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
		Data: data,
		Capabilities: encounter.Capabilities{
			Sight:         everyoneSeesTheWholeMap{},
			Equipment:     encounter.UnobservedEquipment{},
			Conditions:    encounter.UnobservedEquipment{},
			Sheets:        zeroSheets{},
			Participation: everyoneStanding{},
			Initiative:    orderAsGiven{},
			Driver:        passDriver{},
			CheckResolver: resolver,
			Witness:       nobodyPerceives{},
			Actors: encounter.Actors{
				Striker:   passStriker{},
				Mover:     quietMover{},
				Announcer: quietAnnouncer{},
			},
		},
	})
	s.Require().NoError(err)
	return loaded
}

func (s *AutomaticDiscoverySuite) walk(enc *encounter.Encounter, xs ...int) {
	for _, x := range xs {
		s.step(enc, x)
	}
}

func (s *AutomaticDiscoverySuite) TestRestoreSetsThePreferenceAndNothingElse() {
	r := &discoveryRoller{}
	enc := s.world(true, 3, r)
	s.walk(enc, 1, 2, 3)
	s.Equal(1, r.calls)
	s.False(enc.ToData().Discovery["alice"].Attempts["secret"].Armed)
	beforeBob, err := enc.Story(&encounter.StoryInput{Audience: "bob"})
	s.Require().NoError(err)
	usedBefore := enc.ToData().Discovery["alice"].Attempts["secret"].Used

	out, err := enc.RestoreDiscovery(&encounter.RestoreDiscoveryInput{Member: "alice", Private: false})
	s.Require().NoError(err)
	s.False(out.Private)
	out, err = enc.RestoreDiscovery(&encounter.RestoreDiscoveryInput{Member: "alice", Private: true})
	s.Require().NoError(err)
	s.True(out.Private)

	s.Equal(usedBefore, enc.ToData().Discovery["alice"].Attempts["secret"].Used, "restoring a preference spends no attempt")
	s.False(enc.ToData().Discovery["alice"].Attempts["secret"].Armed, "same-visit restoration does not re-arm")
	s.Equal(1, r.calls, "restoration never rolls")
	afterBob, err := enc.Story(&encounter.StoryInput{Audience: "bob"})
	s.Require().NoError(err)
	s.Equal(beforeBob, afterBob, "restoration tells nobody anything")
	sharing, err := enc.DiscoverySharing(&encounter.DiscoverySharingInput{Member: "alice"})
	s.Require().NoError(err)
	s.False(sharing.Sharing, "the restored preference is the one now in effect")
}

func (s *AutomaticDiscoverySuite) TestOnlyPlayersCanRestoreTheirPreference() {
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
		out, err := enc.RestoreDiscovery(&encounter.RestoreDiscoveryInput{Member: member})
		s.ErrorIs(err, encounter.ErrNotMember)
		s.Nil(out)
	}
	out, err := enc.RestoreDiscovery(nil)
	s.ErrorIs(err, encounter.ErrNilInput)
	s.Nil(out)
	out, err = enc.RestoreDiscovery(&encounter.RestoreDiscoveryInput{Member: "missing"})
	s.ErrorIs(err, encounter.ErrNotMember)
	s.Nil(out)

	sharing, err := enc.DiscoverySharing(nil)
	s.ErrorIs(err, encounter.ErrNilInput)
	s.Nil(sharing)
	sharing, err = enc.DiscoverySharing(&encounter.DiscoverySharingInput{Member: "missing"})
	s.ErrorIs(err, encounter.ErrNotMember)
	s.Nil(sharing)
}
