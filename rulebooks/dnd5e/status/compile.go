// Copyright (C) 2026 Kirk Diggler
// SPDX-License-Identifier: GPL-3.0-or-later

package status

import (
	"encoding/json"
	"encoding/json/jsontext"
	jsonv2 "encoding/json/v2"
	"errors"
	"fmt"
	"slices"
	"strings"

	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/conditions"
)

// ErrBadStatus is wrapped by every refusal to compile an authored status. The
// message names the path in the author's shape (changes[0].when.skill) and the
// words allowed there.
var ErrBadStatus = errors.New("status: not a status this engine can run")

// CompileInput is one authored status.
type CompileInput struct {
	// ID is the status's id: the definition's id inside the library, and the
	// id resolution's GiveStatus is asked to give.
	ID string

	// Status is the status in the author's words, as JSON:
	// {"name":…, "changes":[…], "maintenance":[…], "again":…}, with an
	// optional "when_given":[…].
	Status json.RawMessage

	// Until, when set, is how long the status is given for: it ends when those
	// rounds are up, in addition to the endings the status states itself.
	Until *Until
}

// Until is a status given for a number of rounds of world time; the timer is
// kept by whoever gives it, and tells the holder when the rounds are up.
type Until struct {
	Rounds int
}

// CompileOutput is the authored status as the engine runs it.
type CompileOutput struct {
	// Name is the status's display name, as the author wrote it.
	Name string

	// Library is a conditions definition library holding the status, ready
	// for conditions.CompileDefinitions and resolution's GiveStatus.
	Library json.RawMessage

	// DefinitionID is the status's definition inside Library.
	DefinitionID string

	// WhenGiven is what the status does once, when it is given, in the order
	// the author wrote it; nil when the author wrote no when_given. These are
	// not effect parts and are not in Library: the caller runs them, and runs
	// them only on a FIRST giving — a member who already holds the status, and
	// so is answered "already held", receives none of them again.
	WhenGiven []WhenGivenAct
}

// WhenGivenKind names one thing a status does once, when it is given.
type WhenGivenKind string

// WhenGivenHeal heals the holder by the act's Amount of hit points.
const WhenGivenHeal WhenGivenKind = wordHeal

// WhenGivenAct is one thing a status does once, when it is given.
type WhenGivenAct struct {
	// Kind is what is done.
	Kind WhenGivenKind
	// Amount is how much: a whole number of one or more.
	Amount int
}

// Compile turns a status written in an author's words into the definition
// library the engine runs. It is strict: an unknown or missing word or value
// anywhere is refused, wrapping [ErrBadStatus], naming the path in the
// author's shape and the words allowed there. Every library it returns
// compiles with conditions.CompileDefinitions; one that would not is refused.
//
// The status is coordinated on its holder: it holds its own maintenance and
// its own changes. "again": "ignored" adds nothing to the library — a second
// giving is answered by resolution's GiveStatus, which gives nothing to a
// member already holding the status.
//
// when_given is optional and returns as [CompileOutput.WhenGiven], beside the
// library rather than in it. changes stays required even beside a when_given:
// the engine's definitions need at least one benefit, so a status that only
// heals once is not one this engine can hold.
func Compile(in *CompileInput) (*CompileOutput, error) {
	if in == nil {
		return nil, fmt.Errorf("%w: no input", ErrBadStatus)
	}
	id := strings.TrimSpace(in.ID)
	if id == "" || id != in.ID {
		return nil, fmt.Errorf("%w: a status needs an id without surrounding spaces", ErrBadStatus)
	}
	top, err := object(in.Status, "the status", PlaceStatus)
	if err != nil {
		return nil, err
	}
	c := &compiler{id: id, parts: map[string]any{}}
	if err := c.status(top); err != nil {
		return nil, err
	}
	if in.Until != nil {
		if in.Until.Rounds < 1 || in.Until.Rounds > 1000 {
			return nil, fmt.Errorf("%w: until.rounds must be a whole number of one or more, at most 1000", ErrBadStatus)
		}
		c.timed = true
	}
	library, err := c.library()
	if err != nil {
		return nil, err
	}
	if _, err := conditions.CompileDefinitions(&conditions.CompileDefinitionsInput{Data: library}); err != nil {
		return nil, fmt.Errorf("%w: %q compiles to a definition the engine refuses: %w", ErrBadStatus, id, err)
	}
	return &CompileOutput{Name: c.name, Library: library, DefinitionID: id, WhenGiven: c.whenGiven}, nil
}

