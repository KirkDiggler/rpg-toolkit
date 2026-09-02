// Copyright (C) 2026 Kirk Diggler
// SPDX-License-Identifier: GPL-3.0-or-later

package encounter_test

import (
	"testing"

	"github.com/stretchr/testify/suite"

	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/encounter"
)

// worldnpc_test.go covers KindWorld (rpg-toolkit#1404): a placed,
// non-combatant member that carries no NPC content of its own — no ref, no
// capabilities, no policy. It is placed with exactly the bare facts any
// member already has, and its exclusion from combat is structural rather
// than something this file builds: sidesInContactOrder's switch (trigger.go)
// has no default case, so a member typed neither KindPlayer nor KindMonster
// already falls into neither side, and Pump's monster loop is filtered to
// KindMonster explicitly. These tests exist to prove that claim from the
// public API rather than trust the comment on the constant.
type WorldNPCSuite struct {
	suite.Suite
}

func TestWorldNPCSuite(t *testing.T) {
	suite.Run(t, new(WorldNPCSuite))
}

const worldRoom = "world-room"

func worldField() encounter.FieldInput {
	return encounter.FieldInput{
		Canvas:  pointyCanvas(),
		Regions: []encounter.RegionInput{rectRegion(worldRoom, 0, 0, 4, 4)},
	}
}

func (s *WorldNPCSuite) baseSetup(members ...encounter.MemberInput) (*encounter.Encounter, error) {
	return encounter.NewEncounter(&encounter.SetupInput{
		Sight: everyoneSeesTheWholeMap{}, Standing: everyoneStanding{}, Initiative: orderAsGiven{},
		TurnDriver: passDriver{}, Striker: passStriker{}, Announcer: quietAnnouncer{},
		Field:   worldField(),
		Members: members,
		Endings: []encounter.EndingInput{{Key: "withdrawn", Trigger: encounter.TriggerExternal{}}},
	})
}

// --- Placement succeeds ---

func (s *WorldNPCSuite) TestSetupAcceptsAWorldNPCWithBareFacts() {
	enc, err := s.baseSetup(encounter.MemberInput{
		ID: "vendor", Kind: encounter.KindWorld, Name: "Merchant", Position: cellAt(1, 1),
	})
	s.Require().NoError(err)

	members, err := enc.Members()
	s.Require().NoError(err)
	s.Require().Len(members, 1)
	s.Equal(encounter.KindWorld, members[0].Kind)
	s.Equal("Merchant", members[0].Name)
}

func (s *WorldNPCSuite) TestJoinMidSceneAlsoAcceptsAWorldNPC() {
	enc, err := s.baseSetup()
	s.Require().NoError(err)

	out, err := enc.Join(&encounter.JoinInput{
		Member: "vendor", Kind: encounter.KindWorld, Name: "Merchant", Cell: cellAt(2, 2),
	})
	s.Require().NoError(err)
	s.Equal(encounter.KindWorld, out.Member.Kind)
	s.Nil(out.Formed, "joining a world NPC must never form a fight")
}

// --- The one enforced rule: no decider, mirroring the player check exactly ---

func (s *WorldNPCSuite) TestSetupRejectsAWorldNPCWithADecider() {
	_, err := s.baseSetup(encounter.MemberInput{
		ID: "vendor", Kind: encounter.KindWorld, Position: cellAt(1, 1), Decider: &simpleDecider{},
	})
	s.Require().Error(err)
	s.ErrorIs(err, encounter.ErrNoMember)
}

func (s *WorldNPCSuite) TestJoinRejectsAWorldNPCWithADecider() {
	enc, err := s.baseSetup()
	s.Require().NoError(err)

	_, err = enc.Join(&encounter.JoinInput{
		Member: "vendor", Kind: encounter.KindWorld, Cell: cellAt(1, 1), Decider: &simpleDecider{},
	})
	s.Require().Error(err)
	s.ErrorIs(err, encounter.ErrNoMember)
}

func (s *WorldNPCSuite) TestPlayersAndMonstersAreUnaffectedByTheNewCheck() {
	_, err := s.baseSetup(
		encounter.MemberInput{ID: alice, Kind: encounter.KindPlayer, Position: cellAt(0, 0)},
		encounter.MemberInput{ID: goblin, Kind: encounter.KindMonster, Position: cellAt(3, 3), Decider: &simpleDecider{}},
	)
	s.Require().NoError(err)
}

// --- Combat exclusion: proven from the public API, not asserted from a comment ---

