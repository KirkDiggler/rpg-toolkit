// Copyright (C) 2026 Kirk Diggler
// SPDX-License-Identifier: GPL-3.0-or-later

package encounter

// wakebeats_internal_test.go pins TRIGGERS WITH SCOPE (rpg-project#766 S4,
// wakebeats.go): a sound wakes the listeners in its scope in the raise it
// landed in, a sleeper out of scope is never asked, a fight's round lands
// once per faction, and the rounds a creature slept are not owed after a
// save. Internal, because the landed beats, the listener index and the clock
// budget are the composition's own.

import (
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/KirkDiggler/rpg-toolkit/core"
	"github.com/KirkDiggler/rpg-toolkit/mind/machine"
	"github.com/KirkDiggler/rpg-toolkit/play/clock"
)

// earsSheets is a sheet every member hears with: a passive Perception far
// above any sound's DC, thirty feet of speed.
type earsSheets struct{}

func (earsSheets) Sheets(members []MemberID) (map[MemberID]SheetFacts, error) {
	out := make(map[MemberID]SheetFacts, len(members))
	for _, id := range members {
		out[id] = SheetFacts{PassivePerception: 40, SpeedFeet: 30}
	}

	return out, nil
}

// earsHall is [wakeHall] with members who hear.
func earsHall(t *testing.T, members ...MemberInput) (*Encounter, *askedDriver) {
	t.Helper()
	driver := &askedDriver{inner: TableDriver{Roller: wakeRollsOne{}}}
	in := wakeSetup(driver, members...)
	in.Sheets = earsSheets{}
	enc, err := NewEncounter(in)
	require.NoError(t, err)

	return enc, driver
}

// recipe is an authored recipe under a test ref.
func recipe(t *testing.T, initial string, states map[string]machine.State) *machine.Recipe {
	t.Helper()
	ref, err := core.ParseString("dnd5e:machines:test")
	require.NoError(t, err)

	return &machine.Recipe{Ref: *ref, Initial: initial, States: states}
}

// listener is a monster at (col,row) holding on `time`, running r.
func listener(id MemberID, col, row float64, r *machine.Recipe) MemberInput {
	m := monster(id, col, row, holds())
	m.Machine = r

	return m
}

// postedRecipe is the gate post's recipe: posted until a sound within six
// hexes, then alert, rolling `time` every round.
func postedRecipe(t *testing.T) *machine.Recipe {
	return recipe(t, "posted", map[string]machine.State{
		"posted": {Transitions: []machine.Transition{
			{On: beatHeard, When: &machine.Guard{Scope: &machine.ScopeLeaf{Within: 6}}, To: "alert", Reason: "heard something"},
		}},
		"alert": {Transitions: []machine.Transition{pickTime}},
	})
}

// stateOf is a member's machine state, the initial one read for empty.
func stateOf(enc *Encounter, id MemberID) string {
	m := enc.members[id]
	if m.Machine.State != "" {
		return m.Machine.State
	}

	return enc.programFor(m).NewBlob().State
}

// TestWake_SoundWakesListenersSameRaise is W10: a clang lands on a posted
// guard three hexes away and on one a dozen away; the raise the clang's verb
// paid moves the near one to `alert` in that raise — the state beat stamped
// with the raise's own round, after its tick — and leaves the far one posted.
// Posted listens for no time, so neither was on the heap before.
func TestWake_SoundWakesListenersSameRaise(t *testing.T) {
	enc, driver := earsHall(t,
		listener("m-near", 4, 1, postedRecipe(t)),
		listener("m-far", 15, 6, postedRecipe(t)),
	)
	require.Equal(t, 0, enc.due.Len(), "posted wants no time")

	at := uint64(enc.clock.ToData().HighWater)
	require.NoError(t, enc.makeSound(noiseAttack, "alice", at))
	require.Len(t, enc.landed, 2, "the clang landed on both hearers, who listen")
	require.NoError(t, enc.spendWorldAction("alice"))

	require.Equal(t, "alert", stateOf(enc, "m-near"), "the near guard woke in this raise")
	require.Equal(t, "posted", stateOf(enc, "m-far"), "the far guard is out of its scope")
	require.Empty(t, enc.landed, "the raise matched what landed")
	states := beatsTagged(t, enc, BeatState)
	require.Len(t, states, 1)
	require.Equal(t, "m-near", states[0]["member"])
	require.Equal(t, "posted", states[0]["from"])
	require.Equal(t, "alert", states[0]["to"])
	require.Empty(t, driver.asked, "a transition with no pick asks nothing")
	require.Equal(t, uint64(enc.clock.ToData().HighWater)+1, enc.dueAt["m-near"], "alert is due at the next round")

	require.NoError(t, enc.spendWorldAction("alice"))
	require.Equal(t, []MemberID{"m-near"}, driver.asked, "alert rolls its time from the next round on")
}

