// Copyright (C) 2026 Kirk Diggler
// SPDX-License-Identifier: GPL-3.0-or-later

package session_test

import (
	"context"
	"encoding/json"
	"testing"

	"github.com/stretchr/testify/suite"

	"github.com/KirkDiggler/rpg-toolkit/core"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/character"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/conditions"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/refs"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/session"
	"github.com/KirkDiggler/rpg-toolkit/tools/spatial"
)

// PostRollWindowSuite is rpg-project#398's done-when: an attack by somebody
// holding a spendable die STOPS after the d20 and asks them, and the answer
// finishes the swing.
//
// Every scene drives the whole stack — the Attack verb poses, resolution
// freezes its own machine, the ledger persists, Afford offers the row, React
// resumes — because what is being proven is that those agree. A unit test of
// any one of them would pass with the rest wired wrong.
type PostRollWindowSuite struct {
	suite.Suite

	sessions   *fakeSessions
	encounters *fakeEncounters
	characters *fakeCharacters
}

func TestPostRollWindowSuite(t *testing.T) {
	suite.Run(t, new(PostRollWindowSuite))
}

func (s *PostRollWindowSuite) SetupTest() {
	s.sessions = newFakeSessions()
	s.encounters = newFakeEncounters()
	s.characters = newFakeCharacters(armedFighter("alice"), armedFighter("bob"))
}

// inspire hands the named member a die directly on their stored sheet.
//
// The grant itself is exercised by the activation scenes; every scene about
// SPENDING one starts from a member who already holds it, so a change to the
// grant cannot quietly rewrite what the spend scenes are testing.
func (s *PostRollWindowSuite) inspire(member string) {
	ctx := context.Background()
	stored, err := s.characters.GetCharacter(ctx, member)
	s.Require().NoError(err)
	raw, err := conditions.NewInspiredCondition(member, "bob", conditions.InspiredDie).ToJSON()
	s.Require().NoError(err)
	stored.Conditions = append(stored.Conditions, raw)
	s.Require().NoError(s.characters.SaveCharacter(ctx, stored))
}

// holdsTheDie reads whether the member's stored sheet still carries one.
func (s *PostRollWindowSuite) holdsTheDie(member string) bool {
	stored, err := s.characters.GetCharacter(context.Background(), member)
	s.Require().NoError(err)
	return carriesInspiration(s.T(), stored)
}

// carriesInspiration reads a stored sheet for a Bardic Inspiration die.
//
// Decoded LENIENTLY, because a sheet carries several conditions and they do
// not all spell their ref the same way. Only one of them is being looked for.
func carriesInspiration(t fataler, stored *character.Data) bool {
	for _, raw := range stored.Conditions {
		var peek struct {
			Ref *core.Ref `json:"ref"`
		}
		if json.Unmarshal(raw, &peek) != nil || peek.Ref == nil {
			continue
		}
		if peek.Ref.ID == refs.Conditions.Inspired().ID {
			return true
		}
	}
	_ = t
	return false
}

// duel is attack_test.go's authored scene: alice and bob adjacent on a turn
// clock with alice active, so a swing executes only through the selector
// Afford authored and no initiative roll consumes the dice these scenes assert.
func (s *PostRollWindowSuite) duel() *session.Manager {
	return s.duelOverStores()
}

// duelOverStores builds a manager over this suite's stores. A second call with
// the stores unchanged is a RESTART: nothing in memory survives it.
func (s *PostRollWindowSuite) duelOverStores() *session.Manager {
	mgr, err := session.NewManager(&session.Config{
		PresentationIDs: testPresentationIDs{}, Dice: testDice{}, TurnDriver: session.Pass{},
		Sessions: s.sessions, Encounters: s.encounters,
		Characters: s.characters, Events: session.DiscardEvents{},
	})
	s.Require().NoError(err)
	return mgr
}

// open starts the session on the authored duel world.
func (s *PostRollWindowSuite) open(mgr *session.Manager) {
	_, err := mgr.StartSession(context.Background(), &session.StartSessionInput{
		Session: "sess", Encounter: "world", World: duelWorld(s.T()),
	})
	s.Require().NoError(err)
}

