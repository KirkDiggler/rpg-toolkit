// Copyright (C) 2026 Kirk Diggler
// SPDX-License-Identifier: GPL-3.0-or-later

package resolution

import (
	"context"
	"encoding/json"
	"testing"

	"github.com/stretchr/testify/suite"

	coreCombat "github.com/KirkDiggler/rpg-toolkit/core/combat"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/abilities"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/armor"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/character"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/classes"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/combat"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/races"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/shared"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/weapons"
)

const (
	equipperID = "equipper"

	// equipTurn is the turn the fixture's stored economy is filed under. A
	// fight on this turn finds the bank exactly as stored; any other number
	// refreshes it.
	equipTurn = 3
)

var (
	eqLongsword  = string(weapons.Longsword)
	eqHandaxe    = string(weapons.Handaxe)
	eqGreatsword = string(weapons.Greatsword)
	eqShield     = string(armor.Shield)
	eqChainMail  = string(armor.ChainMail)
)

// EquipTestSuite proves the door charges an in-fight equipment change before
// it applies it, and that free roam charges nothing (rpg-project#542 slice 3).
type EquipTestSuite struct {
	suite.Suite

	ctx context.Context
}

func TestEquipSuite(t *testing.T) {
	suite.Run(t, new(EquipTestSuite))
}

func (s *EquipTestSuite) SetupTest() {
	s.ctx = context.Background()
}

// fighter is a record holding hands as given, with no stored economy.
func (s *EquipTestSuite) fighter(hands character.EquipmentSlots) *character.Data {
	return &character.Data{
		ID: equipperID, PlayerID: "equip-player", Name: "Equipper",
		Level: 1, ProficiencyBonus: 2, RaceID: races.Human, ClassID: classes.Fighter,
		Levels: syntheticLevels(classes.Fighter, 1, 12, 8),
		AbilityScores: shared.AbilityScores{
			abilities.STR: 16, abilities.DEX: 14, abilities.CON: 14,
			abilities.INT: 10, abilities.WIS: 12, abilities.CHA: 8,
		},
		HitPoints: 12, MaxHitPoints: 12,
		Inventory: []character.InventoryItemData{
			{Type: shared.EquipmentTypeWeapon, ID: eqLongsword, Quantity: 1},
			{Type: shared.EquipmentTypeWeapon, ID: eqHandaxe, Quantity: 1},
			{Type: shared.EquipmentTypeWeapon, ID: eqGreatsword, Quantity: 1},
			{Type: shared.EquipmentTypeArmor, ID: eqShield, Quantity: 1},
			{Type: shared.EquipmentTypeArmor, ID: eqChainMail, Quantity: 1},
		},
		EquipmentSlots: hands,
	}
}

// inFight is the fighter with an economy stored under equipTurn: the action
// as given, the turn's object interaction unspent.
func (s *EquipTestSuite) inFight(hands character.EquipmentSlots, actions int) *character.Data {
	data := s.fighter(hands)
	data.ActionEconomy = &character.ActionEconomyData{
		TurnNumber:            equipTurn,
		ActionsRemaining:      actions,
		BonusActionsRemaining: 1,
		ReactionsRemaining:    1,
		MovementRemaining:     30,
		Granted: map[character.GrantedActionKey]int{
			character.GrantedObjectInteractions: 1,
		},
	}
	return data
}

func eqThisTurn() *Turn { return &Turn{Number: equipTurn, Speed: 30} }

func eqOneAction() map[coreCombat.ActionType]int {
	return map[coreCombat.ActionType]int{coreCombat.ActionStandard: 1}
}

func eqOneInteraction() map[combat.CapacityType]int {
	return map[combat.CapacityType]int{combat.CapacityObjectInteraction: 1}
}

// §3 done-when: the action already spent and a stow in the change refuses as
// unaffordable and returns no record. The caller's record is untouched.
func (s *EquipTestSuite) TestAStowWithTheActionSpentIsUnaffordableAndReturnsNoRecord() {
	input := s.inFight(character.EquipmentSlots{character.SlotMainHand: eqLongsword}, 0)
	before, err := json.Marshal(input)
	s.Require().NoError(err)

	out, err := Equip(s.ctx, &EquipInput{
		Character: input, Slot: character.SlotMainHand, Fight: eqThisTurn(),
	})

	s.Require().ErrorIs(err, ErrCannotPay)
	s.Require().Nil(out, "an unpayable change returns no record to write")

	after, err := json.Marshal(input)
	s.Require().NoError(err)
	s.Require().JSONEq(string(before), string(after), "the caller's record must not move")
}

