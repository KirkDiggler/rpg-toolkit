// Copyright (C) 2026 Kirk Diggler
// SPDX-License-Identifier: GPL-3.0-or-later

package session_test

// structural_session_test.go drives the session adapter for the promoted
// structural layout (rpg-project#169, P2S) through the REAL Manager and the
// REAL pushed provider — never a stub that invents a field the composition
// does not have.
//
// It is the seam's own half of a provider contract the composition already
// proves: the composition decides which walls, cuts and doors a recipient may
// know; this package copies them, puts them on the one Knowledge atlas and on
// the existing reveal beats, and does not re-evaluate a single visibility
// question. What a test here can therefore assert is agreement — the snapshot,
// the beat, and the replay tell one story — not world truth.
//
// The fixture is one dungeon with a concealed vault behind it. Hidden in that
// vault are a structural wall and a footprint door bound to one of its cuts,
// so a real Search teaches the searcher both the wall (with its once-withheld
// cut) and the independent door — the exact P2E payload shape, produced by the
// provider and read back here.

import (
	"context"
	"testing"

	"github.com/stretchr/testify/suite"

	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/character"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/encounter"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/session"
	"github.com/KirkDiggler/rpg-toolkit/tools/spatial"
)

const (
	structWallPresence = "vault-wall"
	structDoorPresence = "vault-leaf-presence"
	structDoorID       = "vault/leaf"
	structSecret       = "vault-secret-structural"
)

// structuralBox is a rectangle of the given size in feet on one cell's own
// centre — the same canonical frame the composition rasterises in, so a
// presence authored here is a presence the field agrees stands there.
func structuralBox(at spatial.Position, widthFeet, depthFeet float64) spatial.FootprintPlacement {
	return spatial.FootprintPlacement{
		Footprint: spatial.Footprint{Box: &spatial.Box{W: widthFeet, D: depthFeet}},
		Origin:    hallPlane().CellCentre(at),
	}
}

// structuralSecretWorld is concealedWorld with a structural wall and a bound
// footprint door hidden inside the vault.
//
// The wall's line runs across the vault's own canonical plane; the door's
// resolved endpoints sit inside the opening. Neither the wall's presence nor
// the door's presence is listed under the concealment's Props — they are
// withheld because they stand on concealed, unexplored floor and because the
// door id is named — and the whole vault's floor is the secret.
func structuralSecretWorld(t fataler) *encounter.EncounterData {
	wallBox := structuralBox(hexCell(7, 2), 6, 0.5)
	doorBox := structuralBox(hexCell(7, 3), 1, 1)
	origin := hallPlane().CellCentre(hexCell(7, 2))
	at := func(dx float64) spatial.Point { return spatial.Point{X: origin.X + dx, Y: origin.Y} }

	enc, err := encounter.NewEncounter(&encounter.SetupInput{
		Field: encounter.FieldInput{Canvas: pointyCanvas(),
			Regions: []encounter.RegionInput{
				rectRegion("hall", 0, 0, 6, 6),
				rectRegion("vault", 6, 0, 6, 6),
			},
			Walls: axialSeam(-1),
			Concealments: []encounter.ConcealmentInput{{
				ID: structSecret, Checks: vaultFind(),
				Cells: rectCells(6, 0, 6, 6),
				Doors: []encounter.DoorID{structDoorID},
			}},
			Doors: []encounter.DoorInput{{
				ID: structDoorID, Placement: &doorBox, State: encounter.DoorIsClosed(),
			}},
			Placed: []encounter.PlacedPropInput{
				{ID: structWallPresence, Placement: wallBox, BlocksMovement: true, BlocksLineOfSight: true},
				{ID: structDoorPresence, Placement: doorBox},
			},
			StructuralWalls: []encounter.StructuralWallInput{{
				ID: structWallPresence, Ref: "dnd5e:env:test:wall",
				From: at(-3), To: at(3), Height: 3, Thickness: 0.3, Elevation: -0.25,
				Openings: []encounter.StructuralOpeningInput{{
					ID: "gap", Position: 3, Width: 2,
					Door: &encounter.StructuralDoorBindingInput{
						PlacedID: structDoorPresence, DoorID: structDoorID,
						Ref:  "dnd5e:env:test:door",
						From: at(2), To: at(4),
					},
				}},
			}},
		},
		Members: []encounter.MemberInput{
			{ID: "alice", Kind: encounter.KindPlayer, Position: cell(1, 1)},
			{ID: "bob", Kind: encounter.KindPlayer, Position: cell(2, 1)},
		},
		Endings: []encounter.EndingInput{{Key: "out", Trigger: encounter.TriggerExternal{}}},
		Capabilities: encounter.Capabilities{
			Sight:         encEveryoneSees{},
			Equipment:     encNoHandsObserved{},
			Initiative:    encOrderAsGiven{},
			Driver:        encPassDriver{},
			Standing:      encEveryoneStanding{},
			CheckResolver: encNeverResolves{},
			Witness:       encNeverWitnesses{},
			Sheets:        encStandStill{},
			Actors: encounter.Actors{
				Striker:   encounter.RefusingStriker{},
				Mover:     encounter.RefusingMover{},
				Announcer: encQuietAnnouncer{},
			},
		},
	})
	if err != nil {
		t.Fatalf("building structural secret world: %v", err)
	}
	data := enc.ToData()

	return &data
}

