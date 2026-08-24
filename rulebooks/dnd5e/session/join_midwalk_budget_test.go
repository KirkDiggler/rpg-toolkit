// Copyright (C) 2026 Kirk Diggler
// SPDX-License-Identifier: GPL-3.0-or-later

package session_test

import (
	"context"
	"testing"

	"github.com/stretchr/testify/suite"

	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/character"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/session"
	"github.com/KirkDiggler/rpg-toolkit/tools/spatial"
)

// JoinMidwalkBudgetSuite pins rpg-project#253: a member who joins a fight
// mid-walk — recruited into a bubble that is already running, not the one
// that starts it — must still start their own first combat turn with their
// full speed, not whatever their action economy happened to be carrying.
//
// # What this reproduces, and what it turned out NOT to be
//
// The live report (Kirk, 2026-08-24) read as "the free-roam approach spent
// alice's movement before her first turn." That hypothesis does NOT hold:
// [Manager.priceWalk] decides free-roam-vs-turn-clock ONCE, from the clock
// state at the TOP of the Move call, before any step runs — a walk that
// starts on the world clock is priced nothing for the whole call regardless
// of what a mid-path step does, and TestFreeRoamMemberJoiningMidWalk (this
// file, git history) confirmed a genuinely-first-ever combat entrant seeds a
// full 30ft via [character.StartTurn] exactly as designed.
//
// What actually reproduces "5ft left, not 30" is a DIFFERENT, more basic
// defect: [character.Character.ExitCombat] — the method that clears
// actionEconomy back to nil when a fight is over — has NO CALLER anywhere in
// this module. `grep -rn ExitCombat rulebooks/dnd5e/session
// rulebooks/dnd5e/encounter` turns up only the method's own definition and
// type declarations. So [character.Character.InCombat] stays permanently
// true from a member's FIRST-EVER combat turn in a session onward, and
// [readyForTurn]'s `!sheet.InCombat()` branch — the one that unconditionally
// re-seeds via StartTurn — can never fire again for that character. Every
// later fight falls through to RefreshForTurn, which is a documented,
// deliberate no-op whenever the new fight's round number happens to equal
// whatever TurnNumber was left on the sheet — and since every bubble's own
// round counter starts fresh at 1 (play/clock's Turn is per-bubble, never
// global to the session), round 1 colliding with a stale round-1 economy
// from an EARLIER, unrelated fight is the common case, not a rare one.
//
// This suite reproduces exactly that: alice fought once already this
// session (seeded directly as stored data — the mechanics of how she got
// there are covered by the rest of this package's suites) and walked away
// with 5 of her 30 feet left, at round 1, with no ExitCombat ever called.
// She then free-roams toward carol's fight — already running, on its own
// round 1 — and is recruited mid-walk via the SAME wall-and-gap geometry
// fight_starts_test.go's ambushPath()/TestTheFightStopsTheWalk already
// prove opens contact on a specific cell with cells still to go.
type JoinMidwalkBudgetSuite struct {
	suite.Suite

	sessions   *fakeSessions
	encounters *fakeEncounters
	characters *fakeCharacters
	stream     *fakeStream
	mgr        *session.Manager
}

func TestJoinMidwalkBudgetSuite(t *testing.T) { suite.Run(t, new(JoinMidwalkBudgetSuite)) }

// alreadyFoughtOnceThisSession is armedFighter("alice"), carrying the sheet
// state an earlier, entirely separate fight in THIS SAME session would have
// left behind if it ended (by defeat, mid-turn — no EndTurn call, exactly
// as encounter/dissolve.go's ByDefeat documents: "the composition NOTICES
// [defeat], never something a caller declares") with nothing telling her
// sheet the fight was over: TurnNumber 1 (that fight's own round 1), 5 of
// her 30ft left, everything else spent for the turn.
func alreadyFoughtOnceThisSession() *character.Data {
	data := armedFighter("alice")
	data.ActionEconomy = &character.ActionEconomyData{
		TurnNumber:            1,
		ActionsRemaining:      0,
		BonusActionsRemaining: 0,
		ReactionsRemaining:    1,
		MovementRemaining:     5,
	}
	return data
}

