// Copyright (C) 2026 Kirk Diggler
// SPDX-License-Identifier: GPL-3.0-or-later

package encounter

// machineargs_internal_test.go holds what each machine word TAKES (its
// Args, rpg-project#997) to the compiler: a step built only from the
// table's description of a word's arguments compiles, and the arguments
// described reach every field the compiler reads (by reflection), so a
// studio can build any word's input from the table alone.

import (
	"reflect"
	"slices"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/KirkDiggler/rpg-toolkit/mind/machine"
)

// argsBag is the bag every built recipe declares: one bool flag.
const argsFlag = "lit"

// sampleOf is a value a studio could offer for arg, chosen only from what
// the arg says: what it names, its kind and its closed values.
func sampleOf(t *testing.T, word string, a Arg) any {
	t.Helper()
	if len(a.Values) > 0 {
		return a.Values[len(a.Values)-1]
	}
	switch a.Names {
	case SlotState:
		return "s"
	case SlotList:
		return string(AnswerTime)
	case SlotFlag:
		return argsFlag
	case SlotFact:
		return "gate-cowed"
	case SlotTopic:
		return "admitted"
	case SlotClock:
		return "bell:gong"
	case SlotCondition:
		return machine.Guard{Flag: argsFlag}
	}
	switch a.Kind {
	case "int":
		return 2
	case "bool", "key": // the bag's flag is a bool
		return true
	case "string":
		return "x"
	}
	t.Fatalf("%s: an argument %+v a studio cannot offer anything for", word, a)
	return nil
}

// argsOf is the samples for every argument a word describes, in order; at
// fails the test when the word needs an argument the table does not
// describe.
func argsOf(t *testing.T, word string, args []Arg) func(i int) any {
	t.Helper()
	values := make([]any, len(args))
	for i, a := range args {
		values[i] = sampleOf(t, word, a)
	}
	return func(i int) any {
		t.Helper()
		require.Less(t, i, len(values), "%s needs an argument %d the table does not describe", word, i)
		return values[i]
	}
}

// argsRecipe is a one-state recipe of one step, with the bag's flag.
func argsRecipe(t *testing.T, step machine.Transition) *machine.Recipe {
	r := wordsRecipe(t, step)
	r.Bag = machine.Bag{argsFlag: false}
	return r
}

// actFrom is the step an act word makes from its arguments alone.
func actFrom(t *testing.T, w ActWord) machine.Transition {
	t.Helper()
	at := argsOf(t, w.Word, w.Args)
	tr := machine.Transition{On: machine.Beat(AnswerTime)}
	switch w.Word {
	case "to":
		tr.To = at(0).(string)
	case "pick":
		tr.Do = []machine.Act{{Pick: at(0).(string)}}
	case "wake_in":
		tr.Do = []machine.Act{{WakeIn: at(0).(int)}}
	case "set":
		tr.Do = []machine.Act{{Set: &machine.SetAct{Key: at(0).(string), Value: at(1)}}}
	case "clear":
		tr.Do = []machine.Act{{Clear: []string{at(0).(string)}}}
	case "anchor":
		tr.Do = []machine.Act{{Anchor: true}}
	default:
		t.Fatalf("the act %q has no builder here: say how its arguments make a step", w.Word)
	}
	return tr
}

// guardFrom is the guard a condition word makes from its arguments alone.
func guardFrom(t *testing.T, w ConditionWord) machine.Guard {
	t.Helper()
	at := argsOf(t, w.Word, w.Args)
	switch w.Word {
	case "flag":
		return machine.Guard{Flag: at(0).(string)}
	case "within":
		return machine.Guard{Scope: &machine.ScopeLeaf{Within: at(0).(int)}}
	case "regions":
		return machine.Guard{Scope: &machine.ScopeLeaf{Regions: at(0).(int)}}
	case "rounds":
		return machine.Guard{Rounds: &machine.RoundsLeaf{Op: machine.Op(at(0).(string)), N: at(1).(int)}}
	case "all":
		return machine.Guard{All: []machine.Guard{at(0).(machine.Guard)}}
	case "any":
		return machine.Guard{Any: []machine.Guard{at(0).(machine.Guard)}}
	case "not":
		inner := at(0).(machine.Guard)
		return machine.Guard{Not: &inner}
	}
	require.True(t, w.Fact, "%s has a builder here", w.Word)
	name, next := w.Word, 0
	if w.Open != SlotNone {
		prefix := w.Word
		if inner, _ := ConditionWordOf(w.Word + "x"); inner.engine != "" {
			prefix = inner.engine // as the engine reads it once lowered
		}
		name, next = prefix+at(0).(string), 1
	}
	return machine.Guard{Fact: &machine.FactLeaf{Name: machine.FactName(name), Op: machine.OpEq, Value: at(next)}}
}

