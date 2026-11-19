// Copyright (C) 2026 Kirk Diggler
// SPDX-License-Identifier: GPL-3.0-or-later

package encounter

// machineargs_internal_test.go holds what each machine word TAKES and where
// it goes (its Written and Args, rpg-project#997) to the compiler. ONE
// generic writer turns any published word, its arguments and chosen values
// into the step an author writes, with no code of its own per word: if it
// can, a client can. Every word written that way compiles, and the words'
// arguments reach every field the compiler reads.

import (
	"encoding/json"
	"errors"
	"reflect"
	"slices"
	"strconv"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/KirkDiggler/rpg-toolkit/mind/machine"
)

// The bag every written recipe declares: two bool flags.
var argsBag = machine.Bag{"lit": false, "lit2": false}

// argSample is what a studio could offer for one argument, chosen only from
// what the published arg says. nth picks a second item for a list.
func argSample(t *testing.T, args []Arg, a Arg, nth int, condition map[string]any) any {
	t.Helper()
	if a.Fixed != "" {
		return typed(t, a.Kind, a.Fixed)
	}
	if len(a.Values) > 0 {
		return a.Values[len(a.Values)-1]
	}
	switch a.Names {
	case SlotState:
		return "s"
	case SlotList:
		return string(AnswerTime)
	case SlotFlag:
		return []string{"lit", "lit2"}[nth]
	case SlotFact:
		return "gate-cowed"
	case SlotTopic:
		return "admitted"
	case SlotClock:
		return "bell:gong"
	case SlotCondition:
		return condition
	}
	switch a.Kind {
	case "int":
		return 2
	case "bool":
		return true
	case "string":
		return "x"
	case "of":
		i := slices.IndexFunc(args, func(o Arg) bool { return o.Name == a.KindOf })
		require.GreaterOrEqual(t, i, 0, "%q takes its kind from %q, which is no argument", a.Name, a.KindOf)
		require.Equal(t, SlotFlag, args[i].Names, "a kind comes from a flag")
		flag := argSample(t, args, args[i], 0, condition).(string)
		kind := reflect.TypeOf(argsBag[flag]).Kind().String()
		require.Contains(t, a.Kinds, kind, "the flag's kind is one of the kinds the arg takes")
		return typed(t, kind, map[string]string{"bool": "true", "int": "2", "string": "x"}[kind])
	}
	t.Fatalf("an argument %+v a studio cannot offer anything for", a)
	return nil
}

// typed is text as a value of kind.
func typed(t *testing.T, kind, text string) any {
	t.Helper()
	switch kind {
	case "bool":
		v, err := strconv.ParseBool(text)
		require.NoError(t, err)
		return v
	case "int":
		v, err := strconv.Atoi(text)
		require.NoError(t, err)
		return v
	}
	return text
}

// writeWord writes one word into step (the step map an author writes) from
// its published Written and Args alone. It reads the word's text only to
// spell it: where it goes and what it takes come from the table.
func writeWord(t *testing.T, step map[string]any, word string, w Written, args []Arg, condition map[string]any) {
	t.Helper()
	writeWordWith(t, step, word, w, args, condition, writeChoice{})
}

// writeChoice overrides the writer's choice for one argument, by name: a
// value to write in place of the sample, or the argument left out.
type writeChoice struct {
	name  string
	value any
	omit  bool
}

// writeWordWith is [writeWord] with one argument chosen otherwise.
func writeWordWith(t *testing.T, step map[string]any, word string, w Written, args []Arg, condition map[string]any, choice writeChoice) {
	t.Helper()
	spelled := word
	keyed := map[string]any{}
	var value any
	for _, a := range args {
		if choice.name == a.Name && choice.omit {
			continue
		}
		v := argSample(t, args, a, 0, condition)
		if choice.name == a.Name {
			v = choice.value
		}
		if a.Many {
			v = []any{v, argSample(t, args, a, 1, condition)}
		}
		switch a.Goes {
		case PlaceOpen:
			spelled += v.(string)
		case PlaceValue:
			value = v
		case PlaceField:
			keyed[a.Field] = v
		case PlaceImplied: // the word means it; nobody writes it
		default:
			t.Fatalf("%s: an argument placed nowhere: %+v", word, a)
		}
	}
	if w.Field != "" {
		keyed[w.Field] = spelled
	}
	var node any = keyed
	if value != nil {
		node = value
	}
	switch w.In {
	case "on":
		step["on"] = spelled
	case "step":
		step[w.Key] = node
	case "when":
		when, _ := step["when"].(map[string]any)
		if when == nil {
			when = map[string]any{}
			step["when"] = when
		}
		when[w.Key] = node
	case "do":
		do, _ := step["do"].([]any)
		step["do"] = append(do, map[string]any{w.Key: node})
	default:
		t.Fatalf("%s is written in %q, which is no part of a step", word, w.In)
	}
}

