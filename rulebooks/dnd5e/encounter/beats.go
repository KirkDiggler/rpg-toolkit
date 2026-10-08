// Copyright (C) 2026 Kirk Diggler
// SPDX-License-Identifier: GPL-3.0-or-later

package encounter

// The "beat" values of the story beats this composition writes for its own
// verbs. A reader maps these constants, never a string literal, so a rename
// here fails to compile at every reader instead of quietly producing a beat
// nobody recognises (rpg-project#539, "What a verb settles").
//
// Not every beat is here. A beat whose kind belongs beside its verb is
// declared there ([BeatCast], [BeatSighted], [BeatAnswered], ...), and the
// outcome beats a rulebook hands [Encounter.Record] carry their [OutcomeKind]
// ([OutcomeStruck], [OutcomeDown], ...) as the beat.
const (
	// BeatSceneOpened is the first beat of every encounter: the scene its
	// members open on.
	BeatSceneOpened = "scene-opened"

	// BeatJoined records a member joining the encounter.
	BeatJoined = "joined"

	// BeatExited records a member leaving the encounter.
	BeatExited = "exited"

	// BeatMoved records one executed step.
	BeatMoved = "moved"

	// BeatTick records the world clock advancing a round.
	BeatTick = "tick"

	// BeatTurnEnded records a fight's turn passing to the next member.
	BeatTurnEnded = "turn-ended"

	// BeatFightStarted records a fight forming: the members of its turn
	// order. The composition's own word is a bubble forming.
	BeatFightStarted = "bubble-formed"

	// BeatFightEnded records a fight ending: its members and its cause
	// ([DissolveKind]). Read by [Encounter.Settlement] as a [FightEnded].
	BeatFightEnded = "bubble-dissolved"

	// BeatTransferred records a member moving between clocks.
	BeatTransferred = "transferred"

	// BeatEnded records the encounter closing on an ending.
	BeatEnded = "ended"

	// BeatActivated records a member activating an ability.
	BeatActivated = "activated"

	// BeatActivationResult records one result of an activation, a cast, or
	// an area's membership: a condition applied or removed.
	BeatActivationResult = "activation-result"

	// BeatDoor records a door changing state.
	BeatDoor = "door"

	// BeatInteracted records a member interacting with a prop.
	BeatInteracted = "interacted"

	// BeatLooted records a member looting a downed member.
	BeatLooted = "looted"

	// BeatHeld records a member taking up a holdable prop.
	BeatHeld = "held"

	// BeatDropped records a member putting a held prop down.
	BeatDropped = "dropped"

	// BeatStance records the stance between two factions turning.
	BeatStance = "stance"

	// BeatArrived records a reserve member arriving on the field.
	BeatArrived = "arrived"

	// BeatEquipped records a member changing what one equipment slot holds:
	// the item now in it, the item taken out of it, or both for a swap
	// ([Encounter.RecordEquip]).
	BeatEquipped = "equipped"

	// BeatRested records a member resting: the kind of rest and what it
	// restored ([Encounter.RecordRest]).
	BeatRested = "rested"
)
