// Copyright (C) 2026 Kirk Diggler
// SPDX-License-Identifier: GPL-3.0-or-later

package session_test

import (
	"context"
	"encoding/json"
	"os"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/KirkDiggler/rpg-toolkit/core"
	coreResources "github.com/KirkDiggler/rpg-toolkit/core/resources"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/character"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/encounter"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/encounter/dungeonspec"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/magicitems"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/refs"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/resources"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/session"
)

// worn_item_row_test.go plays "a worn item's ability is its own row"
// (rpg-project#1164, story rpg-project#850) on the second-wind walk site: a
// fighter wearing a cloak that grants Second Wind sees two Second Wind rows,
// the member's own and the cloak's, and each runs and is paid on its own.
//
// The dice are secondWindDice: every die is min(size, 10), so a d10 is a 10.

const (
	// The member's own Second Wind row on this site with nothing worn, captured
	// on origin/main (bc93f94f) before activations.go changed: Afford for alice
	// on turn 1 of testdata/second-wind-walk.yaml, session "sess".
	ownSecondWindID   = "v2.B-E_FF4_AoxcpUkZHlAQQWFnPoVOrjnLkQXLC_slPeA"
	ownSecondWindName = "Second Wind"

	cloakName     = "Lieutenant's cloak"
	cloakRowName  = "Second Wind · Lieutenant's cloak"
	secondWindRef = "dnd5e:features:second_wind"
)

func wornDescription(id, name string, slots ...string) magicitems.Description {
	return magicitems.Description{
		Ref:   core.Ref{Module: "site", Type: "items", ID: core.ID(id)},
		Name:  name,
		Slots: slots,
		Pools: map[string]magicitems.Pool{
			"charges": {Maximum: 1, RestoreOn: []coreResources.ResetType{
				coreResources.ResetShortRest, coreResources.ResetLongRest,
			}},
		},
		Grants: map[string]magicitems.Grant{
			"second-wind": {
				Ability:    *refs.Features.SecondWind(),
				Parameters: json.RawMessage(`{"level":1}`),
				Pool:       "charges",
				Uses:       1,
			},
		},
	}
}

func cloakItem(t *testing.T, id string) magicitems.Data {
	t.Helper()
	item, err := magicitems.New(id, wornDescription("lieutenants-cloak", cloakName, "cloak"))
	require.NoError(t, err)
	return item
}

func ringItem(t *testing.T, id string) magicitems.Data {
	t.Helper()
	item, err := magicitems.New(id, wornDescription("ring-"+id, "Ring "+id, "ring_left", "ring_right"))
	require.NoError(t, err)
	return item
}

// wear puts items on saved data and wears each in its slot.
func wear(t *testing.T, data *character.Data, worn map[character.InventorySlot]magicitems.Data) *character.Data {
	t.Helper()
	for slot, item := range worn {
		require.NoError(t, character.AddItem(data, item))
		data.EquipmentSlots[slot] = item.ID
	}
	return data
}

// wornFight is alice in the second-wind-walk fight, her turn first.
type wornFight struct {
	t     *testing.T
	mgr   *session.Manager
	chars *fakeCharacters
	alice string
}

func newWornFight(t *testing.T, alice *character.Data) *wornFight {
	t.Helper()
	hurt := alice.HitPoints
	raw, err := os.ReadFile("testdata/second-wind-walk.yaml")
	require.NoError(t, err)
	compiled, err := dungeonspec.Load(raw)
	require.NoError(t, err)
	f := &wornFight{t: t, chars: newFakeCharacters(alice), alice: "alice"}
	f.mgr, err = session.NewManager(&session.Config{
		Record: session.DiscardRecord{}, Acts: &actCapture{}, Seats: newFakeSeats(),
		CharacterConfig: newFakeCharacterConfig(), PresentationIDs: testPresentationIDs{}, Dice: &secondWindDice{},
		TurnDriver: session.Driver(), Sessions: newFakeSessions(), Encounters: newFakeEncounters(),
		Characters: f.chars,
	})
	require.NoError(t, err)
	_, err = f.mgr.Launch(context.Background(), &session.LaunchInput{
		Session: "sess", Dungeon: &compiled, Party: []string{"alice"},
	})
	require.NoError(t, err)
	turn, err := f.mgr.Turn(context.Background(), &session.TurnInput{Session: "sess", Member: "alice"})
	require.NoError(t, err)
	require.Equal(t, "alice", turn.Active)
	// A launch starts a member at full health; she is hurt once the fight is on.
	data := f.stored()
	data.HitPoints = hurt
	require.NoError(t, f.chars.SaveCharacter(context.Background(), data))
	return f
}

