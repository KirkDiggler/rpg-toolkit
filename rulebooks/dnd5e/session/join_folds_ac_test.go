// Copyright (C) 2026 Kirk Diggler
// SPDX-License-Identifier: GPL-3.0-or-later

package session_test

import (
	"context"
	"encoding/json"
	"testing"

	"github.com/stretchr/testify/suite"

	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/abilities"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/armor"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/character"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/classes"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/conditions"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/races"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/session"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/shared"
)

// JoinFoldsACTestSuite is the base-AC-barbarian pin: the case the whole
// game-context slice was named for.
//
// A barbarian's armour class is not on the sheet. It is 10 + DEX + CON, folded
// from a condition, and it is right only if that condition reached the fold —
// which means the character had to be attached, on a bus, with the truth
// installed. Join does none of those things itself any more: it hands the
// record to resolution and takes back a number.
//
// The failure this pins is the one that was live in production. The stored
// scalar was written once at creation and refreshed only by an equip patch, so
// a barbarian who never changed gear reported base armour for the life of the
// character, and every layer above believed it.
type JoinFoldsACTestSuite struct {
	suite.Suite

	mgr        *session.Manager
	characters *fakeCharacters
	encounters *fakeEncounters
}

func TestJoinFoldsACSuite(t *testing.T) {
	suite.Run(t, new(JoinFoldsACTestSuite))
}

func (s *JoinFoldsACTestSuite) SetupTest() {
	s.characters = testCharacters()
	s.characters.byID[ragingID] = barbarianCharacter(ragingID)
	s.encounters = newFakeEncounters()

	mgr, err := session.NewManager(&session.Config{
		Dice: testDice{}, TurnDriver: session.Pass{},
		Sessions: newFakeSessions(), Encounters: s.encounters, Characters: s.characters,
		Events: session.DiscardEvents{},
	})
	s.Require().NoError(err)
	s.mgr = mgr

	_, err = mgr.StartSession(context.Background(), &session.StartSessionInput{
		Session: "sess", Encounter: "world", World: hexWorld(s.T()),
	})
	s.Require().NoError(err)
}

const ragingID = "standre"

// barbarianCharacter is unarmoured, carries Unarmored Defense, and has DEX +2
// against CON +3 — DELIBERATELY DIFFERENT, so a fold that reached for the wrong
// ability lands on a different number instead of the right one by luck.
//
// ArmorClass on the sheet says 11 and is meant to. That is the stale scalar the
// old read path returned; any assertion below that accidentally echoes the
// sheet reports 11 and says so out loud.
func barbarianCharacter(id string) *character.Data {
	raw, err := (&conditions.UnarmoredDefenseCondition{
		MemberID: id,
		Type:     conditions.UnarmoredDefenseBarbarian,
	}).ToJSON()
	if err != nil {
		panic("barbarian fixture: " + err.Error())
	}

	return &character.Data{
		ID:               id,
		PlayerID:         "player-" + id,
		Name:             "Standre",
		Level:            3,
		ProficiencyBonus: 2,
		RaceID:           races.Human,
		ClassID:          classes.Barbarian,
		HitPoints:        30,
		MaxHitPoints:     30,
		ArmorClass:       11,
		AbilityScores: shared.AbilityScores{
			abilities.STR: 16,
			abilities.DEX: 14, // +2
			abilities.CON: 16, // +3
			abilities.INT: 10,
			abilities.WIS: 12,
			abilities.CHA: 8,
		},
		Conditions: []json.RawMessage{raw},
	}
}

// TestABarbarianJoiningReportsItsFoldedAC states the number rather than
// comparing two runs of the same code.
//
// 15 = 10 base + 2 DEX + 3 CON. Not 11, which is what the sheet says; not 12,
// which is what an unarmoured fold reports when the condition never reached it.
// Those three numbers are far enough apart that the assertion says which of the
// three things went wrong, which a round-trip against another call could not.
func (s *JoinFoldsACTestSuite) TestABarbarianJoiningReportsItsFoldedAC() {
	out, err := s.mgr.Join(context.Background(), &session.JoinInput{
		Session: "sess", Member: ragingID,
		Position: hexCell(2, 2),
	})

	s.Require().NoError(err)
	s.Require().NotNil(out.Character)

	s.Require().Equal(15, out.Character.ArmorClass,
		"10 base + 2 DEX + 3 CON: Unarmored Defense reached the fold")
	s.Require().NotEqual(11, out.Character.ArmorClass,
		"and the stale scalar on the sheet was not echoed back")
	s.Require().NotEqual(12, out.Character.ArmorClass,
		"and the fold was not run without the condition attached")
}

