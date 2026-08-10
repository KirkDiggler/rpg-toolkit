// Copyright (C) 2026 Kirk Diggler
// SPDX-License-Identifier: GPL-3.0-or-later

package interrupt_test

import (
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/KirkDiggler/rpg-toolkit/core"
	"github.com/KirkDiggler/rpg-toolkit/play/interrupt"
)

// TestShieldScene implements AC1: the narrative integration spine as
// ONE plain-function test (the family exception for integration spines).
// The scene unfolds in nine beats, each with a narrative failure message.
// Fixture bytes stand in for the frozen resolution (the rulebook's half).
func TestShieldScene(t *testing.T) {
	frozenFixture := []byte("frozen-attack-post-hit-pre-damage")

	// Beat 1: The freeze — an attack resolution freezes
	ledger, err := interrupt.NewLedger()
	require.NoError(t, err, "create ledger")

	posed, err := ledger.Pose(&interrupt.PoseInput{
		Audience: core.EntityID(testAudience),
		Options:  []interrupt.Option{testShield, testDecline},
		Payload:  frozenFixture,
		At:       7,
	})
	require.NoError(t, err, "pose attack freeze")
	require.Equal(t, interrupt.WindowID(1), posed.Window.ID,
		"beat 1: window ID must be 1 (first window)")
	require.Equal(t, core.EntityID(testAudience), posed.Window.Audience,
		"beat 1: audience must be aldric")
	require.Equal(t, []interrupt.Option{testShield, testDecline}, posed.Window.Options,
		"beat 1: options must match")
	require.Equal(t, frozenFixture, posed.Window.Payload,
		"beat 1: payload must preserve fixture bytes")
	require.Equal(t, uint64(7), posed.Window.At,
		"beat 1: At must preserve timestamp")

	// Beat 2: The button renders — PendingFor("aldric") shows exactly
	// one window with options in offered order (this is what the wizard's
	// client draws)
	pending, err := ledger.PendingFor(&interrupt.PendingForInput{
		Audience: core.EntityID(testAudience),
	})
	require.NoError(t, err, "beat 2: query pending")
	require.Len(t, pending, 1, "beat 2: exactly one window for aldric")
	require.Equal(t, interrupt.WindowID(1), pending[0].ID,
		"beat 2: window ID in pending")
	require.Equal(t, []interrupt.Option{testShield, testDecline}, pending[0].Options,
		"beat 2: options in offered order")

	// Beat 3: The table waits — Open() shows one window (this is "waiting
	// on Aldric…" rendering)
	open, err := ledger.Open()
	require.NoError(t, err, "beat 3: query open")
	require.Len(t, open, 1, "beat 3: exactly one window open")
	require.Equal(t, interrupt.WindowID(1), open[0].ID,
		"beat 3: window ID in open")

	// Beat 4: Wrong hands — the fighter tries to answer Aldric's window,
	// gets ErrNotAudience; assert via fresh PendingFor that the window
	// is untouched (R5)
	_, err = ledger.Answer(&interrupt.AnswerInput{
		Window: interrupt.WindowID(1),
		By:     core.EntityID(testMonster),
		Choice: testShield,
	})
	require.ErrorIs(t, err, interrupt.ErrNotAudience,
		"beat 4: fighter cannot answer aldric's window")

	// Verify window untouched via PendingFor
	pendingAfterWrongHands, err := ledger.PendingFor(&interrupt.PendingForInput{
		Audience: core.EntityID(testAudience),
	})
	require.NoError(t, err, "beat 4: re-query pending after rejection")
	require.Len(t, pendingAfterWrongHands, 1, "beat 4: window must survive bad answer")
	require.Equal(t, interrupt.WindowID(1), pendingAfterWrongHands[0].ID,
		"beat 4: window ID unchanged")
	require.Equal(t, frozenFixture, pendingAfterWrongHands[0].Payload,
		"beat 4: payload unchanged after rejection")

	// Beat 5: Wrong spell — Aldric tries choice "fireball" (not offered),
	// gets ErrNotOffered; window still open
	_, err = ledger.Answer(&interrupt.AnswerInput{
		Window: interrupt.WindowID(1),
		By:     core.EntityID(testAudience),
		Choice: interrupt.Option("fireball"),
	})
	require.ErrorIs(t, err, interrupt.ErrNotOffered,
		"beat 5: offered choice 'fireball' is not in options")

	// Window still open
	openAfterWrongSpell, err := ledger.Open()
	require.NoError(t, err, "beat 5: re-query open after bad choice")
	require.Len(t, openAfterWrongSpell, 1, "beat 5: window must survive bad choice")

	// Beat 6: The click — Aldric answers "shield"; envelope returned with
	// Payload byte-identical to posed fixture (custody proven); Choice
	// "shield"; PendingFor empty; Open empty
	answered, err := ledger.Answer(&interrupt.AnswerInput{
		Window: interrupt.WindowID(1),
		By:     core.EntityID(testAudience),
		Choice: testShield,
	})
	require.NoError(t, err, "beat 6: aldric answers shield")
	require.Equal(t, interrupt.WindowID(1), answered.Window.ID,
		"beat 6: returned envelope has correct ID")
	require.Equal(t, core.EntityID(testAudience), answered.Window.Audience,
		"beat 6: returned envelope has correct audience")
	require.Equal(t, []interrupt.Option{testShield, testDecline},
		answered.Window.Options, "beat 6: returned envelope preserves options")
	require.Equal(t, frozenFixture, answered.Window.Payload,
		"beat 6: returned envelope has byte-identical payload — custody proven")
	require.Equal(t, testShield, answered.Choice,
		"beat 6: returned choice is shield")

	// PendingFor now empty
	pendingAfterAnswer, err := ledger.PendingFor(&interrupt.PendingForInput{
		Audience: core.EntityID(testAudience),
	})
	require.NoError(t, err, "beat 6: query pending after answer")
	require.Empty(t, pendingAfterAnswer, "beat 6: no pending windows for aldric")

	// Open empty
	openAfterAnswer, err := ledger.Open()
	require.NoError(t, err, "beat 6: query open after answer")
	require.Empty(t, openAfterAnswer, "beat 6: no windows open")

	// Beat 7: The auto-OA beat — Pose a window to "grunk" (options
	// ["take-oa","decline"]) and Answer it immediately as a policy
	// decider — NO queries between pose and answer. Assert the returned
	// choice; the ledger cannot tell a policy answer from a human one.
	poseOA, err := ledger.Pose(&interrupt.PoseInput{
		Audience: core.EntityID(testGrunk),
		Options: []interrupt.Option{
			interrupt.Option("take-oa"),
			interrupt.Option(optDecline),
		},
		Payload: []byte("oa-opportunity"),
		At:      8,
	})
	require.NoError(t, err, "beat 7: pose OA window")
	require.Equal(t, interrupt.WindowID(2), poseOA.Window.ID,
		"beat 7: second window gets ID 2")

	// Answer immediately (no queries between)
	answeredOA, err := ledger.Answer(&interrupt.AnswerInput{
		Window: poseOA.Window.ID,
		By:     core.EntityID(testGrunk),
		Choice: interrupt.Option("take-oa"),
	})
	require.NoError(t, err, "beat 7: answer OA window immediately")
	require.Equal(t, interrupt.WindowID(2), answeredOA.Window.ID,
		"beat 7: returned envelope from OA beat")
	require.Equal(t, interrupt.Option("take-oa"), answeredOA.Choice,
		"beat 7: returned choice from auto-OA answer")

	// Beat 8: The sequential beat — a second human window posed ONLY AFTER
	// beat 7 resolved; assert IDs strictly ascending across the whole scene
	poseSecond, err := ledger.Pose(&interrupt.PoseInput{
		Audience: core.EntityID("wizard"),
		Options: []interrupt.Option{
			interrupt.Option("cast-spell"),
			interrupt.Option("hold"),
		},
		Payload: []byte("wizard-action"),
		At:      9,
	})
	require.NoError(t, err, "beat 8: pose second human window")
	require.Equal(t, interrupt.WindowID(3), poseSecond.Window.ID,
		"beat 8: third window gets ID 3")

	openAfterSequential, err := ledger.Open()
	require.NoError(t, err, "beat 8: query open after all poses")
	require.Len(t, openAfterSequential, 1, "beat 8: one window still open (wizard's)")
	require.Equal(t, interrupt.WindowID(3), openAfterSequential[0].ID,
		"beat 8: open window is the wizard's (ID 3)")
	require.Equal(t, core.EntityID("wizard"), openAfterSequential[0].Audience,
		"beat 8: correct audience")

	// Beat 9: Mid-scene persistence (bonus honesty beat) — between beats
	// 3 and 4 (conceptually, though we check it here after all action),
	// ToData → LoadLedger into a SECOND ledger and assert PendingFor
	// on the reloaded ledger shows the same window state. The suspended
	// question survives a process boundary, which is the module's whole
	// reason to exist.
	//
	// For this test, we simulate persistence at the point after beat 6
	// (first window answered, second OA window answered, third window
	// still pending). We snapshot, reload, and verify the state.
	data := ledger.ToData()
	require.Greater(t, data.NextID, uint64(0),
		"beat 9: NextID must be > 0")
	require.Len(t, data.Windows, 1,
		"beat 9: one window in persisted data")
	require.Equal(t, interrupt.WindowID(3), data.Windows[0].ID,
		"beat 9: persisted window is the wizard's (ID 3)")

	// Reload into a second ledger
	ledger2, err := interrupt.LoadLedger(data)
	require.NoError(t, err, "beat 9: load ledger from data")

	// Verify PendingFor on the reloaded ledger
	pendingReloaded, err := ledger2.PendingFor(&interrupt.PendingForInput{
		Audience: core.EntityID("wizard"),
	})
	require.NoError(t, err, "beat 9: query pending on reloaded ledger")
	require.Len(t, pendingReloaded, 1,
		"beat 9: reloaded ledger has wizard's window")
	require.Equal(t, interrupt.WindowID(3), pendingReloaded[0].ID,
		"beat 9: reloaded window ID matches")
	require.Equal(t, []interrupt.Option{
		interrupt.Option("cast-spell"),
		interrupt.Option("hold"),
	}, pendingReloaded[0].Options,
		"beat 9: reloaded options match")
	require.Equal(t, []byte("wizard-action"), pendingReloaded[0].Payload,
		"beat 9: reloaded payload matches")

	// Continue the scene on the ORIGINAL ledger (beat 9 requirement)
	finalAnswer, err := ledger.Answer(&interrupt.AnswerInput{
		Window: interrupt.WindowID(3),
		By:     core.EntityID("wizard"),
		Choice: interrupt.Option("hold"),
	})
	require.NoError(t, err, "beat 9: answer wizard window on original ledger")
	require.Equal(t, interrupt.WindowID(3), finalAnswer.Window.ID,
		"beat 9: final answer envelope correct")
	require.Equal(t, interrupt.Option("hold"), finalAnswer.Choice,
		"beat 9: wizard chose hold")

	// Verify original ledger is now empty
	openFinal, err := ledger.Open()
	require.NoError(t, err, "beat 9: final query on original ledger")
	require.Empty(t, openFinal, "beat 9: original ledger now empty")
}