// TestWake_AsleepIgnoresFarSound: a barracks asleep, waking only to a sound
// within two hexes, hears a clang ten hexes off. It is never stepped: no view
// built, no beat, no state change, still off the heap.
func TestWake_AsleepIgnoresFarSound(t *testing.T) {
	asleep := recipe(t, "asleep", map[string]machine.State{
		"asleep": {Transitions: []machine.Transition{
			{On: beatHeard, When: &machine.Guard{Scope: &machine.ScopeLeaf{Within: 2}}, To: "posted"},
		}},
		"posted": {Transitions: []machine.Transition{pickTime}},
	})
	enc, driver := earsHall(t, listener("m-bunk", 11, 1, asleep))

	for i := 0; i < 10; i++ {
		at := uint64(enc.clock.ToData().HighWater)
		require.NoError(t, enc.makeSound(noiseAttack, "alice", at))
		require.NoError(t, enc.spendWorldAction("alice"))
	}

	require.Equal(t, "asleep", stateOf(enc, "m-bunk"))
	require.Empty(t, driver.asked)
	require.Empty(t, beatsTagged(t, enc, BeatState))
	require.Equal(t, 0, enc.due.Len())
}

// TestWake_ClockFightOncePerFaction is W6: a fight's round lands `clock:
// fight` once for every faction among its fighters, however many of each
// fight; a reserve of the monsters' side picks on it once, without spending
// the world clock, and a listener of another side is not woken.
func TestWake_ClockFightOncePerFaction(t *testing.T) {
	waiting := func() *machine.Recipe {
		return recipe(t, "waiting", map[string]machine.State{
			"waiting": {Transitions: []machine.Transition{
				{On: clockBeat(clockFight), Do: []machine.Act{{Pick: string(AnswerTime)}}},
			}},
		})
	}
	other := listener("m-other", 16, 6, waiting())
	other.Faction = "wanderers"
	in := wakeSetup(&askedDriver{inner: TableDriver{Roller: wakeRollsOne{}}},
		monster("m-a", 4, 1, holds()), monster("m-b", 6, 1, holds()),
		listener("m-res", 12, 6, waiting()), other,
	)
	in.Field.Factions = []FactionInput{{ID: "wanderers"}}
	enc, err := NewEncounter(in)
	require.NoError(t, err)
	driver := enc.driver.(*askedDriver)

	enc.landFightRound([]MemberID{"alice", "m-a", "m-b"}, uint64(enc.clock.ToData().HighWater))
	require.Len(t, enc.landed, 2, "one for the monsters, one for the party")
	require.Equal(t, FactionID(FactionMonsters), enc.landed[0].Faction)
	require.Equal(t, FactionID(FactionParty), enc.landed[1].Faction)

	before, err := enc.clock.Budget(&clock.BudgetInput{ID: "m-res"})
	require.NoError(t, err)
	require.NoError(t, enc.wakePass(false, false))

	require.Equal(t, []MemberID{"m-res"}, driver.asked, "the monsters' reserve picked once; the wanderer was not addressed")
	after, err := enc.clock.Budget(&clock.BudgetInput{ID: "m-res"})
	require.NoError(t, err)
	require.Equal(t, before, after, "a reaction spends no world round")
	require.Empty(t, beatsTagged(t, enc, BeatTick), "a round that raised nothing writes no tick")
}

