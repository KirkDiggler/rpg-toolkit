// Copyright (C) 2026 Kirk Diggler
// SPDX-License-Identifier: GPL-3.0-or-later

package encounter

// machine_internal_test.go pins the raise that wakes who is due
// (rpg-project#766 S3, machine.go): the due heap, its catch-up, the state
// beat, the dwell read back as a wake, and a fight taking a creature off the
// heap and giving it back. Internal, because the heap, the programs and the
// raise itself are the composition's own.

import (
	"context"
	"encoding/json"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/KirkDiggler/rpg-toolkit/core"
	"github.com/KirkDiggler/rpg-toolkit/mind/machine"
	"github.com/KirkDiggler/rpg-toolkit/play/clock"
	"github.com/KirkDiggler/rpg-toolkit/tools/spatial"
)

// wakeBlind is a sight in which nobody sees anybody, so no fight forms and
// every creature stays on the world clock.
type wakeBlind struct{}

func (wakeBlind) Sight(members []MemberID) (map[MemberID]int, error) {
	out := make(map[MemberID]int, len(members))
	for _, id := range members {
		out[id] = 0
	}

	return out, nil
}

// wakeRollsOne is a die that always lands 1.
type wakeRollsOne struct{}

func (wakeRollsOne) Roll(context.Context, int) (int, error) { return 1, nil }

func (wakeRollsOne) RollN(_ context.Context, count, _ int) ([]int, error) {
	out := make([]int, count)
	for i := range out {
		out[i] = 1
	}

	return out, nil
}

// askedDriver is the table driver, counting who it is asked for, in order:
// the hook that says a member was visited (a view built and a table rolled).
type askedDriver struct {
	inner TableDriver
	asked []MemberID
}

func (d *askedDriver) Act(view MonsterView) (Decision, error) {
	d.asked = append(d.asked, view.Self)

	return d.inner.Act(view)
}

// holds is a `time` table that always holds.
func holds() Table {
	return Table{AnswerTime: {{Weight: 1, Hold: true}}}
}

// wakeHall opens a 20×10 room nobody sees across, with alice at (1,1) and
// every other member placed as given, on the table driver.
func wakeHall(t *testing.T, members ...MemberInput) (*Encounter, *askedDriver) {
	t.Helper()
	driver := &askedDriver{inner: TableDriver{Roller: wakeRollsOne{}}}
	enc, err := NewEncounter(wakeSetup(driver, members...))
	require.NoError(t, err)

	return enc, driver
}

func wakeSetup(driver Driver, members ...MemberInput) *SetupInput {
	return &SetupInput{
		Field: FieldInput{
			Canvas:  CanvasInput{Void: VoidIsOpaque(), Orientation: HexesArePointyTop()},
			Regions: []RegionInput{rectRegion("hall", 0, 0, 20, 10)},
		},
		Members: append([]MemberInput{{ID: "alice", Kind: KindPlayer, Position: spatial.Position{X: 1, Y: 1}}}, members...),
		Endings: []EndingInput{{Key: "called", Trigger: TriggerExternal{}}},
		Capabilities: wakeCapabilities(driver),
	}
}

func wakeCapabilities(driver Driver) Capabilities {
	return Capabilities{
		Sight:         wakeBlind{},
		Equipment:     UnobservedEquipment{},
		Conditions:    UnobservedEquipment{},
		Sheets:        zeroSheets{},
		Participation: everyoneStanding{},
		Initiative:    orderAsGiven{},
		Driver:        driver,
		Roller:        wakeRollsOne{},
		Actors: Actors{
			Striker:   passStriker{},
			Mover:     quietMover{},
			Announcer: quietAnnouncer{},
		},
	}
}

// monster is a monster at (col,row) with this table (nil for none).
func monster(id MemberID, col, row float64, table Table) MemberInput {
	return MemberInput{ID: id, Kind: KindMonster, Position: spatial.Position{X: col, Y: row}, Table: table}
}

// beatsTagged is every beat of this kind in record order, decoded.
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

// pickers is the creature of every pick beat, in record order.
func pickers(t *testing.T, enc *Encounter) []MemberID {
	t.Helper()
	var out []MemberID
	for _, beat := range beatsTagged(t, enc, BeatAnswered) {
		if beat["key"] == string(AnswerTime) {
			out = append(out, MemberID(beat["creature"].(string)))
		}
	}

	return out
}

// testProgram compiles a recipe against the encounter's vocabulary plus a
// `noise` beat, the stand-in for S4's scoped beats.
func testProgram(t *testing.T, states map[string]machine.State, initial string) *machine.Program {
	t.Helper()
	ref, err := core.ParseString(defaultRecipeRef)
	require.NoError(t, err)
	vocab := machineVocabulary()
	vocab.Beats["noise"] = machine.BeatSpec{}
	p, err := machine.Recipe{Ref: *ref, Initial: initial, States: states}.Compile(vocab)
	require.NoError(t, err)

	return p
}

