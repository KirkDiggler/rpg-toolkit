// Copyright (C) 2026 Kirk Diggler
// SPDX-License-Identifier: GPL-3.0-or-later

package session_test

import (
	"context"
	"encoding/json"
	"testing"

	"github.com/stretchr/testify/suite"

	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/character"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/conditions"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/refs"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/session"
	"github.com/KirkDiggler/rpg-toolkit/tools/spatial"
)

// MoverSeamSuite is rung 2 of rpg-project#316: the session implements
// encounter.Mover, so an opportunity attack fires again — automatically, and on
// both sides of the fight.
type MoverSeamSuite struct {
	suite.Suite

	sessions   *fakeSessions
	encounters *fakeEncounters
	characters *fakeCharacters
}

func TestMoverSeamSuite(t *testing.T) {
	suite.Run(t, new(MoverSeamSuite))
}

func (s *MoverSeamSuite) SetupTest() {
	s.sessions = newFakeSessions()
	s.encounters = newFakeEncounters()
	s.characters = newFakeCharacters(armedFighter("fighter"), armedFighter("ally"))
}

// managerWith builds this suite's manager over one turn driver.
func (s *MoverSeamSuite) managerWith(driver session.TurnDriver) *session.Manager {
	mgr, err := session.NewManager(&session.Config{
		PresentationIDs: testPresentationIDs{}, Dice: testDice{}, TurnDriver: driver,
		Sessions: s.sessions, Encounters: s.encounters,
		Characters: s.characters, Events: session.DiscardEvents{},
	})
	s.Require().NoError(err)
	return mgr
}

// inCombat gives a stored sheet the action economy of somebody who is IN a
// fight, with reactions left in hand.
//
// AFTER Join, NEVER BEFORE, because Join writes the joined member's sheet from
// a freshly loaded character and an economy set beforehand does not survive it.
//
// THE ECONOMY IS LOAD-BEARING, not fixture decoration. A character with none is
// not in combat at all (character.InCombat) and the opportunity attack's own
// predicate asks CanReact before it publishes anything, so a reactor without
// one proves nothing about this seam. TestAReactorWithNoReactionLeftDoesNotSwing
// is the same fixture with the one number changed.
func (s *MoverSeamSuite) inCombat(id string, reactions int) {
	stored, err := s.characters.GetCharacter(context.Background(), id)
	s.Require().NoError(err)
	stored.ActionEconomy = &character.ActionEconomyData{
		TurnNumber: 1, ActionsRemaining: 1, BonusActionsRemaining: 1,
		ReactionsRemaining: reactions, MovementRemaining: 30,
	}
	s.Require().NoError(s.characters.SaveCharacter(context.Background(), stored))
}

// disengaging seats the Disengaging condition on a stored sheet — what Step of
// the Wind and the Disengage action both leave behind, written straight onto
// the sheet so this scene is about the walk rather than about how the condition
// got there.
func (s *MoverSeamSuite) disengaging(id string) {
	blob, err := conditions.NewDisengagingCondition(id).ToJSON()
	s.Require().NoError(err)

	stored, err := s.characters.GetCharacter(context.Background(), id)
	s.Require().NoError(err)
	stored.Conditions = append(stored.Conditions, blob)
	s.Require().NoError(s.characters.SaveCharacter(context.Background(), stored))
}

// retreatWalker walks one fixed path the first time it is asked and passes ever
// after — a monster that deliberately leaves a threatened square, which is the
// one thing session.Behavior() will never do on its own.
type retreatWalker struct {
	path []spatial.Position
	gone bool
}

func (r *retreatWalker) Act(session.MonsterView) (session.TurnIntent, error) {
	if r.gone {
		return session.Pass{}, nil
	}
	r.gone = true
	return session.Move{Path: r.path}, nil
}

// reactedBeat is one story entry reduced to what these tests assert about it.
type reactedBeat struct {
	Beat     string   `json:"beat"`
	Actor    string   `json:"actor"`
	Targets  []string `json:"targets"`
	Reaction *struct {
		Ref  string `json:"ref"`
		Name string `json:"name"`
	} `json:"reaction"`
}

// reactionBeats reads member's whole story and returns every beat that names a
// reaction.
//
// READ OFF THE STORY, not off a verb's return value, because the story is what
// a player actually sees: the reaction reaches a client through the beat's own
// payload and nothing else carries it.
func (s *MoverSeamSuite) reactionBeats(mgr *session.Manager, member string) []reactedBeat {
	entries, err := mgr.Story(context.Background(), &session.StoryInput{Session: "sess", Member: member})
	s.Require().NoError(err)

	var out []reactedBeat
	for _, entry := range entries {
		var body reactedBeat
		s.Require().NoError(json.Unmarshal(entry.Payload, &body))
		if body.Reaction == nil {
			continue
		}
		s.Require().Contains([]string{"struck", "missed"}, body.Beat,
			"a strike is the only thing recorded as a reaction today")
		out = append(out, body)
	}
	return out
}

