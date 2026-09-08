// Copyright (C) 2026 Kirk Diggler
// SPDX-License-Identifier: GPL-3.0-or-later

package events

import (
	"github.com/KirkDiggler/rpg-toolkit/core"
	"github.com/KirkDiggler/rpg-toolkit/events"
)

// Offer is one spendable thing a member already holds that could join a roll
// the dice have already decided.
//
// It is an OFFER RATHER THAN A MODIFIER, and that is the whole distinction
// this type exists to make. A subscriber to [AttackChain] writes a number into
// a roll nobody has seen yet; a subscriber to [PostRollOfferChain] says "I hold
// a d6 and it is yours to spend on THAT" — and nothing is spent until somebody
// answers. The die is spent when it is TAKEN, which is the same ruling
// [ReactionTakenEvent] carries one layer down.
type Offer struct {
	// Ref names what is offering — the condition or feature holding the die,
	// e.g. "dnd5e:conditions:inspired". It is what an [OfferTakenEvent] names
	// back, so the offerer knows its own offer was the one taken.
	Ref *core.Ref

	// Name is the display name the player is asked with — "Bardic
	// Inspiration". Authored by the offerer, never derived from Ref.
	Name string

	// Audience is the member whose choice this is. The subscriber names it,
	// which is what makes the chain wide enough for an offer posed to somebody
	// other than the roller without this type learning about that case.
	Audience string

	// Die is the notation of what would be rolled and added — "1d6". A
	// notation rather than a number because nothing has been rolled: whoever
	// takes the offer rolls it, with their own roller.
	Die string
}

// PostRollOfferEvent is folded AFTER the d20 is rolled and BEFORE anything
// reads the outcome off it.
//
// # Why it sits exactly there
//
// RAW's own boundary for Bardic Inspiration is "after rolling the d20 and
// before the DM says whether the roll succeeds or fails", and the mechanical
// reason is stronger than the flavour one: [PostAttackRollChain] carries
// WouldHit, and Shield subscribes to it. An offer folded after that chain
// would let Shield react to a total the offered die had not yet joined, and
// WouldHit would be answered twice with two different answers.
//
// # Nothing here is a modifier
//
// Subscribers APPEND to Offers. They do not touch Roll, AttackBonus or Total,
// which are reported so a subscriber can decide whether its die is worth
// offering at all, and so whoever is asked can see what they are deciding
// about.
type PostRollOfferEvent struct {
	// AttackerID is whose d20 was rolled, and TargetID who it was rolled at.
	AttackerID string
	TargetID   string

	// Roll is the d20 as rolled, after advantage or disadvantage.
	Roll int

	// AttackBonus is what the attack chain settled on, and Total is Roll plus
	// it — the number an offer would be added to.
	AttackBonus int
	Total       int

	// Offers are what the roller's own effects put on the table, in
	// subscription order.
	Offers []Offer
}

// OfferTakenEvent says one offer was spent and what it rolled.
//
// Published by the machine that applied the face, because the machine is what
// rolls — the offerer is told afterwards, exactly as [ReactionTakenEvent]
// tells a reaction's condition that its reaction fired. The offerer consumes
// itself on this event; an offer nobody takes costs nothing.
type OfferTakenEvent struct {
	// Audience is the member who took it — the same member the [Offer] named.
	Audience string

	// Ref is the offer's own ref, so an effect can tell its own offer from
	// another one taken on the same roll.
	Ref *core.Ref

	// Face is what the die actually rolled and what was added to the total.
	Face int
}

var (
	// PostRollOfferChain is folded between the d20 and [PostAttackRollChain]:
	// see [PostRollOfferEvent] for why that boundary and no other.
	//
	// Folded on an ATTACK roll only today. Saving throws and ability checks
	// are the same shape and are deliberately not folded yet — they arrive
	// with the slice that asks for them, not as an enumeration against a
	// hypothetical.
	PostRollOfferChain = events.DefineChainedTopic[*PostRollOfferEvent]("dnd5e.roll.offer")

	// OfferTakenTopic carries [OfferTakenEvent]: an offer was spent, and this
	// is its face.
	OfferTakenTopic = events.DefineTypedTopic[OfferTakenEvent]("dnd5e.roll.offer.taken")
)