// scene is the whole setup: stores, manager, world.
func (s *PostRollWindowSuite) scene() *session.Manager {
	mgr := s.duel()
	s.open(mgr)
	return mgr
}

// swing is alice attacking bob through the current offer.
func (s *PostRollWindowSuite) swing(mgr *session.Manager) *session.AttackOutput {
	out, err := mgr.Attack(context.Background(), &session.AttackInput{
		Session: "sess", Attacker: "alice", Target: "bob",
		DeclarationID: currentAttackID(s.T(), mgr, "sess", "alice"),
	})
	s.Require().NoError(err)
	return out
}

// declarations is one member's whole panel.
func (s *PostRollWindowSuite) declarations(mgr *session.Manager, member string) []session.Declaration {
	out, err := mgr.Afford(context.Background(), &session.AffordInput{Session: "sess", Member: member})
	s.Require().NoError(err)
	return out.Declarations
}

// reactRow is the member's own open REACT row, or the zero value.
func (s *PostRollWindowSuite) reactRow(mgr *session.Manager, member string) session.Declaration {
	for _, declaration := range s.declarations(mgr, member) {
		if declaration.Verb == session.VerbReact {
			return declaration
		}
	}
	return session.Declaration{}
}

// answer answers alice's own open window.
func (s *PostRollWindowSuite) answer(mgr *session.Manager, choice session.ReactChoice) {
	row := s.reactRow(mgr, "alice")
	s.Require().NotEmpty(row.ID, "no open window for alice")
	_, err := mgr.React(context.Background(), &session.ReactInput{
		Session: "sess", Member: "alice", DeclarationID: row.ID, Choice: choice,
	})
	s.Require().NoError(err)
}

// beatsOf returns every beat in a member's story, decoded loosely.
func (s *PostRollWindowSuite) beatsOf(mgr *session.Manager, member string) []map[string]any {
	entries, err := mgr.Story(context.Background(), &session.StoryInput{Session: "sess", Member: member})
	s.Require().NoError(err)
	out := make([]map[string]any, 0, len(entries))
	for _, entry := range entries {
		var beat map[string]any
		s.Require().NoError(json.Unmarshal(entry.Payload, &beat))
		out = append(out, beat)
	}
	return out
}

// countBeats counts beats of one kind in a member's story.
func (s *PostRollWindowSuite) countBeats(mgr *session.Manager, member, kind string) int {
	count := 0
	for _, beat := range s.beatsOf(mgr, member) {
		if beat["beat"] == kind {
			count++
		}
	}
	return count
}

// TestAnUninspiredSwingIsUnchanged is the regression that matters most: the
// offer chain folds on every attack now, and an attack by somebody holding
// nothing must resolve in one call exactly as it always did.
func (s *PostRollWindowSuite) TestAnUninspiredSwingIsUnchanged() {
	mgr := s.scene()

	out := s.swing(mgr)

	s.False(out.Paused, "nobody offered anything, so nothing was asked")
	s.Equal(10, out.Roll)
	s.NotZero(out.Against, "an unpaused swing reports what it was against")
	s.Empty(s.reactRow(mgr, "alice").ID, "and no window is open")
	s.Equal(1, s.countBeats(mgr, "alice", "struck")+s.countBeats(mgr, "alice", "missed"))
	s.Zero(s.countBeats(mgr, "alice", "roll_window_opened"))
}

// TestTheSwingStopsAndAsksTheRoller is the pose, end to end.
func (s *PostRollWindowSuite) TestTheSwingStopsAndAsksTheRoller() {
	mgr := s.scene()
	s.inspire("alice")

	out := s.swing(mgr)

	s.True(out.Paused, "the swing is waiting on an answer")
	s.Equal(10, out.Roll, "the d20 the player is deciding about")
	s.NotZero(out.Total)
	s.Zero(out.Against, "the AC is deliberately not shown before the choice")
	s.False(out.Hit)
	s.Zero(out.Damage)

	s.Zero(s.countBeats(mgr, "alice", "struck")+s.countBeats(mgr, "alice", "missed"),
		"no outcome beat: there is no outcome yet")
	s.Equal(1, s.countBeats(mgr, "alice", "roll_window_opened"))
}

