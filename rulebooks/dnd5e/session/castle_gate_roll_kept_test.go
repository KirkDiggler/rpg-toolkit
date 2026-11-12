// Copyright (C) 2026 Kirk Diggler
// SPDX-License-Identifier: GPL-3.0-or-later

package session_test

// castle_gate_roll_kept_test.go is a roll is kept (rpg-project#938, ruled
// 2026-11-12: "a roll is a roll that is kept") and one try at the
// straggler's claim (rpg-project#932), through the session, on the admitted
// gate's shared scene. The cleric arrives after the captain has gone in and
// makes her own case holding a Guidance die, so her roll waits on her; when
// she walks away the walk that took her out of reach settles the claim as
// rolled, the die unspent.

import (
	"context"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/KirkDiggler/rpg-toolkit/core"
	"github.com/KirkDiggler/rpg-toolkit/play/intel"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/encounter"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/session"
	"github.com/KirkDiggler/rpg-toolkit/tools/spatial"
)

// The gate guards' lines for a stranger after the admission (the castle,
// rpg-project#932), and what a guard holds about one he turned away.
const (
	friendsLine     = "friends-inside"
	turnedAwayTopic = "turned-away"
	admittedOn      = "admitted"
)

// holds is whether guard holds topic about member in the stored world.
func (c *freeRoamCastle) holds(t *testing.T, guard, member, topic string) bool {
	t.Helper()
	data := c.world(t).ToData()
	_, ok := data.Perception.Intel.Holdings[core.EntityID("member|"+guard)][intel.Subject("member|"+member)][intel.Topic(topic)]
	return ok
}

// reservations is how many interactions hold a waiting line in the stored
// world.
func (c *freeRoamCastle) reservations(t *testing.T) int {
	t.Helper()
	n := 0
	for _, data := range c.world(t).ToData().Interactions {
		if data.Reservation != nil {
			n++
		}
	}
	return n
}

// theClericsWaitingCase is the admitted gate with the cleric, holding a
// Guidance die, beside the gate making her own case, her check paused on
// the die; every d20 shows face.
func theClericsWaitingCase(t *testing.T, face int) *freeRoamCastle {
	t.Helper()
	c := atTheAdmittedGate(t)
	c.dice.d20s = []int{face}
	withGuidance(c.characters.byID["cleric"])
	c.standInTheRecord(t, "cleric", besideTheCastleGate)
	claim, err := speakAsOffered(context.Background(), c.mgr, &session.SpeakInput{
		Session: "sess", Member: "cleric", Target: "gate-1", OptionID: stragglerLine,
	})
	require.NoError(t, err)
	require.True(t, claim.Paused, "precondition: her Guidance die stops the check to ask")
	require.NotEmpty(t, reactRow(c, t, "cleric").ID, "precondition: she is asked")
	require.Equal(t, 1, c.reservations(t), "precondition: her case holds the gate")
	return c
}

// outOfReach is a cell on the road beyond Speak reach of the gate guards.
var outOfReach = backDownTheRoad[2]

// walkAway walks the cleric from the gate down the road, out of the gate
// guards' Speak reach.
func (c *freeRoamCastle) walkAway(t *testing.T) *session.MoveOutput {
	t.Helper()
	route, err := c.world(t).Route(encounter.RouteInput{Mover: "cleric", Policy: encounter.MoveToward, Anchor: outOfReach, Budget: 200})
	require.NoError(t, err)
	return c.walk(t, "cleric", append(route.Path, outOfReach))
}

