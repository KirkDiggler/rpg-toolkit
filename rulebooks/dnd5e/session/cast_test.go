// Copyright (C) 2026 Kirk Diggler
// SPDX-License-Identifier: GPL-3.0-or-later

package session_test

import (
	"context"
	"testing"

	"github.com/stretchr/testify/suite"

	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/character"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/conditions"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/encounter"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/proficiencies"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/refs"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/session"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/shared"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/spells"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/weapons"
	"github.com/KirkDiggler/rpg-toolkit/tools/spatial"
)

// The cast door, as a scene: a level-1 bard with two cantrips, a skeleton to
// point them at, and the panel a player actually reads.
//
// EVERY ASSERTION HERE IS ABOUT THE DOOR, not about what a cantrip does. What
// True Strike grants and what Vicious Mockery costs are the rulebook's and
// resolution's business, tested where they live; this suite pins that the row
// appears when it should, is priced once, is refused when the world moved, and
// that the beats a player reads reach the story.

type CastSuite struct {
	suite.Suite

	mgr        *session.Manager
	sessions   *fakeSessions
	characters *fakeCharacters
	stream     *fakeStream
}

func TestCastSuite(t *testing.T) {
	suite.Run(t, new(CastSuite))
}

// castingBard is levelOneBard with cantrips written onto the sheet — the sheet
// a finalized draft produces, written directly so this suite does not depend on
// the draft flow.
//
// CHA 16 and proficiency 2 make the spell save DC 13: 8 + 2 + 3.
func castingBard(id string, cantrips ...spells.Spell) *character.Data {
	bard := levelOneBard(id, 2)
	for _, cantrip := range cantrips {
		ref := refs.Spells.ByID(string(cantrip))
		if ref == nil {
			panic("the fixture named a spell the ref catalog does not have: " + cantrip)
		}
		bard.KnownCantrips = append(bard.KnownCantrips, ref.String())
	}
	return bard
}

// armForSwinging gives a sheet a longsword and the proficiency to use it, so
// it can open a post-roll window by swinging. Only the freeze scene needs it:
// a cast has nothing to do with what the caster is holding.
func armForSwinging(sheet *character.Data) {
	sheet.WeaponProficiencies = []proficiencies.Weapon{proficiencies.WeaponMartial}
	sheet.Inventory = []character.InventoryItemData{
		{Type: shared.EquipmentTypeWeapon, ID: string(weapons.Longsword), Quantity: 1},
	}
	sheet.EquipmentSlots = character.EquipmentSlots{
		character.SlotMainHand: string(weapons.Longsword),
	}
}

// bardSaveDC is what castingBard's own sheet answers: 8 + 2 proficiency + 3 for
// CHA 16. Spelled here because every save assertion below is against it, and a
// literal 13 in six places would hide the arithmetic that produced it.
const bardSaveDC = 13

// scene puts the bard in a fight with a skeleton `cells` away, with the dice
// scripted after initiative.
//
// The skeleton's distance is a parameter because RANGE IS PER SPELL: True
// Strike reaches 30 feet and Vicious Mockery 60, so one skeleton can be a
// candidate for one row and a shortfall on the other, which is the thing worth
// pinning.
func (s *CastSuite) scene(bard *character.Data, cells int, rolls ...int) {
	s.T().Helper()

	// One die per member of the bubble the spawn forms, ahead of whatever this
	// scene scripted.
	const initiativeRolls = 2
	scripted := append(make([]int, initiativeRolls), rolls...)

	s.characters = newFakeCharacters(bard)
	s.sessions = newFakeSessions()
	s.stream = &fakeStream{}
	mgr, err := session.NewManager(&session.Config{
		PresentationIDs: testPresentationIDs{}, Dice: &sequenceDice{rolls: scripted},
		TurnDriver: session.Pass{},
		Sessions:   s.sessions, Encounters: newFakeEncounters(),
		Characters: s.characters, Events: s.stream,
	})
	s.Require().NoError(err)
	s.mgr = mgr

	enc, err := encounter.NewEncounter(&encounter.SetupInput{
		Striker: encounter.RefusingStriker{}, Mover: encounter.RefusingMover{},
		Announcer: encQuietAnnouncer{}, Sight: encEveryoneSees{},
		Initiative: encOrderAsGiven{}, TurnDriver: encPassDriver{},
		Standing: encEveryoneStanding{},
		Field: encounter.FieldInput{
			Canvas:  pointyCanvas(),
			Regions: []encounter.RegionInput{rectRegion("hall", 0, 0, 40, 8)},
		},
		Members: []encounter.MemberInput{
			{ID: encounter.MemberID(bard.ID), Kind: encounter.KindPlayer,
				Position: spatial.Position{X: 1, Y: 1}},
		},
		Endings:   []encounter.EndingInput{{Key: "withdrawn", Trigger: encounter.TriggerExternal{}}},
		Retention: encounter.RetentionUnbounded,
	})
	s.Require().NoError(err)
	data := enc.ToData()

	ctx := context.Background()
	_, err = mgr.StartSession(ctx, &session.StartSessionInput{
		Session: "sess", Encounter: "world", World: &data,
	})
	s.Require().NoError(err)

	spawned, err := mgr.Spawn(ctx, &session.SpawnInput{
		Session: "sess", ID: "skeleton", Ref: refs.Monsters.Skeleton().String(),
		Position: spatial.Position{X: float64(1 + cells), Y: 1},
	})
	s.Require().NoError(err)
	s.Require().NotNil(spawned.Formed, "arriving in plain sight must start a fight")

	turn, err := mgr.Turn(ctx, &session.TurnInput{Session: "sess", Member: bard.ID})
	s.Require().NoError(err)
	s.Require().Equal(session.ClockTurn, turn.Clock, "a cast is priced on the turn clock")
	s.Require().Equal(bard.ID, turn.Active)
	s.stream.published = nil
}