// TestTheBeatCarriesTheNumbersAndNotTheAC — the audience decides from the beat.
func (s *PostRollWindowSuite) TestTheBeatCarriesTheNumbersAndNotTheAC() {
	mgr := s.scene()
	s.inspire("alice")
	out := s.swing(mgr)

	var window map[string]any
	for _, beat := range s.beatsOf(mgr, "alice") {
		if beat["beat"] == "roll_window_opened" {
			window = beat
		}
	}
	s.Require().NotNil(window)
	s.Equal("alice", window["audience"])
	s.Equal(float64(out.Roll), window["roll"])
	s.Equal(float64(out.Total), window["total"])
	offer, ok := window["offer"].(map[string]any)
	s.Require().True(ok)
	s.Equal(refs.Conditions.Inspired().String(), offer["ref"])
	s.Equal(conditions.InspiredName, offer["name"])
	_, leaked := window["against"]
	s.False(leaked)
}

// TestTheDockIsToldWhatItMayDo is the panel the player actually sees: their own
// REACT row named for the die, and everybody else told the table is waiting.
func (s *PostRollWindowSuite) TestTheDockIsToldWhatItMayDo() {
	mgr := s.scene()
	s.inspire("alice")
	s.swing(mgr)

	row := s.reactRow(mgr, "alice")
	s.Require().NotEmpty(row.ID)
	s.True(row.Available)
	s.Require().NotNil(row.Reaction)
	s.Equal(conditions.InspiredName, row.Reaction.Name, "the server authors the label")
	s.Equal(refs.Conditions.Inspired().String(), row.Reaction.Ref)
	s.Equal(session.TargetNone, row.TargetKind, "there is no step and nobody to aim at")
	s.Empty(row.Candidates)
	s.Equal(session.SlotNone, row.Slot, "answering costs no reaction")

	s.Empty(s.reactRow(mgr, "bob").ID, "the bard is not the audience")
	for _, declaration := range s.declarations(mgr, "bob") {
		s.Require().NotNil(declaration.Why, "every other row is blocked")
		s.Equal(session.ShortfallWindowOpen, declaration.Why.Reason)
	}
}

// TestSpendingFinishesTheSwingWithTheDieOnIt is the walk's spend branch: one
// outcome beat, the total carrying the face, and the die gone.
func (s *PostRollWindowSuite) TestSpendingFinishesTheSwingWithTheDieOnIt() {
	mgr := s.scene()
	s.inspire("alice")
	posed := s.swing(mgr)

	s.answer(mgr, session.ReactStrike)

	s.Equal(1, s.countBeats(mgr, "alice", "struck")+s.countBeats(mgr, "alice", "missed"),
		"exactly one outcome beat across the pause and the answer")

	beat := s.outcomeBeat(mgr)
	s.Equal(float64(posed.Roll), beat["roll"], "the d20 is not re-rolled")
	s.Greater(beat["total"], float64(posed.Total), "the face joined the total")
	s.Equal(float64(6), beat["total"].(float64)-float64(posed.Total), "a d6 rolling its own face")
	calculation, ok := beat["calculation"].(map[string]any)
	s.Require().True(ok, "the resumed beat retains the frozen sourced calculation")
	s.Equal(beat["total"], calculation["total"])
	components, ok := calculation["components"].([]any)
	s.Require().True(ok)
	s.Require().Len(components, 3)
	inspiration := components[2].(map[string]any)
	source := inspiration["source"].(map[string]any)
	s.Equal(refs.Conditions.Inspired().String(), source["ref"],
		"the offered die keeps the provider-authored source identity")

	s.False(s.holdsTheDie("alice"), "the die is spent when it is TAKEN")
	s.Empty(s.reactRow(mgr, "alice").ID, "and the window is closed")
}

