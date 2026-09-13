// Copyright (C) 2026 Kirk Diggler
// SPDX-License-Identifier: GPL-3.0-or-later

package session_test

import (
	"context"
	"testing"

	"github.com/stretchr/testify/require"
	"github.com/stretchr/testify/suite"

	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/character"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/encounter"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/refs"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/session"
	"github.com/KirkDiggler/rpg-toolkit/tools/spatial"
)

// seen_test.go is #1157's end-to-end case: after a walker crosses a doorway,
// View reports the monster on the far side with Seen.Position equal to where
// that monster actually stands — the whole projection, exercised through the
// real SDK verbs rather than a synthetic intel.Holding.
//
// authoredTomb() (example_session_test.go) is this package's usual reference
// world, but it is one bare room: no doorway, no monster. Reusing it here
// would mean adding wall/doorway geometry to a fixture every other test in
// this file depends on staying simple, or duplicating the reference tomb's
// own wall math (canvas_test.go's tombField, in the encounter package) at
// this seam — a second hand-written copy of exactly the coordinate class of
// bug ADR-0040/#1140 was about. So this builds its own small two-room world,
// the same way onemap_test.go's offsetWorld and read_test.go's hexWorld do:
// a seam wall with a doorway-row gap (squareSeamWalls, mirroring
// hexSeamWalls in testprops_test.go and squareSeamWall in the encounter
// package's own tests), so sight is genuinely gated by the doorway rather
// than open across the whole shared edge.

type SeenTestSuite struct {
	suite.Suite

	sessions   *fakeSessions
	encounters *fakeEncounters
	mgr        *session.Manager
}

func TestSeenTestSuite(t *testing.T) {
	suite.Run(t, new(SeenTestSuite))
}

func (s *SeenTestSuite) SetupTest() {
	s.sessions = newFakeSessions()
	s.encounters = newFakeEncounters()
	mgr, err := session.NewManager(&session.Config{PresentationIDs: testPresentationIDs{}, Dice: testDice{}, TurnDriver: session.Pass{},
		Sessions: s.sessions, Encounters: s.encounters, Characters: testCharacters(),
		Events: session.DiscardEvents{},
	})
	s.Require().NoError(err)
	s.mgr = mgr
}

// skeletonBehindADoor is a two-region world: entrance at [0,0] and hall at
// [6,0], each 6x6, joined by one door on row 2 with a solid wall everywhere
// else along the shared edge. skeleton-1 stands well inside hall at authored
// [9,3], where nothing but the doorway can put it in sight.
func skeletonBehindADoor(t fataler) *encounter.EncounterData {
	enc, err := encounter.NewEncounter(&encounter.SetupInput{Striker: encounter.RefusingStriker{}, Mover: encounter.RefusingMover{}, Announcer: encQuietAnnouncer{},
		Sight: encEveryoneSees{}, Equipment: encNoHandsObserved{}, Initiative: encOrderAsGiven{}, TurnDriver: encPassDriver{}, Standing: encEveryoneStanding{},
		Field: encounter.FieldInput{
			Canvas: pointyCanvas(),
			Regions: []encounter.RegionInput{
				rectRegion("entrance", 0, 0, 6, 6),
				rectRegion("hall", 6, 0, 6, 6),
			},
			Walls: hexSeamWalls(6, 6, 2),
			Doors: []encounter.DoorInput{{
				ID:    "door1",
				Edges: []encounter.DoorEdge{{From: hexCell(5, 2), To: hexCell(6, 2)}},
				State: encounter.DoorIsOpen(),
			}},
		},
		Members: []encounter.MemberInput{
			{ID: "fighter", Kind: encounter.KindPlayer, Position: spatial.Position{X: 5, Y: 0}},
			{ID: "skeleton-1", Kind: encounter.KindMonster, Position: spatial.Position{X: 9, Y: 3}},
		},
		Endings: []encounter.EndingInput{{Key: "done", Trigger: encounter.TriggerExternal{}}},
	})
	if err != nil {
		t.Fatalf("building skeletonBehindADoor: %v", err)
	}
	data := enc.ToData()
	return &data
}

