// Copyright (C) 2026 Kirk Diggler
// SPDX-License-Identifier: GPL-3.0-or-later

package encounter_test

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"math/rand"
	"strings"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/KirkDiggler/rpg-toolkit/core"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/encounter"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/encounter/internal/turntest"
	"github.com/KirkDiggler/rpg-toolkit/tools/spatial"
)

// activate_turn_test.go is the encounter's half of a driven turn that uses an
// ability (rpg-project#1131, ADR-0051, slice 3b-3). Modelled on dash_test.go
// and turnfacts_test.go: a hall, alice at the west end, a creature beside her,
// the shared turn host (internal/turntest) answering what is left of a turn,
// and the verbs a player's turn ends with.

var lieutenant = core.EntityID("lieutenant")

const secondWind = "second wind"

// lieutenantTable is the decision record's example: use the ability, walk when
// a walk gets there, swing in reach.
func lieutenantTable() encounter.Table {
	enemy := &encounter.Selector{Word: encounter.SelectorEnemy}

	return encounter.Table{encounter.AnswerTime: {
		{Weight: 1, Activate: secondWind},
		{Weight: 1, When: []encounter.When{{Enemy: encounter.EnemyWalk}}, Toward: enemy},
		{Weight: 1, When: []encounter.When{{Enemy: encounter.EnemyReach}}, Attack: enemy},
	}}
}

// die is what a scene's Roller is: the two rolls of the world's dice.
type die interface {
	Roll(ctx context.Context, size int) (int, error)
	RollN(ctx context.Context, count, size int) ([]int, error)
}

// rollsPlan is a die that answers each roll from a plan, in order: "low" lands
// on the first candidate, "high" on the last. A roll past the plan is "low".
// The creature's own picks are the only rolls a scene makes, one per consult.
type rollsPlan struct {
	plan []string
	next *int
}

func newPlan(plan ...string) rollsPlan { return rollsPlan{plan: plan, next: new(int)} }

func (p rollsPlan) Roll(_ context.Context, size int) (int, error) {
	i := *p.next
	*p.next++
	if i < len(p.plan) && p.plan[i] == "high" {
		return size, nil
	}

	return 1, nil
}

func (p rollsPlan) RollN(ctx context.Context, count, size int) ([]int, error) {
	out := make([]int, count)
	for i := range out {
		out[i], _ = p.Roll(ctx, size)
	}

	return out, nil
}

// switchDie is a die whose behaviour a test swaps between turns.
type switchDie struct{ cur die }

func (d *switchDie) Roll(ctx context.Context, size int) (int, error) { return d.cur.Roll(ctx, size) }

func (d *switchDie) RollN(ctx context.Context, count, size int) ([]int, error) {
	return d.cur.RollN(ctx, count, size)
}

// seededRolls is a die whose every roll comes from one seed.
type seededRolls struct{ r *rand.Rand }

func (d seededRolls) Roll(_ context.Context, size int) (int, error) { return d.r.Intn(size) + 1, nil }

func (d seededRolls) RollN(ctx context.Context, count, size int) ([]int, error) {
	out := make([]int, count)
	for i := range out {
		out[i], _ = d.Roll(ctx, size)
	}

	return out, nil
}

// markingStriker is a Striker that remembers how long the story was when each
// swing happened, so a test can place a swing among the beats.
type markingStriker struct {
	at []int
}

func (m *markingStriker) Strike(
	_ context.Context, enc *encounter.Encounter, _, _ encounter.MemberID, _ core.Ref,
) error {
	story, err := enc.Story(&encounter.StoryInput{Audience: alice})
	if err != nil {
		return err
	}
	m.at = append(m.at, len(story))

	return nil
}

// abilityScene is one hall with alice and the lieutenant.
type abilityScene struct {
	table     encounter.Table
	driver    encounter.Driver
	roller    die
	give      bool
	uses      int
	activator bool
	world     bool
	free      bool
	// perTurn, when set, is what the lieutenant has on every full turn: the
	// host hands it back at each turn's start, so it is how a turn STARTS
	// without a bonus action (a hand-set Leave is wiped when his turn begins).
	perTurn *encounter.TurnFacts
	// ltAt is how far east of alice the lieutenant stands; default beside her.
	ltAt int
}

type builtScene struct {
	enc     *encounter.Encounter
	host    *turntest.Host
	striker *markingStriker
}

