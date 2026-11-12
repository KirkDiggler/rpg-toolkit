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
	"encoding/json"
	"fmt"
	"sync"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/KirkDiggler/rpg-toolkit/core"
	"github.com/KirkDiggler/rpg-toolkit/play/intel"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/character"
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

// waitingCaseTests are the tests that start from the cleric's waiting case,
// built ONCE per face in [TestMain] for a run that will use them.
var waitingCaseTests = []string{
	"TestCastleAStragglerWhoWalksAwayFromABadRollIsTurnedAway",
	"TestCastleAStragglerWhoWalksAwayFromAGoodRollIsAdmitted",
	"TestCastleASpendAfterWalkingAwayIsSettledDeclined",
	"TestCastleAKeptRollSettlesOnceAfterALoad",
	"TestCastleAWaitingCaseWhoseGuardFallsIsReleased",
	"TestCastleAWaitingCaseWhoseSpeakerExitsIsDropped",
	"TestCastleAKeptRollThatCannotSettleDoesNotWedgeTheTable",
	"TestCastleAKeptRollThatFailsAfterSpeakingSavesNothing",
}

// castleStores is one stored moment of a castle scene.
type castleStores struct {
	sessions   map[string]*session.SessionData
	encounters map[string]*encounter.EncounterData
	characters map[string]*character.Data
	seats      map[string]session.SeatData
}

// waitingCases is the cleric's waiting case per d20 face: the admitted gate
// with the cleric, holding a Guidance die, beside the gate making her own
// case, her check paused on the die.
var waitingCases = map[int]*struct {
	once   sync.Once
	err    error
	stores castleStores
}{2: {}, 18: {}}

// buildTheWaitingCases builds every face's waiting case.
func buildTheWaitingCases() {
	for face, wc := range waitingCases {
		wc.once.Do(func() { wc.stores, wc.err = buildTheWaitingCase(face) })
	}
}

// buildTheWaitingCase plays the waiting case once from the admitted gate's
// stores. It takes no *testing.T because [TestMain] runs it; every
// precondition it checks is an error.
func buildTheWaitingCase(face int) (castleStores, error) {
	admittedGate.once.Do(func() { admittedGate.err = buildTheAdmittedGate() })
	if admittedGate.err != nil {
		return castleStores{}, admittedGate.err
	}
	c := &freeRoamCastle{
		live: &actCapture{}, dice: &castleDice{d20s: []int{face}},
		sessions: newFakeSessions(), encounters: newFakeEncounters(),
		characters: newFakeCharacters(), seats: newFakeSeats(),
	}
	if err := restoreInto(c, castleStores{
		sessions: admittedGate.sessions, encounters: admittedGate.encounters,
		characters: admittedGate.characters, seats: admittedGate.seats,
	}); err != nil {
		return castleStores{}, err
	}
	mgr, err := session.NewManager(&session.Config{Record: session.DiscardRecord{}, Acts: c.live, Seats: c.seats, CharacterConfig: newFakeCharacterConfig(),
		PresentationIDs: testPresentationIDs{}, Dice: c.dice, TurnDriver: session.Driver(),
		Sessions: c.sessions, Encounters: c.encounters, Characters: c.characters,
	})
	if err != nil {
		return castleStores{}, err
	}
	withGuidance(c.characters.byID["cleric"])
	world := c.encounters.byID["sess"]
	for i := range world.Members {
		if world.Members[i].ID == "cleric" {
			world.Members[i].Cell = &encounter.PositionData{X: besideTheCastleGate.X, Y: besideTheCastleGate.Y}
		}
	}
	claim, err := speakAsOffered(context.Background(), mgr, &session.SpeakInput{
		Session: "sess", Member: "cleric", Target: "gate-1", OptionID: stragglerLine,
	})
	if err != nil {
		return castleStores{}, fmt.Errorf("the cleric's case: %w", err)
	}
	if !claim.Paused {
		return castleStores{}, fmt.Errorf("precondition: her Guidance die stops the check to ask: %+v", claim)
	}
	return castleStores{sessions: c.sessions.byID, encounters: c.encounters.byID, characters: c.characters.byID, seats: c.seats.byID}, nil
}

