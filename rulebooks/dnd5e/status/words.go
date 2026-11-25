// Copyright (C) 2026 Kirk Diggler
// SPDX-License-Identifier: GPL-3.0-or-later

package status

import (
	"slices"
	"sort"
	"strings"

	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/abilities"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/damage"
)

// Place is where in a status an authored word may appear.
type Place string

const (
	// PlaceStatus is the status itself: name, changes, maintenance and
	// when_given; its other top-level word, again, has a place of its own.
	PlaceStatus Place = "status"
	// PlaceChange is one entry of changes: what the status does.
	PlaceChange Place = "change"
	// PlaceWhen is a change's when: what narrows it.
	PlaceWhen Place = "when"
	// PlaceMaintenance is one rule of maintenance: what ends the status.
	PlaceMaintenance Place = "maintenance rule"
	// PlaceAgain is again, a top-level word of the status, whose closed values
	// say what a second giving does.
	PlaceAgain Place = "again"
	// PlaceWhenGiven is one entry of when_given: something done once, when the
	// status is first given, not while it is held.
	PlaceWhenGiven Place = "when given"
)

// Shape is what an authored word takes as its value.
type Shape string

const (
	// ShapeOneOf takes exactly one of the word's Values.
	ShapeOneOf Shape = "one of"
	// ShapeListOf takes a non-empty list of the word's Values, none repeated.
	ShapeListOf Shape = "list of"
	// ShapeWholeNumber takes a whole number other than zero, signed, from
	// -1000 to 1000.
	ShapeWholeNumber Shape = "whole number"
	// ShapePositiveWholeNumber takes a whole number from 1 to 1000.
	ShapePositiveWholeNumber Shape = "positive whole number"
	// ShapeText takes non-empty text.
	ShapeText Shape = "text"
	// ShapeList takes a list of entries. Each entry holds exactly one word of
	// the word's Holds place, plus any word marked Narrows beside it.
	ShapeList Shape = "list"
	// ShapeEntries takes an object of one or more words of the word's Holds
	// place, each once.
	ShapeEntries Shape = "entries"
)

// Word is one word an author may write.
type Word struct {
	// Word is what the author writes.
	Word string
	// Place is where it may appear.
	Place Place
	// WrittenIn is the place whose object the word is written in. It equals
	// Place for every word except again, which is a word of its own place but
	// is written on the status itself.
	WrittenIn Place
	// Shape is what it takes as its value.
	Shape Shape
	// Values are the words it accepts, for a closed shape; nil otherwise.
	Values []Value
	// Holds is, for a list or entries word, the place whose words write each
	// entry; empty otherwise.
	Holds Place
	// Required is whether the status must write the word.
	Required bool
	// Narrows is whether the word narrows the change it sits beside rather
	// than being a change itself.
	Narrows bool
	// When is the when words a change word may use, when that does not depend
	// on its value; nil for a word whose values say it (see Value.When) and
	// for words that take none.
	When []string
	// Bounds is the range a number word accepts; nil for any other shape.
	Bounds *Bounds
	// Label is how a studio reads it to a person, lower-case first, with
	// {value} where its value goes.
	Label string
}

// Value is one closed value of a word.
type Value struct {
	// Word is what the author writes.
	Word string
	// Label is how a studio reads the word with this value.
	Label string
	// When is, for an advantage value, the when words it may use; nil
	// otherwise.
	When []string
}

// Bounds is the range a number word accepts, both ends included.
type Bounds struct {
	Min, Max int
	// ZeroAllowed is whether zero is accepted inside the range.
	ZeroAllowed bool
}

// WordsInput asks for the table of words. It has no fields today.
type WordsInput struct{}

// WordsOutput is the table of words, in the table's order: the places in the
// order status, again, change, when, maintenance rule, when given, and each
// place's words in the order the table lists them.
type WordsOutput struct {
	Words []Word
}

// Words returns every word an author may write in a status, for a studio to
// draw from. It is the same table [Compile] reads.
func Words(_ *WordsInput) *WordsOutput {
	out := &WordsOutput{}
	for _, entry := range table {
		word := Word{
			Word: entry.word, Place: entry.place, WrittenIn: entry.writtenIn(), Shape: entry.shape, Label: entry.label,
			Holds: entry.holds, Required: entry.required, Narrows: entry.word == wordWhen && entry.place == PlaceChange,
		}
		if entry.place == PlaceChange {
			word.When = slices.Clone(changeWhen[entry.word])
		}
		if b, ok := numberBounds[entry.shape]; ok {
			word.Bounds = &Bounds{Min: b.min, Max: b.max, ZeroAllowed: b.zero}
		}
		for _, v := range entry.values {
			word.Values = append(word.Values, Value{Word: v.word, Label: v.label, When: slices.Clone(v.when)})
		}
		out.Words = append(out.Words, word)
	}
	return out
}

