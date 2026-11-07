// Copyright (C) 2026 Kirk Diggler
// SPDX-License-Identifier: GPL-3.0-or-later

package encounter

// These clock controls use explicit canonical configurations, not an old-table
// importer or a swapped Program cache. The fixture helper supplies ordinary
// InstanceSpec data; all scheduling, progress and selection are production code.

import (
	"context"
	"encoding/json"
	"strings"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/KirkDiggler/rpg-toolkit/core"
	"github.com/KirkDiggler/rpg-toolkit/mind/behavior"
	"github.com/KirkDiggler/rpg-toolkit/mind/machine"
	"github.com/KirkDiggler/rpg-toolkit/play/clock"
	"github.com/KirkDiggler/rpg-toolkit/tools/spatial"
)

type wakeBlind struct{}

func (wakeBlind) Sight(members []MemberID) (map[MemberID]int, error) {
	out := make(map[MemberID]int, len(members))
	for _, id := range members {
		out[id] = 0
	}
	return out, nil
}

type wakeRollsOne struct{}

func (wakeRollsOne) Roll(context.Context, int) (int, error) { return 1, nil }
func (wakeRollsOne) RollN(_ context.Context, count, _ int) ([]int, error) {
	out := make([]int, count)
	for i := range out {
		out[i] = 1
	}
	return out, nil
}

type askedDriver struct {
	inner Driver
	asked []MemberID
}

func (d *askedDriver) Act(view MonsterView) (Decision, error) {
	d.asked = append(d.asked, view.Self)
	return d.inner.Act(view)
}

func holds() Table { return Table{AnswerTime: {{Weight: 1, Hold: true}}} }

func wakeHall(t *testing.T, members ...MemberInput) (*Encounter, *askedDriver) {
	t.Helper()
	driver := &askedDriver{inner: BehaviorDriver{}}
	enc, err := NewEncounter(wakeSetup(driver, members...))
	require.NoError(t, err)
	return enc, driver
}

func wakeSetup(driver Driver, members ...MemberInput) *SetupInput {
	return &SetupInput{
		Field: FieldInput{Canvas: CanvasInput{Void: VoidIsOpaque(), Orientation: HexesArePointyTop()},
			Regions: []RegionInput{rectRegion("hall", 0, 0, 20, 10)}},
		Members: append([]MemberInput{{ID: "alice", Kind: KindPlayer, Position: spatial.Position{X: 1, Y: 1}}}, members...),
		Endings: []EndingInput{{Key: "called", Trigger: TriggerExternal{}}}, Capabilities: wakeCapabilities(driver),
	}
}

func wakeCapabilities(driver Driver) Capabilities {
	return Capabilities{Sight: wakeBlind{}, Equipment: UnobservedEquipment{}, Conditions: UnobservedEquipment{},
		Sheets: zeroSheets{}, Participation: everyoneStanding{}, Initiative: orderAsGiven{}, Driver: driver, Roller: wakeRollsOne{},
		Actors: Actors{Striker: passStriker{}, Mover: quietMover{}, Announcer: quietAnnouncer{}}}
}

// fixtureBehavior is plain materialized fixture data, not a binder or runner.
func fixtureBehavior(recipe machine.Recipe, table Table) *BehaviorInput {
	return &BehaviorInput{Instances: []BoundBehavior{{Instance: behavior.InstanceSpec{ID: "policy", Recipe: recipe, Table: table}}}}
}

func ordinaryBehavior(table Table) *BehaviorInput {
	if len(table) == 0 {
		return nil
	}
	return fixtureBehavior(machine.Recipe{Ref: core.Ref{Module: "test", Type: "behaviors", ID: "ordinary"}, Initial: "watch",
		States: map[string]machine.State{"watch": {Transitions: []machine.Transition{
			{On: beatTime, Do: []machine.Act{{Pick: string(AnswerTime)}}},
			{On: beatTurn, Do: []machine.Act{{Pick: string(AnswerTime)}}},
		}}}}, table)
}

func monster(id MemberID, col, row float64, table Table) MemberInput {
	return MemberInput{ID: id, Kind: KindMonster, Position: spatial.Position{X: col, Y: row}, Behaviors: ordinaryBehavior(table)}
}

