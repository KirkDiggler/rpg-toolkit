// Copyright (C) 2026 Kirk Diggler
// SPDX-License-Identifier: GPL-3.0-or-later

package session_test

// castle_gate_reserve_test.go is RESERVE BEFORE DICE at the session
// (rpg-project#924, gate slice 4 of #769): a checked line that pauses on its
// speaker's roll decision holds the front gate's conversation, not the
// table. While alice decides whether to spend her Guidance die on her claim
// to be a guest, the cleric beside her sees both claims pending and the
// lines that read no conversation open; a reply from an older revision is
// refused; the waiting roll survives a reload and settles once; and a
// waiting roll whose speaker walks out of reach is closed with nothing
// spent.

import (
	"context"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/character"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/conditions"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/encounter"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/refs"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/session"
	"github.com/KirkDiggler/rpg-toolkit/tools/spatial"
)

// besideAlice is where the cleric stands while alice talks at the gate:
// beside the hail cell, within Speak reach of the captain, in his sight.
var besideAlice = spatial.Position{X: -7, Y: 10}

// withGuidance is a sheet already holding a Guidance die, so a check it
// makes poses instead of settling.
func withGuidance(sheet *character.Data) *character.Data {
	condition, err := conditions.NewGuidedCondition(conditions.NewGuidedConditionInput{
		MemberID: sheet.ID, SourceID: "cleric-1", SourceRef: refs.Spells.Guidance(),
	})
	if err != nil {
		panic(err)
	}
	raw, err := condition.ToJSON()
	if err != nil {
		panic(err)
	}
	sheet.Conditions = append(sheet.Conditions, raw)
	return sheet
}

// holdsGuidance is whether the member's saved sheet still holds the
// Guidance die: the held benefit a refused answer must not spend.
func (c *freeRoamCastle) holdsGuidance(t *testing.T, member string) bool {
	t.Helper()
	sheet, err := c.characters.GetCharacter(context.Background(), member)
	require.NoError(t, err)
	held, err := conditions.HoldsRef(sheet.Conditions, refs.Conditions.Guided())
	require.NoError(t, err)
	return held
}

// atTheGateWithAWaitingClaim opens the castle for a guided alice and the
// cleric, walks alice up until the captain hails her, stands the cleric
// beside her, and has alice claim to be a guest: the check pauses on her
// Guidance die. It returns the cleric's guest row as read BEFORE the
// claim, so a test can reply from that older menu.
func atTheGateWithAWaitingClaim(t *testing.T) (*freeRoamCastle, session.SpeakOptionRow) {
	t.Helper()
	c := openTheCastleFor(t, []*character.Data{armedFighter("alice"), healingCleric()}, 8)
	// THE DIE IS GIVEN AFTER THE LAUNCH, whose long rest would clear it.
	withGuidance(c.characters.byID["alice"])
	c.placeInTheRecord(t, "cleric", besideAlice)
	_, out, _ := c.walkUpToTheGate(t)
	require.NotNil(t, out.Addressed, "precondition: the captain hailed her")

	before := lineFor(t, c, "cleric", "guest")
	require.True(t, before.Available, "precondition: the cleric may claim to be a guest")
	require.NotZero(t, before.Revision, "precondition: the claim reads the front gate")
	require.Equal(t, "front-gate", before.Interaction)

	claim, err := speakAsOffered(context.Background(), c.mgr, &session.SpeakInput{
		Session: "sess", Member: "alice", Target: "captain", OptionID: "guest",
	})
	require.NoError(t, err)
	require.True(t, claim.Paused, "precondition: her Guidance die stops the check to ask")
	return c, before
}

// lineFor is one line on the member's Speak row, said to the captain.
func lineFor(t *testing.T, c *freeRoamCastle, member, option string) session.SpeakOptionRow {
	t.Helper()
	return lineOn(t, c.bar(t, member), option)
}

// lineOn is one line to the captain on a panel already read.
func lineOn(t *testing.T, panel *session.AffordOutput, option string) session.SpeakOptionRow {
	t.Helper()
	for _, row := range speakRow(t, panel).SpeakOptions {
		if row.Target == "captain" && row.OptionID == option {
			return row
		}
	}
	require.FailNowf(t, "line not listed", "no %q for the captain", option)
	return session.SpeakOptionRow{}
}

