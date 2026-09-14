// Copyright (C) 2026 Kirk Diggler
// SPDX-License-Identifier: GPL-3.0-or-later

package session_test

// guidance_check_offer_test.go is Guidance's done-when for Unlock
// (docs/ideas/cleric/plan.md): a lock check by somebody holding a Guidance
// die STOPS after the d20 and asks them, and the answer finishes the
// attempt. Every scene drives the whole stack — Unlock poses, resolution
// freezes its own check, the ledger persists, Afford offers the row, React
// resumes — for [PostRollWindowSuite]'s own reason: proving any one of them
// in isolation would pass with the rest wired wrong.

import (
	"context"
	"encoding/json"
	"testing"

	"github.com/stretchr/testify/suite"

	"github.com/KirkDiggler/rpg-toolkit/core"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/conditions"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/refs"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/session"
)

type GuidedUnlockSuite struct {
	suite.Suite

	sessions   *fakeSessions
	encounters *fakeEncounters
	characters *fakeCharacters
}

func TestGuidedUnlockSuite(t *testing.T) {
	suite.Run(t, new(GuidedUnlockSuite))
}

func (s *GuidedUnlockSuite) SetupTest() {
	s.sessions = newFakeSessions()
	s.encounters = newFakeEncounters()
	// DEX 10 (+0): testDice's flat check-roll of 10 misses tombDC (12) on its
	// own — the scene that makes the die's contribution visible rather than
	// redundant with a check that would have succeeded anyway.
	s.characters = newFakeCharacters(deftCharacter("alice", 10))
}

// guide hands alice a Guidance die directly on her stored sheet —
// [PostRollWindowSuite.inspire]'s pattern for a different condition.
func (s *GuidedUnlockSuite) guide() {
	ctx := context.Background()
	stored, err := s.characters.GetCharacter(ctx, "alice")
	s.Require().NoError(err)
	condition, err := conditions.NewGuidedCondition(conditions.NewGuidedConditionInput{
		MemberID: "alice", SourceID: "cleric-1", SourceRef: refs.Spells.Guidance(),
	})
	s.Require().NoError(err)
	raw, err := condition.ToJSON()
	s.Require().NoError(err)
	stored.Conditions = append(stored.Conditions, raw)
	s.Require().NoError(s.characters.SaveCharacter(ctx, stored))
}

// holdsTheDie reads whether alice's stored sheet still carries the Guidance
// condition.
func (s *GuidedUnlockSuite) holdsTheDie() bool {
	stored, err := s.characters.GetCharacter(context.Background(), "alice")
	s.Require().NoError(err)
	for _, raw := range stored.Conditions {
		var peek struct {
			Ref *core.Ref `json:"ref"`
		}
		if json.Unmarshal(raw, &peek) != nil || peek.Ref == nil {
			continue
		}
		if peek.Ref.ID == refs.Conditions.Guided().ID {
			return true
		}
	}
	return false
}

func (s *GuidedUnlockSuite) sceneOverStores() *session.Manager {
	mgr, err := session.NewManager(&session.Config{
		PresentationIDs: testPresentationIDs{}, Dice: testDice{}, TurnDriver: session.Pass{},
		Sessions: s.sessions, Encounters: s.encounters,
		Characters: s.characters, Events: session.DiscardEvents{},
	})
	s.Require().NoError(err)
	_, err = mgr.StartSession(context.Background(), &session.StartSessionInput{
		Session: "sess", Encounter: "world", World: gatedWorld(s.T(), tombLock()),
	})
	s.Require().NoError(err)
	return mgr
}

func (s *GuidedUnlockSuite) tryLock(mgr *session.Manager) *session.UnlockOutput {
	out, err := mgr.Unlock(context.Background(), &session.UnlockInput{
		Session: "sess", Member: "alice", Door: "gate",
	})
	s.Require().NoError(err)
	return out
}

func (s *GuidedUnlockSuite) declarations(mgr *session.Manager, member string) []session.Declaration {
	out, err := mgr.Afford(context.Background(), &session.AffordInput{Session: "sess", Member: member})
	s.Require().NoError(err)
	return out.Declarations
}

func (s *GuidedUnlockSuite) reactRow(mgr *session.Manager, member string) session.Declaration {
	for _, declaration := range s.declarations(mgr, member) {
		if declaration.Verb == session.VerbReact {
			return declaration
		}
	}
	return session.Declaration{}
}

func (s *GuidedUnlockSuite) answer(mgr *session.Manager, choice session.ReactChoice) {
	row := s.reactRow(mgr, "alice")
	s.Require().NotEmpty(row.ID, "no open window for alice")
	_, err := mgr.React(context.Background(), &session.ReactInput{
		Session: "sess", Member: "alice", DeclarationID: row.ID, Choice: choice,
	})
	s.Require().NoError(err)
}

// TestAnUnguidedUnlockIsUnchanged is the regression that matters most: the
// offer chain now folds on every check, and a checker holding nothing must
// resolve in one call exactly as it always did.
func (s *GuidedUnlockSuite) TestAnUnguidedUnlockIsUnchanged() {
	mgr := s.sceneOverStores()

	out := s.tryLock(mgr)

	s.False(out.Paused, "nobody offered anything, so nothing was asked")
	s.False(out.Beaten, "10 rolled + 0 dex misses DC 12")
	s.Equal(10, out.Total)
	s.Empty(s.reactRow(mgr, "alice").ID, "and no window is open")
}