// holdInspiration puts a Bardic Inspiration die on the member's stored sheet.
// The grant itself is exercised elsewhere; the freeze scene starts from a
// member who already holds one, so a change to the grant cannot quietly
// rewrite what this scene is testing.
func (s *CastSuite) holdInspiration(member string) {
	s.T().Helper()
	ctx := context.Background()
	stored, err := s.characters.GetCharacter(ctx, member)
	s.Require().NoError(err)
	raw, err := conditions.NewInspiredCondition(member, member, conditions.InspiredDie).ToJSON()
	s.Require().NoError(err)
	stored.Conditions = append(stored.Conditions, raw)
	s.Require().NoError(s.characters.SaveCharacter(ctx, stored))
}

// castRows is every VerbCast declaration the bard is currently offered.
func (s *CastSuite) castRows() []session.Declaration {
	s.T().Helper()
	out, err := s.mgr.Afford(context.Background(), &session.AffordInput{
		Session: "sess", Member: "bard",
	})
	s.Require().NoError(err)
	rows := make([]session.Declaration, 0, 2)
	for _, declaration := range out.Declarations {
		if declaration.Verb == session.VerbCast {
			rows = append(rows, declaration)
		}
	}
	return rows
}

// castRow is the one row for a named spell.
func (s *CastSuite) castRow(spell spells.Spell) session.Declaration {
	s.T().Helper()
	want := refs.Spells.ByID(string(spell)).String()
	for _, row := range s.castRows() {
		if row.Spell != nil && row.Spell.Ref == want {
			return row
		}
	}
	s.Require().Failf("no row", "the bard was offered no Cast row for %s", spell)
	return session.Declaration{}
}

// candidateFor finds one member's row in a declaration's candidate universe.
func castCandidate(
	t *testing.T, row session.Declaration, member string,
) (session.TargetCandidate, bool) {
	t.Helper()
	for _, candidate := range row.Candidates {
		if candidate.Member == member {
			return candidate, true
		}
	}
	return session.TargetCandidate{}, false
}

// TestTwoCastableCantripsAreTwoRows is the slice's own done-when at the panel:
// a bard who knows both cantrips this build can cast sees both, one per row,
// each priced at the action and each naming its own spell.
func (s *CastSuite) TestTwoCastableCantripsAreTwoRows() {
	s.scene(castingBard("bard", spells.TrueStrike, spells.ViciousMockery), 2)

	rows := s.castRows()
	s.Require().Len(rows, 2, "one row per castable cantrip, never one row for the verb")

	// Sorted by the spell's ref, which is a fact about the character rather
	// than about the order the sheet happened to hold the choices in.
	s.Equal(refs.Spells.TrueStrike().String(), rows[0].Spell.Ref)
	s.Equal(refs.Spells.ViciousMockery().String(), rows[1].Spell.Ref)
	s.Equal("True Strike", rows[0].Spell.Name)
	s.Equal("Vicious Mockery", rows[1].Spell.Name)

	for _, row := range rows {
		s.Equal(session.SlotAction, row.Slot, "a cantrip costs the action and nothing else")
		s.Equal(session.TargetMember, row.TargetKind, "both name one creature")
		s.True(row.Available, "a bard with an action and a skeleton in range can cast")
		s.Nil(row.Why)
		s.NotEmpty(row.ID, "a compiled offer carries a selector")
		s.Nil(row.Ability, "a cast row is not an activation row")
		s.Nil(row.Attack)

		candidate, found := castCandidate(s.T(), row, "skeleton")
		s.Require().True(found, "the skeleton is in the candidate universe")
		s.True(candidate.Available)
	}
	s.NotEqual(rows[0].ID, rows[1].ID, "two spells are two selectors")
}