func (sc abilityScene) build(t *testing.T) builtScene {
	t.Helper()
	striker := &markingStriker{}
	var driver encounter.Driver = tableDriver()
	if sc.driver != nil {
		driver = sc.driver
	}
	var roller die = rollsLowest{}
	if sc.roller != nil {
		roller = sc.roller
	}
	caps := encounter.Capabilities{
		Sight:     everyoneSeesTheWholeMap{},
		Equipment: encounter.UnobservedEquipment{}, Conditions: encounter.UnobservedEquipment{},
		Sheets: sheetFacts{
			alice:      {SpeedFeet: 30},
			lieutenant: {SpeedFeet: 30, Actions: []encounter.ActionView{{Ref: testMeleeAction, RangeFeet: 5}}},
		},
		Participation: everyoneStanding{},
		Initiative:    orderAsGiven{},
		Driver:        driver,
		Roller:        roller,
		Actors: encounter.Actors{
			Striker: striker, Mover: quietMover{}, Announcer: quietAnnouncer{},
		},
	}
	caps, host := turntest.With(caps)
	if sc.give {
		host.Give(lieutenant, secondWind, "Second Wind", max(sc.uses, 1))
	}
	if sc.free {
		host.FreeActivations = true
	}
	if sc.perTurn != nil {
		host.AllowPerTurn(lieutenant, sc.perTurn.ActionsLeft, sc.perTurn.BonusActionsLeft)
	}
	if sc.activator {
		caps = turntest.WithActivator(caps, host)
	}

	table := sc.table
	if table == nil {
		table = lieutenantTable()
	}
	at := sc.ltAt
	if at == 0 {
		at = 2
	}
	setup := &encounter.SetupInput{
		Field: encounter.FieldInput{
			Canvas:  openAir(),
			Regions: []encounter.RegionInput{rectRegion("hall", 0, 0, 60, 8)},
		},
		Members: []encounter.MemberInput{
			{ID: alice, Kind: encounter.KindPlayer, Position: onRow(1)},
			{ID: lieutenant, Kind: encounter.KindMonster, Position: onRow(at), Behaviors: ordinaryBehavior(table)},
		},
		Endings:      []encounter.EndingInput{{Key: "withdrawn", Trigger: encounter.TriggerExternal{}}},
		Capabilities: caps,
	}
	if sc.world {
		setup.Field.Factions = []encounter.FactionInput{{ID: "vendors", Mind: lieutenant}}
		setup.Field.Dispositions = []encounter.DispositionInput{{
			Between: [2]encounter.FactionID{"vendors", encounter.FactionParty}, Stance: encounter.StanceNeutral,
		}}
		setup.Members[1].Faction = "vendors"
	}
	enc, err := encounter.NewEncounter(setup)
	require.NoError(t, err)

	return builtScene{enc: enc, host: host, striker: striker}
}

// aroundTheTable ends alice's turn, which drives the lieutenant's.
func aroundTheTable(t *testing.T, enc *encounter.Encounter) {
	t.Helper()
	on, err := enc.ClockOf(&encounter.ClockOfInput{Member: alice})
	require.NoError(t, err)
	require.Equal(t, encounter.ClockTurn, on.Kind, "precondition: seeing each other started the fight")
	require.Equal(t, alice, on.Active, "precondition: it is alice's turn")
	_, err = enc.EndTurn(&encounter.EndTurnInput{Member: on.Active})
	require.NoError(t, err)
}

func storyOf(t *testing.T, enc *encounter.Encounter) []map[string]any {
	t.Helper()
	story, err := enc.Story(&encounter.StoryInput{Audience: alice})
	require.NoError(t, err)
	out := make([]map[string]any, 0, len(story))
	for _, entry := range story {
		var beat map[string]any
		require.NoError(t, json.Unmarshal(entry.Payload, &beat))
		out = append(out, beat)
	}

	return out
}