// reactOn is the panel's REACT row, or the zero row when none is listed.
func reactOn(panel *session.AffordOutput) session.Declaration {
	for _, d := range panel.Declarations {
		if d.Verb == session.VerbReact {
			return d
		}
	}
	return session.Declaration{}
}

// reactRow is the member's REACT row, or the zero row when none is listed.
func reactRow(c *freeRoamCastle, t *testing.T, member string) session.Declaration {
	t.Helper()
	return reactOn(c.bar(t, member))
}

// TestCastleAWaitingClaimHoldsTheGateNotTheTable: while alice's claim waits
// on her roll, the cleric's panel is the whole panel. Both claims read the
// front gate and are pending on alice; the lines that read no conversation
// are open. Sending the pending claim is refused [session.ErrLineReserved]
// with no die thrown and no beat written, and a line that reads no
// conversation is spoken.
func TestCastleAWaitingClaimHoldsTheGateNotTheTable(t *testing.T) {
	c, _ := atTheGateWithAWaitingClaim(t)

	require.NotEmpty(t, reactRow(c, t, "alice").ID, "alice is asked about her die")
	panel := c.bar(t, "cleric")
	require.Empty(t, reactOn(panel).ID, "the question is hers alone")

	guest := lineOn(t, panel, "guest")
	require.True(t, guest.Pending, "the guest claim waits on alice's roll")
	require.False(t, guest.Available)
	require.NotNil(t, guest.Why)
	require.Equal(t, session.ShortfallLineReserved, guest.Why.Reason)
	require.Equal(t, "alice", guest.PendingOn, "the cleric sees alice, so she is told who it waits on")

	greet := lineOn(t, panel, "greet")
	require.True(t, greet.Available, "a line that reads no conversation stays open")
	require.False(t, greet.Pending)
	aside := lineOn(t, panel, "stand-aside")
	require.True(t, aside.Available, "a checked line that reads no conversation stays open")
	require.False(t, aside.Pending)

	checks := c.dice.checks
	heard := storyBeats(t, c.mgr, "sess", "cleric")
	_, err := c.mgr.Speak(context.Background(), &session.SpeakInput{
		Session: "sess", Member: "cleric", Target: "captain", OptionID: "guest", Revision: guest.Revision,
	})
	require.ErrorIs(t, err, session.ErrLineReserved)
	require.Equal(t, checks, c.dice.checks, "no die was thrown")
	require.Equal(t, heard, storyBeats(t, c.mgr, "sess", "cleric"), "no beat was written")

	_, err = speakAsOffered(context.Background(), c.mgr, &session.SpeakInput{
		Session: "sess", Member: "cleric", Target: "captain", OptionID: "greet",
	})
	require.NoError(t, err, "the table is not frozen: the greeting is said")
}

// TestCastleAReplyFromAnOlderMenuIsRefused: the cleric read her menu before
// alice's claim reserved the gate. Her reply at that revision is refused
// [session.ErrStaleOption] — never re-read against the gate as it is now —
// with no die thrown and no beat written.
func TestCastleAReplyFromAnOlderMenuIsRefused(t *testing.T) {
	c, before := atTheGateWithAWaitingClaim(t)

	checks := c.dice.checks
	heard := storyBeats(t, c.mgr, "sess", "cleric")
	_, err := c.mgr.Speak(context.Background(), &session.SpeakInput{
		Session: "sess", Member: "cleric", Target: "captain", OptionID: "guest", Revision: before.Revision,
	})
	require.ErrorIs(t, err, session.ErrStaleOption)
	require.Equal(t, checks, c.dice.checks, "no die was thrown")
	require.Equal(t, heard, storyBeats(t, c.mgr, "sess", "cleric"), "no beat was written")
}

