// Copyright (C) 2026 Kirk Diggler
// SPDX-License-Identifier: GPL-3.0-or-later

package magicitems_test

import (
	"encoding/json"
	"fmt"
	"slices"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/magicitems"
)

type unchangedCase struct{ name, block string }

func mustJSON(t *testing.T, v any) string {
	t.Helper()
	raw, err := json.Marshal(v)
	require.NoError(t, err)
	return string(raw)
}

const base = `"name": "N", "slot": "cloak", "lets": [{"use": "second wind", "with": {"level": 1}}]`

var unchangedCases = []unchangedCase{
	{"cloak", cloakBlock},
	{"ring with unique", `{"name": "R", "slot": "ring", "unique": true, "charges": {"max": 1000}, "lets": [{"use": "second wind", "with": {"level": 3}, "cost": {"charges": 1000}}]}`},
	{"unknown item word", `{` + base + `, "colour": "red"}`},
	{"unknown charges word", `{` + base + `, "charges": {"max": 1, "regain": 1}}`},
	{"unknown refills word", `{` + base + `, "charges": {"max": 1, "refills": {"on": ["long rest"], "when": 1}}}`},
	{"unknown lets word", `{"name": "N", "slot": "cloak", "lets": [{"use": "second wind", "with": {"level": 1}, "when": 1}]}`},
	{"unknown cost word", `{"name": "N", "slot": "cloak", "charges": {"max": 2}, "lets": [{"use": "second wind", "with": {"level": 1}, "cost": {"gold": 1}}]}`},
	{"bad slot", `{"name": "N", "slot": "gloves", "lets": [{"use": "second wind", "with": {"level": 1}}]}`},
	{"no slot", `{"name": "N", "lets": [{"use": "second wind", "with": {"level": 1}}]}`},
	{"bad rest", `{` + base + `, "charges": {"max": 1, "refills": {"on": ["dawn"]}}}`},
	{"bad use", `{"name": "N", "slot": "cloak", "lets": [{"use": "fireball"}]}`},
	{"bad unique", `{` + base + `, "unique": "yes"}`},
	{"max zero", `{` + base + `, "charges": {"max": 0}}`},
	{"max over", `{` + base + `, "charges": {"max": 1001}}`},
	{"cost over max", `{"name": "N", "slot": "cloak", "charges": {"max": 2}, "lets": [{"use": "second wind", "with": {"level": 1}, "cost": {"charges": 3}}]}`},
	{"level zero", `{"name": "N", "slot": "cloak", "lets": [{"use": "second wind", "with": {"level": 0}}]}`},
}

type golden struct {
	refused bool
	want    string
}

