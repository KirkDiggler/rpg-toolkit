// Copyright (C) 2026 Kirk Diggler
// SPDX-License-Identifier: GPL-3.0-or-later

package encounter

// machinewords_internal_test.go holds the table of machine words
// (rpg-project#997) to the compiler, both ways: every word compiles where
// the table allows it and is refused where it does not, and every word the
// compiler accepts is in the table.

import (
	"reflect"
	"slices"
	"strings"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/KirkDiggler/rpg-toolkit/core"
	"github.com/KirkDiggler/rpg-toolkit/mind/machine"
)

// wordsRecipe is a one-state recipe of one step.
func wordsRecipe(t *testing.T, step machine.Transition) *machine.Recipe {
	t.Helper()
	ref, err := core.ParseString("test:machines:words")
	require.NoError(t, err)
	return &machine.Recipe{Ref: *ref, Initial: "s", States: map[string]machine.State{"s": {Transitions: []machine.Transition{step}}}}
}

// everyBeat is one authored beat per table word: each whole word, and one
// member of each open family.
func everyBeat() []string {
	var out []string
	for _, b := range MachineWords().Beats {
		switch b.Open {
		case SlotNone:
			out = append(out, b.Word)
		case SlotDeed:
			out = append(out, b.Word+WhenDeeds[0])
		case SlotClock:
			out = append(out, b.Word+"bell:gong")
		}
	}
	return out
}

func leafOf(name string, value any) *machine.Guard {
	return &machine.Guard{Fact: &machine.FactLeaf{Name: machine.FactName(name), Op: machine.OpEq, Value: value}}
}

func TestMachineWords_EveryBeatCompilesAndIsValid(t *testing.T) {
	for _, on := range everyBeat() {
		require.True(t, ValidMachineBeat(on), on)
		require.NoError(t, ValidateMachine(wordsRecipe(t, machine.Transition{On: machine.Beat(on)})), on)
	}
	require.False(t, ValidMachineBeat("smell"))
	require.False(t, ValidMachineBeat(deedPrefix+"nonsense"), "a deed word the rulebook does not name")
	require.Error(t, ValidateMachine(wordsRecipe(t, machine.Transition{On: "smell"})))
}

// TestMachineWords_ScopeIsAllowedOnlyOnScopedBeats: `within` compiles on a
// beat the table marks Scoped and is refused on every other.
func TestMachineWords_ScopeIsAllowedOnlyOnScopedBeats(t *testing.T) {
	for _, on := range everyBeat() {
		b, _ := BeatWordOf(on)
		err := ValidateMachine(wordsRecipe(t, machine.Transition{On: machine.Beat(on), When: &machine.Guard{Scope: &machine.ScopeLeaf{Within: 2}}}))
		require.Equal(t, b.Scoped, err == nil, "%s: scoped %v, refused %v", on, b.Scoped, err)
		require.Equal(t, b.Scoped, MachineBeatScoped(on), on)
	}
}

// TestMachineWords_ConditionsAreReadWhereTheTableSays: each fact condition
// compiles on every beat its On allows and is refused on every other.
func TestMachineWords_ConditionsAreReadWhereTheTableSays(t *testing.T) {
	sample := map[string]*machine.Guard{
		"enemy":           leafOf("enemy", string(EnemySeen)),
		"fact:":           leafOf("fact:gate-cowed", true),
		"heard.kind":      leafOf("heard.kind", SoundVoice),
		"heard.loudness:": leafOf("heard.loudness:shout", true),
		"heard.claim":     leafOf("heard.claim", ClaimFailed),
		"noted:":          leafOf("noted:admitted", true),
		"noted.anyone:":   leafOf("topic:admitted", true), // as the engine reads it once lowered
	}
	for _, c := range MachineWords().Conditions {
		if !c.Fact {
			continue
		}
		g, ok := sample[c.Word]
		require.True(t, ok, "a sample for every fact word in the table: %s", c.Word)
		for _, on := range everyBeat() {
			err := ValidateMachine(wordsRecipe(t, machine.Transition{On: machine.Beat(on), When: g}))
			require.Equal(t, beatAllowed(c.allowedOn(), machine.Beat(on)), err == nil, "%s on %s: %v", c.Word, on, err)
		}
	}
}

// TestMachineWords_AClosedValueIsRefusedOutsideItsSet: `heard.claim` asks
// each of its values and refuses one outside them.
func TestMachineWords_AClosedValueIsRefusedOutsideItsSet(t *testing.T) {
	claim, _ := ConditionWordOf("heard.claim")
	for _, v := range claim.Values {
		require.NoError(t, ValidateMachine(wordsRecipe(t, machine.Transition{On: beatHeard, When: leafOf("heard.claim", v)})), v)
	}
	err := ValidateMachine(wordsRecipe(t, machine.Transition{On: beatHeard, When: leafOf("heard.claim", "maybe")}))
	require.ErrorContains(t, err, "`heard.claim` is held or failed, and this names \"maybe\"")
}