// rows is every Second Wind row alice is offered, in the panel's order.
func (f *wornFight) rows() []session.Declaration {
	f.t.Helper()
	panel, err := f.mgr.Afford(context.Background(), &session.AffordInput{Session: "sess", Member: "alice"})
	require.NoError(f.t, err)
	var out []session.Declaration
	for _, d := range panel.Declarations {
		if d.Verb == session.VerbActivate && d.Ability != nil && d.Ability.Ref == secondWindRef {
			out = append(out, d)
		}
	}
	return out
}

// rowOf is the row whose giver is the item, or the member's own for "".
func (f *wornFight) rowOf(item string) session.Declaration {
	f.t.Helper()
	for _, d := range f.rows() {
		switch {
		case item == "" && d.Ability.Giver == nil:
			return d
		case item != "" && d.Ability.Giver != nil && d.Ability.Giver.ItemID == item:
			return d
		}
	}
	f.t.Fatalf("alice is offered no Second Wind row for giver %q", item)
	return session.Declaration{}
}

func (f *wornFight) activate(id string) (*session.ActivateOutput, error) {
	return f.mgr.Activate(context.Background(), &session.ActivateInput{
		Session: "sess", Member: "alice", DeclarationID: id,
	})
}

func (f *wornFight) stored() *character.Data { return storedSheet(f.t, f.chars, "alice") }

// nextTurn ends alice's turn, lets the guard have its own, and returns to her.
func (f *wornFight) nextTurn() {
	f.t.Helper()
	_, err := f.mgr.EndTurn(context.Background(), &session.EndTurnInput{
		Session: "sess", Member: "alice", DeclarationID: currentEndTurnID(f.t, f.mgr, "sess", "alice"),
	})
	require.NoError(f.t, err)
	turn, err := f.mgr.Turn(context.Background(), &session.TurnInput{Session: "sess", Member: "alice"})
	require.NoError(f.t, err)
	require.Equal(f.t, "alice", turn.Active, "it is alice's turn again")
}

func itemLeft(data *character.Data, id string) int {
	for _, item := range data.Items {
		if item.ID == id {
			return item.Left["charges"]
		}
	}
	return -1
}

// Test 1. The id and name are the ones the fighter's own row had before this
// slice changed anything, written down from that run, so a change to how an id
// is made cannot go unnoticed: with the cloak worn, the own row is the same.
func TestTheMembersOwnSecondWindRowIsUntouched(t *testing.T) {
	bare := newWornFight(t, secondWindFighter(t, "alice", 10, 40))
	own := bare.rowOf("")
	require.Equal(t, ownSecondWindID, own.ID)
	require.Equal(t, ownSecondWindName, own.Ability.Name)

	cloak := cloakItem(t, "cloak-1")
	worn := newWornFight(t, wear(t, secondWindFighter(t, "alice", 10, 40),
		map[character.InventorySlot]magicitems.Data{character.SlotCloak: cloak}))
	own = worn.rowOf("")
	require.Equal(t, ownSecondWindID, own.ID, "wearing the cloak changes no id of the member's own")
	require.Equal(t, ownSecondWindName, own.Ability.Name)
}

// Test 2.
func TestAWornCloakIsASecondWindRowBesideTheFightersOwn(t *testing.T) {
	f := newWornFight(t, wear(t, secondWindFighter(t, "alice", 10, 40),
		map[character.InventorySlot]magicitems.Data{character.SlotCloak: cloakItem(t, "cloak-1")}))

	rows := f.rows()
	require.Len(t, rows, 2)
	own, cloak := f.rowOf(""), f.rowOf("cloak-1")
	require.NotEqual(t, own.ID, cloak.ID)
	require.Equal(t, "Second Wind", own.Ability.Name)
	require.Equal(t, cloakRowName, cloak.Ability.Name)
	require.Equal(t, secondWindRef, cloak.Ability.Ref, "the row's ref is still the ability's")
	require.Equal(t, &session.AbilityGiver{ItemID: "cloak-1", ItemName: cloakName, Grant: "second-wind"},
		cloak.Ability.Giver)
	require.Nil(t, own.Ability.Giver)
	require.True(t, own.Available)
	require.True(t, cloak.Available)
	require.Equal(t, []string{own.ID, cloak.ID}, []string{rows[0].ID, rows[1].ID},
		"the member's own comes before an item's")
}

