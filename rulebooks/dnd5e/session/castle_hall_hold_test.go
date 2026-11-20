// Copyright (C) 2026 Kirk Diggler
// SPDX-License-Identifier: GPL-3.0-or-later

package session_test

import (
	"context"
	"fmt"
	"sync"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/character"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/encounter/dungeonspec"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/session"
	"github.com/KirkDiggler/rpg-toolkit/tools/spatial"
)

// castle_hall_hold_test.go is THE KEEP'S HOLD on the castle's own content
// (rpg-project#990), end to end through the session's verbs: alice shoulders
// past the great hall guard (`push-past` on `hall-1`) and fails, and is Held;
// one step is refused; she attacks him, the fight forms, and she is Held no
// longer. Only what the castle alone can prove: the wiring — the refusal's
// shape, a reload, speaking, sneaking, the Move offer, a held monster — is
// proven on the small site (hold_status_test.go), and a success giving
// nothing by the encounter's own castle test. Played once, prebuilt by
// [TestMain] (the castle is the package's costliest setup).

const heldName = "Held"

// hallHold is the scene's findings, built once.
var hallHold struct {
	once sync.Once
	err  error
	// alice's Held rows on her saved sheet after failing and after the fight
	// formed; her refused step, and where she stood before and after it.
	heldAfterFail, heldAfterFight int
	walkErr                       error
	before, afterWalk             spatial.Position
	// formed is whether attacking the guard formed a fight.
	formed bool
}

// hallHoldTests are the tests that read [hallHold].
var hallHoldTests = []string{"TestCastleShoulderingPastTheHallGuardHoldsTheSpeakerUntilAFight"}

// buildTheHallHold plays it. It takes no *testing.T because [TestMain] runs it.
func buildTheHallHold() { hallHold.err = playHallHold() }

// theHallHold is [hallHold], built if this run's TestMain did not.
func theHallHold(t *testing.T) {
	t.Helper()
	hallHold.once.Do(buildTheHallHold)
	require.NoError(t, hallHold.err, "the hall hold play")
}

func playHallHold() error {
	ctx := context.Background()
	raw, err := castleSourceOrError()
	if err != nil {
		return err
	}
	compiled, err := dungeonspec.Load(raw)
	if err != nil {
		return err
	}
	c := &freeRoamCastle{
		// alice fails her check; every die after is high.
		live: &actCapture{}, dice: &castleDice{d20s: []int{2, 18}},
		characters: newFakeCharacters(armedFighter("alice"), healingCleric()),
		encounters: newFakeEncounters(), sessions: newFakeSessions(), seats: newFakeSeats(),
	}
	if c.mgr, err = c.newManager(); err != nil {
		return err
	}
	if _, err := c.mgr.Launch(ctx, &session.LaunchInput{Session: "sess", Dungeon: &compiled, Party: []string{"alice", "cleric"}}); err != nil {
		return fmt.Errorf("launch: %w", err)
	}
	guard, err := c.mgr.Where(ctx, &session.WhereInput{Session: "sess", Member: "hall-1"})
	if err != nil {
		return err
	}
	if err := c.setCell("alice", guard.Position.X-1, guard.Position.Y); err != nil {
		return err
	}
	if _, err := speakAsOffered(ctx, c.mgr, &session.SpeakInput{Session: "sess", Member: "alice", Target: "hall-1", OptionID: "push-past"}); err != nil {
		return fmt.Errorf("alice shoulders past: %w", err)
	}
	if hallHold.heldAfterFail, err = c.heldRows("alice"); err != nil {
		return err
	}
	at, err := c.mgr.Where(ctx, &session.WhereInput{Session: "sess", Member: "alice"})
	if err != nil {
		return err
	}
	hallHold.before = at.Position
	_, hallHold.walkErr = c.mgr.Move(ctx, &session.MoveInput{Session: "sess", Member: "alice",
		Path: []spatial.Position{{X: at.Position.X, Y: at.Position.Y + 1}}})
	if at, err = c.mgr.Where(ctx, &session.WhereInput{Session: "sess", Member: "alice"}); err != nil {
		return err
	}
	hallHold.afterWalk = at.Position

	// THE GUARD WALKS HIS ROUNDS, and the line passed a round of the world,
	// so he walked on. The host stands him back on his first cell, beside
	// alice: this scene is about the hold, not about catching him up.
	if err := c.setCell("hall-1", guard.Position.X, guard.Position.Y); err != nil {
		return err
	}
	attack, err := attackRowFor(ctx, c.mgr, "alice", "hall-1")
	if err != nil {
		return err
	}
	if _, err := c.mgr.Attack(ctx, &session.AttackInput{Session: "sess", Attacker: "alice", Target: "hall-1", DeclarationID: attack}); err != nil {
		return fmt.Errorf("alice attacks the guard: %w", err)
	}
	turn, err := c.mgr.Turn(ctx, &session.TurnInput{Session: "sess", Member: "alice"})
	if err != nil {
		return err
	}
	hallHold.formed = turn.Clock == session.ClockTurn
	hallHold.heldAfterFight, err = c.heldRows("alice")
	return err
}

// heldRows counts Held among the conditions member's saved sheet shows, as
// the host reads it: one status is one row, whatever halves it is stored as.
func (c *freeRoamCastle) heldRows(member string) (int, error) {
	stored, err := c.characters.GetCharacter(context.Background(), member)
	if err != nil {
		return 0, err
	}
	loaded, err := character.Load(context.Background(), stored)
	if err != nil {
		return 0, err
	}
	view, err := loaded.StatusView(&character.StatusViewInput{})
	if err != nil {
		return 0, err
	}
	n := 0
	for _, condition := range view.View.Conditions {
		if condition.Name == heldName {
			n++
		}
	}
	return n, nil
}

// attackRowFor is member's Attack row id with target among its candidates.
func attackRowFor(ctx context.Context, mgr *session.Manager, member, target string) (string, error) {
	bar, err := mgr.Afford(ctx, &session.AffordInput{Session: "sess", Member: member})
	if err != nil {
		return "", err
	}
	for _, d := range bar.Declarations {
		if d.Verb != session.VerbAttack {
			continue
		}
		for _, candidate := range d.Candidates {
			if candidate.Member == target && candidate.Available {
				return d.ID, nil
			}
		}
	}
	return "", fmt.Errorf("%s is offered no attack on %s", member, target)
}

func TestCastleShoulderingPastTheHallGuardHoldsTheSpeakerUntilAFight(t *testing.T) {
	theHallHold(t)
	require.Equal(t, 1, hallHold.heldAfterFail, "a failed push-past leaves alice Held, once, on her saved sheet")
	require.ErrorIs(t, hallHold.walkErr, session.ErrCannotMove, "her step is refused")
	require.Equal(t, hallHold.before, hallHold.afterWalk, "she has not moved")
	require.True(t, hallHold.formed, "attacking the guard forms a fight")
	require.Zero(t, hallHold.heldAfterFight, "the fight starting ended Held, on her saved sheet")
}
