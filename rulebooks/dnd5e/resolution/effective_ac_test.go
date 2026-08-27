// Copyright (C) 2026 Kirk Diggler
// SPDX-License-Identifier: GPL-3.0-or-later

package resolution

import (
	"context"
	"encoding/json"
	"testing"

	"github.com/stretchr/testify/suite"

	"github.com/KirkDiggler/rpg-toolkit/dice"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/abilities"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/armor"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/character"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/classes"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/conditions"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/encounter"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/monster/monsters"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/races"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/shared"
	"github.com/KirkDiggler/rpg-toolkit/tools/spatial"
)

// EffectiveACTestSuite is the proving test for the AC-chain custody question
// (#965 slice 2's census).
//
// The strike read target.AC() — the flat number on the sheet — because slice 1
// could not tell whether the AC chain would fold correctly from inside an
// interaction. The census argued it would: combat.GetEffectiveAC is bus-free
// at its signature, Character.EffectiveAC folds combat.ACChain on the
// character's parked bus, and Attach sets that bus to whatever it is handed —
// which under Resolve is this interaction's own surface. The two AC
// subscribers, Defense and Unarmored Defense, attach to the same surface.
//
// Argued is not measured. These tests measure it: a Defense-style fighter's
// +1 must reach the number the attack is rolled against, end to end through
// Resolve, or the flip does not happen.
type EffectiveACTestSuite struct {
	suite.Suite

	ctx context.Context
}

func TestEffectiveACSuite(t *testing.T) {
	suite.Run(t, new(EffectiveACTestSuite))
}

func (s *EffectiveACTestSuite) SetupTest() {
	s.ctx = context.Background()
}

// armoredHero wears chain mail (AC 16, no DEX) and carries whatever conditions
// the case needs. Chain mail rather than leather on purpose: its MaxDexBonus
// of zero takes the hero's DEX out of the arithmetic, so the only number that
// can move the total is the one under test.
func (s *EffectiveACTestSuite) armoredHero(conds ...json.RawMessage) *character.Data {
	return &character.Data{
		ID:       heroID,
		PlayerID: "player-1",
		Name:     "Grog",
		Level:    1,
		ClassID:  classes.Fighter,
		RaceID:   races.Human,
		AbilityScores: shared.AbilityScores{
			abilities.STR: 16,
			abilities.DEX: 14,
			abilities.CON: 14,
			abilities.INT: 10,
			abilities.WIS: 12,
			abilities.CHA: 8,
		},
		HitPoints:    14,
		MaxHitPoints: 14,
		// The flat sheet number, deliberately DIFFERENT from what the armor
		// and the fighting style compute. If the strike reads this, the tests
		// below fail with a number that says so.
		ArmorClass:       10,
		ProficiencyBonus: 2,
		Inventory: []character.InventoryItemData{
			{Type: shared.EquipmentTypeArmor, ID: string(armor.ChainMail), Quantity: 1},
		},
		EquipmentSlots: character.EquipmentSlots{
			character.SlotArmor: string(armor.ChainMail),
		},
		Conditions: conds,
	}
}

func (s *EffectiveACTestSuite) defenseStyle() json.RawMessage {
	raw, err := (&conditions.FightingStyleDefenseCondition{
		CharacterID: heroID,
	}).ToJSON()
	s.Require().NoError(err)

	return raw
}

func (s *EffectiveACTestSuite) world() encounter.EncounterData {
	enc, err := encounter.NewEncounter(&encounter.SetupInput{Initiative: orderAsGiven{}, TurnDriver: passDriver{}, Striker: noAttacksExpected{}, Announcer: quietAnnouncer{}, Standing: everyoneStanding{}, Sight: everyoneSeesTheWholeMap{},
		Field: encounter.FieldInput{
			Canvas:  hexCanvas(),
			Regions: []encounter.RegionInput{rectRegion("room-1", 0, 0, 10, 10)},
		},
		Members: []encounter.MemberInput{
			{ID: heroID, Kind: encounter.KindPlayer, Position: spatial.Position{X: 5, Y: 5}},
			{ID: wolfID, Kind: encounter.KindMonster, Position: spatial.Position{X: 5, Y: 6}},
		},
		Endings: []encounter.EndingInput{{Key: "done", Trigger: encounter.TriggerExternal{}}},
	})
	s.Require().NoError(err)

	return enc.ToData()
}

