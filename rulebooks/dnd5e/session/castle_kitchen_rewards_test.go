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

	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/character"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/encounter"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/encounter/dungeonspec"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/session"
)

// castle_kitchen_rewards_test.go is THE KEEP'S KITCHEN REWARDS at the session
// door (rpg-project#1024): alice lends the cook a hand and is "Warmed up"
// (+1 initiative), then tastes the feast and is "Feasted" (+2 maximum hit
// points, 2 healed once, when given) — each on her saved sheet, once, both
// held together, until her run ends. Played once, in order, through the
// session's verbs (the scene is the package's costliest setup, so it is built
// once and each test reads what it left); the sheet is read back from the
// host's character store, as the host reads it.
//
// THE HEAL, ON A HERO AT FULL HEALTH: the castle seats alice at her full 28
// after the launch's first-admission rest, and no route in the castle wounds
// her without a fight. So the heal is proven where it shows here: the feast
// raises her maximum to 30 first, and the heal of 2 then takes her from 28 to
// 30 — a heal of 0 would leave her at 28. That a SECOND tasting heals nothing
// on a wounded speaker is proven on the small site (kitchen_status_test.go),
// where she can be wounded; here a second tasting leaves her at 30 with one
// "Feasted".

const (
	warmedUpName = "Warmed up"
	feastedName  = "Feasted"
)

// kitchenSheet is what one saved sheet shows of the kitchen rewards.
type kitchenSheet struct {
	warmed, feasted         int // status-view rows by name
	hitPoints, maximum      int // current, and the effective maximum
	storedMaximum, initBump int // the saved base, and initiative's held part
}

// kitchenPlay is the scene's findings, built once.
var kitchenPlay struct {
	once sync.Once
	err  error
	// alice's sheet at each step: before eating, after helping, after
	// tasting, after tasting again, after a reload, after she left.
	start, afterHelp, afterTaste, afterAgain, afterReload, afterExit kitchenSheet
	// cleric is the cleric's, who never ate.
	cleric kitchenSheet
}

// kitchenPlayTests are the tests that read [kitchenPlay].
var kitchenPlayTests = []string{
	"TestCastleHelpingTheCookLeavesTheHelperWarmedUp",
	"TestCastleTastingTheFeastRaisesTheMaximumAndHeals",
	"TestCastleASecondTastingChangesNothing",
	"TestCastleTheKitchenRewardsSurviveAReload",
	"TestCastleTheRunsEndTakesBothOff",
}

// buildTheKitchenPlay plays it all. It takes no *testing.T because
// [TestMain] runs it.
func buildTheKitchenPlay() { kitchenPlay.err = playKitchen() }

// theKitchenPlay is [kitchenPlay], built if this run's TestMain did not.
func theKitchenPlay(t *testing.T) {
	t.Helper()
	kitchenPlay.once.Do(buildTheKitchenPlay)
	require.NoError(t, kitchenPlay.err, "the kitchen play")
}

