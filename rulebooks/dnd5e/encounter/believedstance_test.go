// Copyright (C) 2026 Kirk Diggler
// SPDX-License-Identifier: GPL-3.0-or-later

package encounter_test

// believedstance_test.go is WHAT A PLAYER BELIEVES ABOUT A CREATURE'S SIDE
// (rpg-project#458): the fact a per-viewer sighting carries and the ring under
// a token is coloured from. Today it equals the truth for every viewer, and
// the scenes below pin that it is asked PER VIEWER anyway — which is what
// makes `pretend` an edit here rather than a rewrite everywhere.

import (
	"testing"

	"github.com/stretchr/testify/suite"

	"github.com/KirkDiggler/rpg-toolkit/tools/spatial"

	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/encounter"
)

type BelievedStanceTestSuite struct {
	suite.Suite
}

func TestBelievedStanceSuite(t *testing.T) {
	suite.Run(t, new(BelievedStanceTestSuite))
}

// yard puts alice, a neutral goblin, a hostile bandit and a world NPC in one
// room with declared factions, so every stance a pair can hold is reachable.
func (s *BelievedStanceTestSuite) yard() *encounter.Encounter {
	enc, err := encounter.NewEncounter(&encounter.SetupInput{
		Field: encounter.FieldInput{
			Canvas:  openAir(),
			Regions: []encounter.RegionInput{rectRegion("yard", 0, 0, 8, 8)},
			Factions: []encounter.FactionInput{
				{ID: "goblins", Mind: goblin},
				{ID: "bandits", Mind: "bandit"},
			},
			Dispositions: []encounter.DispositionInput{
				{Between: [2]encounter.FactionID{"goblins", encounter.FactionParty}, Stance: encounter.StanceNeutral},
				{Between: [2]encounter.FactionID{"bandits", encounter.FactionParty}, Stance: encounter.StanceHostile},
			},
		},
		Members: []encounter.MemberInput{
			{ID: alice, Kind: encounter.KindPlayer, Position: spatial.Position{X: 0, Y: 1}},
			{ID: billy, Kind: encounter.KindPlayer, Position: spatial.Position{X: 1, Y: 1}},
			{ID: goblin, Kind: encounter.KindMonster, Position: spatial.Position{X: 3, Y: 1}, Faction: "goblins"},
			{ID: "bandit", Kind: encounter.KindMonster, Position: spatial.Position{X: 5, Y: 1}, Faction: "bandits"},
			{ID: "innkeeper", Kind: encounter.KindWorld, Position: spatial.Position{X: 7, Y: 1}},
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

// Every stance the graph can fold is the stance a viewer believes, and it is
// the same answer the two-boolean reads already give.
func (s *BelievedStanceTestSuite) TestBeliefIsTheDerivedStanceToday() {
	enc := s.yard()

	for _, tc := range []struct {
		name    string
		viewer  encounter.MemberID
		subject encounter.MemberID
		want    encounter.Stance
	}{
		{"a neutral goblin", alice, goblin, encounter.StanceNeutral},
		{"a hostile bandit", alice, "bandit", encounter.StanceHostile},
		{"a fellow player", alice, billy, encounter.StanceAllied},
		{"yourself", alice, alice, encounter.StanceAllied},
		{"the goblin looking back", goblin, alice, encounter.StanceNeutral},
		{"the bandit looking back", "bandit", alice, encounter.StanceHostile},
		{"one monster faction at another", goblin, "bandit", encounter.StanceNeutral},
	} {
		s.Run(tc.name, func() {
			got, err := enc.BelievedStance(tc.viewer, tc.subject)
			s.Require().NoError(err)
			s.Equal(tc.want, got)
		})
	}

	hostile, known := enc.IsHostile(alice, "bandit")
	s.Require().True(known)
	s.True(hostile, "and it agrees with the read resolution already uses")

	allied, known := enc.IsAllied(alice, billy)
	s.Require().True(known)
	s.True(allied)
}

// Both players see the same thing, which is the honest answer while nothing
// can lie — and the scene exists so that the day one of them sees differently
// is a change to this function and nowhere else.
func (s *BelievedStanceTestSuite) TestEveryViewerBelievesTheSameThingToday() {
	enc := s.yard()

	forAlice, err := enc.BelievedStance(alice, goblin)
	s.Require().NoError(err)
	forBilly, err := enc.BelievedStance(billy, goblin)
	s.Require().NoError(err)
	s.Equal(forAlice, forBilly, "no deception is in play, so belief is truth for both")
}

// A world NPC is in NO FACTION, so the answer is NO SIDE — StanceNone, the
// one encoding every reader takes — and never "neutral". "Nobody is against
// them" and "there is no side here to be on" are different statements: a
// client drawing a ring would otherwise paint a vendor the same colour as a
// goblin the party had a truce with.
func (s *BelievedStanceTestSuite) TestAWorldNPCIsBelievedNoSide() {
	enc := s.yard()

	got, err := enc.BelievedStance(alice, "innkeeper")
	s.Require().NoError(err)
	s.Equal(encounter.StanceNone, got)

	// And the other direction, for the same reason.
	got, err = enc.BelievedStance("innkeeper", alice)
	s.Require().NoError(err)
	s.Equal(encounter.StanceNone, got)

	// IsAllied still answers, because it asks a different question: "are they
	// on my side" has a correct false.
	allied, known := enc.IsAllied(alice, "innkeeper")
	s.True(known)
	s.False(allied)
}

// Somebody who is not here is not "neutral" and not "no side": the read is
// refused as not a member, so a caller can tell a stranger from an answer.
func (s *BelievedStanceTestSuite) TestSomebodyWhoIsNotHereIsRefused() {
	enc := s.yard()

	_, err := enc.BelievedStance(alice, "nobody")
	s.Require().ErrorIs(err, encounter.ErrNotMember)

	_, err = enc.BelievedStance("nobody", alice)
	s.Require().ErrorIs(err, encounter.ErrNotMember)
}