// §3 done-when: a costed stow pays exactly the profile — one action, and the
// object interaction untouched.
func (s *EquipTestSuite) TestACostedStowPaysExactlyOneAction() {
	out, err := Equip(s.ctx, &EquipInput{
		Character: s.inFight(character.EquipmentSlots{character.SlotMainHand: eqLongsword}, 1),
		Slot:      character.SlotMainHand,
		Fight:     eqThisTurn(),
	})
	s.Require().NoError(err)
	s.Require().NotNil(out.Character)

	s.Require().NotNil(out.Paid)
	s.Equal(eqOneAction(), out.Paid.Slots)
	s.Empty(out.Paid.Capacity)
	s.Empty(out.Paid.Pools)

	economy := out.Character.ActionEconomy
	s.Require().NotNil(economy)
	s.Equal(0, economy.ActionsRemaining, "the stow spent the action")
	s.Equal(1, economy.Granted[character.GrantedObjectInteractions], "and not the interaction")
	s.Equal(1, economy.BonusActionsRemaining)

	s.Empty(out.Character.EquipmentSlots.Get(character.SlotMainHand), "the change was applied")
	s.Equal([]EquipMove{{Slot: character.SlotMainHand, Ref: "dnd5e:weapons:longsword"}}, out.Stowed)
	s.Empty(out.Drawn)
}

// §3 done-when: a costed draw pays exactly the profile — one object
// interaction, and the action untouched.
func (s *EquipTestSuite) TestACostedDrawPaysExactlyOneInteraction() {
	out, err := Equip(s.ctx, &EquipInput{
		Character: s.inFight(character.EquipmentSlots{}, 1),
		Slot:      character.SlotMainHand,
		ItemID:    eqLongsword,
		Fight:     eqThisTurn(),
	})
	s.Require().NoError(err)

	s.Require().NotNil(out.Paid)
	s.Empty(out.Paid.Slots)
	s.Equal(eqOneInteraction(), out.Paid.Capacity)

	economy := out.Character.ActionEconomy
	s.Require().NotNil(economy)
	s.Equal(1, economy.ActionsRemaining, "the draw left the action")
	s.Zero(economy.Granted[character.GrantedObjectInteractions], "and spent the interaction")

	s.Equal(eqLongsword, out.Character.EquipmentSlots.Get(character.SlotMainHand))
	s.Equal([]EquipMove{{Slot: character.SlotMainHand, Ref: "dnd5e:weapons:longsword"}}, out.Drawn)
	s.Empty(out.Stowed)
}

// A swap is both, charged together: the stow's action and the draw's
// interaction.
func (s *EquipTestSuite) TestACostedSwapPaysTheActionAndTheInteraction() {
	out, err := Equip(s.ctx, &EquipInput{
		Character: s.inFight(character.EquipmentSlots{character.SlotMainHand: eqHandaxe}, 1),
		Slot:      character.SlotMainHand,
		ItemID:    eqLongsword,
		Fight:     eqThisTurn(),
	})
	s.Require().NoError(err)

	s.Require().NotNil(out.Paid)
	s.Equal(eqOneAction(), out.Paid.Slots)
	s.Equal(eqOneInteraction(), out.Paid.Capacity)
	s.Equal(0, out.Character.ActionEconomy.ActionsRemaining)
	s.Zero(out.Character.ActionEconomy.Granted[character.GrantedObjectInteractions])
	s.Equal([]EquipMove{{Slot: character.SlotMainHand, Ref: "dnd5e:weapons:handaxe"}}, out.Stowed)
	s.Equal([]EquipMove{{Slot: character.SlotMainHand, Ref: "dnd5e:weapons:longsword"}}, out.Drawn)
}

// The door readies the sheet for the fight's turn BEFORE it prices: a bank
// spent on an earlier turn is full again, so the stow is affordable.
func (s *EquipTestSuite) TestAnEarlierTurnsSpentBankIsRefreshedBeforeThePrice() {
	out, err := Equip(s.ctx, &EquipInput{
		Character: s.inFight(character.EquipmentSlots{character.SlotMainHand: eqLongsword}, 0),
		Slot:      character.SlotMainHand,
		Fight:     &Turn{Number: equipTurn + 1, Speed: 30},
	})
	s.Require().NoError(err)

	s.Equal(equipTurn+1, out.Character.ActionEconomy.TurnNumber)
	s.Equal(0, out.Character.ActionEconomy.ActionsRemaining, "refilled, then spent by the stow")
	s.Equal(1, out.Character.ActionEconomy.Granted[character.GrantedObjectInteractions])
}