// compiler gathers one status's library pieces.
type compiler struct {
	id          string
	name        string
	parts       map[string]any
	benefits    []string
	maintenance []string
	whenGiven   []WhenGivenAct
	timed       bool // given for a number of rounds: carries a lapse part
}

func (c *compiler) status(top map[string]jsontext.Value) error {
	for word := range top {
		if !isTopWord(word) {
			return fmt.Errorf("%w: the status: unknown word %s; one of: %s", ErrBadStatus, word, topWords())
		}
	}
	for _, word := range []string{wordName, wordChanges, wordMaintenance, wordAgain} {
		if _, ok := top[word]; !ok {
			return fmt.Errorf("%w: the status has no %s", ErrBadStatus, word)
		}
	}
	if err := jsonv2.Unmarshal(top[wordName], &c.name); err != nil || strings.TrimSpace(c.name) == "" {
		return fmt.Errorf("%w: name: must be non-empty text", ErrBadStatus)
	}
	again, _ := lookup(PlaceAgain, wordAgain)
	if _, err := oneOf(again, top[wordAgain], wordAgain); err != nil {
		return err
	}
	changes, err := list(top[wordChanges], wordChanges)
	if err != nil {
		return err
	}
	for i, raw := range changes {
		if err := c.change(raw, fmt.Sprintf("%s[%d]", wordChanges, i)); err != nil {
			return err
		}
	}
	rules, err := list(top[wordMaintenance], wordMaintenance)
	if err != nil {
		return err
	}
	for i, raw := range rules {
		if err := c.rule(raw, fmt.Sprintf("%s[%d]", wordMaintenance, i)); err != nil {
			return err
		}
	}
	if raw, ok := top[wordWhenGiven]; ok {
		acts, err := list(raw, wordWhenGiven)
		if err != nil {
			return err
		}
		for i, raw := range acts {
			if err := c.act(raw, fmt.Sprintf("%s[%d]", wordWhenGiven, i)); err != nil {
				return err
			}
		}
	}
	return nil
}

// act reads one when_given entry: one word, done once when the status is
// given.
func (c *compiler) act(raw jsontext.Value, path string) error {
	fields, err := object(raw, path, PlaceWhenGiven)
	if err != nil {
		return err
	}
	if len(fields) != 1 {
		return fmt.Errorf("%w: %s: one act per entry; an act is one of: %s", ErrBadStatus, path, wordsAt(PlaceWhenGiven))
	}
	for word, value := range fields {
		e, ok := lookup(PlaceWhenGiven, word)
		if !ok {
			return fmt.Errorf("%w: %s.%s: not something done when given; one of: %s", ErrBadStatus, path, word, wordsAt(PlaceWhenGiven))
		}
		n, err := number(e, value, path+"."+word)
		if err != nil {
			return err
		}
		// The kind IS the word matched: a when_given word names its own act.
		c.whenGiven = append(c.whenGiven, WhenGivenAct{Kind: WhenGivenKind(e.word), Amount: n})
	}
	return nil
}

