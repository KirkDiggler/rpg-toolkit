// Copyright (C) 2026 Kirk Diggler
// SPDX-License-Identifier: GPL-3.0-or-later

package features

import (
	"bytes"
	"encoding/json"
	jsonv2 "encoding/json/v2"
	"fmt"
	"slices"

	"github.com/KirkDiggler/rpg-toolkit/core"
	coreResources "github.com/KirkDiggler/rpg-toolkit/core/resources"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/combat/actions"
	dnd5eEvents "github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/events"
)

// namedAbility is one ability a giver can let somebody use: a class, a worn
// item, a stat block. An entry is everything the rules know about it by name.
// Adding an ability is adding an entry.
//
// To add a named ability: write its entry beside its own description, with its
// word, ref, label, values, check and describe, and list it in
// [namedAbilityTable]. [NamedAbility], [NamedAbilities], [ValidateParameters],
// [DescribeFromRef] and [AbilityWords] find it there; none of them is edited.
type namedAbility struct {
	// word is what an author writes: "second wind". Lower case, single spaces.
	word string
	// ref is the ability's ref.
	ref func() core.Ref
	// label is how a studio and a story name it: "Second Wind".
	label string
	// values are the words its `with` takes, for the studio's table, in order.
	// Empty for an ability that takes none. The table is also what content is
	// held to: [ValidateParameters] holds each value it names, when present,
	// to a whole number within Min and Max.
	values []AbilityValue
	// check decodes and judges the giver's values, strictly. An ability that
	// takes none refuses any.
	check func(raw json.RawMessage) error
	// counted is true for an ability that only gives: its price charges no
	// slot, so with no count behind it the ability would be free without end.
	// [DescribeFromRef] refuses a counted ability that is given no pools, and
	// a kit refuses to write or load one with no pool to pay from. Second
	// Wind is not counted: it charges a bonus action.
	counted bool
	// describe builds the description the gate runs. The giver and the pools
	// have already been judged by the shared checks when it is called.
	describe func(in *describeInput) (*actions.Definition, error)
}

// describeInput is what an entry's describe is handed: the values as the
// giver wrote them, the giver, and the pools already judged and copied (nil
// when there are none). An entry re-checks none of it but its own values.
type describeInput struct {
	parameters json.RawMessage
	giver      dnd5eEvents.RollSource
	pools      map[coreResources.ResourceKey]int
}

// abilityTable is the ordered collection of named abilities. Order is the
// order they were added; [NamedAbilities] sorts for the caller.
type abilityTable struct {
	entries []*namedAbility
}

// namedAbilityTable is the one table. An ability is added by listing its entry
// here; every function that asks for an ability by word or by ref reads it.
var namedAbilityTable = &abilityTable{entries: []*namedAbility{
	&secondWindAbility,
	&actionSurgeAbility,
	&rageAbility,
}}

// add appends an entry and returns the function that takes it out again, for
// a test that needs a second ability for the length of one test.
func (t *abilityTable) add(entry *namedAbility) (remove func()) {
	t.entries = append(t.entries, entry)
	return func() {
		t.entries = slices.DeleteFunc(t.entries, func(e *namedAbility) bool { return e == entry })
	}
}

func (t *abilityTable) byWord(word string) (*namedAbility, bool) {
	for _, entry := range t.entries {
		if entry.word == word {
			return entry, true
		}
	}
	return nil, false
}

func (t *abilityTable) byRef(ref core.Ref) (*namedAbility, bool) {
	for _, entry := range t.entries {
		if entry.ref() == ref {
			return entry, true
		}
	}
	return nil, false
}

// DescribeFromRefInput asks for a named ability with the values its giver supplies.
type DescribeFromRefInput struct {
	// Ref names the ability: one of the refs [NamedAbilities] lists by word.
	Ref core.Ref
	// Parameters are the ability's values as JSON, decoded strictly.
	// Second Wind takes {"level": n}, n at least 1; Action Surge takes none.
	Parameters json.RawMessage
	// Giver is who supplies the values. It is named on the roll's modifier:
	// a class ("Fighter level"), an item ("Ring level").
	Giver dnd5eEvents.RollSource
	// Pools are the giver's counts that pay, beside what the ability itself
	// costs. May be empty: a giver that keeps its own count pays elsewhere.
	Pools map[coreResources.ResourceKey]int
}

// ValidateParametersInput names an ability and the values to check.
type ValidateParametersInput struct {
	Ref        core.Ref
	Parameters json.RawMessage
}