// labels reads the lieutenant's turn as a line: each of its picks (`answered`
// with its word, and the ability for an activate), each `activated` beat
// (by the ability's name), and each swing at the place it fell.
func labels(t *testing.T, enc *encounter.Encounter, swingsAt []int) []string {
	t.Helper()
	var out []string
	for i, beat := range storyOf(t, enc) {
		for _, at := range swingsAt {
			if at == i {
				out = append(out, "strike")
			}
		}
		switch beat["beat"] {
		case encounter.BeatAnswered:
			if beat["creature"] != string(lieutenant) {
				continue
			}
			label := "answered:" + fmt.Sprint(beat["word"])
			if ab, ok := beat["ability"]; ok {
				label += ":" + fmt.Sprint(ab)
			}
			out = append(out, label)
		case encounter.BeatActivated:
			ability := beat["ability"].(map[string]any)
			out = append(out, "activated:"+fmt.Sprint(beat["actor"])+":"+fmt.Sprint(ability["name"]))
		}
	}

	return out
}

func beatsOf(t *testing.T, enc *encounter.Encounter, kind string) []map[string]any {
	t.Helper()
	var out []map[string]any
	for _, beat := range storyOf(t, enc) {
		if beat["beat"] == kind {
			out = append(out, beat)
		}
	}

	return out
}

func picksOf(t *testing.T, enc *encounter.Encounter) []map[string]any {
	t.Helper()
	var out []map[string]any
	for _, beat := range beatsOf(t, enc, encounter.BeatAnswered) {
		if beat["creature"] == string(lieutenant) {
			out = append(out, beat)
		}
	}

	return out
}

// activateIsACandidate says whether the first row (the activate row, entry 0)
// was among a pick's candidates.
func activateIsACandidate(pick map[string]any) bool {
	for _, c := range pick["candidates"].([]any) {
		if c.(map[string]any)["entry"] == 0.0 {
			return true
		}
	}

	return false
}

// Test 1: HE USES IT AND SWINGS IN ONE TURN, IN WHICHEVER ORDER THE ROLL LANDS.
//
// Once the swing is spent `enemy: reach` is off (it needs an attack to be
// true: facts.go) and `enemy: walk` holds with a walk of nothing, so the
// walk row is a candidate beside the activate row on the pick after the
// swing, and the turn's last pick is that row, told by its own stayed beat
// ("already in reach"). The table is the decision record's, as written.
//
// So "the last candidate each time" would land on the walk row after the
// swing and never on the ability: the second order is scripted as the die
// lands on the LAST candidate for the first pick (the swing, the last of
// the two rows then standing) and on the FIRST for the next (the ability).
func TestALieutenantUsesAnAbilityAndSwingsInOneTurn(t *testing.T) {
	for _, tc := range []struct {
		name   string
		die    die
		expect []string
	}{
		{name: "the die lands on the first candidate each time", die: rollsLowest{}, expect: []string{
			"answered:activate:second wind",
			"activated:lieutenant:Second Wind",
			"answered:attack",
			"strike",
			"answered:toward",
		}},
		{name: "the die lands on the last candidate, then the first", die: newPlan("high", "low", "low"), expect: []string{
			"answered:attack",
			"strike",
			"answered:activate:second wind",
			"activated:lieutenant:Second Wind",
			"answered:toward",
		}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			sc := abilityScene{give: true, activator: true, roller: tc.die}.build(t)
			aroundTheTable(t, sc.enc)

			require.Equal(t, tc.expect, labels(t, sc.enc, sc.striker.at))
			left := sc.host.Left(lieutenant)
			require.Equal(t, 0, left.BonusActionsLeft, "the bonus action is spent")
			require.Equal(t, 0, left.ActionsLeft, "the action is spent by the swing")
			require.Equal(t, 0, sc.host.Uses(lieutenant, secondWind), "the one use is spent")
			require.Len(t, sc.striker.at, 1, "one swing")
			require.Len(t, sc.host.Activations, 1, "one activation")
			require.Len(t, picksOf(t, sc.enc), 3, "three picks, then nothing is left to pick and nothing is written")

			stayed := beatsOf(t, sc.enc, encounter.BeatStayed)
			require.Len(t, stayed, 1, "the walk row stood where he was, and nothing else was refused")
			require.Equal(t, "encounter:table:toward", stayed[0]["cause"])
			require.Equal(t, 1, len(beatsOf(t, sc.enc, encounter.BeatTurnEnded))-turnEndedBefore(t, sc.enc),
				"his turn ended")
			for _, pick := range picksOf(t, sc.enc) {
				if pick["word"] == "activate" {
					require.Equal(t, secondWind, pick["ability"])
				} else {
					require.NotContains(t, pick, "ability", "only a row that activates names an ability")
				}
			}
		})
	}
}