//nolint:gocyclo // one scene, played in order
func playKitchen() error {
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
		live: &actCapture{}, dice: &castleDice{d20s: []int{18}},
		characters: newFakeCharacters(armedFighter("alice"), healingCleric()),
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

	steps := []struct {
		say  string // the line alice says first, if any
		into *kitchenSheet
	}{
		{"", &kitchenPlay.start},
		{"help", &kitchenPlay.afterHelp},
		{"taste", &kitchenPlay.afterTaste},
		{"taste", &kitchenPlay.afterAgain},
	}
	for _, step := range steps {
		if step.say != "" {
			if err := c.sayToTheCook(c.mgr, "alice", step.say); err != nil {
				return err
			}
		}
		if *step.into, err = c.kitchenSheet("alice"); err != nil {
			return err
		}
	}
	if kitchenPlay.cleric, err = c.kitchenSheet("cleric"); err != nil {
		return err
	}

	reloaded, err := c.newManager()
	if err != nil {
		return err
	}
	if kitchenPlay.afterReload, err = c.kitchenSheet("alice"); err != nil {
		return err
	}

	// THE RUN ENDS FOR HER: the castle declares no ending a host can fire,
	// so her own leaving is the end of her run, through the same departure
	// End takes every member through.
	if _, err := reloaded.Exit(ctx, &session.ExitInput{Session: "sess", Member: "alice"}); err != nil {
		return fmt.Errorf("exit: %w", err)
	}
	kitchenPlay.afterExit, err = c.kitchenSheet("alice")
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

// kitchenSheet reads a member's saved sheet as the host would: loaded from
// the character store, asked for its status view, its maximum and its
// initiative.
func (c *freeRoamCastle) kitchenSheet(member string) (kitchenSheet, error) {
	stored := c.characters.byID[member]
	loaded, err := character.Load(context.Background(), stored)
	if err != nil {
		return kitchenSheet{}, err
	}
	view, err := loaded.StatusView(&character.StatusViewInput{})
	if err != nil {
		return kitchenSheet{}, err
	}
	out := kitchenSheet{hitPoints: loaded.GetHitPoints(), maximum: loaded.GetMaxHitPoints(), storedMaximum: stored.MaxHitPoints}
	for _, condition := range view.View.Conditions {
		switch condition.Name {
		case warmedUpName:
			out.warmed++
		case feastedName:
			out.feasted++
		}
	}
	initiative, err := loaded.Initiative()
	if err != nil {
		return kitchenSheet{}, err
	}
	for _, part := range initiative.Contributions {
		if part.Source == warmedUpName {
			out.initBump += part.Amount
		}
	}
	return out, nil
}

// sayToTheCook has member say option to the cook on mgr, as offered.
func (c *freeRoamCastle) sayToTheCook(mgr *session.Manager, member, option string) error {
	if _, err := speakAsOffered(context.Background(), mgr, &session.SpeakInput{
		Session: "sess", Member: member, Target: "cook", OptionID: option,
	}); err != nil {
		return fmt.Errorf("%s says %s: %w", member, option, err)
	}
	return nil
}

func TestCastleHelpingTheCookLeavesTheHelperWarmedUp(t *testing.T) {
	theKitchenPlay(t)
	start, help := kitchenPlay.start, kitchenPlay.afterHelp
	require.Zero(t, start.warmed+start.feasted, "nothing before she eats")
	require.Equal(t, 1, help.warmed, "alice is warmed up, shown once")
	require.Equal(t, 1, help.initBump, "and it adds 1 to her initiative")
	require.Zero(t, help.feasted, "helping is not tasting")
	require.Zero(t, kitchenPlay.cleric.warmed+kitchenPlay.cleric.feasted, "the cleric never ate")
}

func TestCastleTastingTheFeastRaisesTheMaximumAndHeals(t *testing.T) {
	theKitchenPlay(t)
	start, taste := kitchenPlay.start, kitchenPlay.afterTaste
	require.Equal(t, 1, taste.feasted, "alice is feasted, shown once")
	require.Equal(t, 1, taste.warmed, "and still warmed up: the two are held together")
	require.Equal(t, start.maximum+2, taste.maximum, "her maximum is 2 higher")
	require.Equal(t, start.storedMaximum, taste.storedMaximum, "the saved maximum is still the base")
	require.Equal(t, start.maximum, start.hitPoints, "precondition: she starts at full")
	require.Equal(t, start.hitPoints+2, taste.hitPoints, "the heal of 2 took her to the raised maximum")
}

func TestCastleASecondTastingChangesNothing(t *testing.T) {
	theKitchenPlay(t)
	require.Equal(t, kitchenPlay.afterTaste, kitchenPlay.afterAgain, "one Feasted, the same maximum and hit points; the line was said without error")
}

func TestCastleTheKitchenRewardsSurviveAReload(t *testing.T) {
	theKitchenPlay(t)
	require.Equal(t, kitchenPlay.afterTaste, kitchenPlay.afterReload, "the saved sheet holds both, the raised maximum and the healed hit points")
}

func TestCastleTheRunsEndTakesBothOff(t *testing.T) {
	theKitchenPlay(t)
	start, exit := kitchenPlay.start, kitchenPlay.afterExit
	require.Zero(t, exit.warmed+exit.feasted, "her run's end took both off her saved sheet")
	require.Zero(t, exit.initBump)
	require.Equal(t, start.maximum, exit.maximum, "the maximum is back to the base")
	require.Equal(t, start.maximum, exit.hitPoints, "and her hit points are capped at it")
}

// A STATUS THE RULES REFUSE IS REFUSED BEFORE ANYTHING RUNS: by the content
// check a studio asks on save, and by launch, each naming the author's path.
func TestCastleAStatusTheRulesRefuseIsRefusedAtLaunchAndByTheContentCheck(t *testing.T) {
	raw := castleSource(t)
	good := []byte("      - initiative: 1\n")
	require.Equal(t, 1, bytes.Count(raw, good), "precondition: the cook's warmed-up status, once")
	bad := bytes.Replace(raw, good, []byte("      - initiative: 0\n"), 1)
	compiled, err := dungeonspec.Load(bad)
	require.NoError(t, err, "the site compiles: the words inside a status are the rules' to judge")

	checked, err := session.CheckContent(&session.CheckContentInput{Dungeon: &compiled})
	require.NoError(t, err)
	require.NotEmpty(t, checked.Refusals)
	for _, r := range checked.Refusals {
		require.Equal(t, "warmed-up", r.StatusID)
		require.Equal(t, "statuses.warmed-up", r.Path)
		require.Contains(t, r.Message, "changes[0].initiative")
	}

	c := &freeRoamCastle{live: &actCapture{}, characters: newFakeCharacters(armedFighter("alice")), encounters: newFakeEncounters(),
		dice: &castleDice{d20s: []int{18}}, sessions: newFakeSessions(), seats: newFakeSeats()}
	_, err = c.manager(t).Launch(context.Background(), &session.LaunchInput{
		Session: "sess", Dungeon: &compiled, Party: []string{"alice"},
	})
	require.ErrorIs(t, err, session.ErrUnknownContent)
	require.ErrorContains(t, err, "changes[0].initiative")
}
