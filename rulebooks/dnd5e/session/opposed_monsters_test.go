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

	mgr *session.Manager
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
	mgr, err := session.NewManager(&session.Config{Record: session.DiscardRecord{}, Acts: session.DiscardActs{}, Seats: newFakeSeats(), Explorations: newFakeExplorations(),
		PresentationIDs: testPresentationIDs{}, Dice: testDice{},
		TurnDriver: strikesBySelf{omZombie: omSkeleton},
		Sessions:   newFakeSessions(), Encounters: newFakeEncounters(),
		Characters: testCharacters(),
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
		if len(s.swingsAliceSaw()) > 0 {
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

	swings := s.swingsAliceSaw()
	s.Require().Equal([]string{string(encounter.OutcomeStruck)}, beatNames(swings),
		"alice watched one strike land, and nothing else was swung")
	var strike struct {
		Actor   string   `json:"actor"`
		Targets []string `json:"targets"`
		Amount  int      `json:"amount"`
	}
	swings[0].decode(s.T(), &strike)
	s.Equal(omZombie, strike.Actor)
	s.Equal([]string{omSkeleton}, strike.Targets, "the swing landed on the opposed monster")
	s.Positive(strike.Amount)
	for _, beat := range beatsNamed(storyBeats(s.T(), s.mgr, "sess", "alice"), string(encounter.OutcomeStruck)) {
		var b struct {
			Targets []string `json:"targets"`
		}
		beat.decode(s.T(), &b)
		s.NotContains(b.Targets, "alice", "the player is untouched")
	}
}

// swingsAliceSaw is every struck or missed beat in alice's story.
func (s *OpposedMonstersSuite) swingsAliceSaw() []witnessed {
	return beatsNamed(storyBeats(s.T(), s.mgr, "sess", "alice"),
		string(encounter.OutcomeStruck), string(encounter.OutcomeMissed))
}
