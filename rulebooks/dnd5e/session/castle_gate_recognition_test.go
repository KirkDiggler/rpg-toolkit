// Copyright (C) 2026 Kirk Diggler
// SPDX-License-Identifier: GPL-3.0-or-later

package session_test

// castle_gate_recognition_test.go is ADMISSION IS RECOGNITION at the session
// (rpg-project#928). Alice wins the guest claim at the captain's hail, with
// the cleric down the road where no gate guard sees her. The captain
// recognises alice (and gate-1 and gate-2 heard it); the cleric, who was not
// there, is recognised by nobody. So alice's Speak row offers the captain's
// line for whom he recognises and the cleric's, once she stands beside alice,
// does not; the cleric is barred at the gate and alice walks through; and a
// fresh manager over the same stores (the host's load) says the same.

import (
	"context"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"sync"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/abilities"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/character"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/encounter"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/encounter/dungeonspec"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/session"
	"github.com/KirkDiggler/rpg-toolkit/tools/spatial"
)

// downTheRoad is far enough down the road that no gate guard sees it.
var downTheRoad = spatial.Position{X: 0, Y: 16}

// standInTheRecord puts a member on a cell in the stored world, as the
// fixtures here seat the party, so the next verb reads her there.
func (c *freeRoamCastle) standInTheRecord(t *testing.T, member string, cell spatial.Position) {
	t.Helper()
	world := c.encounters.byID["sess"]
	for i := range world.Members {
		if world.Members[i].ID == encounter.MemberID(member) {
			world.Members[i].Cell = &encounter.PositionData{X: cell.X, Y: cell.Y}
			return
		}
	}
	require.FailNowf(t, "not in the run", "%s", member)
}

// walk is member's walk along path, walked on through a hail.
func (c *freeRoamCastle) walk(t *testing.T, member string, path []spatial.Position) *session.MoveOutput {
	t.Helper()
	out, err := walkOn(context.Background(), c.mgr, &session.MoveInput{Session: "sess", Member: member, Path: path})
	require.NoError(t, err)
	return out
}

// admittedGate is the stored state of [atTheAdmittedGate]'s scene, built
// ONCE per test binary in [TestMain] — the castle's launch would otherwise
// be on this test's clock — and restored into fresh stores for the test.
var admittedGate struct {
	once       sync.Once
	err        error
	sessions   map[string]*session.SessionData
	encounters map[string]*encounter.EncounterData
	characters map[string]*character.Data
	seats      map[string]session.SeatData
}

// admittedGateTests are the tests that start from the admitted gate.
var admittedGateTests = []string{
	"TestCastleTheCaptainRecognisesWhomHeAdmitted",
	"TestCastleTheRecognisedWalksThroughTheGate",
	"TestCastleTheUnrecognisedIsBarredAtTheGate",
	"TestCastleAStragglerIsVouchedFor",
	"TestCastleAStragglersWaitingRollHoldsTheVouch",
	"TestCastleAStragglerHasOneTry",
	"TestCastleTheHintIsShoutedToTheYard", "TestCastleTheHintDoesNotReachTheShutHall",
	"TestCastleTheHintReachesTheShutHallThroughAFriend",
}

// captainsPlaceInTheHall is where the captain walks once he admits anyone
// (rpg-project#929), as the castle file writes it.
var captainsPlaceInTheHall = spatial.Position{X: 3, Y: -9}