// TestCantripsWithNoBehaviourAreNoRowsAtAll is R9's consequence, made real: a
// bard who chose Mage Hand and Light is offered no Cast row, rather than two
// permanently disabled ones.
func (s *CastSuite) TestCantripsWithNoBehaviourAreNoRowsAtAll() {
	s.scene(castingBard("bard", spells.MageHand, spells.Light), 2)
	s.Empty(s.castRows(),
		"a known cantrip this build cannot cast mints no row — a row that resolved to nothing would be the lie")
}

// TestOnlyTheCastableHalfOfAMixedListMintsRows pins the intersection rather
// than either list alone.
func (s *CastSuite) TestOnlyTheCastableHalfOfAMixedListMintsRows() {
	s.scene(castingBard("bard", spells.MageHand, spells.ViciousMockery), 2)

	rows := s.castRows()
	s.Require().Len(rows, 1)
	s.Equal(refs.Spells.ViciousMockery().String(), rows[0].Spell.Ref)
}

// TestAFighterIsOfferedNoCastRow — the sheet's known list is empty for
// everybody who never chose a cantrip, and an empty list is not an error.
func (s *CastSuite) TestAFighterIsOfferedNoCastRow() {
	s.scene(armedFighter("bard"), 2)
	s.Empty(s.castRows(), "a fighter knows no cantrips and is offered no cast")
}

// TestRangeIsPerSpell is the point of compiling a definition per cantrip: one
// skeleton, eight cells away, is inside Vicious Mockery's 60 feet and outside
// True Strike's 30.
//
// The out-of-range target KEEPS ITS ROW with its own reason — the same law
// Attack's candidates keep — and the declaration says no target in range
// without erasing the candidate the player can see standing there.
func (s *CastSuite) TestRangeIsPerSpell() {
	s.scene(castingBard("bard", spells.TrueStrike, spells.ViciousMockery), 8)

	mockery := s.castRow(spells.ViciousMockery)
	s.True(mockery.Available, "40 feet is inside Vicious Mockery's 60")
	mockeryTarget, found := castCandidate(s.T(), mockery, "skeleton")
	s.Require().True(found)
	s.True(mockeryTarget.Available)

	trueStrike := s.castRow(spells.TrueStrike)
	s.False(trueStrike.Available, "40 feet is outside True Strike's 30")
	s.Require().NotNil(trueStrike.Why)
	s.Equal(session.ShortfallNoTargetInReach, trueStrike.Why.Reason)

	strikeTarget, found := castCandidate(s.T(), trueStrike, "skeleton")
	s.Require().True(found, "a shortfall does not erase the candidate rows")
	s.False(strikeTarget.Available)
	s.Require().NotNil(strikeTarget.Why)
	s.Equal(session.ShortfallTargetOutOfReach, strikeTarget.Why.Reason)

	// AND IT IS A SHORTFALL RATHER THAN AN ERROR: the read succeeded and said
	// why, which is the whole job of the row.
	s.NotEmpty(trueStrike.ID, "an unavailable offer still carries its selector")
}