// unchangedGolden was captured by compiling unchangedCases on origin/main
// (a193b5fb), before the compiler read the table.
var unchangedGolden = map[string]golden{
	"cloak":                {refused: false, want: "{\"ref\":{\"module\":\"site\",\"type\":\"items\",\"id\":\"it\"},\"name\":\"Lieutenant's cloak\",\"slots\":[\"cloak\"],\"pools\":{\"charges\":{\"maximum\":1,\"restore_on\":[\"short_rest\",\"long_rest\"]}},\"grants\":{\"second-wind\":{\"ability\":{\"module\":\"dnd5e\",\"type\":\"features\",\"id\":\"second_wind\"},\"parameters\":{\"level\":1},\"pool\":\"charges\",\"uses\":1}}}"},
	"ring with unique":     {refused: false, want: "{\"ref\":{\"module\":\"site\",\"type\":\"items\",\"id\":\"it\"},\"name\":\"R\",\"slots\":[\"ring_left\",\"ring_right\"],\"unique\":true,\"pools\":{\"charges\":{\"maximum\":1000}},\"grants\":{\"second-wind\":{\"ability\":{\"module\":\"dnd5e\",\"type\":\"features\",\"id\":\"second_wind\"},\"parameters\":{\"level\":3},\"pool\":\"charges\",\"uses\":1000}}}"},
	"unknown item word":    {refused: true, want: "item \"it\": the item: unknown word `colour`; it is one of name, slot, unique, charges, lets"},
	"unknown charges word": {refused: true, want: "item \"it\": `charges`: unknown word `regain`; it is one of max, refills"},
	"unknown refills word": {refused: true, want: "item \"it\": `refills`: unknown word `when`; it is one of on"},
	"unknown lets word":    {refused: true, want: "item \"it\": lets[0]: unknown word `when`; it is one of use, with, cost"},
	"unknown cost word":    {refused: true, want: "item \"it\": lets[0] `cost`: unknown word `gold`; it is one of charges"},
	"bad slot":             {refused: true, want: "item \"it\": `slot` is \"gloves\"; it is one of cloak, amulet, ring, belt, helm, boots"},
	"no slot":              {refused: true, want: "item \"it\": the item has no `slot`; it is one of cloak, amulet, ring, belt, helm, boots"},
	"bad rest":             {refused: true, want: "item \"it\": `refills.on` has \"dawn\"; it is one of short rest, long rest"},
	"bad use":              {refused: true, want: "item \"it\": lets[0]: `use` is \"fireball\"; it is one of second wind"},
	"bad unique":           {refused: true, want: "item \"it\": `unique` is true or false"},
	"max zero":             {refused: true, want: "item \"it\": `charges.max` is 0; it is a whole number from 1 to 1000"},
	"max over":             {refused: true, want: "item \"it\": `charges.max` is 1001; it is a whole number from 1 to 1000"},
	"cost over max":        {refused: true, want: "item \"it\": lets[0] cost.charges is 3; the item holds at most 2"},
	"level zero":           {refused: true, want: "item \"it\": lets[0] `with` for \"second wind\": second wind level 0 must be at least 1"},
}

// Test 5: nothing an author sees changed.
func TestCompileIsByteForByteWhatItWasBeforeTheTable(t *testing.T) {
	require.Len(t, unchangedGolden, len(unchangedCases))
	for _, c := range unchangedCases {
		want, ok := unchangedGolden[c.name]
		require.True(t, ok, c.name)
		out, err := compile(t, "it", c.block)
		if want.refused {
			require.Error(t, err, c.name)
			assert.Equal(t, want.want, err.Error(), c.name)
			continue
		}
		require.NoError(t, err, c.name)
		assert.Equal(t, want.want, mustJSON(t, out.Description), c.name)
	}
}

// The tests below are driven from magicitems.Words(), not from a list typed
// into the test.

func wordsAt(place magicitems.Place) []magicitems.Word {
	var out []magicitems.Word
	for _, w := range magicitems.Words(&magicitems.WordsInput{}).Words {
		if w.Place == place {
			out = append(out, w)
		}
	}
	return out
}

func names(words []magicitems.Word) []string {
	var out []string
	for _, w := range words {
		out = append(out, w.Word)
	}
	return out
}

func find(t *testing.T, place magicitems.Place, word string) magicitems.Word {
	t.Helper()
	for _, w := range wordsAt(place) {
		if w.Word == word {
			return w
		}
	}
	require.FailNow(t, "no word", "%s %s", place, word)
	return magicitems.Word{}
}

// valueFor is the smallest thing an author can write for a word.
func valueFor(t *testing.T, w magicitems.Word) any {
	t.Helper()
	switch w.Shape {
	case magicitems.ShapeText:
		return "N"
	case magicitems.ShapeTrueOrFalse:
		return true
	case magicitems.ShapeOneOf:
		return w.Values[0].Word
	case magicitems.ShapeListOf:
		return []any{w.Values[0].Word}
	case magicitems.ShapeWholeNumber:
		return w.Bounds.Min
	case magicitems.ShapeList:
		return []any{objectFor(t, w.Holds)}
	case magicitems.ShapeEntries:
		return objectFor(t, w.Holds)
	}
	require.FailNow(t, "unknown shape", string(w.Shape))
	return nil
}

