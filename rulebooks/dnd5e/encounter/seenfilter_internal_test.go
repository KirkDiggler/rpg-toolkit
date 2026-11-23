// Copyright (C) 2026 Kirk Diggler
// SPDX-License-Identifier: GPL-3.0-or-later

package encounter

// seenfilter_internal_test.go pins A SIGHTING WAKES ONLY A CREATURE WHOSE
// STEP COULD FIRE FOR THE ONE SEEN (rpg-project#1123, slice
// rpg-project#1126): when someone comes into a creature's sight, the engine
// first asks what the creature's current `seen` steps REQUIRE of the one
// seen, and lands no beat for someone no step could fire for. Internal,
// because the landed beats and the intel store are the composition's own.
//
// The patterns are slice 2's ([saHall], [saSees]): a sight refresh lands the
// beats and does not match them; the world's own pass matches them; a hostile
// in sight forms a fight, which is a clock of its own ([saWake]).

import (
	"encoding/json"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/KirkDiggler/rpg-toolkit/mind/machine"
)

// sfOff turns the filter off for the rest of the test, restoring it after.
func sfOff(t *testing.T) {
	t.Helper()
	was := seenFilterOn
	seenFilterOn = false
	t.Cleanup(func() { seenFilterOn = was })
}

func sfNoted() *machine.Guard {
	return &machine.Guard{Fact: &machine.FactLeaf{Name: MachineNoted(string(nvTopic)), Op: machine.OpEq, Value: true}}
}

func sfBarred() *machine.Guard { return saFact(MachineNotedAs(string(nvTopic)), "barred") }

func sfHostile() *machine.Guard { return saFact(saStance, string(StanceHostile)) }

func sfAll(gs ...*machine.Guard) *machine.Guard {
	g := &machine.Guard{}
	for _, one := range gs {
		g.All = append(g.All, *one)
	}

	return g
}

func sfAny(gs ...*machine.Guard) *machine.Guard {
	g := &machine.Guard{}
	for _, one := range gs {
		g.Any = append(g.Any, *one)
	}

	return g
}

func sfNot(g *machine.Guard) *machine.Guard { return &machine.Guard{Not: g} }

func TestSeenFilterWhatAGuardRequires(t *testing.T) {
	barred := seenLeaf{kind: seenNotedAs, topic: nvTopic, value: "barred"}
	noted := seenLeaf{kind: seenNotedTopic, topic: nvTopic}
	hostile := seenLeaf{kind: seenStanceIs, value: string(StanceHostile)}

	for _, tc := range []struct {
		name string
		g    *machine.Guard
		want []seenLeaf
	}{
		{"its own noted.as leaf", sfBarred(), []seenLeaf{barred}},
		{"under all", sfAll(sfBarred()), []seenLeaf{barred}},
		{"under all inside all", sfAll(sfAll(sfBarred())), []seenLeaf{barred}},
		{"under any requires nothing", sfAny(sfBarred()), nil},
		{"under not requires nothing", sfNot(sfBarred()), nil},
		{"noted eq true requires the note", sfNoted(), []seenLeaf{noted}},
		{"noted eq false requires nothing", saFact(MachineNoted(string(nvTopic)), false), nil},
		{"noted.as the empty string requires nothing", saFact(MachineNotedAs(string(nvTopic)), ""), nil},
		{"stance eq hostile requires the stance", sfHostile(), []seenLeaf{hostile}},
		{"a note and a stance under all require both", sfAll(sfBarred(), sfHostile()), []seenLeaf{barred, hostile}},
		{"a required leaf beside an any is still required", sfAll(sfBarred(), sfAny(sfHostile(), sfNoted())), []seenLeaf{barred}},
		{"a nil guard requires nothing", nil, nil},
		{"a scope-only guard requires nothing", &machine.Guard{Scope: &machine.ScopeLeaf{Within: 3}}, nil},
		{"a flag-only guard requires nothing", &machine.Guard{Flag: "armed"}, nil},
	} {
		t.Run(tc.name, func(t *testing.T) {
			require.Equal(t, tc.want, requiredOfSeen(tc.g))
		})
	}
}

// sfCallsOutOnBarred is calm -> calls-out on seeing the one he noted barred.
func sfCallsOutOnBarred() map[string]machine.State { return saCallsOut() }