// restoreInto copies stored stores into a castle's fresh fakes.
func restoreInto(c *freeRoamCastle, from castleStores) error {
	for id, data := range from.sessions {
		copied, err := copyOf(data)
		if err != nil {
			return err
		}
		c.sessions.byID[id] = copied
	}
	for id, data := range from.encounters {
		copied, err := copyOf(data)
		if err != nil {
			return err
		}
		c.encounters.byID[id] = copied
	}
	for id, data := range from.characters {
		copied, err := copyOf(data)
		if err != nil {
			return err
		}
		c.characters.byID[id] = copied
	}
	for id, data := range from.seats {
		c.seats.byID[id] = data
	}
	return nil
}

// theClericsWaitingCase restores the cleric's waiting case for one d20 face
// into fresh stores; every d20 after it shows face too.
func theClericsWaitingCase(t *testing.T, face int) *freeRoamCastle {
	t.Helper()
	wc := waitingCases[face]
	require.NotNil(t, wc, "a waiting case is built for face %d", face)
	wc.once.Do(func() { wc.stores, wc.err = buildTheWaitingCase(face) })
	require.NoError(t, wc.err, "the waiting case's scene")
	c := &freeRoamCastle{
		live: &actCapture{}, dice: &castleDice{d20s: []int{face}},
		sessions: newFakeSessions(), encounters: newFakeEncounters(),
		characters: newFakeCharacters(), seats: newFakeSeats(),
	}
	require.NoError(t, restoreInto(c, wc.stores))
	c.mgr = c.manager(t)
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
	require.Zero(t, c.reservations(t), "no hold remains")
	require.Empty(t, c.sessions.byID["sess"].Windows.Windows, "and no question")
	require.True(t, c.holds(t, "gate-1", "cleric", turnedAwayTopic), "the miss lands: gate-1 turned her away")
	require.True(t, c.holds(t, "gate-2", "cleric", turnedAwayTopic), "and gate-2 heard it")
	require.False(t, c.holds(t, "gate-1", "cleric", admittedOn))
	require.True(t, c.holdsGuidance(t, "cleric"), "her Guidance die is still hers")

	c.standInTheRecord(t, "cleric", besideTheCastleGate)
	view, err := c.mgr.View(context.Background(), &session.ViewInput{Session: "sess", Member: "cleric"})
	require.NoError(t, err)
	sighted := map[string]bool{}
	for _, s := range view.Sightings {
		sighted[s.Subject] = true
	}
	panel := c.bar(t, "cleric")
	for _, guard := range []string{"gate-1", "gate-2"} {
		require.True(t, sighted[guard], "precondition: she sights %s, so an absent line is the line's", guard)
		require.True(t, availableOn(panel, guard, friendsLine), "precondition: %s's lines are read for her", guard)
		_, listed := lineOnPanel(panel, guard, stragglerLine)
		require.False(t, listed, "one try: the case is gone on %s", guard)
	}
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

// TestCastleAWaitingCaseWhoseSpeakerExitsIsDropped: the cleric leaves the
// run while her roll waits. Nobody is left to roll: her question and the
// hold end with her, with no verdict.
func TestCastleAWaitingCaseWhoseSpeakerExitsIsDropped(t *testing.T) {
	c := theClericsWaitingCase(t, 2)
	_, err := c.mgr.Exit(context.Background(), &session.ExitInput{Session: "sess", Member: "cleric"})
	require.NoError(t, err)
	require.Zero(t, c.reservations(t), "the hold is dropped")
	require.False(t, c.holds(t, "gate-1", "cleric", turnedAwayTopic), "with no verdict")
	require.Empty(t, c.sessions.byID["sess"].Windows.Windows, "and no question is left open")
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
		c.sightingOf(t, "cleric", guard) // precondition: she sights the guard
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

// TestCastleAKeptRollThatCannotSettleDoesNotWedgeTheTable: the cleric's
// hold is abandoned in the stored world and her stored pause is corrupt, so
// the kept settle fails in resolution — a reason no refusal covers. Every
// member's commit runs the kept-roll pass, so that one window must not
// wedge the table: alice's next walk succeeds, the hold and the question
// are ended with no verdict, the die is unspent, and the fault is recorded
// on her walk's report.
func TestCastleAKeptRollThatCannotSettleDoesNotWedgeTheTable(t *testing.T) {
	c := theClericsWaitingCase(t, 2)
	enc := c.world(t)
	_, err := enc.AbandonReservation(&encounter.AbandonReservationInput{Member: "cleric"})
	require.NoError(t, err)
	abandoned := enc.ToData()
	c.encounters.byID["sess"] = &abandoned
	windows := c.sessions.byID["sess"].Windows.Windows
	require.Len(t, windows, 1, "precondition: her question is the one open")
	var payload map[string]any
	require.NoError(t, json.Unmarshal(windows[0].Payload, &payload))
	payload["pause"].(map[string]any)["frozen"] = []byte("not a paused check")
	windows[0].Payload, err = json.Marshal(payload)
	require.NoError(t, err)

	c.standInTheRecord(t, "alice", backDownTheRoad[0])
	out := c.walk(t, "alice", []spatial.Position{backDownTheRoad[1]})
	require.Len(t, out.Saved.Faults, 1, "the failed settle is recorded")
	require.Contains(t, out.Saved.Faults[0], "cleric's kept roll", "naming whose")
	require.Empty(t, reactRow(c, t, "cleric").ID, "her question is closed")
	require.Zero(t, c.reservations(t), "the hold is released")
	require.False(t, c.holds(t, "gate-1", "cleric", turnedAwayTopic), "with no verdict")
	require.True(t, c.holdsGuidance(t, "cleric"), "and her die unspent")
	c.walk(t, "alice", []spatial.Position{backDownTheRoad[0]}) // and the table moves on
}

// TestCastleAKeptRollThatFailsAfterSpeakingSavesNothing: the kept settle's
// Speak fails AFTER it began writing — its answer's roll on the host's dice
// fails, after the `spoke` beat. That is not absorbed: the React fails, and
// the stored session and world are exactly as they were before it, so
// nothing half-applied is saved.
func TestCastleAKeptRollThatFailsAfterSpeakingSavesNothing(t *testing.T) {
	c := theClericsWaitingCase(t, 2)
	react := reactRow(c, t, "cleric")
	enc := c.world(t)
	_, err := enc.AbandonReservation(&encounter.AbandonReservationInput{Member: "cleric"})
	require.NoError(t, err)
	abandoned := enc.ToData()
	c.encounters.byID["sess"] = &abandoned
	sessionBefore, err := json.Marshal(c.sessions.byID["sess"])
	require.NoError(t, err)
	worldBefore, err := json.Marshal(c.encounters.byID["sess"])
	require.NoError(t, err)
	sheetBefore, err := json.Marshal(c.characters.byID["cleric"])
	require.NoError(t, err)

	c.dice.failOthers = fmt.Errorf("the dice fell off the table")
	_, err = c.mgr.React(context.Background(), &session.ReactInput{
		Session: "sess", Member: "cleric", DeclarationID: react.ID, Answer: session.Take(""),
	})
	require.ErrorContains(t, err, "the dice fell off the table", "the failure after the first write is returned")
	sessionAfter, err := json.Marshal(c.sessions.byID["sess"])
	require.NoError(t, err)
	worldAfter, err := json.Marshal(c.encounters.byID["sess"])
	require.NoError(t, err)
	sheetAfter, err := json.Marshal(c.characters.byID["cleric"])
	require.NoError(t, err)
	require.JSONEq(t, string(sessionBefore), string(sessionAfter), "the session is as it was")
	require.JSONEq(t, string(worldBefore), string(worldAfter), "the world is as it was")
	require.JSONEq(t, string(sheetBefore), string(sheetAfter), "her sheet is as it was")
}
