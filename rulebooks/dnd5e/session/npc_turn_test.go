// Copyright (C) 2026 Kirk Diggler
// SPDX-License-Identifier: GPL-3.0-or-later

package session

// This file is in the INTERNAL package, like npc_pays_test.go: the facts test
// and the Dash seam test need openForWrite and the seams, which are
// unexported, and the played tests use the same stores and fixtures
// (rpg-project#1209, "NPC turn 1d-4").

import (
	"context"
	"encoding/json"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/KirkDiggler/rpg-toolkit/core"
	"github.com/KirkDiggler/rpg-toolkit/mind/behavior"
	"github.com/KirkDiggler/rpg-toolkit/mind/machine"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/character"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/combat"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/encounter"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/encounter/dungeonspec"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/monster"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/refs"
)

// economyOf is a stored sheet's economy with the given slots left.
func economyOf(action, bonus, reaction int) *combat.ActionEconomy {
	economy := combat.NewActionEconomy()
	economy.ActionsRemaining = action
	economy.BonusActionsRemaining = bonus
	economy.ReactionsRemaining = reaction
	return economy
}

// Test 1: what the session tells the encounter about a monster's turn is what
// its stored sheet holds, and a player is told nothing about a turn.
func TestTheFactsAreTheStoredSheetsTurn(t *testing.T) {
	f := newNPCPaysFixture(t, "gob-1", refs.Monsters.Goblin().String())
	seam := sheetsBeside(f.scope.standing)
	turnOf := func() *encounter.TurnFacts {
		facts, err := seam.readFacts("gob-1")
		require.NoError(t, err)
		return facts.Turn
	}

	require.Equal(t, &encounter.TurnFacts{ActionsLeft: 1, BonusActionsLeft: 1}, turnOf(), "nothing spent")

	f.sheet(t, "gob-1").Economy = economyOf(0, 1, 1)
	require.Equal(t, &encounter.TurnFacts{ActionsLeft: 0, BonusActionsLeft: 1}, turnOf(), "its action spent")

	f.sheet(t, "gob-1").Economy = economyOf(1, 0, 1)
	require.Equal(t, &encounter.TurnFacts{ActionsLeft: 1, BonusActionsLeft: 0}, turnOf(), "its bonus action spent")

	player, err := seam.readFacts("fighter")
	require.NoError(t, err)
	require.Nil(t, player.Turn, "a player's turn is paid through its own verbs; nil is a full turn")
}

func (f *npcPaysFixture) dash(id string) error {
	return (strikerSeam{m: f.mgr, scope: f.scope}).Dash(context.Background(), f.scope.enc, encounter.MemberID(id))
}

// Test 7: the Dash seam alone. A run is one action off the stored sheet, the
// seam tells nothing, a second run the same turn is refused and changes
// nothing, and a stranger and a player are refused as Disengage refuses them.
func TestTheDashSeamChargesTheActionAtTheDoor(t *testing.T) {
	f := newNPCPaysFixture(t, "gob-1", refs.Monsters.Goblin().String())
	before := f.storyLen(t)

	require.NoError(t, f.dash("gob-1"))

	requireSlots(t, f.sheet(t, "gob-1"), 0, 1, 1)
	require.Equal(t, before, f.storyLen(t), "the seam tells nothing: the encounter tells the run in its walk's steps")
	afterFirst := f.sheetJSON(t, "gob-1")

	require.ErrorIs(t, f.dash("gob-1"), ErrCannotAfford)

	requireSlots(t, f.sheet(t, "gob-1"), 0, 1, 1)
	require.JSONEq(t, afterFirst, f.sheetJSON(t, "gob-1"), "a refused run changes nothing")
	require.Equal(t, before, f.storyLen(t))

	require.ErrorIs(t, f.dash("nobody"), ErrNoMember)
	require.ErrorIs(t, f.dash("fighter"), ErrNoSheet)
}

// ---- played tests ----

