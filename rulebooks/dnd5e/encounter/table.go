// Copyright (C) 2026 Kirk Diggler
// SPDX-License-Identifier: GPL-3.0-or-later

package encounter

import (
	"github.com/KirkDiggler/rpg-toolkit/mind/behavior"
)

// table.go is THE CREATURE'S TABLE, and this file is the RULEBOOK'S HALF of
// it (rpg-project#465, ideas/creature-table/design.md).
//
// A creature decides by rolling on an authored weighted table, and four things
// load that die: the rulebook's default for its kind, the author's orders, its
// own temperament, and what it has seen and suffered.
//
// # The evaluator moved, and the move is the point
//
// The table is not a D&D idea. Kirk, 2026-09-18: "that it is not dnd specific
// makes it a welcome addition to the mind package which can be used in any
// rulebook and is good validation of its composability." So the policy
// primitive lives in mind/behavior beside mind/perception — perception says
// what a creature KNOWS, behavior says what it DOES with that — and what is
// left here is everything that knows D&D:
//
//   - building the [Facts] from this composition's own board: reach, sight,
//     memory and the deeds done to a member (facts.go);
//   - executing the words: a swing through the [Striker], a walk through
//     [Encounter.Route] and the step that provokes (tabledriver.go, clocks.go);
//   - naming the triggers a D&D verb settles, and refusing a word under a key
//     it is not legal on (below);
//   - the authoring dialect (dungeonspec), the persistence (data.go) and the
//     beats (answer.go).
//
// # Why the names are aliased rather than re-declared
//
// Every type below is the SAME type mind/behavior owns, under the name this
// module and its callers already say. An alias is not a second representation
// — it is one type with a local spelling — and it keeps `MemberInput.Table`,
// `MonsterView.Temper` and `Decision.Pick` reading as they always have. A
// wrapper struct would have been the dual representation this repo bans.

// The creature's policy and its parts, as mind/behavior declares them.
//
// A caller that only ever touches this composition need not know the module
// exists; a caller that wants to build a table without a board can import it
// directly and hand the result to [MemberInput.Table].
type (
	// AnswerKey is one trigger of a creature's table.
	AnswerKey = behavior.AnswerKey

	// Table is one creature's whole policy, keyed by what happened.
	Table = behavior.Table

	// Answer is ONE entry in a trigger's table.
	Answer = behavior.Answer

	// When is a condition on what this creature holds.
	When = behavior.When

	// EnemyWord is a `when: { enemy: … }` band.
	EnemyWord = behavior.EnemyWord

	// DeedScope says whose deed a `when: { <deed>: … }` condition is about:
	// the creature itself (the default), its own side, or the creature as
	// the doer (rpg-toolkit#1883).
	DeedScope = behavior.DeedScope

	// Selector names what a word acts on.
	Selector = behavior.Selector

	// SelectorWord is a selector's word.
	SelectorWord = behavior.SelectorWord

	// Temper is the temperament loading a creature's die.
	Temper = behavior.Temper

	// TemperProfile is what a temperament word multiplies by.
	TemperProfile = behavior.TemperProfile

	// Facts is what a [When] reads — built here, in facts.go, against this
	// composition's own board.
	Facts = behavior.Facts

	// HeldDeed is one deed a creature holds against itself.
	HeldDeed = behavior.HeldDeed

	// Candidate is one eligible entry as the die saw it.
	Candidate = behavior.Candidate

	// Pick is one roll on one key: who was eligible, what the die said, and
	// which entry fired.
	Pick = behavior.PickOutput
)

// Layer returns `over` laid on `base`: for each key, the nearer layer wins
// WHOLESALE. Re-exported so a host that only imports this composition can
// stack the rulebook's default under an author's orders.
var Layer = behavior.Layer

// The bands and selector words, re-exported so the authoring dialect refuses
// by the same names the evaluator reads.
const (
	// EnemyReach holds when an opposed member is within this creature's reach.
	EnemyReach = behavior.EnemyReach

	// EnemySeen holds when one is in sight and NONE is in reach.
	EnemySeen = behavior.EnemySeen

	// EnemyRemembered holds when none is in sight but one is held.
	EnemyRemembered = behavior.EnemyRemembered

	// EnemyNone holds when none of the three does.
	EnemyNone = behavior.EnemyNone

	// SelectorEnemy is the nearest opposed member in sight, else remembered.
	SelectorEnemy = behavior.SelectorEnemy

	// SelectorAttacker is who last landed an attack on this creature.
	SelectorAttacker = behavior.SelectorAttacker

	// SelectorActor is the actor of the deed this entry's `when` named.
	SelectorActor = behavior.SelectorActor
)

