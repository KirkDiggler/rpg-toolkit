// Copyright (C) 2026 Kirk Diggler
// SPDX-License-Identifier: GPL-3.0-or-later

package session

import (
	"context"
	"testing"

	"github.com/stretchr/testify/suite"

	"github.com/KirkDiggler/rpg-toolkit/npc"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/character"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/combat"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/encounter"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/monster"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/races"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/shared"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/weapons"
)

// SheetSeamSuite is the sheet seam answering the composition's Sheets and
// Sight questions from the sheets one verb holds (rpg-project#538): a
// character from the host's store, a monster from its stat block, a world NPC
// from its recorded content — asked at each consult, refused for a member the
// verb holds no sheet for.
type SheetSeamSuite struct {
	suite.Suite

	chars participationCharacterStore
	data  *SessionData
	seam  sheetSeam
}

func TestSheetSeamSuite(t *testing.T) {
	suite.Run(t, new(SheetSeamSuite))
}

func (s *SheetSeamSuite) SetupTest() {
	human := dwarfCharacterRecord("alice", 10)
	human.RaceID = races.Human

	skeleton, err := instantiate("skel-1", "dnd5e:monsters:skeleton", nil, nil)
	s.Require().NoError(err)
	skeleton.Targeting = monster.TargetLowestHP

	s.chars = participationCharacterStore{byID: map[string]*character.Data{"alice": human}}
	s.data = &SessionData{
		NPCs: []monster.Data{*skeleton},
		WorldNPCs: []PlacedWorldNPC{{
			MemberID: "merchant",
			NPC:      npc.Data{DisplayName: "Merchant", MovementPolicy: npc.MovementPolicyBlocking},
		}},
	}
	s.seam = sheetsBeside(standingSeam{
		ctx:    context.Background(),
		sheets: storeOf(s.chars),
		data:   s.data,
		kinds: map[string]encounter.MemberKind{
			"alice":    encounter.KindPlayer,
			"skel-1":   encounter.KindMonster,
			"merchant": encounter.KindWorld,
		},
	})
}

// sheetOf asks the seam for one member's facts, through the capability.
func (s *SheetSeamSuite) sheetOf(id encounter.MemberID) encounter.SheetFacts {
	s.T().Helper()
	all, err := s.seam.Sheets([]encounter.MemberID{id})
	s.Require().NoError(err)
	facts, ok := all[id]
	s.Require().True(ok, "the seam answers about %q", id)
	return facts
}

// sightOf asks the seam how far one member sees, in cells.
func (s *SheetSeamSuite) sightOf(id encounter.MemberID) int {
	s.T().Helper()
	all, err := s.seam.Sight([]encounter.MemberID{id})
	s.Require().NoError(err)
	cells, ok := all[id]
	s.Require().True(ok, "the seam answers about %q", id)
	return cells
}

// TestAPlayerIsAnsweredFromTheSheet carries the pins Join used to make on the
// seated member record: a Human walks 30 — derived from race by the loaded
// sheet and on no record — and an empty main hand reaches 5 feet in melee.
// Both mutants it was written against (every player at speed zero; an attack
// with no range and no kind) would now be answers this seam gave.
func (s *SheetSeamSuite) TestAPlayerIsAnsweredFromTheSheet() {
	facts := s.sheetOf("alice")

	s.Equal(30, facts.SpeedFeet, "a Human walks 30: speed from the sheet, not zero")
	s.Require().Len(facts.Actions, 1, "the main-hand attack is the one action a player answers")
	s.Equal(5, facts.Actions[0].RangeFeet, "an unarmed strike reaches 5 feet")
	s.Equal("melee", facts.Actions[0].Kind, "and it is made in reach")
	s.Empty(facts.Targeting, "a player has no targeting strategy")
}

// TestAPlayersAnswerIsTheSheetOfThatMoment: nothing is remembered between
// consults. A race changed and a bow equipped between two asks are both read
// by the second — the copy taken at Join that this replaces would answer the
// first sheet forever.
func (s *SheetSeamSuite) TestAPlayersAnswerIsTheSheetOfThatMoment() {
	s.Require().Equal(30, s.sheetOf("alice").SpeedFeet, "precondition: a Human")

	sheet := s.chars.byID["alice"]
	sheet.RaceID = races.Dwarf
	sheet.Inventory = []character.InventoryItemData{{Type: shared.EquipmentTypeWeapon, ID: string(weapons.Longbow), Quantity: 1}}
	sheet.EquipmentSlots = character.EquipmentSlots{character.SlotMainHand: string(weapons.Longbow)}

	facts := s.sheetOf("alice")
	s.Equal(25, facts.SpeedFeet, "a Dwarf walks 25, read at this consult")
	s.Require().Len(facts.Actions, 1)
	s.Equal(600, facts.Actions[0].RangeFeet, "the longbow's long range, read at this consult")
}

