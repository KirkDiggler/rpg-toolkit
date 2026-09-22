// Copyright (C) 2026 Kirk Diggler
// SPDX-License-Identifier: GPL-3.0-or-later

package encounter

import (
	"fmt"
)

// disposition.go is WHO FIGHTS WHOM, AS DECLARED (rpg-project#375, the
// hold-out design §2 and §3.1): the factions a field authors, the stance
// between every pair of them, and the predicate that ends a hostility.
//
// This file is the DECLARATION half. It compiles the authored factions and
// dispositions, fills in the defaults that keep every pre-faction dungeon
// unchanged, and answers the construction-time questions — is this a
// faction, what stance were these two declared with, can they ever reach
// that one. The RUN-TIME half — which stance a pair holds right now, after
// whatever the journal says — is a fold over the one world (world.go), and
// it reads this file's answers as its starting point.
//
// # The defaults are the law that keeps today's dungeons today's
//
// Every pair the author did not declare has a stance, stated once here in
// [defaultStance]:
//
//   - a faction is allied with itself;
//   - `party` and `monsters` are mutually hostile — the whole table this
//     composition ran on before factions existed;
//   - a declared faction with no disposition toward `party` is hostile to
//     it — an authored raider camp is a camp, not a neutral crowd;
//   - declared factions are neutral to each other and to `monsters`.
//
// A dungeon that declares nothing therefore has exactly two factions and one
// hostile pair, and every side reader answers as it always did (design A7).

// factionPair is an unordered pair of factions, normalized so {a, b} and
// {b, a} are one key — a disposition is between two factions, not from one
// to the other (directed dispositions are a shelf, design §11).
type factionPair struct {
	a, b FactionID
}

// pairOf normalizes two ids into the one key both orders share.
func pairOf(a, b FactionID) factionPair {
	if b < a {
		a, b = b, a
	}
	return factionPair{a: a, b: b}
}

// String renders the pair the way a refusal names it.
func (p factionPair) String() string { return p.a + " and " + p.b }

// knownStance reports whether a word is one of the closed [Stance] set.
func knownStance(s Stance) bool {
	switch s {
	case StanceHostile, StanceNeutral, StanceAllied:
		return true
	default:
		return false
	}
}

// compileFactions checks the authored factions and dispositions and indexes
// them, refusing every defect by name (ErrNoFaction), then checks every
// `until` predicate against the whole compiled table — a disposition is a
// faction declaration, and its predicate is part of it.
//
// Validation order (first failure wins, R5): per faction — no id, the
// reserved `party`, an id twice; per disposition — a faction that is not
// there, a pair of one, a stance outside the closed set, an until on an
// ALLIED stance, a pair declared twice; then every until as a predicate
// ([field.validatePredicate]), refusing first one that waits on its own
// pair's stance.
func (f *field) compileFactions(factions []FactionInput, dispositions []DispositionInput) error {
	f.factions = append([]FactionInput(nil), factions...)
	f.factionIndex = make(map[FactionID]int, len(factions))
	for i, fa := range factions {
		if fa.ID == "" {
			return fmt.Errorf("factions[%d] has no id: %w", i, ErrNoFaction)
		}
		if fa.ID == FactionParty {
			return fmt.Errorf("factions[%d] declares %q, which is the players' side and is never declared: %w",
				i, FactionParty, ErrNoFaction)
		}
		if prev, dup := f.factionIndex[fa.ID]; dup {
			return fmt.Errorf("factions[%d] and factions[%d] share the id %q: %w", prev, i, fa.ID, ErrNoFaction)
		}
		f.factionIndex[fa.ID] = i
	}

	f.dispositions = append([]DispositionInput(nil), dispositions...)
	f.dispositionOf = make(map[factionPair]int, len(dispositions))
	for i, d := range dispositions {
		for _, id := range d.Between {
			if !f.isFaction(id) {
				return fmt.Errorf("dispositions[%d] names faction %q, which this field does not declare: %w",
					i, id, ErrNoFaction)
			}
		}
		if d.Between[0] == d.Between[1] {
			return fmt.Errorf("dispositions[%d] is between %q and itself, and a faction is always allied with itself: %w",
				i, d.Between[0], ErrNoFaction)
		}
		if !knownStance(d.Stance) {
			return fmt.Errorf("dispositions[%d] declares stance %q, which is not hostile, neutral or allied: %w",
				i, d.Stance, ErrNoFaction)
		}
		if d.Until != nil && d.Stance == StanceAllied {
			return fmt.Errorf("dispositions[%d] is allied and has an until, and %s: %w", i, untilOnAllied, ErrNoFaction)
		}
		pair := pairOf(d.Between[0], d.Between[1])
		if prev, dup := f.dispositionOf[pair]; dup {
			return fmt.Errorf(
				"dispositions[%d] and dispositions[%d] both speak for %s, and one pair has one disposition: %w",
				prev, i, pair, ErrNoFaction)
		}
		f.dispositionOf[pair] = i
	}

	// The predicates. EVERY FORM TURNS A PAIR (rpg-project#493, R2): a fact
	// on the pair's minds, and a round, a fall or another pair's stance on
	// the world's truth. The "only a fact is built" refusal this loop used
	// to open with (hold-out R11, `untilNotBuilt`) is gone: the use case
	// arrived, and the three other forms are evaluated at the sites that
	// already notice their events (flip.go).
	for i, d := range dispositions {
		if d.Until == nil {
			continue
		}
		what := fmt.Sprintf("dispositions[%d]'s until", i)
		if ts, ok := d.Until.(TriggerStance); ok &&
			pairOf(ts.Between[0], ts.Between[1]) == pairOf(d.Between[0], d.Between[1]) {
			return fmt.Errorf("%s: %s: %w", what, untilOnItself, ErrNoFaction)
		}
		if err := f.validatePredicate(what, d.Until, ErrNoFaction); err != nil {
			return err
		}
	}

	return nil
}

