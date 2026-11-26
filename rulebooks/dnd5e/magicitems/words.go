// Copyright (C) 2026 Kirk Diggler
// SPDX-License-Identifier: GPL-3.0-or-later

package magicitems

import (
	"slices"

	coreResources "github.com/KirkDiggler/rpg-toolkit/core/resources"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/features"
)

// Place is where in an item an authored word may appear.
type Place string

const (
	// PlaceItem is the item itself.
	PlaceItem Place = "item"
	// PlaceCharges is the item's charges.
	PlaceCharges Place = "charges"
	// PlaceRefills is when the charges come back.
	PlaceRefills Place = "refills"
	// PlaceLetsEntry is one entry of lets: one ability the item lets its
	// wearer use.
	PlaceLetsEntry Place = "lets entry"
	// PlaceWith is the values an ability is used with. Its words belong to
	// the ability chosen and are on that `use` value's With; the table itself
	// has no rows of this place.
	PlaceWith Place = "with"
	// PlaceCost is what one use costs the item.
	PlaceCost Place = "cost"
)

// Shape is what an authored word takes as its value. A shape that means what a
// status shape means is spelled the same, so a studio control built for one
// serves the other.
type Shape string

const (
	// ShapeOneOf takes exactly one of the word's Values.
	ShapeOneOf Shape = "one of"
	// ShapeListOf takes a non-empty list of the word's Values, none repeated.
	ShapeListOf Shape = "list of"
	// ShapeWholeNumber takes a whole number within the word's Bounds.
	ShapeWholeNumber Shape = "whole number"
	// ShapeText takes non-empty text.
	ShapeText Shape = "text"
	// ShapeTrueOrFalse takes true or false.
	ShapeTrueOrFalse Shape = "true or false"
	// ShapeList takes a non-empty list of entries, each written with the
	// words of the word's Holds place.
	ShapeList Shape = "list"
	// ShapeEntries takes an object of words of the word's Holds place.
	ShapeEntries Shape = "entries"
)

// Bounds is the range a number word accepts, both ends included.
type Bounds struct {
	Min int
	// Max is the most it takes. It means nothing when NoMax is true; a zero
	// Max never means "no limit".
	Max   int
	NoMax bool
}

// Value is one closed value of a word.
type Value struct {
	// Word is what the author writes.
	Word string
	// Label is how a studio reads the word with this value.
	Label string
	// With is, for a value of `use`, the words its `with` takes, in order;
	// nil for every other value.
	With []Word
}

// Word is one word an author may write.
type Word struct {
	// Word is what the author writes.
	Word string
	// Place is where it may appear.
	Place Place
	// Shape is what it takes as its value.
	Shape Shape
	// Values are the words it accepts, for a closed shape; nil otherwise.
	Values []Value
	// Holds is, for a list or entries word, the place whose words write each
	// entry; empty otherwise.
	Holds Place
	// Required is whether the item must write the word.
	Required bool
	// Bounds is the range a number word accepts; nil for any other shape.
	Bounds *Bounds
	// Label is how a studio reads it to a person, lower-case first, with
	// {value} where its value goes.
	Label string
}

// WordsInput asks for the table of words. It has no fields today.
type WordsInput struct{}

// WordsOutput is the table of words in the table's order: the item's words,
// then charges, refills, a lets entry and cost, each place's words in the
// order the table lists them.
type WordsOutput struct {
	Words []Word
}

// Words returns every word an author may write for an item, for a studio to
// draw from. It is the same table [Compile] reads. Each call returns its own
// copy.
//
// The `cost` word `charges` can be at most the item's own `charges.max`. A
// table cannot say a limit that depends on another word, so it says there is no
// upper limit and [Compile] keeps refusing a cost the item cannot pay.
func Words(_ *WordsInput) *WordsOutput {
	out := &WordsOutput{}
	for _, e := range table {
		out.Words = append(out.Words, e.public())
	}
	return out
}

// entry is one row of the table.
type entry struct {
	place    Place
	name     string
	shape    Shape
	label    string
	required bool
	values   []value
	holds    Place
	bounds   *Bounds
}

// value is one closed value and what it maps to. The mapping is never shown to
// an author: a slot word to its sheet slots, a rest word to its reset type.
type value struct {
	word  string
	label string
	slots []string
	reset coreResources.ResetType
	with  []Word
}

func (e entry) public() Word {
	w := Word{
		Word: e.name, Place: e.place, Shape: e.shape, Holds: e.holds,
		Required: e.required, Label: e.label,
	}
	if e.bounds != nil {
		b := *e.bounds
		w.Bounds = &b
	}
	for _, v := range e.values {
		w.Values = append(w.Values, Value{Word: v.word, Label: v.label, With: cloneWords(v.with)})
	}
	return w
}