// TestAGuidedUnlockStopsAndAsks is the pose, end to end.
func (s *GuidedUnlockSuite) TestAGuidedUnlockStopsAndAsks() {
	mgr := s.sceneOverStores()
	s.guide()

	out := s.tryLock(mgr)

	s.True(out.Paused, "the attempt is waiting on an answer")
	s.Require().NotNil(out.Roll)
	s.Equal(10, *out.Roll, "the d20 the player is deciding about")
	s.Equal(10, out.Total, "the pre-offer total, not the lock's DC")
	s.False(out.Beaten, "no verdict yet — Paused says so structurally")
	s.Equal(session.Door{}, out.Door, "nothing about the door has happened yet")
}

// TestTheDockOffersTheGuidanceRow is the panel the player actually sees.
func (s *GuidedUnlockSuite) TestTheDockOffersTheGuidanceRow() {
	mgr := s.sceneOverStores()
	s.guide()
	s.tryLock(mgr)

	row := s.reactRow(mgr, "alice")
	s.Require().NotEmpty(row.ID)
	s.True(row.Available)
	s.Require().NotNil(row.Reaction)
	s.Equal(conditions.GuidedName, row.Reaction.Name, "the server authors the label")
	s.Equal(refs.Conditions.Guided().String(), row.Reaction.Ref)
	s.Equal(session.TargetNone, row.TargetKind, "there is no step and nobody to aim at")
	s.Empty(row.Candidates)
	s.Equal(session.SlotNone, row.Slot, "answering costs no reaction")
}

// TestSpendingFinishesTheUnlockWithTheDieOnIt is the walk's spend branch:
// the die turns a miss into a beaten lock, and the door actually opens.
func (s *GuidedUnlockSuite) TestSpendingFinishesTheUnlockWithTheDieOnIt() {
	mgr := s.sceneOverStores()
	s.guide()
	s.tryLock(mgr)

	s.answer(mgr, session.ReactStrike)

	doors, err := mgr.Doors(context.Background(), &session.DoorsInput{Session: "sess", Member: "alice"})
	s.Require().NoError(err)
	s.Require().Len(doors.Doors, 1)
	s.Equal("open", doors.Doors[0].State, "10 + 0 dex + 4 (a d4 rolling its own face) beats DC 12")

	s.False(s.holdsTheDie(), "the die is spent when it is TAKEN")
	s.Empty(s.reactRow(mgr, "alice").ID, "and the window is closed")
}

// TestKeepingFinishesTheUnlockWithoutIt is the other branch: the lock stays
// beaten by nothing, and the die is still in hand afterwards.
func (s *GuidedUnlockSuite) TestKeepingFinishesTheUnlockWithoutIt() {
	mgr := s.sceneOverStores()
	s.guide()
	s.tryLock(mgr)

	s.answer(mgr, session.ReactHold)

	doors, err := mgr.Doors(context.Background(), &session.DoorsInput{Session: "sess", Member: "alice"})
	s.Require().NoError(err)
	s.Require().Len(doors.Doors, 1)
	s.Equal("locked", doors.Doors[0].State, "declining leaves the miss a miss")

	s.True(s.holdsTheDie(), "declining costs nothing")
}

// TestOnlyTheAudienceMayAnswer — the window is the checker's, and somebody
// else's client cannot spend their die for them.
func (s *GuidedUnlockSuite) TestOnlyTheAudienceMayAnswer() {
	mgr := s.sceneOverStores()
	s.guide()
	s.tryLock(mgr)
	row := s.reactRow(mgr, "alice")

	_, err := mgr.React(context.Background(), &session.ReactInput{
		Session: "sess", Member: "bob", DeclarationID: row.ID, Choice: session.ReactStrike,
	})

	s.Require().ErrorIs(err, session.ErrNotAudience)
	s.True(s.holdsTheDie(), "and the die is untouched")
}

// TestTheWindowSurvivesAReload is "a restart of rpg-api between the pose and
// the answer changes nothing" — [PostRollWindowSuite]'s own scene, for a
// check instead of a swing.
func (s *GuidedUnlockSuite) TestTheWindowSurvivesAReload() {
	mgr := s.sceneOverStores()
	s.guide()
	s.tryLock(mgr)

	restarted, err := session.NewManager(&session.Config{
		PresentationIDs: testPresentationIDs{}, Dice: testDice{}, TurnDriver: session.Pass{},
		Sessions: s.sessions, Encounters: s.encounters,
		Characters: s.characters, Events: session.DiscardEvents{},
	})
	s.Require().NoError(err)

	row := s.reactRow(restarted, "alice")
	s.Require().NotEmpty(row.ID, "the question is still being asked")
	s.Equal(conditions.GuidedName, row.Reaction.Name)

	_, err = restarted.React(context.Background(), &session.ReactInput{
		Session: "sess", Member: "alice", DeclarationID: row.ID, Choice: session.ReactStrike,
	})
	s.Require().NoError(err)

	doors, err := restarted.Doors(context.Background(), &session.DoorsInput{Session: "sess", Member: "alice"})
	s.Require().NoError(err)
	s.Equal("open", doors.Doors[0].State)
	s.False(s.holdsTheDie())
}