// stepOf decodes a written step, lowered as the content compiler lowers it
// (a word's engine spelling, from the table).
func stepOf(t *testing.T, step map[string]any) machine.Transition {
	t.Helper()
	raw, err := json.Marshal(step)
	require.NoError(t, err)
	var tr machine.Transition
	require.NoError(t, json.Unmarshal(raw, &tr), "%s", raw)
	lower(tr.When)
	return tr
}

// lower spells every fact as the engine reads it.
func lower(g *machine.Guard) {
	if g == nil {
		return
	}
	if g.Fact != nil {
		if w, id, ok := SplitConditionWord(string(g.Fact.Name)); ok {
			if inner, _ := ConditionWordOf(string(g.Fact.Name)); inner.engine != "" {
				g.Fact.Name = machine.FactName(inner.engine + id)
			}
			_ = w
		}
	}
	for i := range g.All {
		lower(&g.All[i])
	}
	for i := range g.Any {
		lower(&g.Any[i])
	}
	lower(g.Not)
}

// argsRecipe is a one-state recipe of one step, with the bag.
func argsRecipe(t *testing.T, step machine.Transition) *machine.Recipe {
	r := wordsRecipe(t, step)
	r.Bag = argsBag.Clone()
	return r
}

// beatFor is a beat a word is allowed on.
func beatFor(on []string) string {
	if len(on) == 0 {
		return string(AnswerTime)
	}
	return on[0]
}

// sampleCondition is a condition written from the first table word that
// takes no condition itself: what a `not`, `all` or `any` holds.
func sampleCondition(t *testing.T, table MachineWordsTable) map[string]any {
	t.Helper()
	for _, c := range table.Conditions {
		if !slices.ContainsFunc(c.Args, func(a Arg) bool { return a.Names == SlotCondition }) && len(c.On) == 0 {
			step := map[string]any{}
			writeWord(t, step, c.Word, c.Written, c.Args, nil)
			return step["when"].(map[string]any)
		}
	}
	t.Fatal("no condition to nest")
	return nil
}