func TestSeenFilterAStrangerCostsNoBeat(t *testing.T) {
	enc, _, sight := saHall(t, sfCallsOutOnBarred(), saPerson("bob", 4))
	tbHold(t, enc, "barred", "zed")

	require.Empty(t, saSees(t, enc, sight), "he noted nothing about alice or bob: no seen beat is landed on him")
	require.NoError(t, enc.spendWorldAction("alice"))
	require.Equal(t, "calm", stateOf(enc, soGuard))
}

func TestSeenFilterTheOneHeNotedIsStillWokenFor(t *testing.T) {
	enc, _, sight := saHall(t, sfCallsOutOnBarred(), saPerson("bob", 4))
	tbHold(t, enc, "barred", "alice")

	require.Equal(t, []MemberID{"alice"}, saSees(t, enc, sight), "one beat, naming alice and not bob")
	require.NoError(t, enc.spendWorldAction("alice"))
	require.Equal(t, "calls-out", stateOf(enc, soGuard))
}

func TestSeenFilterTheValueIsAsked(t *testing.T) {
	enc, _, sight := saHall(t, sfCallsOutOnBarred())
	tbHold(t, enc, "warned", "alice")

	require.Empty(t, saSees(t, enc, sight), "alice is noted warned and his step asks for barred: no beat")
}

func TestSeenFilterEnemiesOnly(t *testing.T) {
	states := map[string]machine.State{
		"calm":  {Transitions: []machine.Transition{{On: beatSeen, When: sfHostile(), To: "alarm", Reason: "an enemy"}}},
		"alarm": {},
	}
	t.Run("a neutral lands no beat", func(t *testing.T) {
		enc, _, sight := saHall(t, states, saPerson("bob", 4))
		require.Empty(t, saSees(t, enc, sight))
	})
	t.Run("a hostile lands one", func(t *testing.T) {
		enc, _, sight := saHall(t, states, saRaider("rex", 4))
		require.Equal(t, []MemberID{"rex"}, saSees(t, enc, sight), "alice is neutral and filtered; rex is his enemy")
		saWake(t, enc)
		require.Equal(t, "alarm", stateOf(enc, soGuard))
	})
}

func TestSeenFilterAnyOneStepIsEnough(t *testing.T) {
	states := map[string]machine.State{
		"calm": {Transitions: []machine.Transition{
			{On: beatSeen, When: sfNoted(), To: "knows-you", Reason: "noted"},
			{On: beatSeen, When: sfHostile(), To: "alarm", Reason: "an enemy"},
		}},
		"knows-you": {}, "alarm": {},
	}
	t.Run("a neutral he noted", func(t *testing.T) {
		enc, _, sight := saHall(t, states, saPerson("bob", 4))
		tbHold(t, enc, "barred", "bob")
		require.Equal(t, []MemberID{"bob"}, saSees(t, enc, sight))
	})
	t.Run("a hostile he has not noted", func(t *testing.T) {
		enc, _, sight := saHall(t, states, saRaider("rex", 4))
		require.Equal(t, []MemberID{"rex"}, saSees(t, enc, sight))
	})
	t.Run("a neutral stranger", func(t *testing.T) {
		enc, _, sight := saHall(t, states, saPerson("bob", 4))
		require.Empty(t, saSees(t, enc, sight))
	})
}

func TestSeenFilterAStepThatRequiresNothingIsNeverFiltered(t *testing.T) {
	states := map[string]machine.State{
		"calm": {Transitions: []machine.Transition{
			{On: beatSeen, When: sfNoted(), To: "knows-you", Reason: "noted"},
			{On: beatSeen, To: "curious", Reason: "anyone"},
		}},
		"knows-you": {}, "curious": {},
	}
	enc, _, sight := saHall(t, states, saPerson("bob", 4))

	require.Equal(t, []MemberID{"alice", "bob"}, saSees(t, enc, sight), "strangers both: the open step wants them")
}

func TestSeenFilterNotNotedIsNeverFiltered(t *testing.T) {
	states := map[string]machine.State{
		"calm": {Transitions: []machine.Transition{
			{On: beatSeen, When: sfNot(sfNoted()), To: "first-time", Reason: "never seen you before"},
		}},
		"first-time": {},
	}
	enc, _, sight := saHall(t, states, saPerson("bob", 4))

	require.Equal(t, []MemberID{"alice", "bob"}, saSees(t, enc, sight), "a not of a note requires nothing")
	require.NoError(t, enc.spendWorldAction("alice"))
	require.Equal(t, "first-time", stateOf(enc, soGuard))
}