// Tests 3 and 4.
func TestTheCloaksRowRunsAndPaysTheCloaksOwnUse(t *testing.T) {
	f := newWornFight(t, wear(t, secondWindFighter(t, "alice", 10, 40),
		map[character.InventorySlot]magicitems.Data{character.SlotCloak: cloakItem(t, "cloak-1")}))
	before := f.stored().HitPoints
	cloakID := f.rowOf("cloak-1").ID

	_, err := f.activate(cloakID)
	require.NoError(t, err)

	saved := f.stored()
	require.Equal(t, before+11, saved.HitPoints, "the d10 (a 10) plus the cloak's 1")
	require.Zero(t, saved.ActionEconomy.BonusActionsRemaining, "the bonus action is spent")
	require.Zero(t, itemLeft(saved, "cloak-1"), "the cloak's charge is spent")
	require.Equal(t, 1, saved.Resources[resources.SecondWind].Current, "the fighter's own use is untouched")

	var activated activatedBeat
	beats := beatsNamed(storyBeats(t, f.mgr, "sess", "alice"), encounter.BeatActivated)
	require.Len(t, beats, 1)
	beats[0].decode(t, &activated)
	require.Equal(t, session.SpellRef{Ref: secondWindRef, Name: cloakRowName}, activated.Ability)

	// The next turn: the cloak's row is spent, with a reason that names the
	// cloak; the member's own is there and does its own work.
	f.nextTurn()
	cloakRow := f.rowOf("cloak-1")
	require.False(t, cloakRow.Available)
	require.NotNil(t, cloakRow.Why)
	require.Contains(t, cloakRow.Why.Text, cloakName)
	own := f.rowOf("")
	require.True(t, own.Available)

	hurt := f.stored().HitPoints
	_, err = f.activate(own.ID)
	require.NoError(t, err)
	saved = f.stored()
	require.Equal(t, hurt+13, saved.HitPoints, "the d10 plus fighter level 3")
	require.Zero(t, saved.Resources[resources.SecondWind].Current, "the member's own pool is spent")
	require.Zero(t, itemLeft(saved, "cloak-1"))
}

// Test 5.
func TestTheCloaksRowIsStaleOnceTheCloakIsOff(t *testing.T) {
	f := newWornFight(t, wear(t, secondWindFighter(t, "alice", 10, 40),
		map[character.InventorySlot]magicitems.Data{character.SlotCloak: cloakItem(t, "cloak-1")}))
	cloakID := f.rowOf("cloak-1").ID

	data := f.stored()
	delete(data.EquipmentSlots, character.SlotCloak)
	require.NoError(t, f.chars.SaveCharacter(context.Background(), data))
	before := f.stored()

	out, err := f.activate(cloakID)
	require.Nil(t, out)
	require.ErrorIs(t, err, session.ErrStaleDeclaration)
	after := f.stored()
	require.Equal(t, before.HitPoints, after.HitPoints)
	require.Equal(t, 1, itemLeft(after, "cloak-1"), "nothing is spent")
	require.Equal(t, 1, after.Resources[resources.SecondWind].Current)
	require.Len(t, f.rows(), 1, "only the member's own row is left")
}

// Test 6.
func TestACloakInThePackAddsNoRow(t *testing.T) {
	data := secondWindFighter(t, "alice", 10, 40)
	require.NoError(t, character.AddItem(data, cloakItem(t, "cloak-1")))
	f := newWornFight(t, data)

	rows := f.rows()
	require.Len(t, rows, 1)
	require.Nil(t, rows[0].Ability.Giver)
}

// Test 7. A member with no Second Wind of their own.
func TestAMemberWithoutSecondWindWearingTheCloakHasOnlyTheCloaksRow(t *testing.T) {
	data := armedFighter("alice")
	data.HitPoints, data.MaxHitPoints = 10, 40
	f := newWornFight(t, wear(t, data,
		map[character.InventorySlot]magicitems.Data{character.SlotCloak: cloakItem(t, "cloak-1")}))

	rows := f.rows()
	require.Len(t, rows, 1)
	require.NotNil(t, rows[0].Ability.Giver)
	require.Equal(t, cloakRowName, rows[0].Ability.Name)

	_, err := f.activate(rows[0].ID)
	require.NoError(t, err)
	saved := f.stored()
	require.Equal(t, 21, saved.HitPoints, "10 plus the d10 (a 10) plus the cloak's 1")
	require.Zero(t, itemLeft(saved, "cloak-1"))
}