// TestCastleAStragglerWhoWalksAwayFromABadRollIsTurnedAway: the total shown
// is below the mark and she walks away without answering. The same Move
// settles her case as rolled: turned away by both guards, the case gone, no
// question and no hold left, and her Guidance die still hers.
func TestCastleAStragglerWhoWalksAwayFromABadRollIsTurnedAway(t *testing.T) {
	c := theClericsWaitingCase(t, 2)

	c.walkAway(t)
	require.Empty(t, reactRow(c, t, "cleric").ID, "no question remains")
	require.Zero(t, c.reservations(t), "and no hold")
	require.True(t, c.holds(t, "gate-1", "cleric", turnedAwayTopic), "the miss lands: gate-1 turned her away")
	require.True(t, c.holds(t, "gate-2", "cleric", turnedAwayTopic), "and gate-2 heard it")
	require.False(t, c.holds(t, "gate-1", "cleric", admittedOn))
	require.True(t, c.holdsGuidance(t, "cleric"), "her Guidance die is still hers")

	c.standInTheRecord(t, "cleric", besideTheCastleGate)
	require.True(t, c.offersGuard(t, "cleric", "gate-1", friendsLine), "precondition: the guard's lines are read for her")
	_, listed := c.guardLine(t, "cleric", "gate-1", stragglerLine)
	require.False(t, listed, "one try: the case is gone on gate-1")
	_, listed = c.guardLine(t, "cleric", "gate-2", stragglerLine)
	require.False(t, listed, "and on gate-2")
}

// TestCastleAStragglerWhoWalksAwayFromAGoodRollIsAdmitted: the same with a
// total above the mark: she is recognised, keeps her die, and walks through.
func TestCastleAStragglerWhoWalksAwayFromAGoodRollIsAdmitted(t *testing.T) {
	c := theClericsWaitingCase(t, 18)

	c.walkAway(t)
	require.Empty(t, reactRow(c, t, "cleric").ID)
	require.Zero(t, c.reservations(t))
	require.True(t, c.holds(t, "gate-1", "cleric", admittedOn), "the hit lands: gate-1 recognises her")
	require.True(t, c.holdsGuidance(t, "cleric"), "her die is kept")

	c.standInTheRecord(t, "cleric", besideTheCastleGate)
	cross := c.walk(t, "cleric", []spatial.Position{pastTheCastleGate})
	require.Equal(t, session.MovementCompleted, cross.Status, "recognised, she walks through: %s", cross.StopReason)
}

// TestCastleASpendAfterWalkingAwayIsSettledDeclined: the hold is abandoned
// in the stored world behind the session's back, so her question is still
// open when she answers "spend". The answer is settled DECLINED — her die is
// kept — the claim lands as rolled, and she is told so (RollKept).
func TestCastleASpendAfterWalkingAwayIsSettledDeclined(t *testing.T) {
	c := theClericsWaitingCase(t, 2)
	react := reactRow(c, t, "cleric")
	enc := c.world(t)
	_, err := enc.AbandonReservation(&encounter.AbandonReservationInput{Member: "cleric"})
	require.NoError(t, err)
	abandoned := enc.ToData()
	c.encounters.byID["sess"] = &abandoned

	out, err := c.mgr.React(context.Background(), &session.ReactInput{
		Session: "sess", Member: "cleric", DeclarationID: react.ID, Answer: session.Take(""),
	})
	require.NoError(t, err)
	require.True(t, out.RollKept, "she is told her roll was kept")
	require.True(t, c.holdsGuidance(t, "cleric"), "the spend was declined: her die is kept")
	require.True(t, c.holds(t, "gate-1", "cleric", turnedAwayTopic), "the roll as made lands")
	require.Zero(t, c.reservations(t))

	_, err = c.mgr.React(context.Background(), &session.ReactInput{
		Session: "sess", Member: "cleric", DeclarationID: react.ID, Answer: session.Take(""),
	})
	require.ErrorIs(t, err, session.ErrNoWindow, "the question was answered once")
}

