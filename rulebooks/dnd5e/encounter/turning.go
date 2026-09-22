// Copyright (C) 2026 Kirk Diggler
// SPDX-License-Identifier: GPL-3.0-or-later

package encounter

import (
	"fmt"
	"sort"

	"github.com/KirkDiggler/rpg-toolkit/mind/perception"
)

// turning.go is A PAIR TURNS, AND WHO FINDS OUT (rpg-project#493, the
// both-ways design R1–R3): the authored `until` that is not a fact, the
// aggression law nobody authors, and the fight that a new hostility starts.
//
// # Three things turn a pair, and only one of them was here before
//
//   - A `fact` on an `until` — the pair's MIND comes to know something. That
//     one is the graph's: a Settle reads the mind's flag and this file has
//     nothing to do with it (world.go, flip.go).
//   - A `round`, a `down` or another pair's `stance` on an `until` — the
//     world's own truth, which no flag records until somebody writes one.
//     [Encounter.turnUntils] is that somebody, and it runs at the sites that
//     already notice each of those events.
//   - HOSTILE INTENT DELIVERED ACROSS A NEUTRAL PAIR — nothing authored at
//     all ([Encounter.aggression], R3 as R5 rebound it). An author never
//     writes "if attacked, become hostile" on a camp, and a camp that did
//     not have the line was never meant to stand there and take it. What
//     counts as hostile intent is [hostileIntent]: a swing, or a spell that
//     asks for a save against something that is not a kindness.
//
// All three end in the same place, [Encounter.settleStances]: one `stance`
// beat to everyone, the fights that lost their sides and the fight that just
// gained them, the arrivals waiting on the stance, the cascade, the endings.
// The cause differs; what a turn IS does not.

// turnUntils asks one site's question of every `until` that is not a fact,
// and writes the public turn for each pair whose answer is yes.
//
// holds is the site's own closure — [onRound], [onFall], [onStance] — the
// same one the arrivals at that site are asked with, so a pair and a
// placement waiting for the same thing can never disagree about whether it
// happened.
//
// FACT UNTILS ARE SKIPPED, and that is the grain rule rather than an
// optimisation (R2). A fact is somebody's knowledge: it turns the pair when
// the pair's MIND holds it, which is the fold's answer and not a fact this
// composition may write. The other three are true for the whole world the
// moment they happen, and nobody's mind has to learn them.
//
// ONCE PER PAIR PER STANCE. `until: { round: 3 }` still holds in round 4, and
// the flag it raised is still up; asking [encounterWorld.settled] first is
// what keeps the journal from growing a fact per round and the beat from
// being told twice.
func (e *Encounter) turnUntils(holds func(Trigger) (string, bool), at uint64) error {
	before := e.stanceTable()
	turned := false
	for _, d := range e.field.dispositions {
		if d.Until == nil {
			continue
		}
		if _, isFact := d.Until.(TriggerFact); isFact {
			continue
		}
		pair := pairOf(d.Between[0], d.Between[1])
		to := turnsTo(d.Stance)
		if e.world.settled(pair, to) {
			continue
		}
		cause, ok := holds(d.Until)
		if !ok {
			continue
		}
		if err := e.world.settlePair(pair, to, cause); err != nil {
			return err
		}
		turned = true
	}
	if !turned {
		return nil
	}
	return e.settleStances(before, at)
}

// aggression is THE LAW (R3): a member of one faction attacks a member of a
// faction it is NEUTRAL with, and the pair is hostile from that moment —
// faction-wide, publicly, whatever the author wrote.
//
// # Before the deed, deliberately
//
// Called from [Encounter.landAttack] ahead of the deed it is about, so the
// testimony the swing lands is read by creatures who are ALREADY at war. A
// goblin's `time` table reads `{ attacked: { within: 3 } } -> attack:
// attacker` on one row and `{ enemy: reach } -> attack: enemy` on the next,
// and the second row is the difference between one aggrieved goblin and a
// camp: it is `enemy:` that reads the graph. Stamp the deed first and the
// very pick the attack provokes still sees `enemy: none`.
//
// # Neutral only
//
// An allied pair is not turned — friendly fire is not betrayal in this cut,
// and is named out of scope in the design. A hostile pair has nothing to
// become. A member in no faction is on no side and there is no pair: that is
// a world NPC, and [Encounter.attackable] has already refused it as a target
// before this is reached.
//
// SYMMETRIC, because a pair is a pair. Two neutral monster factions that come
// to blows turn on each other by this same call, with no player involved.
func (e *Encounter) aggression(actor, target MemberID, at uint64) error {
	ma, ok := e.members[actor]
	if !ok {
		return nil
	}
	mb, ok := e.members[target]
	if !ok {
		return nil
	}
	fa, fb := factionOf(ma), factionOf(mb)
	if fa == "" || fb == "" || fa == fb {
		return nil
	}
	pair := pairOf(fa, fb)
	if e.stanceBetween(pair) != StanceNeutral {
		return nil
	}
	before := e.stanceTable()
	if err := e.world.settlePair(pair, StanceHostile, "attacked by "+string(actor)); err != nil {
		return err
	}
	return e.settleStances(before, at)
}

