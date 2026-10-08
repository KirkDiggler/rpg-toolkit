// Copyright (C) 2026 Kirk Diggler
// SPDX-License-Identifier: GPL-3.0-or-later

package session_test

import (
	"context"
	"encoding/json"
	"testing"

	"github.com/stretchr/testify/suite"

	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/encounter"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/monster"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/refs"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/session"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/weapons"
	"github.com/KirkDiggler/rpg-toolkit/tools/spatial"
)

// These cases exercise stored sheets, real perception and fresh Manager loads;
// the equipment capability and observation pipeline are not mocked.
type MonsterWeaponObservationSuite struct {
	suite.Suite
	mgr        *session.Manager
	sessions   *fakeSessions
	encounters *fakeEncounters
}

func TestMonsterWeaponObservationSuite(t *testing.T) {
	suite.Run(t, new(MonsterWeaponObservationSuite))
}

func (s *MonsterWeaponObservationSuite) SetupTest() {
	s.sessions, s.encounters = newFakeSessions(), newFakeEncounters()
	s.mgr = s.manager()
	_, err := s.mgr.StartSession(context.Background(), &session.StartSessionInput{
		Session: "sess", Encounter: "world", World: groundedSkeletonWorld(s.T()),
	})
	s.Require().NoError(err)
}

func (s *MonsterWeaponObservationSuite) SetupSubTest() { s.SetupTest() }

func (s *MonsterWeaponObservationSuite) manager() *session.Manager {
	mgr, err := session.NewManager(&session.Config{Seats: newFakeSeats(),
		PresentationIDs: testPresentationIDs{}, Dice: testDice{}, TurnDriver: session.Pass{},
		Sessions: s.sessions, Encounters: s.encounters,
		Characters: newFakeCharacters(armedFighter("fighter")), Events: session.DiscardEvents{},
	})
	s.Require().NoError(err)
	return mgr
}

func (s *MonsterWeaponObservationSuite) spawnAndSee(ref string, actions []string) *session.Sighting {
	ctx := context.Background()
	out, err := s.mgr.Spawn(ctx, &session.SpawnInput{
		Session: "sess", ID: "skeleton-1", Ref: ref, Actions: actions,
		Position: hexCell(9, 3),
	})
	s.Require().NoError(err)
	s.Nil(out.Formed, "the wall must block the monster before the walk")
	crossed, err := s.mgr.Move(ctx, &session.MoveInput{
		Session: "sess", Member: "fighter",
		Path: []spatial.Position{hexCell(5, 1), hexCell(5, 2), hexCell(6, 2)},
	})
	s.Require().NoError(err)
	s.NotNil(crossed.Formed, "a real first sighting forms the fight")
	discovery, ok := crossed.Discovered["fighter"]
	s.Require().True(ok)
	var report *session.Report
	for i := range discovery.FirstContact {
		if discovery.FirstContact[i].Subject == "skeleton-1" {
			report = &discovery.FirstContact[i]
		}
	}
	s.Require().NotNil(report, "first-contact output must carry the actual new testimony")
	s.Require().NotNil(report.Seen)

	// A fresh manager proves the answer is persisted testimony, not a runtime
	// monster or an equipment cache retained by the spawning manager.
	s.mgr = s.manager()
	view, err := s.mgr.View(ctx, &session.ViewInput{Session: "sess", Member: "fighter"})
	s.Require().NoError(err)
	sighting := findSighting(view, "skeleton-1")
	s.Require().NotNil(sighting)
	s.Require().NotNil(sighting.Seen)
	s.NotEmpty(sighting.CurrentVia)
	s.Equal(report.Seen.Equipment, sighting.Seen.Equipment)
	return sighting
}

func (s *MonsterWeaponObservationSuite) TestVisibleSpawnCarriesEquipmentInArrivalDiscoveryAndReload() {
	ctx := context.Background()
	out, err := s.mgr.Spawn(ctx, &session.SpawnInput{
		Session: "sess", ID: "boss", Ref: refs.Monsters.GoblinBoss().String(),
		Position: hexCell(4, 0),
	})
	s.Require().NoError(err)
	discovery, ok := out.Discovered["fighter"]
	s.Require().True(ok)
	var report *session.Report
	for i := range discovery.FirstContact {
		if discovery.FirstContact[i].Subject == "boss" {
			report = &discovery.FirstContact[i]
		}
	}
	s.Require().NotNil(report, "Spawn itself must discover the visible arrival")
	s.Require().NotNil(report.Seen)
	s.Require().NotNil(report.Seen.Equipment)
	s.Equal("scimitar", report.Seen.Equipment.MainHand)
	view, err := s.manager().View(ctx, &session.ViewInput{Session: "sess", Member: "fighter"})
	s.Require().NoError(err)
	seen := findSighting(view, "boss")
	s.Require().NotNil(seen)
	s.NotEmpty(seen.CurrentVia)
	s.Require().NotNil(seen.Seen)
	s.Equal(report.Seen.Equipment, seen.Seen.Equipment)
}