// TestSeenIsPopulatedAfterCrossingTheDoorway is #1157's headline case: the
// fighter cannot see skeleton-1 from behind the wall, walks through the one
// doorway, and View then reports skeleton-1 with a Seen.Position equal to
// where the skeleton actually stands — read independently via Where, not
// the local literal used to place it, so the assertion cannot pass by both
// sides sharing the same typo.
func (s *SeenTestSuite) TestSeenIsPopulatedAfterCrossingTheDoorway() {
	ctx := context.Background()
	_, err := s.mgr.StartSession(ctx, &session.StartSessionInput{
		Session: "sess", Encounter: "skeleton-behind-a-door", World: skeletonBehindADoor(s.T()),
	})
	s.Require().NoError(err)

	// Before: the wall genuinely blocks it. Asserted first so a fixture that
	// accidentally puts the skeleton in the open cannot make the "after"
	// assertion trivially true.
	before, err := s.mgr.View(ctx, &session.ViewInput{Session: "sess", Member: "fighter"})
	s.Require().NoError(err)
	for _, sight := range before {
		s.NotEqual("skeleton-1", sight.Subject, "the wall must actually block sight before the walk")
	}

	// The fighter walks down to the doorway row and through it. The walk may
	// stop short of the full requested path: reaching the doorway's own gap
	// cell [5,2] already opens sight to skeleton-1 across it, and a sighting
	// between a player and a monster starts a fight, which is news the walk
	// reports rather than something it walks through (session/doc.go — the
	// composition detects contact wherever sight changes and stops the walker
	// there). That is itself part of what this test proves: sight opened
	// exactly because the walk reached the doorway, not before.
	out, err := s.mgr.Move(ctx, &session.MoveInput{
		Session: "sess", Member: "fighter",
		Path: []spatial.Position{hexCell(5, 1), hexCell(5, 2), hexCell(6, 2)},
	})
	s.Require().NoError(err)
	s.Require().NotEmpty(out.Steps, "the fighter must have moved at all")
	s.Require().NotNil(out.Formed, "seeing the skeleton must have started the fight")

	where, err := s.mgr.Where(ctx, &session.WhereInput{Session: "sess", Member: "skeleton-1"})
	s.Require().NoError(err)

	after, err := s.mgr.View(ctx, &session.ViewInput{Session: "sess", Member: "fighter"})
	s.Require().NoError(err)

	var skeleton *session.Sighting
	for i := range after {
		if after[i].Subject == "skeleton-1" {
			skeleton = &after[i]
		}
	}
	s.Require().NotNil(skeleton, "the fighter must see the skeleton once through the doorway")
	s.Require().NotNil(skeleton.Seen, "a sight-channel sighting must carry Seen")
	s.Equal(where.Position, skeleton.Seen.Position,
		"Seen.Position must equal the skeleton's own reported placement, read independently via Where")

	// A skeleton has no character sheet and therefore no hands to observe, which
	// is a DIFFERENT claim from being seen empty-handed. Nil all the way through
	// the SDK is what keeps a client from drawing "we don't know" as "unarmed"
	// (rpg-toolkit#1615).
	s.Nil(skeleton.Seen.Equipment,
		"a monster has no hands to observe; it was not seen empty-handed")
}

// TestSeenEquipmentComesFromTheSnapshotNotTheSheet pins the property the whole
// design rests on: what a client is told a peer is holding is read out of the
// observer's own sight testimony, not resolved from the subject when somebody
// asks. That is what lets a memory keep the hands it last saw — and what makes
// a lie expressible at all, since a live read could only ever be true.
func (s *SeenTestSuite) TestSeenEquipmentComesFromTheSnapshotNotTheSheet() {
	ctx := context.Background()
	_, err := s.mgr.StartSession(ctx, &session.StartSessionInput{
		Session: "sess", Encounter: "skeleton-behind-a-door", World: skeletonBehindADoor(s.T()),
	})
	s.Require().NoError(err)

	_, err = s.mgr.Move(ctx, &session.MoveInput{
		Session: "sess", Member: "fighter",
		Path: []spatial.Position{hexCell(5, 1), hexCell(5, 2), hexCell(6, 2)},
	})
	s.Require().NoError(err)

	after, err := s.mgr.View(ctx, &session.ViewInput{Session: "sess", Member: "fighter"})
	s.Require().NoError(err)

	var skeleton *session.Sighting
	for i := range after {
		if after[i].Subject == "skeleton-1" {
			skeleton = &after[i]
		}
	}
	s.Require().NotNil(skeleton, "the fighter must see the skeleton once through the doorway")
	s.Require().NotNil(skeleton.Seen, "a sight-channel sighting must carry Seen")

	// The decoded payload is the observer's snapshot. Whatever Seen reports has
	// to agree with it, because that is where it came from — no second source.
	testimony, ok := encounter.DecodeSightTestimony(skeleton.Payload)
	s.Require().True(ok, "the composition must decode its own testimony")
	s.Equal(testimony.Equipment == nil, skeleton.Seen.Equipment == nil,
		"Seen.Equipment must mirror the snapshot's own claim, not a live read")
}

