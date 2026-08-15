// Copyright (C) 2026 Kirk Diggler
// SPDX-License-Identifier: GPL-3.0-or-later

package session_test

import (
	"context"
	"testing"

	"github.com/stretchr/testify/suite"

	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/abilities"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/character"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/classes"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/encounter"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/proficiencies"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/races"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/session"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/shared"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/weapons"
	"github.com/KirkDiggler/rpg-toolkit/tools/spatial"
)

// AttackTestSuite covers the swing — the wave's last verb, and the first that
// drives a rules machine through the seam.
type AttackTestSuite struct {
	suite.Suite

	sessions   *fakeSessions
	encounters *fakeEncounters
	characters *fakeCharacters
}

func TestAttackSuite(t *testing.T) {
	suite.Run(t, new(AttackTestSuite))
}

// armedFighter is a sheet that can actually swing: a longsword in the main
// hand and the proficiency to use it.
//
// The shared dwarf fixture carries no weapon, which is correct for every other
// verb and useless here — an empty hand is refused by the compiler rather than
// falling back to an unarmed strike, deliberately.
func armedFighter(id string) *character.Data {
	return &character.Data{
		ID:       id,
		PlayerID: "player-" + id,
		Name:     id,
		Level:    3,
		ClassID:  classes.Fighter,
		RaceID:   races.Human,
		AbilityScores: shared.AbilityScores{
			abilities.STR: 16, abilities.DEX: 14, abilities.CON: 14,
			abilities.INT: 10, abilities.WIS: 12, abilities.CHA: 8,
		},
		HitPoints:           24,
		MaxHitPoints:        28,
		ArmorClass:          16,
		ProficiencyBonus:    2,
		WeaponProficiencies: []proficiencies.Weapon{proficiencies.WeaponMartial},
		Inventory: []character.InventoryItemData{
			{Type: shared.EquipmentTypeWeapon, ID: string(weapons.Longsword), Quantity: 1},
		},
		EquipmentSlots: character.EquipmentSlots{
			character.SlotMainHand: string(weapons.Longsword),
		},
	}
}

// duelAC is what these fixtures actually defend with: 12.
//
// NOT the sheet's ArmorClass field, which is 16 and inert. The strike resolves
// against the EFFECTIVE armour class — folded on the interaction's own bus from
// armour worn and Dexterity — and an unarmoured fighter with DEX 14 is 10 + 2.
// Worth stating rather than discovering twice: a fixture that raises the stored
// field to make a swing miss changes nothing at all.
const duelAC = 12

// duel is two armed characters standing next to each other, which is the
// smallest world where a swing means anything.
//
// Both sides are CHARACTERS on purpose: damage dirties the target's stored
// sheet, which is the case that earns Attack its row in the no-clobber pin.
func (s *AttackTestSuite) duel(dice session.Roller) *session.Manager {
	s.sessions, s.encounters = newFakeSessions(), newFakeEncounters()
	s.characters = newFakeCharacters(armedFighter("alice"), armedFighter("bob"))

	mgr, err := session.NewManager(&session.Config{
		Dice: dice, Sessions: s.sessions, Encounters: s.encounters,
		Characters: s.characters, Events: session.DiscardEvents{},
	})
	s.Require().NoError(err)

	enc, err := encounter.NewEncounter(&encounter.SetupInput{
		Initiative: encOrderAsGiven{},
		Field:      encounter.FieldInput{Rooms: []encounter.RoomInput{{ID: "hall", Width: 8, Height: 8}}},
		Members: []encounter.MemberInput{
			{ID: "alice", Kind: encounter.KindPlayer, Room: "hall", Position: spatial.Position{X: 1, Y: 1}},
			{ID: "bob", Kind: encounter.KindPlayer, Room: "hall", Position: spatial.Position{X: 2, Y: 1}},
		},
		Endings:   []encounter.EndingInput{{Key: "withdrawn", Trigger: encounter.TriggerExternal{}}},
		Retention: encounter.RetentionUnbounded,
	})
	s.Require().NoError(err)
	data := enc.ToData()

	_, err = mgr.StartSession(context.Background(), &session.StartSessionInput{
		Session: "sess", Encounter: "world", World: &data,
	})
	s.Require().NoError(err)
	return mgr
}

func (s *AttackTestSuite) swing(mgr *session.Manager) (*session.AttackOutput, error) {
	return mgr.Attack(context.Background(), &session.AttackInput{
		Session: "sess", Attacker: "alice", Target: "bob",
	})
}

