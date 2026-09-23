// Copyright (C) 2026 Kirk Diggler
// SPDX-License-Identifier: GPL-3.0-or-later

package encounter

import (
	"fmt"
	"sort"

	"github.com/KirkDiggler/rpg-toolkit/mind/behavior/deed"
	"github.com/KirkDiggler/rpg-toolkit/mind/perception"
	"github.com/KirkDiggler/rpg-toolkit/tools/spatial"
)

// facts.go is EXPERIENCE, THE FOURTH THING THAT LOADS THE DIE (design §4).
//
// "Two soldiers with identical orders and temperament still differ because
// one watched the fighter drop its friend, and only its `attacked: {within:
// 3}` holds." Nothing here is new memory: what a creature has seen and
// suffered is the holdings perception already keeps per observer, and this
// file is the projection a [When] condition reads them through.
//
// TWO DOORS, ONE PROJECTION. A social verdict is answered inside the verb,
// where the encounter has the creature in hand ([Encounter.factsFor]); a
// `time` pick is made by a [Driver], which is handed a [MonsterView] and
// nothing else ([factsFromView]). Both end at the same [Facts], and the deed
// half is literally the same function, so the two cannot come to disagree
// about how old a deed is.

// factsFor projects one member's own holdings into the facts its table reads.
//
// THE MEMBER'S OWN HOLDINGS AND NOTHING ELSE (C2), the same call
// [Encounter.buildMonsterView] makes. Opposition is asked of the stance graph
// per subject, so a creature in no faction — a world NPC — reads `enemy:
// none` however crowded the room is, which is what keeps the neutral goblin
// from advancing on the party the first time it has time.
func (e *Encounter) factsFor(id MemberID) (Facts, error) {
	self, ok := e.members[id]
	if !ok {
		return Facts{}, fmt.Errorf("facts: %q: %w", id, ErrNotMember)
	}
	holdings, err := e.intelLog.Held(id)
	if err != nil {
		return Facts{}, fmt.Errorf("facts: held by %q: %w", id, err)
	}

	facts := Facts{
		Deeds: heldDeedsAgainst(holdings, id),
		// THE TWO READINGS BESIDE THE FIRST (rpg-toolkit#1883, rpg-project#498).
		// Every deed a witness holds is ALREADY here — deeds land on every
		// witness, and the payload carries `Target` — so these are the same
		// holdings read two more ways, not a second fact source.
		//
		// ALLY IS THE STANCE GRAPH'S ANSWER, asked per deed at the moment it
		// is read: a disposition that changes changes this reading, and the
		// document never names a faction.
		AllyDeeds: heldDeedsAgainstSide(e, holdings, id),
		OwnDeeds:  heldDeedsBy(holdings, id),
		Now:       uint64(e.clock.ToData().HighWater),
		// NOTHING A SOCIAL VERDICT CAN ANSWER WITH IS PAID OUT OF A TURN.
		// `fact`, `flee` and a bare line cost nothing a budget runs out of,
		// and the three budgeted words are refused under a social key at
		// every door a table comes in through ([validateTable]). Saying
		// "cannot" here would read as a creature with nothing left, which is
		// not what this projection knows or means.
		CanAttack: true,
		CanMove:   true,
	}

	for _, h := range holdings {
		if h.Channel != perception.Sight {
			continue
		}
		subject := MemberID(h.Subject)
		if _, ok := e.members[subject]; !ok {
			continue
		}
		if !e.opposed(id, subject) {
			continue
		}
		location, ok := DecodeSightTestimony(h.Payload)
		if !ok || location.State == LocationUnknown {
			continue
		}
		if h.CurrentOn(perception.Sight) {
			facts.EnemySeen = true
			if e.withinReach(self, location.Position) {
				facts.EnemyInReach = true
			}
			continue
		}
		facts.EnemyRemembered = true
	}

	narrowToOneBand(&facts)

	return facts, nil
}

