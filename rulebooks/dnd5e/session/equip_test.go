// Copyright (C) 2026 Kirk Diggler
// SPDX-License-Identifier: GPL-3.0-or-later

package session_test

import (
	"context"
	"testing"

	"github.com/stretchr/testify/suite"

	coreResources "github.com/KirkDiggler/rpg-toolkit/core/resources"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/armor"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/character"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/resources"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/session"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/shared"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/weapons"
)

// EquipSuite covers rpg-project#542 slice 4's equip done-when: the seat
// decides the path, the price is the rulebook's at resolution's door, and
// nothing is written on a refusal.
type EquipSuite struct {
	suite.Suite

	sessions   *fakeSessions
	encounters *fakeEncounters
	characters *fakeCharacters
	seats      *fakeSeats
	stream     *fakeStream
	mgr        *session.Manager
}

func TestEquipSuite(t *testing.T) { suite.Run(t, new(EquipSuite)) }

// quickFighter is armedFighter carrying a dagger and leather armour besides
// the longsword in hand: something to draw, and something a fight refuses.
func quickFighter(id string) *character.Data {
	data := armedFighter(id)
	data.Inventory = append(data.Inventory,
		character.InventoryItemData{Type: shared.EquipmentTypeWeapon, ID: string(weapons.Dagger), Quantity: 1},
		character.InventoryItemData{Type: shared.EquipmentTypeArmor, ID: string(armor.Leather), Quantity: 1},
	)
	return data
}

// withHitDice gives a fixture a pool of hit dice to spend on a short rest.
func withHitDice(data *character.Data, n int) *character.Data {
	if data.Resources == nil {
		data.Resources = map[coreResources.ResourceKey]character.RecoverableResourceData{}
	}
	data.Resources[resources.HitDice] = character.RecoverableResourceData{
		Current: n, Maximum: n, ResetType: coreResources.ResetLongRest,
	}
	return data
}

// start wires a manager over world with alice and bob as quickFighters and
// launches it. The launch seats both in "sess"; an unseated scene takes the
// seats back out, so the characters stand in the run with no seat naming it.
func (s *EquipSuite) start(world scene, seated bool, dice session.Roller) {
	s.open(dice)
	launchScene(s.T(), s.mgr, world)
	s.settle(seated)
}

// startDuel is start over the duel on an authored turn clock, alice active.
func (s *EquipSuite) startDuel(seated bool, dice session.Roller) {
	s.open(dice)
	launchDuel(s.T(), s.mgr, s.encounters)
	s.settle(seated)
}

func (s *EquipSuite) open(dice session.Roller) {
	s.sessions, s.encounters = newFakeSessions(), newFakeEncounters()
	s.characters = newFakeCharacters(quickFighter("alice"), quickFighter("bob"))
	s.seats = newFakeSeats()
	s.stream = &fakeStream{}
	mgr, err := session.NewManager(&session.Config{Seats: s.seats, PresentationIDs: testPresentationIDs{},
		Dice: dice, TurnDriver: session.Pass{}, Sessions: s.sessions, Encounters: s.encounters,
		Characters: s.characters, Events: s.stream,
	})
	s.Require().NoError(err)
	s.mgr = mgr
}

func (s *EquipSuite) settle(seated bool) {
	if seated {
		s.seat("alice", "sess")
		s.seat("bob", "sess")
	} else {
		delete(s.seats.byID, "alice")
		delete(s.seats.byID, "bob")
	}
	s.stream.published = nil
}

func (s *EquipSuite) seat(character, sess string) {
	s.Require().NoError(s.seats.SaveSeat(context.Background(), &session.SeatData{Character: character, Session: sess}))
}

func (s *EquipSuite) stored(id string) *character.Data {
	data, err := s.characters.GetCharacter(context.Background(), id)
	s.Require().NoError(err)
	return data
}

// equipBeatsTo is every equip event delivered to recipient.
func (s *EquipSuite) equipBeatsTo(recipient string) []session.EquipmentChangedBody {
	var out []session.EquipmentChangedBody
	for _, event := range s.stream.published {
		if event.Kind != session.EventEquipmentChanged || event.Recipient != recipient {
			continue
		}
		body, ok := event.Body.(session.EquipmentChangedBody)
		s.Require().True(ok, "an equip event carries its typed body")
		out = append(out, body)
	}
	return out
}

