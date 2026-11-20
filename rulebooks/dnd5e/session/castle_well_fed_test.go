// Copyright (C) 2026 Kirk Diggler
// SPDX-License-Identifier: GPL-3.0-or-later

package session_test

import (
	"bytes"
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
)

// castle_well_fed_test.go is THE COOK'S PIES at the session door
// (rpg-project#1024): alice lends the cook a hand and is well fed — on her
// saved sheet, once, giving advantage on her Charisma checks — until her run
// ends. Played once, in order, through the session's verbs (the scene is the
// package's costliest setup, so it is built once and each test reads what it
// left); the sheet is read back from the host's character store, as the host
// reads it.

const wellFed = "Well fed"

// wellFedPlay is the scene's findings, built once.
var wellFedPlay struct {
	once sync.Once
	err  error
	// rows are alice's "Well fed" status-view rows from her saved sheet:
	// after helping, after helping again, after a reload, after she left.
	afterHelp, afterAgain, afterReload, afterExit int
	// clericRows are the cleric's, who never helped.
	clericRows int
	// gossip is whether each Charisma check carried advantage naming the
	// status: alice's, the cleric's, and alice's after a reload.
	aliceGossip, clericGossip, reloadedGossip bool
}

// wellFedPlayTests are the tests that read [wellFedPlay].
var wellFedPlayTests = []string{
	"TestCastleHelpingTheCookLeavesTheHelperWellFed",
	"TestCastleASecondHelpingGivesNothingMore",
	"TestCastleWellFedSurvivesAReload",
	"TestCastleTheRunsEndTakesWellFedOff",
}

// buildTheWellFedPlay plays it all. It takes no *testing.T because
// [TestMain] runs it.
func buildTheWellFedPlay() { wellFedPlay.err = playWellFed() }

// theWellFedPlay is [wellFedPlay], built if this run's TestMain did not.
func theWellFedPlay(t *testing.T) {
	t.Helper()
	wellFedPlay.once.Do(buildTheWellFedPlay)
	require.NoError(t, wellFedPlay.err, "the well-fed play")
}

//nolint:gocyclo // one scene, played in order
func playWellFed() error {
	ctx := context.Background()
	raw, err := castleSourceOrError()
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
	if c.mgr, err = c.newManager(); err != nil {
		return err
	}
	if _, err := c.mgr.Launch(ctx, &session.LaunchInput{Session: "sess", Dungeon: &compiled, Party: []string{"alice", "cleric"}}); err != nil {
		return fmt.Errorf("launch: %w", err)
	}
	cook, err := c.mgr.Where(ctx, &session.WhereInput{Session: "sess", Member: "cook"})
	if err != nil {
		return err
	}
	if err := c.setCell("alice", cook.Position.X-1, cook.Position.Y); err != nil {
		return err
	}
	if err := c.setCell("cleric", cook.Position.X+1, cook.Position.Y); err != nil {
		return err
	}

	if _, err := c.sayToTheCook(c.mgr, "alice", "help"); err != nil {
		return err
	}
	if wellFedPlay.afterHelp, err = c.wellFedRows("alice"); err != nil {
		return err
	}
	if wellFedPlay.aliceGossip, err = c.sayToTheCook(c.mgr, "alice", "gossip"); err != nil {
		return err
	}
	if wellFedPlay.clericGossip, err = c.sayToTheCook(c.mgr, "cleric", "gossip"); err != nil {
		return err
	}
	if wellFedPlay.clericRows, err = c.wellFedRows("cleric"); err != nil {
		return err
	}
	if _, err := c.sayToTheCook(c.mgr, "alice", "help"); err != nil {
		return fmt.Errorf("a second helping: %w", err)
	}
	if wellFedPlay.afterAgain, err = c.wellFedRows("alice"); err != nil {
		return err
	}

	reloaded, err := c.newManager()
	if err != nil {
		return err
	}
	if wellFedPlay.afterReload, err = c.wellFedRows("alice"); err != nil {
		return err
	}
	if wellFedPlay.reloadedGossip, err = c.sayToTheCook(reloaded, "alice", "gossip"); err != nil {
		return err
	}

	// THE RUN ENDS FOR HER: the castle declares no ending a host can fire,
	// so her own leaving is the end of her run, through the same departure
	// End takes every member through.
	if _, err := reloaded.Exit(ctx, &session.ExitInput{Session: "sess", Member: "alice"}); err != nil {
		return fmt.Errorf("exit: %w", err)
	}
	wellFedPlay.afterExit, err = c.wellFedRows("alice")
	return err
}

// castleSourceOrError is [castleSource] without a *testing.T.
func castleSourceOrError() ([]byte, error) {
	dir, err := exec.Command("go", "list", "-m", "-f", "{{.Dir}}", encounterModule).Output()
	if err != nil {
		return nil, fmt.Errorf("find the encounter module: %w", err)
	}
	return os.ReadFile(filepath.Join(strings.TrimSpace(string(dir)), castleFixture))
}

// newManager is a Manager over the castle's own repositories.
func (c *freeRoamCastle) newManager() (*session.Manager, error) {
	return session.NewManager(&session.Config{Record: session.DiscardRecord{}, Acts: c.live, Seats: c.seats, CharacterConfig: newFakeCharacterConfig(),
		PresentationIDs: testPresentationIDs{}, Dice: c.dice, TurnDriver: session.Driver(),
		Sessions: c.sessions, Encounters: c.encounters, Characters: c.characters,
	})
}

