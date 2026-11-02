// Copyright (C) 2026 Kirk Diggler
// SPDX-License-Identifier: GPL-3.0-or-later

package session_test

// speak_plain_test.go is rpg-project#785 R5 on the turn clock: "the check is
// the action… speaking with no check would be a free action". A plain line
// (no `check:`) is offered on its own free row and spends nothing; a checked
// line is the action row it always was.

import (
	"context"

	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/encounter"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/encounter/dungeonspec"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/session"
)

// greetLine is a plain line: no check, so nothing is rolled and it lands.
func greetLine() encounter.Option {
	return encounter.Option{ID: "greet", Say: "Evening."}
}

// speakRows is every Speak row alice is offered, keyed by its slot.
func (s *SpeakSuite) speakRows(mgr *session.Manager) map[session.Slot]session.Declaration {
	out, err := mgr.Afford(context.Background(), &session.AffordInput{Session: "sess", Member: "alice"})
	s.Require().NoError(err)
	rows := map[session.Slot]session.Declaration{}
	for _, d := range out.Declarations {
		if d.Verb != session.VerbSpeak {
			continue
		}
		_, dup := rows[d.Slot]
		s.Require().False(dup, "one Speak row per price: %s twice", d.Slot)
		rows[d.Slot] = d
	}
	return rows
}

// optionIDs is a row's menu, by line id.
func optionIDs(row session.Declaration) []string {
	ids := make([]string, 0, len(row.SpeakOptions))
	for _, o := range row.SpeakOptions {
		ids = append(ids, o.OptionID)
	}
	return ids
}

// actionsLeft is alice's stored action budget. A sheet nothing has paid from
// yet is not readied and stores no economy: on alice's first turn that is the
// full one action.
func (s *SpeakSuite) actionsLeft() int {
	stored, err := s.characters.GetCharacter(context.Background(), "alice")
	s.Require().NoError(err)
	if stored.ActionEconomy == nil {
		return 1
	}
	return stored.ActionEconomy.ActionsRemaining
}

// aGreetableGoblin is the goblin carrying a threat and a greeting, standing
// beside alice so a swing reaches it.
func (s *SpeakSuite) aGreetableGoblin() {
	s.authored = func(in *dungeonspec.MonsterPlacement) {
		in.Options = []encounter.Option{threatLine(), greetLine()}
		in.At.X = 3
	}
}

// TWO ROWS, ONE PER PRICE. The greeting rides a free row and the threat the
// action row, each listing only its own lines and the goblin as a candidate.
func (s *SpeakSuite) TestTheMenuIsSplitByPrice() {
	s.aGreetableGoblin()
	mgr := s.aYard(nil)

	rows := s.speakRows(mgr)
	s.Require().Len(rows, 2, "a free row and an action row")

	free := rows[session.SlotNone]
	s.True(free.Available, "%+v", free.Why)
	s.Equal([]string{"greet"}, optionIDs(free), "the free row carries the plain line only")
	s.Require().Len(free.Candidates, 1)
	s.Equal("goblin", free.Candidates[0].Member)

	action := rows[session.SlotAction]
	s.True(action.Available, "%+v", action.Why)
	s.Equal([]string{"threaten"}, optionIDs(action), "the action row carries the checked line only")
	s.Require().Len(action.Candidates, 1)
	s.Equal("goblin", action.Candidates[0].Member)

	s.NotEqual(free.ID, action.ID, "two rows, two selectors")
}