// beatFor is a beat a word is allowed on.
func beatFor(on []string) machine.Beat {
	if len(on) == 0 {
		return machine.Beat(AnswerTime)
	}
	return machine.Beat(on[0])
}

// TestMachineWords_EveryWordIsBuiltFromItsArgs: every act and condition the
// published table holds, built only from its Args, compiles; and the
// arguments reach every field the compiler reads.
func TestMachineWords_EveryWordIsBuiltFromItsArgs(t *testing.T) {
	table := MachineWords()
	var acts []machine.Act
	var guards []machine.Guard
	for _, w := range table.Acts {
		tr := actFrom(t, w)
		r := argsRecipe(t, tr)
		_, err := CheckBehaviors(&CheckBehaviorsInput{Behaviors: *fixtureBehavior(*r, holds())})
		require.NoError(t, err, "the act %s built from its args", w.Word)
		acts = append(acts, tr.Do...)
		if tr.To != "" {
			acts = append(acts, machine.Act{}) // `to` is the step's own field, below
		}
	}
	for _, w := range table.Conditions {
		g := guardFrom(t, w)
		tr := machine.Transition{On: beatFor(w.On), When: &g}
		require.NoError(t, ValidateMachine(argsRecipe(t, tr)), "the condition %s built from its args", w.Word)
		guards = append(guards, g)
	}
	for _, b := range table.Beats {
		if b.Open == SlotNone {
			require.Empty(t, b.Args, "%s takes nothing", b.Word)
			continue
		}
		at := argsOf(t, b.Word, b.Args)
		on := b.Word + at(0).(string)
		require.NoError(t, ValidateMachine(argsRecipe(t, machine.Transition{On: machine.Beat(on)})), on)
	}

	// EVERY FIELD THE COMPILER READS IS REACHED by some word's arguments: a
	// new field on an act or a guard leaf fails here until a word takes it.
	covered := func(name string, values []reflect.Value) {
		t.Helper()
		typ := values[0].Type()
		for i := 0; i < typ.NumField(); i++ {
			hit := slices.ContainsFunc(values, func(v reflect.Value) bool { return !v.Field(i).IsZero() })
			require.True(t, hit, "%s.%s is set by no word's arguments", name, typ.Field(i).Name)
		}
	}
	var actValues, setValues []reflect.Value
	for _, a := range acts {
		actValues = append(actValues, reflect.ValueOf(a))
		if a.Set != nil {
			setValues = append(setValues, reflect.ValueOf(*a.Set))
		}
	}
	covered("machine.Act", actValues)
	covered("machine.SetAct", setValues)
	var guardValues, factValues, scopeValues, roundsValues []reflect.Value
	for _, g := range guards {
		guardValues = append(guardValues, reflect.ValueOf(g))
		if g.Fact != nil {
			factValues = append(factValues, reflect.ValueOf(*g.Fact))
		}
		if g.Scope != nil {
			scopeValues = append(scopeValues, reflect.ValueOf(*g.Scope))
		}
		if g.Rounds != nil {
			roundsValues = append(roundsValues, reflect.ValueOf(*g.Rounds))
		}
	}
	covered("machine.Guard", guardValues)
	covered("machine.FactLeaf", factValues)
	covered("machine.ScopeLeaf", scopeValues)
	covered("machine.RoundsLeaf", roundsValues)
	require.Contains(t, []string{"s"}, actFrom(t, mustAct(t, "to")).To, "`to` reaches the step's To")
}

// mustAct is the published act word.
func mustAct(t *testing.T, word string) ActWord {
	for _, a := range MachineWords().Acts {
		if a.Word == word {
			return a
		}
	}
	t.Fatalf("no act %q", word)
	return ActWord{}
}

// TestMachineWords_ArgsAgreeWithTheWord: an open word's first argument
// names what its open part is, and a condition's value argument has the
// word's own kind and closed values.
func TestMachineWords_ArgsAgreeWithTheWord(t *testing.T) {
	table := MachineWords()
	for _, b := range table.Beats {
		if b.Open != SlotNone {
			require.Equal(t, b.Open, b.Args[0].Names, b.Word)
		}
	}
	for _, c := range table.Conditions {
		if c.Open != SlotNone {
			require.Equal(t, c.Open, c.Args[0].Names, c.Word)
		}
		for _, a := range c.Args {
			if a.Slot == "{value}" && len(c.Values) > 0 {
				require.Equal(t, c.Values, a.Values, c.Word)
				require.Equal(t, c.Kind, a.Kind, c.Word)
			}
		}
	}
}