// TestCastleAWaitingClaimSurvivesAReloadAndSettlesOnce: the host reloads
// mid-check. The window and the hold both come back; alice cannot say her
// claim a second time; her answer settles the claim once, spending her die
// and moving the gate; and a second answer finds no question.
func TestCastleAWaitingClaimSurvivesAReloadAndSettlesOnce(t *testing.T) {
	c, _ := atTheGateWithAWaitingClaim(t)

	c.mgr = c.manager(t)
	panel := c.bar(t, "alice")
	react := reactOn(panel)
	require.NotEmpty(t, react.ID, "the question survives the reload")
	mine := lineOn(t, panel, "guest")
	require.True(t, mine.Pending, "so does the hold, on her own claim too")

	_, err := c.mgr.Speak(context.Background(), &session.SpeakInput{
		Session: "sess", Member: "alice", Target: "captain", OptionID: "guest", Revision: mine.Revision,
	})
	require.ErrorIs(t, err, session.ErrLineReserved, "a second attempt at the waiting claim is refused")

	_, err = c.mgr.React(context.Background(), &session.ReactInput{
		Session: "sess", Member: "alice", DeclarationID: react.ID, Answer: session.Take(""),
	})
	require.NoError(t, err)
	require.False(t, c.holdsGuidance(t, "alice"), "the die was spent on the claim")
	require.NotContains(t, frontGateProgress(t, c.world(t)), "fresh", "the claim moved the gate")
	require.False(t, lineFor(t, c, "cleric", "greet").Pending, "the hold is settled")

	_, err = c.mgr.React(context.Background(), &session.ReactInput{
		Session: "sess", Member: "alice", DeclarationID: react.ID, Answer: session.Take(""),
	})
	require.ErrorIs(t, err, session.ErrNoWindow, "the question was answered once")
}

// TestCastleAWaitingClaimWalkedAwayFromIsClosedWithNothingSpent: alice walks
// back down the road out of the captain's reach with her claim waiting. The
// encounter lets the gate go, and the walk that let it go closes her
// question: the cleric's claims are open again, alice's late answer is
// refused, and her Guidance die is still hers.
func TestCastleAWaitingClaimWalkedAwayFromIsClosedWithNothingSpent(t *testing.T) {
	c, _ := atTheGateWithAWaitingClaim(t)
	react := reactRow(c, t, "alice")
	require.NotEmpty(t, react.ID, "precondition: she is asked")

	c.move(t, backDownTheRoad)

	require.Empty(t, reactRow(c, t, "alice").ID, "the question is gone with the hold")
	guest := lineFor(t, c, "cleric", "guest")
	require.False(t, guest.Pending, "the gate is free again")
	require.True(t, guest.Available)
	require.Contains(t, frontGateProgress(t, c.world(t)), "fresh", "nothing was settled")

	_, err := c.mgr.React(context.Background(), &session.ReactInput{
		Session: "sess", Member: "alice", DeclarationID: react.ID, Answer: session.Take(""),
	})
	require.ErrorIs(t, err, session.ErrNoWindow, "the late answer finds no question")
	require.True(t, c.holdsGuidance(t, "alice"), "and her die was never spent")
}

// TestCastleAnAnswerChecksTheHoldBeforeItSpends: the gate's hold is let go
// in the stored world behind the session's back — no verb of the session's
// closed the window. Alice's answer asks whether the hold still stands
// BEFORE it resumes the check: it is refused [session.ErrStaleOption], the
// question is closed, the gate does not move, and her die is not spent.
func TestCastleAnAnswerChecksTheHoldBeforeItSpends(t *testing.T) {
	c, _ := atTheGateWithAWaitingClaim(t)
	react := reactRow(c, t, "alice")
	require.NotEmpty(t, react.ID, "precondition: she is asked")

	enc := c.world(t)
	_, err := enc.ReleaseReservation(&encounter.ReleaseReservationInput{Member: "alice"})
	require.NoError(t, err)
	released := enc.ToData()
	c.encounters.byID["sess"] = &released

	_, err = c.mgr.React(context.Background(), &session.ReactInput{
		Session: "sess", Member: "alice", DeclarationID: react.ID, Answer: session.Take(""),
	})
	require.ErrorIs(t, err, session.ErrStaleOption, "the answer is told the conversation moved on")
	require.True(t, c.holdsGuidance(t, "alice"), "her die was not spent")
	require.Contains(t, frontGateProgress(t, c.world(t)), "fresh", "the gate did not move")

	_, err = c.mgr.React(context.Background(), &session.ReactInput{
		Session: "sess", Member: "alice", DeclarationID: react.ID, Answer: session.Take(""),
	})
	require.ErrorIs(t, err, session.ErrNoWindow, "the question was closed")
}
