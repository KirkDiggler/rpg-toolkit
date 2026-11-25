// Copyright (C) 2026 Kirk Diggler
// SPDX-License-Identifier: GPL-3.0-or-later

package status

import (
	"encoding/json"
	"encoding/json/jsontext"
	jsonv2 "encoding/json/v2"
	"fmt"
	"slices"
	"strconv"
	"strings"
)

// SayInput is one status in the author's words, as Compile takes it.
type SayInput struct {
	ID     string
	Status json.RawMessage
}

// SaidPart is one row of a status, said.
type SaidPart struct {
	// Path is the row's place in the author's shape: changes[0],
	// when_given[0], maintenance[1] or again.
	Path string
	// Text is the row said with the table's labels.
	Text string
}

// SayOutput is a whole status, said with the table's labels.
type SayOutput struct {
	// Name is the status's name as the author wrote it.
	Name string
	// Sentence is the whole status as one statement.
	Sentence string
	// Parts is one entry per row: the changes, then the when_given acts, then
	// the maintenance rules, then again.
	Parts []SaidPart
}

// Say gives the engine's sentence for one status, built from the words
// table's labels and nothing else but the joining words. A status [Compile]
// refuses is refused the same way, unchanged: the [*Refusal] carries the place
// inside the status.
func Say(in *SayInput) (*SayOutput, error) {
	var compile *CompileInput
	if in != nil {
		compile = &CompileInput{ID: in.ID, Status: in.Status}
	}
	compiled, err := Compile(compile)
	if err != nil {
		return nil, err
	}
	// Compile accepted it, so every decode below is of a document it judged.
	top, err := object(in.Status, "the status", PlaceStatus)
	if err != nil {
		return nil, err
	}
	out := &SayOutput{Name: compiled.Name}
	var changes, given, rules []string
	for _, w := range table {
		if w.writtenIn() != PlaceStatus || w.shape != ShapeList {
			continue
		}
		raw, ok := top[w.word]
		if !ok {
			continue
		}
		items, err := list(raw, w.word)
		if err != nil {
			return nil, err
		}
		for i, item := range items {
			text, err := sayRow(w.holds, item)
			if err != nil {
				return nil, err
			}
			out.Parts = append(out.Parts, SaidPart{Path: fmt.Sprintf("%s[%d]", w.word, i), Text: text})
			switch w.word {
			case wordChanges:
				changes = append(changes, text)
			case wordWhenGiven:
				given = append(given, text)
			case wordMaintenance:
				rules = append(rules, text)
			}
		}
	}
	again, _ := lookup(PlaceAgain, wordAgain)
	v, err := oneOf(again, top[wordAgain], wordAgain)
	if err != nil {
		return nil, err
	}
	out.Parts = append(out.Parts, SaidPart{Path: wordAgain, Text: v.label})
	// The parts follow the table's order of lists; the author's order of rows
	// is changes, when_given, maintenance, again.
	order := map[string]int{wordChanges: 0, wordWhenGiven: 1, wordMaintenance: 2, wordAgain: 3}
	slices.SortStableFunc(out.Parts, func(a, b SaidPart) int {
		return order[partWord(a.Path)] - order[partWord(b.Path)]
	})

	var sentence strings.Builder
	sentence.WriteString(out.Name + ": " + strings.Join(changes, "; ") + ". ")
	if len(given) > 0 {
		sentence.WriteString(upperFirst(strings.Join(given, "; ")) + ". ")
	}
	sentence.WriteString("It " + strings.Join(rules, ", or ") + ". " + upperFirst(v.label) + ".")
	out.Sentence = sentence.String()
	return out, nil
}

// partWord is the word a part's path starts with.
func partWord(path string) string {
	word, _, _ := strings.Cut(path, "[")
	return word
}

// sayRow says one entry of a list: the one word of the place it holds, with
// the narrowing word beside it said after, when there is one.
func sayRow(place Place, raw jsontext.Value) (string, error) {
	fields, err := object(raw, "entry", place)
	if err != nil {
		return "", err
	}
	var text string
	for word, value := range fields {
		if narrows(place, word) {
			continue
		}
		e, _ := lookup(place, word)
		if text, err = sayWord(e, value); err != nil {
			return "", err
		}
	}
	if raw, ok := fields[wordWhen]; ok && narrows(place, wordWhen) {
		whenEntry, _ := lookup(place, wordWhen)
		said, err := sayWhens(whenEntry.holds, raw)
		if err != nil {
			return "", err
		}
		text += ", " + said
	}
	return text, nil
}

// sayWhens says a when object: each when word's value label, in the table's
// order, joined with "and".
func sayWhens(place Place, raw jsontext.Value) (string, error) {
	fields, err := object(raw, "when", place)
	if err != nil {
		return "", err
	}
	var said []string
	for _, e := range table {
		if e.place != place {
			continue
		}
		value, ok := fields[e.word]
		if !ok {
			continue
		}
		text, err := sayWord(e, value)
		if err != nil {
			return "", err
		}
		said = append(said, text)
	}
	return strings.Join(said, " and "), nil
}

// sayWord says one word with its value, by its shape, from the table's labels.
func sayWord(e entry, raw jsontext.Value) (string, error) {
	switch e.shape {
	case ShapeWholeNumber, ShapePositiveWholeNumber:
		var n int
		if err := jsonv2.Unmarshal(raw, &n); err != nil {
			return "", refuse(e.word, "%s: must be a whole number", e.word)
		}
		return strings.ReplaceAll(e.label, "{value}", strconv.Itoa(n)), nil
	case ShapeOneOf:
		v, err := oneOf(e, raw, e.word)
		if err != nil {
			return "", err
		}
		return v.label, nil
	case ShapeListOf:
		var words []string
		if err := jsonv2.Unmarshal(raw, &words); err != nil {
			return "", refuse(e.word, "%s: must be a list", e.word)
		}
		said := make([]string, 0, len(words))
		for _, word := range words {
			v, ok := e.valueOf(word)
			if !ok {
				return "", refuse(e.word, "%s: %q is not one of: %s", e.word, word, e.valueWords())
			}
			said = append(said, v.label)
		}
		return strings.Join(said, " and "), nil
	}
	return "", refuse(e.word, "%s: %s is not a word that says itself", e.word, e.shape)
}

// upperFirst upper-cases the first letter of a sentence.
func upperFirst(s string) string {
	if s == "" {
		return s
	}
	return strings.ToUpper(s[:1]) + s[1:]
}
