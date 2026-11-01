// Copyright (C) 2026 Kirk Diggler
// SPDX-License-Identifier: GPL-3.0-or-later

package session_test

import (
	"context"
	"encoding/json"
	"testing"

	"github.com/stretchr/testify/suite"

	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/encounter"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/encounter/dungeonspec"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/refs"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/session"
	"github.com/KirkDiggler/rpg-toolkit/tools/spatial"
)

// ReadTestSuite covers the read verbs and, more importantly, the
// projection boundary they sit on.
type ReadTestSuite struct {
	suite.Suite

	sessions   *fakeSessions
	encounters *fakeEncounters
	characters *fakeCharacters
	mgr        *session.Manager
}

func (s *ReadTestSuite) SetupTest() {
	s.sessions = newFakeSessions()
	s.encounters = newFakeEncounters()
	s.characters = testCharacters()
	mgr, err := session.NewManager(&session.Config{Record: session.DiscardRecord{}, Acts: session.DiscardActs{}, Seats: newFakeSeats(), CharacterConfig: newFakeCharacterConfig(), PresentationIDs: testPresentationIDs{}, Dice: testDice{}, TurnDriver: session.Pass{},
		Sessions: s.sessions, Encounters: s.encounters, Characters: s.characters,
	})
	s.Require().NoError(err)
	s.mgr = mgr
}

// startWith launches the given scene as the session named "sess", stocking a
// sheet for any party member the repository does not already hold.
func (s *ReadTestSuite) startWith(world scene) {
	stockAuthoredPlayers(world, s.characters)
	launchScene(s.T(), s.mgr, world)
}

// hexWorld is a two-region hex field with a door, an occluder and a wall —
// rich enough that a projection dropping any one field is visible.
func hexWorld() scene {
	return scene{
		Field: encounter.FieldInput{Canvas: pointyCanvas(),
			// Two chambers side by side in the AUTHORED frame: the corridor
			// owns columns 0..5 and the vault 6..11, rows 0..5 each.
			Regions: []encounter.RegionInput{
				rectRegion("corridor", 0, 0, 6, 6),
				rectRegion("vault", 6, 0, 6, 6),
			},
			Props: occludingProps(spatial.Position{X: 1, Y: 1}),
			// The seam between them, open only on row 0 where the gate is.
			// Without these the two chambers are one open space and a walker
			// crosses anywhere along the edge.
			Walls: hexSeamWalls(6, 6, 0),
			Doors: []encounter.DoorInput{{
				ID: "gate",
				// The corridor's own LAST column meets the vault's FIRST, on
				// the shared row the wall leaves open.
				Edges: []encounter.DoorEdge{{From: hexCell(5, 0), To: hexCell(6, 0)}},
				State: encounter.DoorIsOpen(),
			}},
		},
		Party:   []sceneSeat{seatAt("alice", 0, 0)},
		Endings: []encounter.EndingInput{{Key: "out", Trigger: encounter.TriggerExternal{}}},
	}
}

// TestReadVerbsRejectMissingIdentifiers walks every read verb through the same
// rejections, so a verb added later without its guards fails here.
func (s *ReadTestSuite) TestReadVerbsRejectMissingIdentifiers() {
	ctx := context.Background()

	s.Run("nil inputs", func() {
		_, err := s.mgr.Atlas(ctx, nil)
		s.ErrorIs(err, session.ErrNilInput)
		_, err = s.mgr.Status(ctx, nil)
		s.ErrorIs(err, session.ErrNilInput)
		_, err = s.mgr.View(ctx, nil)
		s.ErrorIs(err, session.ErrNilInput)
	})

	s.Run("empty session id", func() {
		_, err := s.mgr.Atlas(ctx, &session.AtlasInput{Member: "alice"})
		s.ErrorIs(err, session.ErrNoSessionID)
		_, err = s.mgr.Status(ctx, &session.StatusInput{})
		s.ErrorIs(err, session.ErrNoSessionID)
	})

	s.Run("empty member id", func() {
		_, err := s.mgr.Atlas(ctx, &session.AtlasInput{Session: "sess"})
		s.ErrorIs(err, session.ErrNoMemberID)
		_, err = s.mgr.View(ctx, &session.ViewInput{Session: "sess"})
		s.ErrorIs(err, session.ErrNoMemberID)
	})

	s.Run("unknown session", func() {
		_, err := s.mgr.Atlas(ctx, &session.AtlasInput{Session: "nope", Member: "alice"})
		s.ErrorIs(err, session.ErrNoSession)
	})
}