// TestMachineWords_PickIsAllowedOnlyOnPickingBeats: a step picks on a beat
// the table allows it, and is refused on every other, at its own step.
func TestMachineWords_PickIsAllowedOnlyOnPickingBeats(t *testing.T) {
	pick, _ := ActWordOf("pick")
	for _, on := range everyBeat() {
		r := wordsRecipe(t, machine.Transition{On: machine.Beat(on), Do: []machine.Act{{Pick: string(AnswerTime)}}})
		_, err := CheckBehaviors(&CheckBehaviorsInput{Behaviors: *fixtureBehavior(*r, holds())})
		allowed := beatAllowed(pick.allowedOn(), machine.Beat(on))
		require.Equal(t, allowed, err == nil, "%s: %v", on, err)
		if !allowed {
			refused := StepRefusals(err)
			require.Len(t, refused, 1, on)
			require.Equal(t, StepRefusal{State: "s", Transition: 0, Leaf: []machine.Segment{{Name: "do", Index: 0, Indexed: true}},
				Message: "selection is only authorized on time, turn, barred or approached, not \"" + on + "\""}, *refused[0])
		}
	}
}

// TestMachineWords_EveryAcceptedWordIsInTheTable: the vocabulary for a
// site with one fact, one topic and one clock holds only table words, and
// every act and guard field the machine has is a table word — so a new beat,
// fact, act or leaf the compiler accepts fails here until the table has it.
func TestMachineWords_EveryAcceptedWordIsInTheTable(t *testing.T) {
	out, err := BehaviorVocabulary(&BehaviorVocabularyInput{Facts: []FactID{"gate-cowed"}, Clocks: []string{"bell:gong"}, Topics: []TopicID{"admitted"}})
	require.NoError(t, err)
	for beat := range out.Vocabulary.Beats {
		_, ok := BeatWordOf(string(beat))
		require.True(t, ok, "beat %q is in the table", beat)
	}
	for name, spec := range out.Vocabulary.Facts {
		_, ok := ConditionWordOf(string(name))
		require.True(t, ok, "fact %q is in the table", name)
		require.NotNil(t, spec.Read, "fact %q has a reader", name)
	}
	jsonName := func(f reflect.StructField) string { return strings.Split(f.Tag.Get("json"), ",")[0] }
	actType := reflect.TypeOf(machine.Act{})
	for i := 0; i < actType.NumField(); i++ {
		_, ok := ActWordOf(jsonName(actType.Field(i)))
		require.True(t, ok, "act %q is in the table", jsonName(actType.Field(i)))
	}
	_, ok := ActWordOf("to")
	require.True(t, ok, "the move is in the table")
	guardType := reflect.TypeOf(machine.Guard{})
	for i := 0; i < guardType.NumField(); i++ {
		name := jsonName(guardType.Field(i))
		switch name {
		case "fact": // its words are the fact conditions, checked above
			continue
		case "scope":
			for _, leaf := range []string{"within", "regions"} {
				_, ok := ConditionWordOf(leaf)
				require.True(t, ok, "scope's %q is in the table", leaf)
			}
			scopeType := reflect.TypeOf(machine.ScopeLeaf{})
			require.Equal(t, 2, scopeType.NumField(), "a new scope leaf needs its table word")
			continue
		}
		_, ok := ConditionWordOf(name)
		require.True(t, ok, "guard leaf %q is in the table", name)
	}
}

// TestMachineWords_TheTableIsWhatTheVocabularyCompiles: every whole beat and
// fact of the table is in the machine's vocabulary with the timing, scope
// and kind the table says, and MachineBeats is the table's whole beats in
// order.
func TestMachineWords_TheTableIsWhatTheVocabularyCompiles(t *testing.T) {
	v := machineVocabulary()
	var whole []string
	for _, b := range MachineWords().Beats {
		if b.Open != SlotNone {
			continue
		}
		whole = append(whole, b.Word)
		spec, ok := v.Beats[machine.Beat(b.Word)]
		require.True(t, ok, b.Word)
		require.Equal(t, b.Timed, spec.Timed, b.Word)
		require.Equal(t, b.Scoped, spec.InScope != nil, b.Word)
	}
	require.Equal(t, whole, MachineBeats)
	for _, c := range MachineWords().Conditions {
		if c.Fact && c.Open == SlotNone {
			require.Contains(t, v.Facts, machine.FactName(c.Word))
		}
	}
	require.Equal(t, len(v.Beats), len(whole), "the vocabulary has no beat the table lacks")
}