// sfTwoStates is a guard in `from` who steps to `to` on a topic, one state
// asking for the one he barred and the other asking for anyone. A topic is
// how the test moves him between the sighting's refresh and the world's pass.
func sfTwoStates() map[string]machine.State {
	states := map[string]machine.State{
		"strict": {Transitions: []machine.Transition{
			{On: beatTopic, To: "open", Reason: "told"},
			{On: beatSeen, When: sfBarred(), To: "calls-out", Reason: "known on sight"},
		}},
		"open": {Transitions: []machine.Transition{
			{On: beatTopic, To: "strict", Reason: "told"},
			{On: beatSeen, To: "greeted", Reason: "anyone"},
		}},
		"calls-out": {}, "greeted": {},
	}

	return states
}

func sfTwoStateHall(t *testing.T, initial string, members ...MemberInput) (*Encounter, *SetupInput, *saSight) {
	t.Helper()
	g := soGuardIn(t, nil)
	g.Behaviors.Instances[0].Instance.Recipe = *recipe(t, initial, sfTwoStates())
	sight := &saSight{}
	in := postedSetup(sight, append([]MemberInput{g}, members...)...)
	enc, err := NewEncounter(in)
	require.NoError(t, err)

	return enc, in, sight
}

func TestSeenFilterTheRequirementFollowsTheState(t *testing.T) {
	t.Run("strict then open", func(t *testing.T) {
		enc, _, sight := sfTwoStateHall(t, "strict", saPerson("bob", 4), saPerson("zed", 5))
		tbHold(t, enc, "warned", "zed")
		require.NoError(t, enc.spendWorldAction("alice"))
		require.Equal(t, "open", stateOf(enc, soGuard), "precondition: told, he stepped to the state that wants anyone")

		require.Equal(t, []MemberID{"alice", "bob", "zed"}, saSees(t, enc, sight), "a stranger now lands a beat")
	})
	t.Run("open then strict", func(t *testing.T) {
		enc, _, sight := sfTwoStateHall(t, "open", saPerson("bob", 4), saPerson("zed", 5))
		tbHold(t, enc, "warned", "zed")
		require.NoError(t, enc.spendWorldAction("alice"))
		require.Equal(t, "strict", stateOf(enc, soGuard), "precondition: told, he stepped to the state that wants the barred")

		require.Empty(t, saSees(t, enc, sight), "a stranger now lands none")
	})
}

func TestSeenFilterItIsRebuiltOnLoad(t *testing.T) {
	enc, in, sight := saHall(t, sfCallsOutOnBarred(), saPerson("bob", 4))
	tbHold(t, enc, "barred", "alice")
	require.NoError(t, enc.spendWorldAction("alice"))

	raw, err := json.Marshal(enc.ToData())
	require.NoError(t, err)
	var data EncounterData
	require.NoError(t, json.Unmarshal(raw, &data))
	loaded, err := LoadEncounter(&LoadEncounterInput{Data: data, Capabilities: in.Capabilities})
	require.NoError(t, err)
	require.Equal(t, "calm", stateOf(loaded, soGuard), "precondition: he has not stepped")

	require.Equal(t, []MemberID{"alice"}, saSees(t, loaded, sight),
		"loaded, the requirement is rebuilt: the noted one lands a beat and bob lands none")
}

// sfGuardRecipe is a guard whose `seen` step on `when` toggles him between
// calm and alert, so each wake is a step the records show.
func sfGuardRecipe(t *testing.T, when *machine.Guard) *machine.Recipe {
	t.Helper()

	return recipe(t, "calm", map[string]machine.State{
		"calm":  {Transitions: []machine.Transition{{On: beatSeen, When: when, To: "alert", Reason: "seen"}}},
		"alert": {Transitions: []machine.Transition{{On: beatSeen, When: when, To: "calm", Reason: "seen again"}}},
	})
}

const (
	sfGNote    = MemberID("g-note")
	sfGHostile = MemberID("g-hostile")
	sfGAny     = MemberID("g-any")
	sfGFresh   = MemberID("g-fresh")
)

var sfGuards = []MemberID{sfGNote, sfGHostile, sfGAny, sfGFresh}

// sfScene is four guards who each see only when the script lets them, beside
// alice, bob, zed and a raider.
func sfScene(t *testing.T) (*Encounter, soSight) {
	t.Helper()
	whens := []*machine.Guard{sfBarred(), sfHostile(), nil, sfNot(sfNoted())}
	var members []MemberInput
	for i, id := range sfGuards {
		g := soListener(t, id, float64(6+i), 4, sfGuardRecipe(t, whens[i]), nil)
		members = append(members, g)
	}
	members = append(members, saPerson("bob", 3), saPerson("zed", 4), saRaider("rex", 5))
	sight := soSight{}
	for _, id := range sfGuards {
		sight[id] = 0
	}
	in := postedSetup(sight, members...)
	in.Field.Factions = append(in.Field.Factions, FactionInput{ID: saRaiders})
	in.Field.Dispositions = append(in.Field.Dispositions,
		DispositionInput{Between: [2]FactionID{postedGarrison, saRaiders}, Stance: StanceHostile})
	enc, err := NewEncounter(in)
	require.NoError(t, err)

	return enc, sight
}