func (s *EquipSuite) TestAnUnseatedEquipCostsNothingAndTellsNoBeat() {
	s.start(freeRoamDuelWorld(), false, testDice{})
	worldSaves := s.encounters.saves

	out, err := s.mgr.Equip(context.Background(), &session.EquipInput{
		Character: "alice", Slot: string(character.SlotOffHand), Item: string(weapons.Dagger),
	})
	s.Require().NoError(err)

	s.Empty(out.Session, "no run holds an unseated character")
	s.Equal(string(weapons.Dagger), s.stored("alice").EquipmentSlots[character.SlotOffHand])
	s.Equal([]string{"character:alice"}, out.Saved.Written)
	s.Empty(out.Seqs)
	s.Empty(s.stream.published, "an unseated change tells nobody anything")
	s.Equal(worldSaves, s.encounters.saves, "no session is touched")
	s.Require().Len(out.Drawn, 1)
	s.Equal(string(character.SlotOffHand), out.Drawn[0].Slot)
}

func (s *EquipSuite) TestASeatedFreeRoamEquipCostsNothingTellsOneBeatAndRechecksSight() {
	s.start(freeRoamDuelWorld(), true, testDice{})
	before := s.stored("alice").ActionEconomy
	s.Equal(&session.SeenEquipment{MainHand: string(weapons.Longsword)}, s.handsBobSees(),
		"the launch's one look already observed the longsword in alice's hand")

	out, err := s.mgr.Unequip(context.Background(), &session.UnequipInput{
		Character: "alice", Slot: string(character.SlotMainHand),
	})
	s.Require().NoError(err)

	s.Equal("sess", out.Session)
	s.Empty(s.stored("alice").EquipmentSlots[character.SlotMainHand])
	s.Equal(before, s.stored("alice").ActionEconomy, "free roam charges nothing")
	s.Len(out.Seqs, 1, "one item stowed, one beat")
	s.Contains(out.Saved.Written, "character:alice")
	s.Contains(out.Saved.Written, "encounter:sess")

	toBob := s.equipBeatsTo("bob")
	s.Require().Len(toBob, 1, "the watcher hears the stow")
	s.Equal(session.EquipmentChangedBody{
		Member: "alice", Slot: string(character.SlotMainHand),
		Item: "dnd5e:weapons:" + string(weapons.Longsword), Change: session.EquipmentStowed,
	}, toBob[0])
	s.Len(s.equipBeatsTo("alice"), 1, "the actor hears their own act")

	// Sight was rechecked inside the verb: bob's own view of alice shows
	// the empty hand without any second call.
	seen := s.handsBobSees()
	s.Require().NotNil(seen, "the recheck observed alice's hands")
	s.Empty(seen.MainHand, "bob sees alice's main hand empty now")
}

// handsBobSees is what bob's own snapshot of alice says she holds; nil when
// the hands were never observed.
func (s *EquipSuite) handsBobSees() *session.SeenEquipment {
	view, err := s.mgr.View(context.Background(), &session.ViewInput{Session: "sess", Member: "bob"})
	s.Require().NoError(err)
	for _, sighting := range view.Sightings {
		if sighting.Subject != "alice" || sighting.Seen == nil {
			continue
		}
		return sighting.Seen.Equipment
	}
	s.Fail("bob does not see alice")
	return nil
}

func (s *EquipSuite) TestASwapTellsTheStowBeforeTheDraw() {
	s.start(freeRoamDuelWorld(), true, testDice{})

	out, err := s.mgr.Equip(context.Background(), &session.EquipInput{
		Character: "alice", Slot: string(character.SlotMainHand), Item: string(weapons.Dagger),
	})
	s.Require().NoError(err)

	s.Len(out.Seqs, 2)
	toBob := s.equipBeatsTo("bob")
	s.Require().Len(toBob, 2)
	s.Equal(session.EquipmentStowed, toBob[0].Change)
	s.Equal(session.EquipmentDrawn, toBob[1].Change)
	s.Equal("dnd5e:weapons:"+string(weapons.Dagger), toBob[1].Item)
}

func (s *EquipSuite) TestAnInFightEquipOffTurnRefusesAndWritesNothing() {
	s.startDuel(true, testDice{})
	charSaves, worldSaves := s.characters.saves, s.encounters.saves

	_, err := s.mgr.Unequip(context.Background(), &session.UnequipInput{
		Character: "bob", Slot: string(character.SlotMainHand),
	})
	s.Require().ErrorIs(err, session.ErrNotYourTurn)

	s.Equal(charSaves, s.characters.saves)
	s.Equal(worldSaves, s.encounters.saves)
	s.Equal(string(weapons.Longsword), s.stored("bob").EquipmentSlots[character.SlotMainHand])
	s.Empty(s.stream.published)
}

