// Copyright (C) 2026 Kirk Diggler
// SPDX-License-Identifier: GPL-3.0-or-later

package encounter_test

import (
	"testing"

	"github.com/stretchr/testify/suite"

	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/encounter"
)

// standins_test.go covers rpg-toolkit#1956: a host builds an empty world from
// this module's own stand-ins, without restating its capability list.

type standInsSuite struct {
	suite.Suite
}

func TestStandInsSuite(t *testing.T) {
	suite.Run(t, new(standInsSuite))
}

// emptyWorld is what a host writes for a world nobody is in yet: one call.
func emptyWorld(members ...encounter.MemberInput) *encounter.SetupInput {
	setup := encounter.CompileOnlySetup(
		encounter.FieldInput{Canvas: pointyCanvas(), Regions: []encounter.RegionInput{rectRegion("yard", 0, 0, 6, 1)}},
		[]encounter.EndingInput{{Key: "withdrawn", Trigger: encounter.TriggerExternal{}}},
	)
	setup.Members = members
	return setup
}

func (s *standInsSuite) TestNewEncounterConstructsFromCompileOnlySetup() {
	enc, err := encounter.NewEncounter(emptyWorld())
	s.Require().NoError(err)
	s.NotNil(enc)

	s.Run("and with members placed, nobody is seen", func() {
		enc, err := encounter.NewEncounter(emptyWorld(
			encounter.MemberInput{ID: alice, Kind: encounter.KindPlayer, Position: cellAt(0, 0)},
			encounter.MemberInput{ID: goblin, Kind: encounter.KindMonster, Position: cellAt(2, 0)},
		))
		s.Require().NoError(err)
		out, err := enc.ObservedContext(&encounter.ViewInput{Member: alice})
		s.Require().NoError(err)
		s.Empty(out.Members, "zero sight writes no sighting")
	})

	s.Run("a concealed door left open is witnessed at first light, and answered", func() {
		field := concealField()
		for i := range field.Doors {
			if field.Doors[i].ID == veilDoor {
				field.Doors[i].State = encounter.DoorIsOpen()
			}
		}
		enc, err := encounter.NewEncounter(encounter.CompileOnlySetup(field,
			[]encounter.EndingInput{{Key: "withdrawn", Trigger: encounter.TriggerExternal{}}}))
		s.Require().NoError(err)
		s.NotNil(enc)
	})
}

func (s *standInsSuite) TestUnobservedEquipmentAnswersNotObserved() {
	asked := []encounter.MemberID{alice, goblin}
	hands, err := encounter.UnobservedEquipment{}.Equipment(asked)
	s.Require().NoError(err)
	s.Equal(map[encounter.MemberID]*encounter.HeldEquipment{alice: nil, goblin: nil}, hands,
		"every member answered, each nil: no hands to observe, not empty hands")

	held, err := encounter.UnobservedEquipment{}.Conditions(asked)
	s.Require().NoError(err)
	s.Equal(map[encounter.MemberID]*encounter.ConditionSet{alice: nil, goblin: nil}, held,
		"every member answered, each nil: nothing observed, not an empty set")
}

// TestAStandInWorldLoadsWithRealCapabilities is the host's path: build the
// empty world from the stand-ins, then load it with the capabilities that
// play it. The loaded world behaves as any other — sightings carry hands'
// absence and the conditions the real capability reports.
func (s *standInsSuite) TestAStandInWorldLoadsWithRealCapabilities() {
	built, err := encounter.NewEncounter(emptyWorld())
	s.Require().NoError(err)

	table := &conditionTable{held: map[encounter.MemberID]*encounter.ConditionSet{
		goblin: heldSet(encounter.ConditionKey{ConditionRef: faerieFire, SourceID: clericSource}),
	}}
	loaded, err := encounter.LoadEncounter(&encounter.LoadEncounterInput{
		Data: built.ToData(), Sight: everyoneSeesTheWholeMap{}, Equipment: table, Standing: everyoneStanding{},
		Initiative: orderAsGiven{}, TurnDriver: passDriver{}, Striker: passStriker{},
		Mover: quietMover{}, Announcer: quietAnnouncer{},
	})
	s.Require().NoError(err)

	_, err = loaded.Join(&encounter.JoinInput{Member: alice, Kind: encounter.KindPlayer, Cell: cellAt(0, 0)})
	s.Require().NoError(err)
	_, err = loaded.Join(&encounter.JoinInput{Member: goblin, Kind: encounter.KindMonster, Cell: cellAt(2, 0)})
	s.Require().NoError(err)

	out, err := loaded.ObservedContext(&encounter.ViewInput{Member: alice})
	s.Require().NoError(err)
	var seen *encounter.ObservedContextMember
	for i := range out.Members {
		if out.Members[i].ID == goblin {
			seen = &out.Members[i]
		}
	}
	s.Require().NotNil(seen, "the loaded world sees with the real sight")
	s.Nil(seen.Equipment)
	s.Equal(heldSet(encounter.ConditionKey{ConditionRef: faerieFire, SourceID: clericSource}), seen.Conditions)
}
