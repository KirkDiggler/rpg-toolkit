// Copyright (C) 2026 Kirk Diggler
// SPDX-License-Identifier: GPL-3.0-or-later

package session_test

import (
	"context"
	"encoding/json"
	"errors"
	"strings"
	"testing"

	"github.com/stretchr/testify/suite"

	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/character"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/encounter"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/encounter/dungeonspec"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/refs"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/session"
	"github.com/KirkDiggler/rpg-toolkit/tools/spatial"
)

// party_status_test.go is A LINE GIVES A STATUS TO SEVERAL PEOPLE
// (rpg-project#990) at the session door, on a small kitchen: the cook sees
// alice and bob beside it, and not carol behind the wall. A line whose row
// gives `held` to the speaker's party in sight holds alice and bob on their
// saved sheets, each once; one already held answers "already held" and the
// others are still held; a receiver the rules have no sheet for refuses the
// whole line, and nothing of it is kept, on a plain store and a fenced one.
// And a game saved before the receiver was always stated is refused with the
// player's sentence, by name, on a read and on a write.

// receiverLine is a plain line to the cook whose row gives the hold to to.
func receiverLine(id, to string) encounter.Option {
	return encounter.Option{
		ID: id, Say: "Let us through.",
		Success: []encounter.Answer{{Weight: 1, Say: "Stay right there, all of you."}},
		Grants: []encounter.RowGrant{{Row: 0, Consequence: encounter.Consequence{
			ApplyStatus: &encounter.StatusGrant{ID: "held", Status: json.RawMessage(heldStatus), To: to},
		}}},
	}
}

// partyKitchen is the ambush hall with the cook alone on the board, neutral
// to the party: alice and bob beside it, carol behind the wall.
func partyKitchen() scene {
	cook := monsterAt("cook", refs.Monsters.BrownBear().String(), 0, 0)
	cook.Faction = "kitchen"
	cook.Options = []encounter.Option{
		receiverLine("hold-all", string(encounter.RecognitionPartyInSight)),
		receiverLine("hold-me", string(encounter.RecognitionSpeaker)),
	}
	// The party's hound, beside the cook: a creature of the party's own
	// side, whose sheet a test can take away to make one give fail.
	hound := monsterAt("zz-hound", refs.Monsters.Goblin().String(), 1, 1)
	hound.Faction = string(encounter.FactionParty)
	sc := ambushWorld()
	sc.Monsters = []dungeonspec.MonsterPlacement{cook, hound}
	sc.Party = []sceneSeat{
		{ID: "alice", At: spatial.Position{X: 1, Y: 0}},
		{ID: "bob", At: spatial.Position{X: 0, Y: 1}},
		{ID: "carol", At: spatial.Position{X: 6, Y: 0}},
	}
	sc.Field.Factions = []encounter.FactionInput{{ID: "kitchen", Mind: "cook"}}
	sc.Field.Dispositions = []encounter.DispositionInput{{
		Between: [2]encounter.FactionID{"kitchen", encounter.FactionParty}, Stance: encounter.StanceNeutral,
	}}
	return sc
}

type PartyStatusSuite struct {
	suite.Suite
	characters *fakeCharacters
	sessions   *fakeSessions
	encounters *fakeEncounters
	mgr        *session.Manager
}

func TestPartyStatusSuite(t *testing.T) { suite.Run(t, new(PartyStatusSuite)) }

func (s *PartyStatusSuite) SetupTest() {
	s.characters = newFakeCharacters(armedFighter("alice"), armedFighter("bob"), armedFighter("carol"))
	s.encounters, s.sessions = newFakeEncounters(), newFakeSessions()
	mgr, err := session.NewManager(&session.Config{Record: session.DiscardRecord{}, Acts: session.DiscardActs{}, Seats: newFakeSeats(),
		CharacterConfig: newFakeCharacterConfig(), PresentationIDs: testPresentationIDs{}, Dice: testDice{}, TurnDriver: session.Pass{},
		Sessions: s.sessions, Encounters: s.encounters, Characters: s.characters,
	})
	s.Require().NoError(err)
	s.mgr = mgr
	launched := launchScene(s.T(), s.mgr, partyKitchen())
	s.Require().Empty(launched.Formed, "nobody hostile is in sight at the start")
}