func beatsTagged(t *testing.T, enc *Encounter, kind string) []map[string]any {
	t.Helper()
	story, err := enc.Story(&StoryInput{Audience: "alice"})
	require.NoError(t, err)
	var out []map[string]any
	for _, entry := range story {
		var beat map[string]any
		require.NoError(t, json.Unmarshal(entry.Payload, &beat))
		if beat["beat"] == kind {
			out = append(out, beat)
		}
	}
	return out
}

func pickers(t *testing.T, enc *Encounter) []MemberID {
	t.Helper()
	var out []MemberID
	for _, beat := range beatsTagged(t, enc, BeatAnswered) {
		key, _ := beat["key"].(string)
		if strings.HasSuffix(key, "/time") {
			out = append(out, MemberID(beat["creature"].(string)))
		}
	}
	return out
}

var pickTime = machine.Transition{On: beatTime, Do: []machine.Act{{Pick: string(AnswerTime)}}}

func TestWake_VisitsOnlyDue(t *testing.T) {
	asleep := listener("m-asleep", 16, 2, recipe(t, "asleep", map[string]machine.State{"asleep": {}}))
	enc, driver := wakeHall(t, monster("m-due", 8, 2, holds()), monster("m-untabled", 12, 2, nil), asleep)
	require.Equal(t, 1, enc.due.Len())
	require.NoError(t, enc.spendWorldAction("alice"))
	require.Equal(t, []MemberID{"m-due"}, driver.asked)
	require.Equal(t, []MemberID{"m-due"}, pickers(t, enc))
	require.Equal(t, uint64(2), enc.dueAt["m-due"])
}

func TestWake_CatchUpPopsTwice(t *testing.T) {
	enc, _ := wakeHall(t, monster("m1", 6, 2, holds()), monster("m2", 10, 2, holds()), monster("m3", 14, 2, holds()))
	enc.worldThinking = true
	require.NoError(t, enc.spendWorldAction("alice"))
	enc.worldThinking = false
	require.Empty(t, pickers(t, enc))
	require.NoError(t, enc.spendWorldAction("alice"))
	require.Equal(t, []MemberID{"m1", "m2", "m3", "m1", "m2", "m3"}, pickers(t, enc))
	for _, id := range []MemberID{"m1", "m2", "m3"} {
		budget, err := enc.clock.Budget(&clock.BudgetInput{ID: core.EntityID(id)})
		require.NoError(t, err)
		require.Zero(t, budget)
		require.Equal(t, uint64(3), enc.dueAt[id])
	}
}

func TestWake_StateBeatAndRelisten(t *testing.T) {
	guard := listener("m-guard", 8, 2, recipe(t, "watch", map[string]machine.State{
		"watch": {Transitions: []machine.Transition{{On: beatTime, To: "alert", Reason: "the hour turned", Do: []machine.Act{{Pick: string(AnswerTime)}}}}},
		"alert": {Transitions: []machine.Transition{pickTime, {On: beatHeard, To: "watch"}}},
	}))
	enc, _ := wakeHall(t, guard)
	require.Empty(t, enc.listeners.For(beatHeard))
	require.NoError(t, enc.spendWorldAction("alice"))
	states := beatsTagged(t, enc, BeatState)
	require.Len(t, states, 1)
	require.Equal(t, map[string]any{"beat": BeatState, "member": "m-guard", "from": "watch", "to": "alert", "reason": "the hour turned"}, states[0])
	require.Equal(t, []machine.Listener[MemberID]{{Key: "m-guard"}}, enc.listeners.For(beatHeard))
	loaded, err := LoadEncounter(&LoadEncounterInput{Data: enc.ToData(), Capabilities: wakeCapabilities(BehaviorDriver{})})
	require.NoError(t, err)
	require.Equal(t, "alert", loaded.members["m-guard"].bodyState())
	require.Equal(t, enc.listeners.For(beatHeard), loaded.listeners.For(beatHeard))
	require.Equal(t, uint64(2), loaded.dueAt["m-guard"])
}