// TestCastleAKeptRollSettlesOnceAfterALoad: the hold is abandoned in the
// stored world and the host reloads before any verb; the next verb's commit
// — alice's step — settles it once.
func TestCastleAKeptRollSettlesOnceAfterALoad(t *testing.T) {
	c := theClericsWaitingCase(t, 2)
	react := reactRow(c, t, "cleric")
	enc := c.world(t)
	_, err := enc.AbandonReservation(&encounter.AbandonReservationInput{Member: "cleric"})
	require.NoError(t, err)
	abandoned := enc.ToData()
	c.encounters.byID["sess"] = &abandoned
	c.mgr = c.manager(t) // the host's load

	c.standInTheRecord(t, "alice", backDownTheRoad[0])
	c.walk(t, "alice", []spatial.Position{backDownTheRoad[1]})
	require.Empty(t, reactRow(c, t, "cleric").ID, "the next commit settled it")
	require.Zero(t, c.reservations(t))
	require.True(t, c.holds(t, "gate-1", "cleric", turnedAwayTopic))
	require.True(t, c.holdsGuidance(t, "cleric"))

	_, err = c.mgr.React(context.Background(), &session.ReactInput{
		Session: "sess", Member: "cleric", DeclarationID: react.ID, Answer: session.Take(""),
	})
	require.ErrorIs(t, err, session.ErrNoWindow, "once")
}

// TestCastleAWaitingCaseWhoseGuardFallsIsReleased: gate-1 falls while her
// roll waits. Nobody is left to answer: the next commit ends her question
// and the hold with no verdict, and her die is kept.
func TestCastleAWaitingCaseWhoseGuardFallsIsReleased(t *testing.T) {
	c := theClericsWaitingCase(t, 2)
	for i := range c.sessions.byID["sess"].NPCs {
		if c.sessions.byID["sess"].NPCs[i].ID == "gate-1" {
			c.sessions.byID["sess"].NPCs[i].HitPoints = 0
		}
	}
	c.standInTheRecord(t, "alice", backDownTheRoad[0])
	c.walk(t, "alice", []spatial.Position{backDownTheRoad[1]})
	require.Empty(t, reactRow(c, t, "cleric").ID, "her question ended")
	require.Zero(t, c.reservations(t), "with the hold")
	require.False(t, c.holds(t, "gate-1", "cleric", turnedAwayTopic), "with no verdict")
	require.False(t, c.holds(t, "gate-1", "cleric", admittedOn))
	require.True(t, c.holdsGuidance(t, "cleric"))
}

// TestCastleAStragglerHasOneTry: one try at the straggler's claim
// (rpg-project#932) through the session. With no die to hold, her case is
// missed at once: it is gone from her row on both guards, the line asking
// after her friends is still offered and answered, and alice's vouch still
// lets her through.
func TestCastleAStragglerHasOneTry(t *testing.T) {
	ctx := context.Background()
	c := atTheAdmittedGate(t)
	c.dice.d20s = []int{2}
	c.upToTheGate(t)
	_, err := speakAsOffered(ctx, c.mgr, &session.SpeakInput{Session: "sess", Member: "cleric", Target: "gate-1", OptionID: stragglerLine})
	require.NoError(t, err)
	require.True(t, c.holds(t, "gate-1", "cleric", turnedAwayTopic), "the miss turns her away")
	for _, guard := range []string{"gate-1", "gate-2"} {
		_, listed := c.guardLine(t, "cleric", guard, stragglerLine)
		require.False(t, listed, "the case is gone on %s", guard)
	}
	require.True(t, c.offersGuard(t, "cleric", "gate-1", friendsLine), "she may ask after her friends")
	_, err = speakAsOffered(ctx, c.mgr, &session.SpeakInput{Session: "sess", Member: "cleric", Target: "gate-1", OptionID: friendsLine})
	require.NoError(t, err, "and is answered")

	c.standInTheRecord(t, "alice", theHailCell)
	_, err = speakAsOffered(ctx, c.mgr, &session.SpeakInput{Session: "sess", Member: "alice", Target: "gate-1", OptionID: vouchLine})
	require.NoError(t, err, "alice vouches for her")
	cross := c.walk(t, "cleric", []spatial.Position{pastTheCastleGate})
	require.Equal(t, session.MovementCompleted, cross.Status, "vouched for, she walks through: %s", cross.StopReason)
}