func (c *compiler) change(raw jsontext.Value, path string) error {
	fields, err := object(raw, path, PlaceChange)
	if err != nil {
		return err
	}
	var kind string
	for word := range fields {
		if word == wordWhen {
			continue
		}
		if _, ok := lookup(PlaceChange, word); !ok {
			return fmt.Errorf("%w: %s.%s: not a change; a change is one of: %s", ErrBadStatus, path, word, changeKinds())
		}
		if kind != "" {
			return fmt.Errorf("%w: %s: one change per entry, got %s and %s", ErrBadStatus, path, kind, word)
		}
		kind = word
	}
	if kind == "" {
		return fmt.Errorf("%w: %s: names no change; a change is one of: %s", ErrBadStatus, path, changeKinds())
	}
	e, _ := lookup(PlaceChange, kind)
	at := path + "." + kind
	part := map[string]any{}
	var whenWords []string
	switch kind {
	case wordAdvantage:
		v, err := oneOf(e, fields[kind], at)
		if err != nil {
			return err
		}
		part["kind"] = "advantage"
		part["params"] = json.RawMessage(v.to)
		whenWords = v.when
	case wordResistance:
		types, err := listOf(e, fields[kind], at)
		if err != nil {
			return err
		}
		part["kind"] = "resistance"
		part["params"] = map[string]any{"types": types}
		whenWords = changeWhen[kind]
	case wordCannot:
		what, err := listOf(e, fields[kind], at)
		if err != nil {
			return err
		}
		if _, ok := fields[wordWhen]; ok {
			return fmt.Errorf("%w: %s.%s: cannot takes no when; it is in force while held", ErrBadStatus, path, wordWhen)
		}
		part["kind"] = "cannot"
		part["params"] = map[string]any{"what": what}
	case wordDamage, wordMaxHP, wordInitiative:
		n, err := number(e, fields[kind], at)
		if err != nil {
			return err
		}
		part["kind"] = changeKind[kind]
		part["amount"] = map[string]any{"fixed": n}
		whenWords = changeWhen[kind]
	}
	if raw, ok := fields[wordWhen]; ok {
		guard, err := when(raw, path+"."+wordWhen, kind, whenWords)
		if err != nil {
			return err
		}
		part["when"] = guard
	}
	key := fmt.Sprintf("change-%d", len(c.benefits))
	c.parts[key] = part
	c.benefits = append(c.benefits, key)
	return nil
}

func when(raw jsontext.Value, path, kind string, allowed []string) (map[string]any, error) {
	fields, err := object(raw, path, PlaceWhen)
	if err != nil {
		return nil, err
	}
	if len(fields) == 0 {
		return nil, fmt.Errorf("%w: %s: is empty; leave it out, or name one of: %s", ErrBadStatus, path, wordsAt(PlaceWhen))
	}
	words := make([]string, 0, len(fields))
	for word := range fields {
		words = append(words, word)
	}
	slices.Sort(words)
	var facts []map[string]any
	for _, word := range words {
		e, ok := lookup(PlaceWhen, word)
		if !ok {
			return nil, fmt.Errorf("%w: %s.%s: not a when word; one of: %s", ErrBadStatus, path, word, wordsAt(PlaceWhen))
		}
		if !slices.Contains(allowed, word) {
			can := "nothing: it takes no when"
			if len(allowed) > 0 {
				can = strings.Join(allowed, ", ")
			}
			return nil, fmt.Errorf("%w: %s.%s: %s cannot be narrowed by %s; it can use %s", ErrBadStatus, path, word, kind, word, can)
		}
		v, err := oneOf(e, fields[word], path+"."+word)
		if err != nil {
			return nil, err
		}
		switch word {
		case wordAbility:
			facts = append(facts, map[string]any{"fact": map[string]any{"name": "action.ability", "op": "eq", "value": v.to}})
		case wordAttack:
			facts = append(facts, map[string]any{"fact": map[string]any{"name": v.to, "op": "eq", "value": true}})
		}
	}
	if len(facts) == 1 {
		return facts[0], nil
	}
	return map[string]any{"all": facts}, nil
}

func (c *compiler) rule(raw jsontext.Value, path string) error {
	fields, err := object(raw, path, PlaceMaintenance)
	if err != nil {
		return err
	}
	if len(fields) != 1 {
		return fmt.Errorf("%w: %s: one rule per entry; a rule is one of: %s", ErrBadStatus, path, wordsAt(PlaceMaintenance))
	}
	for word, value := range fields {
		e, ok := lookup(PlaceMaintenance, word)
		if !ok {
			return fmt.Errorf("%w: %s.%s: not a maintenance rule; one of: %s", ErrBadStatus, path, word, wordsAt(PlaceMaintenance))
		}
		v, err := oneOf(e, value, path+"."+word)
		if err != nil {
			return err
		}
		key := fmt.Sprintf("ends-%d", len(c.maintenance))
		c.parts[key] = map[string]any{"kind": "end_on", "params": map[string]any{"beat": v.to, "reason": v.reason}}
		c.maintenance = append(c.maintenance, key)
	}
	return nil
}

