// Copyright (C) 2026 Kirk Diggler
// SPDX-License-Identifier: GPL-3.0-or-later

package session_test

// spawnactions_test.go is the driving case of rpg-project#448, at this seam:
// place two goblin archers — one with a scimitar as backup, one with nothing
// but the bow — without touching Go.
//
// Everything below is chosen so it cannot pass on a value the caller supplied.
// The caller passes weapon REFS. What is asserted is +4 to hit for 1d6+2
// piercing at 80/320 feet, which exists only because the shortbow was
// assembled against a goblin's DEX 14 and its CR-based +2.

import (
	"context"
	"testing"

	"github.com/stretchr/testify/suite"

	combatActions "github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/combat/actions"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/damage"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/refs"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/session"
	"github.com/KirkDiggler/rpg-toolkit/tools/spatial"
)

type SpawnActionsTestSuite struct {
	suite.Suite

	sessions *fakeSessions
	mgr      *session.Manager
}

func TestSpawnActionsSuite(t *testing.T) { suite.Run(t, new(SpawnActionsTestSuite)) }

func (s *SpawnActionsTestSuite) SetupTest() {
	s.sessions = newFakeSessions()
	mgr, err := session.NewManager(&session.Config{
		PresentationIDs: testPresentationIDs{}, Dice: testDice{}, TurnDriver: session.Pass{},
		Sessions: s.sessions, Encounters: newFakeEncounters(), Characters: testCharacters(),
		Events: session.DiscardEvents{},
	})
	s.Require().NoError(err)
	s.mgr = mgr

	_, err = mgr.StartSession(context.Background(), &session.StartSessionInput{
		Session: "sess", Encounter: "world", World: hexWorld(s.T()),
	})
	s.Require().NoError(err)
}

func (s *SpawnActionsTestSuite) SetupSubTest() { s.SetupTest() }

// armedGoblin spawns a goblin with the author's action list and returns the
// actions on its STORED sheet — the sheet a rehydrated run reads, not a
// projection built for this call.
func (s *SpawnActionsTestSuite) armedGoblin(id string, at float64, actions []string) []combatActions.Definition {
	s.T().Helper()
	_, err := s.mgr.Spawn(context.Background(), &session.SpawnInput{
		Session: "sess", ID: id, Ref: refs.Monsters.Goblin().String(),
		Position: spatial.Position{X: at, Y: 0}, Actions: actions,
	})
	s.Require().NoError(err)

	for _, npc := range s.sessions.byID["sess"].NPCs {
		if npc.ID == id {
			return npc.Actions
		}
	}
	s.Require().Failf("no sheet", "the spawn recorded no sheet for %q", id)
	return nil
}

// TestAnArcherWithNothingButTheBow is the second of the two goblins: the
// author armed it with one weapon, and one weapon is all it has.
func (s *SpawnActionsTestSuite) TestAnArcherWithNothingButTheBow() {
	actions := s.armedGoblin("coward", 0, []string{refs.Weapons.Shortbow().String()})

	s.Require().Len(actions, 1, "the author said bow, so the scimitar its stat block gives it is gone")
	bow := actions[0]
	s.Equal(refs.Weapons.Shortbow().String(), bow.Ref.String(),
		"the action's ref is the weapon's, as it is for a character")
	s.Equal("Shortbow", bow.Name)

	s.Require().NotNil(bow.Attack)
	s.Equal(4, bow.Attack.AttackBonus, "DEX 14 and the goblin's own +2, which the caller never passed")
	s.Require().NotNil(bow.Attack.Ability)
	s.Equal(2, bow.Attack.Ability.Modifier, "the +2 on 1d6+2")
	s.Require().Len(bow.Attack.Damage, 1)
	s.Equal("1d6", bow.Attack.Damage[0].Dice)
	s.Equal(damage.Piercing, bow.Attack.Damage[0].Type)
	s.Equal(&combatActions.RangedDelivery{NormalFeet: 80, LongFeet: 320}, bow.Attack.Delivery.Ranged)
	s.Nil(bow.Attack.Delivery.Melee, "there is no blade on this one")
}

// TestAnArcherWithABladeForWhenYouGetClose is the first goblin, and the
// ORDER is the whole of what makes it different.
func (s *SpawnActionsTestSuite) TestAnArcherWithABladeForWhenYouGetClose() {
	actions := s.armedGoblin("backup", 1, []string{
		refs.Weapons.Scimitar().String(), refs.Weapons.Shortbow().String(),
	})

	s.Require().Len(actions, 2)
	s.Equal(refs.Weapons.Scimitar().String(), actions[0].Ref.String(),
		"the author listed the blade first, and the driver takes the first action in reach")
	s.Equal(refs.Weapons.Shortbow().String(), actions[1].Ref.String())
	s.Equal(&combatActions.MeleeDelivery{ReachFeet: 5}, actions[0].Attack.Delivery.Melee)
}