// TestDiscoveredAlsoCarriesSeen pins the other producer of Report: MoveOutput
// .Discovered's FirstContact list, which the walk above already populates the
// moment the skeleton first comes into view. Discovery is proportional (S6):
// this asserts against out.Discovered directly rather than repeating the walk.
func (s *SeenTestSuite) TestDiscoveredAlsoCarriesSeen() {
	ctx := context.Background()
	_, err := s.mgr.StartSession(ctx, &session.StartSessionInput{
		Session: "sess", Encounter: "skeleton-behind-a-door", World: skeletonBehindADoor(s.T()),
	})
	s.Require().NoError(err)

	out, err := s.mgr.Move(ctx, &session.MoveInput{
		Session: "sess", Member: "fighter",
		Path: []spatial.Position{hexCell(5, 1), hexCell(5, 2), hexCell(6, 2)},
	})
	s.Require().NoError(err)

	where, err := s.mgr.Where(ctx, &session.WhereInput{Session: "sess", Member: "skeleton-1"})
	s.Require().NoError(err)

	discovery, ok := out.Discovered["fighter"]
	s.Require().True(ok, "the fighter's own perception must have changed on this walk")

	var report *session.Report
	for i := range discovery.FirstContact {
		if discovery.FirstContact[i].Subject == "skeleton-1" {
			report = &discovery.FirstContact[i]
		}
	}
	s.Require().NotNil(report, "skeleton-1 must be first contact — the fighter never held it before")
	s.Require().NotNil(report.Seen)
	s.Equal(where.Position, report.Seen.Position)
}

// groundedSkeletonWorld is skeletonBehindADoor's own geometry with the
// skeleton left out: fighter alone, behind the same wall, with the same one
// doorway gap at row 2. Standing's own tests spawn the skeleton themselves —
// spawning it as a real member, not authoring it, is what gives it a sheet a
// direct hit-point edit can later floor (rpg-toolkit#1702, test cases 1 and
// 5): an authored member with no sheet always reads Conscious, and could
// never actually go down for this proof to mean anything.
func groundedSkeletonWorld(t fataler) *encounter.EncounterData {
	enc, err := encounter.NewEncounter(&encounter.SetupInput{Striker: encounter.RefusingStriker{}, Mover: encounter.RefusingMover{}, Announcer: encQuietAnnouncer{},
		Sight: encEveryoneSees{}, Equipment: encNoHandsObserved{}, Initiative: encOrderAsGiven{}, TurnDriver: encPassDriver{}, Standing: encEveryoneStanding{},
		Field: encounter.FieldInput{
			Canvas: pointyCanvas(),
			Regions: []encounter.RegionInput{
				rectRegion("entrance", 0, 0, 6, 6),
				rectRegion("hall", 6, 0, 6, 6),
			},
			Walls: hexSeamWalls(6, 6, 2),
			Doors: []encounter.DoorInput{{
				ID:    "door1",
				Edges: []encounter.DoorEdge{{From: hexCell(5, 2), To: hexCell(6, 2)}},
				State: encounter.DoorIsOpen(),
			}},
		},
		Members: []encounter.MemberInput{
			{ID: "fighter", Kind: encounter.KindPlayer, Position: spatial.Position{X: 5, Y: 0}},
		},
		Endings: []encounter.EndingInput{{Key: "done", Trigger: encounter.TriggerExternal{}}},
	})
	if err != nil {
		t.Fatalf("building groundedSkeletonWorld: %v", err)
	}
	data := enc.ToData()
	return &data
}