// EnemyWords and SelectorWords are the sealed vocabularies, for the dialect's
// refusals.
var (
	// EnemyWords is every value `enemy:` takes, nearest band first.
	EnemyWords = behavior.EnemyWords

	// SelectorWords is every selector word.
	SelectorWords = behavior.SelectorWords

	// WhenDeeds is every word a `when: { <deed>: { within: N } }` may name,
	// in the author's own past tense.
	WhenDeeds = behavior.WhenDeeds

	// DeedScopes is every scope a deed condition may name — the creature
	// itself (the empty string, and the default), its own side, or the
	// creature as the doer (rpg-toolkit#1883).
	DeedScopes = behavior.DeedScopes
)

// The three scopes a `when: { <deed>: { within: N, … } }` may carry, read
// from the evaluator's own declaration so a scope the composition fills and a
// scope the dialect refuses cannot drift apart.
const (
	// ScopeSelf is a deed against the creature itself — what omitting the
	// scope means, and what every document written before scopes means.
	ScopeSelf = behavior.ScopeSelf

	// ScopeAlly is a deed against somebody on this creature's own side. WHO
	// that is comes from the stance graph ([Encounter.IsAllied]), so the
	// reading follows a disposition that changes.
	ScopeAlly = behavior.ScopeAlly

	// ScopeActor is a deed the creature did — the fact a pause reads.
	ScopeActor = behavior.ScopeActor
)

// DeedVerbFor is the deed a `when` word reads: the verb the deeds channel
// files it under, for the word the author wrote.
var DeedVerbFor = behavior.DeedVerbFor

// The five triggers an author may write a table for: one per social verb per
// verdict, and one for the creature having time.
//
// THE FOUR SOCIAL KEYS ARE THIS RULEBOOK'S, which is why they are declared
// here and [behavior.KeyTime] is not. `intimidated` is a D&D verb settling;
// having time is not a D&D idea at all, and mind/behavior names it because a
// creature given time and eligible for nothing HELD its turn — a different
// answer from the silence an unmatched event produces.
//
// THE KEY IS THE VERB AND THE VERDICT TOGETHER, which is why failure has a
// table of its own. "A failed Intimidate that raises the alarm makes
// attempting worse than not attempting. That is deliberate, and it is what
// gives the untrained rule teeth" (the shenanigans design). A verb with no
// table for the outcome that happened does nothing at all and writes no beat
// — absent means absent, not "the creature shrugged".
const (
	// AnswerIntimidated is what the creature does when a threat lands.
	AnswerIntimidated AnswerKey = "intimidated"

	// AnswerIntimidateFailed is what it does when a threat misses.
	AnswerIntimidateFailed AnswerKey = "intimidate_failed"

	// AnswerPersuaded is what it does when an appeal lands.
	AnswerPersuaded AnswerKey = "persuaded"

	// AnswerPersuadeFailed is what it does when an appeal misses.
	AnswerPersuadeFailed AnswerKey = "persuade_failed"

	// AnswerTime is what the creature does when it has time: its turn in a
	// fight, or a round of the world (worldtime.go).
	//
	// THE ONE TIME KEY, and the reason there is no `attacked` event key
	// beside the social four: what a creature does about being attacked is
	// decided when it next has time, reading `attacked: {within: N}` — which
	// is what the retired retaliator preset's grudge did in Go, moved into
	// the author's sight.
	AnswerTime = behavior.KeyTime
)

// AnswerKeys is every SOCIAL outcome key this build accepts, in authored
// order: each verb's success then its failure. EXPORTED because the authoring
// dialect refuses every other key by name and must list the ones it takes, and
// a second copy of the list there is the drift this variable exists to
// prevent.
var AnswerKeys = []AnswerKey{
	AnswerIntimidated,
	AnswerIntimidateFailed,
	AnswerPersuaded,
	AnswerPersuadeFailed,
}

// TableKeys is every key a table may name — the social four plus
// [AnswerTime]. [AnswerKeys]'s reason for being exported, one level up: the
// dialect refuses a key outside this list by name.
var TableKeys = append(append([]AnswerKey(nil), AnswerKeys...), AnswerTime)

// TemperWords is the three temperaments this build multiplies by, sealed
// HERE as well as in the rulebook that ships their numbers.
//
// mind/behavior SEALS NOTHING: it multiplies any word that arrives with a
// profile, which is what lets a second rulebook ship its own vocabulary. The
// list is a D&D fact, and it lives twice on purpose — the profiles are content
// in rulebooks/dnd5e, this composition cannot import the rulebook (C1), and it
// still has to refuse `temper: brave` on the author's form rather than at the
// table. A test in the module that imports both is where the two agree.
var TemperWords = []string{"soldier", "coward", "aggressive"}

// ValidTemperWord reports whether a word is one of [TemperWords]. Exported
// because the authoring dialect refuses every other word by name.
func ValidTemperWord(word string) bool {
	for _, known := range TemperWords {
		if word == known {
			return true
		}
	}

	return false
}