// TestMissingWorldIsDistinctFromMissingSession pins the two failures apart.
//
// A session that exists but points at a world the store does not hold is a
// data-integrity problem on the host's side; an unknown session is an ordinary
// bad request. Collapsing them would send an operator hunting for a deleted
// session when the actual damage is a missing encounter.
func (s *ReadTestSuite) TestMissingWorldIsDistinctFromMissingSession() {
	ctx := context.Background()
	s.Require().NoError(s.sessions.SaveSession(ctx,
		&session.SessionData{ID: "orphan", Encounter: "vanished"}))

	_, err := s.mgr.Atlas(ctx, &session.AtlasInput{Session: "orphan", Member: "alice"})
	s.Require().Error(err)
	s.ErrorIs(err, session.ErrNoEncounter)
	s.NotErrorIs(err, session.ErrNoSession, "the session was found; the world was not")
}

// TestAtlasProjectsTheKnownWorld is the projection's positive control.
//
// A translation that silently dropped a field would satisfy any "did it return
// something" assertion, so every field carried across is asserted — including
// the ones an incomplete projection is most likely to forget: occluders,
// boundaries and the doorway pair. Discovery is by cell (rpg-project#586), so
// the control is the author's whole atlas restricted to alice's known cells:
// the pillar shades part of the corridor and the open gate frames only a
// slice of the vault.
func (s *ReadTestSuite) TestAtlasProjectsTheKnownWorld() {
	world := hexWorld()
	s.startWith(world)
	ctx := context.Background()

	atlas, err := s.mgr.Atlas(ctx, &session.AtlasInput{Session: "sess", Member: "alice"})
	s.Require().NoError(err)
	whole, err := s.mgr.AtlasOf(ctx, &session.AtlasOfInput{Dungeon: sceneInput(world).Dungeon})
	s.Require().NoError(err)

	s.Equal(session.GridHex, atlas.Grid, "the grid family must survive as the wire enum")
	requireKnownSliceOf(s.T(), whole, atlas)
	s.Require().Len(atlas.Regions, 2, "the vault is known by the cells seen through the gate")
	s.Less(len(atlas.Cells), len(whole.Cells), "unseen cells are withheld, not projected")
	s.Require().Len(atlas.Props, 1, "props are not optional decoration")
	s.True(atlas.Props[0].BlocksLineOfSight, "and it must still say it blocks sight")
	s.NotEmpty(atlas.Props[0].Ref, "and name what it is — the whole point of rpg-toolkit#1130")
	s.True(atlas.Boundaries[0].BlocksLineOfSight)
	s.True(atlas.Boundaries[0].BlocksMovement)

	s.Require().Len(atlas.Doorways, 1)
	gate := atlas.Doorways[0]
	s.Equal("gate", gate.Door)
	s.NotEqual(gate.From, gate.To, "the kissing pair is two distinct absolute cells")
}

// TestStatusReportsOpen covers the ordinary case; a closed encounter's outcome
// projection is exercised once End exists.
func (s *ReadTestSuite) TestStatusReportsOpen() {
	s.startWith(hexWorld())

	status, err := s.mgr.Status(context.Background(), &session.StatusInput{Session: "sess"})
	s.Require().NoError(err)
	s.True(status.Open)
	s.Nil(status.Outcome, "an open encounter has no outcome yet")
}

