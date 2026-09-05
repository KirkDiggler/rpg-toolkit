// Copyright (C) 2026 Kirk Diggler
// SPDX-License-Identifier: GPL-3.0-or-later

package dungeonspec

import (
	"fmt"
	"strings"

	"gopkg.in/yaml.v3"
)

// predicate.go is THE PREDICATE, the designer's noun (rpg-project#375, the
// hold-out design §2): one authorable grammar, used by more than one field.
//
// A disposition's `until` takes one; a placement's `arrives` (step B) takes
// the same one; an authored ending (step B, R10) takes the same one again.
// The second consumer arriving inside the same slice is what earned the
// grammar a type of its own rather than a field per form on each consumer
// (the second-instance law). Four forms today, and the set is CLOSED: it
// grows one form per use case, sealed the way [encounter.Trigger] is — every
// form here compiles to one of those, so the engine has one evaluator and the
// file has one spelling.
//
//	until:   { round: 6 }
//	until:   { down: chief }
//	until:   { fact: saved-wiseman }
//	when:    { stance: { between: [raiders, party], is: neutral } }
//
// # Exactly one key
//
// A predicate that says two things is not two predicates, it is an author who
// has not decided, and it is refused naming both keys rather than read as
// either. A predicate that says nothing is refused too — nothing is defaulted
// (rpg-toolkit#1033), and there is no "until forever".
//
// # Read by hand, refused by name
//
// The decoder's KnownFields does not reach inside a custom unmarshaler, so
// the unknown-key refusal every other hand-read shape in this package makes
// ([PositionSpec], [WallSpec], [DoorSpec], [PlaceSpec]) is made here too, in
// the same words.

// PredicateSpec is one authored predicate: exactly one of the four forms.
//
// Pointers and empty strings mean "not this form"; [PredicateSpec.Form]
// names which one it is. The struct carries every form rather than being
// an interface so a validator can name the field it is about in the file's
// own path (`dispositions[0].until.down`).
type PredicateSpec struct {
	// Round is the `{ round: N }` form: holds when any fight in the run has
	// started round N (design R9 — the fight's own clock; outside a fight it
	// never holds). N is at least 1, refused otherwise.
	Round *int

	// Down is the `{ down: <placement id> }` form: holds when that member is
	// Down. Must name a MONSTER placement in this file — a prop cannot fall.
	Down string

	// Fact is the `{ fact: <id> }` form: on `until`, holds when the faction's
	// mind knows the fact; on `arrives`, when anyone in the run does (design
	// R5 — two grains, one spelling). A fact id is a plain string declared by
	// mention; the dungeon does not require any record to reveal it (R8: the
	// dungeon allows, the scenario refuses).
	Fact string

	// Stance is the `{ stance: { between: [a, b], is: <word> } }` form:
	// holds when the pair's stance folds to that value. Both factions must
	// exist (`party` allowed), and the word is one of hostile, neutral,
	// allied.
	Stance *StancePredicateSpec
}

// StancePredicateSpec is the stance form's own object: the pair, and the
// word the pair's stance must fold to. ONE nested key rather than two flat
// ones, so a predicate stays "exactly one key" for every form.
type StancePredicateSpec struct {
	// Between is the two factions, unordered — the same pair a disposition
	// names. REQUIRED.
	Between [2]string `yaml:"between"`

	// Is is the stance the pair must fold to: hostile, neutral, or allied.
	// REQUIRED.
	Is string `yaml:"is"`
}

// Predicate form names, for a refusal that says which form was meant.
const (
	predicateRound  = "round"
	predicateDown   = "down"
	predicateFact   = "fact"
	predicateStance = "stance"
)

// predicateForms is the one sentence every predicate refusal points at.
const predicateForms = "a predicate is exactly one of { round: N }, { down: <placement id> }, " +
	"{ fact: <id> }, or { stance: { between: [a, b], is: hostile|neutral|allied } }"

// stanceForm is the stance form's own sentence.
const stanceForm = "a stance predicate is { stance: { between: [a, b], is: hostile|neutral|allied } }"