// groundedSkeletonScene drives the shared setup both Standing tests below
// need: fighter crosses the doorway to see a freshly spawned, healthy
// skeleton (a CURRENT sighting), then retreats one cell back through the
// gap to lose sight of it again (a GHOST holding) — the exact "observer
// loses sight" half of rpg-toolkit#1702's test case 1, common to case 5 too.
//
// It runs the moves through its OWN manager, bound to the returned stores,
// and hands the stores back rather than the manager itself: each caller
// builds its own Manager over them next, with whichever CharacterRepository
// its own case needs (a real one to keep mutating the scene, a panicking one
// to prove View never touches it).
func groundedSkeletonScene(t *testing.T) (*fakeSessions, *fakeEncounters) {
	t.Helper()
	ctx := context.Background()
	sessions, encounters := newFakeSessions(), newFakeEncounters()
	mgr, err := session.NewManager(&session.Config{PresentationIDs: testPresentationIDs{}, Dice: testDice{}, TurnDriver: session.Pass{},
		Sessions: sessions, Encounters: encounters, Characters: newFakeCharacters(armedFighter("fighter")),
		Events: session.DiscardEvents{},
	})
	require.NoError(t, err)

	_, err = mgr.StartSession(ctx, &session.StartSessionInput{
		Session: "sess", Encounter: "world", World: groundedSkeletonWorld(t),
	})
	require.NoError(t, err)

	spawned, err := mgr.Spawn(ctx, &session.SpawnInput{
		Session: "sess", ID: "skeleton-1", Ref: refs.Monsters.Skeleton().String(),
		Position: hexCell(9, 3),
	})
	require.NoError(t, err)
	require.Nil(t, spawned.Formed, "fighter cannot see into the hall from behind the wall yet")

	crossed, err := mgr.Move(ctx, &session.MoveInput{
		Session: "sess", Member: "fighter",
		Path: []spatial.Position{hexCell(5, 1), hexCell(5, 2), hexCell(6, 2)},
	})
	require.NoError(t, err)
	require.NotNil(t, crossed.Formed, "seeing the living skeleton through the gap must start the fight")

	current, err := mgr.View(ctx, &session.ViewInput{Session: "sess", Member: "fighter"})
	require.NoError(t, err)
	skeleton := findSighting(current, "skeleton-1")
	require.NotNil(t, skeleton, "fighter must currently see the skeleton after crossing")
	require.NotEmpty(t, skeleton.CurrentVia, "this must be a live sighting, not already a ghost")
	require.NotNil(t, skeleton.Seen)
	require.NotNil(t, skeleton.Seen.Standing, "a current sighting of a healthy skeleton observes standing")
	require.Equal(t, session.StandingUp, *skeleton.Seen.Standing)

	declID := currentMoveID(t, mgr, "sess", "fighter")
	_, err = mgr.Move(ctx, &session.MoveInput{
		Session: "sess", Member: "fighter", DeclarationID: declID,
		Path: []spatial.Position{hexCell(5, 1)},
	})
	require.NoError(t, err)

	ghosted, err := mgr.View(ctx, &session.ViewInput{Session: "sess", Member: "fighter"})
	require.NoError(t, err)
	skeleton = findSighting(ghosted, "skeleton-1")
	require.NotNil(t, skeleton, "fighter must still hold a memory of the skeleton after retreating")
	require.Empty(t, skeleton.CurrentVia, "stepping back off the gap cell must actually break sight")

	return sessions, encounters
}

func findSighting(sightings []session.Sighting, subject string) *session.Sighting {
	for i := range sightings {
		if sightings[i].Subject == subject {
			return &sightings[i]
		}
	}
	return nil
}

