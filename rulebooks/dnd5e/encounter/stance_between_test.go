// Copyright (C) 2026 Kirk Diggler
// SPDX-License-Identifier: GPL-3.0-or-later

package encounter_test

// stance_between_test.go is THE AUTHORITATIVE STANCE BETWEEN TWO MEMBERS
// (rpg-project#520, R5): one word per pair, the read an execution frame takes
// its relationship from. A member in no faction has NO stance — never a
// neutral one — and neither does somebody who is not here.

import (
	"testing"

	"github.com/stretchr/testify/suite"

	"github.com/KirkDiggler/rpg-toolkit/tools/spatial"

	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/encounter"
)

type StanceBetweenTestSuite struct {
	suite.Suite
}

func TestStanceBetweenSuite(t *testing.T) {
	suite.Run(t, new(StanceBetweenTestSuite))
}

// court is three declared factions beside the party — goblins neutral to it,
// bandits hostile, guards allied — and a world NPC in no faction, so every
// stance a member pair can hold is reachable.
func (s *StanceBetweenTestSuite) court() *encounter.Encounter {
	enc, err := encounter.NewEncounter(&encounter.SetupInput{
		Field: encounter.FieldInput{
			Canvas:  openAir(),
			Regions: []encounter.RegionInput{rectRegion("court", 0, 0, 10, 8)},
			Factions: []encounter.FactionInput{
				{ID: "goblins", Mind: goblin},
				{ID: "bandits", Mind: "bandit"},
				{ID: "guards", Mind: "guard"},
			},
			Dispositions: []encounter.DispositionInput{
				{Between: [2]encounter.FactionID{"goblins", encounter.FactionParty}, Stance: encounter.StanceNeutral},
				{Between: [2]encounter.FactionID{"bandits", encounter.FactionParty}, Stance: encounter.StanceHostile},
				{Between: [2]encounter.FactionID{"guards", encounter.FactionParty}, Stance: encounter.StanceAllied},
				{Between: [2]encounter.FactionID{"guards", "bandits"}, Stance: encounter.StanceHostile},
			},
		},
		Members: []encounter.MemberInput{
			{ID: alice, Kind: encounter.KindPlayer, Position: spatial.Position{X: 0, Y: 1}},
			{ID: billy, Kind: encounter.KindPlayer, Position: spatial.Position{X: 1, Y: 1}},
			{ID: goblin, Kind: encounter.KindMonster, Position: spatial.Position{X: 3, Y: 1}, Faction: "goblins"},
			{ID: "bandit", Kind: encounter.KindMonster, Position: spatial.Position{X: 5, Y: 1}, Faction: "bandits"},
			{ID: "guard", Kind: encounter.KindMonster, Position: spatial.Position{X: 7, Y: 1}, Faction: "guards"},
			{ID: "innkeeper", Kind: encounter.KindWorld, Position: spatial.Position{X: 9, Y: 1}},
		},
		Endings: []encounter.EndingInput{{Key: "withdrawn", Trigger: encounter.TriggerExternal{}}},
		Capabilities: encounter.Capabilities{
			Sight:      everyoneSeesTheWholeMap{},
			Equipment:  encounter.UnobservedEquipment{},
			Sheets:     zeroSheets{},
			Standing:   everyoneStanding{},
			Initiative: orderAsGiven{},
			Driver:     passDriver{},
			Actors: encounter.Actors{
				Striker:   passStriker{},
				Mover:     quietMover{},
				Announcer: quietAnnouncer{},
			},
		},
	})
	s.Require().NoError(err)

	return enc
}

// A member in no faction has no side: StanceBetween answers StanceNone
// toward every other member, in both directions and toward itself. Neutral is
// a real disposition a fact can turn hostile or allied; no side is not it,
// and it is an answer, not an absence a reader rebuilds.
func (s *StanceBetweenTestSuite) TestStanceBetweenFactionlessIsNoSide() {
	enc := s.court()

	members, err := enc.Members()
	s.Require().NoError(err)
	for _, m := range members {
		for _, pair := range [][2]encounter.MemberID{{m.ID, "innkeeper"}, {"innkeeper", m.ID}} {
			got, err := enc.StanceBetween(pair[0], pair[1])
			s.Require().NoError(err, "%s → %s", pair[0], pair[1])
			s.Equal(encounter.StanceNone, got, "%s → %s", pair[0], pair[1])
		}
	}
}