// TestWake_SleptRoundsAreNotOwed is #774's carry-over: a creature that picks
// and sleeps three rounds on an authored wake_in, saved and loaded while it
// sleeps, is asked once at its wake and once at each round after — never the
// slept rounds again on a later load. Without the wake paying the rounds it
// slept, the budget they left behind read as owed at the next load and the
// raise after it asked three times.
func TestWake_SleptRoundsAreNotOwed(t *testing.T) {
	naps := recipe(t, "watch", map[string]machine.State{
		"watch": {Transitions: []machine.Transition{
			{On: beatTime, To: "awake", Do: []machine.Act{{Pick: string(AnswerTime)}, {WakeIn: 3}}},
		}},
		"awake": {Transitions: []machine.Transition{pickTime}},
	})
	enc, driver := wakeHall(t, listener("m-sleeper", 8, 2, naps))
	caps := wakeCapabilities(driver)

	reload := func() {
		t.Helper()
		loaded, err := LoadEncounter(&LoadEncounterInput{Data: enc.ToData(), Capabilities: caps})
		require.NoError(t, err)
		enc = loaded
	}

	require.NoError(t, enc.spendWorldAction("alice")) // round 1: pick, sleep to 4
	require.Equal(t, []MemberID{"m-sleeper"}, driver.asked)
	require.NoError(t, enc.spendWorldAction("alice")) // round 2: asleep
	require.NoError(t, enc.spendWorldAction("alice")) // round 3: asleep
	require.Len(t, driver.asked, 1, "nobody asks a sleeper")
	reload()
	require.NoError(t, enc.spendWorldAction("alice")) // round 4: the wake
	require.Len(t, driver.asked, 2, "one ask at the wake, not four")
	reload()
	require.NoError(t, enc.spendWorldAction("alice")) // round 5
	require.Len(t, driver.asked, 3, "one ask the round after, the slept rounds not owed")
	budget, err := enc.clock.Budget(&clock.BudgetInput{ID: "m-sleeper"})
	require.NoError(t, err)
	require.Equal(t, 0, budget, "nothing owed")
}

// TestLoad_LandedBeatWaitsForTheRaise: a clang in a verb that raises nothing
// waits for the raise that does, across a save.
func TestLoad_LandedBeatWaitsForTheRaise(t *testing.T) {
	enc, driver := earsHall(t, listener("m-near", 4, 1, postedRecipe(t)))
	require.NoError(t, enc.makeSound(noiseAttack, "alice", uint64(enc.clock.ToData().HighWater)))
	data := enc.ToData()
	require.Len(t, data.Landed, 1)

	caps := wakeCapabilities(driver)
	caps.Sheets = earsSheets{}
	loaded, err := LoadEncounter(&LoadEncounterInput{Data: data, Capabilities: caps})
	require.NoError(t, err)
	require.NoError(t, loaded.spendWorldAction("alice"))
	require.Equal(t, "alert", stateOf(loaded, "m-near"))
}

// TestSetup_MachineRefusals: a recipe this composition could not run is
// refused at the door — a beat it does not supply, a scope on a beat with
// none, a fact guard naming no fact, a pick of a verdict, a recipe on a
// member with no table — and an authored key the recipe picks is a legal
// table key, judged as `time`.
func TestSetup_MachineRefusals(t *testing.T) {
	scoped := &machine.Guard{Scope: &machine.ScopeLeaf{Within: 2}}
	cases := map[string]MemberInput{
		"unknown beat": listener("m", 4, 1, recipe(t, "a", map[string]machine.State{
			"a": {Transitions: []machine.Transition{{On: "hear", To: "a"}}}})),
		"scope on fact": listener("m", 4, 1, recipe(t, "a", map[string]machine.State{
			"a": {Transitions: []machine.Transition{{On: beatFact, When: scoped, To: "a"}}}})),
		"unknown deed word": listener("m", 4, 1, recipe(t, "a", map[string]machine.State{
			"a": {Transitions: []machine.Transition{{On: "deed:waved", To: "a"}}}})),
		"pick a verdict": listener("m", 4, 1, recipe(t, "a", map[string]machine.State{
			"a": {Transitions: []machine.Transition{{On: beatTime, Do: []machine.Act{{Pick: string(AnswerSuccess)}}}}}})),
		"no table": {ID: "m", Kind: KindMonster, Machine: postedRecipe(t)},
		"key nothing picks": func() MemberInput {
			m := listener("m", 4, 1, postedRecipe(t))
			m.Table = Table{AnswerTime: holds()[AnswerTime], "march": holds()[AnswerTime]}
			return m
		}(),
	}
	for name, m := range cases {
		t.Run(name, func(t *testing.T) {
			_, err := NewEncounter(wakeSetup(passDriver{}, m))
			require.ErrorIs(t, err, ErrBadMember)
		})
	}

	march := recipe(t, "waiting", map[string]machine.State{
		"waiting": {Transitions: []machine.Transition{{On: clockBeat("bell:gong"), Do: []machine.Act{{Pick: "march"}}}}},
	})
	m := listener("m", 4, 1, march)
	m.Table = Table{"march": {{Weight: 1, Hold: true}}}
	_, err := NewEncounter(wakeSetup(passDriver{}, m))
	require.NoError(t, err, "a key the recipe picks is a table key")
}

