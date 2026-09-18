// Copyright (C) 2026 Kirk Diggler
// SPDX-License-Identifier: GPL-3.0-or-later

package table

import (
	"fmt"
	"strings"
)

// Temper is a creature's temperament: a sealed word naming a weight profile,
// and nothing else.
//
// It adds no entries where a table is silent, it has no triggers of its own,
// and it holds no memory. The moment it grows any of those it is the mind
// ladder again wearing a new word, and the ladder is what this replaces. Four
// goblins off one sheet with one table are four different creatures because
// their dice are loaded differently, not because they were given different
// orders (design §3, R5).
//
// It is authored on a placement, or dealt from a mix on the faction at spawn.
// It is never on the monster's sheet — a sheet says what a creature IS, and
// two goblins are the same goblin.
type Temper string

// Temper constants. These three are the whole vocabulary; a fourth is a
// design decision, not a constant someone adds.
const (
	// TemperSoldier is the zero profile: every word at 100, a creature that
	// simply does what its orders say. An absent temperament means this
	// one.
	TemperSoldier Temper = "soldier"

	// TemperCoward would rather be elsewhere: it halves what makes it close
	// or strike and trebles what takes it away, so the same orders that
	// make a soldier fight make it run. It still holds as readily as anyone
	// — cowardice is about direction, not idleness.
	TemperCoward Temper = "coward"

	// TemperAggressive is the mirror: it trebles closing and striking,
	// quarters retreat, and halves holding, so it is the one that comes at
	// you off a table that only suggested it.
	TemperAggressive Temper = "aggressive"
)

// Profile is the weight profile a temperament loads onto a table's words, in
// PERCENT: 100 leaves an authored weight as written, 300 trebles it, 25
// quarters it. A word not named here — `fact` — is 100, because a
// temperament changes what a creature is inclined to DO and learning a fact
// is not something it chooses.
//
// The zero value is not a soldier. All-zero would silence every word on the
// table, which is the opposite of what "no temperament named" means, so a
// caller that wants the even profile asks for it by name:
// TemperSoldier.Profile().
type Profile struct {
	// Attack multiplies entries whose word is `attack`.
	Attack int
	// Toward multiplies entries whose word is `toward`.
	Toward int
	// Away multiplies entries whose word is `away`.
	Away int
	// Flee multiplies entries whose word is `flee`.
	Flee int
	// Hold multiplies entries whose word is `hold`.
	Hold int
}

// profiles is the content: the design's §3 table, in percent. A walk tunes
// these numbers, which is the argument for them living here as data rather
// than inside whatever does the arithmetic.
var profiles = map[Temper]Profile{
	TemperSoldier:    {Attack: 100, Toward: 100, Away: 100, Flee: 100, Hold: 100},
	TemperCoward:     {Attack: 50, Toward: 50, Away: 300, Flee: 300, Hold: 100},
	TemperAggressive: {Attack: 300, Toward: 300, Away: 25, Flee: 25, Hold: 50},
}

// Profile returns the weight profile this temperament loads onto a table's
// words.
//
// The empty Temper is a caller that named no temperament, and the design says
// that is a soldier: it answers the even profile with no error. That is the
// only word this method supplies on a caller's behalf.
//
// Every other word it does not know is REFUSED. Answering "solider" with a
// soldier's profile would be a silent degrade — a placement whose `temper:`
// was mistyped would play as a perfectly well-behaved creature and nobody
// would ever learn the word never landed. A refusal is the one thing that
// cannot be mistaken for the author getting what they asked for.
//
// The returned Profile is the zero value on refusal and must not be used; it
// is not a soldier, and reading it as one would weight nothing and silence
// every word on the creature's table.
func (t Temper) Profile() (Profile, error) {
	if t == "" {
		return profiles[TemperSoldier], nil
	}

	if p, ok := profiles[t]; ok {
		return p, nil
	}

	return Profile{}, fmt.Errorf("invalid temper %q (must be one of %s)", string(t), vocabulary())
}

// String is the author-facing label, the inverse of ParseTemper.
func (t Temper) String() string {
	return string(t)
}

// authorable is the vocabulary ParseTemper accepts, in the order its refusal
// lists them: the even profile first, then the two that lean off it.
var authorable = []Temper{TemperSoldier, TemperCoward, TemperAggressive}

// ParseTemper parses the author-facing temperament vocabulary ("soldier",
// "coward", "aggressive") into a Temper.
//
// The empty string is refused along with every unknown word. An absent
// temperament is the CALLER's zero to supply, not this parser's to invent:
// the author who wrote nothing never called this, and a parser that answered
// "soldier" to "" could not tell that author apart from one who wrote
// "solider".
func ParseTemper(s string) (Temper, error) {
	for _, temper := range authorable {
		if Temper(s) == temper {
			return temper, nil
		}
	}

	return "", fmt.Errorf("invalid temper %q (must be one of %s)", s, vocabulary())
}

// vocabulary renders the authorable words for a refusal. ParseTemper and
// Profile share it so the two doors into this package refuse in the same
// words — an author who mistyped should not have to work out which of two
// different messages was about their placement.
func vocabulary() string {
	words := make([]string, len(authorable))
	for i, temper := range authorable {
		words[i] = fmt.Sprintf("%q", temper)
	}

	return strings.Join(words, ", ")
}
