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

// OpportunityAttackSuite is the first place ANYWHERE that the real
// OpportunityAttackCondition fires inside a real fold.
//
// Every OA test before this one published a MovementChain event by hand and
// installed its own readiness map, which proved the condition and not the
// feature — resolution/movement_test.go says so in as many words, standing a
// triggerFrom double in for the condition because "this module cannot seat
// one". Four green suites and nothing that walked.
//
// These walk. A player steps out of a skeleton's reach through the ordinary
// Move verb, and everything in between is production code: the loader's free
// grant, the condition's own predicate, resolution's fold and reaction
// request, the strike, and the beat.
type OpportunityAttackSuite struct {
	suite.Suite

	sessions   *fakeSessions
	encounters *fakeEncounters
}

func TestOpportunityAttackSuite(t *testing.T) {
	suite.Run(t, new(OpportunityAttackSuite))
}

func (s *OpportunityAttackSuite) SetupTest() {
	s.sessions = newFakeSessions()
	s.encounters = newFakeEncounters()
}

// walkAwayFrom stands a fighter next to a freshly spawned skeleton, then walks
// the fighter one cell out of its reach and returns the beats that walk
// produced.
//
// The skeleton is SPAWNED rather than authored into the world, so the free
// opportunity-attack grant monstertraits hands out at Attach is the one under
// test rather than a fixture's idea of it.
func (s *OpportunityAttackSuite) walkAwayFrom(fighter *character.Data) []map[string]any {
	ctx := context.Background()
	mgr, err := session.NewManager(&session.Config{
		Dice: testDice{}, TurnDriver: session.Behavior(),
		Sessions: s.sessions, Encounters: s.encounters,
		Characters: newFakeCharacters(fighter),
		Events:     session.DiscardEvents{},
	})
	s.Require().NoError(err)

	_, err = mgr.StartSession(ctx, &session.StartSessionInput{
		Session: "sess", Encounter: "world", World: tombRoom(12, 6),
	})
	s.Require().NoError(err)

	_, err = mgr.Join(ctx, &session.JoinInput{
		Session: "sess", Member: "fighter", Position: spatial.Position{X: 2, Y: 0},
	})
	s.Require().NoError(err)

	spawned, err := mgr.Spawn(ctx, &session.SpawnInput{
		Session: "sess", ID: "skel-1", Ref: refs.Monsters.Skeleton().String(),
		Position: spatial.Position{X: 3, Y: 0}, // adjacent: one cell is the reach
	})
	s.Require().NoError(err)
	s.Require().NotNil(spawned.Formed, "standing next to a skeleton starts the fight")
	s.Require().Equal("fighter", spawned.Formed.Order[0], "the fighter acts first")

	before := len(s.beats(mgr))

	// One step, straight away from the skeleton: adjacent at From, two cells
	// off at To. That is precisely the predicate — within reach at From,
	// outside it at To.
	_, err = mgr.Move(ctx, &session.MoveInput{
		Session: "sess", Member: "fighter",
		DeclarationID: currentMoveID(s.T(), mgr, "sess", "fighter"),
		Path:          []spatial.Position{{X: 1, Y: 0}},
	})
	s.Require().NoError(err, "the walk itself succeeds whether or not anything swings at it")

	return s.beats(mgr)[before:]
}

// beats reads the fighter's whole story as decoded payloads.
func (s *OpportunityAttackSuite) beats(mgr *session.Manager) []map[string]any {
	entries, err := mgr.Story(context.Background(), &session.StoryInput{Session: "sess", Member: "fighter"})
	s.Require().NoError(err)
	out := make([]map[string]any, 0, len(entries))
	for _, e := range entries {
		var body map[string]any
		s.Require().NoError(json.Unmarshal(e.Payload, &body))
		out = append(out, body)
	}
	return out
}

// reactionBeats is every beat that names itself a reaction.
func reactionBeats(beats []map[string]any) []map[string]any {
	var out []map[string]any
	for _, b := range beats {
		if _, ok := b["reaction"]; ok {
			out = append(out, b)
		}
	}
	return out
}

// TestWalkingOutOfReachProvokesAnOpportunityAttack is the whole slice, end to
// end, through production code only.
func (s *OpportunityAttackSuite) TestWalkingOutOfReachProvokesAnOpportunityAttack() {
	beats := s.walkAwayFrom(armedFighter("fighter"))

	reactions := reactionBeats(beats)
	s.Require().Len(reactions, 1, "the skeleton swings once as the fighter leaves: %+v", beats)

	reaction := reactions[0]
	s.Equal("skel-1", reaction["actor"], "the skeleton reacted, not the walker")
	s.Contains([]any{"struck", "missed"}, reaction["beat"],
		"a reaction is an ordinary strike; a miss is as much a reaction as a hit")

	named, ok := reaction["reaction"].(map[string]any)
	s.Require().True(ok, "the beat carries the reaction identity: %+v", reaction)
	s.Equal(refs.Conditions.OpportunityAttack().String(), named["ref"])
	s.Equal("Opportunity Attack", named["name"],
		"the log can say WHY a skeleton hit during the fighter's own move")
}

