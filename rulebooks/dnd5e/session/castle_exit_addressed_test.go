// Copyright (C) 2026 Kirk Diggler
// SPDX-License-Identifier: GPL-3.0-or-later

package session_test

// castle_exit_addressed_test.go is LEAVING WHILE ADDRESSED at the session
// (rpg-project#942). The operator's walk: alice is hailed by the captain,
// her troupe claim misses so the gate is warned and his address still
// stands, and she leaves the run. Before encounter v0.201.1 the saved run
// named her as the one addressed, the loader refused it, and every later
// verb failed for everyone. Now the run loads throughout and the cleric can
// still see her bar, walk, speak and leave.

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

	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/encounter"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/encounter/dungeonspec"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/session"
	"github.com/KirkDiggler/rpg-toolkit/tools/spatial"
)

// frontGateAddress is the front gate's saved address, or nil.
func frontGateAddress(t *testing.T, enc *encounter.Encounter) *encounter.InteractionAddress {
	t.Helper()
	for _, in := range enc.ToData().Interactions {
		if in.ID == "front-gate" {
			return in.Address
		}
	}
	require.FailNow(t, "the run holds no front gate")
	return nil
}

// walkMemberTo walks member toward cell along the run's own route.
func (c *freeRoamCastle) walkMemberTo(t *testing.T, member string, cell encounter.MemberID) *session.MoveOutput {
	t.Helper()
	anchor := memberOf(t, c.world(t), cell).Position
	route, err := c.world(t).Route(encounter.RouteInput{Mover: encounter.MemberID(member), Policy: encounter.MoveToward, Anchor: anchor, Budget: 200})
	require.NoError(t, err)
	require.NotEmpty(t, route.Path, "precondition: %s has a way toward %s", member, cell)
	return c.walk(t, member, route.Path)
}

// warnedGateTests start from the warned gate, built ONCE in [TestMain].
var warnedGateTests = []string{"TestCastleLeavingWhileAddressedLeavesARunThatLoads", "TestCastleTheArrestAtEachTheirOwn", "TestCastleTheArrestAtSharedView", "TestCastleTheTakenCannotWalkAway", "TestCastleARelocationAtEachTheirOwn", "TestCastleARelocationAtSharedView"}

// warnedGate is the operator's walk stored at the moment before alice
// leaves: told of the feast by the stranger, hailed by the captain, her
// troupe claim missed, his address on her still standing.
var warnedGate struct {
	once   sync.Once
	err    error
	stores castleStores
}

// buildTheWarnedGate plays the warned gate once and keeps its stores. It
// takes no *testing.T because [TestMain] runs it; every precondition it
// checks is an error.
func buildTheWarnedGate() (castleStores, error) {
	ctx := context.Background()
	dir, err := exec.Command("go", "list", "-m", "-f", "{{.Dir}}", encounterModule).Output()
	if err != nil {
		return castleStores{}, fmt.Errorf("find the encounter module: %w", err)
	}
	raw, err := os.ReadFile(filepath.Join(strings.TrimSpace(string(dir)), castleFixture))
	if err != nil {
		return castleStores{}, err
	}
	compiled, err := dungeonspec.Load(raw)
	if err != nil {
		return castleStores{}, err
	}
	// 12 + 3 beats the stranger's 5; 1 + 3 misses the captain's 13.
	c := &freeRoamCastle{
		live: &actCapture{}, dice: &castleDice{d20s: []int{12, 1}},
		characters: newFakeCharacters(armedFighter("alice"), healingCleric()),
		encounters: newFakeEncounters(), sessions: newFakeSessions(), seats: newFakeSeats(),
	}
	c.mgr, err = session.NewManager(&session.Config{Record: session.DiscardRecord{}, Acts: c.live, Seats: c.seats, CharacterConfig: newFakeCharacterConfig(),
		PresentationIDs: testPresentationIDs{}, Dice: c.dice, TurnDriver: session.Driver(),
		Sessions: c.sessions, Encounters: c.encounters, Characters: c.characters,
	})
	if err != nil {
		return castleStores{}, err
	}
	if _, err := c.mgr.Launch(ctx, &session.LaunchInput{Session: "sess", Dungeon: &compiled, Party: []string{"alice", "cleric"}}); err != nil {
		return castleStores{}, fmt.Errorf("launch: %w", err)
	}
	if _, err := speakAsOffered(ctx, c.mgr, &session.SpeakInput{Session: "sess", Member: "alice", Target: "pedlar", OptionID: "tonight"}); err != nil {
		return castleStores{}, fmt.Errorf("the stranger tells alice of the feast: %w", err)
	}
	// ONE STEP ONTO THE HAIL CELL, as the waiting claim's scene does, rather
	// than the whole road: the walk is the hail tests' subject.
	world := c.encounters.byID["sess"]
	for i := range world.Members {
		switch world.Members[i].ID {
		case "alice":
			world.Members[i].Cell = &encounter.PositionData{X: backDownTheRoad[0].X, Y: backDownTheRoad[0].Y}
		case "cleric":
			world.Members[i].Cell = &encounter.PositionData{X: watchingTheRoad.X, Y: watchingTheRoad.Y}
		}
	}
	hailed, err := c.mgr.Move(ctx, &session.MoveInput{Session: "sess", Member: "alice", Path: []spatial.Position{theHailCell}})
	if err != nil {
		return castleStores{}, fmt.Errorf("the step to the hail cell: %w", err)
	}
	if hailed.Addressed == nil || *hailed.Addressed != *theFrontGateAddress {
		return castleStores{}, fmt.Errorf("precondition: the captain hails alice: %+v", hailed.Addressed)
	}
	if _, err := speakAsOffered(ctx, c.mgr, &session.SpeakInput{Session: "sess", Member: "alice", Target: "captain", OptionID: "troupe"}); err != nil {
		return castleStores{}, fmt.Errorf("alice's troupe claim: %w", err)
	}
	return castleStores{sessions: c.sessions.byID, encounters: c.encounters.byID, characters: c.characters.byID, seats: c.seats.byID}, nil
}