// hostileIntent answers R5 (rpg-project#493): did this cast deliver hostile
// intent to this recipient — the one question [Encounter.aggression] and the
// `attacked` deed are both owed, whichever door the harm came through.
//
// # An attack roll, or a save against something that is not a kindness
//
// R3 was written against the SWING, and 5e's damage verbs are heavily
// save-based: a failed-save Vicious Mockery on a scout of a neutral camp left
// the pair neutral, the caster on the world clock and the scout with no
// testimony that anything had been done to it, so even its own
// `attacked within 3 -> attack: attacker` row never fired. That is the exact
// fail-silent the design's opening paragraph exists to remove, surviving one
// delivery over (found by the independent review of #1864, which probed it).
// R5 rebinds the law to the intent rather than the mechanism: an attack roll,
// hit or miss, OR a cast that asks a member of another faction for a saving
// throw against a harmful effect.
//
// ON THE ATTEMPT, LANDED OR NOT — the missed swing's rule, for the missed
// swing's reason. A Vicious Mockery the scout SAVED against was still an
// attempt to hurt it, and a camp that turns only when the dice land is a camp
// that forgives a bad roll.
//
// # Harmful is read off the delivery, because nothing here can read intent
//
// This composition carries no spell semantics (C1): a [SpellIdentity] is a
// ref and a name it may not interpret, and no input anywhere says "this
// effect is harmful". What it CAN read is what the cast delivered, so the
// exemption the design names — a save that only helps provokes nothing — is
// stated in the negative and narrowly: a save whose WHOLE delivery to this
// recipient is a kindness (healing, a condition lifted, a stabilization, a
// granted capacity) provokes nothing, and everything else provokes.
//
// AN EMPTY DELIVERY PROVOKES, which is the attempt above rather than an
// oversight: a spell the target resisted outright delivers nothing at all,
// and that is the case R5 is most about. The residue is one false positive
// nobody can reach in this build — a beneficial save-gated cast the recipient
// RESISTED would read as an empty delivery and provoke — and it is the
// fail-closed direction of the ambiguity: the alternative reads a betrayal as
// nothing happening.
//
// THE MISSED AND WARDED ARMS ARE NOT ASKED, and neither is a hole. `Missed`
// is resolution's "attempted delivery to an outdated location" — a spell
// attack roll that misses arrives on the `Attack` arm as an OutcomeMissed
// strike and provokes there, exactly as the swing's own miss does. `Warded`
// is a fact about the CASTER's own failed save against somebody else's
// Sanctuary, and resolution populates neither a save nor a delivery beside
// it; what a ward stopped before it reached anyone is its own question.
func hostileIntent(target CastTargetResult) bool {
	if target.Attack != nil {
		return true
	}
	if target.Save == nil {
		return false
	}
	for _, result := range target.Results {
		switch result.Kind {
		case ResultHealingApplied, ResultConditionRemoved, ResultStabilized, ResultCapacityGranted:
			continue
		default:
			return true
		}
	}
	return len(target.Results) == 0
}

