package encounter

import "fmt"

// MoveKind is how a body moved on a step: on its own legs by its own choice,
// on its own legs under somebody else's order, or moved without its legs
// (rpg-project#825, design R1).
//
// # Whoever decides the walk names the kind
//
// A creature's own [Move] and a player's [Encounter.Step] are [MoveOwn]. A
// [Routed] intent carries its kind, the zero value meaning own: a table chase,
// search or patrol rejoin is the creature's own walk, and only a compulsion's
// intent names [MoveCompelled]. [Encounter.Direct] takes the kind its caller
// gives it and refuses none. A cause names WHAT moved the body; it never names
// the kind.
//
// # The zero value is own
//
// A stored beat with no kind is an own walk, and a [MoveStep] nobody filled in
// is a walk somebody chose, which provokes. [MoveKind.Displaced] is the only
// question a reader that suppresses something may ask, and it is true for
// exactly one word, so nothing can switch a reaction off by forgetting.
//
// What a kind MEANS for a reaction is the rules layer's (C1): this module
// carries it and does not act on it. What it means for FACING is this
// module's, because facing is member state the engine writes (facing.go):
// legs turn, so every kind but displaced turns along its steps
// (rpg-project#829, design R2).
type MoveKind string

const (
	// MoveOwn is a walk the creature chose. The zero value reads the same.
	MoveOwn MoveKind = "own"

	// MoveCompelled is the creature's own legs under somebody else's order:
	// a Command flee, a Dissonant Whispers rout. It walks, so it provokes.
	MoveCompelled MoveKind = "compelled"

	// MoveDisplaced is a body moved without its legs: a Thunderwave push.
	// Nobody chose to leave anybody's reach, so it never provokes.
	MoveDisplaced MoveKind = "displaced"
)

// Displaced reports whether this step moved the body without its legs. It is
// true only for [MoveDisplaced]; the zero value and every other word are a walk.
func (k MoveKind) Displaced() bool { return k == MoveDisplaced }

// Own reports whether this is the creature's own walk: [MoveOwn] or the zero
// value.
func (k MoveKind) Own() bool { return k == "" || k == MoveOwn }

// named is k with the zero value spelled own, so a [Mover] is told the same
// word for an own walk before a reload and after one (a stored pause reads
// its kind back as a word).
func (k MoveKind) named() MoveKind {
	if k == "" {
		return MoveOwn
	}
	return k
}

// known reports whether k is a word this module writes, the zero value
// included (it reads as own).
func (k MoveKind) known() bool {
	return k.Own() || k == MoveCompelled || k == MoveDisplaced
}

// beatWord is the word a moved beat carries: compelled or displaced, and
// empty for own, so every own record keeps the shape it always had.
func (k MoveKind) beatWord() string {
	if k.Own() {
		return ""
	}
	return string(k)
}

// moveKindFromData reads a stored kind: a stored word wins; a record from
// before the kind derives it from `forced` alone (true is displaced, else
// own). An unknown word is refused rather than guessed (fail closed).
func moveKindFromData(word string, forced bool) (MoveKind, error) {
	if word != "" {
		k := MoveKind(word)
		if !k.known() {
			return "", fmt.Errorf("move kind %q: %w", word, ErrBadMoveKind)
		}
		if forced != k.Displaced() {
			return "", fmt.Errorf("move kind %q disagrees with forced=%t: %w", word, forced, ErrBadMoveKind)
		}
		return k, nil
	}
	if forced {
		return MoveDisplaced, nil
	}
	return MoveOwn, nil
}
