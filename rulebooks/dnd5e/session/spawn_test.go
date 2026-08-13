// Copyright (C) 2026 Kirk Diggler
// SPDX-License-Identifier: GPL-3.0-or-later

package session_test

import (
	"context"
	"encoding/json"
	"testing"

	"github.com/stretchr/testify/suite"

	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/refs"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/session"
	"github.com/KirkDiggler/rpg-toolkit/tools/spatial"
)

// Spawn: content that lives in code, entering the session by ref.
//
// The seam's claim is that a host names WHAT to build and never builds it.
// These tests are chosen so they cannot pass on a caller-supplied value: the
// hit points, armour class and speed asserted below are never passed in, and
// come only from running the catalog constructor. That is the same discipline
// the character tests use with derived Speed — assert something the caller
// could not have echoed.
type SpawnTestSuite struct {
	suite.Suite

	sessions   *fakeSessions
	encounters *fakeEncounters
	characters *fakeCharacters
	mgr        *session.Manager
}

func TestSpawnSuite(t *testing.T) { suite.Run(t, new(SpawnTestSuite)) }

func (s *SpawnTestSuite) SetupTest() {
	s.sessions = newFakeSessions()
	s.encounters = newFakeEncounters()
	s.characters = testCharacters()
	mgr, err := session.NewManager(&session.Config{
		Sessions: s.sessions, Encounters: s.encounters, Characters: s.characters,
		Events: session.DiscardEvents{},
	})
	s.Require().NoError(err)
	s.mgr = mgr

	_, err = mgr.StartSession(context.Background(), &session.StartSessionInput{
		Session: "sess", Encounter: "world", World: hexWorld(s.T()),
	})
	s.Require().NoError(err)
}

func (s *SpawnTestSuite) SetupSubTest() { s.SetupTest() }

func (s *SpawnTestSuite) spawn(id, ref string, at spatial.Position) (*session.SpawnOutput, error) {
	return s.mgr.Spawn(context.Background(), &session.SpawnInput{
		Session: "sess", ID: id, Ref: ref, Room: "vault", Position: at,
	})
}

// TestSpawnBuildsTheCatalogEntry is the headline, and nothing asserted here was
// supplied by the caller.
//
// The caller passed an ID, a ref, a room and a position. Thirteen hit points,
// armour class 13 and a 30-foot walk exist only because NewSkeleton ran.
func (s *SpawnTestSuite) TestSpawnBuildsTheCatalogEntry() {
	out, err := s.spawn("skel-1", refs.Monsters.Skeleton().String(), spatial.Position{X: 0, Y: 0})
	s.Require().NoError(err)

	s.Require().NotNil(out.NPC)
	s.Equal("skel-1", out.NPC.ID, "the member's identity is the caller's, not the catalog's")
	s.Equal(refs.Monsters.Skeleton().String(), out.NPC.Ref, "and it reports what it was built from")
	s.Equal("Skeleton", out.NPC.Name)
	s.Equal(13, out.NPC.HitPoints)
	s.Equal(13, out.NPC.MaxHitPoints)
	s.Equal(13, out.NPC.ArmorClass)
	s.Equal(30, out.NPC.Speed)
}

// TestDifferentRefsBuildDifferentMonsters is the discriminating control for the
// assertion above.
//
// A constant would satisfy the skeleton's numbers. These three differ in both
// directions — a zombie has MORE hit points and LESS armour than a skeleton, a
// giant rat less of both — so no single defaulted value can pass this table.
func (s *SpawnTestSuite) TestDifferentRefsBuildDifferentMonsters() {
	for _, tc := range []struct {
		name    string
		ref     string
		monster string
		hp, ac  int
	}{
		{"skeleton", refs.Monsters.Skeleton().String(), "Skeleton", 13, 13},
		{"zombie", refs.Monsters.Zombie().String(), "Zombie", 22, 8},
		{"giant rat", refs.Monsters.GiantRat().String(), "Giant Rat", 7, 12},
	} {
		s.Run(tc.name, func() {
			out, err := s.spawn("npc-1", tc.ref, spatial.Position{X: 0, Y: 0})
			s.Require().NoError(err)
			s.Equal(tc.monster, out.NPC.Name)
			s.Equal(tc.hp, out.NPC.HitPoints)
			s.Equal(tc.ac, out.NPC.ArmorClass)
		})
	}
}

// TestOneRefMakesManyMembers is why ID and Ref are separate fields.
//
// A template carries no identity. If the ref supplied the member ID, a second
// skeleton would collide with the first, and an encounter could hold exactly
// one of each kind of monster.
func (s *SpawnTestSuite) TestOneRefMakesManyMembers() {
	skeleton := refs.Monsters.Skeleton().String()

	first, err := s.spawn("skel-1", skeleton, spatial.Position{X: 0, Y: 0})
	s.Require().NoError(err)
	second, err := s.spawn("skel-2", skeleton, spatial.Position{X: 1, Y: 0})
	s.Require().NoError(err)

	s.Equal("skel-1", first.NPC.ID)
	s.Equal("skel-2", second.NPC.ID)
	s.Equal(first.NPC.Ref, second.NPC.Ref, "built from the same catalog entry")
	s.Len(s.storedNPCs(), 2, "and both are remembered separately")
}