// TestKeepingFinishesTheSwingWithoutIt is the other branch, and the reason
// declining is worth offering: the die is still in hand afterwards.
func (s *PostRollWindowSuite) TestKeepingFinishesTheSwingWithoutIt() {
	mgr := s.scene()
	s.inspire("alice")
	posed := s.swing(mgr)

	s.answer(mgr, session.ReactHold)

	s.Equal(1, s.countBeats(mgr, "alice", "struck")+s.countBeats(mgr, "alice", "missed"))
	beat := s.outcomeBeat(mgr)
	s.Equal(float64(posed.Total), beat["total"], "no face joined it")
	calculation, ok := beat["calculation"].(map[string]any)
	s.Require().True(ok, "declining preserves the frozen calculation without rewriting it")
	s.Equal(beat["total"], calculation["total"])

	s.True(s.holdsTheDie("alice"), "declining costs nothing")
}

// outcomeBeat is the one struck-or-missed beat in the fighter's story.
func (s *PostRollWindowSuite) outcomeBeat(mgr *session.Manager) map[string]any {
	for _, beat := range s.beatsOf(mgr, "alice") {
		if beat["beat"] == "struck" || beat["beat"] == "missed" {
			return beat
		}
	}
	s.Require().Fail("no outcome beat in the story")
	return nil
}

// TestTheEncounterIsNeverPaused is the boundary between the two pauses. The
// composition's PausedTurn is a driven-turn remainder; a player's own attack
// has no path, so this pause lives in the ledger alone and React's shipped
// guard is what makes that correct.
func (s *PostRollWindowSuite) TestTheEncounterIsNeverPaused() {
	mgr := s.scene()
	s.inspire("alice")
	s.swing(mgr)

	data, err := s.encounters.GetEncounter(context.Background(), "world")
	s.Require().NoError(err)
	s.Nil(data.PausedTurn, "the encounter is not the thing that is waiting")

	s.answer(mgr, session.ReactStrike)

	data, err = s.encounters.GetEncounter(context.Background(), "world")
	s.Require().NoError(err)
	s.Nil(data.PausedTurn)
}

// TestTheFightIsFrozenWhileTheQuestionStands — every change verb refuses, which
// is the shipped freeze applying unchanged to a second kind of window.
func (s *PostRollWindowSuite) TestTheFightIsFrozenWhileTheQuestionStands() {
	mgr := s.scene()
	s.inspire("alice")
	s.swing(mgr)
	ctx := context.Background()

	_, err := mgr.Attack(ctx, &session.AttackInput{
		Session: "sess", Attacker: "alice", Target: "bob", DeclarationID: "anything",
	})
	s.Require().ErrorIs(err, session.ErrWindowOpen, "a second swing cannot start")

	_, err = mgr.EndTurn(ctx, &session.EndTurnInput{
		Session: "sess", Member: "alice", DeclarationID: "anything",
	})
	s.Require().ErrorIs(err, session.ErrWindowOpen)

	_, err = mgr.Move(ctx, &session.MoveInput{
		Session: "sess", Member: "alice", Path: []spatial.Position{hexCell(1, 2)},
		DeclarationID: "anything",
	})
	s.Require().ErrorIs(err, session.ErrWindowOpen)
}

// TestOnlyTheAudienceMayAnswer — the window is the roller's, and somebody
// else's client cannot spend their die for them.
func (s *PostRollWindowSuite) TestOnlyTheAudienceMayAnswer() {
	mgr := s.scene()
	s.inspire("alice")
	s.swing(mgr)
	row := s.reactRow(mgr, "alice")

	_, err := mgr.React(context.Background(), &session.ReactInput{
		Session: "sess", Member: "bob", DeclarationID: row.ID, Choice: session.ReactStrike,
	})

	s.Require().ErrorIs(err, session.ErrNotAudience)
	s.True(s.holdsTheDie("alice"), "and the die is untouched")
}

