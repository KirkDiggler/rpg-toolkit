// Copyright (C) 2026 Kirk Diggler
// SPDX-License-Identifier: GPL-3.0-or-later

package session

import (
	"context"
	"encoding/json"
	"testing"

	"github.com/stretchr/testify/suite"

	"github.com/KirkDiggler/rpg-toolkit/core"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/character"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/encounter"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/refs"
	"github.com/KirkDiggler/rpg-toolkit/tools/spatial"
)

// mover_internal_test.go is about the one fact [encounter.MoveStep] adds and
// this seam is the only thing that can act on: whether the creature CHOSE to
// move. A step somebody chose provokes; a step something imposed does not, and
// the suppression is named by its cause rather than switched on by a boolean
// nobody can trace back to a spell.
//
// INTERNAL BECAUSE THE SEAM IS. [encounter.Encounter.Direct] is the production
// caller and it lives a module down, so the only way to announce a forced step
// from here is to be here.

// ForcedStepSuite walks the fighter out of a skeleton's reach twice over.
//
// THE REACTOR IS A MONSTER ON PURPOSE. A friend does not swing at a friend
// walking away (TestAnAlliedReactorDoesNotSwing), so a scene of two players
// could not tell a suppressed opportunity attack from one that was never
// offered — and the control is the half of this pair that would quietly die if
// Forced were wired the wrong way round.
type ForcedStepSuite struct {
	suite.Suite

	sessions   *strikeSessions
	encounters *strikeEncounters
	characters *strikeCharacters
	mgr        *Manager

	fighterAt, skeletonAt, awayAt spatial.Position
}

func TestForcedStepSuite(t *testing.T) {
	suite.Run(t, new(ForcedStepSuite))
}

// thunderwaveCause stands in for whatever moved somebody. Its identity is not
// what these tests turn on; that something is NAMED is.
func thunderwaveCause() core.Ref { return *refs.Spells.Thunderwave() }

func (s *ForcedStepSuite) SetupTest() {
	ctx := context.Background()

	s.sessions = &strikeSessions{byID: map[string]*SessionData{}}
	s.encounters = &strikeEncounters{byID: map[string]*encounter.EncounterData{}}
	s.characters = &strikeCharacters{byID: map[string]*character.Data{
		"fighter": strikeFixtureFighter("fighter"),
	}}

	mgr, err := NewManager(&Config{
		PresentationIDs: testPresentationIDs{},
		// Faces for the one swing this scene can produce, and no more: a roll
		// nobody expected exhausts the roller and says so out loud.
		Dice: &scriptedDice{rolls: []int{19, 6, 6, 6}}, TurnDriver: Pass{},
		Sessions: s.sessions, Encounters: s.encounters,
		Characters: s.characters, Events: DiscardEvents{},
	})
	s.Require().NoError(err)
	s.mgr = mgr

	world, err := encounter.NewEncounter(&encounter.SetupInput{
		Striker: encounter.RefusingStriker{}, Mover: encounter.RefusingMover{},
		Announcer: encQuietAnnouncer{}, Sight: aggregateRecordEveryoneSees{},
		Equipment: encNoHandsObserved{}, Initiative: aggregateRecordOrderAsGiven{},
		TurnDriver: passDriver{}, Standing: aggregateRecordEveryoneStanding{},
		Field: encounter.FieldInput{
			Canvas:  pointyCanvas(),
			Regions: []encounter.RegionInput{rectRegion("tomb", 0, 0, 12, 6)},
		},
		Endings:   []encounter.EndingInput{{Key: "withdraw", Trigger: encounter.TriggerExternal{}}},
		Retention: encounter.RetentionUnbounded,
	})
	s.Require().NoError(err)
	data := world.ToData()

	_, err = mgr.StartSession(ctx, &StartSessionInput{Session: "sess", Encounter: "world", World: &data})
	s.Require().NoError(err)

	s.fighterAt = encounter.HexCellAt(encounter.HexesArePointyTop(), 2, 0)
	s.skeletonAt = encounter.HexCellAt(encounter.HexesArePointyTop(), 3, 0)
	s.awayAt = encounter.HexCellAt(encounter.HexesArePointyTop(), 1, 0)

	_, err = mgr.Join(ctx, &JoinInput{Session: "sess", Member: "fighter", Position: s.fighterAt})
	s.Require().NoError(err)

	// THE ECONOMY IS LOAD-BEARING, and written after Join because Join writes
	// the joined member's sheet from a freshly loaded character. It is the
	// walker's here rather than the reactor's — the skeleton is metered by its
	// own once-per-turn flag and holds no purse.
	stored, err := s.characters.GetCharacter(ctx, "fighter")
	s.Require().NoError(err)
	stored.ActionEconomy = &character.ActionEconomyData{
		TurnNumber: 1, ActionsRemaining: 1, BonusActionsRemaining: 1,
		ReactionsRemaining: 1, MovementRemaining: 30,
	}
	s.Require().NoError(s.characters.SaveCharacter(ctx, stored))

	spawned, err := mgr.Spawn(ctx, &SpawnInput{
		Session: "sess", ID: "skel-1", Ref: refs.Monsters.Skeleton().String(),
		Position: s.skeletonAt,
	})
	s.Require().NoError(err)
	s.Require().NotNil(spawned.Formed, "adjacent and in sight starts the fight")
}

// takeStep announces one step of the fighter's out of the skeleton's reach
// through the real seam, commits, and reports every beat her story records as a
// reaction.
//
// READ OFF THE STORY, not off a return value, because the story is what a
// player actually sees: a reaction reaches a client through the beat's own
// payload and nothing else carries it.
func (s *ForcedStepSuite) takeStep(step encounter.MoveStep) []string {
	ctx := context.Background()
	scope, err := s.mgr.openForChange(ctx, "sess")
	s.Require().NoError(err)

	s.Require().NoError(moverSeam{m: s.mgr, scope: scope}.Move(ctx, scope.enc, step))
	_, _, err = s.mgr.commit(ctx, scope)
	s.Require().NoError(err)

	entries, err := s.mgr.Story(ctx, &StoryInput{Session: "sess", Member: "fighter"})
	s.Require().NoError(err)

	var reactions []string
	for _, entry := range entries {
		var body struct {
			Reaction *struct {
				Ref string `json:"ref"`
			} `json:"reaction"`
		}
		s.Require().NoError(json.Unmarshal(entry.Payload, &body))
		if body.Reaction != nil {
			reactions = append(reactions, body.Reaction.Ref)
		}
	}
	return reactions
}

// TestAStepSomebodyChoseStillProvokes is the control. The fighter walks out of
// the skeleton's reach of her own accord and the skeleton bites.
func (s *ForcedStepSuite) TestAStepSomebodyChoseStillProvokes() {
	s.Contains(
		s.takeStep(encounter.MoveStep{Mover: "fighter", From: s.fighterAt, To: s.awayAt}),
		refs.Conditions.OpportunityAttack().String(),
		"a walk out of reach is what an opportunity attack is for",
	)
}

// TestAStepSomethingImposedProvokesNothing. The fighter covers the same two
// cells and the only difference is that she did not choose to: nobody left
// anybody's reach, so nobody swings.
func (s *ForcedStepSuite) TestAStepSomethingImposedProvokesNothing() {
	s.Empty(
		s.takeStep(encounter.MoveStep{
			Mover: "fighter", From: s.fighterAt, To: s.awayAt,
			Cause: thunderwaveCause(), Forced: true,
		}),
		"nobody chose to leave anybody's reach",
	)
}
