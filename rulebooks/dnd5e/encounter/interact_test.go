// Copyright (C) 2026 Kirk Diggler
// SPDX-License-Identifier: GPL-3.0-or-later

package encounter_test

import (
	"testing"

	"github.com/stretchr/testify/suite"

	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/encounter"
)

// interact_test.go covers Interact (rpg-toolkit#1404, design.md's
// "Interaction Verb"): confirming a player can reach a placed world NPC.
// This verb answers only identity, adjacency, and visibility — it builds no
// descriptor and carries no NPC content, both of which are session's job
// one layer up.
type InteractSuite struct {
	suite.Suite
}

func TestInteractSuite(t *testing.T) {
	suite.Run(t, new(InteractSuite))
}

func (s *InteractSuite) setup(
	sight encounter.Sight, members ...encounter.MemberInput,
) (*encounter.Encounter, error) {
	return encounter.NewEncounter(&encounter.SetupInput{
		Sight: sight, Standing: everyoneStanding{}, Initiative: orderAsGiven{},
		TurnDriver: passDriver{}, Striker: passStriker{}, Announcer: quietAnnouncer{},
		Field:   worldField(),
		Members: members,
		Endings: []encounter.EndingInput{{Key: "withdrawn", Trigger: encounter.TriggerExternal{}}},
	})
}

func (s *InteractSuite) TestAdjacentPlayerCanInteractAndGetsBackTheConfirmedTargetID() {
	enc, err := s.setup(everyoneSeesTheWholeMap{},
		encounter.MemberInput{ID: alice, Kind: encounter.KindPlayer, Position: cellAt(0, 0)},
		encounter.MemberInput{ID: "vendor", Kind: encounter.KindWorld, Position: cellAt(1, 0)},
	)
	s.Require().NoError(err)

	out, err := enc.Interact(&encounter.InteractInput{Actor: alice, Target: "vendor"})
	s.Require().NoError(err)
	s.Equal(encounter.MemberID("vendor"), out.Target)
	s.NotZero(out.Seq)
}

func (s *InteractSuite) TestAdjacentButNotVisibleTargetIsRefused() {
	enc, err := s.setup(&sightList{fallback: 0},
		encounter.MemberInput{ID: alice, Kind: encounter.KindPlayer, Position: cellAt(0, 0)},
		encounter.MemberInput{ID: "vendor", Kind: encounter.KindWorld, Position: cellAt(1, 0)},
	)
	s.Require().NoError(err)

	_, err = enc.Interact(&encounter.InteractInput{Actor: alice, Target: "vendor"})
	s.Require().Error(err)
	s.ErrorIs(err, encounter.ErrNotVisible)
}

func (s *InteractSuite) TestVisibleButNonAdjacentTargetIsRefused() {
	enc, err := s.setup(everyoneSeesTheWholeMap{},
		encounter.MemberInput{ID: alice, Kind: encounter.KindPlayer, Position: cellAt(0, 0)},
		encounter.MemberInput{ID: "vendor", Kind: encounter.KindWorld, Position: cellAt(3, 3)},
	)
	s.Require().NoError(err)

	_, err = enc.Interact(&encounter.InteractInput{Actor: alice, Target: "vendor"})
	s.Require().Error(err)
	s.ErrorIs(err, encounter.ErrOutOfRange)
}

// TestDistantPlayerIsRefused is TestVisibleButNonAdjacentTargetIsRefused's
// case again, named for the design doc's own wording: a target outside the
// configured range refuses regardless of how far outside it stands.
func (s *InteractSuite) TestDistantPlayerIsRefused() {
	enc, err := s.setup(everyoneSeesTheWholeMap{},
		encounter.MemberInput{ID: alice, Kind: encounter.KindPlayer, Position: cellAt(0, 0)},
		encounter.MemberInput{ID: "vendor", Kind: encounter.KindWorld, Position: cellAt(3, 0)},
	)
	s.Require().NoError(err)

	_, err = enc.Interact(&encounter.InteractInput{Actor: alice, Target: "vendor"})
	s.Require().Error(err)
	s.ErrorIs(err, encounter.ErrOutOfRange)
}

