// Copyright (C) 2026 Kirk Diggler
// SPDX-License-Identifier: GPL-3.0-or-later

package machine

import (
	"encoding/json/jsontext"
	"encoding/json/v2"
	"errors"
	"math"
	"strconv"

	"github.com/KirkDiggler/rpg-toolkit/core"
)

// Recipe is one condition type as data: the states a machine moves through,
// the transitions between them, the trackers that feed the bag, and the
// bag's declared keys with their zero values. Every field is a value, so a
// recipe written as a Go literal and the same recipe written as JSON are one
// shape. Validate checks a recipe against a Vocabulary; Compile binds it.
type Recipe struct {
	Ref      core.Ref         `json:"ref"`
	Stacks   Stacks           `json:"stacks,omitzero"`
	Initial  string           `json:"initial"`
	Bag      Bag              `json:"bag"`
	Trackers []Tracker        `json:"trackers,omitzero"`
	States   map[string]State `json:"states"`
}

// State is one state of a recipe. Outputs are opaque JSON the machine never
// reads: they belong to the composer. Transitions run in order on each beat
// and the first whose guard applies wins. Entering a Terminal state ends the
// condition.
type State struct {
	Outputs     []jsontext.Value `json:"outputs,omitzero"`
	Transitions []Transition     `json:"transitions,omitzero"`
	Terminal    bool             `json:"terminal,omitzero"`
}

// Tracker sets the bool bag key Set to true when the beat On fires about the
// holder, as chosen by the selector About. It is idempotent within a turn: a
// flag already set stays set and reports no change.
type Tracker struct {
	On    Beat     `json:"on"`
	About Selector `json:"about"`
	Set   string   `json:"set"`
}

// Transition is read by the machine on its beat On, when the event is about
// the holder as chosen by About. A nil When always applies. When it applies
// its acts Do run on the bag and, if To names a state, the machine enters
// it. Reason is required when To is terminal and travels on the Transitioned
// event.
type Transition struct {
	On     Beat     `json:"on"`
	About  Selector `json:"about"`
	When   *Guard   `json:"when,omitzero"`
	To     string   `json:"to,omitzero"`
	Do     []Act    `json:"do,omitzero"`
	Reason string   `json:"reason,omitzero"`
}

// Guard is a node of a guard tree: all, any or not over child nodes, or one
// leaf. Exactly one field is set per node; Validate refuses any other count.
type Guard struct {
	All []Guard `json:"all,omitzero"`
	Any []Guard `json:"any,omitzero"`
	Not *Guard  `json:"not,omitzero"`

	Fact       *FactLeaf       `json:"fact,omitzero"`
	Flag       string          `json:"flag,omitzero"`
	Rounds     *RoundsLeaf     `json:"rounds,omitzero"`
	ClassLevel *ClassLevelLeaf `json:"class_level,omitzero"`
}

// FactLeaf compares one vocabulary fact against Value with Op. A fact the
// vocabulary's reader reports unknown answers Depends.
type FactLeaf struct {
	Name  FactName `json:"name"`
	Op    Op       `json:"op"`
	Value any      `json:"value"`
}

// RoundsLeaf compares the beat's round against the bag's anchor
// (KeyRoundActivated). Exactly one of SinceAnchor and WentBackwards is set.
// It answers Depends when the beat carries no round or nothing is anchored.
type RoundsLeaf struct {
	SinceAnchor   *IntOp `json:"since_anchor,omitzero"`
	WentBackwards bool   `json:"went_backwards,omitzero"`
}

// IntOp compares an int with Op against N.
type IntOp struct {
	Op Op  `json:"op"`
	N  int `json:"n"`
}

// ClassLevelLeaf compares a class level with Op against N. Class names an
// int fact of the vocabulary (the rulebook's class space); an unknown level
// answers Depends.
type ClassLevelLeaf struct {
	Class string `json:"class"`
	Op    Op     `json:"op"`
	N     int    `json:"n"`
}

// Act is one bag write a transition performs. Exactly one field is set.
//
//   - AnchorRound sets KeySawTurnEnd true, and KeyRoundActivated to the
//     beat's round when it carries one above zero.
//   - DeriveTurnsActive sets KeyTurnsActive to round - anchor + 1 when the
//     bag is anchored and the beat's round is at or after the anchor.
//   - ClearFlags sets each named bool key false.
//   - Set writes one declared key.
type Act struct {
	AnchorRound       bool     `json:"anchor_round,omitzero"`
	DeriveTurnsActive bool     `json:"derive_turns_active,omitzero"`
	ClearFlags        []string `json:"clear_flags,omitzero"`
	Set               *SetAct  `json:"set,omitzero"`
}

// SetAct writes Value to the declared bag key Key.
type SetAct struct {
	Key   string `json:"key"`
	Value any    `json:"value"`
}

// Beat names a topic a transition or tracker may listen to. The vocabulary
// supplies the names.
type Beat string

// Selector names which id on a beat's event must be the holder. The
// vocabulary lists the selectors each beat supports.
type Selector string

// Op is a comparison in a guard leaf.
type Op string

const (
	// OpEq is equality; it applies to bool, int and string facts.
	OpEq Op = "eq"
	// OpGte is greater-than-or-equal; it applies to int facts only.
	OpGte Op = "gte"
)

// FactName names a fact the vocabulary can read from a frame or event.
type FactName string

// Stacks declares a recipe's stacking identity.
type Stacks string

const (
	// StacksByRef (the default) means one condition per ref per holder.
	StacksByRef Stacks = "by_ref"
	// StacksBySource means one condition per ref per source per holder; it
	// requires a KeySourceID bag key.
	StacksBySource Stacks = "by_source"
)

// Bag keys the machine's own acts and leaves read and write.
const (
	// KeyRoundActivated is the round anchor: zero means not anchored.
	KeyRoundActivated = "round_activated"
	// KeySawTurnEnd records that the first turn end has passed.
	KeySawTurnEnd = "saw_turn_end"
	// KeyTurnsActive is the derived display count of turns since the anchor.
	KeyTurnsActive = "turns_active"
	// KeySourceID is the source qualifier a StacksBySource recipe requires.
	KeySourceID = "source_id"
)

// Bag is the state a condition's parts share and the condition persists.
// Values are bool, int or string.
type Bag map[string]any

// Clone returns a shallow copy of b; nil stays nil.
func (b Bag) Clone() Bag {
	if b == nil {
		return nil
	}
	out := make(Bag, len(b))
	for k, v := range b {
		out[k] = v
	}
	return out
}

// UnmarshalJSONFrom decodes a recipe, landing whole JSON numbers in the bag
// and in leaf values as int so a decoded recipe equals its Go literal.
func (r *Recipe) UnmarshalJSONFrom(dec *jsontext.Decoder) error {
	type plain Recipe
	return json.UnmarshalDecode(dec, (*plain)(r), wholeNumbers)
}

// wholeNumbers decodes a JSON number held in an `any` as int when it is a
// whole number in range, and as float64 otherwise, which Validate refuses.
var wholeNumbers = json.WithUnmarshalers(json.UnmarshalFromFunc(func(dec *jsontext.Decoder, v *any) error {
	if dec.PeekKind() != '0' {
		return errors.ErrUnsupported // not a number: the default decoding applies
	}
	tok, err := dec.ReadToken()
	if err != nil {
		return err
	}
	if n, err := strconv.ParseInt(tok.String(), 10, 0); err == nil && n >= math.MinInt && n <= math.MaxInt {
		*v = int(n)
		return nil
	}
	f, err := tok.Float()
	if err != nil {
		return err
	}
	*v = f
	return nil
}))