// biteAt runs the wolf's bite against the given hero and reports what AC the
// strike measured itself against.
func (s *EffectiveACTestSuite) biteAt(hero *character.Data, roll int) StrikeOutcome {
	data := monsters.NewWolf(wolfID).ToData()
	attack := data.Actions[0]

	out, err := Resolve(s.ctx, &Input{Initiative: orderAsGiven{}, TurnDriver: passDriver{}, Standing: everyoneStanding{}, Sight: everyoneSeesTheWholeMap{}, Roller: dice.NewRoller(),
		World:        s.world(),
		Participants: []Participant{{Character: hero}, {Monster: data}},
		Machine: NewStrike(&StrikeInput{
			AttackerID: wolfID,
			TargetID:   heroID,
			Definition: attack,
			Roller:     &sequenceRoller{singles: []int{roll, 18}, pair: []int{3, 4}},
		}),
	})
	s.Require().NoError(err)

	outcome, ok := out.Outcome.(StrikeOutcome)
	s.Require().True(ok)

	return outcome
}

// unarmoredBarbarian wears nothing at all.
//
// No armor on purpose: Unarmored Defense only applies when unarmored, so the
// whole number has to come from the sheet's own ability scores through the AC
// chain. DEX 14 (+2) and CON 14 (+2) put the answer at 10+2+2 = 14, and the
// flat ArmorClass is 10 so that reading the sheet reports a number that says
// so.
func (s *EffectiveACTestSuite) unarmoredBarbarian(conds ...json.RawMessage) *character.Data {
	return &character.Data{
		ID:       heroID,
		PlayerID: "player-1",
		Name:     "Standre",
		Level:    1,
		ClassID:  classes.Barbarian,
		RaceID:   races.Human,
		AbilityScores: shared.AbilityScores{
			abilities.STR: 16,
			abilities.DEX: 14, // +2
			abilities.CON: 14, // +2
			abilities.INT: 10,
			abilities.WIS: 12,
			abilities.CHA: 8,
		},
		HitPoints:        14,
		MaxHitPoints:     14,
		ArmorClass:       10,
		ProficiencyBonus: 2,
		Conditions:       conds,
	}
}

func (s *EffectiveACTestSuite) unarmoredDefense() json.RawMessage {
	raw, err := (&conditions.UnarmoredDefenseCondition{
		CharacterID: heroID,
		Type:        conditions.UnarmoredDefenseBarbarian,
		Source:      "dnd5e:classes:barbarian",
	}).ToJSON()
	s.Require().NoError(err)

	return raw
}

// THE TEST THIS WHOLE SLICE EXISTS FOR. Unarmored Defense reaches the strike.
//
// It did not, in any real fight, for as long as the rule has existed. The
// condition read gamectx.RequireCharacters — a registry with zero non-test
// install sites — and returned its error into the AC fold, which
// Character.EffectiveAC swallows. So a barbarian with Unarmored Defense
// attached was struck at 10+DEX, every other AC contributor was discarded with
// it, and nothing was logged. Kirk found it by playing: his barbarian fought
// the tomb at 11 instead of 14 (rpg-api#842, rpg-toolkit#1251).
//
// Every test of the rule passed throughout, because every one of them
// installed a registry by hand that production never installed. Which is why
// this one lives HERE and goes through Resolve: the condition now reads its own
// sheet, handed over by the same Attach the interaction performs, and there is
// nothing in this test for a bug to hide behind.
func (s *EffectiveACTestSuite) TestUnarmoredDefenseReachesTheStrike() {
	bare := s.biteAt(s.unarmoredBarbarian(), 12)
	defended := s.biteAt(s.unarmoredBarbarian(s.unarmoredDefense()), 12)

	s.Require().Equal(12, bare.TargetAC,
		"unarmored and without the feature: 10 + DEX(+2)")
	s.Require().Equal(14, defended.TargetAC,
		"Unarmored Defense adds CON(+2) through the AC chain: 10 + DEX + CON")
	s.Require().Equal(2, defended.TargetAC-bare.TargetAC,
		"exactly the feature's contribution, folded on this interaction's bus")
}

// And it decides the hit, which is the part that was costing hit points.
//
// The wolf's +4 against 12 needs an 8; against 14 it needs a 10. The same 8
// must therefore land on the barbarian who lost the feature and miss the one
// who has it — one roll, two points of Constitution between them.
func (s *EffectiveACTestSuite) TestUnarmoredDefenseDecidesTheHit() {
	bare := s.biteAt(s.unarmoredBarbarian(), 8)
	s.Require().True(bare.Hit, "8 + 4 = 12 meets AC 12")

	defended := s.biteAt(s.unarmoredBarbarian(s.unarmoredDefense()), 8)
	s.Require().False(defended.Hit, "the same 12 falls short of AC 14")
	s.Require().Zero(defended.Damage, "and a miss deals nothing")
}