// attackable refuses a member that cannot be the target of an attack
// (rpg-project#493, R4) — a world NPC — and says how to author one that can.
//
// # It is the ref check, in the words this module has
//
// The ruling is written against refs: a `dnd5e:monsters:*` may be attacked
// and a `dnd5e:npcs:*` may not. This composition carries no member ref and
// cannot import the rulebook that would name those types (C1), so it asks the
// question it CAN ask, which is the same question: [KindWorld] is what an
// npcs ref becomes when the session places it, and it is documented as the
// member that "carries no content of its own — no ref, no capabilities, no
// policy". Nothing else here is spared, players included: a player may be
// attacked, and so may every monster.
//
// ASKED AT THE VERB, BEFORE ANY APPEND, at both doors an attack comes through
// — a recorded outcome ([Encounter.Record]) and a cast with an attack roll on
// a target ([Encounter.RecordCast]). Refusing inside landAttack would refuse
// after the beat that announced the swing had already been told.
func (e *Encounter) attackable(what string, id MemberID) error {
	member, ok := e.members[id]
	if !ok || member.Kind != KindWorld {
		return nil
	}
	return fmt.Errorf("%s: %q is an npc and cannot be attacked; author it as a monster to make it a target: %w",
		what, id, ErrNotATarget)
}

// formOnStance is THE FIGHT A TURN STARTS.
//
// The members it names were already in each other's sight — you attack what
// you can see, and the guards were civil in plain view until midnight — so
// the next sight refresh reports them Refreshed and [Encounter.classify]
// reads only FirstContact, by a law that is right for every other reason it
// exists. What just happened is nonetheless a first contact: not with a
// creature, with an ENEMY. So this hands classify that contact, and every
// rule about what a contact means — spotted and mutual form, the drop does
// not, a running fight is joined rather than started twice, surprise is read
// from the current view — stays in the one place it is written.
//
// PLAYER-MONSTER, LIKE EVERY OTHER FORMATION. classify pairs the two supplied
// sides, so two monster factions turning on each other become opposed — the
// graph says so, `enemy:` answers so, and every capability that asks
// [Encounter.opposed] agrees — without a bubble forming around them. A fight
// with no player in it has nobody to end a turn, and inventing one here would
// be a second formation rule rather than this one.
func (e *Encounter) formOnStance(before, after map[factionPair]Stance) error {
	participation, err := e.participationNow()
	if err != nil {
		return err
	}
	deltas, err := e.firstContactWithNewEnemies(before, after, participation.contact)
	if err != nil {
		return err
	}
	if len(deltas) == 0 {
		return nil
	}
	verdict, err := e.classify(deltas, participation.contact)
	if err != nil {
		return err
	}
	if _, _, err := e.applyVerdict(verdict, participation); err != nil {
		return err
	}
	return nil
}

// firstContactWithNewEnemies is the contact a turn created: per member in
// contact, every member it can SEE right now whose faction its own just
// became hostile to.
//
// CURRENT SIGHT, not a transition — which is the whole point. The transition
// is the stance, and it happened; what this answers is who was standing there
// when it did. A remembered position is not sight ([Holding.CurrentOn]) for
// the reason [Encounter.unawareOfOpposition] gives: knowing where a wolf used
// to be is not watching one.
//
// Sorted throughout, from the roster walk down to each member's list, because
// beat order is observable (C8).
func (e *Encounter) firstContactWithNewEnemies(
	before, after map[factionPair]Stance, contact map[MemberID]bool,
) (map[MemberID]*IntelDelta, error) {
	newly := make(map[factionPair]bool)
	for pair, stance := range after {
		if stance == StanceHostile && before[pair] != StanceHostile {
			newly[pair] = true
		}
	}
	if len(newly) == 0 {
		return nil, nil
	}

	out := make(map[MemberID]*IntelDelta)
	for _, id := range e.rosterIDs() {
		if !contact[id] {
			continue
		}
		faction := factionOf(e.members[id])
		if faction == "" {
			continue
		}
		holdings, err := e.intelLog.Held(id)
		if err != nil {
			return nil, fmt.Errorf("stance formation: %q's view: %w", id, err)
		}
		seen := make([]perception.Presence, 0, len(holdings))
		for _, holding := range holdings {
			if !holding.CurrentOn(perception.Sight) {
				continue
			}
			other, ok := e.members[holding.Subject]
			if !ok || !contact[other.ID] {
				continue
			}
			otherFaction := factionOf(other)
			if otherFaction == "" || !newly[pairOf(faction, otherFaction)] {
				continue
			}
			seen = append(seen, perception.Presence{ID: other.ID})
		}
		if len(seen) == 0 {
			continue
		}
		sort.Slice(seen, func(i, j int) bool { return seen[i].ID < seen[j].ID })
		out[id] = &IntelDelta{FirstContact: seen}
	}
	return out, nil
}