// structuralRoomWorld is gatedWorld-style two-region dungeon with a structural
// wall and an independent footprint door standing in the sealed vault. Nothing
// is concealed: the vault is ordinary unexplored floor, so OPENING the gate is
// what teaches the layout — the room_revealed half of P2E, as opposed to the
// concealment half the suite above drives with Search.
func structuralRoomWorld(t fataler) *encounter.EncounterData {
	wallBox := structuralBox(hexCell(7, 2), 6, 0.5)
	doorBox := structuralBox(hexCell(7, 3), 1, 1)
	origin := hallPlane().CellCentre(hexCell(7, 2))
	at := func(dx float64) spatial.Point { return spatial.Point{X: origin.X + dx, Y: origin.Y} }

	enc, err := encounter.NewEncounter(&encounter.SetupInput{
		Field: encounter.FieldInput{Canvas: pointyCanvas(),
			Regions: []encounter.RegionInput{
				rectRegion("corridor", 0, 0, 6, 6),
				rectRegion("vault", 6, 0, 6, 6),
			},
			Walls: hexSeamWalls(6, 6, 0),
			Doors: []encounter.DoorInput{
				{ID: "gate", Edges: []encounter.DoorEdge{{From: hexCell(5, 0), To: hexCell(6, 0)}}, State: encounter.DoorIsClosed()},
				{ID: structDoorID, Placement: &doorBox, State: encounter.DoorIsClosed()},
			},
			Placed: []encounter.PlacedPropInput{
				{ID: structWallPresence, Placement: wallBox, BlocksMovement: true, BlocksLineOfSight: true},
				{ID: structDoorPresence, Placement: doorBox},
			},
			StructuralWalls: []encounter.StructuralWallInput{{
				ID: structWallPresence, Ref: "dnd5e:env:test:wall",
				From: at(-3), To: at(3), Height: 3, Thickness: 0.3, Elevation: 0,
				Openings: []encounter.StructuralOpeningInput{{
					ID: "gap", Position: 3, Width: 2,
					Door: &encounter.StructuralDoorBindingInput{
						PlacedID: structDoorPresence, DoorID: structDoorID,
						Ref:  "dnd5e:env:test:door",
						From: at(2), To: at(4),
					},
				}},
			}},
		},
		Members: []encounter.MemberInput{
			{ID: "alice", Kind: encounter.KindPlayer, Position: spatial.Position{X: 5, Y: 0}},
		},
		Endings: []encounter.EndingInput{{Key: "out", Trigger: encounter.TriggerExternal{}}},
		Capabilities: encounter.Capabilities{
			Sight:         encEveryoneSees{},
			Equipment:     encNoHandsObserved{},
			Initiative:    encOrderAsGiven{},
			Driver:        encPassDriver{},
			Standing:      encEveryoneStanding{},
			CheckResolver: encNeverResolves{},
			Witness:       encNeverWitnesses{},
			Sheets:        encStandStill{},
			Actors: encounter.Actors{
				Striker:   encounter.RefusingStriker{},
				Mover:     encounter.RefusingMover{},
				Announcer: encQuietAnnouncer{},
			},
		},
	})
	if err != nil {
		t.Fatalf("building structural room world: %v", err)
	}
	data := enc.ToData()

	return &data
}

type StructuralSessionSuite struct {
	suite.Suite

	stream *fakeStream
	mgr    *session.Manager
}

func TestStructuralSessionSuite(t *testing.T) { suite.Run(t, new(StructuralSessionSuite)) }

func (s *StructuralSessionSuite) startWith(world *encounter.EncounterData, cast ...*character.Data) {
	s.stream = &fakeStream{}
	mgr, err := session.NewManager(&session.Config{Seats: newFakeSeats(), PresentationIDs: testPresentationIDs{},
		Dice: testDice{}, TurnDriver: session.Pass{},
		Sessions: newFakeSessions(), Encounters: newFakeEncounters(),
		Characters: newFakeCharacters(cast...), Events: s.stream,
	})
	s.Require().NoError(err)
	s.mgr = mgr

	_, err = s.mgr.StartSession(context.Background(), &session.StartSessionInput{
		Session: "sess", Encounter: "world", World: world,
	})
	s.Require().NoError(err)
}