func (s *WorldNPCSuite) TestAPlayerAndAWorldNPCInMutualSightNeverFormABubble() {
	enc, err := s.baseSetup(
		encounter.MemberInput{ID: alice, Kind: encounter.KindPlayer, Position: cellAt(0, 0)},
		encounter.MemberInput{ID: "vendor", Kind: encounter.KindWorld, Position: cellAt(1, 0)},
	)
	s.Require().NoError(err)

	out, err := enc.Step(&encounter.StepInput{Member: alice, To: cellAt(0, 1)})
	s.Require().NoError(err)
	s.Nil(out.Formed)

	for _, id := range []encounter.MemberID{alice, "vendor"} {
		clk, err := enc.ClockOf(&encounter.ClockOfInput{Member: id})
		s.Require().NoError(err)
		s.Equal(encounter.ClockWorld, clk.Kind, "%s must stay on the world clock", id)
	}
}

func (s *WorldNPCSuite) TestAMonstersPumpDrivenWalkIntoAWorldNPCFormsNoFight() {
	enc, err := s.baseSetup(
		encounter.MemberInput{
			ID: goblin, Kind: encounter.KindMonster, Position: cellAt(0, 0), Decider: &simpleDecider{},
		},
		encounter.MemberInput{ID: "vendor", Kind: encounter.KindWorld, Position: cellAt(1, 0)},
	)
	s.Require().NoError(err)

	out, err := enc.Pump(&encounter.PumpInput{})
	s.Require().NoError(err)
	s.Nil(out.Formed)

	clk, err := enc.ClockOf(&encounter.ClockOfInput{Member: "vendor"})
	s.Require().NoError(err)
	s.Equal(encounter.ClockWorld, clk.Kind)
}

// TestTransferRefusesMovingAWorldNPCOntoTheTurnClock is defense-in-depth
// (Transfer has no other kind check) rather than a reachable path: classify()
// never names a KindWorld member in the first place. See Transfer's own
// comment.
func (s *WorldNPCSuite) TestTransferRefusesMovingAWorldNPCOntoTheTurnClock() {
	enc, err := s.baseSetup(
		encounter.MemberInput{
			ID: goblin, Kind: encounter.KindMonster, Position: cellAt(0, 0), Decider: &simpleDecider{},
		},
		encounter.MemberInput{ID: alice, Kind: encounter.KindPlayer, Position: cellAt(3, 3)},
		encounter.MemberInput{ID: "vendor", Kind: encounter.KindWorld, Position: cellAt(1, 0)},
	)
	s.Require().NoError(err)

	// Form a real fight between the player and the monster first — Transfer
	// to ClockTurn requires one running (ErrNoBubble otherwise), and this
	// test is about the world-NPC guard, not that precondition.
	_, err = enc.Step(&encounter.StepInput{Member: alice, To: cellAt(1, 1)})
	s.Require().NoError(err)

	_, err = enc.Transfer(&encounter.TransferInput{Member: "vendor", To: encounter.ClockTurn})
	s.Require().Error(err)
	s.ErrorIs(err, encounter.ErrNoMember)
}

// --- Persistence: nothing new to round-trip, and old blobs are untouched ---

func (s *WorldNPCSuite) TestPersistenceRoundTripsAWorldNPC() {
	enc, err := s.baseSetup(encounter.MemberInput{
		ID: "vendor", Kind: encounter.KindWorld, Name: "Merchant", Position: cellAt(1, 1),
	})
	s.Require().NoError(err)

	data := enc.ToData()

	reloaded, err := encounter.LoadEncounter(&encounter.LoadEncounterInput{
		Data:  data,
		Sight: everyoneSeesTheWholeMap{}, Standing: everyoneStanding{}, Initiative: orderAsGiven{},
		TurnDriver: passDriver{}, Striker: passStriker{}, Announcer: quietAnnouncer{},
	})
	s.Require().NoError(err)

	members, err := reloaded.Members()
	s.Require().NoError(err)
	s.Require().Len(members, 1)
	s.Equal(encounter.KindWorld, members[0].Kind)
	s.Equal("Merchant", members[0].Name)
}

func (s *WorldNPCSuite) TestOldPlayerAndMonsterBlobsStillLoadUnchanged() {
	enc, err := s.baseSetup(
		encounter.MemberInput{ID: alice, Kind: encounter.KindPlayer, Position: cellAt(0, 0)},
		encounter.MemberInput{ID: goblin, Kind: encounter.KindMonster, Position: cellAt(3, 3), Decider: &simpleDecider{}},
	)
	s.Require().NoError(err)
	data := enc.ToData()

	reloaded, err := encounter.LoadEncounter(&encounter.LoadEncounterInput{
		Data:  data,
		Sight: everyoneSeesTheWholeMap{}, Standing: everyoneStanding{}, Initiative: orderAsGiven{},
		TurnDriver: passDriver{}, Striker: passStriker{}, Announcer: quietAnnouncer{},
	})
	s.Require().NoError(err)
	members, err := reloaded.Members()
	s.Require().NoError(err)
	s.Len(members, 2)
}