func (s *JoinMidwalkBudgetSuite) SetupTest() {
	s.sessions, s.encounters = newFakeSessions(), newFakeEncounters()
	s.characters = newFakeCharacters(alreadyFoughtOnceThisSession(), armedFighter("carol"))
	s.stream = &fakeStream{}

	mgr, err := session.NewManager(&session.Config{
		Dice: testDice{}, TurnDriver: session.Pass{}, Sessions: s.sessions, Encounters: s.encounters,
		Characters: s.characters, Events: s.stream,
	})
	s.Require().NoError(err)
	s.mgr = mgr

	ctx := context.Background()
	_, err = mgr.StartSession(ctx, &session.StartSessionInput{
		Session: "sess", Encounter: "world", World: ambushWorld(s.T()),
	})
	s.Require().NoError(err)

	// carol arrives right beside the ogre — same side of the wall, no gap
	// needed — and starts THIS fight on her own, at ITS OWN round 1,
	// before alice's walk enters the picture at all.
	joined, err := mgr.Join(ctx, &session.JoinInput{
		Session: "sess", Member: "carol", Position: spatial.Position{X: 6, Y: 3},
	})
	s.Require().NoError(err)
	s.Require().NotNil(joined.Formed, "carol arrives beside the ogre and starts the fight herself")
	s.Require().Equal([]string{"carol", "ogre"}, joined.Formed.Order,
		"alice is still behind the wall, off the gap row, and must not be swept in yet")

	turn, err := mgr.Turn(ctx, &session.TurnInput{Session: "sess", Member: "carol"})
	s.Require().NoError(err)
	s.Require().Equal(session.ClockTurn, turn.Clock)
	s.Require().Equal("carol", turn.Active, "carol is first registered — first in initiative")
	s.Require().Equal(1, turn.Round, "control: this fight's own round is 1, same as alice's stale TurnNumber")

	confirmAlice, err := mgr.Turn(ctx, &session.TurnInput{Session: "sess", Member: "alice"})
	s.Require().NoError(err)
	s.Require().Equal(session.ClockWorld, confirmAlice.Clock, "control: alice is still free-roaming at setup")
}

func (s *JoinMidwalkBudgetSuite) afford(member string) *session.AffordOutput {
	s.T().Helper()
	out, err := s.mgr.Afford(context.Background(), &session.AffordInput{Session: "sess", Member: member})
	s.Require().NoError(err)
	return out
}

func (s *JoinMidwalkBudgetSuite) moveDecl(out *session.AffordOutput) session.Declaration {
	s.T().Helper()
	for _, d := range out.Declarations {
		if d.Verb == session.VerbMove {
			return d
		}
	}
	s.Require().Fail("no VerbMove declaration", "declarations: %+v", out.Declarations)
	return session.Declaration{}
}

// TestFreeRoamMemberJoiningMidWalkStartsFirstTurnWithFullSpeed is Kirk's live
// observation (2026-08-24): a member recruited into a running fight
// mid-walk must start THEIR OWN first turn in THAT fight with a fresh 30
// feet, never a number left over from a different fight entirely.
func (s *JoinMidwalkBudgetSuite) TestFreeRoamMemberJoiningMidWalkStartsFirstTurnWithFullSpeed() {
	ctx := context.Background()

	// alice free-roams ambushPath()[:2] — the SAME two-cell walk that lands
	// contact exactly on its last requested cell (fight_starts_test.go's
	// own "contact lands on the last cell" pattern), so the join succeeds
	// cleanly rather than being swallowed by the SEPARATE mid-walk-atomicity
	// defect this suite deliberately routes around (see the suite doc).
	_, err := s.mgr.Move(ctx, &session.MoveInput{Session: "sess", Member: "alice", Path: ambushPath()[:2]})
	s.Require().NoError(err, "the walk that carries alice into the fight must still succeed as a walk")

	// Confirm the join actually happened: alice is now on the turn clock,
	// inside the same fight as carol.
	turn, err := s.mgr.Turn(ctx, &session.TurnInput{Session: "sess", Member: "alice"})
	s.Require().NoError(err)
	s.Require().Equal(session.ClockTurn, turn.Clock, "alice's walk must have recruited her into carol's fight")

	// Drive to alice's own first turn in THIS fight: carol ends hers, the
	// Pass-driven ogre auto-resolves through EndTurn, and it becomes
	// alice's turn.
	ended, err := s.mgr.EndTurn(ctx, &session.EndTurnInput{Session: "sess", Member: "carol"})
	s.Require().NoError(err)
	s.Require().Equal("alice", ended.Next, "alice is next after carol and the auto-passed ogre")

	// THE ASSERTION: alice's first turn in THIS fight starts with her full
	// speed — not the 5ft her sheet has been carrying since a fight that
	// has nothing to do with this one.
	decl := s.moveDecl(s.afford("alice"))
	s.Require().NotNil(decl.Remaining, "Move's declaration always carries Remaining")
	s.Equal(30, *decl.Remaining,
		"alice's first turn in a NEW fight must start with a fresh 30ft, not a stale 5ft from an earlier one")
}