// Body armour cannot change in a fight (R6). The refusal passes through under
// its own name, not as a malformed request; the same change in free roam is
// legal.
func (s *EquipTestSuite) TestArmourInAFightIsRefusedUnderItsOwnName() {
	out, err := Equip(s.ctx, &EquipInput{
		Character: s.inFight(character.EquipmentSlots{}, 1),
		Slot:      character.SlotArmor,
		ItemID:    eqChainMail,
		Fight:     eqThisTurn(),
	})
	s.Require().ErrorIs(err, character.ErrArmorInFight)
	s.Require().NotErrorIs(err, ErrBadEquip)
	s.Require().NotErrorIs(err, ErrCannotPay)
	s.Require().Nil(out)

	roam, err := Equip(s.ctx, &EquipInput{
		Character: s.fighter(character.EquipmentSlots{}),
		Slot:      character.SlotArmor,
		ItemID:    eqChainMail,
	})
	s.Require().NoError(err)
	s.Equal(eqChainMail, roam.Character.EquipmentSlots.Get(character.SlotArmor))
	s.Equal([]EquipMove{{Slot: character.SlotArmor, Ref: "dnd5e:armor:chain-mail"}}, roam.Drawn,
		"a worn slot's change is named in free roam, so the session can tell it")
	s.Empty(roam.Stowed)
}

// Free roam is charged nothing and still applies: no price, no economy
// invented, and the hands named for the beats.
func (s *EquipTestSuite) TestFreeRoamChargesNothingAndApplies() {
	out, err := Equip(s.ctx, &EquipInput{
		Character: s.fighter(character.EquipmentSlots{character.SlotMainHand: eqHandaxe}),
		Slot:      character.SlotMainHand,
		ItemID:    eqLongsword,
	})
	s.Require().NoError(err)

	s.Nil(out.Paid)
	s.Nil(out.Character.ActionEconomy, "free roam readies no turn")
	s.Equal(eqLongsword, out.Character.EquipmentSlots.Get(character.SlotMainHand))
	s.Equal([]EquipMove{{Slot: character.SlotMainHand, Ref: "dnd5e:weapons:handaxe"}}, out.Stowed)
	s.Equal([]EquipMove{{Slot: character.SlotMainHand, Ref: "dnd5e:weapons:longsword"}}, out.Drawn)
}

// Free roam, even with the stored action spent: nothing is priced, so nothing
// can be unaffordable.
func (s *EquipTestSuite) TestFreeRoamIgnoresASpentBank() {
	out, err := Equip(s.ctx, &EquipInput{
		Character: s.inFight(character.EquipmentSlots{character.SlotMainHand: eqLongsword}, 0),
		Slot:      character.SlotMainHand,
	})
	s.Require().NoError(err)
	s.Nil(out.Paid)
	s.Equal(0, out.Character.ActionEconomy.ActionsRemaining)
	s.Equal(1, out.Character.ActionEconomy.Granted[character.GrantedObjectInteractions])
	s.Empty(out.Character.EquipmentSlots.Get(character.SlotMainHand))
}

// The free-roam hands diff is this package's; the fight's is the rulebook's.
// Each change here is made both ways and the two must name the same items.
func (s *EquipTestSuite) TestFreeRoamNamesTheSameHandsAsTheFightsPrice() {
	cases := []struct {
		name   string
		hands  character.EquipmentSlots
		slot   character.InventorySlot
		itemID string
	}{
		{name: "draw", hands: character.EquipmentSlots{}, slot: character.SlotMainHand, itemID: eqLongsword},
		{name: "stow", hands: character.EquipmentSlots{character.SlotMainHand: eqLongsword}, slot: character.SlotMainHand},
		{
			name:  "swap",
			hands: character.EquipmentSlots{character.SlotMainHand: eqHandaxe}, slot: character.SlotMainHand, itemID: eqLongsword,
		},
		{
			name: "two-hander over both hands",
			hands: character.EquipmentSlots{
				character.SlotMainHand: eqLongsword, character.SlotOffHand: eqHandaxe,
			},
			slot: character.SlotMainHand, itemID: eqGreatsword,
		},
		{
			name:  "move between hands",
			hands: character.EquipmentSlots{character.SlotMainHand: eqLongsword}, slot: character.SlotOffHand, itemID: eqLongsword,
		},
		{name: "don shield", hands: character.EquipmentSlots{}, slot: character.SlotOffHand, itemID: eqShield},
		{name: "doff shield", hands: character.EquipmentSlots{character.SlotOffHand: eqShield}, slot: character.SlotOffHand},
	}

	for _, tc := range cases {
		s.Run(tc.name, func() {
			fight, err := Equip(s.ctx, &EquipInput{
				Character: s.inFight(tc.hands, 2),
				Slot:      tc.slot, ItemID: tc.itemID, Fight: eqThisTurn(),
			})
			s.Require().NoError(err)

			roam, err := Equip(s.ctx, &EquipInput{
				Character: s.fighter(tc.hands),
				Slot:      tc.slot, ItemID: tc.itemID,
			})
			s.Require().NoError(err)

			s.Equal(fight.Stowed, roam.Stowed)
			s.Equal(fight.Drawn, roam.Drawn)
			s.Equal(fight.Character.EquipmentSlots, roam.Character.EquipmentSlots)
		})
	}
}

