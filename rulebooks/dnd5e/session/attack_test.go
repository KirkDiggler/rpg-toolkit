// Copyright (C) 2026 Kirk Diggler
// SPDX-License-Identifier: GPL-3.0-or-later

package session_test

import (
	"context"
	"encoding/json"
	"testing"

	"github.com/stretchr/testify/suite"

	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/abilities"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/character"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/classes"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/encounter"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/proficiencies"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/races"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/refs"
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

// unarmedFighter is armedFighter's twin with nothing equipped: the fixture
// TestAnEmptyHandThrowsAnUnarmedStrike proves the unarmed catalog entry, not
// a compiler gap (rpg-toolkit#1168).
func unarmedFighter(id string) *character.Data {
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
		HitPoints:        24,
		MaxHitPoints:     28,
		ArmorClass:       16,
		ProficiencyBonus: 2,
	}
}

// armedFighter is a sheet that can actually swing: a longsword in the main
// hand and the proficiency to use it.
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

// duelWorld is two armed characters standing next to each other, which is the
// smallest world where a swing means anything.
//
// Both sides are CHARACTERS on purpose: damage dirties the target's stored
// sheet, which is the case that earns Attack its row in the no-clobber pin.
//
// Shared with the event-kind pins (attackevents_test.go), which need the same
// duel delivered to a real stream rather than discarded. One world, so a
// fixture drift cannot make the two suites disagree about what was swung at.
func duelWorld(t fataler) *encounter.EncounterData {
	enc, err := encounter.NewEncounter(&encounter.SetupInput{Sight: encEveryoneSees{},
		Initiative: encOrderAsGiven{}, TurnDriver: encPassDriver{},
		Standing: encEveryoneStanding{},
		Field:    encounter.FieldInput{Canvas: encounter.CanvasInput{Void: encounter.VoidIsOpaque()}, Rooms: []encounter.RoomInput{{ID: "hall", Width: 8, Height: 8}}},
		Members: []encounter.MemberInput{
			{ID: "alice", Kind: encounter.KindPlayer, Room: "hall", Position: spatial.Position{X: 1, Y: 1}},
			{ID: "bob", Kind: encounter.KindPlayer, Room: "hall", Position: spatial.Position{X: 2, Y: 1}},
		},
		Endings:   []encounter.EndingInput{{Key: "withdrawn", Trigger: encounter.TriggerExternal{}}},
		Retention: encounter.RetentionUnbounded,
	})
	if err != nil {
		t.Fatalf("building the duel: %v", err)
	}
	data := enc.ToData()
	return &data
}

// duel wires the duel world to a manager whose events go nowhere.
func (s *AttackTestSuite) duel(dice session.Roller) *session.Manager {
	mgr, _ := s.breakableDuel(dice)
	return mgr
}

// breakableDuel is the duel with its encounter store wrapped so a test can arm
// the world save to fail.
//
// Unarmed the wrapper delegates, so every duel goes through ONE wiring path
// rather than two that could drift. The failure is armed after StartSession
// because the interesting moment is late: the swing has already made a damaged
// sheet durable, and only the world save is left to fail.
func (s *AttackTestSuite) breakableDuel(dice session.Roller) (*session.Manager, *failingEncounters) {
	s.sessions, s.encounters = newFakeSessions(), newFakeEncounters()
	s.characters = newFakeCharacters(armedFighter("alice"), armedFighter("bob"))
	encounters := &failingEncounters{fakeEncounters: s.encounters}

	mgr, err := session.NewManager(&session.Config{
		Dice: dice, TurnDriver: session.Pass{}, Sessions: s.sessions, Encounters: encounters,
		Characters: s.characters, Events: session.DiscardEvents{},
	})
	s.Require().NoError(err)

	_, err = mgr.StartSession(context.Background(), &session.StartSessionInput{
		Session: "sess", Encounter: "world", World: duelWorld(s.T()),
	})
	s.Require().NoError(err)
	return mgr, encounters
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
		`{"beat":"struck","actor":"alice","targets":["bob"],"roll":15,"total":20,"against":12,"amount":8,`+
			`"critical":false,"attack":{"ref":"dnd5e:weapons:longsword","name":"Longsword","damage_type":"slashing"}}`,
		string(last.Payload))
	s.Equal(session.AttackRef{Ref: "longsword", Name: "Longsword", DamageType: session.DamageSlashing}, out.Attack)
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
		`{"beat":"missed","actor":"alice","targets":["bob"],"roll":2,"total":7,"against":12,`+
			`"attack":{"ref":"dnd5e:weapons:longsword","name":"Longsword","damage_type":"slashing"}}`,
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

