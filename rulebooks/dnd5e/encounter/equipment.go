// Copyright (C) 2026 Kirk Diggler
// SPDX-License-Identifier: GPL-3.0-or-later

package encounter

import (
	"fmt"
	"sort"
)

// HeldEquipment is what one member has in their hands, as the rulebook reports
// it. Two hands, because two hands is what a weapon visual needs and what
// rpg-toolkit#1615 asked for; a belt, a quiver and a worn cloak are not absent
// because they are unimportant but because nothing has needed them yet.
//
// The strings are ITEM IDS, bare — "longsword", "shield" — and deliberately not
// refs. The identity a client keys a model off is `dnd5e:item:<id>`, which is
// the rpg-game-assets provider manifest's namespace, minted by the host on the
// way out. It flattens two rulebook namespaces the rules keep apart: a shield is
// `dnd5e:armor:shield` to [combat.AC] and a longsword is `dnd5e:weapons:longsword`
// to an attack, and both are one `item` to the thing that puts a model in a
// hand. This module owns neither end of that translation and should not learn
// it — see rpg-toolkit#1615 for the census.
//
// An empty string is a hand with nothing in it, which is a POSITIVE fact and
// not an absence: the rulebook turns an empty main hand into an unarmed strike
// with its own ref and its own name. See [Equipment] for the absence.
type HeldEquipment struct {
	// MainHand is the item id in the member's main hand, or "" for a hand with
	// nothing in it.
	MainHand string

	// OffHand is the item id in the member's off hand, or "" for a hand with
	// nothing in it.
	OffHand string
}

// Equipment reports what each of the given members is holding. The composition
// asks; the rulebook answers.
//
// Injected rather than held, exactly as [Sight] and [Standing] are, and for the
// same reason theirs give: this module's go.mod cannot import the rulebook (law
// C1), so what is in a creature's hands is a fact it can only be TOLD. Member
// IDs in, two hands out per member — nothing here learns what a longsword is,
// or which hand a shield belongs in, or that a two-handed weapon fills both.
//
// # Truth in, lies later, and that split is the whole point
//
// This capability answers what a member IS holding. It never dissembles, and it
// is not asked who is looking. That is not a simplification — it is where the
// seam belongs.
//
// A percept built from this answer becomes per-observer TESTIMONY the moment it
// is surveilled: each observer's holding is their own, snapshotted at the
// instant they saw it, and thereafter free to disagree with the world and with
// every other observer. A charmed player who believes the ogre is holding a toy
// is a false payload written into THAT player's holding by whatever effect
// charmed them — not this capability lying to them, and not the composition
// deciding who deserves the truth. Rules stay in resolution; this is a pull of
// current fact, like [Sight]'s reach and [Standing]'s down.
//
// The alternative — resolving hands from the sheet when a client asks — was
// considered and refused (Kirk, rpg-toolkit#1615): a live read can only ever
// return the truth, so it forecloses illusion, disguise and enchantment
// permanently rather than merely deferring them. What a member is holding must
// be able to be WRONG in one observer's eyes, and only a snapshot can be wrong.
//
// # Absence is an answer, and it is not empty hands
//
// Every member asked about must appear in the answer, exactly as [Sight]
// requires and for the same reason: a member the answer skipped is a member
// whose hands this module would have to invent, and inventing is what
// rpg-toolkit#1033 forbids. A stranger in the answer is refused too
// (ErrNotMember), as [Sight] and [Standing] both do.
//
// What differs is that the ANSWER may be nil. A nil [HeldEquipment] says there
// is nothing to observe: a skeleton has no character sheet and no hands to
// report on, and claiming it was observed empty-handed would be inventing
// testimony in the other direction. Two claims, two shapes:
//
//   - nil          — no hands to speak of. Nothing is known and nothing is claimed.
//   - &{"", ""}    — hands, and they are empty. A player standing there unarmed.
//
// Collapsing those into one value is the bug this shape exists to prevent, and
// the reason the answer is a pointer rather than a struct with an "unknown"
// flag inside it: a flag beside two strings that mean nothing when the flag is
// set is a zero value that lies. Compare [actions.CastProfile]'s Concentration,
// a pointer for exactly this reason.
//
// # It is a pull, and it is never remembered
//
// The composition stores no equipment, in memory or in its blob. It asks at the
// choke point where percepts are built and uses the answer once, for that
// refresh. Being a pull is what makes it COMPLETE: every route to a changed
// hand — an equip, an unequip, a swap, a disarm, a rule nobody has written yet
// — is noticed at the next refresh without that route knowing this interface
// exists. Carrying the answer between refreshes would be a cache, and a cache
// is the smallest possible version of the dual state [Sight]'s doc describes.
//
// Errors abort whatever verb was running, atomically (R5), the same as
// [Sight]'s and [Standing]'s.
type Equipment interface {
	// Equipment reports what each of the given members is holding. Every member
	// asked about must appear in the answer and nobody else may; a nil value
	// says that member has no hands to observe.
	Equipment(members []MemberID) (map[MemberID]*HeldEquipment, error)
}

// equipmentNow asks the capability what every member is holding, right now, and
// returns the answer keyed by member.
//
// The roster goes over SORTED, for C8's reason and [Encounter.sightNow]'s: what
// a pass concludes must be a function of persisted data rather than of map
// iteration order, and a capability handed its question in a different order
// each time could answer differently each time. An empty roster is not a
// question worth asking, and the capability is not called for one.
//
// The whole roster is asked about rather than only this refresh's observers,
// which is the same choice [Encounter.sightNow] and [Encounter.downNow]
// make and for the same reason: the question a rulebook receives should not
// depend on which verb happens to be running.
//
// Two refusals, each existing to make a mis-wired capability LOOK mis-wired
// rather than like a rule that silently never fires: a stranger in the answer
// (ErrNotMember), and a member the answer skipped (ErrNoEquipment). A nil value
// is not a skipped member — it is the answer "no hands to observe", and it is
// accepted. See [Equipment] for why those are different claims.
func (e *Encounter) equipmentNow() (map[MemberID]*HeldEquipment, error) {
	if len(e.members) == 0 {
		return nil, nil
	}

	roster := make([]MemberID, 0, len(e.members))
	for id := range e.members {
		roster = append(roster, id)
	}
	sort.Slice(roster, func(i, j int) bool { return roster[i] < roster[j] })

	reported, err := e.equipment.Equipment(roster)
	if err != nil {
		return nil, fmt.Errorf("equipment: %w", err)
	}

	for id := range reported {
		if _, ok := e.members[id]; !ok {
			return nil, fmt.Errorf("equipment: reported hands for %q, who is not a member: %w", id, ErrNotMember)
		}
	}

	for _, id := range roster {
		if _, ok := reported[id]; !ok {
			return nil, fmt.Errorf("equipment: did not say what %q is holding: %w", id, ErrNoEquipment)
		}
	}

	return reported, nil
}