// THE PROVING TEST. Worn armor reaches the strike.
//
// Chain mail is AC 16 with no DEX contribution. The sheet's flat ArmorClass
// says 10. If the strike measured against the flat number this reports 10.
func (s *EffectiveACTestSuite) TestWornArmorReachesTheStrike() {
	outcome := s.biteAt(s.armoredHero(), 12)

	s.Require().Equal(16, outcome.TargetAC,
		"chain mail's 16, not the sheet's flat 10")
}

// AND THE ONE THAT MATTERS: a fighting style that folds onto the AC chain
// reaches it too. Defense is +1 while wearing armor, so 16 becomes 17.
//
// This is the case the flat number could never produce and the armor lookup
// alone could not either — it can only arrive through the AC chain folding on
// this interaction's bus.
func (s *EffectiveACTestSuite) TestTheDefenseStyleFoldsIntoTheACTheStrikeUses() {
	plain := s.biteAt(s.armoredHero(), 12)
	defended := s.biteAt(s.armoredHero(s.defenseStyle()), 12)

	s.Require().Equal(16, plain.TargetAC)
	s.Require().Equal(17, defended.TargetAC, "Defense is +1 in armor")
	s.Require().Equal(1, defended.TargetAC-plain.TargetAC,
		"exactly the fighting style's contribution, folded on this interaction's bus")
}

// The folded AC is the number the hit is DECIDED against, not merely reported.
//
// The wolf's +4 against 16 needs a 12; against 17 it needs a 13. A 12 must
// therefore hit the plain fighter and miss the defended one — the same roll,
// the same attacker, one point of fighting style between them.
func (s *EffectiveACTestSuite) TestTheFoldedACDecidesTheHitNotJustTheReport() {
	plain := s.biteAt(s.armoredHero(), 12)
	s.Require().True(plain.Hit, "12 + 4 = 16 meets AC 16")

	defended := s.biteAt(s.armoredHero(s.defenseStyle()), 12)
	s.Require().False(defended.Hit, "the same 16 falls short of AC 17")
	s.Require().Zero(defended.Damage, "and a miss deals nothing")
}

// A monster target has no AC chain to fold — stat blocks carry a number — so
// it reports its own AC unchanged. Confirms the character path did not become
// a requirement for everyone.
func (s *EffectiveACTestSuite) TestAMonsterTargetStillReportsItsStatBlockAC() {
	data := monsters.NewWolf(wolfID).ToData()
	second := monsters.NewWolf(secondWolfID).ToData()
	attack := data.Actions[0]

	enc, err := encounter.NewEncounter(&encounter.SetupInput{Initiative: orderAsGiven{}, TurnDriver: passDriver{}, Striker: noAttacksExpected{}, Announcer: quietAnnouncer{}, Standing: everyoneStanding{}, Sight: everyoneSeesTheWholeMap{},
		Field: encounter.FieldInput{
			Canvas:  hexCanvas(),
			Regions: []encounter.RegionInput{rectRegion("room-1", 0, 0, 10, 10)},
		},
		Members: []encounter.MemberInput{
			{ID: wolfID, Kind: encounter.KindMonster, Position: spatial.Position{X: 5, Y: 5}},
			{ID: secondWolfID, Kind: encounter.KindMonster, Position: spatial.Position{X: 5, Y: 6}},
		},
		Endings: []encounter.EndingInput{{Key: "done", Trigger: encounter.TriggerExternal{}}},
	})
	s.Require().NoError(err)

	out, err := Resolve(s.ctx, &Input{Initiative: orderAsGiven{}, TurnDriver: passDriver{}, Standing: everyoneStanding{}, Sight: everyoneSeesTheWholeMap{}, Roller: dice.NewRoller(),
		World:        enc.ToData(),
		Participants: []Participant{{Monster: data}, {Monster: second}},
		Machine: NewStrike(&StrikeInput{
			AttackerID: wolfID,
			TargetID:   secondWolfID,
			Definition: attack,
			Roller:     &sequenceRoller{singles: []int{15, 18}, pair: []int{3, 4}},
		}),
	})
	s.Require().NoError(err)

	outcome, ok := out.Outcome.(StrikeOutcome)
	s.Require().True(ok)
	s.Require().Equal(13, outcome.TargetAC, "the wolf's own natural armor")
}
