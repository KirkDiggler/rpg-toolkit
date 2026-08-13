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
	mgr, err := session.NewManager(&session.Config{
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

// hexWorld is a two-room hex field with a doorway, occluders and a wall — rich
// enough that a projection dropping any one field is visible.
func hexWorld(t fataler) *encounter.EncounterData {
	enc, err := encounter.NewEncounter(&encounter.SetupInput{
		Field: encounter.FieldInput{
			Rooms: []encounter.RoomInput{
				{
					ID: "corridor", Width: 6, Height: 6, Grid: spatial.GridShapeHex,
					Origin:    spatial.Position{X: 0, Y: 0},
					Occluders: []spatial.Position{{X: 1, Y: 1}},
					Boundaries: []spatial.Boundary{{
						From: spatial.Position{X: -2, Y: -2}, To: spatial.Position{X: -2, Y: -1},
						BlocksMovement: true, BlocksLineOfSight: true,
					}},
				},
				{
					ID: "vault", Width: 6, Height: 6, Grid: spatial.GridShapeHex,
					Origin: spatial.Position{X: 6, Y: 0},
				},
			},
			Connections: []encounter.ConnectionInput{{
				ID: "gate", From: "corridor", To: "vault",
				FromPosition: spatial.Position{X: 2, Y: 0},
				ToPosition:   spatial.Position{X: -3, Y: 0},
			}},
		},
		Members: []encounter.MemberInput{
			{ID: "alice", Kind: encounter.KindPlayer, Room: "corridor", Position: spatial.Position{X: 0, Y: 0}},
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
		_, err := s.mgr.Atlas(ctx, &session.AtlasInput{})
		s.ErrorIs(err, session.ErrNoSessionID)
		_, err = s.mgr.Status(ctx, &session.StatusInput{})
		s.ErrorIs(err, session.ErrNoSessionID)
	})

	s.Run("empty member id", func() {
		_, err := s.mgr.View(ctx, &session.ViewInput{Session: "sess"})
		s.ErrorIs(err, session.ErrNoMemberID)
		_, err = s.mgr.Story(ctx, &session.StoryInput{Session: "sess"})
		s.ErrorIs(err, session.ErrNoMemberID)
	})

	s.Run("unknown session", func() {
		_, err := s.mgr.Atlas(ctx, &session.AtlasInput{Session: "nope"})
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

	_, err := s.mgr.Atlas(ctx, &session.AtlasInput{Session: "orphan"})
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

	atlas, err := s.mgr.Atlas(context.Background(), &session.AtlasInput{Session: "sess"})
	s.Require().NoError(err)
	s.Require().Len(atlas.Rooms, 2)
	s.Require().Len(atlas.Doorways, 1)

	corridor := atlas.Rooms[0]
	s.Equal("corridor", corridor.ID)
	s.Equal(session.GridHex, corridor.Grid, "the grid family must survive as the wire enum")
	s.Equal(6, corridor.Width)
	s.Equal(6, corridor.Height)
	s.NotEmpty(corridor.Cells, "absolute cells must be carried across")
	s.Require().Len(corridor.Occluders, 1, "occluders are not optional decoration")
	s.Require().Len(corridor.Boundaries, 1, "walls must survive the projection")
	s.True(corridor.Boundaries[0].BlocksLineOfSight)
	s.True(corridor.Boundaries[0].BlocksMovement)

	gate := atlas.Doorways[0]
	s.Equal("gate", gate.Connection)
	s.Equal("corridor", gate.From)
	s.Equal("vault", gate.To)
	s.NotEqual(gate.FromCell, gate.ToCell, "the kissing pair is two distinct absolute cells")
}

// TestGridProjectionCoversBothFamilies guards the enum mapping in both
// directions. A projection hard-coded to one family would pass every hex test
// in this file and quietly mislabel every square room in the game.
func (s *ReadTestSuite) TestGridProjectionCoversBothFamilies() {
	s.startWith(hexWorld(s.T()))
	atlas, err := s.mgr.Atlas(context.Background(), &session.AtlasInput{Session: "sess"})
	s.Require().NoError(err)
	s.Equal(session.GridHex, atlas.Rooms[0].Grid)

	square := newFakeSessions()
	squareEnc := newFakeEncounters()
	mgr, err := session.NewManager(&session.Config{Sessions: square, Encounters: squareEnc, Characters: testCharacters(),
		Events: session.DiscardEvents{},
	})
	s.Require().NoError(err)
	_, err = mgr.StartSession(context.Background(), &session.StartSessionInput{
		Session: "sq", Encounter: "sq-world", World: authoredWorld(s.T()),
	})
	s.Require().NoError(err)

	atlas, err = mgr.Atlas(context.Background(), &session.AtlasInput{Session: "sq"})
	s.Require().NoError(err)
	s.Equal(session.GridSquare, atlas.Rooms[0].Grid,
		"a square room must not be reported as hex")
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

	// If StoryEntry ever grows an Audience field, this assertion's premise is
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
	s.startWith(trimmedWorld(s.T()))

	_, err := s.mgr.Story(context.Background(),
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
	enc, err := encounter.NewEncounter(&encounter.SetupInput{
		Field: encounter.FieldInput{Rooms: []encounter.RoomInput{{ID: "hall", Width: 5, Height: 5}}},
		Members: []encounter.MemberInput{
			{ID: "alice", Kind: encounter.KindPlayer, Room: "hall", Position: spatial.Position{X: 1, Y: 1}},
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
		if _, err := enc.Move(&encounter.MoveInput{Member: "alice", To: to}); err != nil {
			t.Fatalf("generating beats: %v", err)
		}
	}
	data := enc.ToData()
	return &data
}

func TestReadSuite(t *testing.T) {
	suite.Run(t, new(ReadTestSuite))
}