// TestKnowledgeCarriesThePermittedStructuralLayout is the headline: a real
// Search teaches the searcher the provider-scoped wall (with its cut) and the
// independent door, and Manager.Knowledge answers all of it on the ONE atlas.
//
// BEFORE THE FIND the vault's structural layout is withheld whole, and AFTER
// it the snapshot agrees with the concealment_revealed beat byte for byte — the
// snapshot-or-event parity the promotion is for. bob, who did not search,
// learns nothing.
func (s *StructuralSessionSuite) TestKnowledgeCarriesThePermittedStructuralLayout() {
	ctx := context.Background()
	s.startWith(structuralSecretWorld(s.T()), sharpEyed("alice"), dullEyed("bob"))
	input := &session.KnowledgeInput{Session: "sess", Member: "alice", Player: "player-alice"}

	before, err := s.mgr.Knowledge(ctx, input)
	s.Require().NoError(err)
	s.Empty(before.Atlas.StructuralWalls, "the vault's wall is withheld whole before the find")
	s.Empty(before.Atlas.StructuralDoors, "and so is the door it was withholding")

	s.stream.published = nil
	_, err = s.mgr.Search(ctx, &session.SearchInput{Session: "sess", Member: "alice", Region: "hall"})
	s.Require().NoError(err)

	after, err := s.mgr.Knowledge(ctx, input)
	s.Require().NoError(err)

	s.Require().Len(after.Atlas.StructuralWalls, 1, "the wall, on the one Knowledge atlas")
	wall := after.Atlas.StructuralWalls[0]
	s.Equal(structWallPresence, wall.ID, "the raw static presence id, verbatim")
	s.Equal("dnd5e:env:test:wall", wall.Ref, "opaque, unread")
	s.Equal(3.0, wall.Height)
	s.Equal(0.3, wall.Thickness)
	s.Equal(-0.25, wall.Elevation, "a negative elevation is an authored fact, not a default")
	s.Require().Len(wall.Openings, 1, "and its once-withheld cut")
	s.Equal("gap", wall.Openings[0].ID)
	s.Equal(3.0, wall.Openings[0].Position)
	s.Equal(2.0, wall.Openings[0].Width)

	s.Require().Len(after.Atlas.StructuralDoors, 1, "the independent door its own identity permitted")
	door := after.Atlas.StructuralDoors[0]
	s.Equal(structDoorID, door.ID, "the actual canonical gameplay door id, no client reconstruction")
	s.Equal("dnd5e:env:test:door", door.Ref)
	s.Equal(3.0, door.Height)

	// SNAPSHOT EQUALS EVENT. The beat the provider wrote carries the same
	// rows, so a client applying the reveal and a client refetching the atlas
	// hold one picture.
	reveals := eventsOfKind(s.stream.published, "alice", session.EventConcealmentRevealed)
	s.Require().Len(reveals, 1, "one concealment_revealed for the finder")
	body, ok := reveals[0].Body.(session.ConcealmentRevealedBody)
	s.Require().True(ok, "a reveal event carries its typed body")
	s.Equal(after.Atlas.StructuralWalls, body.StructuralWalls, "the wall row the beat carried")
	s.Equal(after.Atlas.StructuralDoors, body.StructuralDoors, "and the door row")

	// The one atlas rule: no second top-level structural collection beside the
	// atlas, and no structural row smuggled onto the mutable observation list.
	s.Empty(after.View.Doors, "structural doors are fixed layout, not door state observations")

	// REPLAY IS THE SAME STORY. Manager.Story decodes the stored payload and
	// must equal what the live stream delivered — the dense-numbering and
	// payload-fidelity half.
	replay, err := s.mgr.Story(ctx, &session.StoryInput{Session: "sess", Member: "alice", FromSeq: before.Seq + 1})
	s.Require().NoError(err)
	s.Equal(eventsFor(s.stream.published, "alice"), replay,
		"live and replay project the same stored structural payload")

	// bob never searched: he is taught neither floor nor layout.
	bob, err := s.mgr.Knowledge(ctx, &session.KnowledgeInput{Session: "sess", Member: "bob", Player: "player-bob"})
	s.Require().NoError(err)
	s.Empty(bob.Atlas.StructuralWalls, "the secret's wall stays absent from a non-finder's atlas")
	s.Empty(bob.Atlas.StructuralDoors)
}