// TestTheRestOfTheProjectionSurvivesTheReroute keeps the other fields honest.
//
// The AC now comes from resolution and everything beside it still comes from
// the loaded sheet, which is the seam most likely to be got wrong quietly: a
// reroute that returned a correct AC on a state built from the wrong record
// would pass the test above and be entirely broken.
//
// Speed carries the weight, as it does everywhere in this package — it is
// stored nowhere and derived from race when asked, so it cannot be produced by
// echoing bytes.
func (s *JoinFoldsACTestSuite) TestTheRestOfTheProjectionSurvivesTheReroute() {
	out, err := s.mgr.Join(context.Background(), &session.JoinInput{
		Session: "sess", Member: ragingID,
		Position: hexCell(2, 2),
	})

	s.Require().NoError(err)

	s.Equal(ragingID, out.Character.ID)
	s.Equal("Standre", out.Character.Name)
	s.Equal(3, out.Character.Level)
	s.Equal(30, out.Character.HitPoints)
	s.Equal(30, out.Character.MaxHitPoints)
	s.Equal(2, out.Character.ProficiencyBonus)
	s.Equal(30, out.Character.Speed, "a human's speed, derived rather than stored")
}

// TestTheSeatedMemberCarriesWhatResolutionDerived pins everything Join takes
// from the answer that does NOT come back out on JoinOutput.
//
// The armour class is read off JoinOutput and pinned three ways over. The speed
// and the main-hand attack are not on JoinOutput at all — they go into the
// member record the encounter stores, where a turn's movement budget and a
// TurnDriver eventually read them — so nothing here noticed where they came
// from. BOTH mutants survived the whole suite: seating every player with speed
// zero, and seating them with an attack of no range and no kind. That is how
// this test came to exist.
//
// The values are chosen so a dropped one is a different number rather than a
// plausible one. 30 is what a HUMAN walks, derived from race by the loaded
// sheet and on no record. 5 feet and "melee" are what an unarmed strike is —
// this fixture equips nothing, and the rules say empty hands still punch.
func (s *JoinFoldsACTestSuite) TestTheSeatedMemberCarriesWhatResolutionDerived() {
	_, err := s.mgr.Join(context.Background(), &session.JoinInput{
		Session: "sess", Member: ragingID, Position: hexCell(3, 2),
	})
	s.Require().NoError(err)

	world, ok := s.encounters.byID["world"]
	s.Require().True(ok, "the join committed a world")

	var seated bool
	for _, member := range world.Members {
		if string(member.ID) != ragingID {
			continue
		}
		seated = true
		s.Equal(30, member.SpeedFeet,
			"a Human walks 30: the seated member carries the speed resolution derived, not zero")

		s.Require().Len(member.Actions, 1, "the main-hand attack was compiled into the member record")
		s.Equal(5, member.Actions[0].RangeFeet,
			"an unarmed strike reaches 5 feet — 0 would seat a member who cannot reach anything")
		s.Equal("melee", member.Actions[0].Kind,
			"and it is made in reach; an empty kind is not a kind")
	}
	s.Require().True(seated, "the barbarian is on the stored roster")
}

// TestABadMainHandIsReportedAsABadAttack pins a vocabulary that was lost and
// given back, and the round trip is why this test still exists.
//
// Join has always documented ErrBadAttack for a main-hand weapon that will not
// compile. When the compiling moved into resolution's projection, that entry
// reported the failure as a bad participant and this seam could only translate
// it one way — so the answer became ErrBadCharacter, and this test was written
// to pin the NARROWING rather than let a host discover it.
//
// Resolution reports the finer failure under its own ErrBadAttack now, and this
// seam reads that to choose its own word. So the assertion flips: the thing it
// was written to record is over, and what it holds instead is the restoration.
//
// Both halves are asserted, because "is ErrBadAttack" alone would pass if
// everything became ErrBadAttack. A broken loadout and a sheet that will not
// reconstitute are different repairs, and the test says which this is by saying
// which it is NOT.
//
// The fixture is the reachable case rather than an invented one: a stored sheet
// whose main-hand slot names an item that is not in its inventory. The equip
// path cannot produce that; a persisted record can.
func (s *JoinFoldsACTestSuite) TestABadMainHandIsReportedAsABadAttack() {
	broken := barbarianCharacter("broken-hand")
	broken.EquipmentSlots = character.EquipmentSlots{character.SlotMainHand: armor.ChainMail}
	s.characters.byID[broken.ID] = broken

	_, err := s.mgr.Join(context.Background(), &session.JoinInput{
		Session: "sess", Member: "broken-hand", Position: hexCell(3, 2),
	})

	s.Require().Error(err)
	s.Require().ErrorIs(err, session.ErrBadAttack,
		"a weapon that will not compile is a broken loadout, and Join's doc has always said so")
	s.Assert().NotErrorIs(err, session.ErrBadCharacter,
		"and NOT the coarser word this briefly answered — a host that could tell a broken "+
			"weapon from a corrupt sheet can tell them apart again")
}
