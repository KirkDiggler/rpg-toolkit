// Copyright (C) 2026 Kirk Diggler
// SPDX-License-Identifier: GPL-3.0-or-later

package session_test

import (
	"context"
	"errors"
	"testing"

	"github.com/stretchr/testify/suite"

	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/encounter"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/session"
	"github.com/KirkDiggler/rpg-toolkit/tools/spatial"
)

// ReadTestSuite covers the four read verbs and, more importantly, the
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
	mgr, err := session.NewManager(&session.Config{PresentationIDs: testPresentationIDs{}, Dice: testDice{}, TurnDriver: session.Pass{},
		Sessions: s.sessions, Encounters: s.encounters, Characters: s.characters,
		Events: session.DiscardEvents{},
	})
	s.Require().NoError(err)
	s.mgr = mgr
}

// startWith puts the given world behind a session named "sess".
func (s *ReadTestSuite) startWith(world *encounter.EncounterData) {
	_, err := s.mgr.StartSession(context.Background(), &session.StartSessionInput{
		Session: "sess", Encounter: "world", World: world,
	})
	s.Require().NoError(err)
}

// hexWorld is a two-region hex field with a door, an occluder and a wall —
// rich enough that a projection dropping any one field is visible.
func hexWorld(t fataler) *encounter.EncounterData {
	enc, err := encounter.NewEncounter(&encounter.SetupInput{Striker: encounter.RefusingStriker{}, Announcer: encQuietAnnouncer{}, Sight: encEveryoneSees{}, Initiative: encOrderAsGiven{}, TurnDriver: encPassDriver{},
		Standing: encEveryoneStanding{},
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
		Members: []encounter.MemberInput{
			{ID: "alice", Kind: encounter.KindPlayer, Position: spatial.Position{X: 0, Y: 0}},
		},
		Endings: []encounter.EndingInput{
			{Key: "out", Trigger: encounter.TriggerExternal{}},
		},
	})
	if err != nil {
		t.Fatalf("building hex world: %v", err)
	}
	data := enc.ToData()
	return &data
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
		_, err = s.mgr.Story(ctx, nil)
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
		_, err = s.mgr.Story(ctx, &session.StoryInput{Session: "sess"})
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

// TestAtlasProjectsTheWholeWorld is the projection's positive control.
//
// A translation that silently dropped a field would satisfy any "did it return
// something" assertion, so every field carried across is asserted — including
// the ones an incomplete projection is most likely to forget: occluders,
// boundaries and the doorway pair.
func (s *ReadTestSuite) TestAtlasProjectsTheWholeWorld() {
	s.startWith(hexWorld(s.T()))

	atlas, err := s.mgr.Atlas(context.Background(), &session.AtlasInput{Session: "sess", Member: "alice"})
	s.Require().NoError(err)

	s.Equal(session.GridHex, atlas.Grid, "the grid family must survive as the wire enum")
	s.Len(atlas.Cells, 72, "both 6x6 regions, every cell, once each")
	s.Require().Len(atlas.Props, 1, "props are not optional decoration")
	s.True(atlas.Props[0].BlocksLineOfSight, "and it must still say it blocks sight")
	s.NotEmpty(atlas.Props[0].Ref, "and name what it is — the whole point of rpg-toolkit#1130")
	s.Require().Len(atlas.Boundaries, 10, "every seam wall must survive the projection")
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
	s.startWith(hexWorld(s.T()))

	status, err := s.mgr.Status(context.Background(), &session.StatusInput{Session: "sess"})
	s.Require().NoError(err)
	s.True(status.Open)
	s.Nil(status.Outcome, "an open encounter has no outcome yet")
}

// TestViewReturnsProjectedSightings pins that perception survives the boundary.
func (s *ReadTestSuite) TestViewReturnsProjectedSightings() {
	s.startWith(hexWorld(s.T()))

	sightings, err := s.mgr.View(context.Background(),
		&session.ViewInput{Session: "sess", Member: "alice"})
	s.Require().NoError(err)
	s.NotNil(sightings, "a member with nothing in view gets an empty result, not an error")
}

// TestViewRejectsUnknownMember pins the sentinel translation for members.
func (s *ReadTestSuite) TestViewRejectsUnknownMember() {
	s.startWith(hexWorld(s.T()))

	_, err := s.mgr.View(context.Background(),
		&session.ViewInput{Session: "sess", Member: "nobody"})
	s.Require().Error(err)
	s.ErrorIs(err, session.ErrNoMember)
}

// TestStoryOmitsTheAudienceRoster is an information-leak pin, not a shape test.
//
// A record entry carries the full roster of viewers a beat was addressed to.
// Handing that back would tell Alice which other members exist and were
// present — including members she has never perceived and rooms she has never
// entered. The projection drops it deliberately, and this test fails if a
// future "carry everything across" refactor puts it back.
//
// It is written against the type rather than a value so it fails at compile
// time if the field is reintroduced, which is the earliest possible moment.
func (s *ReadTestSuite) TestStoryOmitsTheAudienceRoster() {
	s.startWith(hexWorld(s.T()))

	entries, err := s.mgr.Story(context.Background(),
		&session.StoryInput{Session: "sess", Member: "alice", FromSeq: 0})
	s.Require().NoError(err)
	s.Require().NotEmpty(entries, "setup records a scene-opened beat")

	// If Event ever grows an Audience field, this assertion's premise is
	// gone and the reflection below reports it.
	for _, field := range structFields(entries[0]) {
		s.NotEqual("Audience", field,
			"the audience roster is a delivery rule, not story content: exposing it "+
				"tells a viewer which members exist and were present")
	}
}

// TestStoryFromSeqIsInclusive pins the semantics our field name promises,
// independent of the composition's misnamed one.
func (s *ReadTestSuite) TestStoryFromSeqIsInclusive() {
	s.startWith(hexWorld(s.T()))
	ctx := context.Background()

	all, err := s.mgr.Story(ctx, &session.StoryInput{Session: "sess", Member: "alice"})
	s.Require().NoError(err)
	s.Require().NotEmpty(all)

	first := all[0].Seq
	from, err := s.mgr.Story(ctx,
		&session.StoryInput{Session: "sess", Member: "alice", FromSeq: first})
	s.Require().NoError(err)
	s.Require().NotEmpty(from)
	s.Equal(first, from[0].Seq, "FromSeq includes the entry it names")
}

// TestTrimmedStoryUsesOurSentinelNotTheirs is the pin for the leak channel the
// boundary test cannot see.
//
// The AST test reads exported signatures, and a sentinel error is not a type in
// a signature. If the composition's ErrTrimmed were reachable through our
// returned error, hosts would match on it, and replacing the composition would
// break their error handling exactly as surely as leaking a struct would —
// silently, with CI green throughout.
func (s *ReadTestSuite) TestTrimmedStoryUsesOurSentinelNotTheirs() {
	ctx := context.Background()
	world := trimmedWorld(s.T())
	s.startWith(world)

	// Under per-recipient numbering a cursorless session seeds from the
	// retained window and FromSeq 1 is answerable — the trimmed refusal
	// needs a cursor remembering beats the window no longer holds
	// (stream.go; the SentinelSuite's own trimmed scene says the same).
	data, err := s.sessions.GetSession(ctx, "sess")
	s.Require().NoError(err)
	data.Streams = map[string]session.StreamCursor{
		"alice": {UpTo: world.Log.NextSeq - 1, Count: uint64(len(world.Log.Entries)) + 5},
	}
	s.Require().NoError(s.sessions.SaveSession(ctx, data))

	_, err = s.mgr.Story(ctx,
		&session.StoryInput{Session: "sess", Member: "alice", FromSeq: 1})
	s.Require().Error(err)
	s.ErrorIs(err, session.ErrStoryTrimmed, "the caller sees our vocabulary")
	s.False(errors.Is(err, encounter.ErrTrimmed),
		"and must NOT be able to reach the composition's sentinel — matching on it "+
			"would couple every host to a module we intend to replace")
}

// trimmedWorld builds a world whose story log has already aged past its
// retention window, so a resume from sequence 1 can no longer be honoured.
func trimmedWorld(t fataler) *encounter.EncounterData {
	enc, err := encounter.NewEncounter(&encounter.SetupInput{Striker: encounter.RefusingStriker{}, Announcer: encQuietAnnouncer{}, Sight: encEveryoneSees{}, Initiative: encOrderAsGiven{}, TurnDriver: encPassDriver{},
		Standing: encEveryoneStanding{},
		Field:    encounter.FieldInput{Canvas: pointyCanvas(), Regions: []encounter.RegionInput{rectRegion("hall", 0, 0, 5, 5)}},
		Members: []encounter.MemberInput{
			{ID: "alice", Kind: encounter.KindPlayer, Position: spatial.Position{X: 1, Y: 1}},
		},
		Endings: []encounter.EndingInput{
			{Key: "out", Trigger: encounter.TriggerExternal{}},
		},
		Retention: 2,
	})
	if err != nil {
		t.Fatalf("building trimmed world: %v", err)
	}
	for i := 0; i < 10; i++ {
		to := spatial.Position{X: 2, Y: 1}
		if i%2 == 1 {
			to = spatial.Position{X: 1, Y: 1}
		}
		if _, err := enc.Step(&encounter.StepInput{Member: "alice", To: to}); err != nil {
			t.Fatalf("generating beats: %v", err)
		}
	}
	data := enc.ToData()
	return &data
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
	s.startWith(hexWorld(s.T()))
	atlas, err := s.mgr.Atlas(context.Background(), &session.AtlasInput{Session: "sess", Member: "alice"})
	s.Require().NoError(err)
	s.Equal(session.HexLayoutPointyTop, atlas.Layout,
		"hexWorld is authored pointy-top, and after rpg-toolkit#1141 that IS the way to draw it")
}

// TestAtlasLayoutCoversBothHexLayouts guards the mapping in both directions:
// a projection hard-coded to pointy would pass every fixture in this file.
func (s *ReadTestSuite) TestAtlasLayoutCoversBothHexLayouts() {
	flat, err := encounter.NewEncounter(&encounter.SetupInput{Striker: encounter.RefusingStriker{}, Announcer: encQuietAnnouncer{}, Sight: encEveryoneSees{}, Initiative: encOrderAsGiven{}, TurnDriver: encPassDriver{},
		Standing: encEveryoneStanding{},
		Field: encounter.FieldInput{Canvas: encounter.CanvasInput{Void: encounter.VoidIsOpaque(), Orientation: encounter.HexesAreFlatTop()},
			Regions: []encounter.RegionInput{rectRegion("cell", 0, 0, 4, 4)},
		},
		Members: []encounter.MemberInput{
			{ID: "alice", Kind: encounter.KindPlayer, Position: spatial.Position{X: 0, Y: 0}},
		},
		Endings: []encounter.EndingInput{{Key: "out", Trigger: encounter.TriggerExternal{}}},
	})
	s.Require().NoError(err)
	data := flat.ToData()
	s.startWith(&data)

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
	for i := range sightings {
		if sightings[i].Subject == "skeleton" {
			skeleton = &sightings[i]
		}
	}
	s.Require().NotNil(skeleton, "alice must currently see the skeleton aFight spawned adjacent to her")
	s.NotEmpty(skeleton.Name, "a spawned monster's catalog display name projects onto Sighting.Name")
	s.Require().NotNil(skeleton.Seen, "a live sight-channel sighting carries Seen")
	s.Equal(session.StandingUp, skeleton.Seen.Standing, "nothing has touched it yet")
}

// twoObservedWorld is one open 6x6 hall with no occluders and unbounded
// sight: "scout" watches "ally" (a fellow player) and "goblin" (a monster),
// both mutually visible from the moment the session starts — no walk needed
// to open sight, unlike seen_test.go's doorway scenes, because this is about
// what Kind reports once a sighting exists, not about how one opens.
func twoObservedWorld(t fataler) *encounter.EncounterData {
	enc, err := encounter.NewEncounter(&encounter.SetupInput{Striker: encounter.RefusingStriker{}, Announcer: encQuietAnnouncer{}, Sight: encEveryoneSees{},
		Initiative: encOrderAsGiven{}, TurnDriver: encPassDriver{}, Standing: encEveryoneStanding{},
		Field: encounter.FieldInput{Canvas: pointyCanvas(), Regions: []encounter.RegionInput{rectRegion("hall", 0, 0, 6, 6)}},
		Members: []encounter.MemberInput{
			{ID: "scout", Kind: encounter.KindPlayer, Position: spatial.Position{X: 0, Y: 0}},
			{ID: "ally", Kind: encounter.KindPlayer, Position: spatial.Position{X: 1, Y: 0}},
			{ID: "goblin", Kind: encounter.KindMonster, Position: spatial.Position{X: 2, Y: 0}},
		},
		Endings:   []encounter.EndingInput{{Key: "out", Trigger: encounter.TriggerExternal{}}},
		Retention: encounter.RetentionUnbounded,
	})
	if err != nil {
		t.Fatalf("building twoObservedWorld: %v", err)
	}
	data := enc.ToData()
	return &data
}

// TestViewCarriesKind pins rpg-toolkit#1230: a Sighting reports what kind of
// member the subject is, looked up from the roster the same way Name already
// is, so a client can tell a player from a monster without guessing (the
// diagnosis behind rpg-dnd5e-web#792). One session, one player and one
// monster besides the observer, one View call — both kinds must come back
// right, not just whichever one a hard-coded default would fake.
func (s *ReadTestSuite) TestViewCarriesKind() {
	s.startWith(twoObservedWorld(s.T()))

	sightings, err := s.mgr.View(context.Background(),
		&session.ViewInput{Session: "sess", Member: "scout"})
	s.Require().NoError(err)

	var ally, goblin *session.Sighting
	for i := range sightings {
		switch sightings[i].Subject {
		case "ally":
			ally = &sightings[i]
		case "goblin":
			goblin = &sightings[i]
		}
	}
	s.Require().NotNil(ally, "scout must see ally in the open hall")
	s.Require().NotNil(goblin, "scout must see goblin in the open hall")
	s.Equal(session.KindPlayer, ally.Kind, "a fellow player's Sighting must report Kind player")
	s.Equal(session.KindMonster, goblin.Kind, "a monster's Sighting must report Kind monster")
}