// TestASwingNamesTheSheetItWrote is the success half of S6 for the one verb
// that writes a character.
//
// The report is the caller's only account of what this call made durable, and
// a swing makes TWO things durable — the damaged sheet and the world that
// records the blow. A report naming only the world describes a call that half
// happened.
//
// The order is the order the writes really went in: sheets first, then the
// world. Reading the report top to bottom retells the save.
func (s *AttackTestSuite) TestASwingNamesTheSheetItWrote() {
	mgr := s.duel(&sequenceDice{rolls: []int{15, 5}})

	out, err := s.swing(mgr)
	s.Require().NoError(err)
	s.Require().True(out.Hit)

	s.Equal([]string{"character:bob", "encounter:world"}, out.Saved.Written,
		"the damaged sheet is durable and the report has to say so")
	s.Empty(out.Saved.Failed)
	s.False(out.Saved.Partial(), "a whole save is not a partial one")
}

// TestAMissNamesNoCharacterWrite is the negative control that gives the test
// above its meaning.
//
// Nothing is dirtied by a miss, so nothing is written, and the report must be
// derived from the writes that actually happened rather than from the fact
// that a swing occurred. An implementation that named the target
// unconditionally would satisfy the success pin and lie here.
func (s *AttackTestSuite) TestAMissNamesNoCharacterWrite() {
	mgr := s.duel(&sequenceDice{rolls: []int{2, 5}})

	out, err := s.swing(mgr)
	s.Require().NoError(err)
	s.Require().False(out.Hit)

	s.Equal([]string{"encounter:world"}, out.Saved.Written,
		"a miss changed no sheet, so the report names no sheet")
}

// TestAFailedWorldSaveStillNamesTheSheetThatLanded is the reason this report
// exists at all, and the bug it was written for (rpg-toolkit#1056).
//
// The sheet is written BEFORE the world, so a world save that fails leaves the
// damage durable and the blow unrecorded. The report's own documented rule —
// "nothing was written" is safe to retry — then decides what the host does
// next, and it decides it from Written. An empty Written makes Partial() read
// false, the host retries, the retry loads the already-damaged sheet and
// applies the damage again: DOUBLE DAMAGE for one recorded swing, with nothing
// in the stored world to betray it, because the beat lived only on the
// encounter whose save failed.
//
// So the assertion is not cosmetic. Written naming the sheet is what turns this
// into a repair.
func (s *AttackTestSuite) TestAFailedWorldSaveStillNamesTheSheetThatLanded() {
	mgr, encounters := s.breakableDuel(&sequenceDice{rolls: []int{15, 5}})
	before := s.characters.byID["bob"].HitPoints

	// Armed only now, so the session could be started.
	encounters.saveErr = errBroken

	out, err := s.swing(mgr)
	s.Require().Error(err)
	s.Nil(out)
	s.Require().ErrorIs(err, session.ErrSaveFailed)
	s.ErrorIs(err, errBroken, "the store's own failure survives")

	var saved *session.SaveError
	s.Require().ErrorAs(err, &saved, "the report must survive the error")
	s.Equal([]string{"character:bob"}, saved.Report.Written,
		"bob's damaged sheet is already durable — retrying the swing would damage him twice")
	s.Equal([]string{"encounter:world"}, saved.Report.Failed,
		"and the world that would have recorded the blow is what needs repair")
	s.True(saved.Report.Partial(), "half a save is a repair, not a retry")

	s.Equal(before-8, s.characters.byID["bob"].HitPoints,
		"the write the report names really did land")
}