// TestMachineWords_EveryWordIsWrittenFromItsArgs: every beat, condition and
// act the published table holds, written by the one generic writer from its
// Written and Args, compiles; and the arguments reach every field the
// compiler reads.
func TestMachineWords_EveryWordIsWrittenFromItsArgs(t *testing.T) {
	table := MachineWords()
	inner := sampleCondition(t, table)
	var steps []machine.Transition
	for _, b := range table.Beats {
		step := map[string]any{}
		writeWord(t, step, b.Word, b.Written, b.Args, inner)
		tr := stepOf(t, step)
		require.NoError(t, ValidateMachine(argsRecipe(t, tr)), "the beat %s, written as %v", b.Word, step)
	}
	for _, c := range table.Conditions {
		step := map[string]any{"on": beatFor(c.On)}
		writeWord(t, step, c.Word, c.Written, c.Args, inner)
		tr := stepOf(t, step)
		require.NoError(t, ValidateMachine(argsRecipe(t, tr)), "the condition %s, written as %v", c.Word, step)
		steps = append(steps, tr)
	}
	for _, a := range table.Acts {
		step := map[string]any{"on": beatFor(a.On)}
		writeWord(t, step, a.Word, a.Written, a.Args, inner)
		tr := stepOf(t, step)
		_, err := CheckBehaviors(&CheckBehaviorsInput{Behaviors: *fixtureBehavior(*argsRecipe(t, tr), holds())})
		require.NoError(t, err, "the act %s, written as %v", a.Word, step)
		steps = append(steps, tr)
	}

	// EVERY FIELD THE COMPILER READS IS REACHED by some word's arguments: a
	// new field on a step, an act or a guard leaf fails here until a word
	// takes it.
	var trs, acts, sets, guards, facts, scopes, rounds []reflect.Value
	var walk func(g *machine.Guard)
	walk = func(g *machine.Guard) {
		if g == nil {
			return
		}
		guards = append(guards, reflect.ValueOf(*g))
		if g.Fact != nil {
			facts = append(facts, reflect.ValueOf(*g.Fact))
		}
		if g.Scope != nil {
			scopes = append(scopes, reflect.ValueOf(*g.Scope))
		}
		if g.Rounds != nil {
			rounds = append(rounds, reflect.ValueOf(*g.Rounds))
		}
		for i := range g.All {
			walk(&g.All[i])
		}
		for i := range g.Any {
			walk(&g.Any[i])
		}
		walk(g.Not)
	}
	for _, tr := range steps {
		trs = append(trs, reflect.ValueOf(tr))
		walk(tr.When)
		for _, a := range tr.Do {
			acts = append(acts, reflect.ValueOf(a))
			if a.Set != nil {
				sets = append(sets, reflect.ValueOf(*a.Set))
			}
		}
	}
	covered := func(name string, values []reflect.Value, skip ...string) {
		t.Helper()
		require.NotEmpty(t, values, name)
		typ := values[0].Type()
		for i := 0; i < typ.NumField(); i++ {
			if slices.Contains(skip, typ.Field(i).Name) {
				continue
			}
			hit := slices.ContainsFunc(values, func(v reflect.Value) bool { return !v.Field(i).IsZero() })
			require.True(t, hit, "%s.%s is written by no word's arguments", name, typ.Field(i).Name)
		}
	}
	covered("machine.Transition", trs, "Reason") // a step's reason is free text, said by no word
	covered("machine.Act", acts)
	covered("machine.SetAct", sets)
	covered("machine.Guard", guards)
	covered("machine.FactLeaf", facts)
	covered("machine.ScopeLeaf", scopes)
	covered("machine.RoundsLeaf", rounds)
}

// TestMachineWords_ArgsAreWellFormed: every argument has a name, a place
// and a kind from the closed sets; an open word's first argument fills its
// open part and names what the word's Open says; a condition's value
// argument has the word's kind and closed values; a list is a list.
func TestMachineWords_ArgsAreWellFormed(t *testing.T) {
	table := MachineWords()
	check := func(word string, open OpenSlot, args []Arg) {
		t.Helper()
		for _, a := range args {
			require.Contains(t, []string{"id", "value", "n", "op", "parts", "inner"}, a.Name, word)
			require.Contains(t, []ArgPlace{PlaceOpen, PlaceValue, PlaceField, PlaceImplied}, a.Goes, word)
			require.False(t, a.Goes == PlaceImplied && a.Fixed == "", "%s.%s: an implied argument says its value", word, a.Name)
			require.Equal(t, a.Goes == PlaceField, a.Field != "", "%s.%s: a field is named exactly when it goes in one", word, a.Name)
			require.Contains(t, []string{"string", "int", "bool", "condition", "of"}, a.Kind, word)
			require.Equal(t, a.Kind == "of", a.KindOf != "", "%s.%s", word, a.Name)
			require.False(t, a.Optional && a.Default == "", "%s.%s: an optional argument says what leaving it out means", word, a.Name)
		}
		// AN OPEN WORD'S FIRST ARGUMENT IS ITS OPEN PART: spelled after its
		// colon, or (`flag: lit`, read back as `flag:lit`) its key's value.
		if open != SlotNone {
			require.Equal(t, open, args[0].Names, word)
			require.Contains(t, []ArgPlace{PlaceOpen, PlaceValue}, args[0].Goes, word)
		} else {
			require.False(t, slices.ContainsFunc(args, func(a Arg) bool { return a.Goes == PlaceOpen }), "%s: a whole word has no open part", word)
		}
	}
	for _, b := range table.Beats {
		check(b.Word, b.Open, b.Args)
	}
	for _, c := range table.Conditions {
		check(c.Word, c.Open, c.Args)
		for _, a := range c.Args {
			if a.Name == "value" && len(c.Values) > 0 {
				require.Equal(t, c.Values, a.Values, c.Word)
				require.Equal(t, c.Kind, a.Kind, c.Word)
			}
		}
	}
	for _, a := range table.Acts {
		check(a.Word, SlotNone, a.Args)
	}
	// A LIST IS WRITTEN AS A LIST, and these are the words that take one.
	var many []string
	for _, c := range table.Conditions {
		for _, a := range c.Args {
			if a.Many {
				many = append(many, c.Word+"."+a.Name)
			}
		}
	}
	for _, a := range table.Acts {
		for _, g := range a.Args {
			if g.Many {
				many = append(many, a.Word+"."+g.Name)
			}
		}
	}
	require.Equal(t, []string{"all.parts", "any.parts", "clear.id"}, many)
}