// TestTheCastRowIsBlockedWhileAWindowIsOpen — while an interrupt window is
// open anywhere in the fight, every declaring verb is announced as unavailable
// before the click, because every one of them will be refused at its own door.
//
// The window is a REAL one: the bard, holding a Bardic Inspiration die, swings
// and the swing stops after the d20 to ask whether to spend it. Borrowed
// machinery rather than a fabricated ledger, so what this pins is what the seam
// actually does while the table is waiting.
func (s *CastSuite) TestTheCastRowIsBlockedWhileAWindowIsOpen() {
	bard := castingBard("bard", spells.TrueStrike, spells.ViciousMockery)
	armForSwinging(bard)
	// Initiative twice, then the attack's own d20 — which pauses before
	// anything else is rolled.
	s.scene(bard, 1, 12)
	s.holdInspiration("bard")

	_, err := s.mgr.Attack(context.Background(), &session.AttackInput{
		Session: "sess", Attacker: "bard", Target: "skeleton",
		DeclarationID: currentAttackID(s.T(), s.mgr, "sess", "bard"),
	})
	s.Require().NoError(err)

	// ONE blocked Cast row, not one per cantrip: the reason is identical for
	// every spell, and two copies of "the table is waiting" would be a panel
	// that looks like it has choices.
	rows := s.castRows()
	s.Require().Len(rows, 1)
	s.False(rows[0].Available)
	s.Require().NotNil(rows[0].Why)
	s.Equal(session.ShortfallWindowOpen, rows[0].Why.Reason)
	s.Empty(rows[0].ID, "a blocker has compiled no offer")
	s.Nil(rows[0].Spell, "a blocker names no spell")

	// And the door refuses what the panel announced.
	_, err = s.mgr.Cast(context.Background(), &session.CastInput{
		Session: "sess", Member: "bard", Target: "skeleton", DeclarationID: "anything",
	})
	s.ErrorIs(err, session.ErrWindowOpen,
		"Cast is refused while the table is waiting, exactly as Afford announced")
}

// TestNotYourTurnIsOneCastRow — the same collapse, for the same reason.
func (s *CastSuite) TestNotYourTurnIsOneCastRow() {
	s.scene(castingBard("bard", spells.TrueStrike, spells.ViciousMockery), 2)

	out, err := s.mgr.Afford(context.Background(), &session.AffordInput{
		Session: "sess", Member: "skeleton",
	})
	s.Require().NoError(err)
	rows := 0
	for _, declaration := range out.Declarations {
		if declaration.Verb != session.VerbCast {
			continue
		}
		rows++
		s.False(declaration.Available)
		s.Require().NotNil(declaration.Why)
		s.Equal(session.ShortfallNotYourTurn, declaration.Why.Reason)
		s.Empty(declaration.ID)
	}
	s.Equal(1, rows, "one blocked Cast row, not one per cantrip")
}

// TestCastIsRefusedForAMonster — a monster's innate cast is driven by
// behaviour rather than declared, and its economy belongs to whoever runs its
// turn. The same line Attack and Activate draw.
func (s *CastSuite) TestCastIsRefusedForAMonster() {
	s.scene(castingBard("bard", spells.ViciousMockery), 2)

	_, err := s.mgr.Cast(context.Background(), &session.CastInput{
		Session: "sess", Member: "skeleton", DeclarationID: "anything", Target: "bard",
	})
	s.ErrorIs(err, session.ErrNotACharacter)
}

// TestCastRefusesAnEmptySelector — the caller's own omission, and it is that
// rather than a stale offer.
func (s *CastSuite) TestCastRefusesAnEmptySelector() {
	s.scene(castingBard("bard", spells.ViciousMockery), 2)

	_, err := s.mgr.Cast(context.Background(), &session.CastInput{
		Session: "sess", Member: "bard", Target: "skeleton",
	})
	s.ErrorIs(err, session.ErrNoDeclarationID)
}

// TestCastRefusesATargetOutsideTheOffer — the offer's per-candidate gate is
// re-enforced at execution, not only shown at the door. A client that echoes a
// selector against somebody the offer never listed is refused as stale.
func (s *CastSuite) TestCastRefusesATargetOutsideTheOffer() {
	s.scene(castingBard("bard", spells.ViciousMockery), 2)

	_, err := s.mgr.Cast(context.Background(), &session.CastInput{
		Session: "sess", Member: "bard", Target: "nobody",
		DeclarationID: s.castRow(spells.ViciousMockery).ID,
	})
	s.ErrorIs(err, session.ErrStaleDeclaration)
}

// TestCastRefusesNoTargetForASpellThatNeedsOne — a cast that names one
// creature and was given none is a caller defect, refused rather than resolved
// against nobody.
func (s *CastSuite) TestCastRefusesNoTargetForASpellThatNeedsOne() {
	s.scene(castingBard("bard", spells.ViciousMockery), 2)

	_, err := s.mgr.Cast(context.Background(), &session.CastInput{
		Session: "sess", Member: "bard",
		DeclarationID: s.castRow(spells.ViciousMockery).ID,
	})
	s.ErrorIs(err, session.ErrBadCast)
}

