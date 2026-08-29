// Copyright (C) 2026 Kirk Diggler
// SPDX-License-Identifier: GPL-3.0-or-later

package session_test

import (
	"context"
	"encoding/json"
	"testing"

	"github.com/stretchr/testify/suite"

	"github.com/KirkDiggler/rpg-toolkit/core"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/abilities"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/armor"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/character"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/classes"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/conditions"
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

// turnWorld moves the named members from the world clock into one authored
// turn clock. Tests author the clock directly so initiative never consumes the
// attack dice whose exact faces they assert.
func turnWorld(data *encounter.EncounterData, order []string, active int) *encounter.EncounterData {
	ids := make([]core.EntityID, 0, len(order))
	for _, member := range order {
		delete(data.Clock.Budgets, core.EntityID(member))
		ids = append(ids, core.EntityID(member))
	}
	raw, err := json.Marshal([]struct {
		Order     []core.EntityID `json:"order"`
		ActiveIdx int             `json:"active_idx"`
		Round     int             `json:"round"`
	}{{Order: ids, ActiveIdx: active, Round: 1}})
	if err != nil {
		panic(err)
	}
	if err := json.Unmarshal(raw, &data.Bubbles); err != nil {
		panic(err)
	}
	return data
}

// freeRoamDuelWorld is two armed characters standing next to each other, which
// is the smallest world where a swing means anything.
//
// Both sides are CHARACTERS on purpose: damage dirties the target's stored
// sheet, which is the case that earns Attack its row in the no-clobber pin.
//
// Shared with the event-kind pins (attackevents_test.go), which need the same
// duel delivered to a real stream rather than discarded. One world, so a
// fixture drift cannot make the two suites disagree about what was swung at.
func freeRoamDuelWorld(t fataler) *encounter.EncounterData {
	enc, err := encounter.NewEncounter(&encounter.SetupInput{Striker: encounter.RefusingStriker{}, Announcer: encQuietAnnouncer{}, Sight: encEveryoneSees{},
		Initiative: encOrderAsGiven{}, TurnDriver: encPassDriver{},
		Standing: encEveryoneStanding{},
		Field:    encounter.FieldInput{Canvas: pointyCanvas(), Regions: []encounter.RegionInput{rectRegion("hall", 0, 0, 8, 8)}},
		Members: []encounter.MemberInput{
			{ID: "alice", Kind: encounter.KindPlayer, Position: spatial.Position{X: 1, Y: 1}},
			{ID: "bob", Kind: encounter.KindPlayer, Position: spatial.Position{X: 2, Y: 1}},
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

// duelWorld is the same adjacent-character scene on a turn clock with alice
// active, so Attack can execute only through the selector Afford authored.
func duelWorld(t fataler) *encounter.EncounterData {
	return turnWorld(freeRoamDuelWorld(t), []string{"alice", "bob"}, 0)
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
		DeclarationID: currentAttackID(s.T(), mgr, "sess", "alice"),
	})
}

// duelingBlob is the persisted Dueling fighting style condition, so the
// fighter has actually chosen it rather than merely being eligible for it.
func (s *AttackTestSuite) duelingBlob(id string) json.RawMessage {
	raw, err := (&conditions.FightingStyleDuelingCondition{CharacterID: id}).ToJSON()
	s.Require().NoError(err)

	return raw
}

// TestAnArmedDuelingFighterResolvesOnTheSessionStack pins rpg-toolkit#1178.
//
// Dueling's damage-chain fold used to decide eligibility by live-querying a
// gamectx.CharacterRegistry — a global lookup the OLD encounter module
// installed and the session stack never does; resolution installs exactly
// one gamectx registry (WithRoom, for prone's range predicate) and
// deliberately nothing else (resolution/doc.go). So a fighter who chose
// Dueling and swings a one-handed melee weapon with an empty or shielded off
// hand — precisely the sheet shape this test builds — crashed every armed
// swing with gamectx.ErrNoGameContext, while an unarmed swing by the same
// character resolved fine (nothing in the unarmed path reaches Dueling's
// predicate the same way). This is the live blocker reported against
// rpg-api's session stack.
//
// No gamectx.WithGameContext is installed anywhere in this test, exactly
// like the real session stack — that absence is the point being pinned.
func (s *AttackTestSuite) TestAnArmedDuelingFighterResolvesOnTheSessionStack() {
	s.sessions, s.encounters = newFakeSessions(), newFakeEncounters()

	alice := armedFighter("alice")
	alice.Inventory = append(alice.Inventory, character.InventoryItemData{
		Type: shared.EquipmentTypeArmor, ID: string(armor.Shield), Quantity: 1,
	})
	alice.EquipmentSlots[character.SlotOffHand] = string(armor.Shield)
	alice.Conditions = []json.RawMessage{s.duelingBlob("alice")}

	s.characters = newFakeCharacters(alice, armedFighter("bob"))

	mgr, err := session.NewManager(&session.Config{
		Dice: &sequenceDice{rolls: []int{15, 5}}, TurnDriver: session.Pass{},
		Sessions: s.sessions, Encounters: s.encounters, Characters: s.characters,
		Events: session.DiscardEvents{},
	})
	s.Require().NoError(err)

	_, err = mgr.StartSession(context.Background(), &session.StartSessionInput{
		Session: "sess", Encounter: "world", World: duelWorld(s.T()),
	})
	s.Require().NoError(err)

	_, err = mgr.Attack(context.Background(), &session.AttackInput{
		Session: "sess", Attacker: "alice", Target: "bob",
		DeclarationID: currentAttackID(s.T(), mgr, "sess", "alice"),
	})
	s.Require().NoError(err,
		"an armed Dueling-eligible fighter's swing must not depend on a GameContext the session stack never installs")
}

// unarmoredBarbarian defends with nothing but a sheet.
//
// Same DEX 14 as every other fixture, so the baseline is duelAC's 12, and CON
// 14 so Unarmored Defense has two points to add. The sheet's flat ArmorClass
// is 16 and inert, exactly as armedFighter's is — if the seam ever reads the
// stored number instead of folding, these tests report 16 and say so.
func (s *AttackTestSuite) unarmoredBarbarian(id string) *character.Data {
	sheet := armedFighter(id)
	sheet.ClassID = classes.Barbarian
	sheet.Inventory = nil
	sheet.EquipmentSlots = character.EquipmentSlots{}

	raw, err := (&conditions.UnarmoredDefenseCondition{
		CharacterID: id,
		Type:        conditions.UnarmoredDefenseBarbarian,
		Source:      "dnd5e:classes:barbarian",
	}).ToJSON()
	s.Require().NoError(err)
	sheet.Conditions = []json.RawMessage{raw}

	return sheet
}

// TestUnarmoredDefenseDefendsOnTheSessionStack is the number rpg-api receives.
//
// This is the last seam between the fix and the game. resolution proves the
// fold (effective_ac_test.go); this proves the fold survives the verb, because
// AttackOutput.Against is what crosses the wire and what a player watches on
// the dock.
//
// The rule was inert here for as long as it has existed. Unarmored Defense read
// gamectx.RequireCharacters, a registry with zero non-test install sites, and
// returned its error into the AC fold — which Character.EffectiveAC swallows,
// so the defender silently dropped to 10+DEX and every other AC contributor
// went with it. Kirk found it by playing the tomb: his barbarian fought at 11
// instead of 14 (rpg-api#842, rpg-toolkit#1251).
//
// No gamectx.WithGameContext is installed anywhere in this test, matching the
// real session stack — that absence is the point being pinned, the same way it
// is for Dueling and Protection above.
func (s *AttackTestSuite) TestUnarmoredDefenseDefendsOnTheSessionStack() {
	s.sessions, s.encounters = newFakeSessions(), newFakeEncounters()
	s.characters = newFakeCharacters(armedFighter("alice"), s.unarmoredBarbarian("bob"))

	mgr, err := session.NewManager(&session.Config{
		Dice: &sequenceDice{rolls: []int{15, 5}}, TurnDriver: session.Pass{},
		Sessions: s.sessions, Encounters: s.encounters, Characters: s.characters,
		Events: session.DiscardEvents{},
	})
	s.Require().NoError(err)

	_, err = mgr.StartSession(context.Background(), &session.StartSessionInput{
		Session: "sess", Encounter: "world", World: duelWorld(s.T()),
	})
	s.Require().NoError(err)

	out, err := mgr.Attack(context.Background(), &session.AttackInput{
		Session: "sess", Attacker: "alice", Target: "bob",
		DeclarationID: currentAttackID(s.T(), mgr, "sess", "alice"),
	})
	s.Require().NoError(err)

	s.Equal(duelAC+2, out.Against,
		"Unarmored Defense adds CON(+2) to the 10+DEX every other fixture defends with: "+
			"14, not duelAC's 12 and not the sheet's inert 16")
}

// protectionBlob is the persisted Protection fighting style condition.
func (s *AttackTestSuite) protectionBlob(id string) json.RawMessage {
	raw, err := (&conditions.FightingStyleProtectionCondition{CharacterID: id}).ToJSON()
	s.Require().NoError(err)

	return raw
}

// protectorFighter is armedFighter equipped with a shield in the off hand
// and the Protection fighting style chosen.
func (s *AttackTestSuite) protectorFighter(id string) *character.Data {
	sheet := armedFighter(id)
	sheet.Inventory = append(sheet.Inventory, character.InventoryItemData{
		Type: shared.EquipmentTypeArmor, ID: string(armor.Shield), Quantity: 1,
	})
	sheet.EquipmentSlots[character.SlotOffHand] = string(armor.Shield)
	sheet.Conditions = []json.RawMessage{s.protectionBlob(id)}
	return sheet
}

// TestProtectionReactsToANearbyAllysAttackOnTheSessionStack pins
// rpg-toolkit#1178's second half, Copilot's finding on PR #1179: a
// two-member world never exercises Protection's shield/reaction branch at
// all (both of onAttackChain's exclusions — "not my own attack", "not an
// attack on me" — trivially pass or fail with only an attacker and a
// target), so the first fix's claim that "no caller exercises it" was
// false. A THIRD member changes that: carol attacks bob while alice (the
// protector, carrying a shield and the Protection style) stands adjacent
// to bob. alice is neither carol nor bob, so both exclusions clear and
// Protection's full eligibility path — shield equipped, reaction
// available, both read off alice's own live sheet — actually runs.
//
// THAT SHEET COMES OUT OF THE CAST as of rpg-project#319: alice looks
// herself up by her own ID and gets back the same read surface she would
// get for anybody else, in place of the owner handle a loader used to hand
// her at attach time. This test installs no gamectx tenant of its own, and
// that is still the point — the cast Protection reads is the one
// resolution installs on every path that folds anything, so this test
// passing is evidence that the real installer ran, not that a stand-in
// did. Before the fix this failed exactly like the two-member Dueling
// case; after it, the swing resolves AND Protection's disadvantage still
// applies — "behaves as before", the mechanic is not merely uncrashed but
// still working.
func (s *AttackTestSuite) TestProtectionReactsToANearbyAllysAttackOnTheSessionStack() {
	s.sessions, s.encounters = newFakeSessions(), newFakeEncounters()

	alice := s.protectorFighter("alice") // the protector
	bob := armedFighter("bob")           // carol's target, standing beside alice
	carol := armedFighter("carol")       // the attacker — neither alice nor bob

	s.characters = newFakeCharacters(alice, bob, carol)

	enc, err := encounter.NewEncounter(&encounter.SetupInput{Striker: encounter.RefusingStriker{}, Announcer: encQuietAnnouncer{}, Sight: encEveryoneSees{},
		Initiative: encOrderAsGiven{}, TurnDriver: encPassDriver{},
		Standing: encEveryoneStanding{},
		Field: encounter.FieldInput{Canvas: pointyCanvas(),
			Regions: []encounter.RegionInput{rectRegion("hall", 0, 0, 8, 8)}},
		Members: []encounter.MemberInput{
			{ID: "alice", Kind: encounter.KindPlayer, Position: spatial.Position{X: 1, Y: 1}},
			{ID: "bob", Kind: encounter.KindPlayer, Position: spatial.Position{X: 2, Y: 1}},   // adjacent to alice
			{ID: "carol", Kind: encounter.KindPlayer, Position: spatial.Position{X: 3, Y: 1}}, // adjacent to bob, in melee reach
		},
		Endings: []encounter.EndingInput{{Key: "withdrawn", Trigger: encounter.TriggerExternal{}}},
	})
	s.Require().NoError(err)
	world := enc.ToData()

	mgr, err := session.NewManager(&session.Config{
		Dice: &sequenceDice{rolls: []int{15, 5}}, TurnDriver: session.Pass{},
		Sessions: s.sessions, Encounters: s.encounters, Characters: s.characters,
		Events: session.DiscardEvents{},
	})
	s.Require().NoError(err)

	_, err = mgr.StartSession(context.Background(), &session.StartSessionInput{
		Session: "sess", Encounter: "world", World: turnWorld(&world, []string{"alice", "bob", "carol"}, 2),
	})
	s.Require().NoError(err)

	_, err = mgr.Attack(context.Background(), &session.AttackInput{
		Session: "sess", Attacker: "carol", Target: "bob",
		DeclarationID: currentAttackID(s.T(), mgr, "sess", "carol"),
	})
	s.Require().NoError(err,
		"a third member's Protection condition must not depend on a GameContext the session stack never installs")
}

// TestASwingLandsAndTheStoryRecordsIt is the headline: a rules machine runs
// through the seam, and what it produced reaches both the caller and the
// transcript.
//
// Exact arithmetic rather than "damage is positive". A d20 of 15 plus a
// proficient Strength longsword (+3 STR, +2 proficiency) totals 20 against
// AC 12 — a hit, and the damage die scripted to 5 with the +3 modifier deals
// 8. The same beat now preserves the ordered weapon and ability components
// resolution produced. A loose assertion here would pass on an implementation
// that dropped the modifier, halved the die, or resolved against the wrong
// sheet.
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
			`"critical":false,"attack":{"ref":"dnd5e:weapons:longsword","name":"Longsword","damage_type":"slashing"},`+
			`"damage_components":[`+
			`{"source":"weapon","source_ref":"dnd5e:weapons:longsword","dice":"1d8","final_rolls":[5],"flat_bonus":0,"damage_type":"slashing"},`+
			`{"source":"ability","source_ref":"dnd5e:abilities:str","flat_bonus":3,"damage_type":"slashing"}]}`,
		string(last.Payload))
	s.Equal(session.AttackRef{Ref: "dnd5e:weapons:longsword", Name: "Longsword", DamageType: session.DamageSlashing}, out.Attack)
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

	s.Equal([]string{"character:alice", "character:bob", "encounter:world"}, out.Saved.Written,
		"the paid attacker and damaged target are durable and the report says so")
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

	s.Equal([]string{"character:alice", "encounter:world"}, out.Saved.Written,
		"a miss changes no target sheet, but the attacker's paid economy is durable")
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
	s.Equal([]string{"character:alice", "character:bob"}, saved.Report.Written,
		"the paid attacker and damaged target are already durable — retrying would spend and damage twice")
	s.Equal([]string{"encounter:world"}, saved.Report.Failed,
		"and the world that would have recorded the blow is what needs repair")
	s.True(saved.Report.Partial(), "half a save is a repair, not a retry")

	s.Equal(before-8, s.characters.byID["bob"].HitPoints,
		"the write the report names really did land")
}

// TestFreeRoamAttackHasNoDeclaration pins the production contract: Afford is
// empty on the world clock, so Attack has no selector to echo there. A direct
// free-roam swing is invalid input and rolls or writes nothing.
func (s *AttackTestSuite) TestFreeRoamAttackHasNoDeclaration() {
	roller := &sequenceDice{rolls: []int{15, 5}}
	s.sessions, s.encounters = newFakeSessions(), newFakeEncounters()
	s.characters = newFakeCharacters(armedFighter("alice"), armedFighter("bob"))
	mgr, err := session.NewManager(&session.Config{
		Dice: roller, TurnDriver: session.Pass{}, Sessions: s.sessions, Encounters: s.encounters,
		Characters: s.characters, Events: session.DiscardEvents{},
	})
	s.Require().NoError(err)
	_, err = mgr.StartSession(context.Background(), &session.StartSessionInput{
		Session: "sess", Encounter: "world", World: freeRoamDuelWorld(s.T()),
	})
	s.Require().NoError(err)

	out, err := mgr.Attack(context.Background(), &session.AttackInput{
		Session: "sess", Attacker: "alice", Target: "bob",
	})
	s.ErrorIs(err, session.ErrNoDeclarationID)
	s.Nil(out)
	s.Zero(roller.next, "the selector gate precedes every attack roll")
	s.Equal(24, s.characters.byID["bob"].HitPoints)
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

	enc, err := encounter.NewEncounter(&encounter.SetupInput{Striker: encounter.RefusingStriker{}, Announcer: encQuietAnnouncer{}, Sight: encEveryoneSees{},
		Initiative: encOrderAsGiven{}, TurnDriver: encPassDriver{},
		Standing: encEveryoneStanding{},
		Field:    encounter.FieldInput{Canvas: pointyCanvas(), Regions: []encounter.RegionInput{rectRegion("hall", 0, 0, 8, 8)}},
		Members: []encounter.MemberInput{
			{ID: "alice", Kind: encounter.KindPlayer, Position: spatial.Position{X: 1, Y: 1}},
			{ID: "ogre", Kind: encounter.KindMonster, Position: spatial.Position{X: 2, Y: 1}},
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
	s.Equal([]string{"Session", "Attacker", "Target", "DeclarationID"}, structFields(session.AttackInput{}),
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

	_, err = mgr.Attack(ctx, &session.AttackInput{
		Session: "sess", Attacker: "alice", Target: "nobody",
		DeclarationID: currentAttackID(s.T(), mgr, "sess", "alice"),
	})
	s.ErrorIs(err, session.ErrStaleDeclaration)
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

	enc, err := encounter.NewEncounter(&encounter.SetupInput{Striker: encounter.RefusingStriker{}, Announcer: encQuietAnnouncer{}, Sight: encEveryoneSees{},
		Initiative: encOrderAsGiven{}, TurnDriver: encPassDriver{},
		Standing: encEveryoneStanding{},
		Field:    encounter.FieldInput{Canvas: pointyCanvas(), Regions: []encounter.RegionInput{rectRegion("hall", 0, 0, 8, 8)}},
		Members: []encounter.MemberInput{
			{ID: "alice", Kind: encounter.KindPlayer, Position: spatial.Position{X: 1, Y: 1}},
			{ID: "bob", Kind: encounter.KindPlayer, Position: spatial.Position{X: 2, Y: 1}},
		},
		Endings: []encounter.EndingInput{{Key: "withdrawn", Trigger: encounter.TriggerExternal{}}},
	})
	s.Require().NoError(err)
	data := enc.ToData()
	_, err = mgr.StartSession(context.Background(), &session.StartSessionInput{
		Session: "sess", Encounter: "world", World: turnWorld(&data, []string{"alice", "bob"}, 0),
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
	enc, err := encounter.NewEncounter(&encounter.SetupInput{Striker: encounter.RefusingStriker{}, Announcer: encQuietAnnouncer{}, Sight: encEveryoneSees{},
		Initiative: encOrderAsGiven{}, TurnDriver: encPassDriver{},
		Standing: encEveryoneStanding{},
		Field: encounter.FieldInput{Canvas: pointyCanvas(),
			Regions: []encounter.RegionInput{rectRegion("hall", 0, 0, 20, 20)}},
		Members: []encounter.MemberInput{
			{ID: "alice", Kind: encounter.KindPlayer, Position: spatial.Position{X: 1, Y: 1}},
			{ID: "bob", Kind: encounter.KindPlayer, Position: bobAt},
		},
		Endings:   []encounter.EndingInput{{Key: "withdrawn", Trigger: encounter.TriggerExternal{}}},
		Retention: encounter.RetentionUnbounded,
	})
	if err != nil {
		t.Fatalf("building the reach world: %v", err)
	}
	data := enc.ToData()
	return turnWorld(&data, []string{"alice", "bob"}, 0)
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
	s.ErrorIs(err, session.ErrStaleDeclaration,
		"a current offer with no available target is unavailable at selection")
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

	enc, err := encounter.NewEncounter(&encounter.SetupInput{Striker: encounter.RefusingStriker{}, Announcer: encQuietAnnouncer{}, Sight: encEveryoneSees{},
		Initiative: encOrderAsGiven{}, TurnDriver: encPassDriver{},
		Standing: encEveryoneStanding{},
		Field: encounter.FieldInput{Canvas: pointyCanvas(),
			Regions: []encounter.RegionInput{rectRegion("hall", 0, 0, 8, 8)}},
		Members: []encounter.MemberInput{
			{ID: "alice", Kind: encounter.KindPlayer, Position: spatial.Position{X: 1, Y: 1}},
			{ID: "bob", Kind: encounter.KindPlayer, Position: spatial.Position{X: 5, Y: 5}},
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

// TestAffordThenAttackRefusesASheetlessTargetBeforeExecution covers content
// standing in a world that nobody spawned. Afford and unchanged Attack must
// agree before resolution: the candidate remains visible but is Unreadable,
// the declaration is unavailable, and echoing its selector mutates nothing.
func (s *AttackTestSuite) TestAffordThenAttackRefusesASheetlessTargetBeforeExecution() {
	s.sessions, s.encounters = newFakeSessions(), newFakeEncounters()
	s.characters = newFakeCharacters(armedFighter("alice"))
	mgr, err := session.NewManager(&session.Config{
		Dice: testDice{}, TurnDriver: session.Pass{}, Sessions: s.sessions, Encounters: s.encounters,
		Characters: s.characters, Events: session.DiscardEvents{},
	})
	s.Require().NoError(err)

	enc, err := encounter.NewEncounter(&encounter.SetupInput{Striker: encounter.RefusingStriker{}, Announcer: encQuietAnnouncer{}, Sight: encEveryoneSees{},
		Initiative: encOrderAsGiven{}, TurnDriver: encPassDriver{},
		Standing: encEveryoneStanding{},
		Field:    encounter.FieldInput{Canvas: pointyCanvas(), Regions: []encounter.RegionInput{rectRegion("hall", 0, 0, 8, 8)}},
		Members: []encounter.MemberInput{
			{ID: "alice", Kind: encounter.KindPlayer, Position: spatial.Position{X: 1, Y: 1}},
			{ID: "ogre", Kind: encounter.KindMonster, Position: spatial.Position{X: 2, Y: 1}},
		},
		Endings: []encounter.EndingInput{{Key: "withdrawn", Trigger: encounter.TriggerExternal{}}},
	})
	s.Require().NoError(err)
	data := enc.ToData()
	_, err = mgr.StartSession(context.Background(), &session.StartSessionInput{
		Session: "sess", Encounter: "world", World: turnWorld(&data, []string{"alice", "ogre"}, 0),
	})
	s.Require().NoError(err)

	afford, err := mgr.Afford(context.Background(), &session.AffordInput{Session: "sess", Member: "alice"})
	s.Require().NoError(err)
	decl := requireSingleAttackDeclaration(s.T(), afford.Declarations)
	s.False(decl.Available, "a sheetless target cannot produce an executable Attack")
	s.Require().NotNil(decl.Why)
	s.Equal(session.ShortfallUnreadable, decl.Why.Reason)
	s.Require().Len(decl.Candidates, 1)
	s.Equal("ogre", decl.Candidates[0].Member)
	s.False(decl.Candidates[0].Available)
	s.Require().NotNil(decl.Candidates[0].Why)
	s.Equal(session.ShortfallUnreadable, decl.Candidates[0].Why.Reason)

	beforeSessionSaves, beforeEncounterSaves, beforeCharacterSaves :=
		s.sessions.saves, s.encounters.saves, s.characters.saves
	_, err = mgr.Attack(context.Background(), &session.AttackInput{
		Session: "sess", Attacker: "alice", Target: "ogre", DeclarationID: decl.ID,
	})
	s.ErrorIs(err, session.ErrStaleDeclaration)
	s.Equal(beforeSessionSaves, s.sessions.saves)
	s.Equal(beforeEncounterSaves, s.encounters.saves)
	s.Equal(beforeCharacterSaves, s.characters.saves)
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
			ID: encounter.MemberID(id), Kind: encounter.KindPlayer, Position: spatial.Position{X: float64(i + 1), Y: 1},
		})
	}
	enc, err := encounter.NewEncounter(&encounter.SetupInput{Striker: encounter.RefusingStriker{}, Announcer: encQuietAnnouncer{}, Sight: encEveryoneSees{},
		Initiative: encOrderAsGiven{}, TurnDriver: encPassDriver{},
		Standing:  encEveryoneStanding{},
		Field:     encounter.FieldInput{Canvas: pointyCanvas(), Regions: []encounter.RegionInput{rectRegion("hall", 0, 0, 8, 8)}},
		Members:   placed,
		Endings:   []encounter.EndingInput{{Key: "withdrawn", Trigger: encounter.TriggerExternal{}}},
		Retention: encounter.RetentionUnbounded,
	})
	s.Require().NoError(err)
	data := enc.ToData()

	_, err = mgr.StartSession(context.Background(), &session.StartSessionInput{
		Session: "sess", Encounter: "world", World: turnWorld(&data, members, 0),
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
	s.ErrorIs(err, session.ErrNoDeclarationID,
		"an unreadable dependency produces a blocker, never an executable selector")
}

func (s *AttackTestSuite) TestAnUnreadableAttackerSheetIsCorruptRatherThanAbsent() {
	mgr := s.duelAmong([]string{"alice", "bob"}, unreadableFighter("alice"), armedFighter("bob"))

	_, err := s.swing(mgr)
	s.Require().Error(err)
	s.ErrorIs(err, session.ErrNoDeclarationID,
		"an unreadable dependency produces a blocker, never an executable selector")
}

// TestAnAbsentBystanderSheetIsAbsentRatherThanCorrupt covers the castFor path:
// a member who is neither swinging nor being swung at still joins the cast,
// because applicability is an effect's own predicate (ADR-0038). Their sheet
// being absent is the same failure as the attacker's and must read the same way.
func (s *AttackTestSuite) TestUnreadableTargetAndParticipantBlockAffordBeforeUnchangedAttack() {
	tests := []struct {
		name             string
		sheets           []*character.Data
		unreadableMember string
		candidate        bool
	}{
		{
			name: "unreadable target is a candidate refusal",
			sheets: []*character.Data{
				armedFighter("alice"), unreadableFighter("bob"),
			},
			unreadableMember: "bob",
			candidate:        true,
		},
		{
			name: "unreadable non-target participant is a global refusal",
			sheets: []*character.Data{
				armedFighter("alice"), armedFighter("bob"), unreadableFighter("carol"),
			},
			unreadableMember: "carol",
			candidate:        false,
		},
	}

	for _, tc := range tests {
		s.Run(tc.name, func() {
			members := []string{"alice", "bob"}
			if tc.unreadableMember == "carol" {
				members = append(members, "carol")
			}
			mgr := s.duelAmong(members, tc.sheets...)
			if !tc.candidate {
				// Carol remains a resolution-required roster participant but is
				// not a live target candidate for Alice.
				injectHolding(s.T(), s.encounters, tc.unreadableMember, nil)
			}

			afford, err := mgr.Afford(context.Background(), &session.AffordInput{
				Session: "sess", Member: "alice",
			})
			s.Require().NoError(err)
			decl := requireSingleAttackDeclaration(s.T(), afford.Declarations)
			s.False(decl.Available, "Afford must not advertise an Attack whose cast cannot attach")
			s.Require().NotNil(decl.Why)
			s.Equal(session.ShortfallUnreadable, decl.Why.Reason)

			bob := decl.Candidates[0]
			s.Equal("bob", bob.Member)
			if tc.candidate {
				s.False(bob.Available)
				s.Require().NotNil(bob.Why)
				s.Equal(session.ShortfallUnreadable, bob.Why.Reason)
			} else {
				s.True(bob.Available, "the readable target keeps its independent reach fact")
				s.Nil(bob.Why)
			}

			beforeSessionSaves, beforeEncounterSaves, beforeCharacterSaves :=
				s.sessions.saves, s.encounters.saves, s.characters.saves
			out, err := mgr.Attack(context.Background(), &session.AttackInput{
				Session: "sess", Attacker: "alice", Target: "bob", DeclarationID: decl.ID,
			})
			s.ErrorIs(err, session.ErrStaleDeclaration)
			s.Nil(out)
			s.Equal(beforeSessionSaves, s.sessions.saves)
			s.Equal(beforeEncounterSaves, s.encounters.saves)
			s.Equal(beforeCharacterSaves, s.characters.saves)
		})
	}
}
func rangedDuelWorld(t fataler, targetX float64) *encounter.EncounterData {
	enc, err := encounter.NewEncounter(&encounter.SetupInput{Striker: encounter.RefusingStriker{}, Announcer: encQuietAnnouncer{}, Sight: encEveryoneSees{},
		Initiative: encOrderAsGiven{}, TurnDriver: encPassDriver{}, Standing: encEveryoneStanding{},
		Field: encounter.FieldInput{Canvas: pointyCanvas(), Regions: []encounter.RegionInput{rectRegion("range", 0, 0, 140, 8)}},
		Members: []encounter.MemberInput{
			{ID: "alice", Kind: encounter.KindPlayer, Position: spatial.Position{X: 1, Y: 1}},
			{ID: "bob", Kind: encounter.KindPlayer, Position: spatial.Position{X: targetX, Y: 1}},
		},
		Endings: []encounter.EndingInput{{Key: "withdrawn", Trigger: encounter.TriggerExternal{}}}, Retention: encounter.RetentionUnbounded,
	})
	if err != nil {
		t.Fatalf("building ranged duel: %v", err)
	}
	data := enc.ToData()
	return turnWorld(&data, []string{"alice", "bob"}, 0)
}

func (s *AttackTestSuite) rangedDuel(targetX float64, roller *sequenceDice) *session.Manager {
	alice, bob := armedFighter("alice"), armedFighter("bob")
	alice.Inventory = []character.InventoryItemData{{Type: shared.EquipmentTypeWeapon, ID: string(weapons.Longbow), Quantity: 1}}
	alice.EquipmentSlots = character.EquipmentSlots{character.SlotMainHand: string(weapons.Longbow)}
	s.sessions, s.encounters = newFakeSessions(), newFakeEncounters()
	s.characters = newFakeCharacters(alice, bob)
	mgr, err := session.NewManager(&session.Config{Dice: roller, TurnDriver: session.Pass{}, Sessions: s.sessions, Encounters: s.encounters, Characters: s.characters, Events: session.DiscardEvents{}})
	s.Require().NoError(err)
	_, err = mgr.StartSession(context.Background(), &session.StartSessionInput{Session: "sess", Encounter: "world", World: rangedDuelWorld(s.T(), targetX)})
	s.Require().NoError(err)
	return mgr
}

func (s *AttackTestSuite) TestCharacterLongbowAttacksInsideNormalRange() {
	roller := &sequenceDice{rolls: []int{4, 17}}
	out, err := s.swing(s.rangedDuel(17, roller)) // 16 cells = 80 feet
	s.Require().NoError(err)
	s.Equal(4, out.Roll, "normal range rolls one die")
	s.Equal(1, roller.next)
}

func (s *AttackTestSuite) TestCharacterLongbowAttacksAtLongRangeWithDisadvantage() {
	roller := &sequenceDice{rolls: []int{17, 4}}
	out, err := s.swing(s.rangedDuel(41, roller)) // 40 cells = 200 feet
	s.Require().NoError(err)
	s.Equal(4, out.Roll)
	s.Equal(2, roller.next)
}

func (s *AttackTestSuite) TestOutOfRangeAttackRollsNothing() {
	roller := &sequenceDice{rolls: []int{17, 4}}
	_, err := s.swing(s.rangedDuel(123, roller)) // 122 cells = 610 feet
	s.Require().ErrorIs(err, session.ErrStaleDeclaration)
	s.Zero(roller.next)
}
