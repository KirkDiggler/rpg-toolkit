// Copyright (C) 2026 Kirk Diggler
// SPDX-License-Identifier: GPL-3.0-or-later

package events

import (
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/abilities"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/damage"
)

// ChildRef is the address of one condition on one member's sheet: whose sheet,
// and which condition.
//
// An ADDRESS RATHER THAN A POINTER, and that is the whole reason the type
// exists. A condition that owns effects sitting on other members' sheets is
// persisted as an opaque blob, so a live pointer to another member's condition
// object cannot survive the round trip. A {member, ref} pair can — and it is
// already the exact address [ConditionRemovedEvent] takes, so an owner that
// holds addresses can publish the removal it means with nothing in between.
type ChildRef struct {
	MemberID     string `json:"member_id"`
	ConditionRef string `json:"condition_ref"`
}

// Consequence is what happens when a [FollowUp]'s check fails: these
// conditions come off those sheets, and the owner that asked for the check
// ends with the named reason.
//
// PLAIN DATA, like the follow-up that carries it. It names addresses rather
// than holding conditions, and a reason rather than a decision, because the
// thing that acts on it is a machine the driver runs and not the subscriber
// that wrote it down.
type Consequence struct {
	// Remove are the addresses to strip. Each becomes one
	// [ConditionRemovedEvent] published by whoever delivers this.
	Remove []ChildRef

	// Owner is the condition that asked for the check, stripped last so the
	// children come off while their owner still names them.
	Owner ChildRef

	// Reason is why, in the vocabulary of whoever wrote the consequence, for
	// the removal facts and the record that reads them.
	Reason string
}

// FollowUp is DATA. It describes a check somebody owes.
//
// # It carries no roller, no dice, no outcome, and no way to obtain one
//
// That absence is the type's whole point, and it is deliberate rather than
// incidental. The alternative this shape exists to forbid is a subscriber that
// rolls dice off the bus inside its handler and reports a result: that would
// put a rules decision in a subscription order, hide the roll from the record,
// and make the outcome depend on who subscribed first.
//
// SUBSCRIBERS DESCRIBE, MACHINES ROLL. A follow-up is a question, and only the
// driver's machines answer questions.
//
// # Which is why the DC is a settled int
//
// This package may not import saves or resolution, so there is nothing here to
// express "a formula the machine evaluates" with. There does not need to be:
// whoever appends a follow-up was handed the facts it needed by the event it
// is answering, and arithmetic is description rather than rolling.
type FollowUp struct {
	// SaverID is who owes the check.
	SaverID string

	// Ability is which ability it is rolled against.
	Ability abilities.Ability

	// DC is the difficulty class, already settled by whoever appended this.
	DC int

	// Cause says what provoked the check, so the record can name the spell
	// that was at stake and the creature that threatened it.
	Cause SaveCause

	// OnFailure is what a failed check costs. A follow-up with no consequence
	// is a check nobody needed, since the consequence is the reason it exists.
	OnFailure Consequence
}

// DamageTakenEvent is a NOTIFICATION: damage has been applied, the number is
// settled, and it is already on the sheet.
//
// # It is not [DamageReceivedEvent]
//
// That topic means "damage has landed" too, but one of its subscribers treats
// it as an INSTRUCTION and applies the damage again (see rpg-toolkit#977), so
// it is not reusable until that conversion lands. This is a new topic with a
// new meaning and no such subscriber: nothing may apply anything in answer to
// it.
//
// # FollowUps is a return channel, which is why the topic carries a pointer
//
// [DamageTakenTopic] is defined over *DamageTakenEvent rather than the value,
// because a typed topic hands each subscriber a copy and a subscriber that
// appended to a copy would have its follow-up silently discarded. The whole
// point of the field is that what a subscriber appends reaches the machine
// that published — so the pointer is the shape that makes the channel true
// rather than decorative.
//
// The machine that applied the damage publishes this on the driver's bus and
// then runs what came back, in the same interaction. Nothing else may publish
// it: applying damage is bus-free on a sheet by design, and a bus parked on a
// sheet is a captured bus.
type DamageTakenEvent struct {
	// MemberID is whose sheet took it — character or monster.
	MemberID string

	// Amount is the applied total, after every fold: resistance, immunity and
	// vulnerability are already in this number.
	Amount int

	// DamageType is what kind it was.
	DamageType damage.Type

	// DroppedToZero reports that this blow took the member to 0 hit points.
	// Carried on the fact rather than looked up, because a subscriber reading
	// a sheet mid-interaction reads whatever that sheet happens to hold.
	DroppedToZero bool

	// Cause says what dealt it, and by whom.
	Cause SaveCause

	// FollowUps is THE RETURN CHANNEL — checks appended by subscribers, for
	// the publishing machine to run. See the type's godoc.
	FollowUps []FollowUp
}

// ConcentrationEndedEvent says that one caster stopped holding one spell
// together, and WHY.
//
// # A notification, and the only thing that carries the reason
//
// The removals this rides with are addresses and a bare string; they say what
// came off which sheet, not what happened. Six things end a concentration and
// each happens somewhere different — a failed check inside a strike, a recast
// inside the next cast, the clock or the fight ending on a boundary, the last
// child leaving, the caster going down. Nothing outside the condition can see
// all six, so the condition is what publishes this.
//
// No return channel and nothing to append. A reader records it; it decides
// nothing.
type ConcentrationEndedEvent struct {
	// CasterID is whose hold ended.
	CasterID string

	// SpellRef is what they were holding, as a ref string.
	SpellRef string

	// SpellName is what to call it, so a record never has to turn a ref back
	// into English.
	SpellName string

	// Reason is why, from the rulebook's vocabulary of end reasons.
	Reason string

	// Removed are the child addresses that came off with it, in the order
	// their removal facts were published. Empty when the hold had none left.
	Removed []ChildRef
}