func (s *EquipTestSuite) TestRequestsTheSheetCannotMakeAreBadEquip() {
	s.Run("nil input", func() {
		out, err := Equip(s.ctx, nil)
		s.Require().ErrorIs(err, ErrNilInput)
		s.Require().Nil(out)
	})

	s.Run("no character", func() {
		out, err := Equip(s.ctx, &EquipInput{Slot: character.SlotMainHand})
		s.Require().ErrorIs(err, ErrBadParticipant)
		s.Require().Nil(out)
	})

	s.Run("no slot", func() {
		out, err := Equip(s.ctx, &EquipInput{Character: s.fighter(character.EquipmentSlots{})})
		s.Require().ErrorIs(err, ErrBadEquip)
		s.Require().Nil(out)
	})

	s.Run("an item not carried, free roam", func() {
		out, err := Equip(s.ctx, &EquipInput{
			Character: s.fighter(character.EquipmentSlots{}),
			Slot:      character.SlotMainHand, ItemID: "rapier",
		})
		s.Require().ErrorIs(err, ErrBadEquip)
		s.Require().Nil(out)
	})

	s.Run("an item not carried, in a fight", func() {
		out, err := Equip(s.ctx, &EquipInput{
			Character: s.inFight(character.EquipmentSlots{}, 1),
			Slot:      character.SlotMainHand, ItemID: "rapier", Fight: eqThisTurn(),
		})
		s.Require().ErrorIs(err, ErrBadEquip)
		s.Require().NotErrorIs(err, ErrCannotPay)
		s.Require().Nil(out)
	})

	// A fight's change against a sheet with no stored economy: the refresh has
	// nothing to ready, and the rulebook will not price an absent turn.
	s.Run("in a fight with no readied turn", func() {
		out, err := Equip(s.ctx, &EquipInput{
			Character: s.fighter(character.EquipmentSlots{}),
			Slot:      character.SlotMainHand, ItemID: eqLongsword, Fight: eqThisTurn(),
		})
		s.Require().ErrorIs(err, ErrBadEquip)
		s.Require().Nil(out)
	})
}

// Each move names its slot and the item's full ref: a shield is armour, and a
// two-hander drawn over two held items stows both from the hands they held.
func (s *EquipTestSuite) TestMovesNameTheSlotAndTheFullRef() {
	shieldOn, err := Equip(s.ctx, &EquipInput{
		Character: s.fighter(character.EquipmentSlots{}),
		Slot:      character.SlotOffHand, ItemID: eqShield,
	})
	s.Require().NoError(err)
	s.Equal([]EquipMove{{Slot: character.SlotOffHand, Ref: "dnd5e:armor:shield"}}, shieldOn.Drawn)

	twoHanded, err := Equip(s.ctx, &EquipInput{
		Character: s.inFight(character.EquipmentSlots{
			character.SlotMainHand: eqLongsword, character.SlotOffHand: eqHandaxe,
		}, 3),
		Slot: character.SlotMainHand, ItemID: eqGreatsword, Fight: eqThisTurn(),
	})
	s.Require().NoError(err)
	s.Equal([]EquipMove{
		{Slot: character.SlotMainHand, Ref: "dnd5e:weapons:longsword"},
		{Slot: character.SlotOffHand, Ref: "dnd5e:weapons:handaxe"},
	}, twoHanded.Stowed)
	s.Equal([]EquipMove{{Slot: character.SlotMainHand, Ref: "dnd5e:weapons:greatsword"}}, twoHanded.Drawn)
}

// The returned record is the caller's own: writing it does not reach the
// input record.
func (s *EquipTestSuite) TestTheReturnedRecordIsIndependent() {
	input := s.fighter(character.EquipmentSlots{character.SlotMainHand: eqHandaxe})

	out, err := Equip(s.ctx, &EquipInput{
		Character: input, Slot: character.SlotMainHand, ItemID: eqLongsword,
	})
	s.Require().NoError(err)

	s.Equal(eqHandaxe, input.EquipmentSlots.Get(character.SlotMainHand), "the input was not applied in place")
	out.Character.EquipmentSlots[character.SlotOffHand] = "anything"
	s.Empty(input.EquipmentSlots.Get(character.SlotOffHand))
}
