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
	"encoding/json"
	"flag"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"slices"
	"strings"
	"sync"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/character"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/conditions"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/encounter"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/encounter/dungeonspec"
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
		MemberID: sheet.ID, SourceID: "cleric", SourceRef: refs.Spells.Guidance(),
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

// waitingClaim is the stored state of [atTheGateWithAWaitingClaim]'s
// scene, built ONCE per test binary — in [TestMain], so no test's clock
// pays for it — and restored into fresh stores for every test that starts
// from it: the castle's launch is the expensive part of every castle test,
// and these tests all start from the same moment.
var waitingClaim struct {
	once       sync.Once
	err        error
	sessions   map[string]*session.SessionData
	encounters map[string]*encounter.EncounterData
	characters map[string]*character.Data
	seats      map[string]session.SeatData
	before     session.SpeakOptionRow
}

// waitingClaimTests are the tests that start from the waiting claim, so
// [TestMain] builds it only for a run that will use it.
var waitingClaimTests = []string{
	"TestCastleAReplyFromAnOlderMenuIsRefused",
	"TestCastleAWaitingClaimHoldsTheGateNotTheTable",
	"TestCastleAWaitingClaimSurvivesAReloadAndSettlesOnce",
	"TestCastleAWaitingClaimWalkedAwayFromIsKept",
	"TestCastleAnAnswerChecksTheHoldBeforeItSpends",
	"TestCastleAnAnswerAfterTheCaptainLeftReachKeepsTheRoll",
	"TestCastleWhileHerRollWaitsSheMayOnlyWalk",
	"TestCastleNoVerbClosesTheRunUnderAWaitingRoll",
	"TestCastleAFightFormingUnderAWaitingClaimFreezesThenSettles",
	"TestCastleTwoConversationsHeldAtOnceBothSettle",
}

// TestMain builds the waiting claim's scene before any test runs, when the
// run's -run pattern selects one of [waitingClaimTests]: the CI test budget
// is judged per top-level test, and a build inside the first test to need
// it would put the castle's launch on that one test's clock. A pattern this
// cannot read builds anyway; a run that skips the build and needs it after
// all builds it lazily ([atTheGateWithAWaitingClaim]).
func TestMain(m *testing.M) {
	flag.Parse()
	pattern := flag.Lookup("test.run").Value.String()
	if wanted(pattern) {
		waitingClaim.once.Do(func() { waitingClaim.err = buildTheWaitingClaim() })
	}
	if wantedBy(pattern, admittedGateTests) {
		admittedGate.once.Do(func() { admittedGate.err = buildTheAdmittedGate() })
	}
	os.Exit(m.Run())
}

// wanted reports whether a -run pattern can select a waiting-claim test.
func wanted(pattern string) bool {
	return wantedBy(pattern, waitingClaimTests)
}

// wantedBy reports whether a -run pattern can select one of tests.
func wantedBy(pattern string, tests []string) bool {
	if pattern == "" {
		return true
	}
	top := strings.SplitN(pattern, "/", 2)[0]
	re, err := regexp.Compile(top)
	if err != nil {
		return true
	}
	return slices.ContainsFunc(tests, re.MatchString)
}

// atTheGateWithAWaitingClaim is the castle (with a second conversation,
// [withASecondConversation]) for a guided alice and the cleric: alice where
// the captain's hail stops her, the cleric beside her, and alice's claim to
// be a guest paused on her Guidance die. It returns the cleric's guest row
// as read BEFORE the claim, so a test can reply from that older menu. Every
// call gets its own copy of the stores, and dice that throw 8 on every d20.
func atTheGateWithAWaitingClaim(t *testing.T) (*freeRoamCastle, session.SpeakOptionRow) {
	t.Helper()
	waitingClaim.once.Do(func() { waitingClaim.err = buildTheWaitingClaim() })
	require.NoError(t, waitingClaim.err, "the waiting claim's scene")

	c := &freeRoamCastle{
		live: &actCapture{}, dice: &castleDice{d20s: []int{8}},
		sessions: newFakeSessions(), encounters: newFakeEncounters(),
		characters: newFakeCharacters(), seats: newFakeSeats(),
	}
	for id, data := range waitingClaim.sessions {
		copied, err := copyOf(data)
		require.NoError(t, err)
		c.sessions.byID[id] = copied
	}
	for id, data := range waitingClaim.encounters {
		copied, err := copyOf(data)
		require.NoError(t, err)
		c.encounters.byID[id] = copied
	}
	for id, data := range waitingClaim.characters {
		copied, err := copyOf(data)
		require.NoError(t, err)
		c.characters.byID[id] = copied
	}
	for id, data := range waitingClaim.seats {
		c.seats.byID[id] = data
	}
	c.mgr = c.manager(t)
	return c, waitingClaim.before
}