// npcGame is a fighter and one monster, launched through the manager and
// played through its verbs.
type npcGame struct {
	t          *testing.T
	mgr        *Manager
	sessions   *strikeSessions
	encounters *strikeEncounters
	characters *strikeCharacters
}

// newNPCGame launches a fighter at (0,0) and the placement. A fight forms only
// from contact, so when onClock is set the stored run gets one authored turn
// clock, fighter first, as the suites that are not about how a fight forms do.
func newNPCGame(t *testing.T, driver TurnDriver, placement dungeonspec.MonsterPlacement, fighterCol int, onClock bool) *npcGame {
	t.Helper()
	g := &npcGame{
		t:          t,
		sessions:   &strikeSessions{byID: map[string]*SessionData{}},
		encounters: &strikeEncounters{byID: map[string]*encounter.EncounterData{}},
		characters: &strikeCharacters{byID: map[string]*character.Data{"fighter": strikeFixtureFighter("fighter")}},
	}
	rolls := []int{20, 20, 20, 20, 20, 20, 20, 20, 20, 20, 20, 20}
	for range 80 {
		rolls = append(rolls, 1)
	}
	mgr, err := NewManager(&Config{Record: DiscardRecord{}, Acts: DiscardActs{}, Seats: newFakeSeats(), CharacterConfig: newFakeCharacterConfig(), PresentationIDs: testPresentationIDs{},
		Dice: &scriptedDice{rolls: rolls}, TurnDriver: driver,
		Sessions: g.sessions, Encounters: g.encounters, Characters: g.characters,
	})
	require.NoError(t, err)
	g.mgr = mgr
	out := launchScene(t, mgr, scene{
		Field:    encounter.FieldInput{Canvas: pointyCanvas(), Regions: []encounter.RegionInput{rectRegion("tomb", 0, 0, 30, 6)}},
		Party:    []sceneSeat{seatAt("fighter", fighterCol, 0)},
		Monsters: []dungeonspec.MonsterPlacement{placement},
		Endings:  []encounter.EndingInput{{Key: "withdraw", Trigger: encounter.TriggerExternal{}}},
	})
	if onClock {
		authorTurnClock(t, g.encounters.byID["sess"], []string{"fighter", placement.MemberID}, 0)
	} else {
		require.NotEmpty(t, out.Formed, "adjacent and in sight starts the fight")
	}
	g.characters.byID["fighter"].ActionEconomy = &character.ActionEconomyData{
		TurnNumber: 1, ActionsRemaining: 1, BonusActionsRemaining: 1, ReactionsRemaining: 1, MovementRemaining: 30,
	}
	return g
}

// endRound ends the fighter's turn, which plays the monster's.
func (g *npcGame) endRound() error {
	ctx := context.Background()
	_, err := g.mgr.EndTurn(ctx, &EndTurnInput{Session: "sess", Member: "fighter",
		DeclarationID: declarationOf(g.t, g.mgr, "fighter", VerbEndTurn).ID})
	return err
}

// story is the fighter's view of the run's story, one decoded payload each.
func (g *npcGame) story() []map[string]any {
	g.t.Helper()
	enc, err := g.mgr.LoadWorldForTest(context.Background(), "sess")
	require.NoError(g.t, err)
	story, err := enc.Story(&encounter.StoryInput{Audience: "fighter"})
	require.NoError(g.t, err)
	out := make([]map[string]any, 0, len(story))
	for _, entry := range story {
		var fields map[string]any
		require.NoError(g.t, json.Unmarshal(entry.Payload, &fields))
		out = append(out, fields)
	}
	return out
}

// swings counts the swings the member has taken in the story so far.
func (g *npcGame) swings(member string) int {
	n := 0
	for _, fields := range g.story() {
		if (fields["beat"] == "struck" || fields["beat"] == "missed") && fields["actor"] == member {
			n++
		}
	}
	return n
}

// sheetOf is the member's stored sheet right now.
func (g *npcGame) sheetOf(member string) *monster.Data {
	g.t.Helper()
	stored, err := g.sessions.GetSession(context.Background(), "sess")
	require.NoError(g.t, err)
	data, found := npcSheet(stored, member)
	require.True(g.t, found)
	return data
}

