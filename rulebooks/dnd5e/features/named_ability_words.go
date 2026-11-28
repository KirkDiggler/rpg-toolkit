// Copyright (C) 2026 Kirk Diggler
// SPDX-License-Identifier: GPL-3.0-or-later

package features

// AbilityValue is one value a named ability takes in its `with`, for a studio
// to draw a control from.
type AbilityValue struct {
	// Word is what an author writes: "level".
	Word string
	// Required is whether the ability cannot be used without it.
	Required bool
	// Min is the least it takes, both ends included.
	Min int
	// Max is the most it takes. It means nothing when NoMax is true; a zero
	// Max never means "no limit".
	Max int
	// NoMax is true when there is no upper limit.
	NoMax bool
	// Label is how a studio reads the value to a person, lower-case first,
	// with {value} where the number goes.
	Label string
}

// AbilityWord is one named ability and the values it takes.
type AbilityWord struct {
	// Name is what an author writes: "second wind".
	Name string
	// Label is how a studio names it to a person: "Second Wind".
	Label string
	// Values are the words its `with` takes, in order.
	Values []AbilityValue
}

// AbilityWordsInput asks for the named abilities and their values. It has no
// fields today.
type AbilityWordsInput struct{}

// AbilityWordsOutput lists one AbilityWord per named ability, in the order
// [NamedAbilities] lists them.
type AbilityWordsOutput struct {
	Words []AbilityWord
}

// AbilityWords returns every named ability with the values its `with` takes,
// in the order [NamedAbilities] lists them. Each call returns its own copy.
func AbilityWords(_ *AbilityWordsInput) *AbilityWordsOutput {
	out := &AbilityWordsOutput{}
	for _, name := range NamedAbilities() {
		entry, ok := namedAbilityTable.byWord(name)
		if !ok {
			continue
		}
		out.Words = append(out.Words, AbilityWord{
			Name:   entry.word,
			Label:  entry.label,
			Values: append([]AbilityValue(nil), entry.values...),
		})
	}
	return out
}