// objectFor writes only the required words of a place. A lets entry also
// writes the `with` its first ability needs.
func objectFor(t *testing.T, place magicitems.Place) map[string]any {
	t.Helper()
	obj := map[string]any{}
	for _, w := range wordsAt(place) {
		if !w.Required {
			continue
		}
		obj[w.Word] = valueFor(t, w)
	}
	if place == magicitems.PlaceLetsEntry {
		obj["with"] = withFor(t, obj["use"].(string))
	}
	return obj
}

// withFor writes the required words of an ability's `with`, or nothing.
func withFor(t *testing.T, ability string) map[string]any {
	t.Helper()
	with := map[string]any{}
	for _, v := range find(t, magicitems.PlaceLetsEntry, "use").Values {
		if v.Word != ability {
			continue
		}
		for _, w := range v.With {
			if w.Required {
				with[w.Word] = w.Bounds.Min
			}
		}
	}
	return with
}

// itemAt is the smallest item that has the given place, with change applied to
// the object of that place. The result is JSON.
func itemAt(t *testing.T, place magicitems.Place, change func(obj map[string]any)) string {
	t.Helper()
	top := objectFor(t, magicitems.PlaceItem)
	lets := top["lets"].([]any)[0].(map[string]any)
	var obj map[string]any
	switch place {
	case magicitems.PlaceItem:
		obj = top
	case magicitems.PlaceCharges:
		obj = objectFor(t, magicitems.PlaceCharges)
		top["charges"] = obj
	case magicitems.PlaceRefills:
		charges := objectFor(t, magicitems.PlaceCharges)
		obj = objectFor(t, magicitems.PlaceRefills)
		charges["refills"] = obj
		top["charges"] = charges
	case magicitems.PlaceLetsEntry:
		obj = lets
	case magicitems.PlaceCost:
		top["charges"] = objectFor(t, magicitems.PlaceCharges)
		obj = objectFor(t, magicitems.PlaceCost)
		lets["cost"] = obj
	default:
		require.FailNow(t, "no such place", string(place))
	}
	if change != nil {
		change(obj)
	}
	return mustJSON(t, top)
}

// compilerPlaces are the places whose object the compiler reads. `with` is
// not one: its words belong to the ability chosen.
var compilerPlaces = []magicitems.Place{
	magicitems.PlaceItem, magicitems.PlaceCharges, magicitems.PlaceRefills,
	magicitems.PlaceLetsEntry, magicitems.PlaceCost,
}

// Test 1: the table and the compiler agree, place by place.
func TestAWordNotInTheTableIsRefusedAndTheRefusalOffersTheTable(t *testing.T) {
	for _, place := range compilerPlaces {
		t.Run(string(place), func(t *testing.T) {
			_ = mustCompile(t, "it", itemAt(t, place, nil))

			_, err := compile(t, "it", itemAt(t, place, func(obj map[string]any) { obj["zzz"] = 1 }))
			require.Error(t, err)
			assert.Contains(t, err.Error(), "unknown word `zzz`; it is one of "+strings.Join(names(wordsAt(place)), ", "))
			assert.True(t, strings.HasSuffix(err.Error(), strings.Join(names(wordsAt(place)), ", ")), err.Error())
		})
	}
}