// A pair naming somebody who is not here is refused as not a member, and an
// empty id as an empty id: there is no stance to answer about a stranger.
func (s *StanceBetweenTestSuite) TestStanceBetweenRefusesANonMember() {
	enc := s.court()

	for _, pair := range [][2]encounter.MemberID{
		{alice, "nobody"},
		{"nobody", alice},
		{"nobody", "innkeeper"},
	} {
		got, err := enc.StanceBetween(pair[0], pair[1])
		s.Require().ErrorIs(err, encounter.ErrNotMember, "%s → %s", pair[0], pair[1])
		s.NotErrorIs(err, encounter.ErrNoMember, "%s → %s", pair[0], pair[1])
		s.Empty(got, "%s → %s", pair[0], pair[1])
	}
	_, err := enc.StanceBetween(alice, "")
	s.Require().ErrorIs(err, encounter.ErrNoMember)
}

// For every ordered pair of members the one word and the two booleans agree:
// hostile exactly when IsHostile, allied exactly when IsAllied. A pair with
// the world NPC in it is StanceNone, and the booleans still answer known
// false — "are they my enemy" and "on my side" each have a correct no.
func (s *StanceBetweenTestSuite) TestStanceBetweenAgreesWithIsHostileAndIsAllied() {
	enc := s.court()

	want := map[[2]encounter.MemberID]encounter.Stance{
		{alice, billy}:      encounter.StanceAllied,
		{alice, goblin}:     encounter.StanceNeutral,
		{alice, "bandit"}:   encounter.StanceHostile,
		{alice, "guard"}:    encounter.StanceAllied,
		{"guard", "bandit"}: encounter.StanceHostile,
		{goblin, "bandit"}:  encounter.StanceNeutral,
	}

	members, err := enc.Members()
	s.Require().NoError(err)
	for _, a := range members {
		for _, b := range members {
			got, err := enc.StanceBetween(a.ID, b.ID)
			s.Require().NoError(err, "%s → %s", a.ID, b.ID)
			factionless := a.ID == "innkeeper" || b.ID == "innkeeper"
			s.Equal(factionless, got == encounter.StanceNone, "%s → %s no side", a.ID, b.ID)

			hostile, hKnown := enc.IsHostile(a.ID, b.ID)
			allied, aKnown := enc.IsAllied(a.ID, b.ID)
			s.True(hKnown, "%s → %s IsHostile known", a.ID, b.ID)
			s.True(aKnown, "%s → %s IsAllied known", a.ID, b.ID)
			s.Equal(hostile, got == encounter.StanceHostile, "%s → %s hostile", a.ID, b.ID)
			s.Equal(allied, got == encounter.StanceAllied, "%s → %s allied", a.ID, b.ID)

			for _, pair := range [][2]encounter.MemberID{{a.ID, b.ID}, {b.ID, a.ID}} {
				if w, ok := want[pair]; ok {
					s.Equal(w, got, "%s → %s", a.ID, b.ID)
				}
			}
		}
	}
}

// With every member sighted and no deception in play, what a member believes
// about another's side is the authoritative stance — including no side, for a
// pair with the world NPC in it.
func (s *StanceBetweenTestSuite) TestStanceBetweenAgreesWithBelievedStance() {
	enc := s.court()

	members, err := enc.Members()
	s.Require().NoError(err)
	for _, a := range members {
		for _, b := range members {
			truth, err := enc.StanceBetween(a.ID, b.ID)
			s.Require().NoError(err)
			belief, err := enc.BelievedStance(a.ID, b.ID)
			s.Require().NoError(err)
			s.Equal(truth, belief, "%s → %s", a.ID, b.ID)
		}
	}
}