// Form names which of the four forms this predicate is, or "" for one that
// says nothing — which [Validate] refuses; a decoded predicate has exactly
// one, because [PredicateSpec.UnmarshalYAML] refused every other count.
func (p *PredicateSpec) Form() string {
	switch {
	case p == nil:
		return ""
	case p.Round != nil:
		return predicateRound
	case p.Down != "":
		return predicateDown
	case p.Fact != "":
		return predicateFact
	case p.Stance != nil:
		return predicateStance
	default:
		return ""
	}
}

// UnmarshalYAML reads a predicate, refusing anything but exactly one of the
// four forms, and any unknown key, by name — for [PositionSpec.UnmarshalYAML]'s
// reason: a custom unmarshaler bypasses the decoder's KnownFields, and a
// typo silently dropped is exactly what Decode's strictness exists to prevent.
func (p *PredicateSpec) UnmarshalYAML(value *yaml.Node) error {
	if value.Kind != yaml.MappingNode {
		return fmt.Errorf("line %d: %s", value.Line, predicateForms)
	}

	var forms []string
	for i := 0; i < len(value.Content); i += 2 {
		switch key := value.Content[i].Value; key {
		case predicateRound, predicateDown, predicateFact, predicateStance:
			forms = append(forms, key)
		default:
			return fmt.Errorf("line %d: field %s not found in type dungeonspec.PredicateSpec",
				value.Content[i].Line, key)
		}
	}
	switch {
	case len(forms) == 0:
		return fmt.Errorf("line %d: this predicate says nothing — %s", value.Line, predicateForms)
	case len(forms) > 1:
		return fmt.Errorf("line %d: this predicate says both `%s` and `%s` — %s",
			value.Line, forms[0], forms[1], predicateForms)
	}

	var obj struct {
		Round  *int                 `yaml:"round"`
		Down   string               `yaml:"down"`
		Fact   string               `yaml:"fact"`
		Stance *StancePredicateSpec `yaml:"stance"`
	}
	if err := value.Decode(&obj); err != nil {
		return err
	}
	*p = PredicateSpec{Round: obj.Round, Down: obj.Down, Fact: obj.Fact, Stance: obj.Stance}

	// A form that decoded to its own zero value said the key and nothing
	// else — `{ fact: }` or `{ down: "" }` — which is the same author halfway
	// through as an empty predicate, and refused the same way.
	if p.Form() == "" {
		return fmt.Errorf("line %d: this predicate's `%s` says nothing — %s", value.Line, forms[0], predicateForms)
	}

	return nil
}

// UnmarshalYAML reads the stance form's object, refusing an unknown key and a
// missing half by name — both `between` and `is` are the form, and one
// without the other is an author halfway through it.
func (s *StancePredicateSpec) UnmarshalYAML(value *yaml.Node) error {
	if value.Kind != yaml.MappingNode {
		return fmt.Errorf("line %d: %s", value.Line, stanceForm)
	}
	var sawBetween, sawIs bool
	for i := 0; i < len(value.Content); i += 2 {
		switch key := value.Content[i].Value; key {
		case "between":
			sawBetween = true
		case "is":
			sawIs = true
		default:
			return fmt.Errorf("line %d: field %s not found in type dungeonspec.StancePredicateSpec",
				value.Content[i].Line, key)
		}
	}
	if !sawBetween {
		return fmt.Errorf("line %d: a stance predicate does not say which pair (`between: [a, b]`) — %s",
			value.Line, stanceForm)
	}
	if !sawIs {
		return fmt.Errorf("line %d: a stance predicate does not say which stance (`is: hostile|neutral|allied`) — %s",
			value.Line, stanceForm)
	}
	var obj struct {
		Between [2]string `yaml:"between"`
		Is      string    `yaml:"is"`
	}
	if err := value.Decode(&obj); err != nil {
		return err
	}
	s.Between, s.Is = obj.Between, obj.Is

	return nil
}

// String renders the predicate the way the file spells it, for a refusal
// that quotes it back.
func (p *PredicateSpec) String() string {
	switch p.Form() {
	case predicateRound:
		return fmt.Sprintf("{ round: %d }", *p.Round)
	case predicateDown:
		return fmt.Sprintf("{ down: %s }", p.Down)
	case predicateFact:
		return fmt.Sprintf("{ fact: %s }", p.Fact)
	case predicateStance:
		return fmt.Sprintf("{ stance: { between: [%s], is: %s } }",
			strings.Join(p.Stance.Between[:], ", "), p.Stance.Is)
	default:
		return "{ }"
	}
}