// untilOnAllied is the one sentence an `until` on an allied pair gets, in the
// file and in the run alike (rpg-project#493, R1). Hostile and neutral are
// the two a pair moves between; allied is a static, authorable stance with no
// other side to move to.
const untilOnAllied = "an allied pair has nothing to become"

// untilOnItself is the one sentence a disposition waiting on its OWN stance
// gets. It used to be caught by accident — the pair held the stance it waited
// for from the start, which [field.validatePredicate] refuses as unfireable —
// and a neutral pair waiting to become hostile now passes that check honestly
// (R3 makes it reachable), so the rule is stated rather than inferred.
const untilOnItself = "a disposition cannot wait on its own stance"

// turnsTo is the stance a declared one moves to when its `until` holds
// (rpg-project#493, R1): the OTHER of hostile and neutral. Allied never
// turns and never has an until ([untilOnAllied]), so it answers itself.
func turnsTo(declared Stance) Stance {
	switch declared {
	case StanceHostile:
		return StanceNeutral
	case StanceNeutral:
		return StanceHostile
	default:
		return declared
	}
}

// validateMemberFaction refuses a member naming a faction this field does
// not have, and a member arriving under a faction's mind id into some other
// faction — the mind is a monster IN its faction, or it is nobody's mind.
// Asked at construction and at Join, the two ways a member enters
// ([FactionInput.Mind] on why it cannot be asked earlier).
func (f *field) validateMemberFaction(id MemberID, kind MemberKind, faction FactionID) error {
	if faction != "" && !f.isFaction(faction) {
		return fmt.Errorf("member %q names faction %q, which this field does not declare: %w", id, faction, ErrNoFaction)
	}
	resolved := factionOf(&memberRecord{ID: id, Kind: kind, Faction: faction})
	for _, fa := range f.factions {
		if fa.Mind == id && fa.ID != resolved {
			return fmt.Errorf("member %q is the mind of faction %q and joins %q instead: %w",
				id, fa.ID, resolved, ErrNoFaction)
		}
	}
	return nil
}

// isFaction reports whether an id names a faction this field has: one of the
// two reserved ones, or one it declared.
func (f *field) isFaction(id FactionID) bool {
	if id == FactionParty || id == FactionMonsters {
		return true
	}
	_, declared := f.factionIndex[id]
	return declared
}

// declaredStance is the stance a pair was authored with, or its default, and
// the predicate that ends it — nil for a static stance.
func (f *field) declaredStance(pair factionPair) (Stance, Trigger) {
	if i, ok := f.dispositionOf[pair]; ok {
		return f.dispositions[i].Stance, f.dispositions[i].Until
	}
	return defaultStance(pair), nil
}

