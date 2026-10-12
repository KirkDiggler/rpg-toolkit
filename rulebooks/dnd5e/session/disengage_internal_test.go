// Copyright (C) 2026 Kirk Diggler
// SPDX-License-Identifier: GPL-3.0-or-later

package session

import (
	"context"
	"testing"

	"github.com/stretchr/testify/suite"

	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/character"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/conditions"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/encounter"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/encounter/dungeonspec"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/refs"
)

// DisengageSeamSuite is the session's half of a monster's bonus-action
// Disengage (rpg-toolkit#2032, step 3): the composition decides when the
// bonus action is spent and tells the story; this seam runs resolution's
// driven activation, saves the monster sheet it changed, and tells nothing.
type DisengageSeamSuite struct {
	suite.Suite

	mgr   *Manager
	scope *writeScope
}

func TestDisengageSeamSuite(t *testing.T) {
	suite.Run(t, new(DisengageSeamSuite))
}

func (s *DisengageSeamSuite) SetupTest() {
	ctx := context.Background()
	sessions := &strikeSessions{byID: map[string]*SessionData{}}
	encounters := &strikeEncounters{byID: map[string]*encounter.EncounterData{}}
	characters := &strikeCharacters{byID: map[string]*character.Data{
		"fighter": strikeFixtureFighter("fighter"),
	}}
	mgr, err := NewManager(&Config{Seats: newFakeSeats(), Explorations: newFakeExplorations(),
		PresentationIDs: testPresentationIDs{}, Dice: &scriptedDice{rolls: []int{10, 10, 10, 10, 10, 10}},
		TurnDriver: Pass{}, Sessions: sessions, Encounters: encounters,
		Characters: characters, Events: DiscardEvents{},
	})
	s.Require().NoError(err)
	s.mgr = mgr

	launchScene(s.T(), mgr, scene{
		Field: encounter.FieldInput{
			Canvas:  pointyCanvas(),
			Regions: []encounter.RegionInput{rectRegion("tomb", 0, 0, 12, 6)},
		},
		Party:    []sceneSeat{seatAt("fighter", 2, 0)},
		Monsters: []dungeonspec.MonsterPlacement{monsterAt("goblin", refs.Monsters.Goblin().String(), 3, 0)},
		Endings:  []encounter.EndingInput{{Key: "withdraw", Trigger: encounter.TriggerExternal{}}},
	})

	s.scope, err = mgr.openForWrite(ctx, "sess")
	s.Require().NoError(err)
}

// disengage asks the seam exactly as the composition does.
func (s *DisengageSeamSuite) disengage(member encounter.MemberID) error {
	return (strikerSeam{m: s.mgr, scope: s.scope}).Disengage(context.Background(), s.scope.enc, member)
}

// disengagingBlobs counts the Disengaging conditions the goblin's stored
// sheet holds right now.
func (s *DisengageSeamSuite) disengagingBlobs() int {
	sheet, found := npcSheet(s.scope.data, "goblin")
	s.Require().True(found)
	held, err := conditions.HeldAddresses("goblin", sheet.Conditions)
	s.Require().NoError(err)
	count := 0
	for _, address := range held {
		if address.ConditionRef == refs.Conditions.Disengaging().String() {
			count++
		}
	}
	return count
}

// TestDisengageSeatsTheConditionOnTheMonstersSheet: the same Disengaging
// condition a player's Disengage leaves, landed by resolution on the goblin's
// stored sheet and marked for the commit, and nothing told — the composition
// writes the beat.
func (s *DisengageSeamSuite) TestDisengageSeatsTheConditionOnTheMonstersSheet() {
	before, err := s.scope.enc.Story(&encounter.StoryInput{Audience: "fighter"})
	s.Require().NoError(err)
	s.Require().Zero(s.disengagingBlobs(), "precondition: not disengaging yet")

	s.Require().NoError(s.disengage("goblin"))

	s.Equal(1, s.disengagingBlobs(), "the goblin's sheet now holds Disengaging")
	s.True(s.scope.touched, "the changed sheet is persisted with the verb's commit")

	after, err := s.scope.enc.Story(&encounter.StoryInput{Audience: "fighter"})
	s.Require().NoError(err)
	s.Len(after, len(before), "the seam records nothing; the composition tells the story")
}

// TestASecondDisengageDoesNotStack: one condition per identity, and the rule
// is resolution's, not this seam's. The driven activation lands through
// replaceSameAddress (rpg-toolkit#1946), so a second Disengage replaces the
// first — exactly as a player's second Disengage does — and the sheet still
// holds one.
func (s *DisengageSeamSuite) TestASecondDisengageDoesNotStack() {
	s.Require().NoError(s.disengage("goblin"))
	s.Require().NoError(s.disengage("goblin"))

	s.Equal(1, s.disengagingBlobs())
}

// TestDisengageRefusesAStrangerAndAPlayer: an error is a malfunction that
// aborts the caller's verb. Nobody by that name is ErrNoMember; a player has
// no stat block for this seam to write, so it is ErrNoSheet — a player's
// Disengage is its own Activate.
func (s *DisengageSeamSuite) TestDisengageRefusesAStrangerAndAPlayer() {
	s.ErrorIs(s.disengage("nobody"), ErrNoMember)
	s.ErrorIs(s.disengage("fighter"), ErrNoSheet)
	s.Zero(s.disengagingBlobs())
	s.False(s.scope.touched, "a refused ask writes nothing")
}