// TestViewReturnsProjectedSightings pins that perception survives the boundary.
func (s *ReadTestSuite) TestViewReturnsProjectedSightings() {
	s.startWith(hexWorld())

	sightings, err := s.mgr.View(context.Background(),
		&session.ViewInput{Session: "sess", Member: "alice"})
	s.Require().NoError(err)
	s.NotNil(sightings, "a member with nothing in view gets an empty result, not an error")
}

// TestViewRejectsUnknownMember pins the sentinel translation for members.
func (s *ReadTestSuite) TestViewRejectsUnknownMember() {
	s.startWith(hexWorld())

	_, err := s.mgr.View(context.Background(),
		&session.ViewInput{Session: "sess", Member: "nobody"})
	s.Require().Error(err)
	s.ErrorIs(err, session.ErrNoMember)
}

func TestReadSuite(t *testing.T) {
	suite.Run(t, new(ReadTestSuite))
}

// TestAtlasSaysWhichWayTheHexesPoint pins the one fact a client cannot recover
// from the cells (rpg-toolkit#1140). The same axial set draws as two different
// pictures, pointy-top and flat-top, and the first client to render the
// reference tomb guessed from the content and got a diagonal staircase. So the
// atlas says it — as a render word, Layout, not the authoring word the
// composition keeps, because confusing those two is exactly how the staircase
// happened.
func (s *ReadTestSuite) TestAtlasSaysWhichWayTheHexesPoint() {
	s.startWith(hexWorld())
	atlas, err := s.mgr.Atlas(context.Background(), &session.AtlasInput{Session: "sess", Member: "alice"})
	s.Require().NoError(err)
	s.Equal(session.HexLayoutPointyTop, atlas.Layout,
		"hexWorld is authored pointy-top, and after rpg-toolkit#1141 that IS the way to draw it")
}

// TestAtlasLayoutCoversBothHexLayouts guards the mapping in both directions:
// a projection hard-coded to pointy would pass every fixture in this file.
func (s *ReadTestSuite) TestAtlasLayoutCoversBothHexLayouts() {
	sc := scene{
		Field: encounter.FieldInput{Canvas: encounter.CanvasInput{Void: encounter.VoidIsOpaque(), Orientation: encounter.HexesAreFlatTop()},
			Regions: []encounter.RegionInput{rectRegion("cell", 0, 0, 4, 4)},
		},
		Party:   []sceneSeat{seatAt("alice", 0, 0)},
		Endings: []encounter.EndingInput{{Key: "out", Trigger: encounter.TriggerExternal{}}},
	}
	s.startWith(sc)

	atlas, err := s.mgr.Atlas(context.Background(), &session.AtlasInput{Session: "sess", Member: "alice"})
	s.Require().NoError(err)
	s.Equal(session.HexLayoutFlatTop, atlas.Layout, "a flat-top field must not be reported as pointy")
}

// TestViewCarriesNameAndStanding pins rpg-toolkit#1137's perception half:
// a sighted subject's display name and whether they are on their feet ride
// along on the SAME read that already answers "what do I see" — no second
// lookup, and no roster read this seam otherwise refuses to offer.
func (s *ReadTestSuite) TestViewCarriesNameAndStanding() {
	mgr, _, _, characters := aFight(s.T(), armedFighter("alice"), nil)
	s.mgr = mgr
	s.characters = characters

	sightings, err := s.mgr.View(context.Background(),
		&session.ViewInput{Session: "sess", Member: "alice"})
	s.Require().NoError(err)

	var skeleton *session.Sighting
	for i := range sightings.Sightings {
		if sightings.Sightings[i].Subject == "skeleton" {
			skeleton = &sightings.Sightings[i]
		}
	}
	s.Require().NotNil(skeleton, "alice must currently see the skeleton aFight spawned adjacent to her")
	s.NotEmpty(skeleton.Name, "a spawned monster's catalog display name projects onto Sighting.Name")
	s.Require().NotNil(skeleton.Seen, "a live sight-channel sighting carries Seen")
	s.Require().NotNil(skeleton.Seen.Standing, "a current sighting observes standing")
	s.Equal(session.StandingUp, *skeleton.Seen.Standing, "nothing has touched it yet")
}