// alwaysSwings answers Attack every time it is asked, and counts the asks.
type alwaysSwings struct{ asks int }

func (w *alwaysSwings) Act(view MonsterView) (TurnIntent, error) {
	w.asks++
	return Attack{Target: "fighter", Action: view.Actions[0].Ref}, nil
}

// Test 2: one swing a turn, by the sheet. A goblin whose driver answers Attack
// every time it is asked swings once in a turn, the verb does not fail, and it
// swings once again in the next round.
func TestAGoblinSwingsOnceATurnBecauseItsSheetSaysSo(t *testing.T) {
	driver := &alwaysSwings{}
	g := newNPCGame(t, driver, monsterAt("gob-1", refs.Monsters.Goblin().String(), 2, 0), 1, false)

	require.NoError(t, g.endRound())
	require.Equal(t, 1, g.swings("gob-1"), "round one: one swing, and the verb did not fail")
	requireSlots(t, g.sheetOf("gob-1"), 0, 1, 1)

	require.NoError(t, g.endRound())
	require.Equal(t, 2, g.swings("gob-1"), "round two: one more swing")
	requireSlots(t, g.sheetOf("gob-1"), 0, 1, 1)
	require.Positive(t, driver.asks)
}

// tableBehavior is the one behavior instance a placement is given in these
// tests: a table answered at the creature's own time, and nothing else, as
// launchscene_test.go's ordinaryBehavior builds it.
func tableBehavior(table encounter.Table) *encounter.BehaviorInput {
	return &encounter.BehaviorInput{Instances: []encounter.BoundBehavior{{Instance: behavior.InstanceSpec{
		ID: "policy", Table: table,
		Recipe: machine.Recipe{Ref: core.Ref{Module: "test", Type: "behaviors", ID: "npc-turn"}, Initial: "ready",
			States: map[string]machine.State{"ready": {Transitions: []machine.Transition{
				{On: "time", Do: []machine.Act{{Pick: "time"}}},
				{On: "turn", Do: []machine.Act{{Pick: "time"}}},
			}}}},
	}}}}
}

// swingsOrRuns is a table with a swing when a foe is in reach and a run at the
// nearest foe with no condition at all, so a creature that can afford both is
// given the run by the top die face (the scripted dice roll twenty).
func swingsOrRuns() encounter.Table {
	return encounter.Table{encounter.AnswerTime: {
		{Weight: 1, When: []encounter.When{{Enemy: encounter.EnemyReach}}, Attack: &encounter.Selector{Word: encounter.SelectorEnemy}},
		{Weight: 1, Dash: &encounter.Selector{Word: encounter.SelectorEnemy}},
	}}
}

// dashBeats is the story's moved beats whose cause is the table's run.
func (g *npcGame) dashBeats(member string) int {
	n := 0
	for _, fields := range g.story() {
		if fields["beat"] == "moved" && fields["member"] == member && fields["cause"] == "encounter:table:dash" {
			n++
		}
	}
	return n
}

// Test 4: a run costs the action, at the door. A zombie too far to walk to the
// fighter but near enough to run runs; the story shows the run, its stored
// sheet has no action left, nothing it does that turn is a swing, and the
// next round it has its action again.
func TestARunCostsTheActionAtTheDoor(t *testing.T) {
	placement := monsterAt("zom-1", refs.Monsters.Zombie().String(), 8, 0)
	placement.Behaviors = tableBehavior(swingsOrRuns())
	g := newNPCGame(t, Driver(), placement, 1, true)

	require.NoError(t, g.endRound())

	require.NotZero(t, g.dashBeats("zom-1"), "the story shows the run: its steps name the run as their cause")
	requireSlots(t, g.sheetOf("zom-1"), 0, 1, 1)
	require.Zero(t, g.swings("zom-1"), "the action went on the run: nothing was offered to swing with")

	// The door agrees with the story: a swing the same turn is refused there.
	scope, err := g.mgr.openForWrite(context.Background(), "sess")
	require.NoError(t, err)
	err = (strikerSeam{m: g.mgr, scope: scope}).Strike(context.Background(), scope.enc, "zom-1", "fighter", *refs.MonsterActions.ZombieSlam())
	require.ErrorIs(t, err, ErrCannotAfford)

	// Round two: the turn is the sheet's again, and the zombie is beside the
	// fighter, so it swings with it.
	require.NoError(t, g.endRound())
	require.Equal(t, 1, g.swings("zom-1"), "round two: its action came back, and it swung")
	requireSlots(t, g.sheetOf("zom-1"), 0, 1, 1)
}