func (s *EquipSuite) TestAnInFightStowWithTheActionSpentRefusesAndWritesNothing() {
	s.startDuel(true, &sequenceDice{rolls: []int{15, 5}})
	_, err := s.mgr.Attack(context.Background(), &session.AttackInput{
		Session: "sess", Attacker: "alice", Target: "bob",
		DeclarationID: currentAttackID(s.T(), s.mgr, "sess", "alice"),
	})
	s.Require().NoError(err, "the swing spends alice's action")
	s.stream.published = nil
	charSaves, worldSaves := s.characters.saves, s.encounters.saves

	_, err = s.mgr.Unequip(context.Background(), &session.UnequipInput{
		Character: "alice", Slot: string(character.SlotMainHand),
	})
	s.Require().ErrorIs(err, session.ErrCannotAfford)

	s.Equal(charSaves, s.characters.saves)
	s.Equal(worldSaves, s.encounters.saves)
	s.Equal(string(weapons.Longsword), s.stored("alice").EquipmentSlots[character.SlotMainHand])
	s.Empty(s.stream.published)
}

func (s *EquipSuite) TestAnInFightDrawOnTurnPaysTheInteractionAndTellsTheBeat() {
	s.startDuel(true, testDice{})

	out, err := s.mgr.Equip(context.Background(), &session.EquipInput{
		Character: "alice", Slot: string(character.SlotOffHand), Item: string(weapons.Dagger),
	})
	s.Require().NoError(err)

	s.Len(out.Seqs, 1)
	s.Equal(string(weapons.Dagger), s.stored("alice").EquipmentSlots[character.SlotOffHand])
	s.Len(s.equipBeatsTo("bob"), 1)
	s.Equal(1, s.stored("alice").ActionEconomy.ActionsRemaining, "a draw into an empty hand is the interaction, not the action")

	// The stow is the action.
	_, err = s.mgr.Unequip(context.Background(), &session.UnequipInput{
		Character: "alice", Slot: string(character.SlotMainHand),
	})
	s.Require().NoError(err)
	s.Equal(0, s.stored("alice").ActionEconomy.ActionsRemaining, "putting a held item away is the action")

	// The interaction is spent, so a second draw costs the action, which is
	// gone too.
	_, err = s.mgr.Equip(context.Background(), &session.EquipInput{
		Character: "alice", Slot: string(character.SlotMainHand), Item: string(weapons.Longsword),
	})
	s.Require().ErrorIs(err, session.ErrCannotAfford)
	s.Empty(s.stored("alice").EquipmentSlots[character.SlotMainHand])
}

func (s *EquipSuite) TestBodyArmourInAFightRefusesAndWritesNothing() {
	s.startDuel(true, testDice{})
	charSaves := s.characters.saves

	_, err := s.mgr.Equip(context.Background(), &session.EquipInput{
		Character: "alice", Slot: string(character.SlotArmor), Item: string(armor.Leather),
	})
	s.Require().ErrorIs(err, session.ErrArmorInFight)
	s.Equal(charSaves, s.characters.saves)
}

func (s *EquipSuite) TestADownedMemberInAFightRefusesAndWritesNothing() {
	s.startDuel(true, testDice{})
	alice := s.stored("alice")
	alice.HitPoints = 0
	s.Require().NoError(s.characters.SaveCharacter(context.Background(), alice))
	charSaves := s.characters.saves

	_, err := s.mgr.Unequip(context.Background(), &session.UnequipInput{
		Character: "alice", Slot: string(character.SlotMainHand),
	})
	s.Require().ErrorIs(err, session.ErrDowned)
	s.Equal(charSaves, s.characters.saves)
}

func (s *EquipSuite) TestAnItemTheInventoryDoesNotHoldIsABadEquip() {
	s.start(freeRoamDuelWorld(), true, testDice{})
	charSaves := s.characters.saves

	_, err := s.mgr.Equip(context.Background(), &session.EquipInput{
		Character: "alice", Slot: string(character.SlotOffHand), Item: string(weapons.Greataxe),
	})
	s.Require().ErrorIs(err, session.ErrBadEquip)
	s.Equal(charSaves, s.characters.saves)
	s.Empty(s.stream.published)
}

func (s *EquipSuite) TestASeatNamingARunThatDoesNotHoldTheCharacterRefuses() {
	s.start(freeRoamDuelWorld(), false, testDice{})
	s.characters.byID["carol"] = quickFighter("carol")
	s.seat("carol", "sess")

	_, err := s.mgr.Equip(context.Background(), &session.EquipInput{
		Character: "carol", Slot: string(character.SlotOffHand), Item: string(weapons.Dagger),
	})
	s.Require().ErrorIs(err, session.ErrNoMember)
}
