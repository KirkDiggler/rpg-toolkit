// Copyright (C) 2026 Kirk Diggler
// SPDX-License-Identifier: GPL-3.0-or-later

package dungeonspec

import (
	"fmt"
	"strings"

	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/encounter"
)

// factions.go is WHO FIGHTS WHOM, IN THE FILE (rpg-project#375, the hold-out
// design §2): the factions an author declares, the placements in them, the
// dispositions between them, and every predicate an `until` may carry —
// each defect reported at the YAML path of the thing that is wrong, in the
// words a form-filler can act on.
//
// The run's own compiler refuses the same shapes (encounter/disposition.go),
// in the composition's words; this file exists so the builder can draw the
// refusal at the field it names, and so every defect is reported rather
// than the first.

// stances is the closed stance vocabulary, the words [encounter.Stance]
// carries.
var stances = map[string]bool{
	string(encounter.StanceHostile): true,
	string(encounter.StanceNeutral): true,
	string(encounter.StanceAllied):  true,
}

// stanceWords is the one sentence every stance refusal points at.
const stanceWords = "hostile, neutral or allied"

// factions checks the declared factions: an id, not the reserved `party`,
// no id twice. RUN BEFORE place(), which asks whether a placement's faction
// exists.
func (v *validation) factions() {
	v.factionIDs = map[string]int{}
	v.factionMembers = map[string][]int{}
	v.mindValid = map[string]bool{}
	for i, fa := range v.spec.Factions {
		p := fmt.Sprintf("factions[%d]", i)
		switch fa.ID {
		case "":
			v.fail(p+".id", "the faction has no id")
		case encounter.FactionParty:
			v.fail(p+".id", "`party` is the players' side and is never declared — name the faction the monsters are in")
		default:
			if prev, dup := v.factionIDs[fa.ID]; dup {
				v.fail(p+".id", "faction %q is already declared at factions[%d]", fa.ID, prev)
			} else {
				v.factionIDs[fa.ID] = i
			}
		}
	}
}

// factionExists reports whether an id names a faction this file has: one
// of the two reserved ones, or one it declared.
func (v *validation) factionExists(id string) bool {
	if id == encounter.FactionParty || id == encounter.FactionMonsters {
		return true
	}
	_, ok := v.factionIDs[id]
	return ok
}

// placementFaction is the faction a monster placement is in: the one it
// named, or the reserved `monsters`.
func placementFaction(pl PlaceSpec) string {
	if pl.Faction == "" {
		return encounter.FactionMonsters
	}
	return pl.Faction
}

// placeFaction checks one placement's `faction` (called from place(), which
// owns the placement loop): a prop cannot have one; a monster's must exist
// and must not be `party`. Records the membership for the faction-of-one
// rule either way.
func (v *validation) placeFaction(p string, i int, pl PlaceSpec, kind string) {
	if kind != typeMonsters {
		if pl.Faction != "" {
			v.fail(p+".faction", "%q is not a monster and cannot be in a faction", pl.Ref)
		}
		return
	}
	switch {
	case pl.Faction == encounter.FactionParty:
		v.fail(p+".faction", "%q cannot be in `party`: that is the players' side", pl.Ref)
		return
	case pl.Faction != "" && !v.factionExists(pl.Faction):
		v.fail(p+".faction", "%q is in faction %q, and no faction in this dungeon has that id — declare it under `factions:`",
			pl.Ref, pl.Faction)
		return
	}
	faction := placementFaction(pl)
	v.factionMembers[faction] = append(v.factionMembers[faction], i)
}

// minds checks every declared mind: a placement that exists, a monster, and
// one standing in its own faction. RUN AFTER place(), which built the
// placement index and the memberships.
func (v *validation) minds() {
	for i, fa := range v.spec.Factions {
		if fa.Mind == "" {
			continue
		}
		p := fmt.Sprintf("factions[%d].mind", i)
		idx, ok := v.placeIDs[fa.Mind]
		if !ok {
			v.fail(p, "faction %q names %q as its mind, and no placement in this dungeon has that id", fa.ID, fa.Mind)
			continue
		}
		pl := v.spec.Place[idx]
		if kind, _ := refKind(pl.Ref); kind != typeMonsters {
			v.fail(p, "faction %q names %q as its mind, and %q is a prop — a mind is a monster in the faction",
				fa.ID, fa.Mind, pl.Ref)
			continue
		}
		if placementFaction(pl) != fa.ID {
			v.fail(p, "faction %q names %q as its mind, but %q is in faction %q — a mind is a monster in its own faction",
				fa.ID, fa.Mind, pl.Ref, placementFaction(pl))
			continue
		}
		v.mindValid[fa.ID] = true
	}
}