func TestLoad_WakeIsHonouredAndDwellStays(t *testing.T) {
	enc, _ := wakeHall(t, monster("m-sleeper", 8, 2, holds()), monster("m-patrol", 12, 2, holds()))
	require.NoError(t, enc.spendWorldAction("alice"))
	data := enc.ToData()
	now, wake := uint64(data.Clock.HighWater), uint64(data.Clock.HighWater)+3
	var dwell *DwellData
	for i := range data.Members {
		switch data.Members[i].ID {
		case "m-sleeper":
			data.Members[i].Behaviors.Progress.Instances[0].Blob.WakeAt = &wake
		case "m-patrol":
			dwell = &DwellData{Cell: *data.Members[i].Cell, Rounds: 2}
			data.Members[i].Dwell = dwell
		}
	}
	driver := &askedDriver{inner: BehaviorDriver{}}
	loaded, err := LoadEncounter(&LoadEncounterInput{Data: data, Capabilities: wakeCapabilities(driver)})
	require.NoError(t, err)
	require.Equal(t, wake, loaded.dueAt["m-sleeper"])
	require.Equal(t, now+1, loaded.dueAt["m-patrol"])
	require.Nil(t, loaded.members["m-patrol"].behaviors.composer.ToData().Instances[0].Blob.WakeAt)
	for _, m := range loaded.ToData().Members {
		if m.ID == "m-patrol" {
			require.Equal(t, dwell, m.Dwell)
		}
	}
	for range 2 {
		require.NoError(t, loaded.spendWorldAction("alice"))
	}
	require.Equal(t, []MemberID{"m-patrol", "m-patrol"}, driver.asked)
	require.NoError(t, loaded.spendWorldAction("alice"))
	require.Contains(t, driver.asked, MemberID("m-sleeper"))
	require.Nil(t, loaded.members["m-sleeper"].behaviors.composer.ToData().Instances[0].Blob.WakeAt)
}

func TestTransfer_DisarmsAndRearms(t *testing.T) {
	enc, _ := wakeHall(t, monster("m-brute", 4, 2, holds()), monster("m-guard", 12, 2, holds()))
	_, err := enc.form(&FormInput{Order: []MemberID{"alice", "m-brute"}})
	require.NoError(t, err)
	_, armed := enc.dueAt["m-brute"]
	require.False(t, armed)
	_, err = enc.Transfer(&TransferInput{Member: "m-guard", To: ClockTurn, Pos: 2})
	require.NoError(t, err)
	_, armed = enc.dueAt["m-guard"]
	require.False(t, armed)
	_, err = enc.Transfer(&TransferInput{Member: "m-guard", To: ClockWorld})
	require.NoError(t, err)
	require.Equal(t, uint64(enc.clock.ToData().HighWater)+1, enc.dueAt["m-guard"])
}

func TestNoAuthoredBehaviorInventsNoDefaultMachine(t *testing.T) {
	enc, driver := wakeHall(t, MemberInput{ID: "m-watch", Kind: KindMonster, Position: spatial.Position{X: 8}})
	for range 3 {
		require.NoError(t, enc.spendWorldAction("alice"))
	}
	require.Empty(t, driver.asked)
	require.Empty(t, beatsTagged(t, enc, BeatState))
	require.Nil(t, enc.members["m-watch"].behaviors.toData())
}

func TestLoad_ClockNamesTheSeat(t *testing.T) {
	enc, _ := wakeHall(t, monster("m-guard", 8, 2, holds()))
	data := enc.ToData()
	for i := range data.Members {
		data.Members[i].Clock = worldClockName
	}
	loaded, err := LoadEncounter(&LoadEncounterInput{Data: data, Capabilities: wakeCapabilities(passDriver{})})
	require.NoError(t, err)
	require.Equal(t, worldClockName, loaded.members["m-guard"].Clock)
	for i := range data.Members {
		data.Members[i].Clock = "tomb"
	}
	_, err = LoadEncounter(&LoadEncounterInput{Data: data, Capabilities: wakeCapabilities(passDriver{})})
	require.ErrorIs(t, err, ErrInvalidData)
}

