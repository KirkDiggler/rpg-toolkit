// Copyright (C) 2026 Kirk Diggler
// SPDX-License-Identifier: GPL-3.0-or-later

package session

import (
	"context"
	"encoding/json"
	"testing"

	"github.com/stretchr/testify/suite"

	"github.com/KirkDiggler/rpg-toolkit/core"
	"github.com/KirkDiggler/rpg-toolkit/npc"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/abilities"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/character"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/combat"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/conditions"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/encounter"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/encounter/dungeonspec"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/monster"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/proficiencies"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/races"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/refs"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/shared"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/skills"
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

// TestAPlayerAnswersEveryAttackItCanMake: the sheet seam answers the same
// projection a monster's stat block gets, so a character holding a weapon in
// each hand answers both, main hand first. The off hand used to be invisible
// to a driver reading this answer.
func (s *SheetSeamSuite) TestAPlayerAnswersEveryAttackItCanMake() {
	sheet := s.chars.byID["alice"]
	sheet.WeaponProficiencies = []proficiencies.Weapon{proficiencies.WeaponSimple, proficiencies.WeaponMartial}
	sheet.Inventory = []character.InventoryItemData{
		{Type: shared.EquipmentTypeWeapon, ID: string(weapons.Shortsword), Quantity: 1},
		{Type: shared.EquipmentTypeWeapon, ID: string(weapons.Dagger), Quantity: 1},
	}
	sheet.EquipmentSlots = character.EquipmentSlots{
		character.SlotMainHand: string(weapons.Shortsword),
		character.SlotOffHand:  string(weapons.Dagger),
	}

	facts := s.sheetOf("alice")

	names := make([]string, 0, len(facts.Actions))
	for _, action := range facts.Actions {
		names = append(names, action.Name)
	}
	s.Equal([]string{"Shortsword", "Dagger"}, names, "the main hand, then the off hand")
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
	s.Equal(memberActionsOf(block.Actions), facts.Actions)
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

// placeMonster spawns one more stat block into this suite's session record
// and classifies it, so a test can ask about a creature the setup does not
// carry.
func (s *SheetSeamSuite) placeMonster(id, ref string) {
	s.T().Helper()
	block, err := instantiate(id, ref, nil, nil)
	s.Require().NoError(err)
	s.data.NPCs = append(s.data.NPCs, *block)
	s.seam.kinds[id] = encounter.KindMonster
}

// TestAGoblinMayDisengageAsABonusAction is the sheet half of rpg-toolkit#2032:
// a goblin's stat block carries Nimble Escape, so its facts name the trait —
// its ref, and the name the conditions registry renders for it — and the
// composition can tell the story with both. A wolf carries no such trait, and
// a player's bonus actions are its own sheet's economy, so both answer zero.
func (s *SheetSeamSuite) TestAGoblinMayDisengageAsABonusAction() {
	s.placeMonster("gob-1", refs.Monsters.Goblin().String())
	s.placeMonster("wolf-1", refs.Monsters.Wolf().String())

	display, known := conditions.DisplayFor(*refs.MonsterTraits.NimbleEscape())
	s.Require().True(known, "the trait has a registry descriptor")

	s.Equal(encounter.ActivationIdentity{
		Ref:  refs.MonsterTraits.NimbleEscape().String(),
		Name: display.Name,
	}, s.sheetOf("gob-1").BonusDisengage)
	s.Equal("Nimble Escape", s.sheetOf("gob-1").BonusDisengage.Name,
		"the descriptor's name, which is what the story says")

	s.Zero(s.sheetOf("wolf-1").BonusDisengage, "a wolf has no Nimble Escape")
	s.Zero(s.sheetOf("skel-1").BonusDisengage, "nor does a skeleton")
	s.Zero(s.sheetOf("alice").BonusDisengage, "a player's bonus actions are its own sheet's economy")
}

// TestAnUnreadableStatBlockIsRefusedNotAnsweredZero: a monster whose stored
// conditions cannot be read is refused as corrupt. Answering zero would be a
// goblin that silently forgot its trait.
func (s *SheetSeamSuite) TestAnUnreadableStatBlockIsRefusedNotAnsweredZero() {
	s.placeMonster("gob-1", refs.Monsters.Goblin().String())
	block := &s.data.NPCs[len(s.data.NPCs)-1]
	block.Conditions = append([]json.RawMessage{json.RawMessage(`not json`)}, block.Conditions...)

	_, err := s.seam.Sheets([]encounter.MemberID{"gob-1"})
	s.ErrorIs(err, ErrBadNPC)
}

// TestATraitWithNoDescriptorIsRefusedNotTold: a trait the registry cannot
// name is refused, rather than handed to the composition as a beat with no
// name (which the composition would refuse as half-named anyway).
func (s *SheetSeamSuite) TestATraitWithNoDescriptorIsRefusedNotTold() {
	s.placeMonster("gob-1", refs.Monsters.Goblin().String())
	block := &s.data.NPCs[len(s.data.NPCs)-1]

	_, err := bonusDisengageOf(block, func(core.Ref) (string, bool) { return "", false })
	s.ErrorIs(err, ErrUnknownContent)
}

// TestAPlayerHearsWithItsSheetsPassivePerception is the character half of
// rpg-project#569 R5: the number the composition compares a sound's DC
// against is the sheet's own passive Perception — 10, Wisdom, and proficiency
// when trained — read at every consult. Wisdom 14 trained is 14; Wisdom 8
// untrained is 9; a Wisdom raised between two asks is heard with at the
// second, because nothing is stored.
func (s *SheetSeamSuite) TestAPlayerHearsWithItsSheetsPassivePerception() {
	sheet := s.chars.byID["alice"]
	sheet.AbilityScores[abilities.WIS] = 14
	sheet.Skills = map[skills.Skill]shared.ProficiencyLevel{skills.Perception: shared.Proficient}
	s.Equal(14, s.sheetOf("alice").PassivePerception, "10 + WIS 14's +2 + trained Perception's +2")

	sheet.AbilityScores[abilities.WIS] = 8
	sheet.Skills = nil
	s.Equal(9, s.sheetOf("alice").PassivePerception, "10 + WIS 8's -1, untrained: the sheet of this moment")
}

// TestAMonsterHearsWithItsStatBlocksPassivePerception is the monster half:
// the stat block's own passive Perception, never this seam's arithmetic. A
// template trained in Perception carries 10 + WIS + proficiency; one with no
// Perception skill still carries 10 + WIS — the assembly's answer, read here.
// A stat block changed between two asks is read by the second.
func (s *SheetSeamSuite) TestAMonsterHearsWithItsStatBlocksPassivePerception() {
	s.placeTemplate("keen", dungeonspec.TemplateSpec{
		Base: "dnd5e:monsters:human", Abilities: map[string]int{"wis": 16},
		Skills: []string{"perception"}, Actions: []string{"dnd5e:weapons:spear"},
	})
	s.placeTemplate("plain", dungeonspec.TemplateSpec{
		Base: "dnd5e:monsters:human", Abilities: map[string]int{"wis": 14},
		Actions: []string{"dnd5e:weapons:spear"},
	})

	s.Equal(15, s.sheetOf("keen").PassivePerception, "10 + WIS 16's +3 + trained Perception's +2")
	s.Equal(12, s.sheetOf("plain").PassivePerception, "no Perception skill: 10 + WIS 14's +2")

	block := &s.data.NPCs[len(s.data.NPCs)-1]
	block.Senses.PassivePerception = 7
	s.Equal(7, s.sheetOf("plain").PassivePerception, "the stat block of this moment, not of the spawn")
}

// TestAWorldNPCHearsNothing: its recorded content states no senses, so the
// true answer from that content is zero — deaf above DC 0, the composition's
// fail-closed reading of an unfilled field.
func (s *SheetSeamSuite) TestAWorldNPCHearsNothing() {
	s.Zero(s.sheetOf("merchant").PassivePerception)
}

// placeTemplate spawns one authored stat block into this suite's session
// record under the template's own ref, and classifies it.
func (s *SheetSeamSuite) placeTemplate(id string, spec dungeonspec.TemplateSpec) {
	s.T().Helper()
	block, err := instantiate(id, "dnd5e:monsters:"+id, nil, &spec)
	s.Require().NoError(err)
	s.data.NPCs = append(s.data.NPCs, *block)
	s.seam.kinds[id] = encounter.KindMonster
}

// storeOf wraps a test repository in the read-only sheet store a seam reads
// through, so a seam built by hand reads exactly the way a verb's does.
func storeOf(repo CharacterRepository) sheetStore {
	return sheetStore{repo: repo}
}

// TestTheWatchResetSeed is the session's seed for the sneak re-arm distance
// (rpg-project#606, design R10, O6): a monster watcher re-arms a sneaker two
// cells outside its zone; a character is never a watcher and seeds zero. A
// world NPC's recorded content states no watch terms, so it answers zero too.
func (s *SheetSeamSuite) TestTheWatchResetSeed() {
	s.Equal(2, s.sheetOf("skel-1").WatchResetCells, "a monster's seeded re-arm distance")
	s.Zero(s.sheetOf("alice").WatchResetCells, "a character watches nobody")
	s.Zero(s.sheetOf("merchant").WatchResetCells, "a world NPC states no watch terms")
}

// TestTheWatchArcSeed is the session's seed for a watcher's arc
// (rpg-project#621, design R14, O6): a monster whose stat block states no arc
// watches 120 degrees around its facing; a character is never a watcher and
// answers zero, the encounter's all-round no-arc answer.
func (s *SheetSeamSuite) TestTheWatchArcSeed() {
	s.Equal(120, s.sheetOf("skel-1").WatchArc, "a monster with no stated arc")
	s.Zero(s.sheetOf("alice").WatchArc, "a character watches nobody")
}

// sneakingSet answers the live encounter's one question from a set.
type sneakingSet map[encounter.MemberID]bool

func (s sneakingSet) Sneaking(member encounter.MemberID) bool { return s[member] }

// TestAPushRunsASneakerAtFullSpeed: sneaking halves the sneaker's own walking
// (design R8), and a move imposed on it is not its walking (lead ruling on
// rpg-toolkit#2079). So the composition's speed answer for sneaking alice is
// half, the push budget's is whole, and asking the push budget leaves the
// halved answer in place for the next walk.
func (s *SheetSeamSuite) TestAPushRunsASneakerAtFullSpeed() {
	s.seam.live = &liveEncounter{enc: sneakingSet{"alice": true}}

	s.Equal(15, s.sheetOf("alice").SpeedFeet, "her own walking is half her thirty feet")
	speeds, err := sheetSpeeds(s.seam, []encounter.MemberID{"alice", "skel-1"})
	s.Require().NoError(err)
	s.Equal(30, speeds["alice"], "a push runs her at her full speed")
	s.Equal(s.data.NPCs[0].Speed.Walk, speeds["skel-1"], "and nobody else's answer changes")
	s.Equal(15, s.sheetOf("alice").SpeedFeet, "the push budget did not clear the shared slot")
}
