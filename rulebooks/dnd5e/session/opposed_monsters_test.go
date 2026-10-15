// Copyright (C) 2026 Kirk Diggler
// SPDX-License-Identifier: GPL-3.0-or-later

package session_test

// opposed_monsters_test.go is rpg-toolkit#2054, the session's half of the mad
// king D0 (rpg-toolkit#2052): a fight that holds two hostile monster factions
// alongside a player. The encounter suites mock the striker; what is pinned
// HERE is that a driven monster's swing at an OPPOSED MONSTER goes through the
// session's real strikerSeam — the cast built over the whole roster — and
// lands on that monster, with the player in the same fight untouched.

import (
	"context"
	"testing"

	"github.com/stretchr/testify/suite"

	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/encounter"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/encounter/dungeonspec"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/refs"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/session"
	"github.com/KirkDiggler/rpg-toolkit/tools/spatial"
)

const (
	omZombie   = "zombie"
	omSkeleton = "skeleton"
	omWolves   = "wolves"
	omRaiders  = "raiders"
)

// OpposedMonstersSuite is one hall, one player, and two monster factions at
// war with each other and, by the composition's default, with the party.
type OpposedMonstersSuite struct {
	suite.Suite

	mgr    *session.Manager
	stream *fakeStream
}

func TestOpposedMonstersSuite(t *testing.T) { suite.Run(t, new(OpposedMonstersSuite)) }

// strikesBySelf is a driver that swings each named monster at its named
// target with its first action, and passes everyone else.
type strikesBySelf map[string]string

func (d strikesBySelf) Act(view session.MonsterView) (session.TurnIntent, error) {
	target, ok := d[view.Self]
	if !ok || len(view.Actions) == 0 {
		return session.Pass{}, nil
	}
	return session.Attack{Target: target, Action: view.Actions[0].Ref}, nil
}

func (s *OpposedMonstersSuite) SetupTest() {
	s.stream = &fakeStream{}
	mgr, err := session.NewManager(&session.Config{Seats: newFakeSeats(), Explorations: newFakeExplorations(),
		PresentationIDs: testPresentationIDs{}, Dice: testDice{},
		TurnDriver: strikesBySelf{omZombie: omSkeleton},
		Sessions:   newFakeSessions(), Encounters: newFakeEncounters(),
		Characters: testCharacters(), Events: s.stream,
	})
	s.Require().NoError(err)
	s.mgr = mgr
}

// TestAMonsterStrikesAnOpposedMonsterThroughTheRealStriker walks the table
// round until the zombie has acted, then reads the story: one strike, by the
// zombie, on the skeleton. Nothing swung at alice, who shares the fight.
func (s *OpposedMonstersSuite) TestAMonsterStrikesAnOpposedMonsterThroughTheRealStriker() {
	ctx := context.Background()
	zombie := bwPlacement(omZombie, refs.Monsters.Zombie().String(), spatial.Position{X: 4, Y: 2}, omWolves)
	skeleton := bwPlacement(omSkeleton, refs.Monsters.Skeleton().String(), spatial.Position{X: 5, Y: 2}, omRaiders)
	formed := launchScene(s.T(), s.mgr, scene{
		Field: encounter.FieldInput{
			Canvas:   pointyCanvas(),
			Regions:  []encounter.RegionInput{rectRegion("hall", 0, 0, 8, 8)},
			Factions: []encounter.FactionInput{{ID: omWolves}, {ID: omRaiders}},
			Dispositions: []encounter.DispositionInput{{
				Between: [2]encounter.FactionID{omWolves, omRaiders}, Stance: encounter.StanceHostile,
			}},
		},
		Party:    []sceneSeat{seatAt("alice", 1, 2)},
		Monsters: []dungeonspec.MonsterPlacement{zombie, skeleton},
	})
	s.Require().Len(formed.Formed, 1, "precondition: one fight, anchored by alice")
	s.Require().ElementsMatch([]string{"alice", omZombie, omSkeleton}, formed.Formed[0].Order,
		"precondition: both factions are in the player's fight")

	// End alice's turn until the zombie's has been driven. The monsters'
	// turns never rest; each EndTurn runs the table to the next player.
	for range 3 {
		if len(eventsOfKind(s.stream.published, "alice", session.EventStruck))+
			len(eventsOfKind(s.stream.published, "alice", session.EventMissed)) > 0 {
			break
		}
		turn, err := s.mgr.Turn(ctx, &session.TurnInput{Session: "sess", Member: "alice"})
		s.Require().NoError(err)
		s.Require().Equal("alice", turn.Active, "the clock rests only on the player")
		_, err = s.mgr.EndTurn(ctx, &session.EndTurnInput{
			Session: "sess", Member: "alice", DeclarationID: currentEndTurnID(s.T(), s.mgr, "sess", "alice"),
		})
		s.Require().NoError(err)
	}

	struck := eventsOfKind(s.stream.published, "alice", session.EventStruck)
	s.Require().Len(struck, 1, "alice watched one strike land")
	body, ok := struck[0].Body.(session.StruckBody)
	s.Require().True(ok)
	s.Equal(omZombie, body.Attacker)
	s.Equal(omSkeleton, body.Target, "the swing landed on the opposed monster")
	s.Positive(body.Damage)
	s.Empty(eventsOfKind(s.stream.published, "alice", session.EventMissed), "and nothing else was swung")
	for _, event := range s.stream.published {
		if b, isStruck := event.Body.(session.StruckBody); isStruck {
			s.NotEqual("alice", b.Target, "the player is untouched")
		}
	}
}