// defaultStance is the stance of a pair nobody declared — [DefaultStance],
// on the normalized pair.
func defaultStance(pair factionPair) Stance { return DefaultStance(pair.a, pair.b) }

// DefaultStance is the stance of two factions nobody declared a disposition
// between (rpg-project#375, design §2) — the whole of the table this
// composition ran on before factions existed, plus one line for an authored
// faction: it is hostile to the party unless it says otherwise.
//
//   - a faction is allied with itself;
//   - `party` is hostile to every faction that did not say otherwise —
//     `monsters` included, which is the pre-faction world entire;
//   - every other pair is neutral: declared factions to each other, and to
//     `monsters`.
//
// EXPORTED FOR THE AUTHORING DIALECT, which refuses a predicate that waits
// for a stance a pair can never reach and has to know what the pair starts
// at to say so in the file's own path. One rule, read from one place, so the
// file and the run cannot disagree about what "nobody said" means.
func DefaultStance(a, b FactionID) Stance {
	switch {
	case a == b:
		return StanceAllied
	case a == FactionParty || b == FactionParty:
		return StanceHostile
	default:
		return StanceNeutral
	}
}

// stanceReachable reports whether a pair holds a stance from the start (now)
// and whether it can ever hold it (ever).
//
// THREE WAYS A PAIR MOVES, and they are the whole answer (rpg-project#493):
// it is declared that way; its own `until` holds, which turns it to the other
// of hostile and neutral ([turnsTo], R1); or it is NEUTRAL and somebody
// attacks across it, which turns it hostile with nothing authored at all
// (R3). The third is why `{ stance: { between: [a, b], is: hostile } }` over
// a neutral pair is never refused as unreachable any more: every neutral pair
// can come to blows.
func (f *field) stanceReachable(pair factionPair, s Stance) (now, ever bool) {
	declared, until := f.declaredStance(pair)
	now = declared == s
	switch {
	case now:
		ever = true
	case until != nil && s == turnsTo(declared):
		ever = true
	case declared == StanceNeutral && s == StanceHostile:
		ever = true
	}
	return now, ever
}

// validatePredicate refuses a predicate that is not one, or one that can
// never hold — the liveness argument [ErrNoEnding] makes for an unreachable
// trigger cell, applied to every form of the grammar (design §3.8). what is
// the caller's noun for the refusal ("ending \"turned\"", "dispositions[0]'s
// until"); sentinel is the caller's own.
//
// The three forms that already have endings — a cell, a member down, an
// exit held — keep their checks where they were (validateEndingTriggers);
// this is the forms the grammar adds.
func (f *field) validatePredicate(what string, t Trigger, sentinel error) error {
	switch p := t.(type) {
	case TriggerRound:
		if p.Round < 1 {
			return fmt.Errorf("%s waits for round %d, and a round is counted from 1: %w", what, p.Round, sentinel)
		}
	case TriggerFact:
		if p.Fact == "" {
			return fmt.Errorf("%s names no fact: %w", what, sentinel)
		}
	case TriggerMemberDown:
		if p.Member == "" {
			return fmt.Errorf("%s names no member: %w", what, sentinel)
		}
	case TriggerStance:
		for _, id := range p.Between {
			if !f.isFaction(id) {
				return fmt.Errorf("%s waits on faction %q, which this field does not declare: %w", what, id, sentinel)
			}
		}
		if p.Between[0] == p.Between[1] {
			return fmt.Errorf("%s waits on a stance between %q and itself: %w", what, p.Between[0], sentinel)
		}
		if !knownStance(p.Stance) {
			return fmt.Errorf("%s waits for stance %q, which is not hostile, neutral or allied: %w",
				what, p.Stance, sentinel)
		}
		pair := pairOf(p.Between[0], p.Between[1])
		now, ever := f.stanceReachable(pair, p.Stance)
		if !ever {
			return fmt.Errorf("%s waits for %s to be %s, which they can never be: %w", what, pair, p.Stance, sentinel)
		}
		if now {
			return fmt.Errorf("%s waits for %s to be %s, which they are from the start, so nothing can fire it: %w",
				what, pair, p.Stance, sentinel)
		}
	}
	return nil
}