// TestWake_ReactionForgivesIdleRounds is the reaction half of #774's
// carry-over (review of rpg-toolkit#2219): a posted guard accrues budget
// every raise it is not due; woken by a sound, saved and loaded, it is asked
// once the round after it woke, never its idle rounds replayed.
func TestWake_ReactionForgivesIdleRounds(t *testing.T) {
	enc, driver := earsHall(t, listener("m-near", 4, 1, postedRecipe(t)))
	caps := wakeCapabilities(driver)
	caps.Sheets = earsSheets{}
	for i := 0; i < 5; i++ {
		require.NoError(t, enc.spendWorldAction("alice"))
	}
	require.NoError(t, enc.makeSound(noiseAttack, "alice", uint64(enc.clock.ToData().HighWater)))
	require.NoError(t, enc.spendWorldAction("alice")) // heard -> alert
	loaded, err := LoadEncounter(&LoadEncounterInput{Data: enc.ToData(), Capabilities: caps})
	require.NoError(t, err)
	require.NoError(t, loaded.spendWorldAction("alice"))
	require.Len(t, driver.asked, 1, "one ask the round after it woke, not its idle rounds replayed")
}

// TestWake_CutAtTheBoundIsRecorded: a pass that reaches the bound on landed
// beats drops the rest and says so, with the count it dropped; the count is
// the pass's, so a second raise matches afresh.
func TestWake_CutAtTheBoundIsRecorded(t *testing.T) {
	saved := wakeBeatBound
	wakeBeatBound = 1
	t.Cleanup(func() { wakeBeatBound = saved })
	enc, _ := earsHall(t,
		listener("m-a", 4, 1, postedRecipe(t)),
		listener("m-b", 5, 1, postedRecipe(t)),
	)
	require.NoError(t, enc.makeSound(noiseAttack, "alice", uint64(enc.clock.ToData().HighWater)))
	require.Len(t, enc.landed, 2)
	require.NoError(t, enc.spendWorldAction("alice"))

	require.Equal(t, "alert", stateOf(enc, "m-a"), "the first beat was matched")
	require.Equal(t, "posted", stateOf(enc, "m-b"), "the second was cut")
	cuts := beatsTagged(t, enc, BeatWakeCut)
	require.Len(t, cuts, 1)
	require.Equal(t, float64(1), cuts[0]["dropped"])
	require.Equal(t, float64(1), cuts[0]["matched"])

	require.NoError(t, enc.makeSound(noiseAttack, "alice", uint64(enc.clock.ToData().HighWater)))
	require.NoError(t, enc.spendWorldAction("alice"))
	require.Equal(t, "alert", stateOf(enc, "m-b"), "the next pass matches afresh")
}

// TestWake_FightRoundWithNoReactionPickRefreshesNoSight: a fight round that
// raised nothing, heard by a `clock:fight` listener that only changes state,
// moves nobody and pays no roster-wide sight refresh.
func TestWake_FightRoundWithNoReactionPickRefreshesNoSight(t *testing.T) {
	alarmed := recipe(t, "waiting", map[string]machine.State{
		"waiting": {Transitions: []machine.Transition{{On: clockBeat(clockFight), To: "braced"}}},
		"braced":  {},
	})
	enc, driver := wakeHall(t, monster("m-fighter", 4, 1, holds()), listener("m-res", 12, 6, alarmed))
	encodes := 0
	saved := encodeSighting
	encodeSighting = func(t SightTestimony) ([]byte, error) {
		encodes++
		return saved(t)
	}
	defer func() { encodeSighting = saved }()

	enc.landFightRound([]MemberID{"alice", "m-fighter"}, uint64(enc.clock.ToData().HighWater))
	require.NoError(t, enc.wakePass(false, false))

	require.Equal(t, "braced", stateOf(enc, "m-res"), "the listener heard the round")
	require.Empty(t, driver.asked)
	require.Zero(t, encodes, "nobody moved, so nothing was refreshed")
}