// TestASwingLandsAndTheStoryRecordsIt is the headline: a rules machine runs
// through the seam, and what it produced reaches both the caller and the
// transcript.
//
// Exact arithmetic rather than "damage is positive". A d20 of 15 plus a
// proficient Strength longsword (+3 STR, +2 proficiency) totals 20 against
// AC 12 — a hit, and the damage die scripted to 5 with the +3 modifier deals
// 8. A loose assertion here would pass on an implementation that dropped the
// modifier, halved the die, or resolved against the wrong sheet.
func (s *AttackTestSuite) TestASwingLandsAndTheStoryRecordsIt() {
	mgr := s.duel(&sequenceDice{rolls: []int{15, 5}})

	out, err := s.swing(mgr)
	s.Require().NoError(err)
	s.Equal(15, out.Roll)
	s.Equal(20, out.Total, "15 + 3 STR + 2 proficiency")
	s.Equal(duelAC, out.Against, "effective AC: unarmoured, DEX 14")
	s.True(out.Hit)
	s.False(out.Critical)
	s.Equal(8, out.Damage, "d8 scripted to 5, plus 3 STR")
	s.NotZero(out.Seq)

	story, err := mgr.Story(context.Background(), &session.StoryInput{Session: "sess", Member: "bob"})
	s.Require().NoError(err)
	last := story[len(story)-1]
	s.Equal(out.Seq, last.Seq, "AttackOutput.Seq references the recorded beat")
	s.Equal("outcome", last.Tags["tag"])
	s.JSONEq(
		`{"beat":"struck","actor":"alice","targets":["bob"],"roll":15,"total":20,"against":12,"amount":8}`,
		string(last.Payload))
}

// TestAMissIsRecordedToo pins the other arm, including that a miss carries no
// amount — a beat saying "missed for 0" would read as a hit that did nothing.
func (s *AttackTestSuite) TestAMissIsRecordedToo() {
	mgr := s.duel(&sequenceDice{rolls: []int{2, 5}})

	out, err := s.swing(mgr)
	s.Require().NoError(err)
	s.False(out.Hit, "2 + 5 is under 12")
	s.Zero(out.Damage)

	story, err := mgr.Story(context.Background(), &session.StoryInput{Session: "sess", Member: "alice"})
	s.Require().NoError(err)
	s.JSONEq(
		`{"beat":"missed","actor":"alice","targets":["bob"],"roll":2,"total":7,"against":12}`,
		string(story[len(story)-1].Payload))
}

// TestDamagePersists is the row Attack earns in the no-clobber pin.
//
// Every other verb leaves the character store untouched and TestAVerbLeaves-
// TheCharacterStoreUntouched guards that. This is the one that must write:
// damage that did not persist is damage that did not happen. The target is a
// CHARACTER precisely so the write lands in the host's repository rather than
// in the session's own NPC list.
func (s *AttackTestSuite) TestDamagePersists() {
	mgr := s.duel(&sequenceDice{rolls: []int{15, 5}})
	before := s.characters.byID["bob"].HitPoints

	out, err := s.swing(mgr)
	s.Require().NoError(err)
	s.Require().True(out.Hit)

	s.Equal(before-out.Damage, s.characters.byID["bob"].HitPoints,
		"the blow reached the stored sheet")
	s.Equal(24, s.characters.byID["alice"].HitPoints, "and the swinger is untouched")
}

// TestNothingSpendsYet is the known gap, pinned so it cannot be mistaken for a
// decision that attacks are free.
//
// A character may swing as many times in a turn as the caller asks, because
// the thing that spends is an economy machine above this one and it does not
// exist. The verb names the single place it will go. When it lands, THIS test
// is the one that should fail — and its failure is the signal to write the
// real economy assertions rather than to relax this one.
func (s *AttackTestSuite) TestNothingSpendsYet() {
	mgr := s.duel(&sequenceDice{rolls: []int{15, 5, 15, 5, 15, 5}})

	for i := 0; i < 3; i++ {
		out, err := s.swing(mgr)
		s.Require().NoError(err, "swing %d", i+1)
		s.True(out.Hit, "swing %d", i+1)
	}

	s.Equal(28-24, 4, "sanity: the fixture starts damaged, so HP is not clamped at max")
	s.Less(s.characters.byID["bob"].HitPoints, 24,
		"three unspent swings all landed — nothing anywhere is counting actions")
}

