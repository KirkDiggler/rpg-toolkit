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
	enc, err := encounter.NewEncounter(&encounter.SetupInput{Initiative: orderAsGiven{}, TurnDriver: passDriver{}, Standing: everyoneStanding{}, Sight: everyoneSeesTheWholeMap{},
		Field: encounter.FieldInput{
			Canvas: encounter.CanvasInput{Void: encounter.VoidIsOpaque()},
			Rooms:  []encounter.RoomInput{{ID: "room-1", Width: 10, Height: 10}},
		},
		Members: []encounter.MemberInput{
			{ID: heroID, Kind: encounter.KindPlayer, Room: "room-1", Position: spatial.Position{X: 5, Y: 5}},
			{ID: wolfID, Kind: encounter.KindMonster, Room: "room-1", Position: spatial.Position{X: 8, Y: 5}},
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
	attack, err := AttackFromMonsterAction(data.Actions[0])
	s.Require().NoError(err)

	out, err := Resolve(s.ctx, &Input{Initiative: orderAsGiven{}, TurnDriver: passDriver{}, Standing: everyoneStanding{}, Sight: everyoneSeesTheWholeMap{}, Roller: dice.NewRoller(),
		World:        s.world(),
		Participants: []Participant{{Character: hero}, {Monster: data}},
		Machine: NewStrike(&StrikeInput{
			AttackerID: wolfID,
			TargetID:   heroID,
			Attack:     attack,
			Roller:     &sequenceRoller{singles: []int{roll, 18}, pair: []int{3, 4}, fallback: 2},
		}),
	})
	s.Require().NoError(err)

	outcome, ok := out.Outcome.(StrikeOutcome)
	s.Require().True(ok)

	return outcome
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
	attack, err := AttackFromMonsterAction(data.Actions[0])
	s.Require().NoError(err)

	enc, err := encounter.NewEncounter(&encounter.SetupInput{Initiative: orderAsGiven{}, TurnDriver: passDriver{}, Standing: everyoneStanding{}, Sight: everyoneSeesTheWholeMap{},
		Field: encounter.FieldInput{
			Canvas: encounter.CanvasInput{Void: encounter.VoidIsOpaque()},
			Rooms:  []encounter.RoomInput{{ID: "room-1", Width: 10, Height: 10}},
		},
		Members: []encounter.MemberInput{
			{ID: wolfID, Kind: encounter.KindMonster, Room: "room-1", Position: spatial.Position{X: 5, Y: 5}},
			{ID: secondWolfID, Kind: encounter.KindMonster, Room: "room-1", Position: spatial.Position{X: 5, Y: 6}},
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
			Attack:     attack,
			Roller:     &sequenceRoller{singles: []int{15, 18}, pair: []int{3, 4}, fallback: 2},
		}),
	})
	s.Require().NoError(err)

	outcome, ok := out.Outcome.(StrikeOutcome)
	s.Require().True(ok)
	s.Require().Equal(13, outcome.TargetAC, "the wolf's own natural armor")
}
