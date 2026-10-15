package encounter_test

import (
	"testing"

	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/encounter"
	"github.com/stretchr/testify/suite"
)

type WalkEntrySuite struct{ suite.Suite }

func TestWalkEntrySuite(t *testing.T) { suite.Run(t, new(WalkEntrySuite)) }

func (s *WalkEntrySuite) scene(reach int) *encounter.Encounter {
	return s.sceneLit(map[encounter.MemberID]int{alice: reach})
}

// sceneLit is the corridor with the named members' sight reach set; everyone
// else sees one cell. Bob and the goblin stand adjacent and fight from first
// light; alice stands one cell behind bob, two from the goblin.
func (s *WalkEntrySuite) sceneLit(reach map[encounter.MemberID]int) *encounter.Encounter {
	enc, err := encounter.NewEncounter(&encounter.SetupInput{
		Field: encounter.FieldInput{Canvas: pointyCanvas(), Regions: []encounter.RegionInput{rectRegion("corridor", 0, 0, 5, 3)}},
		Members: []encounter.MemberInput{
			{ID: alice, Kind: encounter.KindPlayer, Position: cellAt(0, 0)},
			{ID: "bob", Kind: encounter.KindPlayer, Position: cellAt(1, 0)},
			{ID: goblin, Kind: encounter.KindMonster, Position: cellAt(2, 0)},
		},
		Endings: []encounter.EndingInput{{Key: "withdrawn", Trigger: encounter.TriggerExternal{}}},
		Capabilities: encounter.Capabilities{
			Sight:         &sightList{reach: reach, fallback: 1},
			Equipment:     encounter.UnobservedEquipment{},
			Conditions:    encounter.UnobservedEquipment{},
			Sheets:        zeroSheets{},
			Participation: everyoneStanding{},
			Initiative:    orderAsGiven{},
			Driver:        passDriver{},
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

// TestSeeingATeammateInAFightIsNotContact is the castle walk's finding
// (rpg-project#569, D0): alice sees bob, who is fighting the goblin; the
// goblin cannot see alice and she cannot see it. That is no contact — a seat
// joins a fight only when it sees a hostile or a hostile sees it — so she
// walks on the world clock. This replaced a test that pinned the opposite
// (#1910's "admit walks to observed allied combat").
func (s *WalkEntrySuite) TestSeeingATeammateInAFightIsNotContact() {
	enc := s.scene(1)
	bob, err := enc.ClockOf(&encounter.ClockOfInput{Member: "bob"})
	s.Require().NoError(err)
	s.Require().Equal(encounter.ClockTurn, bob.Kind, "precondition: bob is fighting the goblin")

	joined, err := enc.AdmitWalk(&encounter.AdmitWalkInput{Member: alice})
	s.Require().NoError(err)
	s.False(joined.Joined, "sight of an ally is not contact")
	after, err := enc.ClockOf(&encounter.ClockOfInput{Member: alice})
	s.Require().NoError(err)
	s.Equal(encounter.ClockWorld, after.Kind)
}

// TestAWatchedWalkerOutOfTheFightIsReadmitted pins AdmitWalk's own admission
// branch: a member who stepped out of the fight while an opposed member of it
// still watches her has had that contact classified already, so no
// transition re-adds her; AdmitWalk does, before her next step, without
// moving her, and a second call is a no-op.
func (s *WalkEntrySuite) TestAWatchedWalkerOutOfTheFightIsReadmitted() {
	enc := s.sceneLit(map[encounter.MemberID]int{alice: 1, goblin: 2}) // spotted from first light
	before, err := enc.Members()
	s.Require().NoError(err)
	_, err = enc.Transfer(&encounter.TransferInput{Member: alice, To: encounter.ClockWorld})
	s.Require().NoError(err)
	got, err := enc.AdmitWalk(&encounter.AdmitWalkInput{Member: alice})
	s.Require().NoError(err)
	s.True(got.Joined, "a hostile in the fight watching her is contact")
	after, err := enc.Members()
	s.Require().NoError(err)
	s.Equal(before, after, "placement unchanged")
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

func (s *WalkEntrySuite) TestVoluntaryEndpointCannotStopOnAnAlly() {
	enc := s.scene(0)
	_, err := enc.Step(&encounter.StepInput{Member: alice, To: cellAt(1, 0), EndWalk: true})
	s.Require().Error(err)
	s.ErrorIs(err, encounter.ErrBadPlacement)
	var obstruction *encounter.StepObstructedError
	s.ErrorAs(err, &obstruction)
}

func (s *WalkEntrySuite) TestCombatEntryCannotBecomeAVoluntaryEndOnAnAlly() {
	// The goblin sees two cells, so alice is in contact — spotted — and in
	// the fight from first light, first in its order.
	enc := s.sceneLit(map[encounter.MemberID]int{alice: 1, goblin: 2})
	clock, err := enc.ClockOf(&encounter.ClockOfInput{Member: alice})
	s.Require().NoError(err)
	s.Require().Equal(alice, clock.Active)
	_, err = enc.Step(&encounter.StepInput{Member: alice, To: cellAt(1, 0)})
	s.Require().NoError(err)
	_, err = enc.EndTurn(&encounter.EndTurnInput{Member: alice})
	s.Require().ErrorIs(err, encounter.ErrBadPlacement)
	_, err = enc.Step(&encounter.StepInput{Member: alice, To: cellAt(0, 0), EndWalk: true})
	s.Require().NoError(err)
	_, err = enc.EndTurn(&encounter.EndTurnInput{Member: alice})
	s.Require().NoError(err)
}

func (s *WalkEntrySuite) TestWalkEndPreflightRefusesSeenAlly() {
	enc := s.scene(1)
	err := enc.ValidateWalkEnd(encounter.CellAtInput{Mover: alice, Cell: cellAt(1, 0)})
	s.ErrorIs(err, encounter.ErrBadPlacement)
	s.NoError(enc.ValidateWalkEnd(encounter.CellAtInput{Mover: alice, Cell: cellAt(3, 0)}))
}

func (s *WalkEntrySuite) TestWalkEndPreflightDoesNotRevealUnseenOccupants() {
	enc := s.scene(0)
	// Unknown ally and hostile occupants are discovered during execution.
	s.NoError(enc.ValidateWalkEnd(encounter.CellAtInput{Mover: alice, Cell: cellAt(1, 0)}))
	s.NoError(enc.ValidateWalkEnd(encounter.CellAtInput{Mover: alice, Cell: cellAt(2, 0)}))
	s.NoError(enc.ValidateWalkEnd(encounter.CellAtInput{Mover: alice, Cell: cellAt(3, 0)}))
}