// oaRef is the one reaction a step can provoke today.
func oaRef() string { return refs.Conditions.OpportunityAttack().String() }

// duel starts the standard scene: one fighter and one skeleton standing next to
// each other in an open tomb, in a fight, with the fighter holding a reaction.
func (s *MoverSeamSuite) duel(mgr *session.Manager, fighterAt, skeletonAt spatial.Position, reactions int) {
	ctx := context.Background()
	_, err := mgr.StartSession(ctx, &session.StartSessionInput{
		Session: "sess", Encounter: "world", World: tombRoom(12, 6),
	})
	s.Require().NoError(err)

	_, err = mgr.Join(ctx, &session.JoinInput{Session: "sess", Member: "fighter", Position: fighterAt})
	s.Require().NoError(err)
	s.inCombat("fighter", reactions)

	spawned, err := mgr.Spawn(ctx, &session.SpawnInput{
		Session: "sess", ID: "skel-1", Ref: refs.Monsters.Skeleton().String(), Position: skeletonAt,
	})
	s.Require().NoError(err)
	s.Require().NotNil(spawned.Formed, "adjacent and in sight starts the fight")
}

// TestAMonsterLeavingTheFightersReachTakesTheBlow is the design's first
// done-when: a monster walks out of a player's reach on its OWN turn and the
// player's longsword answers, with nobody declaring anything.
func (s *MoverSeamSuite) TestAMonsterLeavingTheFightersReachTakesTheBlow() {
	ctx := context.Background()
	mgr := s.managerWith(&retreatWalker{path: []spatial.Position{hexCell(3, 0), hexCell(4, 0)}})
	s.duel(mgr, hexCell(1, 0), hexCell(2, 0), 1)

	// The fighter passes; the skeleton's whole turn drives inside this one
	// call, and its first cell leaves the fighter's reach.
	_, err := mgr.EndTurn(ctx, &session.EndTurnInput{
		Session: "sess", Member: "fighter",
		DeclarationID: currentEndTurnID(s.T(), mgr, "sess", "fighter"),
	})
	s.Require().NoError(err)

	beats := s.reactionBeats(mgr, "fighter")
	s.Require().Len(beats, 1, "one opportunity attack, on the one cell that left reach")
	s.Equal("fighter", beats[0].Actor, "the reactor swings; the mover is struck")
	s.Equal([]string{"skel-1"}, beats[0].Targets)
	s.Equal(oaRef(), beats[0].Reaction.Ref)
	s.Equal("Opportunity Attack", beats[0].Reaction.Name)
}

// TestTheFighterLeavingAMonstersReachTakesTheBite is the same rule read from
// the other side — the half no stack has ever run, because the player's walk
// announced nothing at all.
func (s *MoverSeamSuite) TestTheFighterLeavingAMonstersReachTakesTheBite() {
	ctx := context.Background()
	mgr := s.managerWith(session.Pass{})
	s.duel(mgr, hexCell(2, 0), hexCell(3, 0), 1)

	out, err := mgr.Move(ctx, &session.MoveInput{
		Session: "sess", Member: "fighter", Path: []spatial.Position{hexCell(1, 0)},
		DeclarationID: currentMoveID(s.T(), mgr, "sess", "fighter"),
	})
	s.Require().NoError(err)
	s.Require().Len(out.Steps, 1, "the walk itself completes; the bite does not stop it")

	beats := s.reactionBeats(mgr, "fighter")
	s.Require().Len(beats, 1)
	s.Equal("skel-1", beats[0].Actor, "the skeleton swings")
	s.Equal([]string{"fighter"}, beats[0].Targets)
	s.Equal(oaRef(), beats[0].Reaction.Ref)
}

// TestADisengagingMoverProvokesNothing proves the design's claim that Disengage
// needs no special case here: the movement machine folds the chain, the
// Disengaging condition marks opportunity attacks prevented on the fold, and
// this seam never learns the word.
func (s *MoverSeamSuite) TestADisengagingMoverProvokesNothing() {
	ctx := context.Background()
	mgr := s.managerWith(session.Pass{})
	s.duel(mgr, hexCell(2, 0), hexCell(3, 0), 1)
	s.disengaging("fighter")

	out, err := mgr.Move(ctx, &session.MoveInput{
		Session: "sess", Member: "fighter", Path: []spatial.Position{hexCell(1, 0)},
		DeclarationID: currentMoveID(s.T(), mgr, "sess", "fighter"),
	})
	s.Require().NoError(err)
	s.Require().Len(out.Steps, 1)

	s.Empty(s.reactionBeats(mgr, "fighter"),
		"a disengaging walker leaves the same square and takes nothing")
}