// TestTheWindowSurvivesAReload is "a restart of rpg-api between the pose and
// the answer changes nothing". Both aggregates carry their half.
func (s *PostRollWindowSuite) TestTheWindowSurvivesAReload() {
	mgr := s.scene()
	s.inspire("alice")
	posed := s.swing(mgr)

	// A second manager over the same stored records: nothing in memory
	// survives, only what was written.
	restarted, err := session.NewManager(&session.Config{
		PresentationIDs: testPresentationIDs{}, Dice: testDice{}, TurnDriver: session.Pass{},
		Sessions: s.sessions, Encounters: s.encounters,
		Characters: s.characters, Events: session.DiscardEvents{},
	})
	s.Require().NoError(err)

	row := s.reactRow(restarted, "alice")
	s.Require().NotEmpty(row.ID, "the question is still being asked")
	s.Equal(conditions.InspiredName, row.Reaction.Name)

	_, err = restarted.React(context.Background(), &session.ReactInput{
		Session: "sess", Member: "alice", DeclarationID: row.ID, Choice: session.ReactStrike,
	})
	s.Require().NoError(err)

	beat := s.outcomeBeat(restarted)
	s.Equal(float64(posed.Roll), beat["roll"])
	s.False(s.holdsTheDie("alice"))
}

// TestTheActionIsChargedOnceAcrossThePause: the cost was paid at the door of
// the first swing, and the resume passes none.
func (s *PostRollWindowSuite) TestTheActionIsChargedOnceAcrossThePause() {
	mgr := s.scene()
	s.inspire("alice")
	s.swing(mgr)

	afterPose := s.actionsLeft("alice")
	s.answer(mgr, session.ReactStrike)

	s.Equal(afterPose, s.actionsLeft("alice"), "answering charges nothing")
	s.Equal(1, s.reactionsLeftFor("alice"), "and it is not a reaction either")
}

func (s *PostRollWindowSuite) actionsLeft(member string) int {
	stored, err := s.characters.GetCharacter(context.Background(), member)
	s.Require().NoError(err)
	s.Require().NotNil(stored.ActionEconomy)
	return stored.ActionEconomy.ActionsRemaining
}

func (s *PostRollWindowSuite) reactionsLeftFor(member string) int {
	stored, err := s.characters.GetCharacter(context.Background(), member)
	s.Require().NoError(err)
	s.Require().NotNil(stored.ActionEconomy)
	return stored.ActionEconomy.ReactionsRemaining
}

// aFreshTurnFor makes the member's stored economy stale, so the next door
// refreshes it and a second swing is affordable.
//
// The turn is not actually advanced, because this scene is two players and
// nothing else: ending alice's turn dissolves a fight with no hostile pair in
// it, and the point being made here is about the DIE rather than about the
// clock. Filing the economy under another turn is what
// character.RefreshForTurn already treats as "this is stale, fill it" — the
// same path a real second turn takes.
func (s *PostRollWindowSuite) aFreshTurnFor(member string) {
	ctx := context.Background()
	stored, err := s.characters.GetCharacter(ctx, member)
	s.Require().NoError(err)
	s.Require().NotNil(stored.ActionEconomy, "%q has not acted yet", member)
	stored.ActionEconomy.TurnNumber = 0
	s.Require().NoError(s.characters.SaveCharacter(ctx, stored))
}

// TestTheDieIsConsumedOnce — a second swing after a spend poses nothing.
func (s *PostRollWindowSuite) TestTheDieIsConsumedOnce() {
	mgr := s.scene()
	s.inspire("alice")
	s.swing(mgr)
	s.answer(mgr, session.ReactStrike)
	s.aFreshTurnFor("alice")

	second := s.swing(mgr)

	s.False(second.Paused, "the die is gone, so nothing is offered")
	s.Empty(s.reactRow(mgr, "alice").ID)
}

// TestKeepingLeavesTheDieForTheNextSwing — the other half of the same claim.
func (s *PostRollWindowSuite) TestKeepingLeavesTheDieForTheNextSwing() {
	mgr := s.scene()
	s.inspire("alice")
	s.swing(mgr)
	s.answer(mgr, session.ReactHold)
	s.aFreshTurnFor("alice")

	second := s.swing(mgr)

	s.True(second.Paused, "the die is still in hand, so it is offered again")
}