// TestDisengagingWalksAwayUntouched is the Done-when bullet as restated for
// this slice (ruling R4): a PLAYER who disengages walks away untouched.
//
// Monsters have no route to the Disengaging condition — monstertraits routes
// four traits and the free OA, and no combat abilities — so the player side is
// the one that exists end to end. Giving monsters Disengage is a separate
// question.
//
// It is the same walk as the test above with one condition added, which is
// what makes the pair worth having: the difference between them is entirely
// the rule under test, not the fixture.
func (s *OpportunityAttackSuite) TestDisengagingWalksAwayUntouched() {
	fighter := armedFighter("fighter")
	blob, err := conditions.NewDisengagingCondition("fighter").ToJSON()
	s.Require().NoError(err)
	fighter.Conditions = append(fighter.Conditions, blob)

	beats := s.walkAwayFrom(fighter)

	s.Empty(reactionBeats(beats),
		"disengaging suppresses the opportunity attack entirely: %+v", beats)
}

// TestAnOpportunityAttackThatDropsTheWalkerStopsTheWalk is ruling R6 on the
// player's path, and the mirror of what the encounter's own Move case does for
// a driven member. Two paths, one rule.
//
// The walker falls in the cell they were LEAVING and never enters the one they
// were entering — which here means the walk takes ZERO steps, because the
// reaction fires on the first cell's announcement, before that cell is stepped
// into. Announce-before-step is what makes that the honest outcome rather than
// an off-by-one: the skeleton swung because the fighter was still standing next
// to it.
//
// Before this slice, Manager.Move asked standing ONCE for the whole walk, on
// the documented grounds that "a Move cannot down or revive anyone". Announcing
// steps to the rules made that false, and the comment there now says so.
func (s *OpportunityAttackSuite) TestAnOpportunityAttackThatDropsTheWalkerStopsTheWalk() {
	ctx := context.Background()

	// Ten hit points against a shortsword that deals twelve on this suite's
	// flat dice: one opportunity attack is lethal, so the drop lands on the
	// first announced cell rather than somewhere down the path.
	fighter := armedFighter("fighter")
	fighter.HitPoints = 10

	mgr, err := session.NewManager(&session.Config{
		Dice: testDice{}, TurnDriver: session.Behavior(),
		Sessions: s.sessions, Encounters: s.encounters,
		Characters: newFakeCharacters(fighter),
		Events:     session.DiscardEvents{},
	})
	s.Require().NoError(err)

	_, err = mgr.StartSession(ctx, &session.StartSessionInput{
		Session: "sess", Encounter: "world", World: tombRoom(12, 6),
	})
	s.Require().NoError(err)

	_, err = mgr.Join(ctx, &session.JoinInput{
		Session: "sess", Member: "fighter", Position: spatial.Position{X: 2, Y: 0},
	})
	s.Require().NoError(err)

	spawned, err := mgr.Spawn(ctx, &session.SpawnInput{
		Session: "sess", ID: "skel-1", Ref: refs.Monsters.Skeleton().String(),
		Position: spatial.Position{X: 3, Y: 0},
	})
	s.Require().NoError(err)
	s.Require().NotNil(spawned.Formed)

	// Two cells asked for, straight away from the skeleton.
	out, err := mgr.Move(ctx, &session.MoveInput{
		Session: "sess", Member: "fighter",
		DeclarationID: currentMoveID(s.T(), mgr, "sess", "fighter"),
		Path:          []spatial.Position{{X: 1, Y: 0}, {X: 0, Y: 0}},
	})
	s.Require().NoError(err, "being dropped mid-walk is an outcome, not an error")

	s.Empty(out.Steps,
		"the reaction fired on the first cell's ANNOUNCEMENT, so no cell was entered")

	reactions := reactionBeats(s.beats(mgr))
	s.Require().Len(reactions, 1,
		"exactly one swing: the second cell is never announced, so it cannot provoke")

	where, err := mgr.Where(ctx, &session.WhereInput{Session: "sess", Member: "fighter"})
	s.Require().NoError(err)
	s.Equal(spatial.Position{X: 2, Y: 0}, where.Position,
		"the walker fell in the cell they were leaving")
}