// cast points the named spell at the skeleton through the current offer.
func (s *CastSuite) cast(spell spells.Spell) (*session.CastOutput, error) {
	s.T().Helper()
	return s.mgr.Cast(context.Background(), &session.CastInput{
		Session: "sess", Member: "bard", Target: "skeleton",
		DeclarationID: s.castRow(spell).ID,
	})
}

// storedSkeleton reads the NPC's hit points back out of the session record.
func (s *CastSuite) storedSkeleton() int {
	s.T().Helper()
	for _, npc := range s.sessions.byID["sess"].NPCs {
		if npc.ID == "skeleton" {
			return npc.HitPoints
		}
	}
	s.FailNow("no skeleton in the record")
	return 0
}

// woundSkeleton sets the skeleton's stored hit points, so a 1d4 can finish it.
func (s *CastSuite) woundSkeleton(hp int) {
	s.T().Helper()
	for i := range s.sessions.byID["sess"].NPCs {
		if s.sessions.byID["sess"].NPCs[i].ID == "skeleton" {
			s.sessions.byID["sess"].NPCs[i].HitPoints = hp
			return
		}
	}
	s.FailNow("no skeleton in the record")
}

// beats is every event of the named kinds the bard received, in order.
func (s *CastSuite) beats(kinds ...session.EventKind) []session.Event {
	s.T().Helper()
	wanted := make(map[session.EventKind]bool, len(kinds))
	for _, kind := range kinds {
		wanted[kind] = true
	}
	out := make([]session.Event, 0, 4)
	for _, event := range eventsFor(s.stream.published, "bard") {
		if wanted[event.Kind] {
			out = append(out, event)
		}
	}
	return out
}

// TestTrueStrikeIsACastBeatAndAConditionOnTheCaster is the gateless half of the
// door: no roll, no save beat, and the condition lands on the BARD rather than
// on the creature it was pointed at.
func (s *CastSuite) TestTrueStrikeIsACastBeatAndAConditionOnTheCaster() {
	s.scene(castingBard("bard", spells.TrueStrike), 2)

	out, err := s.cast(spells.TrueStrike)
	s.Require().NoError(err)
	s.Equal(refs.Spells.TrueStrike().String(), out.Spell.Ref)
	s.Nil(out.Saved, "nobody resists True Strike, and a save report would say one did")
	s.Require().Len(out.Seqs, 2, "the cast beat and one result beat")

	events := s.beats(session.EventCast, session.EventSaved, session.EventActivationResult)
	s.Require().Len(events, 2, "a cast beat and its result, and no save beat at all")

	cast, ok := events[0].Body.(session.CastBody)
	s.Require().True(ok)
	s.Equal("bard", cast.Actor)
	s.Equal(refs.Spells.TrueStrike().String(), cast.Spell.Ref)
	s.Equal("True Strike", cast.Spell.Name, "the server authors the label")
	s.Equal("skeleton", cast.Target, "the creature it was pointed at is on the beat")

	result, ok := events[1].Body.(session.ActivationResultBody)
	s.Require().True(ok)
	s.Require().NotNil(result.ConditionApplied)
	s.Equal("bard", result.ConditionApplied.Target,
		"True Strike's advantage is the CASTER's, keyed to the creature named")
	s.Equal(refs.Conditions.TrueStrike().String(), result.ConditionApplied.Ref)
	s.Nil(result.DamageApplied)
}

