// Copyright (C) 2026 Kirk Diggler
// SPDX-License-Identifier: GPL-3.0-or-later

package session_test

// dungeon_key_test.go pins the one fact this seam learned when it stopped
// carrying the World Builder's scene (rpg-project#479): WHICH DUNGEON A
// SESSION IS PLAYING.
//
// The map no longer says what the room looks like, so a host has to be told
// where to fetch that from — and the answer is a content key, written down
// once at launch and handed back on every atlas read. These tests are about
// carriage and nothing else, because carriage is all this package does with
// it: it is never parsed, never resolved, and never compared against anything
// this package knows.

import (
	"context"
	"encoding/json"
	"testing"

	"github.com/stretchr/testify/suite"

	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/session"
)

type DungeonKeySuite struct {
	suite.Suite

	sessions *fakeSessions
	mgr      *session.Manager
}

func TestDungeonKeySuite(t *testing.T) { suite.Run(t, new(DungeonKeySuite)) }

func (s *DungeonKeySuite) SetupTest() {
	s.sessions = newFakeSessions()
	mgr, err := session.NewManager(&session.Config{
		PresentationIDs: testPresentationIDs{}, Dice: testDice{}, TurnDriver: session.Pass{},
		Sessions: s.sessions, Encounters: newFakeEncounters(), Characters: testCharacters(),
		Events: session.DiscardEvents{},
	})
	s.Require().NoError(err)
	s.mgr = mgr
}

// start launches a session in the plain hall under the given key. An empty
// key is a host that had no entry to name, which is the other case under test
// rather than an omission in the fixture.
func (s *DungeonKeySuite) start(sessionID, encounterID, dungeon string) {
	_, err := s.mgr.StartSession(context.Background(), &session.StartSessionInput{
		Session: sessionID, Encounter: encounterID, World: plainHallWorld(s.T()), Dungeon: dungeon,
	})
	s.Require().NoError(err)
}

func (s *DungeonKeySuite) atlas(sessionID string) *session.Atlas {
	atlas, err := s.mgr.Atlas(context.Background(), &session.AtlasInput{Session: sessionID, Member: "alice"})
	s.Require().NoError(err)
	return atlas
}

// TestTheAtlasNamesTheDungeonTheSessionWasLaunchedFrom is the slice's whole
// claim from the player's side: the key the host launched with comes back on
// the map every play view already fetches, which is the only reason it is on
// the session record at all.
func (s *DungeonKeySuite) TestTheAtlasNamesTheDungeonTheSessionWasLaunchedFrom() {
	s.start("sess", "world", "workshop-room")

	s.Equal("workshop-room", s.atlas("sess").DungeonKey,
		"the map names the entry a host fetches the room's appearance from")
}

// TestASessionLaunchedWithNoKeyNamesNone pins the honest absence. Empty is a
// host that assembled a world itself rather than loading a registry entry —
// which is what every session before this field was — and the answer must be
// nothing rather than a stand-in a client would try to fetch.
func (s *DungeonKeySuite) TestASessionLaunchedWithNoKeyNamesNone() {
	s.start("sess", "world", "")

	s.Empty(s.atlas("sess").DungeonKey,
		"no key given, so no key reported — not a default and not the encounter id")
}

// TestTheKeyIsCarriedVerbatim pins that this package does not interpret it.
//
// A key is the HOST's own name for a registry entry, and every transformation
// that looks harmless here — trimming, lowercasing, splitting on a colon —
// is this package deciding what a content registry's keyspace looks like. The
// fixture is deliberately something no scheme this package could invent would
// leave alone.
func (s *DungeonKeySuite) TestTheKeyIsCarriedVerbatim() {
	const awkward = "  Workshop Room/ONE:v3  "
	s.start("sess", "world", awkward)

	s.Equal(awkward, s.atlas("sess").DungeonKey, "the host's own string, byte for byte")
}

// TestAKeyNamingNothingIsStillCarried pins that a miss is the host's to
// discover. This package asks no registry anything, so it cannot tell a live
// entry from a dead one — and refusing here would be a refusal invented from
// no evidence.
func (s *DungeonKeySuite) TestAKeyNamingNothingIsStillCarried() {
	s.start("sess", "world", "no-such-entry")

	s.Equal("no-such-entry", s.atlas("sess").DungeonKey,
		"a content miss is discovered by the host that fetches, not refused here")
}

