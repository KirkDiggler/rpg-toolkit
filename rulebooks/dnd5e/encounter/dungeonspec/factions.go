// Copyright (C) 2026 Kirk Diggler
// SPDX-License-Identifier: GPL-3.0-or-later

package dungeonspec

import (
	"fmt"
	"strings"

	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/encounter"
)

// factions.go is WHO FIGHTS WHOM, IN THE FILE (rpg-project#375, the hold-out
// design §2): the factions an author declares, the members in them, the
// dispositions between them, and every predicate an `until` may carry —
// each defect reported at the YAML path of the thing that is wrong, in the
// words a form-filler can act on.
//
// THE FACTION HALF OF THE ONE GAMEPLAY GRAMMAR (rpg-project#484). Every
// function here is a method on [grammar], reads no document, and asks about
// the cast through [members] — so the same refusals reach an author whichever
// dialect drew the room. grammar.go holds the value, its two inputs, and the
// orders half.
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
func (g *grammar) factions() {
	g.factionIDs = map[string]int{}
	g.factionMembers = map[string][]int{}
	g.mindValid = map[string]bool{}
	for i, fa := range g.declaredFactions {
		p := fmt.Sprintf("factions[%d]", i)
		switch fa.ID {
		case "":
			g.fail(p+".id", "the faction has no id")
		case encounter.FactionParty:
			g.fail(p+".id", "`party` is the players' side and is never declared — name the faction the monsters are in")
		default:
			if prev, dup := g.factionIDs[fa.ID]; dup {
				g.fail(p+".id", "faction %q is already declared at factions[%d]", fa.ID, prev)
			} else {
				g.factionIDs[fa.ID] = i
			}
		}
	}
}

// factionExists reports whether an id names a faction this file has: one
// of the two reserved ones, or one it declared.
func (g *grammar) factionExists(id string) bool {
	if id == encounter.FactionParty || id == encounter.FactionMonsters {
		return true
	}
	_, ok := g.factionIDs[id]
	return ok
}

// placeFaction checks one member's `faction` (called from the dialect, which
// owns the placement loop): a prop cannot have one; a monster's must exist
// and must not be `party`. Records the membership for the faction-of-one
// rule either way.
//
// THE KIND IS THE CALLER'S ANSWER, not one read back off the ref: a dialect
// that places only monsters says so, and a dialect that has already refused a
// malformed ref by name does not get a second defect for it here.
func (g *grammar) placeFaction(p string, i int, m member, kind string) {
	if kind != typeMonsters {
		if m.faction != "" {
			g.fail(p+".faction", "%q is not a monster and cannot be in a faction", m.ref)
		}
		return
	}
	switch {
	case m.faction == encounter.FactionParty:
		g.fail(p+".faction", "%q cannot be in `party`: that is the players' side", m.ref)
		return
	case m.faction != "" && !g.factionExists(m.faction):
		g.fail(p+".faction", "%q is in faction %q, and no faction in this dungeon has that id — declare it under `factions:`",
			m.ref, m.faction)
		return
	}
	g.factionMembers[m.side()] = append(g.factionMembers[m.side()], i)
}

// minds checks every declared mind: a placement that exists, a monster, and
// one standing in its own faction. RUN AFTER place(), which built the
// placement index and the memberships.
func (g *grammar) minds() {
	for i, fa := range g.declaredFactions {
		if fa.Mind == "" {
			continue
		}
		p := fmt.Sprintf("factions[%d].mind", i)
		m, ok := g.members.at(fa.Mind)
		if !ok {
			g.fail(p, "faction %q names %q as its mind, and no placement in this dungeon has that id", fa.ID, fa.Mind)
			continue
		}
		if !m.isMonster() {
			g.fail(p, "faction %q names %q as its mind, and %q is a prop — a mind is a monster in the faction",
				fa.ID, fa.Mind, m.ref)
			continue
		}
		if m.side() != fa.ID {
			g.fail(p, "faction %q names %q as its mind, but %q is in faction %q — a mind is a monster in its own faction",
				fa.ID, fa.Mind, m.ref, m.side())
			continue
		}
		g.mindValid[fa.ID] = true
	}
}