// TestMachineWords_EveryWordIsSaid: every entry has a Label and a Long, and
// a beat a Short; no label says "holds" or "a note".
func TestMachineWords_EveryWordIsSaid(t *testing.T) {
	check := func(word, label, long string) {
		require.NotEmpty(t, label, word)
		require.NotEmpty(t, long, word)
		require.NotContains(t, label, "hold", word)
		require.NotContains(t, label, "a note", word)
	}
	for _, b := range MachineWords().Beats {
		check(b.Word, b.Label, b.Long)
		require.NotEmpty(t, b.Short, b.Word)
	}
	for _, c := range MachineWords().Conditions {
		check(c.Word, c.Label, c.Long)
	}
	for _, a := range MachineWords().Acts {
		check(a.Word, a.Label, a.Long)
		require.NotEmpty(t, a.Short, a.Word)
	}
}

// TestMachineWords_TheTableIsACopy: a caller's change to the table reaches
// nothing.
func TestMachineWords_TheTableIsACopy(t *testing.T) {
	w := MachineWords()
	w.Beats[0].Word = "changed"
	w.Conditions[4].Values[0] = "changed"
	require.NotEqual(t, "changed", MachineWords().Beats[0].Word)
	require.False(t, slices.Contains(MachineWords().Conditions[4].Values, "changed"))
}

// TestMachineWords_WhereAllowedIsPinned pins the table's where-allowed rules
// as the engine has ruled them, so a change to the table is a decision, not
// a slip: the scoped beats, the naming beats, the picking beats, and the
// beats `heard.claim` and `noted:` are read on.
func TestMachineWords_WhereAllowedIsPinned(t *testing.T) {
	require.Equal(t, []string{"heard", "deed", "seen", "barred", "approached"}, beatsWhere(func(b BeatWord) bool { return b.Scoped }))
	require.Equal(t, []string{"heard", "seen", "barred", "approached"}, beatsWhere(func(b BeatWord) bool { return b.Names }))
	require.Equal(t, []string{"time", "turn", "barred", "approached"}, beatsWhere(func(b BeatWord) bool { return b.Picks }))
	require.Equal(t, []string{"time", "turn"}, beatsWhere(func(b BeatWord) bool { return b.Timed }))
	claim, _ := ConditionWordOf("heard.claim")
	require.Equal(t, []string{"heard"}, claim.allowedOn())
	noted, _ := ConditionWordOf("noted:x")
	require.Equal(t, []string{"heard", "seen", "barred", "approached"}, noted.allowedOn())
	require.Equal(t, []string{"time", "turn", "heard", "deed", "seen", "fact", "topic", "clock:world", "clock:fight", "barred", "approached"}, MachineBeats)
}

// TestMachineWords_PublishesRealBeatsOnly holds the published view to the
// shape a reader gets: every On a list of real beats, expanded from the
// table's families, and no engine spelling.
func TestMachineWords_PublishesRealBeatsOnly(t *testing.T) {
	table := MachineWords()
	realBeats := func(on []string) {
		for _, b := range on {
			bw, ok := BeatWordOf(b)
			require.True(t, ok && bw.Open == SlotNone && bw.Word == b, "%q is not a real beat", b)
		}
	}
	for _, c := range table.Conditions {
		realBeats(c.On)
		require.Empty(t, c.engine, c.Word)
		require.Equal(t, familyNone, c.family, c.Word)
		inner, _ := ConditionWordOf(c.Word + "x")
		if c.Open == SlotNone {
			inner, _ = ConditionWordOf(c.Word)
		}
		require.Equal(t, inner.allowedOn(), c.On, c.Word)
	}
	for _, a := range table.Acts {
		realBeats(a.On)
		inner, _ := ActWordOf(a.Word)
		require.Equal(t, inner.allowedOn(), a.On, a.Word)
	}
	within, _ := ConditionWordOf("within")
	require.Equal(t, []string{"heard", "deed", "seen", "barred", "approached"}, within.allowedOn())
}

// TestMachineWords_AboutIsTheNamesColumn holds a beat's about-whom words to
// its Names column: the read-back says who a `noted:` asks about from them.
func TestMachineWords_AboutIsTheNamesColumn(t *testing.T) {
	for _, b := range machineBeatWords {
		require.Equal(t, b.Names, b.About != "", b.Word)
	}
}

// TestMachineWords_TargetedReactionsArePickingBeats pins the runtime's own
// list of targeted reactions (wakebeats.go react: barred, approached) to the
// table: the beats a step may pick on that the clock does not raise.
func TestMachineWords_TargetedReactionsArePickingBeats(t *testing.T) {
	require.Equal(t, []string{string(beatBarred), string(beatApproached)},
		beatsWhere(func(b BeatWord) bool { return b.Picks && !b.Timed }))
}