// TestTheKeyIsPersistedOnTheRecord pins where it lives. The atlas answer
// above would also pass if the key were held in memory by the Manager, which
// is exactly the shape S1 forbids — nothing is retained between verbs, so a
// key that is not on the record is a key that is gone by the next read.
func (s *DungeonKeySuite) TestTheKeyIsPersistedOnTheRecord() {
	s.start("sess", "world", "workshop-room")

	stored, err := json.Marshal(s.sessions.byID["sess"])
	s.Require().NoError(err)
	s.Contains(string(stored), `"dungeon":"workshop-room"`,
		"the record carries the key under its own json name")
}

// TestARecordWrittenWithNoKeyOmitsIt pins the other half of the persistence
// shape: a session that named no dungeon writes no key, rather than an empty
// string a reader would have to know to ignore.
func (s *DungeonKeySuite) TestARecordWrittenWithNoKeyOmitsIt() {
	s.start("sess", "world", "")

	stored, err := json.Marshal(s.sessions.byID["sess"])
	s.Require().NoError(err)
	s.NotContains(string(stored), `"dungeon"`, "absent, not empty")
}

// TestARecordWrittenBeforeTheKeyExistedLoadsWithNone is the no-migration
// claim, checked against the exact bytes such a record holds rather than
// against a value this test constructed.
//
// It rewrites the stored session with an older-shaped blob — id and encounter
// and nothing else — so the load path is the real one: the repository hands
// back those bytes, the Manager reads them, and the atlas reports what it
// found.
func (s *DungeonKeySuite) TestARecordWrittenBeforeTheKeyExistedLoadsWithNone() {
	s.start("sess", "world", "workshop-room")

	var older session.SessionData
	s.Require().NoError(json.Unmarshal([]byte(`{"id":"sess","encounter":"world"}`), &older))
	s.Empty(older.Dungeon, "an absent key unmarshals to no key")
	s.sessions.byID["sess"] = &older

	s.Empty(s.atlas("sess").DungeonKey, "an older record still loads, and names no dungeon")
}

// TestARecordWithAKeyLoadsWithIt is its twin, and the pair is what makes
// either one evidence: without this, a projection that ignored the record
// entirely would pass the test above.
func (s *DungeonKeySuite) TestARecordWithAKeyLoadsWithIt() {
	s.start("sess", "world", "")

	var stored session.SessionData
	s.Require().NoError(json.Unmarshal(
		[]byte(`{"id":"sess","encounter":"world","dungeon":"workshop-room"}`), &stored))
	s.sessions.byID["sess"] = &stored

	s.Equal("workshop-room", s.atlas("sess").DungeonKey,
		"the key comes off the record on every read, not off the launch call")
}

// TestAtlasOfEchoesTheAuthorsOwnKey pins the authoring half. There is no
// session to ask, so the caller's key is handed straight back — which is what
// makes a builder's preview and a player's map the same answer, key included,
// and lets one client code path draw both.
func (s *DungeonKeySuite) TestAtlasOfEchoesTheAuthorsOwnKey() {
	preview, err := s.mgr.AtlasOf(context.Background(), &session.AtlasOfInput{
		World: plainHallWorld(s.T()), Dungeon: "workshop-room",
	})
	s.Require().NoError(err)
	s.Equal("workshop-room", preview.DungeonKey)

	s.start("sess", "world", "workshop-room")
	live := s.atlas("sess")
	s.Equal(preview.DungeonKey, live.DungeonKey, "what a builder previews is what the game plays")
	s.Equal(preview.Cells, live.Cells, "and the map underneath it is the same map it always was")
}

// TestAtlasOfWithNoKeyNamesNone pins that the echo is an echo. A preview of a
// world whose caller named no entry reports none, rather than inventing one
// from the world it was handed.
func (s *DungeonKeySuite) TestAtlasOfWithNoKeyNamesNone() {
	preview, err := s.mgr.AtlasOf(context.Background(), &session.AtlasOfInput{World: plainHallWorld(s.T())})
	s.Require().NoError(err)
	s.Empty(preview.DungeonKey)
}