// setCell stands a member on a cell in the saved record.
func (c *freeRoamCastle) setCell(member string, x, y float64) error {
	data, ok := c.encounters.byID["sess"]
	if !ok {
		return fmt.Errorf("the launch saved no world")
	}
	for i := range data.Members {
		if data.Members[i].ID == encounter.MemberID(member) {
			data.Members[i].Cell = &encounter.PositionData{X: x, Y: y}
			return nil
		}
	}
	return fmt.Errorf("%s is not in the record", member)
}

// wellFedRows is how many rows of a member's status view, read from the
// saved sheet, are the status.
func (c *freeRoamCastle) wellFedRows(member string) (int, error) {
	loaded, err := character.Load(context.Background(), c.characters.byID[member])
	if err != nil {
		return 0, err
	}
	view, err := loaded.StatusView(&character.StatusViewInput{})
	if err != nil {
		return 0, err
	}
	rows := 0
	for _, condition := range view.View.Conditions {
		if condition.Name == wellFed {
			rows++
		}
	}
	return rows, nil
}

// sayToTheCook has member say option to the cook on mgr and reports whether
// the line's own roll carried an advantage naming the status.
func (c *freeRoamCastle) sayToTheCook(mgr *session.Manager, member, option string) (bool, error) {
	c.live.mu.Lock()
	c.live.acts = nil
	c.live.mu.Unlock()
	if _, err := speakAsOffered(context.Background(), mgr, &session.SpeakInput{
		Session: "sess", Member: member, Target: "cook", OptionID: option,
	}); err != nil {
		return false, fmt.Errorf("%s says %s: %w", member, option, err)
	}
	c.live.mu.Lock()
	defer c.live.mu.Unlock()
	for _, act := range c.live.acts {
		line, ok := act.Body.(session.SpeechAct)
		if !ok || act.Recipient != member || line.Speaker.ID != member || line.Option != option {
			continue
		}
		if line.Roll == nil {
			return false, fmt.Errorf("%s's %s carries no roll", member, option)
		}
		for _, influence := range line.Roll.Influences {
			if influence.Kind == session.InfluenceAdvantage && influence.Name == wellFed {
				return true, nil
			}
		}
		return false, nil
	}
	return false, fmt.Errorf("%s was sent no act of their own %s", member, option)
}

func TestCastleHelpingTheCookLeavesTheHelperWellFed(t *testing.T) {
	theWellFedPlay(t)
	require.Equal(t, 1, wellFedPlay.afterHelp, "alice is well fed, shown once")
	require.True(t, wellFedPlay.aliceGossip, "her Charisma check carries it")
	require.False(t, wellFedPlay.clericGossip, "the cleric's does not")
	require.Zero(t, wellFedPlay.clericRows, "the cleric never helped")
}

func TestCastleASecondHelpingGivesNothingMore(t *testing.T) {
	theWellFedPlay(t)
	require.Equal(t, 1, wellFedPlay.afterAgain, "still one, and the line was said without error")
}

func TestCastleWellFedSurvivesAReload(t *testing.T) {
	theWellFedPlay(t)
	require.Equal(t, 1, wellFedPlay.afterReload, "the saved sheet holds it")
	require.True(t, wellFedPlay.reloadedGossip, "and it still gives the advantage")
}

func TestCastleTheRunsEndTakesWellFedOff(t *testing.T) {
	theWellFedPlay(t)
	require.Zero(t, wellFedPlay.afterExit, "her run's end took it off her saved sheet")
}

// A STATUS THE RULES REFUSE IS REFUSED BEFORE ANYTHING RUNS: by the content
// check a studio asks on save, and by launch, each naming the author's path.
func TestCastleAStatusTheRulesRefuseIsRefusedAtLaunchAndByTheContentCheck(t *testing.T) {
	raw := castleSource(t)
	good := []byte("      - advantage: checks\n        when: { ability: charisma }\n")
	require.Equal(t, 1, bytes.Count(raw, good), "precondition: the cook's status, once")
	bad := bytes.Replace(raw, good, []byte("      - advantage: checks\n        when: { skill: persuasion }\n"), 1)
	compiled, err := dungeonspec.Load(bad)
	require.NoError(t, err, "the site compiles: the words inside a status are the rules' to judge")

	checked, err := session.CheckContent(&session.CheckContentInput{Dungeon: &compiled})
	require.NoError(t, err)
	require.NotEmpty(t, checked.Refusals)
	for _, r := range checked.Refusals {
		require.Equal(t, "well-fed", r.StatusID)
		require.Equal(t, "statuses.well-fed", r.Path)
		require.Contains(t, r.Message, "changes[0].when.skill")
	}

	c := &freeRoamCastle{live: &actCapture{}, characters: newFakeCharacters(armedFighter("alice")), encounters: newFakeEncounters(),
		dice: &castleDice{d20s: []int{18}}, sessions: newFakeSessions(), seats: newFakeSeats()}
	_, err = c.manager(t).Launch(context.Background(), &session.LaunchInput{
		Session: "sess", Dungeon: &compiled, Party: []string{"alice"},
	})
	require.ErrorIs(t, err, session.ErrUnknownContent)
	require.ErrorContains(t, err, "changes[0].when.skill")
}