// pickTime is the default recipe's one transition: on time, roll time.
var pickTime = machine.Transition{On: beatTime, Do: []machine.Act{{Pick: string(AnswerTime)}}}

// TestWake_VisitsOnlyDue is W1: a raise visits only who the due heap pops.
// Three creatures: one with a table on the default recipe, one with no table
// (no machine, never armed), and one with a table whose machine stands in a
// state with no `time` transition. The raise asks the first alone: one view
// built, one pick beat. The census asked every tabled creature it walked.
func TestWake_VisitsOnlyDue(t *testing.T) {
	enc, driver := wakeHall(t,
		monster("m-due", 8, 2, holds()),
		monster("m-untabled", 12, 2, nil),
		monster("m-asleep", 16, 2, holds()),
	)
	enc.programs[defaultRecipeRef] = testProgram(t, map[string]machine.State{
		defaultInitial: {Transitions: []machine.Transition{pickTime}},
		"asleep":       {},
	}, defaultInitial)
	enc.members["m-asleep"].Machine.State = "asleep"
	require.NoError(t, enc.rebuildWake())
	require.Equal(t, 1, enc.due.Len(), "only the creature whose state wants time is on the heap")

	require.NoError(t, enc.spendWorldAction("alice"))

	require.Equal(t, []MemberID{"m-due"}, driver.asked, "one view built, for the creature that was due")
	require.Equal(t, []MemberID{"m-due"}, pickers(t, enc), "one pick beat")
	require.Equal(t, 1, enc.due.Len(), "and it is due again at the next round")
	require.Equal(t, uint64(2), enc.dueAt["m-due"])
}

// TestWake_CatchUpPopsTwice is W2 and the census's catch-up: a raise that
// the wake could not take (it came inside another pass) leaves three
// creatures a round behind; the next raise steps each at its own round and
// again at the next, round by round in seat order.
func TestWake_CatchUpPopsTwice(t *testing.T) {
	enc, _ := wakeHall(t,
		monster("m1", 6, 2, holds()),
		monster("m2", 10, 2, holds()),
		monster("m3", 14, 2, holds()),
	)

	enc.worldThinking = true // a raise inside another pass
	require.NoError(t, enc.spendWorldAction("alice"))
	enc.worldThinking = false
	require.Empty(t, pickers(t, enc), "the skipped raise woke nobody")

	require.NoError(t, enc.spendWorldAction("alice"))
	require.Equal(t, []MemberID{"m1", "m2", "m3", "m1", "m2", "m3"}, pickers(t, enc),
		"each acts twice, round by round, in seat order")
	for _, id := range []MemberID{"m1", "m2", "m3"} {
		budget, err := enc.clock.Budget(&clock.BudgetInput{ID: core.EntityID(id)})
		require.NoError(t, err)
		require.Zero(t, budget, "%s spent both rounds the clock granted", id)
		require.Equal(t, uint64(3), enc.dueAt[id], "%s is due at the next round", id)
	}
}

// TestWake_StateBeatAndRelisten: a transition that names a state lands a
// `state` beat (member, from, to, reason), re-lists what the member listens
// for, and persists in the blob, from which the index is rebuilt. (A load
// refuses a state the shipped recipe lacks: TestLoad_MachineRefusals.)
func TestWake_StateBeatAndRelisten(t *testing.T) {
	enc, _ := wakeHall(t, monster("m-guard", 8, 2, holds()))
	program := testProgram(t, map[string]machine.State{
		defaultInitial: {Transitions: []machine.Transition{
			{On: beatTime, To: "alert", Reason: "the hour turned", Do: []machine.Act{{Pick: string(AnswerTime)}}},
		}},
		"alert": {Transitions: []machine.Transition{pickTime, {On: "noise", To: defaultInitial}}},
	}, defaultInitial)
	enc.programs[defaultRecipeRef] = program
	require.NoError(t, enc.rebuildWake())
	require.Empty(t, enc.listeners.For("noise"), "watch listens for nothing but time")

	require.NoError(t, enc.spendWorldAction("alice"))

	states := beatsTagged(t, enc, BeatState)
	require.Len(t, states, 1)
	require.Equal(t, map[string]any{
		"beat": BeatState, "member": "m-guard", "from": defaultInitial, "to": "alert", "reason": "the hour turned",
	}, states[0])
	require.Equal(t, []machine.Listener[MemberID]{{Key: "m-guard"}}, enc.listeners.For("noise"), "alert listens for noise")
	require.Equal(t, uint64(2), enc.dueAt["m-guard"], "alert still wants time")

	data := enc.ToData()
	var stored *MachineData
	for _, m := range data.Members {
		if m.ID == "m-guard" {
			stored = m.Machine
		}
	}
	require.Equal(t, &MachineData{State: "alert"}, stored, "the blob persists its state")

	// The indexes are the blobs' answers, never stored: thrown away and
	// rebuilt (what a load does), the guard still listens for noise.
	enc.listeners = machine.Listeners[MemberID]{}
	enc.due = machine.Due[MemberID]{}
	require.NoError(t, enc.rebuildWake())
	require.Equal(t, []machine.Listener[MemberID]{{Key: "m-guard"}}, enc.listeners.For("noise"),
		"the index is rebuilt from the blob")
	require.Equal(t, uint64(2), enc.dueAt["m-guard"])
}