// sfHold has holder hold nvTopic about the subjects as `as`.
func sfHold(t *testing.T, enc *Encounter, holder MemberID, as string, subjects ...MemberID) {
	t.Helper()
	reach, err := enc.placedSightReach()
	require.NoError(t, err)
	require.NoError(t, enc.holdTopic(reach, holder, subjects, nvTopic, ChannelJudgement,
		recognitionLine{speaker: "alice", spokenTo: holder, as: as}))
}

// sfRun plays the one scripted run and returns the guards' states, the run's
// data marshalled and how many `seen` beats it landed.
func sfRun(t *testing.T) (map[MemberID]string, string, int) {
	t.Helper()
	enc, sight := sfScene(t)
	landed := 0
	look := func(range_ int) {
		for _, id := range sfGuards {
			sight[id] = range_
		}
		before := len(enc.landed)
		_, _, err := enc.refreshSight(enc.rosterIDs())
		require.NoError(t, err)
		for _, b := range enc.landed[before:] {
			if b.Beat == beatSeen {
				landed++
			}
		}
		saWake(t, enc)
	}
	const far = 1_000_000
	look(far)                                // 1 everyone comes into view
	look(0)                                  // 2 and out of it
	sfHold(t, enc, sfGNote, "barred", "bob") // 3 notes are given
	sfHold(t, enc, sfGHostile, "warned", "zed")
	look(far)                                // 4 back in view
	look(0)                                  // 5 out again
	sfHold(t, enc, sfGNote, "warned", "bob") // 6 a note changes its value
	sfHold(t, enc, sfGFresh, "barred", "zed")
	look(far) // 7
	look(0)   // 8
	look(far) // 9

	states := map[MemberID]string{}
	for _, id := range sfGuards {
		states[id] = stateOf(enc, id)
	}
	raw, err := json.Marshal(enc.ToData())
	require.NoError(t, err)

	return states, string(raw), landed
}

func TestSeenFilterTheSamePlayBothWays(t *testing.T) {
	onStates, onData, onLanded := sfRun(t)
	sfOff(t)
	offStates, offData, offLanded := sfRun(t)

	require.Equal(t, offStates, onStates, "the guards end in the same states")
	require.Equal(t, offData, onData, "and the two runs' records are equal entry for entry (EncounterData, marshalled)")
	require.Less(t, onLanded, offLanded, "with the filter fewer seen beats were landed: %d against %d", onLanded, offLanded)
}

// TestSeenFilterTheStateAtTheSightingDecides pins THE LISTENING RULE, not an
// accident: whether a sighting wakes a creature is decided by the state it is
// in WHEN THE SIGHTING HAPPENS, as it already is for listening at all. A
// guard in `strict` (barred only) is told something that moves him to `open`
// (anyone), and a stranger's sighting is landed while he is still in `strict`,
// before the world's pass. With the filter the strict state's requirement
// already decided: bob does not wake him. Without it the beat would have been
// landed and `open` asked.
func TestSeenFilterTheStateAtTheSightingDecides(t *testing.T) {
	run := func(t *testing.T) string {
		enc, _, sight := sfTwoStateHall(t, "strict", saPerson("bob", 4))
		tbHold(t, enc, "warned", "alice") // lands a topic beat in strict, who steps to open at the pass
		require.NotEmpty(t, enc.landed, "precondition: the topic beat waits for the pass")
		sight.guard = 1_000_000
		_, _, err := enc.refreshSight(enc.rosterIDs()) // bob's sighting lands (or not) while he is in strict
		require.NoError(t, err)
		require.NoError(t, enc.spendWorldAction("alice"))

		return stateOf(enc, soGuard)
	}
	t.Run("with the filter he is not woken by it", func(t *testing.T) {
		require.Equal(t, "open", run(t), "told, he moved to open; the sighting before it did not wake him")
	})
	t.Run("without it the new state would be asked", func(t *testing.T) {
		sfOff(t)
		require.Equal(t, "greeted", run(t))
	})
}
