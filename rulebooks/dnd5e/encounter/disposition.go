// Copyright (C) 2026 Kirk Diggler
// SPDX-License-Identifier: GPL-3.0-or-later

package encounter

import (
	"fmt"
	"strings"
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
//     it — an authored goblin camp is a camp, not a neutral crowd;
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
// there, a pair of one, a stance outside the closed set, an until on a
// stance that is not hostile, a pair declared twice; then every until as a
// predicate ([field.validatePredicate]); then a ring of untils waiting on
// each other's stance.
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
		if d.Until != nil && d.Stance != StanceHostile {
			return fmt.Errorf(
				"dispositions[%d] is %s and has an until, and only a hostile pair has something to stop doing: %w",
				i, d.Stance, ErrNoFaction)
		}
		pair := pairOf(d.Between[0], d.Between[1])
		if prev, dup := f.dispositionOf[pair]; dup {
			return fmt.Errorf(
				"dispositions[%d] and dispositions[%d] both speak for %s, and one pair has one disposition: %w",
				prev, i, pair, ErrNoFaction)
		}
		f.dispositionOf[pair] = i
	}

	// The predicates, against the whole table: an until that waits on
	// another pair's stance needs every pair indexed before it can be
	// judged.
	for i, d := range dispositions {
		if d.Until == nil {
			continue
		}
		pair := pairOf(d.Between[0], d.Between[1])
		what := fmt.Sprintf("dispositions[%d]'s until", i)
		if err := f.validatePredicate(what, d.Until, &pair, ErrNoFaction); err != nil {
			return err
		}
	}

	return f.refuseUntilRings()
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
// and whether it can ever hold it (ever): the declared stance, or neutral
// once a hostile pair's until holds — the only change there is (R2).
func (f *field) stanceReachable(pair factionPair, s Stance) (now, ever bool) {
	declared, until := f.declaredStance(pair)
	now = declared == s
	ever = now || (declared == StanceHostile && until != nil && s == StanceNeutral)
	return now, ever
}

// validatePredicate refuses a predicate that is not one, or one that can
// never hold — the liveness argument [ErrNoEnding] makes for an unreachable
// trigger cell, applied to every form of the grammar (design §3.8). what is
// the caller's noun for the refusal ("ending \"turned\"", "dispositions[0]'s
// until"); self is the pair an until belongs to, so it cannot wait on
// itself; sentinel is the caller's own.
//
// The three forms that already have endings — a cell, a member down, an
// exit held — keep their checks where they were (validateEndingTriggers);
// this is the four forms the grammar adds, and the refusal for a trigger
// that is not a predicate at all on a field that asked for one.
func (f *field) validatePredicate(what string, t Trigger, self *factionPair, sentinel error) error {
	switch p := t.(type) {
	case TriggerRound:
		if p.Round < 1 {
			return fmt.Errorf("%s waits for round %d, and a round starts at 1: %w", what, p.Round, sentinel)
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
		if self != nil && *self == pair {
			return fmt.Errorf("%s waits on its own stance: %w", what, sentinel)
		}
		now, ever := f.stanceReachable(pair, p.Stance)
		if !ever {
			return fmt.Errorf("%s waits for %s to be %s, which they can never be: %w", what, pair, p.Stance, sentinel)
		}
		if now {
			return fmt.Errorf("%s waits for %s to be %s, which they are from the start, so nothing can fire it: %w",
				what, pair, p.Stance, sentinel)
		}
	default:
		if self != nil {
			return fmt.Errorf("%s is a %T, which is not a predicate — write round, down, fact or stance: %w",
				what, t, sentinel)
		}
	}
	return nil
}

// refuseUntilRings refuses dispositions that wait on each other's stance: a
// ring of untils resolves to nothing, which is an until that can never hold
// — the liveness hole an unreachable ending is (design §3.8). A self-loop is
// [field.validatePredicate]'s; this is the ring of two or more.
func (f *field) refuseUntilRings() error {
	const (
		unseen = iota
		onTrail
		done
	)
	state := make(map[factionPair]int, len(f.dispositions))

	var visit func(pair factionPair, trail []factionPair) error
	visit = func(pair factionPair, trail []factionPair) error {
		switch state[pair] {
		case done:
			return nil
		case onTrail:
			names := make([]string, 0, len(trail)+1)
			for _, p := range trail {
				names = append(names, p.String())
			}
			names = append(names, pair.String())
			return fmt.Errorf("dispositions wait on each other's stance in a ring (%s), so none of them can ever turn: %w",
				strings.Join(names, " -> "), ErrNoFaction)
		}
		state[pair] = onTrail
		if _, until := f.declaredStance(pair); until != nil {
			if ts, ok := until.(TriggerStance); ok {
				if err := visit(pairOf(ts.Between[0], ts.Between[1]), append(trail, pair)); err != nil {
					return err
				}
			}
		}
		state[pair] = done
		return nil
	}

	for _, d := range f.dispositions {
		if err := visit(pairOf(d.Between[0], d.Between[1]), nil); err != nil {
			return err
		}
	}
	return nil
}