// twoObservedWorld is one open 6x6 hall with no occluders and unbounded
// sight: "scout" watches "ally" (a fellow player) and "goblin" (a monster),
// both mutually visible from the moment the session starts — no walk needed
// to open sight, unlike seen_test.go's doorway scenes, because this is about
// what Kind reports once a sighting exists, not about how one opens.
func twoObservedWorld() scene {
	return scene{
		Field:    encounter.FieldInput{Canvas: pointyCanvas(), Regions: []encounter.RegionInput{rectRegion("hall", 0, 0, 6, 6)}},
		Party:    []sceneSeat{seatAt("scout", 0, 0), seatAt("ally", 1, 0)},
		Monsters: []dungeonspec.MonsterPlacement{monsterAt("goblin", refs.Monsters.Goblin().String(), 2, 0)},
		Endings:  []encounter.EndingInput{{Key: "out", Trigger: encounter.TriggerExternal{}}},
	}
}

// TestViewCarriesKind pins rpg-toolkit#1230: a Sighting reports what kind of
// member the subject is, looked up from the roster the same way Name already
// is, so a client can tell a player from a monster without guessing (the
// diagnosis behind rpg-dnd5e-web#792). One session, one player and one
// monster besides the observer, one View call — both kinds must come back
// right, not just whichever one a hard-coded default would fake.
func (s *ReadTestSuite) TestViewCarriesKind() {
	s.startWith(twoObservedWorld())

	sightings, err := s.mgr.View(context.Background(),
		&session.ViewInput{Session: "sess", Member: "scout"})
	s.Require().NoError(err)

	var ally, goblin *session.Sighting
	for i := range sightings.Sightings {
		switch sightings.Sightings[i].Subject {
		case "ally":
			ally = &sightings.Sightings[i]
		case "goblin":
			goblin = &sightings.Sightings[i]
		}
	}
	s.Require().NotNil(ally, "scout must see ally in the open hall")
	s.Require().NotNil(goblin, "scout must see goblin in the open hall")
	s.Equal(session.KindPlayer, ally.Kind, "a fellow player's Sighting must report Kind player")
	s.Equal(session.KindMonster, goblin.Kind, "a monster's Sighting must report Kind monster")
}

// TestOldSessionDataLoads (R29): a session saved while the v1alpha1 stream
// existed carries a `streams` key, each member's delivered-stream cursor.
// The cursor left with that stream (rpg-project#631), so the key is ignored:
// the session decodes, reads and takes a write verb.
func (s *ReadTestSuite) TestOldSessionDataLoads() {
	s.startWith(hexWorld())

	saved, err := json.Marshal(s.sessions.byID["sess"])
	s.Require().NoError(err)
	var blob map[string]json.RawMessage
	s.Require().NoError(json.Unmarshal(saved, &blob))
	blob["streams"] = json.RawMessage(`{"alice":{"up_to":7,"count":3}}`)
	older, err := json.Marshal(blob)
	s.Require().NoError(err)
	var data session.SessionData
	s.Require().NoError(json.Unmarshal(older, &data), "an older session decodes")
	s.sessions.byID["sess"] = &data

	_, err = s.mgr.View(context.Background(), &session.ViewInput{Session: "sess", Member: "alice"})
	s.Require().NoError(err, "an older session reads")
	_, err = s.mgr.Move(context.Background(), &session.MoveInput{
		Session: "sess", Member: "alice", Path: []spatial.Position{hexCell(1, 0)},
	})
	s.Require().NoError(err, "an older session takes a write verb")
}