// cannotLearn reports why a faction cannot come to know a fact, or "" when
// it can: it has a valid mind, or it is a DECLARED faction of one — whose
// sole member the compiler declares as its mind ([singletonMind]). `party`
// never learns; `monsters` learns only once declared, which is how the
// unauthored side is given a mind (design §2, R4).
func (g *grammar) cannotLearn(id string) string {
	if id == encounter.FactionParty {
		return "`party` is the players' side and has no mind"
	}
	if g.mindValid[id] {
		return ""
	}
	if _, declared := g.factionIDs[id]; !declared {
		return fmt.Sprintf("faction %q is not declared, so it has no mind — declare it under `factions:` to give it one", id)
	}
	switch roster := g.factionMembers[id]; len(roster) {
	case 1:
		if g.members.all[roster[0]].id == "" {
			return fmt.Sprintf("faction %q's one monster has no id to be its mind", id)
		}
		return ""
	case 0:
		return fmt.Sprintf("faction %q has nobody in it", id)
	default:
		return fmt.Sprintf("faction %q has %d monsters and no mind", id, len(roster))
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
func (g *grammar) declaredStance(pair [2]string) (string, *PredicateSpec) {
	if idx, ok := g.dispositionAt[pair]; ok {
		return g.declaredDispositions[idx].Stance, g.declaredDispositions[idx].Until
	}
	return string(encounter.DefaultStance(pair[0], pair[1])), nil
}

// dispositions checks every declared disposition, in two passes: the pairs
// and stances first, so the whole table is indexed; then every until as a
// predicate against that table, and the fact-untils against who can learn.
// RUN AFTER place() and minds(): a `{ down }` names a placement, and the
// faction-of-one rule counts them.
func (g *grammar) dispositions() {
	g.dispositionAt = map[[2]string]int{}
	for i, d := range g.declaredDispositions {
		p := fmt.Sprintf("dispositions[%d]", i)
		pairOK := true
		for j, id := range d.Between {
			bp := fmt.Sprintf("%s.between[%d]", p, j)
			switch {
			case id == "":
				g.fail(bp, "the disposition does not say which faction")
				pairOK = false
			case !g.factionExists(id):
				g.fail(bp, "%q is not a faction in this dungeon — declare it under `factions:`, or write `party`", id)
				pairOK = false
			}
		}
		if pairOK && d.Between[0] == d.Between[1] {
			g.fail(p+".between", "a disposition is between two different factions, and this one names %q twice", d.Between[0])
			pairOK = false
		}
		switch {
		case d.Stance == "":
			g.fail(p+".stance", "the disposition does not say its stance: %s", stanceWords)
		case !stances[d.Stance]:
			g.fail(p+".stance", "%q is not a stance: %s", d.Stance, stanceWords)
		case d.Until != nil && d.Stance != string(encounter.StanceHostile):
			g.fail(p+".until", "only a hostile pair has something to stop doing: this pair is %s, so drop the until or make it hostile",
				d.Stance)
		}
		if !pairOK {
			continue
		}
		key := normalizedPair(d.Between[0], d.Between[1])
		if prev, dup := g.dispositionAt[key]; dup {
			g.fail(p+".between", "%s and %s already have a disposition at dispositions[%d], and one pair has one",
				key[0], key[1], prev)
			continue
		}
		g.dispositionAt[key] = i
	}

	for i, d := range g.declaredDispositions {
		if d.Until == nil {
			continue
		}
		key := normalizedPair(d.Between[0], d.Between[1])
		if idx, ok := g.dispositionAt[key]; !ok || idx != i {
			continue // a pair that failed above has nothing to wait on
		}
		p := fmt.Sprintf("dispositions[%d].until", i)
		// ONLY A FACT TURNS A PAIR IN THIS VERSION (rpg-project#375, R11):
		// the run settles a pair on a journal fact, and rounds, falls and
		// other pairs' stances are not journal facts yet. Refused here, at
		// the line, and again by the run at construction.
		if d.Until.Fact == "" {
			g.fail(p, "%s", untilNotBuilt)
			continue
		}
		g.predicate(p, d.Until, &key)
		g.requireALearner(p, key)
	}
}

// untilNotBuilt is the one sentence a round, a fall or a stance on an until
// gets, in the file and in the run alike.
const untilNotBuilt = "in this version a disposition turns only on a fact; " +
	"`until` on a round, a fall, or another stance is not built yet"

// requireALearner is the mind rule (design §2): a fact-until between two
// factions needs one of them able to come to know the fact — a valid mind,
// or a faction of one. A pair where nobody can learn is an until that can
// never hold.
func (g *grammar) requireALearner(p string, pair [2]string) {
	var reasons []string
	for _, id := range pair {
		why := g.cannotLearn(id)
		if why == "" {
			return
		}
		reasons = append(reasons, why)
	}
	g.fail(p, "this until waits for a fact, and %s — name a mind, or the faction cannot learn", strings.Join(reasons, ", and "))
}

// predicate checks one authored predicate at its path: a round that starts,
// a placement that exists and can fall, a stance a pair can actually reach.
// self is the pair an until belongs to, so it cannot wait on itself; nil
// for a predicate that is not an until.
func (g *grammar) predicate(path string, p *PredicateSpec, self *[2]string) {
	switch p.Form() {
	case "":
		g.fail(path, "this predicate says nothing — %s", predicateForms)
	case predicateRound:
		if *p.Round < 1 {
			g.fail(path+".round", "round %d: a round is counted from 1", *p.Round)
		}
	case predicateDown:
		m, ok := g.members.at(p.Down)
		if !ok {
			g.fail(path+".down", "%q is not a placement in this dungeon", p.Down)
			return
		}
		if !m.isMonster() {
			g.fail(path+".down", "%q is a prop, and only a monster can be down", p.Down)
		}
	case predicateFact:
		// Declared by mention: the dungeon allows a fact no record reveals
		// (R8), and the scenario is where "a hold-out nobody can win" is
		// refused.
	case predicateStance:
		g.stancePredicate(path+".stance", p.Stance, self)
	}
}

// stancePredicate checks the stance form: two factions that exist and
// differ, a stance word, and a pair that can reach it — not one it holds
// from the start, and not one it can never hold (design §3.8's liveness
// rule, in the file's own words).
func (g *grammar) stancePredicate(path string, s *StancePredicateSpec, self *[2]string) {
	ok := true
	for j, id := range s.Between {
		bp := fmt.Sprintf("%s.between[%d]", path, j)
		switch {
		case id == "":
			g.fail(bp, "the stance does not say which faction")
			ok = false
		case !g.factionExists(id):
			g.fail(bp, "%q is not a faction in this dungeon — declare it under `factions:`, or write `party`", id)
			ok = false
		}
	}
	if ok && s.Between[0] == s.Between[1] {
		g.fail(path+".between", "a stance is between two different factions, and this one names %q twice", s.Between[0])
		ok = false
	}
	if !stances[s.Is] {
		g.fail(path+".is", "%q is not a stance: %s", s.Is, stanceWords)
		ok = false
	}
	if !ok {
		return
	}
	key := normalizedPair(s.Between[0], s.Between[1])
	if self != nil && *self == key {
		g.fail(path, "a disposition cannot wait on its own stance")
		return
	}
	declared, until := g.declaredStance(key)
	switch {
	case declared == s.Is:
		g.fail(path, "%s and %s are %s from the start, so nothing can fire this", key[0], key[1], s.Is)
	case declared == string(encounter.StanceHostile) && until != nil && s.Is == string(encounter.StanceNeutral):
		// Reachable: the pair turns neutral when its until holds.
	default:
		g.fail(path, "%s and %s can never be %s: they are %s, and nothing turns them", key[0], key[1], s.Is, declared)
	}
}
