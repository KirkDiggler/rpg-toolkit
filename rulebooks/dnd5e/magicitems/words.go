// Copyright (C) 2026 Kirk Diggler
// SPDX-License-Identifier: GPL-3.0-or-later

package magicitems

import (
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/kits"
)

// The words are the kit's types: a studio that draws an item's words draws the
// kit's the same way, and a caller that names magicitems.Word names the type
// the kit's own table holds.
type (
	// Place is where in an item an authored word may appear.
	Place = kits.Place
	// Shape is what an authored word takes as its value.
	Shape = kits.Shape
	// Bounds is the range a number word accepts, both ends included.
	Bounds = kits.Bounds
	// Value is one closed value of a word.
	Value = kits.Value
	// Word is one word an author may write.
	Word = kits.Word
	// WordsInput asks for the table of words. It has no fields today.
	WordsInput = kits.WordsInput
	// WordsOutput is the table of words in the table's order: the item's
	// words, then charges, refills, a lets entry and cost, each place's words
	// in the order the table lists them.
	WordsOutput = kits.WordsOutput
)

const (
	// PlaceItem is the item itself.
	PlaceItem Place = "item"
	// PlaceCharges is the item's charges.
	PlaceCharges = kits.PlaceCharges
	// PlaceRefills is when the charges come back.
	PlaceRefills = kits.PlaceRefills
	// PlaceLetsEntry is one entry of lets: one ability the item lets its
	// wearer use.
	PlaceLetsEntry = kits.PlaceLetsEntry
	// PlaceWith is the values an ability is used with.
	PlaceWith = kits.PlaceWith
	// PlaceCost is what one use costs the item.
	PlaceCost = kits.PlaceCost
)

const (
	// ShapeOneOf takes exactly one of the word's Values.
	ShapeOneOf = kits.ShapeOneOf
	// ShapeListOf takes a non-empty list of the word's Values, none repeated.
	ShapeListOf = kits.ShapeListOf
	// ShapeWholeNumber takes a whole number within the word's Bounds.
	ShapeWholeNumber = kits.ShapeWholeNumber
	// ShapeText takes non-empty text.
	ShapeText = kits.ShapeText
	// ShapeTrueOrFalse takes true or false.
	ShapeTrueOrFalse = kits.ShapeTrueOrFalse
	// ShapeList takes a non-empty list of entries, each written with the
	// words of the word's Holds place.
	ShapeList = kits.ShapeList
	// ShapeEntries takes an object of words of the word's Holds place.
	ShapeEntries = kits.ShapeEntries
)

// Words returns every word an author may write for an item, for a studio to
// draw from. It is the item's own words followed by the kit's ([kits.Words]):
// the same table [Compile] reads. Each call returns its own copy.
//
// The `cost` word `charges` can be at most the item's own `charges.max`. A
// table cannot say a limit that depends on another word, so it says there is no
// upper limit and [Compile] keeps refusing a cost the item cannot pay.
func Words(in *WordsInput) *WordsOutput {
	out := &WordsOutput{}
	for _, e := range table {
		out.Words = append(out.Words, e.public())
	}
	out.Words = append(out.Words, kits.Words(in).Words...)
	return out
}

// entry is one row of the item's own table.
type entry struct {
	place    Place
	name     string
	shape    Shape
	label    string
	required bool
	values   []value
	holds    Place
}

// value is one closed value and what it maps to. The mapping is never shown to
// an author: a slot word to its sheet slots.
type value struct {
	word  string
	label string
	slots []string
}

func (e entry) public() Word {
	w := Word{
		Word: e.name, Place: e.place, Shape: e.shape, Holds: e.holds,
		Required: e.required, Label: e.label,
	}
	for _, v := range e.values {
		w.Values = append(w.Values, Value{Word: v.word, Label: v.label})
	}
	return w
}

// table is every word an author may write for the item itself. The words of
// its kit are the kit's own table ([kits.Words]). The order is the order a
// refusal offers the words in.
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
}

// wordsAt lists the words of the item itself, in table order.
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