// turnEndedBefore is how many turn-ended beats precede his: alice's own.
func turnEndedBefore(t *testing.T, enc *encounter.Encounter) int {
	t.Helper()
	n := 0
	for _, beat := range beatsOf(t, enc, encounter.BeatTurnEnded) {
		if beat["member"] == string(alice) {
			n++
		}
	}

	return n
}

// Test 2: ONCE. His next turn has no activate row on its table, over twenty
// seeded dice. The first turn is his ability being used (asked of the host
// directly, so the seeds are only the second turn's).
func TestTheAbilityIsOffTheTableOnceItsUseIsSpent(t *testing.T) {
	for seed := int64(1); seed <= 20; seed++ {
		// The first turn is scripted so the ability is used on it; the seeded
		// die is the second turn's.
		dice := &switchDie{cur: rollsLowest{}}
		sc := abilityScene{give: true, activator: true, roller: dice}.build(t)
		aroundTheTable(t, sc.enc)
		require.Equal(t, 0, sc.host.Uses(lieutenant, secondWind))
		dice.cur = seededRolls{rand.New(rand.NewSource(seed))} //nolint:gosec // a test's seeded die
		activations := len(sc.host.Activations)
		activated := len(beatsOf(t, sc.enc, encounter.BeatActivated))
		picksBefore := len(picksOf(t, sc.enc))
		swings := len(sc.striker.at)

		// Alice ends hers; the lieutenant has a full turn again, and no use.
		aroundTheTable(t, sc.enc)

		require.Len(t, sc.host.Activations, activations, "seed %d: it was not asked again", seed)
		require.Len(t, beatsOf(t, sc.enc, encounter.BeatActivated), activated, "seed %d", seed)
		second := picksOf(t, sc.enc)[picksBefore:]
		require.NotEmpty(t, second, "seed %d", seed)
		for _, pick := range second {
			require.NotEqual(t, "activate", pick["word"], "seed %d", seed)
			require.False(t, activateIsACandidate(pick), "seed %d: the row is absent", seed)
		}
		require.GreaterOrEqual(t, len(sc.striker.at), swings, "seed %d", seed)
	}
}

// Test 3: ABSENT, EACH FOR ITS OWN REASON.
func TestTheActivateRowIsAbsentForEachReason(t *testing.T) {
	spent := &encounter.TurnFacts{ActionsLeft: 1, BonusActionsLeft: 0}
	for _, tc := range []struct {
		name  string
		scene abilityScene
	}{
		{name: "(a) no ability given", scene: abilityScene{activator: true}},
		{name: "(c) given, his bonus action already spent", scene: abilityScene{give: true, activator: true, perTurn: spent}},
		{name: "(d) given and payable, but no Activator wired", scene: abilityScene{give: true}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			sc := tc.scene.build(t)
			aroundTheTable(t, sc.enc)

			picks := picksOf(t, sc.enc)
			require.NotEmpty(t, picks)
			for _, pick := range picks {
				require.False(t, activateIsACandidate(pick), "the row is not a candidate")
				require.NotEqual(t, "activate", pick["word"])
			}
			require.Empty(t, sc.host.Activations)
			require.Empty(t, beatsOf(t, sc.enc, encounter.BeatActivated))
			for _, stayed := range beatsOf(t, sc.enc, encounter.BeatStayed) {
				require.NotEqual(t, "encounter:intent:activate", stayed["cause"], "not picked and refused: not offered")
			}
		})
	}

	t.Run("(b) given, the use already spent", func(t *testing.T) {
		sc := abilityScene{give: true, activator: true}.build(t)
		require.NoError(t, sc.host.Activate(context.Background(), sc.enc, lieutenant, secondWind))
		aroundTheTable(t, sc.enc)

		require.Equal(t, 0, sc.host.Uses(lieutenant, secondWind))
		for _, pick := range picksOf(t, sc.enc) {
			require.False(t, activateIsACandidate(pick))
		}
		require.Len(t, sc.host.Activations, 1, "only the setup's own ask")
	})

	t.Run("(e) given, payable and wired, on the world clock", func(t *testing.T) {
		sc := abilityScene{give: true, activator: true, world: true}.build(t)
		require.NoError(t, encounter.ExportedSpendWorldAction(sc.enc, alice))

		picks := picksOf(t, sc.enc)
		require.NotEmpty(t, picks, "the lieutenant had its round")
		for _, pick := range picks {
			require.False(t, activateIsACandidate(pick), "a round of the world pays for no ability")
		}
		require.Empty(t, sc.host.Activations)
		require.Empty(t, beatsOf(t, sc.enc, encounter.BeatActivated))
	})
}