// TestOpenDoorRoomRevealCarriesStructuralRows is the room_revealed half: a
// real OpenDoor into a previously unexplored chamber teaches the opener the
// chamber's structural wall and independent door, on the same beat hook the
// concealment reveal uses.
//
// The beat and the refreshed snapshot carry the SAME rows, and the stored
// payload replays identically — so a client that applies the room reveal and
// one that refetches agree, exactly as P2S requires.
func (s *StructuralSessionSuite) TestOpenDoorRoomRevealCarriesStructuralRows() {
	ctx := context.Background()
	s.startWith(structuralRoomWorld(s.T()), sharpEyed("alice"))
	input := &session.KnowledgeInput{Session: "sess", Member: "alice", Player: "player-alice"}

	before, err := s.mgr.Knowledge(ctx, input)
	s.Require().NoError(err)
	s.Empty(before.Atlas.StructuralWalls, "the sealed vault's wall is not known yet")
	s.Empty(before.Atlas.StructuralDoors)

	s.stream.published = nil
	_, err = s.mgr.OpenDoor(ctx, &session.OpenDoorInput{Session: "sess", Member: "alice", Door: "gate"})
	s.Require().NoError(err)

	after, err := s.mgr.Knowledge(ctx, input)
	s.Require().NoError(err)
	s.Require().Len(after.Atlas.StructuralWalls, 1, "the room's wall arrives with the room")
	s.Equal(structWallPresence, after.Atlas.StructuralWalls[0].ID)
	s.Require().Len(after.Atlas.StructuralWalls[0].Openings, 1, "with its permitted cut")
	s.Require().Len(after.Atlas.StructuralDoors, 1, "and the independent door")
	s.Equal(structDoorID, after.Atlas.StructuralDoors[0].ID)

	reveals := eventsOfKind(s.stream.published, "alice", session.EventRoomRevealed)
	s.Require().Len(reveals, 1, "one room_revealed for the opener")
	body, ok := reveals[0].Body.(session.RoomRevealedBody)
	s.Require().True(ok, "a room reveal carries its typed body")
	s.Equal("vault", body.Region.ID)
	s.Equal(after.Atlas.StructuralWalls, body.StructuralWalls, "the beat carries the same wall row as the snapshot")
	s.Equal(after.Atlas.StructuralDoors, body.StructuralDoors, "and the same door row")

	replay, err := s.mgr.Story(ctx, &session.StoryInput{Session: "sess", Member: "alice", FromSeq: before.Seq + 1})
	s.Require().NoError(err)
	s.Equal(eventsFor(s.stream.published, "alice"), replay, "live and replay project the same stored room payload")
}

// TestKnowledgeStructuralLayoutSurvivesARepositoryReload pins that the fixed
// layout is repository truth, not a value held only in the verb that produced
// it: the same world loaded through a SECOND Knowledge read (a fresh load from
// the host repository) answers identically.
func (s *StructuralSessionSuite) TestKnowledgeStructuralLayoutSurvivesARepositoryReload() {
	ctx := context.Background()
	s.startWith(structuralSecretWorld(s.T()), sharpEyed("alice"), dullEyed("bob"))

	_, err := s.mgr.Search(ctx, &session.SearchInput{Session: "sess", Member: "alice", Region: "hall"})
	s.Require().NoError(err)

	first, err := s.mgr.Knowledge(ctx, &session.KnowledgeInput{Session: "sess", Member: "alice", Player: "player-alice"})
	s.Require().NoError(err)
	second, err := s.mgr.Knowledge(ctx, &session.KnowledgeInput{Session: "sess", Member: "alice", Player: "player-alice"})
	s.Require().NoError(err)

	s.Equal(first.Atlas.StructuralWalls, second.Atlas.StructuralWalls, "a reload answers the same walls")
	s.Equal(first.Atlas.StructuralDoors, second.Atlas.StructuralDoors, "and the same doors")
	s.NotEmpty(second.Atlas.StructuralWalls, "and it really did carry them, rather than both being empty")
}

// TestAnUnownedSeatCannotReadTheStructuralLayout is the boundary check on the
// read: a caller naming another player's character is refused exactly as it
// was before structural rows existed. The layout rides an authenticated
// snapshot, so it inherits the seat check and never becomes a side channel.
func (s *StructuralSessionSuite) TestAnUnownedSeatCannotReadTheStructuralLayout() {
	ctx := context.Background()
	s.startWith(structuralSecretWorld(s.T()), sharpEyed("alice"), dullEyed("bob"))

	denied, err := s.mgr.Knowledge(ctx, &session.KnowledgeInput{Session: "sess", Member: "alice", Player: "player-bob"})
	s.Require().ErrorIs(err, session.ErrNotSeated)
	s.Nil(denied, "no atlas is returned to a seat that does not own the member")
}