// TestGhostSeenStandingIsWhatItLastSaw is rpg-toolkit#1702's headline case,
// and the whole reason the chain from #1680 onward existed: a ghost's
// Seen.Standing must be what the observer last SAW, never what is true now.
//
// The skeleton fighter last saw was alive and standing. Once fighter has
// lost sight of it, the skeleton is floored to zero hit points OFF-SCREEN —
// fighter never re-observes this, no Recheck is called, nothing refreshes
// sight. If Seen.Standing still asked the roster live (the KNOWN DEFECT this
// issue closes), View would now report the skeleton StandingDowned: a
// standing change fighter never witnessed, asserted as fact. It must keep
// reporting exactly what fighter last saw instead.
func TestGhostSeenStandingIsWhatItLastSaw(t *testing.T) {
	ctx := context.Background()
	sessions, encounters := groundedSkeletonScene(t)

	mgr, err := session.NewManager(&session.Config{PresentationIDs: testPresentationIDs{}, Dice: testDice{}, TurnDriver: session.Pass{},
		Sessions: sessions, Encounters: encounters, Characters: newFakeCharacters(armedFighter("fighter")),
		Events: session.DiscardEvents{},
	})
	require.NoError(t, err)

	// The kill happens entirely in the persisted NPC record, off the story
	// fighter can read — the same shape floorNPC uses elsewhere to floor a
	// monster without a swing anyone witnessed.
	data, err := sessions.GetSession(ctx, "sess")
	require.NoError(t, err)
	found := false
	for i := range data.NPCs {
		if data.NPCs[i].ID == "skeleton-1" {
			data.NPCs[i].HitPoints = 0
			found = true
		}
	}
	require.True(t, found, "the scene must have spawned skeleton-1's own NPC sheet")
	require.NoError(t, sessions.SaveSession(ctx, data))

	after, err := mgr.View(ctx, &session.ViewInput{Session: "sess", Member: "fighter"})
	require.NoError(t, err)
	skeleton := findSighting(after, "skeleton-1")
	require.NotNil(t, skeleton, "the ghost holding must still be there")
	require.Empty(t, skeleton.CurrentVia, "still a ghost — nothing re-observed it")
	require.NotNil(t, skeleton.Seen)
	require.NotNil(t, skeleton.Seen.Standing)
	require.Equal(t, session.StandingUp, *skeleton.Seen.Standing,
		"THE BUG: the skeleton is truly downed now, but fighter's ghost holding never "+
			"witnessed it happen — reporting StandingDowned here would assert a standing "+
			"change nobody actually saw, exactly the defect rpg-toolkit#1702 closes")
}

// panickingCharacters is a CharacterRepository that panics if either method
// is ever called. Wiring it into a Manager and driving a real read through
// it is a stronger proof than a mock expectation: it fails the moment the
// call happens, wherever in the call graph it happens, rather than only when
// someone remembers to assert on a spy afterward.
type panickingCharacters struct{}

func (panickingCharacters) GetCharacter(context.Context, string) (*character.Data, error) {
	panic("GetCharacter must not be called: View no longer consults standing at all (rpg-toolkit#1702)")
}

func (panickingCharacters) SaveCharacter(context.Context, *character.Data) error {
	panic("SaveCharacter must not be called from a read")
}

// TestViewNeverConsultsStandingEvenWithACurrentAndAGhostSighting is
// rpg-toolkit#1702's test case 5 — the structural guard. It is not enough
// that View happens not to call the standing seam today; the sighting path
// must be UNABLE to, so a future change that reintroduces the call fails
// loudly here rather than only in behavior nobody happened to test.
//
// A Manager built over a CharacterRepository that panics on any call, reused
// against the exact scene case 1 built (a live sighting existed briefly, a
// ghost exists now), still produces the full projection — proving the
// sighting path never reaches the character store at all, not merely that
// it returned the right answer this time.
func TestViewNeverConsultsStandingEvenWithACurrentAndAGhostSighting(t *testing.T) {
	sessions, encounters := groundedSkeletonScene(t)

	mgr, err := session.NewManager(&session.Config{PresentationIDs: testPresentationIDs{}, Dice: testDice{}, TurnDriver: session.Pass{},
		Sessions: sessions, Encounters: encounters, Characters: panickingCharacters{},
		Events: session.DiscardEvents{},
	})
	require.NoError(t, err)

	sightings, err := mgr.View(context.Background(), &session.ViewInput{Session: "sess", Member: "fighter"})
	require.NoError(t, err, "a panicking character store must never be reached")
	skeleton := findSighting(sightings, "skeleton-1")
	require.NotNil(t, skeleton, "the ghost holding must still project — a full sighting, not an empty result")
	require.NotNil(t, skeleton.Seen)
	require.NotNil(t, skeleton.Seen.Standing, "Standing itself must still come from the testimony's own snapshot")
	require.Equal(t, session.StandingUp, *skeleton.Seen.Standing)
}