// Test 2: every closed value compiles, and one outside the table does not.
func TestEveryClosedValueCompilesAndOneOutsideTheTableDoesNot(t *testing.T) {
	slot := find(t, magicitems.PlaceItem, "slot")
	require.NotEmpty(t, slot.Values)
	for _, v := range slot.Values {
		d := mustCompile(t, "it", itemAt(t, magicitems.PlaceItem, func(obj map[string]any) { obj["slot"] = v.Word }))
		assert.NotEmpty(t, d.Slots, v.Word)
	}
	_, err := compile(t, "it", itemAt(t, magicitems.PlaceItem, func(obj map[string]any) { obj["slot"] = "gloves" }))
	require.Error(t, err)
	assert.Contains(t, err.Error(), "it is one of "+strings.Join(valueWords(slot), ", "))

	on := find(t, magicitems.PlaceRefills, "on")
	require.NotEmpty(t, on.Values)
	for _, v := range on.Values {
		d := mustCompile(t, "it", itemAt(t, magicitems.PlaceRefills, func(obj map[string]any) { obj["on"] = []any{v.Word} }))
		assert.Len(t, d.Pools["charges"].RestoreOn, 1, v.Word)
	}
	_, err = compile(t, "it", itemAt(t, magicitems.PlaceRefills, func(obj map[string]any) { obj["on"] = []any{"dawn"} }))
	require.Error(t, err)
	assert.Contains(t, err.Error(), "it is one of "+strings.Join(valueWords(on), ", "))

	use := find(t, magicitems.PlaceLetsEntry, "use")
	require.NotEmpty(t, use.Values)
	for _, v := range use.Values {
		d := mustCompile(t, "it", itemAt(t, magicitems.PlaceLetsEntry, func(obj map[string]any) {
			obj["use"] = v.Word
			obj["with"] = withFor(t, v.Word)
		}))
		assert.Len(t, d.Grants, 1, v.Word)
	}
	_, err = compile(t, "it", itemAt(t, magicitems.PlaceLetsEntry, func(obj map[string]any) { obj["use"] = "fireball" }))
	require.Error(t, err)
	assert.Contains(t, err.Error(), "it is one of "+strings.Join(valueWords(use), ", "))
}

func valueWords(w magicitems.Word) []string {
	var out []string
	for _, v := range w.Values {
		out = append(out, v.Word)
	}
	return out
}

// Test 3: bounds are the compiler's.
func TestTheBoundsTheTableReportsAreTheBoundsTheCompilerKeeps(t *testing.T) {
	maxWord := find(t, magicitems.PlaceCharges, "max")
	require.NotNil(t, maxWord.Bounds)
	require.False(t, maxWord.Bounds.NoMax)
	setMax := func(n int) func(obj map[string]any) {
		return func(obj map[string]any) { obj["max"] = n }
	}
	_ = mustCompile(t, "it", itemAt(t, magicitems.PlaceCharges, setMax(maxWord.Bounds.Min)))
	_ = mustCompile(t, "it", itemAt(t, magicitems.PlaceCharges, setMax(maxWord.Bounds.Max)))
	for _, n := range []int{maxWord.Bounds.Min - 1, maxWord.Bounds.Max + 1} {
		_, err := compile(t, "it", itemAt(t, magicitems.PlaceCharges, setMax(n)))
		require.Error(t, err, n)
		assert.Contains(t, err.Error(), fmt.Sprintf("`charges.max` is %d", n))
	}

	for _, v := range find(t, magicitems.PlaceLetsEntry, "use").Values {
		for _, w := range v.With {
			require.NotNil(t, w.Bounds, w.Word)
			setWith := func(n int) func(obj map[string]any) {
				return func(obj map[string]any) {
					obj["use"] = v.Word
					obj["with"] = map[string]any{w.Word: n}
				}
			}
			_ = mustCompile(t, "it", itemAt(t, magicitems.PlaceLetsEntry, setWith(w.Bounds.Min)))
			_, err := compile(t, "it", itemAt(t, magicitems.PlaceLetsEntry, setWith(w.Bounds.Min-1)))
			require.Error(t, err, w.Word)
			if w.Bounds.NoMax {
				_ = mustCompile(t, "it", itemAt(t, magicitems.PlaceLetsEntry, setWith(w.Bounds.Min+1_000_000)))
				continue
			}
			_ = mustCompile(t, "it", itemAt(t, magicitems.PlaceLetsEntry, setWith(w.Bounds.Max)))
			_, err = compile(t, "it", itemAt(t, magicitems.PlaceLetsEntry, setWith(w.Bounds.Max+1)))
			require.Error(t, err, w.Word)
		}
	}

	cost := find(t, magicitems.PlaceCost, "charges")
	require.NotNil(t, cost.Bounds)
	assert.True(t, cost.Bounds.NoMax)
	_, err := compile(t, "it", itemAt(t, magicitems.PlaceCost, func(obj map[string]any) { obj["charges"] = cost.Bounds.Min - 1 }))
	require.Error(t, err)
}