// buildTheAdmittedGate plays the admitted gate's scene once: the castle for
// alice (Charisma 16, every d20 an 18) and the cleric, alice stepped onto
// the captain's hail cell with the cleric down the road out of every gate
// guard's sight, and alice's guest claim won. It takes no *testing.T
// because [TestMain] runs it; every precondition it checks is an error.
func buildTheAdmittedGate() error {
	ctx := context.Background()
	dir, err := exec.Command("go", "list", "-m", "-f", "{{.Dir}}", encounterModule).Output()
	if err != nil {
		return fmt.Errorf("find the encounter module: %w", err)
	}
	raw, err := os.ReadFile(filepath.Join(strings.TrimSpace(string(dir)), castleFixture))
	if err != nil {
		return err
	}
	compiled, err := dungeonspec.Load(raw)
	if err != nil {
		return err
	}
	alice := armedFighter("alice")
	alice.AbilityScores[abilities.CHA] = 16
	c := &freeRoamCastle{
		live: &actCapture{}, dice: &castleDice{d20s: []int{18}},
		characters: newFakeCharacters(alice, healingCleric()),
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
	world := c.encounters.byID["sess"]
	for i := range world.Members {
		switch world.Members[i].ID {
		case "alice":
			world.Members[i].Cell = &encounter.PositionData{X: backDownTheRoad[0].X, Y: backDownTheRoad[0].Y}
		case "cleric":
			world.Members[i].Cell = &encounter.PositionData{X: downTheRoad.X, Y: downTheRoad.Y}
		}
	}
	if _, err := walkOn(ctx, c.mgr, &session.MoveInput{Session: "sess", Member: "alice", Path: []spatial.Position{theHailCell}}); err != nil {
		return fmt.Errorf("the step to the hail cell: %w", err)
	}
	if _, err := speakAsOffered(ctx, c.mgr, &session.SpeakInput{Session: "sess", Member: "alice", Target: "captain", OptionID: "guest"}); err != nil {
		return fmt.Errorf("alice's claim: %w", err)
	}
	if state := c.encounters.byID["sess"].Interactions[0].Blob.State; state != "admitted" {
		return fmt.Errorf("precondition: the claim admitted, the gate is %q", state)
	}
	// THE CAPTAIN GOES IN (rpg-project#929): alice paces at the hail cell
	// while the world turns, until he stands in the hall and gate-1, drawn
	// off a cell by the gate opening, stands his post again.
	pace := []spatial.Position{backDownTheRoad[0], theHailCell, backDownTheRoad[0], theHailCell, backDownTheRoad[0], theHailCell}
	placed := func(id encounter.MemberID) spatial.Position {
		for _, m := range c.encounters.byID["sess"].Members {
			if m.ID == id && m.Cell != nil {
				return spatial.Position{X: m.Cell.X, Y: m.Cell.Y}
			}
		}
		return spatial.Position{}
	}
	for range 6 {
		if placed("captain") == captainsPlaceInTheHall && placed("gate-1") == beneathTheCastleGate {
			break
		}
		if _, err := walkOn(ctx, c.mgr, &session.MoveInput{Session: "sess", Member: "alice", Path: pace}); err != nil {
			return fmt.Errorf("alice paces: %w", err)
		}
	}
	if placed("captain") != captainsPlaceInTheHall || placed("gate-1") != beneathTheCastleGate {
		return fmt.Errorf("precondition: the captain stands in the hall (%v) and gate-1 on his post (%v)",
			placed("captain"), placed("gate-1"))
	}
	admittedGate.sessions = c.sessions.byID
	admittedGate.encounters = c.encounters.byID
	admittedGate.characters = c.characters.byID
	admittedGate.seats = c.seats.byID
	return nil
}

// atTheAdmittedGate restores the admitted gate's stores into fresh ones.
func atTheAdmittedGate(t *testing.T) *freeRoamCastle {
	t.Helper()
	admittedGate.once.Do(func() { admittedGate.err = buildTheAdmittedGate() })
	require.NoError(t, admittedGate.err, "the admitted gate's scene")
	c := &freeRoamCastle{
		live: &actCapture{}, dice: &castleDice{d20s: []int{18}},
		sessions: newFakeSessions(), encounters: newFakeEncounters(),
		characters: newFakeCharacters(), seats: newFakeSeats(),
	}
	for id, data := range admittedGate.sessions {
		copied, err := copyOf(data)
		require.NoError(t, err)
		c.sessions.byID[id] = copied
	}
	for id, data := range admittedGate.encounters {
		copied, err := copyOf(data)
		require.NoError(t, err)
		c.encounters.byID[id] = copied
	}
	for id, data := range admittedGate.characters {
		copied, err := copyOf(data)
		require.NoError(t, err)
		c.characters.byID[id] = copied
	}
	for id, data := range admittedGate.seats {
		c.seats.byID[id] = data
	}
	c.mgr = c.manager(t)
	return c
}

func TestCastleTheCaptainRecognisesWhomHeAdmitted(t *testing.T) {
	c := atTheAdmittedGate(t)
	world := c.world(t)

	// THE BEAT KEEPS ITS AUDIENCE: the session reads a member's story by
	// audience, so alice, who saw it, reads the recognition, and the cleric,
	// out of sight down the road, does not.
	recognitionsFor := func(member string) int {
		t.Helper()
		story, err := world.Story(&encounter.StoryInput{Audience: encounter.MemberID(member)})
		require.NoError(t, err)
		n := 0
		for _, e := range story {
			if e.Tags["tag"] == encounter.BeatRecognized {
				n++
			}
		}
		return n
	}
	require.Positive(t, recognitionsFor("alice"), "alice reads that she was recognised")
	// THE TOPIC WAKE IS THE MACHINE'S (rpg-project#929): it woke the captain
	// into the hall and is in no member's story, so no act carries it.
	for _, member := range []string{"alice", "captain"} {
		story, err := world.Story(&encounter.StoryInput{Audience: encounter.MemberID(member)})
		require.NoError(t, err)
		for _, e := range story {
			require.NotEqual(t, "topic", e.Tags["tag"], "no topic beat in %s's story", member)
			require.NotContains(t, string(e.Payload), `"beat":"topic"`, "no topic beat in %s's story", member)
		}
	}
	require.Zero(t, recognitionsFor("cleric"), "the cleric, who saw none of it, reads nothing")

	// The cleric walks up to the gate after the fact, into gate-1's sight and
	// Speak reach, as alice at the hail cell is.
	c.upToTheGate(t)

	check := func(when string) {
		t.Helper()
		require.Equal(t, captainsPlaceInTheHall, memberOf(t, c.world(t), "captain").Position,
			"%s, the captain stands in the hall (rpg-project#929)", when)
		alice, cleric := c.bar(t, "alice"), c.bar(t, "cleric")
		require.True(t, availableOn(alice, "gate-1", vouchLine), "%s, gate-1 offers alice the vouch", when)
		require.False(t, availableOn(alice, "gate-1", stragglerLine), "%s, and not the straggler's case", when)
		require.True(t, availableOn(cleric, "gate-1", stragglerLine), "%s, he offers the cleric the straggler's case", when)
		require.False(t, availableOn(cleric, "gate-1", vouchLine), "%s, and not the vouch", when)
	}
	check("after the claim")
	c.mgr = c.manager(t)
	check("after a load")
}

func TestCastleTheRecognisedWalksThroughTheGate(t *testing.T) {
	c := atTheAdmittedGate(t)
	c.standInTheRecord(t, "alice", besideTheCastleGate)
	// The captain went in and left the gate open (rpg-project#929); shut, it
	// opens to her hand: recognised, she works it.
	c.setTheGateInTheRecord(t, "closed", nil)
	opened, err := c.mgr.OpenDoor(context.Background(), &session.OpenDoorInput{Session: "sess", Member: "alice", Door: castleGateDoor})
	require.NoError(t, err)
	require.Nil(t, opened.Refused, "recognised, alice works the gate door")
	c.mgr = c.manager(t) // the host's load: the recognition is in the record
	cross := c.walk(t, "alice", []spatial.Position{pastTheCastleGate})
	require.Equal(t, session.MovementCompleted, cross.Status, "recognised, alice crosses: %s", cross.StopReason)
}

func TestCastleTheUnrecognisedIsBarredAtTheGate(t *testing.T) {
	c := atTheAdmittedGate(t)
	// Alice stands aside out of the gate's way; the cleric, who was not
	// there, comes up the road to the gate after the fact.
	c.standInTheRecord(t, "alice", besideAlice)
	c.standInTheRecord(t, "cleric", theHailCell)
	route, err := c.world(t).Route(encounter.RouteInput{Mover: "cleric", Policy: encounter.MoveToward, Anchor: besideTheCastleGate, Budget: 200})
	require.NoError(t, err)
	up := c.walk(t, "cleric", append(route.Path, besideTheCastleGate))
	require.Equal(t, session.MovementCompleted, up.Status, "precondition: the cleric reaches the gate: %s", up.StopReason)
	c.mgr = c.manager(t) // the host's load: still nobody recognises her
	barred := c.walk(t, "cleric", []spatial.Position{pastTheCastleGate})
	require.Equal(t, session.MovementStopped, barred.Status, "the cleric, recognised by nobody, is barred")
	require.Contains(t, barred.StopReason, "bars the way", "by a hold, not a body")
	require.Equal(t, besideTheCastleGate, c.where(t, "cleric"), "and has not moved")
}

// The gate guards' lines for a straggler (rpg-project#929).
const (
	vouchLine     = "vouch"
	stragglerLine = "straggler"
)

// offersGuard reports whether member's Speak row offers a guard a line,
// available.
func (c *freeRoamCastle) offersGuard(t *testing.T, member, guard, option string) bool {
	t.Helper()
	return availableOn(c.bar(t, member), guard, option)
}

// availableOn reports whether a panel already read offers a guard a line,
// available.
func availableOn(panel *session.AffordOutput, guard, option string) bool {
	row, ok := lineOnPanel(panel, guard, option)
	return ok && row.Available
}

// guardLine is member's Speak row's line for a guard, if listed.
func (c *freeRoamCastle) guardLine(t *testing.T, member, guard, option string) (session.SpeakOptionRow, bool) {
	t.Helper()
	return lineOnPanel(c.bar(t, member), guard, option)
}

// lineOnPanel is a panel's Speak row line for a guard, if listed.
func lineOnPanel(panel *session.AffordOutput, guard, option string) (session.SpeakOptionRow, bool) {
	for _, d := range panel.Declarations {
		if d.Verb != session.VerbSpeak {
			continue
		}
		for _, row := range d.SpeakOptions {
			if row.Target == guard && row.OptionID == option {
				return row, true
			}
		}
	}
	return session.SpeakOptionRow{}, false
}

// upToTheGate walks the cleric from beside alice to beside the gate, in
// gate-1's sight.
func (c *freeRoamCastle) upToTheGate(t *testing.T) {
	t.Helper()
	c.standInTheRecord(t, "cleric", besideAlice)
	route, err := c.world(t).Route(encounter.RouteInput{Mover: "cleric", Policy: encounter.MoveToward, Anchor: besideTheCastleGate, Budget: 200})
	require.NoError(t, err)
	up := c.walk(t, "cleric", append(route.Path, besideTheCastleGate))
	require.Equal(t, session.MovementCompleted, up.Status, "precondition: the cleric reaches the gate: %s", up.StopReason)
}

// TestCastleAStragglerIsVouchedFor: the cleric, who was not there, comes up
// to the gate after the captain has gone in. A vouch from down the road is
// refused by the reach door; alice comes back and vouches with the cleric in
// gate-1's sight, and the cleric walks through. A load keeps it, and the
// captain does not walk again.
func TestCastleAStragglerIsVouchedFor(t *testing.T) {
	ctx := context.Background()
	c := atTheAdmittedGate(t)
	c.upToTheGate(t)
	require.False(t, c.offersGuard(t, "cleric", "gate-1", vouchLine), "the vouch is not the cleric's to say")

	c.standInTheRecord(t, "alice", backDownTheRoad[2])
	_, err := c.mgr.Speak(ctx, &session.SpeakInput{Session: "sess", Member: "alice", Target: "gate-1", OptionID: vouchLine})
	require.Error(t, err, "from down the road the vouch is out of Speak reach")

	c.standInTheRecord(t, "alice", theHailCell)
	_, err = speakAsOffered(ctx, c.mgr, &session.SpeakInput{Session: "sess", Member: "alice", Target: "gate-1", OptionID: vouchLine})
	require.NoError(t, err, "alice, back at the gate, vouches")

	c.mgr = c.manager(t) // the host's load
	cross := c.walk(t, "cleric", []spatial.Position{pastTheCastleGate})
	require.Equal(t, session.MovementCompleted, cross.Status, "vouched for, the cleric walks through: %s", cross.StopReason)
	require.Equal(t, captainsPlaceInTheHall, memberOf(t, c.world(t), "captain").Position, "the captain stays in the hall")
	require.Equal(t, "in-hall", stateOf(t, c.world(t), "captain"), "and does not walk again")
}

// TestCastleAStragglersWaitingRollHoldsTheVouch pins today's reservation
// interplay (gate slice 4, reserve before dice) for the operator's open call
// on it: the straggler's case reads the front gate, so a cleric holding a
// Guidance die makes her roll wait and holds every line that reads the gate,
// alice's vouch included, which shows pending on the cleric. It frees when
// she answers.
func TestCastleAStragglersWaitingRollHoldsTheVouch(t *testing.T) {
	ctx := context.Background()
	c := atTheAdmittedGate(t)
	withGuidance(c.characters.byID["cleric"])
	c.upToTheGate(t)

	claim, err := speakAsOffered(ctx, c.mgr, &session.SpeakInput{Session: "sess", Member: "cleric", Target: "gate-1", OptionID: stragglerLine})
	require.NoError(t, err)
	require.True(t, claim.Paused, "precondition: her Guidance die stops the check to ask")

	vouch, listed := c.guardLine(t, "alice", "gate-1", vouchLine)
	require.True(t, listed, "the vouch is still listed")
	require.True(t, vouch.Pending, "and waits on the cleric's roll")
	require.False(t, vouch.Available)
	require.Equal(t, "cleric", vouch.PendingOn)

	react := reactRow(c, t, "cleric")
	require.NotEmpty(t, react.ID, "precondition: she is asked")
	_, err = c.mgr.React(ctx, &session.ReactInput{Session: "sess", Member: "cleric", DeclarationID: react.ID, Answer: session.Take("")})
	require.NoError(t, err)
	vouch, _ = c.guardLine(t, "alice", "gate-1", vouchLine)
	require.False(t, vouch.Pending, "her answer frees the vouch")
	require.True(t, vouch.Available)
}
