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

// Beat names something that happens which a transition may be on. The
// vocabulary supplies the names; the machine gives none of them a meaning.
type Beat string

// FactName names a fact the vocabulary can read from the host's env.
type FactName string

// Recipe is a machine as data: the initial state, the states with the
// transitions each listens to, and the bag's declared keys with their zero
// values. Every field is a value, so a recipe written as a Go literal and
// the same recipe written as JSON are one shape. Compile checks it against
// a Vocabulary and binds it into a Program.
type Recipe struct {
	Ref     core.Ref         `json:"ref"`
	Initial string           `json:"initial"`
	Bag     Bag              `json:"bag,omitzero"`
	States  map[string]State `json:"states"`
}

// State is one state of a recipe: what it listens to. Transitions are asked
// in order on each beat and the first whose guard applies fires. A state
// with no transitions is a sink; a Terminal one ends the machine and may
// carry none.
type State struct {
	Transitions []Transition `json:"transitions,omitzero"`
	Terminal    bool         `json:"terminal,omitzero"`
}

// Transition fires on the beat On when its guard When applies (a nil When
// always applies): its acts Do run in order and, if To names a state, the
// machine enters it. Reason travels on the Stepped result so a host can say
// why.
type Transition struct {
	On     Beat   `json:"on"`
	When   *Guard `json:"when,omitzero"`
	To     string `json:"to,omitzero"`
	Do     []Act  `json:"do,omitzero"`
	Reason string `json:"reason,omitzero"`
}

// Act is one thing a transition does. Exactly one field is set.
//
//   - Pick names a key for the host to act on (a table key, for a creature);
//     the machine returns it and does nothing else with it.
//   - WakeIn arms the blob's wake n rounds after now; n is above zero.
//   - Set writes one declared bag key.
//   - Clear returns each named declared key to its zero value.
//   - Anchor stamps the blob's anchor with now, which a rounds leaf counts
//     from.
type Act struct {
	Pick   string   `json:"pick,omitzero"`
	WakeIn int      `json:"wake_in,omitzero"`
	Set    *SetAct  `json:"set,omitzero"`
	Clear  []string `json:"clear,omitzero"`
	Anchor bool     `json:"anchor,omitzero"`
}

// SetAct writes Value to the declared bag key Key; Value has the key's kind.
type SetAct struct {
	Key   string `json:"key"`
	Value any    `json:"value"`
}

// Bag is the state a machine's guards and acts share beyond its current
// state. Values are bool, int or string. A recipe's bag declares every key
// with its zero value; a blob's bag carries only the keys that differ from
// their zero.
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

// MarshalJSONTo encodes the recipe through an addressable copy, so its ref
// is written in its string form even when the recipe is marshaled by value.
func (r Recipe) MarshalJSONTo(enc *jsontext.Encoder) error {
	type plain Recipe
	p := plain(r)
	return json.MarshalEncode(enc, &p)
}

// UnmarshalJSONFrom decodes a recipe, landing whole JSON numbers in the bag
// and in leaf and set values as int so a decoded recipe equals its Go
// literal. It returns the decoder's error for malformed JSON or a bad ref.
func (r *Recipe) UnmarshalJSONFrom(dec *jsontext.Decoder) error {
	type plain Recipe
	return json.UnmarshalDecode(dec, (*plain)(r), wholeNumbers)
}

// UnmarshalJSONFrom decodes a set act, landing a whole JSON number Value as
// int so it matches an int bag key.
func (a *SetAct) UnmarshalJSONFrom(dec *jsontext.Decoder) error {
	type plain SetAct
	return json.UnmarshalDecode(dec, (*plain)(a), wholeNumbers)
}

// wholeNumbers decodes a JSON number held in an `any` as int when it is a
// whole number in range, and as float64 otherwise, which Compile refuses.
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