func TestLoad_MachineRefusals(t *testing.T) {
	enc, _ := wakeHall(t, monster("m-guard", 8, 2, holds()))
	for _, name := range []string{"unknown state", "unknown bag", "retired machine"} {
		t.Run(name, func(t *testing.T) {
			data := enc.ToData()
			for i := range data.Members {
				m := &data.Members[i]
				if m.ID != "m-guard" {
					continue
				}
				switch name {
				case "unknown state":
					m.Behaviors.Progress.Instances[0].Blob.State = "missing"
				case "unknown bag":
					m.Behaviors.Progress.Instances[0].Blob.Bag = machine.Bag{"unknown": true}
				case "retired machine":
					m.Machine = &MachineData{State: "watch"}
				}
			}
			_, err := LoadEncounter(&LoadEncounterInput{Data: data, Capabilities: wakeCapabilities(BehaviorDriver{})})
			require.ErrorIs(t, err, ErrInvalidData)
		})
	}
}

func TestWake_CatchUpSurvivesReload(t *testing.T) {
	enc, _ := wakeHall(t, monster("m1", 6, 2, holds()))
	enc.worldThinking = true
	require.NoError(t, enc.spendWorldAction("alice"))
	enc.worldThinking = false
	driver := &askedDriver{inner: BehaviorDriver{}}
	loaded, err := LoadEncounter(&LoadEncounterInput{Data: enc.ToData(), Capabilities: wakeCapabilities(driver)})
	require.NoError(t, err)
	require.Equal(t, uint64(1), loaded.dueAt["m1"])
	require.NoError(t, loaded.spendWorldAction("alice"))
	require.Equal(t, []MemberID{"m1", "m1"}, driver.asked)
	budget, err := loaded.clock.Budget(&clock.BudgetInput{ID: "m1"})
	require.NoError(t, err)
	require.Zero(t, budget)
}

func TestWake_CatchUpAsksAtEachRound(t *testing.T) {
	states := map[string]machine.State{"done": {}}
	names := []string{"zero", "one", "two", "three", "done"}
	for i, name := range names[:4] {
		var guard *machine.Guard
		if i > 0 {
			guard = &machine.Guard{Rounds: &machine.RoundsLeaf{Op: machine.OpGte, N: 1}}
		}
		states[name] = machine.State{Transitions: []machine.Transition{{On: beatTime, When: guard,
			To: names[i+1], Do: []machine.Act{{Anchor: true}, {Pick: string(AnswerTime)}}}}}
	}
	enc, driver := wakeHall(t, listener("m1", 6, 2, recipe(t, "zero", states)))
	for range 2 {
		require.NoError(t, enc.spendWorldAction("alice"))
	}
	enc.worldThinking = true
	require.NoError(t, enc.spendWorldAction("alice"))
	enc.worldThinking = false
	require.NoError(t, enc.spendWorldAction("alice"))
	require.Equal(t, "done", enc.members["m1"].bodyState(), "each anchored step needs a distinct round; twice at four cannot pass")
	require.Equal(t, []MemberID{"m1", "m1", "m1", "m1"}, driver.asked)
	require.Equal(t, uint64(4), *enc.members["m1"].behaviors.composer.ToData().Instances[0].Blob.Anchor)
}

type raisingDriver struct {
	inner  Driver
	enc    *Encounter
	member MemberID
	done   bool
	asked  []MemberID
}

func (d *raisingDriver) Act(view MonsterView) (Decision, error) {
	d.asked = append(d.asked, view.Self)
	if view.Self == d.member && !d.done {
		d.done = true
		if _, err := d.enc.advanceWorld("alice", 1); err != nil {
			return Decision{}, err
		}
		if err := d.enc.wake(); err != nil {
			return Decision{}, err
		}
	}
	return d.inner.Act(view)
}

func TestWake_ARoundRaisedInsideThePassIsTakenInIt(t *testing.T) {
	driver := &raisingDriver{inner: BehaviorDriver{}, member: "m1"}
	enc, err := NewEncounter(wakeSetup(driver, monster("m1", 6, 2, holds()), monster("m2", 10, 2, holds())))
	require.NoError(t, err)
	driver.enc = enc
	require.NoError(t, enc.spendWorldAction("alice"))
	require.Equal(t, []MemberID{"m1", "m2", "m1", "m2"}, driver.asked)
	for _, id := range []core.EntityID{"m1", "m2"} {
		budget, err := enc.clock.Budget(&clock.BudgetInput{ID: id})
		require.NoError(t, err)
		require.Zero(t, budget)
	}
}