// Test 4: A ROW MAY SPEAK AND ACTIVATE.
func TestARowMaySpeakAndActivate(t *testing.T) {
	table := encounter.Table{encounter.AnswerTime: {
		{Weight: 1, Say: "Hold the line", Activate: secondWind},
	}}
	sc := abilityScene{give: true, activator: true, table: table}.build(t)
	aroundTheTable(t, sc.enc)

	require.Len(t, sc.host.Activations, 1, "the ability was activated")
	require.Len(t, beatsOf(t, sc.enc, encounter.BeatActivated), 1)
	picks := picksOf(t, sc.enc)
	require.NotEmpty(t, picks)
	require.Equal(t, "activate", picks[0]["word"])
	require.Empty(t, picks[0]["say"], "the pick carries no words; the line is its own beat")
	said := false
	for _, beat := range storyOf(t, sc.enc) {
		raw, err := json.Marshal(beat)
		require.NoError(t, err)
		if beat["beat"] != encounter.BeatAnswered && strings.Contains(string(raw), "Hold the line") {
			said = true
		}
	}
	require.True(t, said, "and the line was said")
}

// Test 5: THE BACKSTOP TELLS WHY, for a driver that is not the table's.
func TestAHandRolledActivateIsRefusedAndToldWhy(t *testing.T) {
	spent := &encounter.TurnFacts{ActionsLeft: 1, BonusActionsLeft: 0}
	for _, tc := range []struct {
		name  string
		scene abilityScene
		why   string
		word  string
		bonus int
	}{
		{name: "a round of the world", scene: abilityScene{give: true, activator: true, world: true},
			why: "an ability is used on a turn, and this is a round of the world", word: secondWind, bonus: 1},
		{name: "nobody can run it", scene: abilityScene{give: true},
			why: "nobody can run an ability for it", word: secondWind, bonus: 1},
		{name: "no such ability", scene: abilityScene{give: true, activator: true},
			why: `it has no ability "rally"`, word: "rally", bonus: 1},
		{name: "it cannot pay", scene: abilityScene{give: true, activator: true, perTurn: spent},
			why: "it cannot pay for Second Wind now", word: secondWind, bonus: 0},
	} {
		t.Run(tc.name, func(t *testing.T) {
			driver := &scriptedDriver{intents: []encounter.TurnIntent{encounter.Activate{Ability: tc.word}}}
			scene := tc.scene
			scene.driver = driver
			sc := scene.build(t)
			if scene.world {
				require.NoError(t, encounter.ExportedSpendWorldAction(sc.enc, alice))
			} else {
				aroundTheTable(t, sc.enc)
			}

			stayed := beatsOf(t, sc.enc, encounter.BeatStayed)
			require.Len(t, stayed, 1)
			require.Equal(t, "encounter:intent:activate", stayed[0]["cause"])
			require.Equal(t, tc.why, stayed[0]["why"])
			require.Empty(t, sc.host.Activations, "the host was not asked")
			require.Equal(t, 1, sc.host.Uses(lieutenant, secondWind), "nothing was spent")
			require.Equal(t, tc.bonus, sc.host.Left(lieutenant).BonusActionsLeft, "nothing was spent")
			require.Len(t, driver.calls, 1, "the turn ended: the driver is not asked again")
			require.Empty(t, beatsOf(t, sc.enc, encounter.BeatActivated))
		})
	}
}

// Test 6: A MALFUNCTION ABORTS THE VERB.
func TestAnActivatorThatFailsAbortsTheVerb(t *testing.T) {
	boom := errors.New("the host broke")
	sc := abilityScene{give: true, activator: true}.build(t)
	sc.host.FailActivate = boom
	on, err := sc.enc.ClockOf(&encounter.ClockOfInput{Member: alice})
	require.NoError(t, err)

	_, err = sc.enc.EndTurn(&encounter.EndTurnInput{Member: on.Active})
	require.ErrorIs(t, err, boom)
	require.Contains(t, err.Error(), "activate: ")
}