// TestTheAuthorsOrderIsNotNormalised is the mutant that made the two tests
// above worth writing.
//
// Drop the ordering from the spawn — sort the list, deduplicate it, or forward
// it in any order but the one written — and the two goblins stop being
// different creatures: the one meant to swing when cornered shoots point blank
// instead. So the same two weapons are spawned the other way round here, and
// the bow has to be first.
func (s *SpawnActionsTestSuite) TestTheAuthorsOrderIsNotNormalised() {
	blade := s.armedGoblin("blade-first", 0, []string{
		refs.Weapons.Scimitar().String(), refs.Weapons.Shortbow().String(),
	})
	s.Equal(refs.Weapons.Scimitar().String(), blade[0].Ref.String())

	bow := s.armedGoblin("bow-first", 1, []string{
		refs.Weapons.Shortbow().String(), refs.Weapons.Scimitar().String(),
	})
	s.Equal(refs.Weapons.Shortbow().String(), bow[0].Ref.String(),
		"the same two weapons the other way round stay the other way round")
	s.Equal(refs.Weapons.Scimitar().String(), bow[1].Ref.String())
}

// TestAnUnarmedPlacementKeepsTheStatBlocksOwnArms is the negative that makes
// the tests above claims about Actions rather than about spawning.
func (s *SpawnActionsTestSuite) TestAnUnarmedPlacementKeepsTheStatBlocksOwnArms() {
	actions := s.armedGoblin("default", 0, nil)

	s.Require().Len(actions, 2, "a goblin's own scimitar and shortbow")
	s.Equal(refs.Weapons.Scimitar().String(), actions[0].Ref.String())
	s.Equal(refs.Weapons.Shortbow().String(), actions[1].Ref.String())
}

// TestASpawnRefusesAWeaponNothingCanBuild is decision 5 at this seam: it fails
// here, reading the file, not at a turn.
func (s *SpawnActionsTestSuite) TestASpawnRefusesAWeaponNothingCanBuild() {
	for _, tc := range []struct {
		name   string
		action string
		is     error
	}{
		{"a weapon the catalog does not have", "dnd5e:weapons:trebuchet", session.ErrUnknownContent},
		{"a ref that is not a weapon", "dnd5e:monster_actions:wolf-bite", session.ErrUnknownContent},
		// The row that makes the TYPE check load-bearing rather than
		// decorative. Deleting the module/type test leaves the two rows
		// above passing — nothing answers to "wolf-bite" in the weapons
		// catalog either — but `dnd5e:monster_actions:mace` has a weapon's
		// id in a namespace that is not the weapons catalog, and only the
		// type check refuses it. Found by running that mutant.
		{"an authored action whose id collides with a weapon's",
			"dnd5e:monster_actions:mace", session.ErrUnknownContent},
		{"another module's weapon", "homebrew:weapons:shortbow", session.ErrUnknownContent},
		{"a bare weapon id", "shortbow", session.ErrBadRef},
	} {
		s.Run(tc.name, func() {
			_, err := s.mgr.Spawn(context.Background(), &session.SpawnInput{
				Session: "sess", ID: "doomed", Ref: refs.Monsters.Goblin().String(),
				Position: spatial.Position{X: 0, Y: 0}, Actions: []string{tc.action},
			})
			s.Require().Error(err)
			s.ErrorIs(err, tc.is)
			s.ErrorContains(err, tc.action, "the refusal names the ref the author wrote")
			s.Empty(s.sessions.byID["sess"].NPCs, "a refused spawn stores nothing")
		})
	}
}

// TestABadWeaponLateInTheListStillRefusesTheWholeSpawn: the monster is armed
// all at once, so it cannot arrive holding the half of the list that parsed.
func (s *SpawnActionsTestSuite) TestABadWeaponLateInTheListStillRefusesTheWholeSpawn() {
	_, err := s.mgr.Spawn(context.Background(), &session.SpawnInput{
		Session: "sess", ID: "doomed", Ref: refs.Monsters.Goblin().String(),
		Position: spatial.Position{X: 0, Y: 0},
		Actions:  []string{refs.Weapons.Shortbow().String(), "dnd5e:weapons:trebuchet"},
	})
	s.Require().Error(err)
	s.ErrorIs(err, session.ErrUnknownContent)
	s.Empty(s.sessions.byID["sess"].NPCs)
}