// withinReach reports whether a cell is inside any of this member's own
// actions' reach — the SAME test [SeenMember.InReach] makes, so a table that
// says `attack` under `enemy: reach` can always land it.
//
// ANY ACTION, not the longest: InReach is per action and [attackIntent] takes
// the first one whose target is in reach, so "in reach" means "in reach of
// something I can do".
func (e *Encounter) withinReach(m *memberRecord, cell spatial.Position) bool {
	own, placed := e.canvas.GetEntityPosition(string(m.ID))
	if !placed {
		return false
	}
	distance := e.Distance(own, cell)
	for _, a := range m.Actions {
		if distance <= float64(CellsFromFeet(a.RangeFeet)) {
			return true
		}
	}

	return false
}

// narrowToOneBand keeps the enemy bands EXCLUSIVE: the nearest one that holds
// is the only one that holds.
//
// ENFORCED IN ONE PLACE, and both projections call it. An author writes one
// entry per band and knows exactly one is on the table; a reader that
// re-derived the exclusion would be a second place it could stop being true.
func narrowToOneBand(facts *Facts) {
	if facts.EnemyInReach {
		facts.EnemySeen = false
		facts.EnemyRemembered = false

		return
	}
	if facts.EnemySeen {
		facts.EnemyRemembered = false
	}
}

// factsFromView is the same projection for a [Driver], which is handed a
// [MonsterView] rather than the encounter.
//
// IT READS Opposed OFF THE VIEW rather than asking the stance graph, because
// a driver has no graph to ask: [Encounter.buildMonsterView] fills the flag
// from `opposed(self, other)` at the moment the view is built, which is the
// anti-wall-hack contract holding for opposition exactly as it holds for
// position.
func factsFromView(view MonsterView) Facts {
	facts := Facts{
		Deeds:     view.Deeds,
		AllyDeeds: view.AllyDeeds,
		OwnDeeds:  view.OwnDeeds,
		Now:       view.At,
		// AFFORDABILITY IS ELIGIBILITY (rpg-project#465, ruled on an
		// api-builder finding): what the creature can still pay for this turn
		// decides which entries are on the table at all. The budget is the
		// turn's own, so a creature asked again after its swing is not handed
		// `attack` a second time — and on the world clock, where the budget
		// carries no attacks at all, a table's `attack` row is simply never a
		// candidate.
		CanAttack: view.Budget.AttacksLeft > 0,
		CanMove:   CellsFromFeet(view.Budget.MovementFeet) > 0,
	}
	for _, s := range view.Seen {
		if !s.Opposed {
			continue
		}
		facts.EnemySeen = true
		// THE SAME REACH AN Attack IS TESTED AGAINST, read off the view the
		// encounter already computed it onto ([attackIntent] asks the same
		// map) — so `enemy: reach` and `attack: enemy` can never disagree
		// about whether the swing lands.
		//
		// STANDING IS PART OF IT: a body is not an enemy in reach, and
		// attackIntent will not swing at one either.
		if !s.Standing {
			continue
		}
		for _, a := range view.Actions {
			if s.InReach[a.Ref] {
				facts.EnemyInReach = true
			}
		}
	}
	for _, r := range view.Remembered {
		if r.Opposed {
			facts.EnemyRemembered = true
			break
		}
	}
	narrowToOneBand(&facts)

	return facts
}

// heldDeedsAgainst is every deed this creature holds that was done TO IT,
// freshest last is NOT promised — the list is sorted by actor so two
// identical situations read identically (C8), and [freshestDeed] is what
// picks by recency.
//
// AGAINST ITSELF, which is what a `when` condition asks. A creature that
// watched somebody else get hit holds that deed too; it is testimony about
// the room and not a thing that happened to the creature, and reading it as
// one would have a goblin retaliate for a blow it merely witnessed.
//
// THE WITNESSED DEED IS NOT THROWN AWAY, and this filter is where it stops
// being invisible: [heldDeedsAgainstSide] reads the SAME holdings for the
// blows that landed on this creature's own side, which is the reading
// `when: { <deed>: { … , on: ally } }` asks for (rpg-toolkit#1883).
func heldDeedsAgainst(holdings []perception.Holding, self MemberID) []HeldDeed {
	return heldDeedsWhere(holdings, func(d deed.Deed) bool { return d.Target == self })
}