// THE GREETING SPENDS NO ACTION: the fighter greets, then swings with the
// action the greeting did not take, then greets again with none left; the
// threat is refused for want of the action, and its row says so first.
func (s *SpeakSuite) TestAPlainLineSpendsNoActionOnTheTurnClock() {
	s.aGreetableGoblin()
	// Every die a 1: the greeting rolls no check, and the swing misses, so
	// the goblin stays standing to be spoken to and the fight stays formed.
	mgr := s.aYard([]int{1, 1, 1, 1, 1, 1})
	s.Require().Len(s.speakRows(mgr), 2)
	s.Require().Equal(1, s.actionsLeft())

	out, err := s.say(mgr, "greet")
	s.Require().NoError(err)
	s.True(out.Beaten, "a plain line rolls nothing, so there is nothing to miss")
	s.Zero(out.Total)
	s.Zero(out.DC)
	s.Empty(out.Applied.Ability, "no route was taken")
	s.Equal(1, s.actionsLeft(), "the greeting cost nothing")
	s.False(s.held(mgr, encounter.DeedIntimidate), "and lands no deed")

	body := s.lastSpoke(mgr, "alice")
	s.Require().NotNil(body, "the greeting is still a line the table heard")
	s.Equal("greet", body.OptionID)
	s.Empty(body.Skill)
	s.Nil(body.Calculation)
	s.True(body.Beaten)

	_, err = mgr.Attack(context.Background(), &session.AttackInput{
		Session: "sess", Attacker: "alice", Target: "goblin",
		DeclarationID: currentAttackID(s.T(), mgr, "sess", "alice"),
	})
	s.Require().NoError(err, "the action is still there to swing with")
	s.Equal(0, s.actionsLeft())

	rows := s.speakRows(mgr)
	s.True(rows[session.SlotNone].Available, "a greeting needs no action: %+v", rows[session.SlotNone].Why)
	action := rows[session.SlotAction]
	s.False(action.Available, "the threat needs the action the swing took")
	s.Require().NotNil(action.Why)
	s.Equal(session.ShortfallNoBudget, action.Why.Reason)

	_, err = s.threaten(mgr)
	s.Require().ErrorIs(err, session.ErrCannotAfford)

	_, err = s.say(mgr, "greet")
	s.Require().NoError(err, "and a second greeting is as free as the first")
}

// THE THREAT IS THE ACTION: after it, the swing is out of budget too.
func (s *SpeakSuite) TestACheckedLineSpendsTheActionOnTheTurnClock() {
	s.aGreetableGoblin()
	mgr := s.aYard([]int{10, 4, 4, 4, 4})
	s.Require().Len(s.speakRows(mgr), 2)

	out, err := s.threaten(mgr)
	s.Require().NoError(err)
	s.Equal("intimidation", out.Applied.Ability)
	s.Equal(0, s.actionsLeft(), "the check is the action")

	attack := currentDeclaration(s.T(), mgr, "sess", "alice", session.VerbAttack)
	s.False(attack.Available, "the threat took the action the swing needs")
}

// A TARGET WITH ONLY PLAIN LINES offers only the free row: there is no
// checked line to price, so there is no action row to grey out.
func (s *SpeakSuite) TestOnlyPlainLinesOfferOnlyTheFreeRow() {
	s.authored = func(in *dungeonspec.MonsterPlacement) {
		in.Options = []encounter.Option{greetLine()}
	}
	mgr := s.aYard(nil)

	rows := s.speakRows(mgr)
	s.Require().Len(rows, 1)
	free, ok := rows[session.SlotNone]
	s.Require().True(ok, "the one row is the free one")
	s.True(free.Available)
	s.Equal([]string{"greet"}, optionIDs(free))
}

// FREE IS NOT OUT OF TURN: on the turn clock a plain line is still the active
// member's, refused on another member's turn exactly as a checked one is
// (talking out of turn is a later ruling). The gate is asked before the line
// is looked up, so the refusal is the turn's and not a missing line's.
func (s *SpeakSuite) TestAPlainLineIsStillRefusedOffTurn() {
	s.aGreetableGoblin()
	mgr := s.aYard([]int{1, 1, 1, 1})
	// The goblin's turn, authored: the Pass driver hands an ended turn
	// straight back to alice, so endTurn cannot get here.
	authorTurnClock(s.T(), s.encounters, testSession, []string{"alice", "goblin"}, 1)

	_, err := s.say(mgr, "greet")
	s.Require().ErrorIs(err, session.ErrNotYourTurn)
}