// TestFreeRoamChargesNothing is TestNothingSpendsYet, flipped.
//
// It used to pin a KNOWN GAP: nothing anywhere spent anything, a character could
// swing as many times in a turn as the caller asked, and its failure was to be
// the signal that the economy had landed. The economy has landed
// (rpg-toolkit#1097), and the same three swings in the same scene still land —
// which is now a RULING rather than a gap, and that is why the test is renamed
// instead of deleted.
//
// The ruling is that the action economy is a FIGHT's economy. It is not this
// package's invention: combat.Ledger opens with InCombat and refuses every
// payment from a holder who is not in a fight, and a member on the world clock
// has no turn to spend a turn's slots from. The duel below is free roam — two
// characters standing next to each other, no bubble, no initiative order — so
// the swing is passed no cost at all.
//
// EconomySuite is the other half, and neither test means much alone: the same
// verb refuses a second swing the moment there is a fight to refuse it in. What
// would make BOTH wrong is a swing charged where there is no economy to charge
// it against, which the gate would refuse forever rather than once.
func (s *AttackTestSuite) TestFreeRoamChargesNothing() {
	mgr := s.duel(&sequenceDice{rolls: []int{15, 5, 15, 5, 15, 5}})

	turn, err := mgr.Turn(context.Background(), &session.TurnInput{Session: "sess", Member: "alice"})
	s.Require().NoError(err)
	s.Require().Equal(session.ClockWorld, turn.Clock,
		"the scene is free roam, which is the whole premise of this test")

	for i := 0; i < 3; i++ {
		out, err := s.swing(mgr)
		s.Require().NoError(err, "swing %d", i+1)
		s.True(out.Hit, "swing %d", i+1)
	}

	s.Equal(28-24, 4, "sanity: the fixture starts damaged, so HP is not clamped at max")
	s.Less(s.characters.byID["bob"].HitPoints, 24,
		"three uncharged swings all landed — off the turn clock there is no economy")
	s.Nil(s.characters.byID["alice"].ActionEconomy,
		"and nothing lit one on her sheet: free roam is not a turn")
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
		Dice: testDice{}, TurnDriver: session.Pass{}, Sessions: s.sessions, Encounters: s.encounters,
		Characters: s.characters, Events: session.DiscardEvents{},
	})
	s.Require().NoError(err)

	enc, err := encounter.NewEncounter(&encounter.SetupInput{Sight: encEveryoneSees{},
		Initiative: encOrderAsGiven{}, TurnDriver: encPassDriver{},
		Standing: encEveryoneStanding{},
		Field:    encounter.FieldInput{Canvas: encounter.CanvasInput{Void: encounter.VoidIsOpaque()}, Rooms: []encounter.RoomInput{{ID: "hall", Width: 8, Height: 8}}},
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

// TestAnEmptyHandThrowsAnUnarmedStrike pins rpg-toolkit#1168 at the seam: an
// empty main hand swings and lands with the unarmed strike's own numbers
// rather than being refused. Exact arithmetic, the same discipline
// TestASwingLandsAndTheStoryRecordsIt holds attack.go to: a d20 of 15 plus a
// proficient STR 16 (+3) plus the proficiency bonus every 5e character has
// for an unarmed strike regardless of weapon training (+2) totals 20 against
// bob's effective AC 12 — a hit — and the damage die is 1d1, so the roll
// script's damage entry (any value) resolves to 1 plus the +3 modifier: 4
// bludgeoning.
func (s *AttackTestSuite) TestAnEmptyHandThrowsAnUnarmedStrike() {
	s.sessions, s.encounters = newFakeSessions(), newFakeEncounters()
	s.characters = newFakeCharacters(unarmedFighter("alice"), armedFighter("bob"))
	mgr, err := session.NewManager(&session.Config{
		Dice: &sequenceDice{rolls: []int{15, 1}}, TurnDriver: session.Pass{},
		Sessions: s.sessions, Encounters: s.encounters,
		Characters: s.characters, Events: session.DiscardEvents{},
	})
	s.Require().NoError(err)

	enc, err := encounter.NewEncounter(&encounter.SetupInput{Sight: encEveryoneSees{},
		Initiative: encOrderAsGiven{}, TurnDriver: encPassDriver{},
		Standing: encEveryoneStanding{},
		Field:    encounter.FieldInput{Canvas: encounter.CanvasInput{Void: encounter.VoidIsOpaque()}, Rooms: []encounter.RoomInput{{ID: "hall", Width: 8, Height: 8}}},
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

	out, err := s.swing(mgr)
	s.Require().NoError(err)
	s.Equal(15, out.Roll)
	s.Equal(20, out.Total, "15 + 3 STR + 2 proficiency (unarmed is always proficient)")
	s.Equal(12, out.Against)
	s.True(out.Hit)
	s.Equal(4, out.Damage, "1d1 + 3 STR")
}

// reachWorld is duelWorld's shape with the distance between alice and bob
// under the caller's control, so a test can put the target just inside or
// just outside a weapon's reach without touching sight range at all —
// encEveryoneSees keeps the two in contact (and so in one fight) regardless
// of how far apart they stand, which is what lets this fixture isolate
// reach from perception.
func reachWorld(t fataler, bobAt spatial.Position) *encounter.EncounterData {
	enc, err := encounter.NewEncounter(&encounter.SetupInput{Sight: encEveryoneSees{},
		Initiative: encOrderAsGiven{}, TurnDriver: encPassDriver{},
		Standing: encEveryoneStanding{},
		Field: encounter.FieldInput{Canvas: encounter.CanvasInput{Void: encounter.VoidIsOpaque()},
			Rooms: []encounter.RoomInput{{ID: "hall", Width: 20, Height: 20}}},
		Members: []encounter.MemberInput{
			{ID: "alice", Kind: encounter.KindPlayer, Room: "hall", Position: spatial.Position{X: 1, Y: 1}},
			{ID: "bob", Kind: encounter.KindPlayer, Room: "hall", Position: bobAt},
		},
		Endings:   []encounter.EndingInput{{Key: "withdrawn", Trigger: encounter.TriggerExternal{}}},
		Retention: encounter.RetentionUnbounded,
	})
	if err != nil {
		t.Fatalf("building the reach world: %v", err)
	}
	data := enc.ToData()
	return &data
}

// TestOutOfReachIsRefused pins rpg-toolkit#1010 at the seam: a target beyond
// the weapon's reach is refused before anything is priced or resolved, even
// though the two are in the same fight (encEveryoneSees keeps them in
// contact regardless of distance — reach is a different question from
// sight).
func (s *AttackTestSuite) TestOutOfReachIsRefused() {
	s.sessions, s.encounters = newFakeSessions(), newFakeEncounters()
	s.characters = newFakeCharacters(armedFighter("alice"), armedFighter("bob"))
	mgr, err := session.NewManager(&session.Config{
		Dice: testDice{}, TurnDriver: session.Pass{}, Sessions: s.sessions, Encounters: s.encounters,
		Characters: s.characters, Events: session.DiscardEvents{},
	})
	s.Require().NoError(err)

	// A longsword reaches 1 cell; bob stands 4 away.
	_, err = mgr.StartSession(context.Background(), &session.StartSessionInput{
		Session: "sess", Encounter: "world", World: reachWorld(s.T(), spatial.Position{X: 5, Y: 1}),
	})
	s.Require().NoError(err)

	_, err = s.swing(mgr)
	s.ErrorIs(err, session.ErrOutOfReach)
	s.Contains(err.Error(), "bob")
}

// TestReachPropertyExtendsToTwoCells pins the other half: a weapon carrying
// the Reach property swings at 2 cells, where a plain longsword could not.
func (s *AttackTestSuite) TestReachPropertyExtendsToTwoCells() {
	s.sessions, s.encounters = newFakeSessions(), newFakeEncounters()
	glaiveAlice := armedFighter("alice")
	glaiveAlice.Inventory = []character.InventoryItemData{
		{Type: shared.EquipmentTypeWeapon, ID: string(weapons.Glaive), Quantity: 1},
	}
	glaiveAlice.EquipmentSlots = character.EquipmentSlots{character.SlotMainHand: string(weapons.Glaive)}
	s.characters = newFakeCharacters(glaiveAlice, armedFighter("bob"))
	mgr, err := session.NewManager(&session.Config{
		Dice: &sequenceDice{rolls: []int{2, 1}}, TurnDriver: session.Pass{},
		Sessions: s.sessions, Encounters: s.encounters,
		Characters: s.characters, Events: session.DiscardEvents{},
	})
	s.Require().NoError(err)

	_, err = mgr.StartSession(context.Background(), &session.StartSessionInput{
		Session: "sess", Encounter: "world", World: reachWorld(s.T(), spatial.Position{X: 3, Y: 1}),
	})
	s.Require().NoError(err)

	_, err = s.swing(mgr)
	s.Require().NoError(err, "a glaive reaches 2 cells; bob stands 2 away")
}

// TestNotYourTurnIsRefused pins the third refusal Afford is supposed to
// announce ahead of time: only the fight's active member swings.
//
// TWO REAL PLAYERS is what makes "not alice's turn" observable at all — two
// players standing near each other never form a fight on their own (contact
// is a hostile-sides question), so this borrows move_turnclock_test.go's own
// fixture shape: alice and bob start in the hall, a skeleton spawns adjacent
// and pulls all three into one bubble, initiative order [alice, bob,
// skel-1]. Bob — seated but not active — is refused for trying to act out of
// turn, distinctly from being out of reach or unable to pay.
func (s *AttackTestSuite) TestNotYourTurnIsRefused() {
	s.sessions, s.encounters = newFakeSessions(), newFakeEncounters()
	s.characters = newFakeCharacters(armedFighter("alice"), armedFighter("bob"))
	mgr, err := session.NewManager(&session.Config{
		Dice: testDice{}, TurnDriver: session.Pass{}, Sessions: s.sessions, Encounters: s.encounters,
		Characters: s.characters, Events: session.DiscardEvents{},
	})
	s.Require().NoError(err)

	enc, err := encounter.NewEncounter(&encounter.SetupInput{Sight: encEveryoneSees{},
		Initiative: encOrderAsGiven{}, TurnDriver: encPassDriver{},
		Standing: encEveryoneStanding{},
		Field: encounter.FieldInput{Canvas: encounter.CanvasInput{Void: encounter.VoidIsOpaque()},
			Rooms: []encounter.RoomInput{{ID: "hall", Width: 8, Height: 8}}},
		Members: []encounter.MemberInput{
			{ID: "alice", Kind: encounter.KindPlayer, Room: "hall", Position: spatial.Position{X: 1, Y: 1}},
			{ID: "bob", Kind: encounter.KindPlayer, Room: "hall", Position: spatial.Position{X: 5, Y: 5}},
		},
		Endings:   []encounter.EndingInput{{Key: "withdrawn", Trigger: encounter.TriggerExternal{}}},
		Retention: encounter.RetentionUnbounded,
	})
	s.Require().NoError(err)
	data := enc.ToData()

	ctx := context.Background()
	_, err = mgr.StartSession(ctx, &session.StartSessionInput{Session: "sess", Encounter: "world", World: &data})
	s.Require().NoError(err)

	spawned, err := mgr.Spawn(ctx, &session.SpawnInput{
		Session: "sess", ID: "skel-1", Ref: refs.Monsters.Skeleton().String(),
		Position: spatial.Position{X: 2, Y: 1},
	})
	s.Require().NoError(err)
	s.Require().NotNil(spawned.Formed, "arriving in plain sight must start a fight")

	turn, err := mgr.Turn(ctx, &session.TurnInput{Session: "sess", Member: "alice"})
	s.Require().NoError(err)
	s.Require().Equal("alice", turn.Active, "alice is first registered — first in initiative")

	// bob swinging at the skeleton while it is alice's turn, not his.
	before := s.characters.asked["bob"]
	_, err = mgr.Attack(ctx, &session.AttackInput{
		Session: "sess", Attacker: "bob", Target: "skel-1",
	})
	s.ErrorIs(err, session.ErrNotYourTurn)
	s.Contains(err.Error(), "bob")

	// Copilot's finding on PR #1174: the turn gate must be asked before
	// compileAttack ever loads bob's sheet — the same precedence Move's own
	// gate keeps (Copilot on #1171) and Afford's DOWNED-vs-NOT_YOUR_TURN
	// ordering keeps (TestNotActiveWinsOverAffordability's own claim,
	// asked here of Attack instead of Move).
	s.Equal(before, s.characters.asked["bob"],
		"the clock is asked before the sheet is — a refusal this early must never have loaded it")
}

// TestASheetlessTargetIsRefusedByName covers content standing in a world that
// nobody spawned.
//
// An authored monster has no stored sheet until Spawn records one, so there is
// nothing to read an armour class off and nothing for damage to land on. The
// strike would fail on its own, but further from the cause and in the
// resolution module's vocabulary — this refuses earlier, names who, and keeps
// that module's sentinels off the seam.
func (s *AttackTestSuite) TestASheetlessTargetIsRefusedByName() {
	s.sessions, s.encounters = newFakeSessions(), newFakeEncounters()
	s.characters = newFakeCharacters(armedFighter("alice"))
	mgr, err := session.NewManager(&session.Config{
		Dice: testDice{}, TurnDriver: session.Pass{}, Sessions: s.sessions, Encounters: s.encounters,
		Characters: s.characters, Events: session.DiscardEvents{},
	})
	s.Require().NoError(err)

	enc, err := encounter.NewEncounter(&encounter.SetupInput{Sight: encEveryoneSees{},
		Initiative: encOrderAsGiven{}, TurnDriver: encPassDriver{},
		Standing: encEveryoneStanding{},
		Field:    encounter.FieldInput{Canvas: encounter.CanvasInput{Void: encounter.VoidIsOpaque()}, Rooms: []encounter.RoomInput{{ID: "hall", Width: 8, Height: 8}}},
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
		Session: "sess", Attacker: "alice", Target: "ogre",
	})
	s.ErrorIs(err, session.ErrNoSheet)
}

// duelAmong wires a manager whose world holds the named players and whose
// character repository holds exactly the sheets given.
//
// The two lists are independent on purpose. Every test below turns on a member
// the roster HAS and the repository does NOT — a state only a host can produce,
// and the one the character sentinels exist to describe.
func (s *AttackTestSuite) duelAmong(members []string, sheets ...*character.Data) *session.Manager {
	s.sessions, s.encounters = newFakeSessions(), newFakeEncounters()
	s.characters = newFakeCharacters(sheets...)

	mgr, err := session.NewManager(&session.Config{
		Dice: testDice{}, TurnDriver: session.Pass{}, Sessions: s.sessions, Encounters: s.encounters,
		Characters: s.characters, Events: session.DiscardEvents{},
	})
	s.Require().NoError(err)

	placed := make([]encounter.MemberInput, 0, len(members))
	for i, id := range members {
		placed = append(placed, encounter.MemberInput{
			ID: encounter.MemberID(id), Kind: encounter.KindPlayer, Room: "hall",
			Position: spatial.Position{X: float64(i + 1), Y: 1},
		})
	}
	enc, err := encounter.NewEncounter(&encounter.SetupInput{Sight: encEveryoneSees{},
		Initiative: encOrderAsGiven{}, TurnDriver: encPassDriver{},
		Standing:  encEveryoneStanding{},
		Field:     encounter.FieldInput{Canvas: encounter.CanvasInput{Void: encounter.VoidIsOpaque()}, Rooms: []encounter.RoomInput{{ID: "hall", Width: 8, Height: 8}}},
		Members:   placed,
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

// unreadableFighter is a stored sheet that EXISTS and cannot be reconstituted.
//
// The malformed condition blob is what makes it unreadable, and it bites only
// on the path under test: character.Load — what compileAttack uses — is STRICT
// and fails the whole load, while the lenient character.LoadFromData that Join
// uses drops the blob and carries on (pinned by
// TestACorruptConditionIsDroppedRatherThanRejected). So this fixture is corrupt
// exactly where the sentinel under test is chosen.
func unreadableFighter(id string) *character.Data {
	sheet := armedFighter(id)
	sheet.Conditions = []json.RawMessage{json.RawMessage(`{"ref":"nonsense","x":`)}
	return sheet
}

// The three tests below pin one contract: ABSENT and CORRUPT are different
// answers, and Attack must give the same ones every other verb gives.
//
// errors.go defines ErrNoCharacter as "the repository does not hold it" and
// ErrBadCharacter as "stored data exists but cannot be reconstituted", and
// loadCharacter has always honoured that. Attack did not: it carried its own
// copies of the fetch, and they mapped the pair backwards (rpg-toolkit#1057).
// A host branching on these does opposite repairs — re-check the ID versus go
// inspect storage — so the two must never be swapped. Each test asserts the
// sentinel it wants AND denies the other, because an implementation returning
// both would satisfy a one-sided assertion while telling the host nothing.

func (s *AttackTestSuite) TestAnAbsentAttackerSheetIsAbsentRatherThanCorrupt() {
	mgr := s.duelAmong([]string{"alice", "bob"}, armedFighter("bob"))

	_, err := s.swing(mgr)
	s.Require().Error(err)
	s.ErrorIs(err, session.ErrNoCharacter, "the repository does not hold alice at all")
	s.NotErrorIs(err, session.ErrBadCharacter, "absent is not corrupt")
}

func (s *AttackTestSuite) TestAnUnreadableAttackerSheetIsCorruptRatherThanAbsent() {
	mgr := s.duelAmong([]string{"alice", "bob"}, unreadableFighter("alice"), armedFighter("bob"))

	_, err := s.swing(mgr)
	s.Require().Error(err)
	s.ErrorIs(err, session.ErrBadCharacter, "alice's bytes are there and cannot be read")
	s.NotErrorIs(err, session.ErrNoCharacter, "corrupt is not absent")
}

// TestAnAbsentBystanderSheetIsAbsentRatherThanCorrupt covers the castFor path:
// a member who is neither swinging nor being swung at still joins the cast,
// because applicability is an effect's own predicate (ADR-0038). Their sheet
// being absent is the same failure as the attacker's and must read the same way.
func (s *AttackTestSuite) TestAnAbsentBystanderSheetIsAbsentRatherThanCorrupt() {
	mgr := s.duelAmong([]string{"alice", "bob", "carol"}, armedFighter("alice"), armedFighter("bob"))

	_, err := s.swing(mgr)
	s.Require().Error(err)
	s.ErrorIs(err, session.ErrNoCharacter, "carol is in the roster and not in the repository")
	s.NotErrorIs(err, session.ErrBadCharacter, "absent is not corrupt")

	// The role noun earns its keep here more than anywhere else: the host asked
	// alice to swing at bob and gets back a complaint about carol, who it never
	// named. "participant" is the word that explains why she was read at all.
	s.Contains(err.Error(), `participant "carol"`, "say which part the missing member was playing")
}