// TestLoad_WakeIsHonouredAndDwellStays: a stored wake (an authored wake_in
// act's, which a later build writes) is rebuilt into the heap at load: the
// creature is not due until it, is asked at it, and the wake is spent. A
// stored dwell is NOT a wake (rpg-project#773's ruling): it loads as the
// dwell it was, written back as itself, and sets no wake, so a dwelling
// creature stays due every round.
func TestLoad_WakeIsHonouredAndDwellStays(t *testing.T) {
	enc, _ := wakeHall(t, monster("m-sleeper", 8, 2, holds()), monster("m-patrol", 12, 2, holds()))
	require.NoError(t, enc.spendWorldAction("alice"))
	data := enc.ToData()
	now := uint64(data.Clock.HighWater)
	wake := now + 3
	var dwell *DwellData
	for i := range data.Members {
		switch data.Members[i].ID {
		case "m-sleeper":
			data.Members[i].Machine = &MachineData{WakeAt: &wake}
		case "m-patrol":
			dwell = &DwellData{Cell: *data.Members[i].Cell, Rounds: 2}
			data.Members[i].Dwell = dwell
		}
	}

	driver := &askedDriver{inner: TableDriver{Roller: wakeRollsOne{}}}
	loaded, err := LoadEncounter(&LoadEncounterInput{Data: data, Capabilities: wakeCapabilities(driver)})
	require.NoError(t, err)
	require.Equal(t, wake, loaded.dueAt["m-sleeper"], "the wake is the round it is due")
	require.Equal(t, now+1, loaded.dueAt["m-patrol"], "a dwell is no wake: due at the next round")
	require.Nil(t, loaded.members["m-patrol"].Machine.WakeAt)
	for _, m := range loaded.ToData().Members {
		if m.ID == "m-patrol" {
			require.Equal(t, dwell, m.Dwell, "the dwell is written back as itself")
		}
	}

	for range 2 {
		require.NoError(t, loaded.spendWorldAction("alice"))
	}
	require.NotContains(t, driver.asked, MemberID("m-sleeper"), "it sleeps to its wake unasked")
	require.Equal(t, []MemberID{"m-patrol", "m-patrol"}, driver.asked, "the dwelling patrol is asked every round")
	require.NoError(t, loaded.spendWorldAction("alice"))
	require.Contains(t, driver.asked, MemberID("m-sleeper"), "and is asked at its wake")
	require.Nil(t, loaded.members["m-sleeper"].Machine.WakeAt, "the wake is spent")
	require.Equal(t, uint64(loaded.clock.ToData().HighWater)+1, loaded.dueAt["m-sleeper"])
}

// TestTransfer_DisarmsAndRearms: a creature that joins a fight leaves the
// due heap (the fight prices its time), and one that comes back — by
// transfer or by the fight dissolving — is due at the world's next round.
func TestTransfer_DisarmsAndRearms(t *testing.T) {
	enc, _ := wakeHall(t, monster("m-brute", 4, 2, holds()), monster("m-guard", 12, 2, holds()))
	now := uint64(enc.clock.ToData().HighWater)
	require.Equal(t, now+1, enc.dueAt["m-guard"])

	_, err := enc.form(&FormInput{Order: []MemberID{"alice", "m-brute"}})
	require.NoError(t, err)
	_, armed := enc.dueAt["m-brute"]
	require.False(t, armed, "a fight takes its members off the heap")

	_, err = enc.Transfer(&TransferInput{Member: "m-guard", To: ClockTurn, Pos: 2})
	require.NoError(t, err)
	_, armed = enc.dueAt["m-guard"]
	require.False(t, armed, "a straggler transferred in leaves it too")
	require.Zero(t, enc.due.Len())

	_, err = enc.Transfer(&TransferInput{Member: "m-guard", To: ClockWorld})
	require.NoError(t, err)
	require.Equal(t, uint64(enc.clock.ToData().HighWater)+1, enc.dueAt["m-guard"], "transferred back: due at the next round")

	_, err = enc.Dissolve(&DissolveInput{Member: "alice"})
	require.NoError(t, err)
	require.Equal(t, uint64(enc.clock.ToData().HighWater)+1, enc.dueAt["m-brute"], "the dissolve re-arms the fight's creatures")
	require.Equal(t, 2, enc.due.Len())
}

