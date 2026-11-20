// Copyright (C) 2026 Kirk Diggler
// SPDX-License-Identifier: GPL-3.0-or-later

package session_test

import (
	"testing"

	"github.com/stretchr/testify/suite"

	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/abilities"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/character"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/encounter"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/encounter/dungeonspec"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/refs"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/session"
)

// INITIATIVE ADDS THE DEXTERITY MODIFIER (rpg-project#990): a fight formed at
// launch, with dice scripted per member (members are asked in sorted ID
// order, one d20 each), ordered by d20 plus Dexterity modifier for players
// and monsters alike. Ties still order by ID, so every scene is built where
// the ID order disagrees with the rule: a zero modifier fails each one.
//
// Monster Dexterity, from the stat blocks: skeleton 14 (+2), zombie 6 (-2).
type InitiativeDexteritySuite struct{ suite.Suite }

func TestInitiativeDexteritySuite(t *testing.T) { suite.Run(t, new(InitiativeDexteritySuite)) }

// fighterWithDex is armedFighter with the given Dexterity score.
func fighterWithDex(id string, dex int) *character.Data {
	sheet := armedFighter(id)
	sheet.AbilityScores[abilities.DEX] = dex
	return sheet
}

// formed launches a hall with the party side by side and one monster in
// plain sight, the given initiative faces in sorted-ID order, and returns the
// order of the fight that forms.
func (s *InitiativeDexteritySuite) formed(party []*character.Data, monster dungeonspec.MonsterPlacement, faces ...int) []string {
	mgr, err := session.NewManager(&session.Config{Record: session.DiscardRecord{}, Acts: &fakeStream{}, Seats: newFakeSeats(),
		CharacterConfig: newFakeCharacterConfig(), PresentationIDs: testPresentationIDs{},
		Dice: &sequenceDice{rolls: faces}, TurnDriver: session.Pass{},
		Sessions: newFakeSessions(), Encounters: newFakeEncounters(),
		Characters: newFakeCharacters(party...),
	})
	s.Require().NoError(err)
	seats := make([]sceneSeat, 0, len(party))
	for i, member := range party {
		seats = append(seats, seatAt(member.ID, 1+i, 1))
	}
	launched := launchScene(s.T(), mgr, scene{
		Field: encounter.FieldInput{Canvas: pointyCanvas(),
			Regions: []encounter.RegionInput{rectRegion("hall", 0, 0, 40, 8)}},
		Party:    seats,
		Monsters: []dungeonspec.MonsterPlacement{monster},
	})
	s.Require().Len(launched.Formed, 1, "standing in plain sight starts one fight")
	return launched.Formed[0].Order
}

// Catches a modifier left at zero for characters (ID order would put a-slow
// first), or one read from the wrong ability.
func (s *InitiativeDexteritySuite) TestHigherDexterityActsFirstBetweenCharacters() {
	order := s.formed(
		[]*character.Data{fighterWithDex("a-slow", 8), fighterWithDex("b-quick", 18)},
		monsterAt("m-zombie", refs.Monsters.Zombie().String(), 6, 1),
		10, 10, 10) // a-slow, b-quick, m-zombie
	s.Equal([]string{"b-quick", "a-slow", "m-zombie"}, order, "14, 9 and 8")
}

// Catches a modifier left at zero for monsters, or read for one kind of
// member only: the skeleton beats the slow character and loses to the quick
// one, against ID order both times.
func (s *InitiativeDexteritySuite) TestHigherDexterityActsFirstBetweenACharacterAndAMonster() {
	order := s.formed(
		[]*character.Data{fighterWithDex("a-slow", 8), fighterWithDex("z-quick", 18)},
		monsterAt("m-skel", refs.Monsters.Skeleton().String(), 6, 1),
		10, 10, 10) // a-slow, m-skel, z-quick
	s.Equal([]string{"z-quick", "m-skel", "a-slow"}, order, "14, 12 and 9")
}

// Catches Dexterity used to OVERRIDE the roll rather than add to it: the
// low-Dexterity member's die beats the difference, so it still goes first.
func (s *InitiativeDexteritySuite) TestTheModifierAddsToTheRollAndDoesNotOverrideIt() {
	order := s.formed(
		[]*character.Data{fighterWithDex("a-slow", 8), fighterWithDex("b-quick", 18)},
		monsterAt("z-zombie", refs.Monsters.Zombie().String(), 6, 1),
		15, 5, 1) // a-slow, b-quick, z-zombie
	s.Equal([]string{"a-slow", "b-quick", "z-zombie"}, order, "15-1=14 beats 5+4=9")
}

// Catches a Dexterity of 10 adding something (a score read as the modifier,
// or a formula off by one): z-ten ties a-twelve only if 10 adds nothing, and
// a tie goes to the lower ID.
func (s *InitiativeDexteritySuite) TestDexterityTenAddsNothing() {
	order := s.formed(
		[]*character.Data{fighterWithDex("a-twelve", 12), fighterWithDex("z-ten", 10)},
		monsterAt("m-zombie", refs.Monsters.Zombie().String(), 6, 1),
		10, 1, 11) // a-twelve, m-zombie, z-ten
	s.Equal([]string{"a-twelve", "z-ten", "m-zombie"}, order, "10+1 and 11+0 tie; the tie goes to the lower ID")
}