// TestAMonsterIsAnsweredFromItsStatBlock: its walking speed, every executable
// action in author order, and the targeting word its placement wrote onto it.
// A stat block changed between two asks is read by the second.
func (s *SheetSeamSuite) TestAMonsterIsAnsweredFromItsStatBlock() {
	block := &s.data.NPCs[0]
	facts := s.sheetOf("skel-1")

	s.Equal(block.Speed.Walk, facts.SpeedFeet)
	s.Equal(memberActionsFromMonster(block.Actions), facts.Actions)
	s.Equal("lowest-health", facts.Targeting, "the word the placement wrote onto the stat block")

	block.Speed.Walk = 10
	s.Equal(10, s.sheetOf("skel-1").SpeedFeet, "the stat block of this moment, not of the spawn")
}

// TestAMonsterSeesWhatItsStatBlockAuthors is slice 4's done-when: a monster
// whose stat block authors darkvision sees that far; one that authors none
// sees the stated default — the rulebook's, held on the stat block, not this
// seam's.
func (s *SheetSeamSuite) TestAMonsterSeesWhatItsStatBlockAuthors() {
	s.data.NPCs[0].Senses.Darkvision = 0
	s.Equal(encounter.CellsFromFeet(combat.DefaultSightFeet), s.sightOf("skel-1"),
		"a stat block that authors no range sees the stated default")

	s.data.NPCs[0].Senses.Darkvision = 60
	s.Equal(encounter.CellsFromFeet(60), s.sightOf("skel-1"),
		"a stat block that authors darkvision sees that far")
}

// TestACharacterSeesWhatItsSheetStates: the race table states no range today,
// so the sheet answers the stated default — through the sheet, never through
// a constant applied here.
func (s *SheetSeamSuite) TestACharacterSeesWhatItsSheetStates() {
	s.Equal(encounter.CellsFromFeet(combat.DefaultSightFeet), s.sightOf("alice"))
}

// TestAWorldNPCIsAnsweredFromItsContent: stationary and non-acting by
// construction, so its recorded content answers zero speed and nothing to
// swing; it states no senses, so it sees the stated default.
func (s *SheetSeamSuite) TestAWorldNPCIsAnsweredFromItsContent() {
	s.Equal(encounter.SheetFacts{}, s.sheetOf("merchant"))
	s.Equal(encounter.CellsFromFeet(combat.DefaultSightFeet), s.sightOf("merchant"))
}

// TestAMemberTheVerbHoldsNoSheetForIsRefused is slice 4's done-when: a
// consult for a member the verb holds no sheet for is refused by name, never
// answered with a default — for either capability and every kind.
func (s *SheetSeamSuite) TestAMemberTheVerbHoldsNoSheetForIsRefused() {
	cases := []struct {
		name   string
		member encounter.MemberID
		kind   encounter.MemberKind
		want   error
	}{
		{"a player the store does not hold", "bob", encounter.KindPlayer, ErrNoCharacter},
		{"a monster the session holds no stat block for", "ogre", encounter.KindMonster, ErrNoSheet},
		{"a world NPC with no recorded content", "stranger", encounter.KindWorld, ErrNoSheet},
		{"a member with no roster kind", "ghost", "", ErrInvalidSession},
	}
	for _, tc := range cases {
		s.Run(tc.name, func() {
			if tc.kind != "" {
				s.seam.kinds[string(tc.member)] = tc.kind
			}
			ask := []encounter.MemberID{"alice", tc.member}

			sheets, err := s.seam.Sheets(ask)
			s.ErrorIs(err, tc.want)
			s.Nil(sheets, "no partial answer")

			sight, err := s.seam.Sight(ask)
			s.ErrorIs(err, tc.want)
			s.Nil(sight, "no partial answer, and no default range")
		})
	}
}

// TestAnUnreadableSheetIsRefusedAsCorrupt: a stored sheet that exists and
// will not reconstitute is ErrBadCharacter for both questions — corrupt, not
// absent, and not a default.
func (s *SheetSeamSuite) TestAnUnreadableSheetIsRefusedAsCorrupt() {
	s.chars.byID["alice"].Conditions = append(s.chars.byID["alice"].Conditions, []byte(`{"ref":"nonsense","x":`))

	_, err := s.seam.Sight([]encounter.MemberID{"alice"})
	s.ErrorIs(err, ErrBadCharacter)
	s.NotErrorIs(err, ErrNoCharacter)

	_, err = s.seam.Sheets([]encounter.MemberID{"alice"})
	s.ErrorIs(err, ErrBadCharacter)
	s.NotErrorIs(err, ErrNoCharacter)
}

// TestThePushBudgetAsksTheSameAnswer: the session's own push budget reads
// speed from the seam the composition asks, at the moment of the push.
func (s *SheetSeamSuite) TestThePushBudgetAsksTheSameAnswer() {
	speeds, err := sheetSpeeds(s.seam, []encounter.MemberID{"skel-1", "alice"})
	s.Require().NoError(err)
	s.Equal(s.data.NPCs[0].Speed.Walk, speeds["skel-1"])
	s.Equal(30, speeds["alice"])

	_, err = sheetSpeeds(s.seam, []encounter.MemberID{"nobody"})
	s.ErrorIs(err, ErrInvalidSession, "a member the verb cannot classify is refused, not answered zero")
}

// storeOf wraps a test repository in the read-only sheet store a seam reads
// through, so a seam built by hand reads exactly the way a verb's does.
func storeOf(repo CharacterRepository) sheetStore {
	return sheetStore{repo: repo}
}