// TestDefaultRecipe_IsTodaysTurn is R4 at the program: the default recipe's
// one state is due every round, listens for nothing else, and on time rolls
// the time table and stays where it is — so a creature with a table is asked
// once per raise, as the census asked it. The castle harness is this test's
// golden (TestMadKingKeepFacesAlongItsPatrol and
// TestMadKingTheAlarmCarriesToTheKeenEar, unchanged).
func TestDefaultRecipe_IsTodaysTurn(t *testing.T) {
	programs, err := compilePrograms()
	require.NoError(t, err)
	p := programs[defaultRecipeRef]
	require.NotNil(t, p)

	blob := machine.Blob{}
	at, due := p.DueAt(&blob, 7)
	require.True(t, due)
	require.Equal(t, uint64(8), at, "due at the next round")
	require.Nil(t, p.Listens(&blob), "it listens for nothing the heap does not answer")
	stepped, err := p.Step(&blob, beatTime, tickEv{Round: 8}, nil, 8)
	require.NoError(t, err)
	require.Equal(t, machine.Stepped{Fired: true, Acts: []machine.Act{{Pick: string(AnswerTime)}}}, stepped,
		"it rolls the time table and enters no state")
	require.Nil(t, machineDataFrom(p, blob), "a creature on its initial state writes no machine")

	enc, driver := wakeHall(t, monster("m-watch", 8, 2, holds()))
	for range 3 {
		require.NoError(t, enc.spendWorldAction("alice"))
	}
	require.Equal(t, []MemberID{"m-watch", "m-watch", "m-watch"}, driver.asked, "asked once per raise")
	require.Empty(t, beatsTagged(t, enc, BeatState), "the default recipe never changes state")
}

// TestLoad_ClockNamesTheSeat is W5 on the member: the world clock is the
// absent name, written as nothing, and a clock this build does not know is
// refused at load.
func TestLoad_ClockNamesTheSeat(t *testing.T) {
	enc, _ := wakeHall(t, monster("m-guard", 8, 2, holds()))
	data := enc.ToData()
	for _, m := range data.Members {
		require.Empty(t, m.Clock, "%s on the world clock writes no name", m.ID)
	}
	for i := range data.Members {
		data.Members[i].Clock = worldClockName
	}
	loaded, err := LoadEncounter(&LoadEncounterInput{Data: data, Capabilities: wakeCapabilities(passDriver{})})
	require.NoError(t, err, "the world clock's own name loads")
	require.Equal(t, worldClockName, loaded.members["m-guard"].Clock)

	for i := range data.Members {
		data.Members[i].Clock = "tomb"
	}
	_, err = LoadEncounter(&LoadEncounterInput{Data: data, Capabilities: wakeCapabilities(passDriver{})})
	require.ErrorIs(t, err, ErrInvalidData, "an area clock is S7's")
}

// TestLoad_MachineRefusals: a stored machine names a state its recipe has,
// sits on a member that runs one, and holds only scalars.
func TestLoad_MachineRefusals(t *testing.T) {
	enc, _ := wakeHall(t, monster("m-guard", 8, 2, holds()), monster("m-plain", 12, 2, nil))
	for name, spoil := range map[string]func(m *MemberData){
		"a state the recipe lacks": func(m *MemberData) {
			if m.ID == "m-guard" {
				m.Machine = &MachineData{State: "asleep"}
			}
		},
		"a machine on a member with no table": func(m *MemberData) {
			if m.ID == "m-plain" {
				m.Machine = &MachineData{State: defaultInitial}
			}
		},
		"a bag value that is not a scalar": func(m *MemberData) {
			if m.ID == "m-guard" {
				m.Machine = &MachineData{Bag: map[string]any{"seen": []any{1}}}
			}
		},
	} {
		t.Run(name, func(t *testing.T) {
			data := enc.ToData()
			for i := range data.Members {
				spoil(&data.Members[i])
			}
			_, err := LoadEncounter(&LoadEncounterInput{Data: data, Capabilities: wakeCapabilities(passDriver{})})
			require.ErrorIs(t, err, ErrInvalidData)
		})
	}
}