// cannotLearn reports why a faction cannot come to know a fact, or "" when
// it can: it has a valid mind, or it is a faction of one — whose sole
// member is its mind by rule. `party` never learns.
func (v *validation) cannotLearn(id string) string {
	if id == encounter.FactionParty {
		return "`party` is the players' side and has no mind"
	}
	if v.mindValid[id] {
		return ""
	}
	switch n := len(v.factionMembers[id]); n {
	case 1:
		return ""
	case 0:
		return fmt.Sprintf("faction %q has nobody in it", id)
	default:
		return fmt.Sprintf("faction %q has %d monsters and no mind", id, n)
	}
}

// normalizedPair is the one key both orders of a pair share.
func normalizedPair(a, b string) [2]string {
	if b < a {
		a, b = b, a
	}
	return [2]string{a, b}
}

// declaredStance is the stance a pair was authored with, or its default,
// and the predicate that ends it — nil for a static stance.
func (v *validation) declaredStance(pair [2]string) (string, *PredicateSpec) {
	if idx, ok := v.dispositionAt[pair]; ok {
		return v.spec.Dispositions[idx].Stance, v.spec.Dispositions[idx].Until
	}
	return string(encounter.DefaultStance(pair[0], pair[1])), nil
}

// dispositions checks every declared disposition, in two passes: the pairs
// and stances first, so the whole table is indexed; then every until as a
// predicate against that table, and the fact-untils against who can learn.
// RUN AFTER place() and minds(): a `{ down }` names a placement, and the
// faction-of-one rule counts them.
func (v *validation) dispositions() {
	v.dispositionAt = map[[2]string]int{}
	for i, d := range v.spec.Dispositions {
		p := fmt.Sprintf("dispositions[%d]", i)
		pairOK := true
		for j, id := range d.Between {
			bp := fmt.Sprintf("%s.between[%d]", p, j)
			switch {
			case id == "":
				v.fail(bp, "the disposition does not say which faction")
				pairOK = false
			case !v.factionExists(id):
				v.fail(bp, "%q is not a faction in this dungeon — declare it under `factions:`, or write `party`", id)
				pairOK = false
			}
		}
		if pairOK && d.Between[0] == d.Between[1] {
			v.fail(p+".between", "a disposition is between two different factions, and this one names %q twice", d.Between[0])
			pairOK = false
		}
		switch {
		case d.Stance == "":
			v.fail(p+".stance", "the disposition does not say its stance: %s", stanceWords)
		case !stances[d.Stance]:
			v.fail(p+".stance", "%q is not a stance: %s", d.Stance, stanceWords)
		case d.Until != nil && d.Stance != string(encounter.StanceHostile):
			v.fail(p+".until", "only a hostile pair has something to stop doing: this pair is %s, so drop the until or make it hostile",
				d.Stance)
		}
		if !pairOK {
			continue
		}
		key := normalizedPair(d.Between[0], d.Between[1])
		if prev, dup := v.dispositionAt[key]; dup {
			v.fail(p+".between", "%s and %s already have a disposition at dispositions[%d], and one pair has one",
				key[0], key[1], prev)
			continue
		}
		v.dispositionAt[key] = i
	}

	for i, d := range v.spec.Dispositions {
		if d.Until == nil {
			continue
		}
		key := normalizedPair(d.Between[0], d.Between[1])
		if idx, ok := v.dispositionAt[key]; !ok || idx != i {
			continue // a pair that failed above has nothing to wait on
		}
		p := fmt.Sprintf("dispositions[%d].until", i)
		v.predicate(p, d.Until, &key)
		if d.Until.Fact != "" {
			v.requireALearner(p, key)
		}
	}

	v.untilRings()
}

// requireALearner is the mind rule (design §2): a fact-until between two
// factions needs one of them able to come to know the fact — a valid mind,
// or a faction of one. A pair where nobody can learn is an until that can
// never hold.
func (v *validation) requireALearner(p string, pair [2]string) {
	var reasons []string
	for _, id := range pair {
		why := v.cannotLearn(id)
		if why == "" {
			return
		}
		reasons = append(reasons, why)
	}
	v.fail(p, "this until waits for a fact, and %s — name a mind, or the faction cannot learn", strings.Join(reasons, ", and "))
}