// heldDeedsAgainstSide is every deed this creature holds that was done to ONE
// OF ITS OWN SIDE — the reading `on: ally` asks for.
//
// THE STANCE GRAPH ANSWERS "WHOSE", asked here, per deed, at the moment the
// facts are projected. That is what makes this reading follow a disposition
// that changes: a member who was an ally when the blow landed and is an enemy
// now is not counted, because the question is asked against the CURRENT
// graph, not against a list frozen into the document.
//
// SELF IS NOT ONE OF ITS OWN SIDE. A blow to the creature is [Deeds]'s
// reading, and counting it here too would make `on: ally` fire for a wound to
// the creature itself — the two readings would stop being different questions.
func heldDeedsAgainstSide(e *Encounter, holdings []perception.Holding, self MemberID) []HeldDeed {
	return heldDeedsWhere(holdings, func(d deed.Deed) bool {
		target := MemberID(d.Target)
		if target == "" || target == self {
			return false
		}
		// THE ALLIED EDGE, NOT "NOT HOSTILE": two neutral factions are
		// neither, so a blow to a neutral bystander is not a blow to my side.
		// That is the honest reading and it is what makes this follow a
		// disposition that changes.
		allied, _ := e.IsAllied(self, target)

		return allied
	})
}

// heldDeedsBy is every deed this creature DID — the reading `as: actor` asks
// for, and what a pause is made of ("after I strike, stand still").
//
// THE ACTOR IS THE CREATURE, and nothing checks whether anyone was there to
// see it: a creature knows what it did. The deed is on this creature's own
// holdings because it was a witness to itself, which is how every deed lands
// on its actor as well as on the room.
func heldDeedsBy(holdings []perception.Holding, self MemberID) []HeldDeed {
	return heldDeedsWhere(holdings, func(d deed.Deed) bool { return d.Actor == self })
}

// heldDeedsWhere is the ONE walk every reading goes through: decode each
// holding on the deeds channel, keep the ones this reading asks for, and sort
// by actor so two identical situations read identically (C8).
//
// ONE WALK, THREE ANSWERS. The three readings are filters over the same
// holdings, so they cannot disagree about how old a deed is or which payload
// decodes — the failure a second decode loop would eventually produce.
func heldDeedsWhere(
	holdings []perception.Holding, keep func(deed.Deed) bool,
) []HeldDeed {
	var out []HeldDeed
	for _, h := range holdings {
		if h.Channel != deed.Channel {
			continue
		}
		d, err := deed.Decode(h.Payload)
		if err != nil {
			// A payload this module did not write, on the channel it reads.
			// Skipped rather than refused: a pick is not the place to
			// discover another module's wire change, and a deed nobody can
			// read is a condition that does not hold.
			continue
		}
		if !keep(d) {
			continue
		}
		out = append(out, HeldDeed{Kind: d.Verb, Actor: d.Actor, At: h.Confirmed})
	}
	sort.SliceStable(out, func(i, j int) bool { return out[i].Actor < out[j].Actor })

	return out
}

// freshestDeed is the most recent deed of a kind, and the actor who did it —
// what `attacker` and `actor` select. Ties break on the actor's id so two
// deeds landed in the same round pick the same one every run (C8).
//
// Returns ok=false when nothing of that kind is held, which is a decision
// rather than an error: a table that said `attack: attacker` and has never
// been attacked has nobody to swing at, and the entry still rolled.
func freshestDeed(deeds []HeldDeed, kind string) (HeldDeed, bool) {
	var best HeldDeed
	found := false
	for _, d := range deeds {
		if d.Kind != kind {
			continue
		}
		if !found || d.At > best.At || (d.At == best.At && d.Actor < best.Actor) {
			best = d
			found = true
		}
	}

	return best, found
}