// TestAnAlliedReactorDoesNotSwing is where rpg-toolkit#899 and #766 close. The
// condition publishes its trigger for an ally exactly as it does for an enemy —
// it holds geometry and economy, not sides — and this seam declines to hand a
// friend a weapon, because who is an enemy of whom is the RUN's answer.
func (s *MoverSeamSuite) TestAnAlliedReactorDoesNotSwing() {
	ctx := context.Background()
	mgr := s.managerWith(session.Pass{})

	_, err := mgr.StartSession(ctx, &session.StartSessionInput{
		Session: "sess", Encounter: "world", World: tombRoom(12, 6),
	})
	s.Require().NoError(err)
	_, err = mgr.Join(ctx, &session.JoinInput{Session: "sess", Member: "fighter", Position: hexCell(2, 0)})
	s.Require().NoError(err)
	_, err = mgr.Join(ctx, &session.JoinInput{Session: "sess", Member: "ally", Position: hexCell(3, 0)})
	s.Require().NoError(err)
	s.inCombat("ally", 1)

	// Free roam: two players and no monster, so no fight forms and no
	// declaration is offered.
	out, err := mgr.Move(ctx, &session.MoveInput{
		Session: "sess", Member: "fighter", Path: []spatial.Position{hexCell(1, 0)},
	})
	s.Require().NoError(err)
	s.Require().Len(out.Steps, 1)

	s.Empty(s.reactionBeats(mgr, "fighter"), "a friend does not swing at a friend walking away")
	s.Empty(s.reactionBeats(mgr, "ally"))
}

// TestAReactorWithNoReactionLeftDoesNotSwing is TestAMonsterLeaving... with one
// number changed: the fighter has already spent this turn's reaction, so the
// condition never publishes and no beat is recorded.
func (s *MoverSeamSuite) TestAReactorWithNoReactionLeftDoesNotSwing() {
	ctx := context.Background()
	mgr := s.managerWith(&retreatWalker{path: []spatial.Position{hexCell(3, 0), hexCell(4, 0)}})
	s.duel(mgr, hexCell(1, 0), hexCell(2, 0), 0)

	_, err := mgr.EndTurn(ctx, &session.EndTurnInput{
		Session: "sess", Member: "fighter",
		DeclarationID: currentEndTurnID(s.T(), mgr, "sess", "fighter"),
	})
	s.Require().NoError(err)

	s.Empty(s.reactionBeats(mgr, "fighter"), "a spent reaction is no reaction")
}

// TestAReactionThatDropsTheWalkerStopsTheWalk is ruling R6 on the player's side
// of the walk: the swing was checked against the cell the walker still stood
// on, so a walker it fells stays in that cell and the remaining path is
// abandoned — a stop, not an error.
func (s *MoverSeamSuite) TestAReactionThatDropsTheWalkerStopsTheWalk() {
	ctx := context.Background()

	// Four hit points at FULL health, because Join re-seats a joining member
	// at their maximum: the shortsword's eight is certain to fell her.
	brittle := armedFighter("fighter")
	brittle.MaxHitPoints = 4
	brittle.HitPoints = 4
	s.characters = newFakeCharacters(brittle, armedFighter("ally"))
	mgr := s.managerWith(session.Pass{})
	s.duel(mgr, hexCell(2, 0), hexCell(3, 0), 1)
	out, err := mgr.Move(ctx, &session.MoveInput{
		Session: "sess", Member: "fighter", Path: []spatial.Position{hexCell(1, 0), hexCell(0, 0)},
		DeclarationID: currentMoveID(s.T(), mgr, "sess", "fighter"),
	})
	s.Require().NoError(err, "a walk cut short by a reaction is news, not a failure")
	s.Empty(out.Steps, "the blow landed before the first cell was entered")

	where, err := mgr.Where(ctx, &session.WhereInput{Session: "sess", Member: "fighter"})
	s.Require().NoError(err)
	s.Equal(hexCell(2, 0), where.Position, "she falls in the cell she was leaving, not the one she was entering")

	beats := s.reactionBeats(mgr, "fighter")
	s.Require().Len(beats, 1)
	s.Equal("struck", beats[0].Beat)
	s.Equal("skel-1", beats[0].Actor)
}