// encounterID is the one encounter the launch stored.
func (s *PartyStatusSuite) encounterID() string {
	s.Require().Len(s.encounters.byID, 1, "precondition: one stored encounter")
	for id := range s.encounters.byID {
		return id
	}
	return ""
}

func (s *PartyStatusSuite) say(option string) error {
	_, err := s.mgr.Speak(context.Background(), &session.SpeakInput{Session: "sess", Member: "alice", Target: "cook", OptionID: option})
	return err
}

// held counts the Held rows on who's saved sheet, as the host reads it.
func (s *PartyStatusSuite) held(who string) int {
	return heldRowsOf(s.T(), s.characters, who)
}

// heldRowsOf counts the Held rows a saved sheet shows: one status is one row.
func heldRowsOf(t *testing.T, characters *fakeCharacters, who string) int {
	t.Helper()
	stored, err := characters.GetCharacter(context.Background(), who)
	if err != nil {
		t.Fatalf("reading %s: %v", who, err)
	}
	loaded, err := character.Load(context.Background(), stored)
	if err != nil {
		t.Fatalf("loading %s: %v", who, err)
	}
	view, err := loaded.StatusView(&character.StatusViewInput{})
	if err != nil {
		t.Fatalf("viewing %s: %v", who, err)
	}
	n := 0
	for _, condition := range view.View.Conditions {
		if condition.Name == "Held" {
			n++
		}
	}
	return n
}

// Catches a receiver dropped, given twice, or given out of sight.
func (s *PartyStatusSuite) TestTheHoldGoesToEachOfThePartyTheCookSees() {
	s.Require().NoError(s.say("hold-all"))
	s.Equal(1, s.held("alice"), "the speaker, seen, is held once")
	s.Equal(1, s.held("bob"), "the companion in sight is held once")
	s.Equal(0, s.held("carol"), "the one behind the wall is not")
}

// Catches an "already held" answer stopping the others: alice holds it from
// her own line, and the party's line still holds bob.
func (s *PartyStatusSuite) TestOneAlreadyHeldDoesNotStopTheOthers() {
	s.Require().NoError(s.say("hold-me"))
	s.Require().Equal(1, s.held("alice"))
	s.Require().Equal(0, s.held("bob"))
	s.Require().NoError(s.say("hold-all"))
	s.Equal(1, s.held("alice"), "already held: still once")
	s.Equal(1, s.held("bob"), "and bob is held")
}

// Catches a partial hold: the hound's sheet is gone from the run. Through
// Speak the verb's own sight refuses the line before any give is asked (the
// give's own all-or-nothing is party_status_internal_test.go's), and nothing
// of the line is kept — no sheet, no encounter, no story beat — on this
// plain store.
func (s *PartyStatusSuite) TestAReceiverTheRulesCannotGiveRefusesTheWholeLine() {
	dropNPCSheet(s.T(), s.sessions.byID["sess"], "zz-hound")
	saves, sheetSaves := s.encounters.saves, s.characters.saves
	before, err := s.encounters.GetEncounter(context.Background(), s.encounterID())
	s.Require().NoError(err)

	err = s.say("hold-all")
	s.Require().Error(err, "a receiver with no sheet refuses the line")
	s.Contains(err.Error(), `"zz-hound": member has no stored sheet`, "refused for the hound's missing sheet")
	s.Equal(0, s.held("alice"), "alice, asked first, is not held either")
	s.Equal(sheetSaves, s.characters.saves, "no sheet was saved")
	s.Equal(saves, s.encounters.saves, "the encounter was not saved")
	after, err := s.encounters.GetEncounter(context.Background(), s.encounterID())
	s.Require().NoError(err)
	s.Equal(before.Log.NextSeq, after.Log.NextSeq, "the line is not in the record")
}

// dropNPCSheet takes a monster's stored sheet out of the session record.
func dropNPCSheet(t *testing.T, data *session.SessionData, id string) {
	t.Helper()
	for i := range data.NPCs {
		if data.NPCs[i].ID == id {
			data.NPCs = append(data.NPCs[:i], data.NPCs[i+1:]...)
			return
		}
	}
	t.Fatalf("precondition: %s has a stored sheet", id)
}