// Test 8. Putting it on in the run: free roam takes the cloak, a fight does not.
func TestPuttingTheCloakOnInTheRunAddsItsRow(t *testing.T) {
	sessions, encounters := newFakeSessions(), newFakeEncounters()
	alice := quickFighter("alice")
	require.NoError(t, character.AddItem(alice, cloakItem(t, "cloak-1")))
	chars := newFakeCharacters(alice, quickFighter("bob"))
	seats := newFakeSeats()
	mgr, err := session.NewManager(&session.Config{Record: session.DiscardRecord{}, Acts: &actCapture{}, Seats: seats,
		CharacterConfig: newFakeCharacterConfig(), PresentationIDs: testPresentationIDs{}, Dice: testDice{},
		TurnDriver: session.Pass{}, Sessions: sessions, Encounters: encounters, Characters: chars})
	require.NoError(t, err)
	launchScene(t, mgr, freeRoamDuelWorld())
	for _, id := range []string{"alice", "bob"} {
		require.NoError(t, seats.SaveSeat(context.Background(), &session.SeatData{Character: id, Session: "sess"}))
	}

	activations := func(m *session.Manager) []session.Declaration {
		panel, err := m.Afford(context.Background(), &session.AffordInput{Session: "sess", Member: "alice"})
		require.NoError(t, err)
		var out []session.Declaration
		for _, d := range panel.Declarations {
			if d.Verb == session.VerbActivate && d.Ability != nil && d.Ability.Giver != nil {
				out = append(out, d)
			}
		}
		return out
	}
	require.Empty(t, activations(mgr), "in the pack the cloak grants nothing")

	_, err = mgr.Equip(context.Background(), &session.EquipInput{
		Character: "alice", Slot: string(character.SlotCloak), Item: "cloak-1",
	})
	require.NoError(t, err)
	rows := activations(mgr)
	require.Len(t, rows, 1)
	require.Equal(t, cloakRowName, rows[0].Ability.Name)

	// In a fight the same verb is refused.
	fightSessions, fightEncounters := newFakeSessions(), newFakeEncounters()
	fighter := quickFighter("alice")
	require.NoError(t, character.AddItem(fighter, cloakItem(t, "cloak-1")))
	fightChars, fightSeats := newFakeCharacters(fighter, quickFighter("bob")), newFakeSeats()
	fight, err := session.NewManager(&session.Config{Record: session.DiscardRecord{}, Acts: &actCapture{}, Seats: fightSeats,
		CharacterConfig: newFakeCharacterConfig(), PresentationIDs: testPresentationIDs{}, Dice: testDice{},
		TurnDriver: session.Pass{}, Sessions: fightSessions, Encounters: fightEncounters, Characters: fightChars})
	require.NoError(t, err)
	launchDuel(t, fight, fightEncounters)
	for _, id := range []string{"alice", "bob"} {
		require.NoError(t, fightSeats.SaveSeat(context.Background(), &session.SeatData{Character: id, Session: "sess"}))
	}
	_, err = fight.Equip(context.Background(), &session.EquipInput{
		Character: "alice", Slot: string(character.SlotCloak), Item: "cloak-1",
	})
	// The refusal is the rulebook's, relayed as ErrArmorInFight: a worn slot
	// is not a hand, and the fight does not let one change.
	require.ErrorIs(t, err, session.ErrArmorInFight)
	require.Empty(t, storedSheet(t, fightChars, "alice").EquipmentSlots[character.SlotCloak], "nothing was worn")
}

// Test 9. Two rings, each granting Second Wind: three rows, three ids; using
// one leaves the other available on a later turn.
func TestTwoRingsAreTwoMoreRowsEachPaidOnItsOwn(t *testing.T) {
	f := newWornFight(t, wear(t, secondWindFighter(t, "alice", 10, 60),
		map[character.InventorySlot]magicitems.Data{
			character.SlotRingLeft:  ringItem(t, "ring-a"),
			character.SlotRingRight: ringItem(t, "ring-b"),
		}))

	rows := f.rows()
	require.Len(t, rows, 3)
	require.Len(t, map[string]struct{}{rows[0].ID: {}, rows[1].ID: {}, rows[2].ID: {}}, 3, "three ids")
	require.Nil(t, rows[0].Ability.Giver, "the member's own first")
	require.Equal(t, "ring-a", rows[1].Ability.Giver.ItemID, "then the items by id")
	require.Equal(t, "ring-b", rows[2].Ability.Giver.ItemID)

	_, err := f.activate(f.rowOf("ring-a").ID)
	require.NoError(t, err)
	f.nextTurn()
	require.False(t, f.rowOf("ring-a").Available)
	require.True(t, f.rowOf("ring-b").Available)
	require.True(t, f.rowOf("").Available)

	_, err = f.activate(f.rowOf("ring-b").ID)
	require.NoError(t, err)
	saved := f.stored()
	require.Zero(t, itemLeft(saved, "ring-a"))
	require.Zero(t, itemLeft(saved, "ring-b"))
	require.Equal(t, 1, saved.Resources[resources.SecondWind].Current, "the member's own was never touched")
}