// Test 5: a swing, then no run. A zombie beside the fighter swings; the table
// would run it if it could (its run row has no condition), yet it neither runs
// nor swings again, the verb does not fail, and only the one action is
// charged.
func TestASwingThenNoRun(t *testing.T) {
	placement := monsterAt("zom-1", refs.Monsters.Zombie().String(), 2, 0)
	placement.Behaviors = tableBehavior(swingsOrRuns())
	g := newNPCGame(t, Driver(), placement, 1, false)

	require.NoError(t, g.endRound())

	require.Equal(t, 1, g.swings("zom-1"), "one swing")
	require.Zero(t, g.dashBeats("zom-1"), "no run after the swing")
	requireSlots(t, g.sheetOf("zom-1"), 0, 1, 1)
}

// Test 5, at the door: the same refusal the encounter's backstop would meet.
// A run asked of a sheet whose action is spent is refused and charges
// nothing.
func TestARunAfterASwingIsRefusedAtTheDoorAndChargesNothing(t *testing.T) {
	f := newNPCPaysFixture(t, "skel-1", refs.Monsters.Skeleton().String())
	require.NoError(t, f.strike(t, "skel-1"))
	afterSwing := f.sheetJSON(t, "skel-1")

	require.ErrorIs(t, f.dash("skel-1"), ErrCannotAfford)

	require.JSONEq(t, afterSwing, f.sheetJSON(t, "skel-1"), "nothing is charged for the refused run")
}

// Test 3, at the door (see the gap on rpg-project#1209: a monster's own turn
// start drops its stored account, so two actions cannot be hand-set into a
// played turn). The facts say what the sheet holds, and the door charges each
// swing against it: two actions, two swings, and not a third.
func TestTwoActionsOnTheSheetAreTwoSwingsAtTheDoor(t *testing.T) {
	f := newNPCPaysFixture(t, "skel-1", refs.Monsters.Skeleton().String())
	f.sheet(t, "skel-1").Economy = economyOf(2, 1, 1)
	facts, err := sheetsBeside(f.scope.standing).readFacts("skel-1")
	require.NoError(t, err)
	require.Equal(t, &encounter.TurnFacts{ActionsLeft: 2, BonusActionsLeft: 1}, facts.Turn)

	require.NoError(t, f.strike(t, "skel-1"))
	requireSlots(t, f.sheet(t, "skel-1"), 1, 1, 1)
	require.NoError(t, f.strike(t, "skel-1"))
	requireSlots(t, f.sheet(t, "skel-1"), 0, 1, 1)
	require.ErrorIs(t, f.strike(t, "skel-1"), ErrCannotAfford, "a third is not paid for")
}

// Test 6, at the door (see the gap on rpg-project#1209: a goblin's shortbow
// keeps every foe within its reach, so its table never runs): a run and a
// Disengage in one turn are both paid, an action and a bonus action, by a
// goblin with Nimble Escape.
func TestARunAndADisengageInOneTurnAreBothPaid(t *testing.T) {
	f := newNPCPaysFixture(t, "gob-1", refs.Monsters.Goblin().String())

	require.NoError(t, f.disengage("gob-1"))
	require.NoError(t, f.dash("gob-1"))

	requireSlots(t, f.sheet(t, "gob-1"), 0, 0, 1)
}