// The authored words. Every word, its place, its label and what it maps to
// lives here and only here.
const (
	wordName        = "name"
	wordChanges     = "changes"
	wordMaintenance = "maintenance"
	wordAgain       = "again"
	wordWhenGiven   = "when_given"

	wordAdvantage  = "advantage"
	wordResistance = "resistance"
	wordDamage     = "damage"
	wordMaxHP      = "max_hit_points"
	wordInitiative = "initiative"
	wordCannot     = "cannot"

	wordHeal = "heal"

	wordWhen = "when"

	wordAbility = "ability"
	wordAttack  = "attack"

	wordEndsOn = "ends_on"
)

// entry is one row of the table: a word, where it goes, what it takes, and
// what each value maps to in the engine's definition library.
type entry struct {
	place Place
	// in is the place whose object the word is written in, when that is not
	// its own place (again, written on the status).
	in     Place
	word   string
	shape  Shape
	label  string
	values []value
	// holds is the place whose words write each entry of a list or entries.
	holds Place
	// required is whether the status must write the word.
	required bool
}

// writtenIn is the place whose object the word is written in.
func (e entry) writtenIn() Place {
	if e.in != "" {
		return e.in
	}
	return e.place
}

// value is one closed value and its engine mapping. For an advantage it is the
// part params and the when words it can use; for a when word, the fact value;
// for a maintenance rule, the beat and the reason it ends with.
type value struct {
	word   string
	label  string
	to     string
	reason string
	when   []string
}

var table = []entry{
	{place: PlaceStatus, word: wordName, shape: ShapeText, label: "the name players see", required: true},
	{place: PlaceStatus, word: wordChanges, shape: ShapeList, holds: PlaceChange, required: true, label: "what the status changes"},
	{place: PlaceStatus, word: wordMaintenance, shape: ShapeList, holds: PlaceMaintenance, required: true, label: "what ends the status; any one rule ends it"},
	{place: PlaceStatus, word: wordWhenGiven, shape: ShapeList, holds: PlaceWhenGiven, label: "what happens once, when the status is first given"},
	{place: PlaceAgain, in: PlaceStatus, required: true, word: wordAgain, shape: ShapeOneOf, label: "what a second giving does", values: []value{
		{word: "ignored", label: "a second giving is ignored"},
	}},

	{place: PlaceChange, word: wordAdvantage, shape: ShapeOneOf, label: "advantage on {value}", values: []value{
		{word: "checks", label: "advantage on checks", to: `{"on":"ability_check"}`, when: []string{wordAbility}},
		{word: "saves", label: "advantage on saving throws", to: `{"on":"saving_throw"}`, when: []string{wordAbility}},
		{word: "attacks", label: "advantage on attacks", to: `{"on":"attack","side":"by_holder"}`, when: []string{wordAbility, wordAttack}},
		{word: "attacks against you", label: "attacks against the holder have advantage", to: `{"on":"attack","side":"against_holder"}`, when: []string{wordAbility, wordAttack}},
	}},
	{place: PlaceChange, word: wordResistance, shape: ShapeListOf, label: "resistance to {value} damage", values: damageValues()},
	{place: PlaceChange, word: wordDamage, shape: ShapeWholeNumber, label: "{value} to the damage the holder deals"},
	{place: PlaceChange, word: wordMaxHP, shape: ShapePositiveWholeNumber, label: "raises the holder's hit point maximum by {value}"},
	{place: PlaceChange, word: wordInitiative, shape: ShapePositiveWholeNumber, label: "adds {value} to the holder's initiative"},
	{place: PlaceChange, word: wordCannot, shape: ShapeListOf, label: "the holder cannot {value}", values: []value{
		{word: "move", label: "cannot move by its own will", to: "move"},
	}},
	{place: PlaceChange, word: wordWhen, shape: ShapeEntries, holds: PlaceWhen, label: "narrows the change it sits beside"},

	{place: PlaceWhen, word: wordAbility, shape: ShapeOneOf, label: "only when the ability is {value}", values: abilityValues()},
	{place: PlaceWhen, word: wordAttack, shape: ShapeOneOf, label: "only for {value} attacks", values: []value{
		{word: "melee", label: "only for melee attacks", to: "action.melee"},
	}},

	{place: PlaceMaintenance, word: wordEndsOn, shape: ShapeOneOf, label: "ends when {value}", values: []value{
		{word: "run", label: "ends when the run ends", to: "run_end", reason: "run_ended"},
		{word: "rest", label: "ends at the next rest", to: "rest", reason: "rested"},
		{word: "fight starts", label: "ends when a fight the holder is in starts", to: "combat_start", reason: "fight_started"},
		{word: "fight ends", label: "ends when the fight ends", to: "combat_end", reason: "fight_ended"},
		{word: "next turn", label: "ends when the holder's next turn starts", to: "turn_start", reason: "next_turn"},
		{word: "dropped to zero", label: "ends when the holder drops to zero hit points", to: "dropped_to_zero", reason: "dropped_to_zero"},
	}},

	{place: PlaceWhenGiven, word: wordHeal, shape: ShapePositiveWholeNumber, label: "heals the holder by {value} when given"},
}