func (s *InteractSuite) TestNonPlayerActorIsRefused() {
	enc, err := s.setup(everyoneSeesTheWholeMap{},
		encounter.MemberInput{
			ID: goblin, Kind: encounter.KindMonster, Position: cellAt(0, 0), Decider: &simpleDecider{},
		},
		encounter.MemberInput{ID: "vendor", Kind: encounter.KindWorld, Position: cellAt(1, 0)},
	)
	s.Require().NoError(err)

	_, err = enc.Interact(&encounter.InteractInput{Actor: goblin, Target: "vendor"})
	s.Require().Error(err)
	s.ErrorIs(err, encounter.ErrNotMember)
}

func (s *InteractSuite) TestMonsterTargetIsRefused() {
	enc, err := s.setup(everyoneSeesTheWholeMap{},
		encounter.MemberInput{ID: alice, Kind: encounter.KindPlayer, Position: cellAt(0, 0)},
		encounter.MemberInput{
			ID: goblin, Kind: encounter.KindMonster, Position: cellAt(1, 0), Decider: &simpleDecider{},
		},
	)
	s.Require().NoError(err)

	_, err = enc.Interact(&encounter.InteractInput{Actor: alice, Target: goblin})
	s.Require().Error(err)
	s.ErrorIs(err, encounter.ErrNotMember)
}

func (s *InteractSuite) TestClosedEncounterRefusesInteract() {
	enc, err := s.setup(everyoneSeesTheWholeMap{},
		encounter.MemberInput{ID: alice, Kind: encounter.KindPlayer, Position: cellAt(0, 0)},
		encounter.MemberInput{ID: "vendor", Kind: encounter.KindWorld, Position: cellAt(1, 0)},
	)
	s.Require().NoError(err)

	_, err = enc.End(&encounter.EndInput{Ending: "withdrawn"})
	s.Require().NoError(err)

	_, err = enc.Interact(&encounter.InteractInput{Actor: alice, Target: "vendor"})
	s.Require().Error(err)
	s.ErrorIs(err, encounter.ErrClosed)
}

func (s *InteractSuite) TestNilInputRejected() {
	enc, err := s.setup(everyoneSeesTheWholeMap{},
		encounter.MemberInput{ID: alice, Kind: encounter.KindPlayer, Position: cellAt(0, 0)},
	)
	s.Require().NoError(err)

	_, err = enc.Interact(nil)
	s.Require().Error(err)
	s.ErrorIs(err, encounter.ErrNilInput)
}

func (s *InteractSuite) TestEmptyActorOrTargetRejected() {
	enc, err := s.setup(everyoneSeesTheWholeMap{},
		encounter.MemberInput{ID: alice, Kind: encounter.KindPlayer, Position: cellAt(0, 0)},
	)
	s.Require().NoError(err)

	_, err = enc.Interact(&encounter.InteractInput{Actor: "", Target: "vendor"})
	s.Require().Error(err)
	s.ErrorIs(err, encounter.ErrNoMember)

	_, err = enc.Interact(&encounter.InteractInput{Actor: alice, Target: ""})
	s.Require().Error(err)
	s.ErrorIs(err, encounter.ErrNoMember)
}

// A custom Range widens what counts as adjacent.
func (s *InteractSuite) TestACustomRangeAllowsAFartherTarget() {
	enc, err := s.setup(everyoneSeesTheWholeMap{},
		encounter.MemberInput{ID: alice, Kind: encounter.KindPlayer, Position: cellAt(0, 0)},
		encounter.MemberInput{ID: "vendor", Kind: encounter.KindWorld, Position: cellAt(2, 0)},
	)
	s.Require().NoError(err)

	out, err := enc.Interact(&encounter.InteractInput{Actor: alice, Target: "vendor", Range: 2})
	s.Require().NoError(err)
	s.Equal(encounter.MemberID("vendor"), out.Target)
}