func cloneWords(in []Word) []Word {
	if in == nil {
		return nil
	}
	out := make([]Word, len(in))
	for i, w := range in {
		out[i] = w
		if w.Bounds != nil {
			b := *w.Bounds
			out[i].Bounds = &b
		}
		out[i].Values = slices.Clone(w.Values)
	}
	return out
}

// table is every word an author may write for an item. Every word, its place,
// its label and what it maps to lives here and only here. The order is the
// order a refusal offers the words in.
var table = []entry{
	{place: PlaceItem, name: wordName, shape: ShapeText, required: true, label: "the name players see"},
	{place: PlaceItem, name: wordSlot, shape: ShapeOneOf, required: true, label: "where it is worn", values: []value{
		{word: "cloak", label: "worn as a cloak", slots: []string{"cloak"}},
		{word: "amulet", label: "worn as an amulet", slots: []string{"amulet"}},
		{word: "ring", label: "worn as a ring, on either hand", slots: []string{"ring_left", "ring_right"}},
		{word: "belt", label: "worn as a belt", slots: []string{"belt"}},
		{word: "helm", label: "worn as a helm", slots: []string{"helm"}},
		{word: "boots", label: "worn as boots", slots: []string{"boots"}},
	}},
	{place: PlaceItem, name: wordUnique, shape: ShapeTrueOrFalse, label: "a character owns at most one of it"},
	{place: PlaceItem, name: wordCharges, shape: ShapeEntries, holds: PlaceCharges, label: "the uses the item holds"},
	{place: PlaceItem, name: wordLets, shape: ShapeList, required: true, holds: PlaceLetsEntry, label: "what the item lets its wearer use"},

	{place: PlaceCharges, name: wordMax, shape: ShapeWholeNumber, required: true, bounds: &Bounds{Min: 1, Max: 1000}, label: "holds {value} charges"},
	{place: PlaceCharges, name: wordRefills, shape: ShapeEntries, holds: PlaceRefills, label: "when its charges come back"},

	{place: PlaceRefills, name: wordOn, shape: ShapeListOf, required: true, label: "back to full on {value}", values: []value{
		{word: "short rest", label: "a short rest", reset: coreResources.ResetShortRest},
		{word: "long rest", label: "a long rest", reset: coreResources.ResetLongRest},
	}},

	{place: PlaceLetsEntry, name: wordUse, shape: ShapeOneOf, required: true, label: "lets its wearer use {value}", values: abilityValues()},
	{place: PlaceLetsEntry, name: wordWith, shape: ShapeEntries, holds: PlaceWith, label: "the values the ability is used with"},
	{place: PlaceLetsEntry, name: wordCost, shape: ShapeEntries, holds: PlaceCost, label: "what one use costs the item"},

	{place: PlaceCost, name: wordCharges, shape: ShapeWholeNumber, required: true, bounds: &Bounds{Min: 1, NoMax: true}, label: "costs {value} of the item's charges"},
}

// abilityValues is every named ability as a value of `use`, each with the
// words its `with` takes, built from the rulebook's own list.
func abilityValues() []value {
	var out []value
	for _, ability := range features.AbilityWords(&features.AbilityWordsInput{}).Words {
		v := value{word: ability.Name, label: ability.Label}
		for _, av := range ability.Values {
			v.with = append(v.with, Word{
				Word: av.Word, Place: PlaceWith, Shape: ShapeWholeNumber, Required: av.Required,
				Bounds: &Bounds{Min: av.Min, Max: av.Max, NoMax: av.NoMax}, Label: av.Label,
			})
		}
		out = append(out, v)
	}
	return out
}

// wordsAt lists the words of one place, in table order.
func wordsAt(place Place) []string {
	var out []string
	for _, e := range table {
		if e.place == place {
			out = append(out, e.name)
		}
	}
	return out
}

// find returns the table row for a word of a place.
func find(place Place, word string) entry {
	for _, e := range table {
		if e.place == place && e.name == word {
			return e
		}
	}
	panic("magicitems: no table row for " + string(place) + " " + word)
}

// valueWords lists the closed values of a word, in table order.
func valueWords(place Place, word string) []string {
	var out []string
	for _, v := range find(place, word).values {
		out = append(out, v.word)
	}
	return out
}

// valueOf returns the closed value of a word an author wrote.
func valueOf(place Place, word, written string) (value, bool) {
	for _, v := range find(place, word).values {
		if v.word == written {
			return v, true
		}
	}
	return value{}, false
}