func (c *compiler) library() (json.RawMessage, error) {
	if len(c.benefits) == 0 {
		return nil, fmt.Errorf("%w: changes: needs at least one change", ErrBadStatus)
	}
	if len(c.maintenance) == 0 {
		return nil, fmt.Errorf("%w: maintenance: needs at least one rule, or the status would never end", ErrBadStatus)
	}
	if c.timed {
		// After the check above: the giving's until is in addition to the
		// endings the status states itself, never instead of them.
		c.parts["lapse"] = map[string]any{"kind": "lapse"}
		c.maintenance = append(c.maintenance, "lapse")
	}
	maintenance := make([]map[string]string, 0, len(c.maintenance))
	for _, key := range c.maintenance {
		maintenance = append(maintenance, map[string]string{"key": key, "part": key})
	}
	library := map[string]any{
		"parts":        c.parts,
		"benefit_sets": map[string][]string{wordChanges: c.benefits},
		"definitions": map[string]any{c.id: map[string]any{
			"name":        c.name,
			"coordinator": "recipient",
			"maintenance": maintenance,
			"benefits":    []map[string]string{{"key": wordChanges, "set": wordChanges}},
		}},
	}
	out, err := jsonv2.Marshal(library, jsonv2.Deterministic(true))
	if err != nil {
		return nil, fmt.Errorf("%w: %w", ErrBadStatus, err)
	}
	return out, nil
}

// object decodes an author's object, refusing anything else and any repeated
// key.
func object(raw json.RawMessage, path string, place Place) (map[string]jsontext.Value, error) {
	var fields map[string]jsontext.Value
	if err := jsonv2.Unmarshal(raw, &fields); err != nil || fields == nil {
		words := wordsAt(place)
		if place == PlaceStatus {
			words = topWords()
		}
		return nil, fmt.Errorf("%w: %s: must be an object of: %s", ErrBadStatus, path, words)
	}
	return fields, nil
}

func list(raw jsontext.Value, path string) ([]jsontext.Value, error) {
	var items []jsontext.Value
	if err := jsonv2.Unmarshal(raw, &items); err != nil || items == nil {
		return nil, fmt.Errorf("%w: %s: must be a list", ErrBadStatus, path)
	}
	return items, nil
}

// number decodes a whole number within its word's shape's bound.
func number(e entry, raw jsontext.Value, path string) (int, error) {
	bound, ok := numberBounds[e.shape]
	if !ok {
		return 0, fmt.Errorf("%w: %s: %s is not a number word", ErrBadStatus, path, e.word)
	}
	var n int
	if err := jsonv2.Unmarshal(raw, &n); err != nil || n < bound.min || n > bound.max || (n == 0 && !bound.zero) {
		return 0, fmt.Errorf("%w: %s: %s", ErrBadStatus, path, bound.says)
	}
	return n, nil
}

func oneOf(e entry, raw jsontext.Value, path string) (value, error) {
	var word string
	if err := jsonv2.Unmarshal(raw, &word); err != nil {
		return value{}, fmt.Errorf("%w: %s: must be one of: %s", ErrBadStatus, path, e.valueWords())
	}
	v, ok := e.valueOf(word)
	if !ok {
		return value{}, fmt.Errorf("%w: %s: %q is not one of: %s", ErrBadStatus, path, word, e.valueWords())
	}
	return v, nil
}

func listOf(e entry, raw jsontext.Value, path string) ([]string, error) {
	var words []string
	if err := jsonv2.Unmarshal(raw, &words); err != nil || len(words) == 0 {
		return nil, fmt.Errorf("%w: %s: must be a non-empty list of: %s", ErrBadStatus, path, e.valueWords())
	}
	out := make([]string, 0, len(words))
	for i, word := range words {
		v, ok := e.valueOf(word)
		if !ok {
			return nil, fmt.Errorf("%w: %s[%d]: %q is not one of: %s", ErrBadStatus, path, i, word, e.valueWords())
		}
		if slices.Contains(out, v.to) {
			return nil, fmt.Errorf("%w: %s[%d]: %q is listed twice", ErrBadStatus, path, i, word)
		}
		out = append(out, v.to)
	}
	return out, nil
}