// A GAME SAVED BEFORE THE RECEIVER WAS STATED (the old shape: a status grant
// with no `to`) is refused on a read and on a write with the player's
// sentence, by name; the same game in the new shape loads.
func (s *PartyStatusSuite) TestAnOldSavedGameIsRefusedPlainly() {
	stored, err := s.encounters.GetEncounter(context.Background(), s.encounterID())
	s.Require().NoError(err)
	raw, err := json.Marshal(stored)
	s.Require().NoError(err)
	s.Require().Contains(string(raw), `"to":"party_in_sight"`, "precondition: the saved grant states its receiver")
	old := strings.ReplaceAll(strings.ReplaceAll(string(raw), `,"to":"party_in_sight"`, ""), `,"to":"speaker"`, "")
	var oldShape encounter.EncounterData
	s.Require().NoError(json.Unmarshal([]byte(old), &oldShape))
	s.encounters.byID[s.encounterID()] = &oldShape

	requireTooOld := func(err error, entry string) {
		s.Require().ErrorIs(err, session.ErrSavedByOlderVersion, entry)
		s.Require().ErrorIs(err, session.ErrInvalidWorld, entry)
		s.False(errors.Is(err, encounter.ErrSavedByOlderVersion), "%s: the composition's sentinel stays behind the seam", entry)
		var tooOld *session.SavedByOlderVersionError
		s.Require().ErrorAs(err, &tooOld, entry)
		s.Equal("This game was saved by an older version of the game and can no longer be loaded. Start a new game.",
			tooOld.Error(), entry)
		s.Contains(tooOld.Why, "with no receiver", "%s: why is kept for the logs", entry)
	}
	_, err = s.mgr.Where(context.Background(), &session.WhereInput{Session: "sess", Member: "alice"})
	requireTooOld(err, "a read")
	_, err = s.mgr.Move(context.Background(), &session.MoveInput{Session: "sess", Member: "alice",
		Path: []spatial.Position{{X: 2, Y: 0}}})
	requireTooOld(err, "a write")

	s.encounters.byID[s.encounterID()] = stored
	_, err = s.mgr.Where(context.Background(), &session.WhereInput{Session: "sess", Member: "alice"})
	s.Require().NoError(err, "the same game in the new shape loads")
}

// THE FENCED STORE, the one the api runs: a receiver the rules cannot give
// refuses the line, and nothing lands, because the verb never commits.
func TestAPartyHoldOnAFencedStoreIsAllOrNothing(t *testing.T) {
	store := newFencedStore()
	store.chars = newFakeCharacters(armedFighter("alice"), armedFighter("bob"), armedFighter("carol"))
	mgr, err := session.NewManager(&session.Config{
		Record: session.DiscardRecord{}, Acts: session.DiscardActs{}, Seats: store.seats,
		CharacterConfig: newFakeCharacterConfig(), PresentationIDs: testPresentationIDs{}, Dice: testDice{},
		TurnDriver: session.Pass{}, Sessions: storeSessions{store: store}, Encounters: store, Characters: store.chars,
	})
	if err != nil {
		t.Fatal(err)
	}
	launchScene(t, mgr, partyKitchen())
	stored, err := store.sessions.GetSession(context.Background(), "sess")
	if err != nil {
		t.Fatal(err)
	}
	dropNPCSheet(t, stored, "zz-hound")
	if err := store.sessions.SaveSession(context.Background(), stored); err != nil {
		t.Fatal(err)
	}
	landed := store.fencedSaves

	_, err = mgr.Speak(context.Background(), &session.SpeakInput{Session: "sess", Member: "alice", Target: "cook", OptionID: "hold-all"})
	if err == nil || !strings.Contains(err.Error(), `"zz-hound": member has no stored sheet`) {
		t.Fatalf("the hound's missing sheet refuses the line: %v", err)
	}
	if store.fencedSaves != landed {
		t.Fatalf("a fenced save landed: %d, was %d", store.fencedSaves, landed)
	}
	if n := heldRowsOf(t, store.chars, "alice"); n != 0 {
		t.Fatalf("alice is held %d times; nothing of the line may be kept", n)
	}
}