// numberBound is the range a number shape accepts. Every authored whole
// number is bounded here, by its shape, and only here: an unbounded one would
// overflow the sheet it lands on (a maximum or a heal wrapping negative).
type numberBound struct {
	min, max int
	// zero is whether zero is allowed inside the range.
	zero bool
	// says is the refusal's wording, with the bound in it.
	says string
}

var numberBounds = map[Shape]numberBound{
	ShapeWholeNumber:         {min: -1000, max: 1000, says: "must be a whole number other than zero, from -1000 to 1000, such as 2 or -1"},
	ShapePositiveWholeNumber: {min: 1, max: 1000, says: "must be a whole number of one or more, at most 1000, such as 1 or 2"},
}

// changeWhen is which when words each change word may use, for a change whose
// when words do not depend on its value.
var changeWhen = map[string][]string{
	wordResistance: nil,
	wordDamage:     {wordAbility, wordAttack},
	wordMaxHP:      nil,
	wordInitiative: nil,
	wordCannot:     nil,
}

// changeKind is the effect part kind each fixed-number change word maps to.
var changeKind = map[string]string{
	wordDamage:     "damage_bonus",
	wordMaxHP:      "hit_point_maximum",
	wordInitiative: "initiative_bonus",
}

func abilityValues() []value {
	names := []struct {
		word string
		id   abilities.Ability
	}{
		{"strength", abilities.STR}, {"dexterity", abilities.DEX}, {"constitution", abilities.CON},
		{"intelligence", abilities.INT}, {"wisdom", abilities.WIS}, {"charisma", abilities.CHA},
	}
	out := make([]value, 0, len(names))
	for _, n := range names {
		out = append(out, value{word: n.word, label: "only when the ability is " + n.word, to: string(n.id)})
	}
	return out
}

func damageValues() []value {
	var out []value
	for id, kind := range damage.All {
		if kind == damage.None {
			continue
		}
		out = append(out, value{word: id, label: "resistance to " + id + " damage", to: string(kind)})
	}
	sort.Slice(out, func(i, j int) bool { return out[i].word < out[j].word })
	return out
}

// lookup returns the entry for a word at a place.
func lookup(place Place, word string) (entry, bool) {
	for _, e := range table {
		if e.place == place && e.word == word {
			return e, true
		}
	}
	return entry{}, false
}

// wordsAt lists the words allowed at a place, for a refusal.
func wordsAt(place Place) string {
	var words []string
	for _, e := range table {
		if e.place == place {
			words = append(words, e.word)
		}
	}
	return strings.Join(words, ", ")
}

// changeKinds lists the words that name a change, for a refusal: every change
// word but when, which narrows one.
func changeKinds() string {
	var words []string
	for _, e := range table {
		if e.place == PlaceChange && e.word != wordWhen {
			words = append(words, e.word)
		}
	}
	return strings.Join(words, ", ")
}

// topWords lists the words a status itself is written with, for a refusal:
// every word written in the status place, again among them.
func topWords() string {
	var words []string
	for _, e := range table {
		if e.writtenIn() == PlaceStatus {
			words = append(words, e.word)
		}
	}
	return strings.Join(words, ", ")
}

// isTopWord reports whether word is written on the status itself.
func isTopWord(word string) bool {
	for _, e := range table {
		if e.writtenIn() == PlaceStatus && e.word == word {
			return true
		}
	}
	return false
}

// valueOf returns the value entry a word accepts.
func (e entry) valueOf(word string) (value, bool) {
	i := slices.IndexFunc(e.values, func(v value) bool { return v.word == word })
	if i < 0 {
		return value{}, false
	}
	return e.values[i], true
}

// valueWords lists the values a word accepts, for a refusal.
func (e entry) valueWords() string {
	words := make([]string, 0, len(e.values))
	for _, v := range e.values {
		words = append(words, `"`+v.word+`"`)
	}
	return strings.Join(words, ", ")
}