// TestCastleLeavingWhileAddressedLeavesARunThatLoads is the operator's case
// on rpg-project#942 through the session: hailed, a missed claim with the
// address still standing, alice leaves; then the cleric's bar, walk, line
// and leaving all work, each on the stored run. On encounter v0.200.0 this
// fails at the first load after alice leaves: interaction "front-gate" is
// saved addressing "alice", who is not in the run.
func TestCastleLeavingWhileAddressedLeavesARunThatLoads(t *testing.T) {
	warnedGate.once.Do(func() { warnedGate.stores, warnedGate.err = buildTheWarnedGate() })
	require.NoError(t, warnedGate.err)
	c := &freeRoamCastle{
		live: &actCapture{}, dice: &castleDice{d20s: []int{1}},
		sessions: newFakeSessions(), encounters: newFakeEncounters(),
		characters: newFakeCharacters(), seats: newFakeSeats(),
	}
	require.NoError(t, restoreInto(c, warnedGate.stores))
	c.mgr = c.manager(t)
	ctx := context.Background()
	standing := frontGateAddress(t, c.world(t))
	require.NotNil(t, standing, "precondition: the missed claim left the captain's address standing")
	require.Equal(t, encounter.MemberID("alice"), standing.Member)

	_, err := c.mgr.Exit(ctx, &session.ExitInput{Session: "sess", Member: "alice"})
	require.NoError(t, err, "alice leaves the run")
	require.Nil(t, frontGateAddress(t, c.world(t)), "the stored run loads, and her leaving ended the address")

	require.NotEmpty(t, c.bar(t, "cleric").Declarations, "the cleric's bar is offered")
	walked := c.walkMemberTo(t, "cleric", "pedlar")
	require.NotEmpty(t, walked.Steps, "the cleric walks")
	_, err = speakAsOffered(ctx, c.mgr, &session.SpeakInput{Session: "sess", Member: "cleric", Target: "pedlar", OptionID: "tonight"})
	require.NoError(t, err, "the cleric speaks")
	_, err = c.mgr.Exit(ctx, &session.ExitInput{Session: "sess", Member: "cleric"})
	require.NoError(t, err, "the cleric leaves the run")
	c.world(t)
}

// TestCastleTheHolderOfAWaitingRollMayLeaveAndTheTablePlaysOn: the gate
// slice's one exception (rpg-project#924) — the speaker whose spoken check
// waits on her held benefit may leave. The stored run loads after she does,
// and alice can still see her bar, walk, speak and leave.
func TestCastleTheHolderOfAWaitingRollMayLeaveAndTheTablePlaysOn(t *testing.T) {
	c := theClericsWaitingCase(t, 2)
	ctx := context.Background()

	_, err := c.mgr.Exit(ctx, &session.ExitInput{Session: "sess", Member: "cleric"})
	require.NoError(t, err, "the cleric leaves while her roll waits")
	require.Zero(t, c.reservations(t), "the stored run loads, with her hold dropped")

	c.mgr = c.manager(t)
	require.NotEmpty(t, c.bar(t, "alice").Declarations, "alice's bar is offered")
	walked := c.walkMemberTo(t, "alice", "gate-2")
	require.NotEmpty(t, walked.Steps, "alice walks")
	offered := false
	for _, guard := range []string{"gate-1", "gate-2"} {
		if row, ok := c.guardLine(t, "alice", guard, "vouch"); ok && !row.Pending {
			_, err = speakAsOffered(ctx, c.mgr, &session.SpeakInput{Session: "sess", Member: "alice", Target: guard, OptionID: "vouch"})
			require.NoError(t, err, "alice speaks to %s", guard)
			offered = true
			break
		}
	}
	require.True(t, offered, "precondition: a gate guard offers alice her vouch")
	_, err = c.mgr.Exit(ctx, &session.ExitInput{Session: "sess", Member: "alice"})
	require.NoError(t, err, "alice leaves the run")
	c.world(t)
}
