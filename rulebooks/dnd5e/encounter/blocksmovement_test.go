// Copyright (C) 2026 Kirk Diggler
// SPDX-License-Identifier: GPL-3.0-or-later

package encounter_test

import (
	"testing"

	"github.com/stretchr/testify/suite"

	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/encounter"
)

// blocksmovement_test.go covers MemberInput/JoinInput.BlocksMovement
// (rpg-toolkit#1434): a bare fact, the same species as SpeedFeet and
// SightFeet, that a member may set to refuse a later arrival on its cell.
// No member kind blocked movement before this field existed
// (memberEntity.BlocksMovement() was hardcoded false for everyone); this
// file proves the new field actually changes canvas behavior in both
// directions, rather than trusting the wiring by inspection — co-location of
// two non-blocking members is verified to genuinely succeed at the
// tools/spatial layer, not assumed.
type BlocksMovementSuite struct {
	suite.Suite
}

func TestBlocksMovementSuite(t *testing.T) {
	suite.Run(t, new(BlocksMovementSuite))
}

func (s *BlocksMovementSuite) setup(members ...encounter.MemberInput) (*encounter.Encounter, error) {
	return encounter.NewEncounter(&encounter.SetupInput{
		Sight: everyoneSeesTheWholeMap{}, Standing: everyoneStanding{}, Initiative: orderAsGiven{},
		TurnDriver: passDriver{}, Striker: passStriker{}, Announcer: quietAnnouncer{},
		Field:   worldField(),
		Members: members,
		Endings: []encounter.EndingInput{{Key: "withdrawn", Trigger: encounter.TriggerExternal{}}},
	})
}

// --- A blocking member refuses a later arrival, from both placement paths ---

func (s *BlocksMovementSuite) TestAMemberPlacedAtSetupWithBlocksMovementRefusesALaterJoinOntoItsCell() {
	enc, err := s.setup(encounter.MemberInput{
		ID: "vendor", Kind: encounter.KindWorld, Position: cellAt(1, 1), BlocksMovement: true,
	})
	s.Require().NoError(err)

	_, err = enc.Join(&encounter.JoinInput{Member: alice, Kind: encounter.KindPlayer, Cell: cellAt(1, 1)})
	s.Require().Error(err)
	s.ErrorIs(err, encounter.ErrBadPlacement)
}

func (s *BlocksMovementSuite) TestAMemberPlacedMidSceneWithBlocksMovementRefusesALaterJoinOntoItsCell() {
	enc, err := s.setup()
	s.Require().NoError(err)

	_, err = enc.Join(&encounter.JoinInput{
		Member: "vendor", Kind: encounter.KindWorld, Cell: cellAt(1, 1), BlocksMovement: true,
	})
	s.Require().NoError(err)

	_, err = enc.Join(&encounter.JoinInput{Member: alice, Kind: encounter.KindPlayer, Cell: cellAt(1, 1)})
	s.Require().Error(err)
	s.ErrorIs(err, encounter.ErrBadPlacement)
}

// --- A non-blocking member genuinely allows co-location — not merely "no test caught a refusal" ---

func (s *BlocksMovementSuite) TestAMemberWithBlocksMovementFalseDoesNotBlock() {
	enc, err := s.setup(encounter.MemberInput{
		ID: "vendor", Kind: encounter.KindWorld, Position: cellAt(1, 1), BlocksMovement: false,
	})
	s.Require().NoError(err)

	_, err = enc.Join(&encounter.JoinInput{Member: alice, Kind: encounter.KindPlayer, Cell: cellAt(1, 1)})
	s.Require().NoError(err, "a non-blocking member must genuinely allow co-location, not merely fail to refuse it for some other reason")
}

// --- Regression: existing player/monster fixtures never set this field, and must not start blocking now ---

func (s *BlocksMovementSuite) TestAnExistingPlayerWithNoBlocksMovementSetStillDoesNotBlock() {
	enc, err := s.setup(encounter.MemberInput{ID: alice, Kind: encounter.KindPlayer, Position: cellAt(1, 1)})
	s.Require().NoError(err)

	_, err = enc.Join(&encounter.JoinInput{Member: goblin, Kind: encounter.KindMonster, Cell: cellAt(1, 1)})
	s.Require().NoError(err, "a player who never set BlocksMovement must not start blocking just because the field now exists")
}

func (s *BlocksMovementSuite) TestAnExistingMonsterWithNoBlocksMovementSetStillDoesNotBlock() {
	enc, err := s.setup(encounter.MemberInput{
		ID: goblin, Kind: encounter.KindMonster, Position: cellAt(1, 1), Decider: &simpleDecider{},
	})
	s.Require().NoError(err)

	_, err = enc.Join(&encounter.JoinInput{Member: alice, Kind: encounter.KindPlayer, Cell: cellAt(1, 1)})
	s.Require().NoError(err, "a monster who never set BlocksMovement must not start blocking just because the field now exists")
}

// --- Persistence: the blocking fact survives a save/load round trip, and keeps blocking afterward ---

func (s *BlocksMovementSuite) TestBlocksMovementSurvivesPersistenceAndStillBlocksAfterReload() {
	enc, err := s.setup(encounter.MemberInput{
		ID: "vendor", Kind: encounter.KindWorld, Position: cellAt(1, 1), BlocksMovement: true,
	})
	s.Require().NoError(err)

	data := enc.ToData()
	s.Require().Len(data.Members, 1)
	s.True(data.Members[0].BlocksMovement, "the persisted blob must carry the blocking fact forward")

	reloaded, err := encounter.LoadEncounter(&encounter.LoadEncounterInput{
		Data:  data,
		Sight: everyoneSeesTheWholeMap{}, Standing: everyoneStanding{}, Initiative: orderAsGiven{},
		TurnDriver: passDriver{}, Striker: passStriker{}, Announcer: quietAnnouncer{},
	})
	s.Require().NoError(err)

	members, err := reloaded.Members()
	s.Require().NoError(err)
	s.Require().Len(members, 1)
	s.True(members[0].BlocksMovement, "the reloaded read shape must still report blocking")

	// The reconstructed canvas entity must actually refuse an arrival, not
	// merely report the flag correctly — the same distinction the two live
	// tests above draw.
	_, err = reloaded.Join(&encounter.JoinInput{Member: alice, Kind: encounter.KindPlayer, Cell: cellAt(1, 1)})
	s.Require().Error(err)
	s.ErrorIs(err, encounter.ErrBadPlacement)
}

// TestBlocksMovementSurvivesPersistenceWhenFalse pins the other direction of
// the same round trip: a non-blocking member must not start blocking after a
// reload either.
func (s *BlocksMovementSuite) TestBlocksMovementSurvivesPersistenceWhenFalse() {
	enc, err := s.setup(encounter.MemberInput{
		ID: "vendor", Kind: encounter.KindWorld, Position: cellAt(1, 1), BlocksMovement: false,
	})
	s.Require().NoError(err)

	data := enc.ToData()
	s.Require().Len(data.Members, 1)
	s.False(data.Members[0].BlocksMovement)

	reloaded, err := encounter.LoadEncounter(&encounter.LoadEncounterInput{
		Data:  data,
		Sight: everyoneSeesTheWholeMap{}, Standing: everyoneStanding{}, Initiative: orderAsGiven{},
		TurnDriver: passDriver{}, Striker: passStriker{}, Announcer: quietAnnouncer{},
	})
	s.Require().NoError(err)

	_, err = reloaded.Join(&encounter.JoinInput{Member: alice, Kind: encounter.KindPlayer, Cell: cellAt(1, 1)})
	s.Require().NoError(err)
}