// TestAFailedSaveAgainstViciousMockeryDeliversBothHalves is the gated half: the
// save beat carries the whole roll, and the failure delivers the psychic damage
// and the rider.
func (s *CastSuite) TestAFailedSaveAgainstViciousMockeryDeliversBothHalves() {
	// The skeleton's Wisdom save reaches 10, under the bard's DC 13, and the
	// 1d4 comes up 4.
	s.scene(castingBard("bard", spells.ViciousMockery), 2, 10, 4)
	before := s.storedSkeleton()

	out, err := s.cast(spells.ViciousMockery)
	s.Require().NoError(err)
	s.Require().NotNil(out.Saved, "a gated cast rolled a save and must say so")
	s.Equal("skeleton", out.Saved.Saver)
	s.Equal(bardSaveDC, out.Saved.DC, "the caster's own DC: 8 + 2 proficiency + 3 for CHA 16")
	s.False(out.Saved.Succeeded)

	saved := s.beats(session.EventSaved)
	s.Require().Len(saved, 1)
	body, ok := saved[0].Body.(session.SavedBody)
	s.Require().True(ok)
	s.Equal("skeleton", body.Saver)
	s.Equal(bardSaveDC, body.DC)
	s.False(body.Succeeded, "false is the answer here, not an absence")
	s.Equal(refs.Spells.ViciousMockery().String(), body.Source.Ref,
		"the save names what demanded it")

	results := s.beats(session.EventActivationResult)
	s.Require().Len(results, 2, "damage and the rider")
	damage, ok := results[0].Body.(session.ActivationResultBody)
	s.Require().True(ok)
	s.Require().NotNil(damage.DamageApplied)
	s.Equal("skeleton", damage.DamageApplied.Target)
	s.Equal(refs.Spells.ViciousMockery().String(), damage.DamageApplied.SourceRef,
		"the spell is what dealt it")
	s.Equal(session.DamagePsychic, damage.DamageApplied.DamageType,
		"psychic reaches the client as itself — a client that had to infer it from "+
			"the spell ref would be deriving a rule the record already holds")
	s.Require().NotNil(damage.DamageApplied.Calculation,
		"the 1d4 face reaches the client, not only its total")

	rider, ok := results[1].Body.(session.ActivationResultBody)
	s.Require().True(ok)
	s.Require().NotNil(rider.ConditionApplied)
	s.Equal("skeleton", rider.ConditionApplied.Target,
		"Vicious Mockery's disadvantage is the TARGET's")
	s.Equal(refs.Conditions.ViciousMockery().String(), rider.ConditionApplied.Ref)

	s.Less(s.storedSkeleton(), before, "the psychic damage reached the stored sheet")
}

// TestASuccessfulSaveDeliversNothing — RAW 2014: a made Wisdom save negates the
// damage AND the rider. The save beat is the whole story, and that is a
// complete cast rather than a cast that failed.
func (s *CastSuite) TestASuccessfulSaveDeliversNothing() {
	// 15 beats the bard's DC 13.
	s.scene(castingBard("bard", spells.ViciousMockery), 2, 15)
	before := s.storedSkeleton()

	out, err := s.cast(spells.ViciousMockery)
	s.Require().NoError(err)
	s.Require().NotNil(out.Saved)
	s.True(out.Saved.Succeeded)
	s.Require().Len(out.Seqs, 2, "the cast beat and the save beat, and nothing after them")

	s.Empty(s.beats(session.EventActivationResult),
		"a made save negates both halves, so there is nothing to narrate")
	s.Equal(before, s.storedSkeleton(), "and nothing reached the sheet")
}

// TestTheActionIsChargedOnceAndASecondCastIsRefused is R10 at the door: the
// price is paid there, and nothing in the verb counts casts — the ledger does.
func (s *CastSuite) TestTheActionIsChargedOnceAndASecondCastIsRefused() {
	s.scene(castingBard("bard", spells.ViciousMockery), 2, 15)

	first := s.castRow(spells.ViciousMockery)
	s.Require().True(first.Available)

	_, err := s.mgr.Cast(context.Background(), &session.CastInput{
		Session: "sess", Member: "bard", Target: "skeleton", DeclarationID: first.ID,
	})
	s.Require().NoError(err)

	// The regenerated offer reads the same ledger the door charged, so the
	// panel goes dark before the second click rather than after it.
	second := s.castRow(spells.ViciousMockery)
	s.False(second.Available, "the action is spent")
	s.Require().NotNil(second.Why)
	s.Equal(session.ShortfallNoBudget, second.Why.Reason)
	s.Equal(session.CurrencyAction, second.Why.Currency)
	s.Equal(0, second.Why.Left)

	// And the door refuses the selector the panel just disabled.
	_, err = s.mgr.Cast(context.Background(), &session.CastInput{
		Session: "sess", Member: "bard", Target: "skeleton", DeclarationID: second.ID,
	})
	s.ErrorIs(err, session.ErrStaleDeclaration,
		"charged once, and the second cast is refused rather than charged again")
}

// TestACantripCanDropATarget — damage delivered by a cast flows exactly like a
// strike's: the sheet is written, and the composition notices the drop for
// itself rather than being told.
func (s *CastSuite) TestACantripCanDropATarget() {
	s.scene(castingBard("bard", spells.ViciousMockery), 2, 10, 4)
	s.woundSkeleton(2)

	_, err := s.cast(spells.ViciousMockery)
	s.Require().NoError(err)

	s.Equal(0, s.storedSkeleton(), "4 psychic through 2 hit points leaves none")
	s.NotEmpty(s.beats(session.EventDowned),
		"the world noticed the drop; nobody announced it")
}