// DescribeFromRef returns the action description the gate runs for a named
// ability, with the values its giver supplies. Every giver of Second Wind, a
// class feature or an item, asks here; the meaning is written once.
func DescribeFromRef(in *DescribeFromRefInput) (*actions.Definition, error) {
	if in == nil {
		return nil, fmt.Errorf("describing an ability requires input")
	}
	entry, err := checkedEntry(in.Ref, in.Parameters)
	if err != nil {
		return nil, err
	}
	if err := validateGiver(entry.word, in.Giver); err != nil {
		return nil, err
	}
	pools, err := copyPools(entry.word, in.Pools)
	if err != nil {
		return nil, err
	}
	if entry.counted && len(pools) == 0 {
		return nil, fmt.Errorf("%s must be paid from a count: its giver gives it uses or a cost", entry.word)
	}
	return entry.describe(&describeInput{parameters: in.Parameters, giver: in.Giver, pools: pools})
}

// ValidateParameters checks values with the same contract DescribeFromRef uses.
// It refuses the ref and the parameters; the giver and pools are not its concern.
// It is asked by content only (a kit, an item, a stat block), so after the
// entry's check passes it also holds each value the entry's values table names,
// when present, to a whole number of at least Min and, unless NoMax, at most
// Max. An ability's own check may accept a form that only a class feature can
// answer (Rage by barbarian level); content may not write it.
func ValidateParameters(in *ValidateParametersInput) error {
	if in == nil {
		return fmt.Errorf("validating ability parameters requires input")
	}
	entry, err := checkedEntry(in.Ref, in.Parameters)
	if err != nil {
		return err
	}
	return checkValuesTable(entry, in.Parameters)
}

// checkValuesTable holds each value the entry's table names, when present, to a
// whole number within the table's limits. It names no ability.
func checkValuesTable(entry *namedAbility, raw json.RawMessage) error {
	var members map[string]json.RawMessage
	if err := json.Unmarshal(raw, &members); err != nil {
		return nil // not an object: the entry's check has judged it
	}
	for _, value := range entry.values {
		written, present := members[value.Word]
		if !present {
			continue
		}
		written = bytes.TrimSpace(written)
		var amount int
		ok := jsonv2.Unmarshal(written, &amount) == nil && amount >= value.Min && (value.NoMax || amount <= value.Max)
		if ok {
			continue
		}
		limits := fmt.Sprintf("at least %d", value.Min)
		if !value.NoMax {
			limits += fmt.Sprintf(" and at most %d", value.Max)
		}
		return fmt.Errorf("%s %s is %s; it is a whole number of %s", entry.word, value.Word, written, limits)
	}
	return nil
}

// checkedEntry finds the entry for a ref and has it judge the values.
func checkedEntry(ref core.Ref, raw json.RawMessage) (*namedAbility, error) {
	entry, ok := namedAbilityTable.byRef(ref)
	if !ok {
		return nil, fmt.Errorf("no described ability %s", ref.String())
	}
	if err := entry.check(raw); err != nil {
		return nil, err
	}
	return entry, nil
}

// validateGiver judges the giver for the ability the word names.
func validateGiver(word string, giver dnd5eEvents.RollSource) error {
	if giver.Ref == nil {
		return fmt.Errorf("%s giver requires a Ref", word)
	}
	if giver.Label == "" {
		return fmt.Errorf("%s giver requires a Label", word)
	}
	if giver.SourceID == "" {
		return fmt.Errorf("%s giver requires a SourceID", word)
	}
	return nil
}

// copyPools judges the giver's pools for the ability the word names and
// returns a copy, nil when there are none.
func copyPools(word string, in map[coreResources.ResourceKey]int) (map[coreResources.ResourceKey]int, error) {
	pools := make(map[coreResources.ResourceKey]int, len(in))
	for key, amount := range in {
		if key == "" {
			return nil, fmt.Errorf("%s pool key is empty", word)
		}
		if amount < 1 {
			return nil, fmt.Errorf("%s pool %q amount %d must be at least 1", word, key, amount)
		}
		pools[key] = amount
	}
	if len(pools) == 0 {
		pools = nil
	}
	return pools, nil
}

// MustBeCounted reports whether the named ability charges no slot, so that its
// giver must pay it from a count. It is false for a ref no entry knows.
func MustBeCounted(ref core.Ref) bool {
	entry, ok := namedAbilityTable.byRef(ref)
	return ok && entry.counted
}

// NamedAbility returns the ref of a described ability by the plain name an
// author writes: "second wind". The name is matched exactly: lower case,
// single spaces.
func NamedAbility(name string) (core.Ref, bool) {
	entry, ok := namedAbilityTable.byWord(name)
	if !ok {
		return core.Ref{}, false
	}
	return entry.ref(), true
}

// NamedAbilities lists the plain names, sorted, for a refusal to offer.
func NamedAbilities() []string {
	words := make([]string, 0, len(namedAbilityTable.entries))
	for _, entry := range namedAbilityTable.entries {
		words = append(words, entry.word)
	}
	slices.Sort(words)
	return words
}