// TestABadRefIsRejectedAtSpawn is the rejection table.
//
// Four distinct sentinels rather than one, because the remedies genuinely
// differ: fix the call, ship a build that has that content, or pick a monster
// that exists. Collapsing them would send whoever debugs it to the wrong place.
func (s *SpawnTestSuite) TestABadRefIsRejectedAtSpawn() {
	for _, tc := range []struct {
		name string
		ref  string
		want error
	}{
		{"empty", "", session.ErrNoRef},
		{"not a ref at all", "skeleton", session.ErrBadRef},
		{"module we cannot load", "homebrew:monsters:mind-flayer", session.ErrNoLoader},
		{"type that is not content", refs.Conditions.Raging().String(), session.ErrNoLoader},
		{"canonical but unconstructed", refs.Monsters.SkeletonArcher().String(), session.ErrUnknownContent},
		{"no such monster", "dnd5e:monsters:tarrasque", session.ErrUnknownContent},
	} {
		s.Run(tc.name, func() {
			_, err := s.spawn("npc-1", tc.ref, spatial.Position{X: 0, Y: 0})
			s.Require().Error(err)
			s.ErrorIs(err, tc.want)
			s.Empty(s.storedNPCs(), "a rejected spawn stores nothing")
		})
	}
}

// TestTheSpawnedSheetSurvivesAProcessRestart pins that an NPC is remembered as
// DATA rather than as a ref to re-run.
//
// The distinction is not academic. The stored skeleton here has been wounded,
// which is a state the catalog constructor cannot produce — so a load that
// rebuilt from the ref would silently heal it, and the test would catch a
// full-health skeleton on the far side.
func (s *SpawnTestSuite) TestTheSpawnedSheetSurvivesAProcessRestart() {
	_, err := s.spawn("skel-1", refs.Monsters.Skeleton().String(), spatial.Position{X: 0, Y: 0})
	s.Require().NoError(err)

	// Wound it in the store, the way a later wave's damage would.
	stored := s.sessions.byID["sess"]
	s.Require().Len(stored.NPCs, 1)
	stored.NPCs[0].HitPoints = 4

	raw, err := json.Marshal(stored)
	s.Require().NoError(err)
	var reloaded session.SessionData
	s.Require().NoError(json.Unmarshal(raw, &reloaded))

	s.Require().Len(reloaded.NPCs, 1, "the NPC outlived the process")
	s.Equal("skel-1", reloaded.NPCs[0].ID)
	s.Equal(4, reloaded.NPCs[0].HitPoints, "the wounded instance, not a fresh skeleton")
	s.Equal(13, reloaded.NPCs[0].MaxHitPoints, "and it still knows what it was")
	s.Require().NotNil(reloaded.NPCs[0].Ref)
	s.Equal(refs.Monsters.Skeleton().String(), reloaded.NPCs[0].Ref.String())
}

// TestBothEntryVerbsEnforceTheSamePlacementRules is the shared-path pin.
//
// Join and Spawn differ in where a sheet comes from and in nothing else. If
// they held separate placement code, a rule added to one would silently not
// apply to the other — so this asserts a placement rule through BOTH doors and
// fails if either stops enforcing it.
func (s *SpawnTestSuite) TestBothEntryVerbsEnforceTheSamePlacementRules() {
	nowhere := "no-such-room"

	_, joinErr := s.mgr.Join(context.Background(), &session.JoinInput{
		Session: "sess", Member: "bob",
		Room: nowhere, Position: spatial.Position{X: 0, Y: 0},
	})
	_, spawnErr := s.mgr.Spawn(context.Background(), &session.SpawnInput{
		Session: "sess", ID: "skel-1", Ref: refs.Monsters.Skeleton().String(),
		Room: nowhere, Position: spatial.Position{X: 0, Y: 0},
	})

	s.Require().Error(joinErr, "join must reject a room that does not exist")
	s.Require().Error(spawnErr, "and spawn must reject it identically")
	s.Empty(s.storedNPCs(), "the rejected spawn stored nothing")
}

// TestSpawnIsRefusedWhileTheWorldIsFrozen pins that the new verb obeys the
// interrupt spine like every other change verb.
//
// A verb added later that quietly bypasses the freeze is the failure this
// guards: the world is waiting on an answer, and something walks a new monster
// into it.
func (s *SpawnTestSuite) TestSpawnIsRefusedWhileTheWorldIsFrozen() {
	sessions, encounters := newFakeSessions(), newFakeEncounters()
	mgr, err := session.NewManager(&session.Config{
		Sessions: sessions, Encounters: encounters,
		Characters: testCharacters(), Events: session.DiscardEvents{},
	})
	s.Require().NoError(err)
	_, err = mgr.StartSession(context.Background(), &session.StartSessionInput{
		Session: "sess", Encounter: "world", World: ambushWorld(s.T()),
	})
	s.Require().NoError(err)

	out, err := mgr.Move(context.Background(), &session.MoveInput{
		Session: "sess", Member: "alice",
		Path: []spatial.Position{{X: 2, Y: 2}, {X: 2, Y: 3}, {X: 2, Y: 4}},
	})
	s.Require().NoError(err)
	s.Require().NotNil(out.Pending, "the walk must suspend for this test to mean anything")

	_, err = mgr.Spawn(context.Background(), &session.SpawnInput{
		Session: "sess", ID: "skel-1", Ref: refs.Monsters.Skeleton().String(),
		Room: "hall", Position: spatial.Position{X: 6, Y: 6},
	})
	s.ErrorIs(err, session.ErrFrozen)
}

// storedNPCs reports what the session repository actually holds.
func (s *SpawnTestSuite) storedNPCs() []monsterRow {
	data, ok := s.sessions.byID["sess"]
	if !ok {
		return nil
	}
	rows := make([]monsterRow, 0, len(data.NPCs))
	for _, npc := range data.NPCs {
		rows = append(rows, monsterRow{ID: npc.ID, HitPoints: npc.HitPoints})
	}
	return rows
}

type monsterRow struct {
	ID        string
	HitPoints int
}