// predicate checks one authored predicate at its path: a round that starts,
// a placement that exists and can fall, a stance a pair can actually reach.
// self is the pair an until belongs to, so it cannot wait on itself; nil
// for a predicate that is not an until.
func (v *validation) predicate(path string, p *PredicateSpec, self *[2]string) {
	switch p.Form() {
	case "":
		v.fail(path, "this predicate says nothing — %s", predicateForms)
	case predicateRound:
		if *p.Round < 1 {
			v.fail(path+".round", "round %d: a round starts at 1", *p.Round)
		}
	case predicateDown:
		idx, ok := v.placeIDs[p.Down]
		if !ok {
			v.fail(path+".down", "%q is not a placement in this dungeon", p.Down)
			return
		}
		if kind, _ := refKind(v.spec.Place[idx].Ref); kind != typeMonsters {
			v.fail(path+".down", "%q is a prop, and only a monster can be down", p.Down)
		}
	case predicateFact:
		// Declared by mention: the dungeon allows a fact no record reveals
		// (R8), and the scenario is where "a hold-out nobody can win" is
		// refused.
	case predicateStance:
		v.stancePredicate(path+".stance", p.Stance, self)
	}
}

// stancePredicate checks the stance form: two factions that exist and
// differ, a stance word, and a pair that can reach it — not one it holds
// from the start, and not one it can never hold (design §3.8's liveness
// rule, in the file's own words).
func (v *validation) stancePredicate(path string, s *StancePredicateSpec, self *[2]string) {
	ok := true
	for j, id := range s.Between {
		bp := fmt.Sprintf("%s.between[%d]", path, j)
		switch {
		case id == "":
			v.fail(bp, "the stance does not say which faction")
			ok = false
		case !v.factionExists(id):
			v.fail(bp, "%q is not a faction in this dungeon — declare it under `factions:`, or write `party`", id)
			ok = false
		}
	}
	if ok && s.Between[0] == s.Between[1] {
		v.fail(path+".between", "a stance is between two different factions, and this one names %q twice", s.Between[0])
		ok = false
	}
	if !stances[s.Is] {
		v.fail(path+".is", "%q is not a stance: %s", s.Is, stanceWords)
		ok = false
	}
	if !ok {
		return
	}
	key := normalizedPair(s.Between[0], s.Between[1])
	if self != nil && *self == key {
		v.fail(path, "a disposition cannot wait on its own stance")
		return
	}
	declared, until := v.declaredStance(key)
	switch {
	case declared == s.Is:
		v.fail(path, "%s and %s are %s from the start, so nothing can fire this", key[0], key[1], s.Is)
	case declared == string(encounter.StanceHostile) && until != nil && s.Is == string(encounter.StanceNeutral):
		// Reachable: the pair turns neutral when its until holds.
	default:
		v.fail(path, "%s and %s can never be %s: they are %s, and nothing turns them", key[0], key[1], s.Is, declared)
	}
}

// untilRings refuses dispositions that wait on each other's stance: a ring
// of untils resolves to nothing, so none of them can ever turn. A self-loop
// is stancePredicate's; this is the ring of two or more.
func (v *validation) untilRings() {
	const (
		unseen = iota
		onTrail
		done
	)
	state := map[[2]string]int{}

	var visit func(pair [2]string, trail [][2]string) bool
	visit = func(pair [2]string, trail [][2]string) bool {
		switch state[pair] {
		case done:
			return false
		case onTrail:
			names := make([]string, 0, len(trail)+1)
			for _, t := range trail {
				names = append(names, t[0]+"~"+t[1])
			}
			names = append(names, pair[0]+"~"+pair[1])
			idx := v.dispositionAt[trail[0]]
			v.fail(fmt.Sprintf("dispositions[%d].until.stance", idx),
				"these dispositions wait on each other's stance in a ring (%s), so none of them can ever turn",
				strings.Join(names, " -> "))
			return true
		}
		state[pair] = onTrail
		if _, until := v.declaredStance(pair); until != nil && until.Stance != nil {
			next := normalizedPair(until.Stance.Between[0], until.Stance.Between[1])
			if next != pair && v.factionExists(next[0]) && v.factionExists(next[1]) {
				if visit(next, append(trail, pair)) {
					return true
				}
			}
		}
		state[pair] = done
		return false
	}

	for i, d := range v.spec.Dispositions {
		key := normalizedPair(d.Between[0], d.Between[1])
		if idx, ok := v.dispositionAt[key]; !ok || idx != i {
			continue
		}
		if visit(key, nil) {
			return
		}
	}
}
