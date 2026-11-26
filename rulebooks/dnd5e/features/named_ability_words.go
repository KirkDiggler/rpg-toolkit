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

// abilityWordTable holds what an author may write for each named ability. It
// is written beside the decode it describes: a bound here that decodeParameters
// also enforces is the same constant.
var abilityWordTable = map[string]AbilityWord{
	"second wind": {
		Name:  "second wind",
		Label: "Second Wind",
		Values: []AbilityValue{{
			Word:     "level",
			Required: true,
			Min:      secondWindMinLevel,
			NoMax:    true,
			Label:    "regains 1d10 hit points plus {value}",
		}},
	},
}

// AbilityWords returns every named ability with the values its `with` takes,
// in the order [NamedAbilities] lists them. Each call returns its own copy.
func AbilityWords(_ *AbilityWordsInput) *AbilityWordsOutput {
	out := &AbilityWordsOutput{}
	for _, name := range NamedAbilities() {
		word := abilityWordTable[name]
		word.Name = name
		word.Values = append([]AbilityValue(nil), word.Values...)
		out.Words = append(out.Words, word)
	}
	return out
}