// buildTheWaitingClaim plays the waiting claim's scene once and keeps its
// stores ([waitingClaim]). It takes no *testing.T because [TestMain] runs it
// before any test exists; every precondition it checks is an error.
func buildTheWaitingClaim() error {
	ctx := context.Background()
	dir, err := exec.Command("go", "list", "-m", "-f", "{{.Dir}}", encounterModule).Output()
	if err != nil {
		return fmt.Errorf("find the encounter module: %w", err)
	}
	raw, err := os.ReadFile(filepath.Join(strings.TrimSpace(string(dir)), castleFixture))
	if err != nil {
		return err
	}
	raw, err = withASecondConversation(raw)
	if err != nil {
		return err
	}
	compiled, err := dungeonspec.Load(raw)
	if err != nil {
		return err
	}

	c := &freeRoamCastle{
		live: &actCapture{}, dice: &castleDice{d20s: []int{8}},
		characters: newFakeCharacters(armedFighter("alice"), healingCleric()),
		encounters: newFakeEncounters(), sessions: newFakeSessions(), seats: newFakeSeats(),
	}
	c.mgr, err = session.NewManager(&session.Config{Record: session.DiscardRecord{}, Acts: c.live, Seats: c.seats, CharacterConfig: newFakeCharacterConfig(),
		PresentationIDs: testPresentationIDs{}, Dice: c.dice, TurnDriver: session.Driver(),
		Sessions: c.sessions, Encounters: c.encounters, Characters: c.characters,
	})
	if err != nil {
		return err
	}
	if _, err := c.mgr.Launch(ctx, &session.LaunchInput{Session: "sess", Dungeon: &compiled, Party: []string{"alice", "cleric"}}); err != nil {
		return fmt.Errorf("launch: %w", err)
	}
	// THE DIE IS GIVEN AFTER THE LAUNCH, whose long rest would clear it.
	withGuidance(c.characters.byID["alice"])
	// ONE STEP ONTO THE CELL WHERE THE CAPTAIN'S HAIL STOPS HER, rather than
	// the whole road: the walk is the hail tests' subject, and the slowest
	// part of the scene. The step is what has the room see the two of them.
	world := c.encounters.byID["sess"]
	for i := range world.Members {
		switch world.Members[i].ID {
		case "alice":
			world.Members[i].Cell = &encounter.PositionData{X: backDownTheRoad[0].X, Y: backDownTheRoad[0].Y}
		case "cleric":
			world.Members[i].Cell = &encounter.PositionData{X: besideAlice.X, Y: besideAlice.Y}
		}
	}
	if _, err := walkOn(ctx, c.mgr, &session.MoveInput{Session: "sess", Member: "alice", Path: []spatial.Position{theHailCell}}); err != nil {
		return fmt.Errorf("the step to the hail cell: %w", err)
	}

	panel, err := c.mgr.Afford(ctx, &session.AffordInput{Session: "sess", Member: "cleric"})
	if err != nil {
		return err
	}
	var before session.SpeakOptionRow
	for _, d := range panel.Declarations {
		for _, row := range d.SpeakOptions {
			if row.Target == "captain" && row.OptionID == "guest" {
				before = row
			}
		}
	}
	if !before.Available || before.Revision == 0 || before.Interaction != "front-gate" {
		return fmt.Errorf("precondition: the cleric may claim to be a guest, on the front gate: %+v", before)
	}

	claim, err := speakAsOffered(ctx, c.mgr, &session.SpeakInput{
		Session: "sess", Member: "alice", Target: "captain", OptionID: "guest",
	})
	if err != nil {
		return fmt.Errorf("alice's claim: %w", err)
	}
	if !claim.Paused {
		return fmt.Errorf("precondition: her Guidance die stops the check to ask: %+v", claim)
	}

	waitingClaim.sessions = c.sessions.byID
	waitingClaim.encounters = c.encounters.byID
	waitingClaim.characters = c.characters.byID
	waitingClaim.seats = c.seats.byID
	waitingClaim.before = before
	return nil
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

// TestCastleAWaitingClaimSurvivesAReloadAndSettlesOnce: the host reloads
// mid-check. The window and the hold both come back; alice cannot say her
// claim a second time; her answer settles the claim once, spending her die
// and moving the gate; and a second answer finds no question.
func TestCastleAWaitingClaimSurvivesAReloadAndSettlesOnce(t *testing.T) {
	c, _ := atTheGateWithAWaitingClaim(t)

	c.mgr = c.manager(t)
	react := reactRow(c, t, "alice")
	require.NotEmpty(t, react.ID, "the question survives the reload")
	require.True(t, lineFor(t, c, "cleric", "guest").Pending, "so does the hold")

	_, err := speakAsOffered(context.Background(), c.mgr, &session.SpeakInput{
		Session: "sess", Member: "alice", Target: "captain", OptionID: "guest",
	})
	require.ErrorIs(t, err, session.ErrWindowOpen, "a second attempt while her roll waits is refused")

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

// TestCastleAWaitingClaimWalkedAwayFromIsKept: alice walks back down the
// road out of the captain's reach with her claim waiting. A roll is kept
// (rpg-project#938) — this test pinned "nothing spent and no verdict" before,
// and now pins the ruling: the walk that took her out of reach settles her
// claim as rolled, declined, in the same commit. Her eight misses, the gate
// is warned, her question and the hold are gone, alice's late answer finds
// no question, and her Guidance die is still hers.
func TestCastleAWaitingClaimWalkedAwayFromIsKept(t *testing.T) {
	c, _ := atTheGateWithAWaitingClaim(t)
	react := reactRow(c, t, "alice")
	require.NotEmpty(t, react.ID, "precondition: she is asked")

	c.move(t, backDownTheRoad)

	require.Empty(t, reactRow(c, t, "alice").ID, "the question is settled with the hold")
	require.Zero(t, c.reservations(t), "the gate is held no longer")
	require.Contains(t, frontGateProgress(t, c.world(t)), "warned-guest", "the claim landed as rolled: a miss")

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

// TestCastleWhileHerRollWaitsSheMayOnlyWalk: while alice's claim waits on
// her die, she has one question and nothing else to declare. Every other
// verb of hers — a second claim, a plain greeting — is refused
// [session.ErrWindowOpen] before anything is charged or rolled, so no
// second question can stack on the first. She may walk, and a step that
// keeps her within reach keeps the hold.
func TestCastleWhileHerRollWaitsSheMayOnlyWalk(t *testing.T) {
	c, _ := atTheGateWithAWaitingClaim(t)

	panel := c.bar(t, "alice")
	speak := speakRow(t, panel)
	require.False(t, speak.Available, "her Speak row is blocked while her roll waits")
	require.NotNil(t, speak.Why)
	require.Equal(t, session.ShortfallWindowOpen, speak.Why.Reason)
	require.NotEmpty(t, reactOn(panel).ID, "her question is on her panel")

	checks := c.dice.checks
	for _, line := range []string{"stand-aside", "greet"} {
		_, err := speakAsOffered(context.Background(), c.mgr, &session.SpeakInput{
			Session: "sess", Member: "alice", Target: "captain", OptionID: line,
		})
		require.ErrorIs(t, err, session.ErrWindowOpen, "%s: refused while her roll waits", line)
	}
	require.Equal(t, checks, c.dice.checks, "no die was thrown")

	c.move(t, backDownTheRoad[:1])
	require.Equal(t, backDownTheRoad[0], c.where(t, "alice"), "she may walk")
	require.True(t, lineFor(t, c, "cleric", "guest").Pending, "a step within reach keeps the hold")
	require.Equal(t, reactOn(panel).ID, reactRow(c, t, "alice").ID, "and the one question stands")
}

// TestCastleAFightFormingUnderAWaitingClaimFreezesThenSettles: the
// cleric's threat fails while alice's claim waits, and the garrison falls on
// the party. The clock changes under the question: alice is on the turn
// clock now, so her waiting claim freezes the table as every question in a
// fight does — the cleric's next line is refused [session.ErrWindowOpen] —
// and her answer settles the claim, spending her die. No question is left
// open.
func TestCastleAFightFormingUnderAWaitingClaimFreezesThenSettles(t *testing.T) {
	c, _ := atTheGateWithAWaitingClaim(t)
	react := reactRow(c, t, "alice")
	require.NotEmpty(t, react.ID, "precondition: she is asked")

	c.dice.mu.Lock()
	c.dice.d20s = []int{2} // the cleric's threat fails; the fight's own dice follow
	c.dice.mu.Unlock()
	_, err := speakAsOffered(context.Background(), c.mgr, &session.SpeakInput{
		Session: "sess", Member: "cleric", Target: "captain", OptionID: "stand-aside",
	})
	require.NoError(t, err)
	turn, err := c.mgr.Turn(context.Background(), &session.TurnInput{Session: "sess", Member: "alice"})
	require.NoError(t, err)
	require.Equal(t, session.ClockTurn, turn.Clock, "precondition: the threat brought the fight")

	_, err = speakAsOffered(context.Background(), c.mgr, &session.SpeakInput{
		Session: "sess", Member: "cleric", Target: "captain", OptionID: "greet",
	})
	require.ErrorIs(t, err, session.ErrWindowOpen, "in a fight her question holds the table")

	_, err = c.mgr.React(context.Background(), &session.ReactInput{
		Session: "sess", Member: "alice", DeclarationID: react.ID, Answer: session.Take(""),
	})
	require.NoError(t, err, "her answer settles the claim")
	require.False(t, c.holdsGuidance(t, "alice"), "and spends the die")
	require.Empty(t, reactRow(c, t, "alice").ID, "no question is left open")
}

// withASecondConversation is the castle with a second interaction,
// `side-talk`, and a checked line on the captain that reads it: two
// conversations at one creature, so two speakers can each hold one.
func withASecondConversation(castle []byte) ([]byte, error) {
	raw := string(castle)
	const greet = "          - id: greet\n            say: Good evening, captain.\n"
	const interactions = "interactions:\n"
	if strings.Count(raw, greet) != 1 || strings.Count(raw, interactions) != 1 {
		return nil, fmt.Errorf("the castle no longer authors one captain's greeting and one interactions block")
	}
	raw = strings.Replace(raw, greet, `          - id: rumour
            say: Heard anything odd tonight?
            check:
              - ability: persuasion
                dc: 11
            requires:
              - interaction:
                  id: side-talk
                  in: [idle]
            advance:
              interaction: side-talk
              success: told
              failure: refused
            success:
              - say: Only the usual.
            failure:
              - say: Move along.
`+greet, 1)
	return []byte(strings.Replace(raw, interactions, `interactions:
  side-talk:
    recipe:
      ref: site:interactions:side-talk
      initial: idle
      states:
        idle:
          transitions:
            - { on: told, to: told, reason: the rumour was told }
            - { on: refused, to: told, reason: the rumour was refused }
        told: { terminal: true }
`, 1)), nil
}

// TestCastleTwoConversationsHeldAtOnceBothSettle: alice's guest claim holds
// the front gate while the cleric's rumour holds the side talk, two
// waiting rolls on two conversations at once. Neither is pending on the
// other, and each answer settles its own line.
func TestCastleTwoConversationsHeldAtOnceBothSettle(t *testing.T) {
	c, _ := atTheGateWithAWaitingClaim(t)
	withGuidance(c.characters.byID["cleric"])

	rumour := lineFor(t, c, "cleric", "rumour")
	require.False(t, rumour.Pending, "the side talk is not the gate's to hold")
	require.Equal(t, "side-talk", rumour.Interaction)
	asked, err := c.mgr.Speak(context.Background(), &session.SpeakInput{
		Session: "sess", Member: "cleric", Target: "captain", OptionID: "rumour", Revision: rumour.Revision,
	})
	require.NoError(t, err)
	require.True(t, asked.Paused, "precondition: the cleric's rumour waits too")

	for _, member := range []string{"alice", "cleric"} {
		react := reactRow(c, t, member)
		require.NotEmpty(t, react.ID, "%s is asked", member)
		_, err := c.mgr.React(context.Background(), &session.ReactInput{
			Session: "sess", Member: member, DeclarationID: react.ID, Answer: session.Take(""),
		})
		require.NoError(t, err, "%s's answer settles", member)
		require.False(t, c.holdsGuidance(t, member), "%s spent the die", member)
	}
	require.NotContains(t, frontGateProgress(t, c.world(t)), "fresh", "the gate moved")
}

// TestCastleNoVerbClosesTheRunUnderAWaitingRoll: End and Dissolve act on
// the whole table, so while alice's claim waits they wait on her too
// ([session.ErrWindowOpen]) — an ended run would hold a question nobody
// could answer. Her answer is still hers to give after.
func TestCastleNoVerbClosesTheRunUnderAWaitingRoll(t *testing.T) {
	c, _ := atTheGateWithAWaitingClaim(t)
	ctx := context.Background()

	_, err := c.mgr.End(ctx, &session.EndInput{Session: "sess", Ending: "withdrawn"})
	require.ErrorIs(t, err, session.ErrWindowOpen, "End waits on her roll")
	_, err = c.mgr.Dissolve(ctx, &session.DissolveInput{Session: "sess", Member: "cleric", Cause: session.ByDecision()})
	require.ErrorIs(t, err, session.ErrWindowOpen, "Dissolve waits on her roll")

	react := reactRow(c, t, "alice")
	require.NotEmpty(t, react.ID, "her question stands")
	_, err = c.mgr.React(ctx, &session.ReactInput{Session: "sess", Member: "alice", DeclarationID: react.ID, Answer: session.Take("")})
	require.NoError(t, err, "and her answer settles it")
}

// TestCastleAnAnswerAfterTheCaptainLeftReachKeepsTheRoll: the captain is
// moved beyond Speak reach in the stored world, with no verb of the
// session's and no participation pass to notice, so the encounter's hold
// still stands, unabandoned, while its settle would be refused. Alice's
// answer asks the settle's own doors first and finds them out of reach: a
// roll is kept (rpg-project#938) — this test pinned a release before — so
// the session abandons the hold and settles it declined whatever she
// answered. She is told [session.ReactOutput.RollKept], her die is not
// spent, the claim lands as rolled, and no hold is released.
func TestCastleAnAnswerAfterTheCaptainLeftReachKeepsTheRoll(t *testing.T) {
	c, _ := atTheGateWithAWaitingClaim(t)
	react := reactRow(c, t, "alice")
	require.NotEmpty(t, react.ID, "precondition: she is asked")
	c.placeInTheRecord(t, "captain", spatial.Position{X: -4, Y: 2})
	heldBefore := heldReleases(t, c)

	out, err := c.mgr.React(context.Background(), &session.ReactInput{
		Session: "sess", Member: "alice", DeclarationID: react.ID, Answer: session.Take(""),
	})
	require.NoError(t, err)
	require.True(t, out.RollKept, "she is told her roll was kept")
	require.True(t, c.holdsGuidance(t, "alice"), "her die was not spent")
	require.Empty(t, reactRow(c, t, "alice").ID, "her question is gone")
	require.Equal(t, heldBefore, heldReleases(t, c), "nothing was released")
	require.Contains(t, frontGateProgress(t, c.world(t)), "warned-guest", "the claim landed as rolled")
}

// heldReleases counts the `interaction-held` beats with `held: false` alice
// has witnessed.
func heldReleases(t *testing.T, c *freeRoamCastle) int {
	t.Helper()
	entries, err := c.world(t).Story(&encounter.StoryInput{Audience: "alice"})
	require.NoError(t, err)
	n := 0
	for _, e := range entries {
		var body struct {
			Beat string `json:"beat"`
			Held bool   `json:"held"`
		}
		if json.Unmarshal(e.Payload, &body) == nil && body.Beat == encounter.BeatInteractionHeld && !body.Held {
			n++
		}
	}
	return n
}
