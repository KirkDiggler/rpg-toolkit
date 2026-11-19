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

// publishedAllows is how a reader of [MachineWords] reads an On list: a
// whole beat by itself, a member of an open family by the family's word.
// It is written here apart from [beatAllowed] so the published lists are
// checked as a reader meets them.
func publishedAllows(on []string, beat string) bool {
	if len(on) == 0 || slices.Contains(on, beat) {
		return true
	}
	for _, b := range MachineWords().Beats {
		if b.Open != SlotNone && strings.HasPrefix(beat, b.Word) && slices.Contains(on, b.Word) {
			return true
		}
	}
	return false
}

// publishedCondition is a condition's published entry.
func publishedCondition(t *testing.T, word string) ConditionWord {
	t.Helper()
	for _, c := range MachineWords().Conditions {
		if c.Word == word {
			return c
		}
	}
	t.Fatalf("no condition %q", word)
	return ConditionWord{}
}

// TestMachineWords_ScopeIsAllowedOnlyOnScopedBeats: `within` and `regions`
// compile on a beat the table marks Scoped, `deed:<word>` included, and are
// refused on every other; and the published On of each says exactly that.
func TestMachineWords_ScopeIsAllowedOnlyOnScopedBeats(t *testing.T) {
	for _, on := range everyBeat() {
		b, _ := BeatWordOf(on)
		for word, leaf := range map[string]machine.ScopeLeaf{"within": {Within: 2}, "regions": {Regions: 1}} {
			err := ValidateMachine(wordsRecipe(t, machine.Transition{On: machine.Beat(on), When: &machine.Guard{Scope: &leaf}}))
			require.Equal(t, b.Scoped, err == nil, "%s %s: scoped %v, refused %v", word, on, b.Scoped, err)
			require.Equal(t, err == nil, publishedAllows(publishedCondition(t, word).On, on), "%s on %s: the published On disagrees", word, on)
		}
		require.Equal(t, b.Scoped, MachineBeatScoped(on), on)
	}
	require.NoError(t, ValidateMachine(wordsRecipe(t, machine.Transition{On: "deed:attacked",
		When: &machine.Guard{Scope: &machine.ScopeLeaf{Within: 2}}})), "a deed:<word> step with within compiles")
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
			require.Equal(t, publishedAllows(c.On, on), err == nil, "%s on %s: the published On disagrees", c.Word, on)
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
	var pick ActWord
	for _, a := range MachineWords().Acts {
		if a.Word == "pick" {
			pick = a
		}
	}
	for _, on := range everyBeat() {
		r := wordsRecipe(t, machine.Transition{On: machine.Beat(on), Do: []machine.Act{{Pick: string(AnswerTime)}}})
		_, err := CheckBehaviors(&CheckBehaviorsInput{Behaviors: *fixtureBehavior(*r, holds())})
		allowed := beatAllowed(pick.allowedOn(), machine.Beat(on))
		require.Equal(t, allowed, err == nil, "%s: %v", on, err)
		require.Equal(t, publishedAllows(pick.On, on), err == nil, "pick on %s: the published On disagrees", on)
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
// shape a reader gets: every On a list of the table's own beat words (no
// family name like "scoped"), expanded from the table's families, and no
// engine spelling.
func TestMachineWords_PublishesRealBeatsOnly(t *testing.T) {
	table := MachineWords()
	var words []string
	for _, b := range table.Beats {
		words = append(words, b.Word)
	}
	realBeats := func(on []string) {
		for _, b := range on {
			require.Contains(t, words, b, "%q is not a beat of the table", b)
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
	require.Equal(t, []string{"heard", "deed", "seen", "barred", "approached", "deed:"}, within.allowedOn())
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

// TestMachineWords_LabelsArePinned pins every label as the operator approved
// it (rpg-project#997), so a change to one is a decision: per word, its
// Label, then (for a beat) LabelWith or (for a condition) LabelNot, then
// each value's own label.
func TestMachineWords_LabelsArePinned(t *testing.T) {
	got := map[string][]string{}
	for _, b := range MachineWords().Beats {
		got["beat "+b.Word] = []string{b.Label, b.LabelWith}
	}
	for _, c := range MachineWords().Conditions {
		row := []string{c.Label, c.LabelNot}
		for _, v := range c.Values {
			row = append(row, v+"="+c.ValueLabels[v])
		}
		require.Len(t, c.ValueLabels, len(c.Values), "a label for each closed value of %s, and no other", c.Word)
		got["cond "+c.Word] = row
	}
	for _, a := range MachineWords().Acts {
		got["act "+a.Word] = []string{a.Label}
	}
	require.Equal(t, map[string][]string{
		"beat time":        {"time passes", ""},
		"beat turn":        {"its turn in a fight", ""},
		"beat heard":       {"hears something", ""},
		"beat deed":        {"a deed is done", ""},
		"beat seen":        {"sees someone opposed", ""},
		"beat fact":        {"learns a fact", "learns {id}"},
		"beat topic":       {"notes a topic about someone", "notes {id} about someone"},
		"beat clock:world": {"a round of world time", ""},
		"beat clock:fight": {"a round of its side's fight", ""},
		"beat barred":      {"someone walks into it", ""},
		"beat approached":  {"hails someone", ""},
		"beat deed:":       {"{id} is done", "{id} is done"},
		"beat clock:":      {"a round of {id}", "a round of {id}"},
		"cond enemy": {"an enemy is {value}", "",
			"seen=sees an enemy", "remembered=remembers an enemy", "none=knows of no enemy"},
		"cond fact:": {"has learned {id}", "has not learned {id}"},
		"cond heard.kind": {"it was a {value}", "",
			"boom=it was a boom", "crash=it was a crash", "clang=it was a clang", "voice=it was a voice"},
		"cond heard.loudness:": {"it was at least a {id}", ""},
		"cond heard.claim": {"it was a claim that {value}", "",
			"held=it was a claim that held", "failed=it was a claim that failed"},
		"cond noted:":        {"has noted {id} about them", "has not noted {id} about them"},
		"cond noted.anyone:": {"has noted {id} about anyone", "has not noted {id} about anyone"},
		"cond flag":          {"remembers {id}", ""},
		"cond within":        {"within {n} cells", ""},
		"cond regions":       {"within {n} rooms", ""},
		"cond rounds":        {"{op} {n} rounds since it marked the moment", ""},
		"cond all":           {"all of these", ""},
		"cond any":           {"any of these", ""},
		"cond not":           {"not this", ""},
		"act to":             {"changes to {id}"},
		"act pick":           {"acts from the {id} list"},
		"act wake_in":        {"waits {n} rounds"},
		"act set":            {"remembers {id}"},
		"act clear":          {"forgets {id}"},
		"act anchor":         {"marks the moment"},
	}, got)
}

// TestMachineWords_LabelsKeepTheRules checks the label rules mechanically:
// lower case first, the creature is "it", never "holds", never "a note".
func TestMachineWords_LabelsKeepTheRules(t *testing.T) {
	var labels []string
	for _, b := range MachineWords().Beats {
		labels = append(labels, b.Label, b.LabelWith)
	}
	for _, c := range MachineWords().Conditions {
		labels = append(labels, c.Label, c.LabelNot)
		for _, l := range c.ValueLabels {
			labels = append(labels, l)
		}
	}
	for _, a := range MachineWords().Acts {
		labels = append(labels, a.Label)
	}
	for _, l := range labels {
		if l == "" {
			continue
		}
		require.Equal(t, strings.ToLower(l[:1]), l[:1], "%q starts lower case", l)
		padded := " " + l + " "
		for _, banned := range []string{" he ", " him ", " his ", "holds", "a note"} {
			require.NotContains(t, padded, banned, l)
		}
	}
}
