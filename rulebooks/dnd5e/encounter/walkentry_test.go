package encounter_test

import (
	"testing"

	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/encounter"
	"github.com/stretchr/testify/suite"
)

type WalkEntrySuite struct{ suite.Suite }

func TestWalkEntrySuite(t *testing.T) { suite.Run(t, new(WalkEntrySuite)) }

func (s *WalkEntrySuite) scene(reach int) *encounter.Encounter {
	enc, err := encounter.NewEncounter(&encounter.SetupInput{
		Sight:     &sightList{reach: map[encounter.MemberID]int{alice: reach}, fallback: 1},
		Equipment: noHandsAreObserved{}, Standing: everyoneStanding{},
		Initiative: orderAsGiven{}, TurnDriver: passDriver{}, Striker: passStriker{},
		Mover: quietMover{}, Announcer: quietAnnouncer{},
		Field: encounter.FieldInput{Canvas: pointyCanvas(), Regions: []encounter.RegionInput{rectRegion("corridor", 0, 0, 5, 3)}},
		Members: []encounter.MemberInput{
			{ID: alice, Kind: encounter.KindPlayer, Position: cellAt(0, 0)},
			{ID: "bob", Kind: encounter.KindPlayer, Position: cellAt(1, 0)},
			{ID: goblin, Kind: encounter.KindMonster, Position: cellAt(2, 0)},
		},
		Endings: []encounter.EndingInput{{Key: "withdrawn", Trigger: encounter.TriggerExternal{}}},
	})
	s.Require().NoError(err)
	return enc
}

func (s *WalkEntrySuite) TestJoinThroughVisibleTeammateWithoutEnteringTheirCell() {
	enc := s.scene(1)
	before, err := enc.ClockOf(&encounter.ClockOfInput{Member: alice})
	s.Require().NoError(err)
	s.Require().Equal(encounter.ClockWorld, before.Kind)
	positions, err := enc.Members()
	s.Require().NoError(err)
	joined, err := enc.AdmitWalk(&encounter.AdmitWalkInput{Member: alice})
	s.Require().NoError(err)
	s.True(joined.Joined)
	after, err := enc.ClockOf(&encounter.ClockOfInput{Member: alice})
	s.Require().NoError(err)
	s.Equal(encounter.ClockTurn, after.Kind)
	unchanged, err := enc.Members()
	s.Require().NoError(err)
	s.Equal(positions, unchanged)
	again, err := enc.AdmitWalk(&encounter.AdmitWalkInput{Member: alice})
	s.Require().NoError(err)
	s.False(again.Joined)
}

func (s *WalkEntrySuite) TestUnseenTeammateDoesNotPullWalkerIntoCombat() {
	enc := s.scene(0)
	joined, err := enc.AdmitWalk(&encounter.AdmitWalkInput{Member: alice})
	s.Require().NoError(err)
	s.False(joined.Joined)
}