// Test 4: required is the compiler's. A `with` is needed exactly when the
// ability chosen has a required value of its own, so the test asks each
// `use` value's With.
func TestRequiredWordsAreTheOnesTheCompilerNeeds(t *testing.T) {
	for _, place := range compilerPlaces {
		for _, w := range wordsAt(place) {
			if place == magicitems.PlaceLetsEntry && w.Word == "with" {
				continue
			}
			t.Run(string(place)+" "+w.Word, func(t *testing.T) {
				without := itemAt(t, place, func(obj map[string]any) { delete(obj, w.Word) })
				if w.Required {
					_, err := compile(t, "it", without)
					require.Error(t, err, "the smallest item without a required word is refused")
					return
				}
				_ = mustCompile(t, "it", without)
			})
		}
	}
	for _, v := range find(t, magicitems.PlaceLetsEntry, "use").Values {
		needed := false
		for _, w := range v.With {
			needed = needed || w.Required
		}
		_, err := compile(t, "it", itemAt(t, magicitems.PlaceLetsEntry, func(obj map[string]any) {
			obj["use"] = v.Word
			delete(obj, "with")
		}))
		assert.Equal(t, needed, err != nil, "an item using %q without `with` is refused exactly when a value of its `with` is required", v.Word)
	}
}

// Test 7: Words hands out its own copy.
func TestWordsHandsOutItsOwnCopy(t *testing.T) {
	first := magicitems.Words(&magicitems.WordsInput{})
	before := mustJSON(t, first)
	for i := range first.Words {
		w := &first.Words[i]
		w.Word = "changed"
		w.Label = "changed"
		if w.Bounds != nil {
			w.Bounds.Min, w.Bounds.Max = -5, -5
		}
		for j := range w.Values {
			w.Values[j].Word = "changed"
			for k := range w.Values[j].With {
				w.Values[j].With[k].Word = "changed"
				if b := w.Values[j].With[k].Bounds; b != nil {
					b.Min = -5
				}
			}
		}
		if len(w.Values) > 0 {
			w.Values = slices.Delete(w.Values, 0, 1)
		}
	}
	assert.Equal(t, before, mustJSON(t, magicitems.Words(&magicitems.WordsInput{})))

	d := mustCompile(t, "it", itemAt(t, magicitems.PlaceItem, func(obj map[string]any) { obj["slot"] = "ring" }))
	assert.Equal(t, []string{"ring_left", "ring_right"}, d.Slots)
	_, err := compile(t, "it", `{"name": "N", "slot": "cloak", "charges": {"max": 1000}, "lets": [{"use": "second wind", "with": {"level": 1}}]}`)
	require.NoError(t, err)
}

// The table keeps the place order and the labels the studio reads.
func TestWordsAreInTheOrderTheRefusalsOfferThem(t *testing.T) {
	var got []string
	for _, w := range magicitems.Words(&magicitems.WordsInput{}).Words {
		got = append(got, string(w.Place)+"."+w.Word)
	}
	assert.Equal(t, []string{
		"item.name", "item.slot", "item.unique", "item.charges", "item.lets",
		"charges.max", "charges.refills", "refills.on",
		"lets entry.use", "lets entry.with", "lets entry.cost", "cost.charges",
	}, got)
	for _, v := range find(t, magicitems.PlaceLetsEntry, "use").Values {
		require.NotEmpty(t, v.With, v.Word)
		assert.Equal(t, magicitems.PlaceWith, v.With[0].Place)
		assert.Equal(t, magicitems.ShapeWholeNumber, v.With[0].Shape)
	}
	for _, v := range find(t, magicitems.PlaceItem, "slot").Values {
		assert.Nil(t, v.With, v.Word)
	}
}
