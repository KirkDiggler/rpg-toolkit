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

// theWayIn is the captain's line for whom he recognises.
const theWayIn = "the-way-in"

// downTheRoad is far enough down the road that no gate guard sees it.
var downTheRoad = spatial.Position{X: 0, Y: 16}

// offersTheCaptain reports whether member's Speak row offers the captain a
// line, available.
func (c *freeRoamCastle) offersTheCaptain(t *testing.T, member, option string) bool {
	t.Helper()
	for _, d := range c.bar(t, member).Declarations {
		if d.Verb != session.VerbSpeak {
			continue
		}
		for _, row := range d.SpeakOptions {
			if row.Target == "captain" && row.OptionID == option && row.Available {
				return true
			}
		}
	}
	return false
}

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
}

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

	// THE BEAT KEEPS ITS AUDIENCE: the session reads a member's story by
	// audience, so alice, who saw it, reads the recognition, and the cleric,
	// out of sight down the road, does not.
	recognitionsFor := func(member string) int {
		t.Helper()
		story, err := c.world(t).Story(&encounter.StoryInput{Audience: encounter.MemberID(member)})
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
	require.Zero(t, recognitionsFor("cleric"), "the cleric, who saw none of it, reads nothing")

	// The cleric comes up beside alice, after the fact.
	c.standInTheRecord(t, "cleric", besideAlice)

	check := func(when string) {
		t.Helper()
		require.True(t, c.offersTheCaptain(t, "alice", theWayIn), "%s, the captain offers alice his line for whom he recognises", when)
		require.False(t, c.offersTheCaptain(t, "cleric", theWayIn), "%s, and not the cleric, who was not there", when)
	}
	check("after the claim")
	c.mgr = c.manager(t)
	check("after a load")
}

func TestCastleTheRecognisedWalksThroughTheGate(t *testing.T) {
	c := atTheAdmittedGate(t)
	c.standInTheRecord(t, "alice", besideTheCastleGate)
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