func (s *MonsterWeaponObservationSuite) TestAuthoredWeaponOverrideIsObservedInAuthorOrder() {
	for _, tc := range []struct {
		name    string
		ref     string
		actions []string
		want    string
	}{
		{"goblin-bow-only", refs.Monsters.Goblin().String(), []string{refs.Weapons.Shortbow().String()}, "shortbow"},
		{"goblin-sword-first", refs.Monsters.Goblin().String(), []string{refs.Weapons.Scimitar().String(), refs.Weapons.Shortbow().String()}, "scimitar"},
		{"goblin-bow-first", refs.Monsters.Goblin().String(), []string{refs.Weapons.Shortbow().String(), refs.Weapons.Scimitar().String()}, "shortbow"},
		{"skeleton-sword", refs.Monsters.Skeleton().String(), []string{refs.Weapons.Shortsword().String()}, "shortsword"},
		{"default-skeleton", refs.Monsters.Skeleton().String(), nil, "shortsword"},
		{"default-goblin-boss", refs.Monsters.GoblinBoss().String(), nil, "scimitar"},
	} {
		s.Run(tc.name, func() {
			sighting := s.spawnAndSee(tc.ref, tc.actions)
			s.Require().NotNil(sighting.Seen.Equipment)
			s.Equal(tc.want, sighting.Seen.Equipment.MainHand, "bare item ID, not an asset ref")
			s.Empty(sighting.Seen.Equipment.OffHand)
			testimony, ok := encounter.DecodeSightTestimony(sighting.Payload)
			s.Require().True(ok)
			s.Require().NotNil(testimony.Equipment)
			s.Equal(tc.want, testimony.Equipment.MainHand)
		})
	}
}

// This directly changes stored action order ONLY as a test probe of the
// observation seam. It adds no free weapon-change verb or equipped state.
func (s *MonsterWeaponObservationSuite) replaceStoredWeapon(id weapons.WeaponID) {
	ctx := context.Background()
	data, err := s.sessions.GetSession(ctx, "sess")
	s.Require().NoError(err)
	found := false
	for i := range data.NPCs {
		if data.NPCs[i].ID != "skeleton-1" {
			continue
		}
		m, loadErr := monster.Load(ctx, &data.NPCs[i])
		s.Require().NoError(loadErr)
		s.Require().NoError(m.SetWeapons([]weapons.WeaponID{id}))
		data.NPCs[i] = *m.ToData()
		found = true
	}
	s.Require().True(found)
	s.Require().NoError(s.sessions.SaveSession(ctx, data))
}

func (s *MonsterWeaponObservationSuite) TestCurrentReadKeepsSnapshotUntilARealRefresh() {
	seen := s.spawnAndSee(refs.Monsters.Skeleton().String(), nil)
	s.Require().Equal("shortsword", seen.Seen.Equipment.MainHand)
	s.replaceStoredWeapon(weapons.Shortbow)

	view, err := s.manager().View(context.Background(), &session.ViewInput{Session: "sess", Member: "fighter"})
	s.Require().NoError(err)
	s.Equal("shortsword", findSighting(view, "skeleton-1").Seen.Equipment.MainHand,
		"even a live read reports its stored testimony, not an unobserved sheet edit")
	_, err = s.mgr.Recheck(context.Background(), &session.RecheckInput{Session: "sess", Members: []string{"skeleton-1"}})
	s.Require().NoError(err)
	view, err = s.manager().View(context.Background(), &session.ViewInput{Session: "sess", Member: "fighter"})
	s.Require().NoError(err)
	s.Equal("shortbow", findSighting(view, "skeleton-1").Seen.Equipment.MainHand)
}

func (s *MonsterWeaponObservationSuite) TestRememberedWeaponDoesNotRevealOffscreenChangeEvenAfterRefresh() {
	s.sessions, s.encounters = groundedSkeletonScene(s.T())
	s.mgr = s.manager()
	s.replaceStoredWeapon(weapons.Shortbow)
	_, err := s.mgr.Recheck(context.Background(), &session.RecheckInput{Session: "sess", Members: []string{"skeleton-1"}})
	s.Require().NoError(err)
	view, err := s.manager().View(context.Background(), &session.ViewInput{Session: "sess", Member: "fighter"})
	s.Require().NoError(err)
	ghost := findSighting(view, "skeleton-1")
	s.Require().NotNil(ghost)
	s.Empty(ghost.CurrentVia, "the wall genuinely blocks the remembered monster")
	s.Require().NotNil(ghost.Seen)
	s.Require().NotNil(ghost.Seen.Equipment)
	s.Equal("shortsword", ghost.Seen.Equipment.MainHand,
		"rechecking a subject out of sight must not replace remembered equipment with live truth")
}

func (s *MonsterWeaponObservationSuite) TestNaturalActionMonsterDoesNotInventObservedEmptyHands() {
	seen := s.spawnAndSee(refs.Monsters.Wolf().String(), nil)
	s.Nil(seen.Seen.Equipment)
}

// TestFirstContactPayloadCarriesNoConditions: a visible arrival's first-contact
// report delivers what was seen, and conditions are not among it — they are
// testimony for rules (rpg-project#520, R16), never delivered.
func (s *MonsterWeaponObservationSuite) TestFirstContactPayloadCarriesNoConditions() {
	out, err := s.mgr.Spawn(context.Background(), &session.SpawnInput{
		Session: "sess", ID: "boss", Ref: refs.Monsters.GoblinBoss().String(),
		Position: hexCell(4, 0),
	})
	s.Require().NoError(err)
	reports := 0
	for watcher, discovery := range out.Discovered {
		for _, report := range discovery.FirstContact {
			var payload map[string]json.RawMessage
			s.Require().NoError(json.Unmarshal(report.Payload, &payload))
			s.NotContains(payload, "conditions", "%s's first contact with %s", watcher, report.Subject)
			reports++
		}
	}
	s.Positive(reports, "precondition: the arrival was first contact for somebody")
}