// compiles reports whether one word, written with choice, makes a step the
// engine takes.
func compiles(t *testing.T, kind, word string, w Written, on []string, args []Arg, inner map[string]any, choice writeChoice) error {
	t.Helper()
	step := map[string]any{"on": beatFor(on)}
	writeWordWith(t, step, word, w, args, inner, choice)
	raw, err := json.Marshal(step)
	require.NoError(t, err)
	var tr machine.Transition
	if err := json.Unmarshal(raw, &tr); err != nil {
		return err
	}
	lower(tr.When)
	if tr.On == "" {
		return errors.New("no beat")
	}
	if kind == "act" {
		_, err := CheckBehaviors(&CheckBehaviorsInput{Behaviors: *fixtureBehavior(*argsRecipe(t, tr), holds())})
		return err
	}
	return ValidateMachine(argsRecipe(t, tr))
}

// TestMachineWords_FixedAndRequiredArePinned: the fixed arguments are
// exactly these, and writing any other value for one is refused; no
// argument is optional, and leaving out any written argument is refused.
func TestMachineWords_FixedAndRequiredArePinned(t *testing.T) {
	table := MachineWords()
	inner := sampleCondition(t, table)
	type word struct {
		kind, word string
		w          Written
		on         []string
		args       []Arg
	}
	var words []word
	for _, b := range table.Beats {
		words = append(words, word{"beat", b.Word, b.Written, nil, b.Args})
	}
	for _, c := range table.Conditions {
		words = append(words, word{"condition", c.Word, c.Written, c.On, c.Args})
	}
	for _, a := range table.Acts {
		words = append(words, word{"act", a.Word, a.Written, a.On, a.Args})
	}
	var fixed, optional []string
	for _, w := range words {
		for _, a := range w.args {
			if a.Fixed != "" {
				fixed = append(fixed, w.word+"."+a.Name+"="+a.Fixed)
			}
			if a.Optional {
				optional = append(optional, w.word+"."+a.Name)
			}
			if a.Goes == PlaceImplied {
				continue // written nowhere: nothing to write wrong or leave out
			}
			if a.Fixed != "" {
				other := any("x")
				switch a.Kind {
				case ArgKindBool:
					other = !typed(t, a.Kind, a.Fixed).(bool)
				case ArgKindString:
					if a.Names == SlotOp {
						other = string(machine.OpGte)
					}
				}
				require.Error(t, compiles(t, w.kind, w.word, w.w, w.on, w.args, inner, writeChoice{name: a.Name, value: other}),
					"%s.%s is fixed at %s: writing %v must be refused", w.word, a.Name, a.Fixed, other)
			}
			if a.Optional {
				left := map[string]any{"on": beatFor(w.on)}
				writeWordWith(t, left, w.word, w.w, w.args, inner, writeChoice{name: a.Name, omit: true})
				given := map[string]any{"on": beatFor(w.on)}
				writeWordWith(t, given, w.word, w.w, w.args, inner, writeChoice{name: a.Name, value: typed(t, a.Kind, a.Default)})
				require.Equal(t, stepOf(t, given), stepOf(t, left), "%s.%s left out means %s", w.word, a.Name, a.Default)
			}
			if !a.Optional {
				require.Error(t, compiles(t, w.kind, w.word, w.w, w.on, w.args, inner, writeChoice{name: a.Name, omit: true}),
					"%s.%s is required: leaving it out must be refused", w.word, a.Name)
			}
		}
	}
	require.Equal(t, []string{
		"enemy.op=eq", "fact:.op=eq", "heard.kind.op=eq", "heard.loudness:.op=eq", "heard.claim.op=eq",
		"noted:.op=eq", "noted.anyone:.op=eq", "within.op=lte", "regions.op=lte", "anchor.value=true",
	}, fixed)
	require.Equal(t, []string{"rounds.n"}, optional, "the arguments that may be left out")
}