// Test 7: A SHEET'S ABILITIES ARE JUDGED AT THE DOOR.
func TestASheetsAbilitiesAreJudgedAtTheDoor(t *testing.T) {
	for _, tc := range []struct {
		name      string
		abilities []encounter.AbilityFacts
	}{
		{name: "an empty word", abilities: []encounter.AbilityFacts{{Name: "Second Wind"}}},
		{name: "an empty name", abilities: []encounter.AbilityFacts{{Word: secondWind}}},
		{name: "the same word twice", abilities: []encounter.AbilityFacts{
			{Word: secondWind, Name: "Second Wind"}, {Word: secondWind, Name: "Again"},
		}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			sc := abilityScene{}.build(t)
			_ = sc
			enc, err := encounter.NewEncounter(&encounter.SetupInput{
				Field: encounter.FieldInput{
					Canvas: openAir(), Regions: []encounter.RegionInput{rectRegion("hall", 0, 0, 60, 8)},
				},
				Members: []encounter.MemberInput{
					{ID: alice, Kind: encounter.KindPlayer, Position: onRow(1)},
					{ID: lieutenant, Kind: encounter.KindMonster, Position: onRow(30)},
				},
				Endings: []encounter.EndingInput{{Key: "withdrawn", Trigger: encounter.TriggerExternal{}}},
				Capabilities: encounter.Capabilities{
					Sight: everyoneSeesTheWholeMap{}, Equipment: encounter.UnobservedEquipment{},
					Conditions: encounter.UnobservedEquipment{},
					Sheets: sheetFacts{
						alice:      {SpeedFeet: 30},
						lieutenant: {SpeedFeet: 30, Abilities: tc.abilities},
					},
					Participation: everyoneStanding{}, Initiative: orderAsGiven{}, Driver: tableDriver(),
					Roller: rollsLowest{},
					Actors: encounter.Actors{Striker: &markingStriker{}, Mover: quietMover{}, Announcer: quietAnnouncer{}},
				},
			})
			if err == nil {
				// The door is a sheet read: the first verb that asks refuses.
				_, err = enc.EndTurn(&encounter.EndTurnInput{Member: alice})
			}
			require.ErrorIs(t, err, encounter.ErrInvalidData)
			require.Contains(t, err.Error(), `"lieutenant"`, "it names the member")
		})
	}
}

// Test 8: A HOST THAT SAYS NOTHING IS AS IT WAS. A creature with the activate
// row whose host lists no ability acts as it did before the row could be
// chosen: the same seed gives the same turn as a table without the row's
// words changing the candidates.
func TestAHostThatListsNoAbilityActsAsItDid(t *testing.T) {
	withoutTheRow := encounter.Table{encounter.AnswerTime: lieutenantTable()[encounter.AnswerTime][1:]}
	// The activate row is never a candidate here, so the picks and the turn
	// match a table with only the other two rows, once the entry numbers
	// shift by the row that is gone.
	for _, name := range []string{"low", "high"} {
		with := abilityScene{activator: true, roller: newPlan(name, name, name)}.build(t)
		aroundTheTable(t, with.enc)
		without := abilityScene{activator: true, roller: newPlan(name, name, name), table: withoutTheRow}.build(t)
		aroundTheTable(t, without.enc)

		require.Equal(t, labels(t, without.enc, without.striker.at), labels(t, with.enc, with.striker.at))
		require.Equal(t, without.striker.at, with.striker.at)
		require.Empty(t, with.host.Activations)
	}
}

// Test 9: THE BOUND HOLDS. A host that never charges leaves the ability payable
// for ever; the creature is asked at most one more time than the turn's
// ordinary bound allows for it and the turn ends.
func TestAnAbilityTheHostNeverChargesEndsTheTurnAtTheBound(t *testing.T) {
	table := encounter.Table{encounter.AnswerTime: {{Weight: 1, Activate: secondWind}}}
	sc := abilityScene{give: true, activator: true, free: true, table: table, ltAt: 30}.build(t)
	aroundTheTable(t, sc.enc)

	// 2 + twice six cells of speed, plus one ability: 15 asks, and every ask
	// uses the ability, since it never stops being payable.
	require.Len(t, sc.host.Activations, 15, "it is asked the bound and no more")
	require.Len(t, beatsOf(t, sc.enc, encounter.BeatActivated), 15)
	require.NotEmpty(t, beatsOf(t, sc.enc, encounter.BeatTurnEnded), "and the turn ended")
}

var _ = spatial.Position{}