// TestAMonsterAttackerIsRefused pins v1's scope as a decision with a successor.
//
// A monster's action can declare a save gate, which makes a strike's rider —
// a second interaction with its own DC and imposed effects — reachable, and
// this seam has no vocabulary for recording one. So the case is refused by
// name until the behavior work that calls for it arrives, exactly as defeat
// waits for the composition to be able to see it.
func (s *AttackTestSuite) TestAMonsterAttackerIsRefused() {
	s.sessions, s.encounters = newFakeSessions(), newFakeEncounters()
	s.characters = newFakeCharacters(armedFighter("alice"))
	mgr, err := session.NewManager(&session.Config{
		Dice: testDice{}, Sessions: s.sessions, Encounters: s.encounters,
		Characters: s.characters, Events: session.DiscardEvents{},
	})
	s.Require().NoError(err)

	enc, err := encounter.NewEncounter(&encounter.SetupInput{
		Initiative: encOrderAsGiven{},
		Field:      encounter.FieldInput{Rooms: []encounter.RoomInput{{ID: "hall", Width: 8, Height: 8}}},
		Members: []encounter.MemberInput{
			{ID: "alice", Kind: encounter.KindPlayer, Room: "hall", Position: spatial.Position{X: 1, Y: 1}},
			{ID: "ogre", Kind: encounter.KindMonster, Room: "hall", Position: spatial.Position{X: 2, Y: 1}},
		},
		Endings: []encounter.EndingInput{{Key: "withdrawn", Trigger: encounter.TriggerExternal{}}},
	})
	s.Require().NoError(err)
	data := enc.ToData()
	_, err = mgr.StartSession(context.Background(), &session.StartSessionInput{
		Session: "sess", Encounter: "world", World: &data,
	})
	s.Require().NoError(err)

	_, err = mgr.Attack(context.Background(), &session.AttackInput{
		Session: "sess", Attacker: "ogre", Target: "alice",
	})
	s.ErrorIs(err, session.ErrNotACharacter)
}

// TestTheInputIsMemberNeutral pins the shape rather than the scope.
//
// v1 compiles character attackers only, but nothing in the INPUT says so —
// both sides are plain IDs. The day a monster attacker compiles, this input
// does not change and no host that wrote against it has to. Scope by the case,
// shape for the future.
func (s *AttackTestSuite) TestTheInputIsMemberNeutral() {
	s.Equal([]string{"Session", "Attacker", "Target"}, structFields(session.AttackInput{}),
		"a character-shaped field here would make the v1 scope permanent")
}

// TestRefusals covers the ways a caller can get it wrong.
func (s *AttackTestSuite) TestRefusals() {
	mgr := s.duel(testDice{})
	ctx := context.Background()

	_, err := mgr.Attack(ctx, nil)
	s.ErrorIs(err, session.ErrNilInput)

	_, err = mgr.Attack(ctx, &session.AttackInput{Session: "sess", Target: "bob"})
	s.ErrorIs(err, session.ErrNoMemberID)

	_, err = mgr.Attack(ctx, &session.AttackInput{Session: "sess", Attacker: "alice"})
	s.ErrorIs(err, session.ErrNoMemberID)

	_, err = mgr.Attack(ctx, &session.AttackInput{Session: "sess", Attacker: "nobody", Target: "bob"})
	s.ErrorIs(err, session.ErrNoMember)

	_, err = mgr.Attack(ctx, &session.AttackInput{Session: "sess", Attacker: "alice", Target: "nobody"})
	s.ErrorIs(err, session.ErrNoMember)
}

// TestAnEmptyHandIsRefused pins the compiler's own gap reaching the host under
// this package's sentinel rather than the rulebook's.
func (s *AttackTestSuite) TestAnEmptyHandIsRefused() {
	s.sessions, s.encounters = newFakeSessions(), newFakeEncounters()
	s.characters = newFakeCharacters(dwarfCharacter("alice"), armedFighter("bob"))
	mgr, err := session.NewManager(&session.Config{
		Dice: testDice{}, Sessions: s.sessions, Encounters: s.encounters,
		Characters: s.characters, Events: session.DiscardEvents{},
	})
	s.Require().NoError(err)

	enc, err := encounter.NewEncounter(&encounter.SetupInput{
		Initiative: encOrderAsGiven{},
		Field:      encounter.FieldInput{Rooms: []encounter.RoomInput{{ID: "hall", Width: 8, Height: 8}}},
		Members: []encounter.MemberInput{
			{ID: "alice", Kind: encounter.KindPlayer, Room: "hall", Position: spatial.Position{X: 1, Y: 1}},
			{ID: "bob", Kind: encounter.KindPlayer, Room: "hall", Position: spatial.Position{X: 2, Y: 1}},
		},
		Endings: []encounter.EndingInput{{Key: "withdrawn", Trigger: encounter.TriggerExternal{}}},
	})
	s.Require().NoError(err)
	data := enc.ToData()
	_, err = mgr.StartSession(context.Background(), &session.StartSessionInput{
		Session: "sess", Encounter: "world", World: &data,
	})
	s.Require().NoError(err)

	_, err = s.swing(mgr)
	s.ErrorIs(err, session.ErrBadAttack,
		"an unarmed character is refused rather than silently swinging for 1")
}
