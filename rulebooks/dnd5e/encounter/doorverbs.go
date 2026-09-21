// Copyright (C) 2026 Kirk Diggler
// SPDX-License-Identifier: GPL-3.0-or-later

package encounter

import (
	"encoding/json"
	"fmt"
	"slices"

	"github.com/KirkDiggler/rpg-toolkit/play/record"
	"github.com/KirkDiggler/rpg-toolkit/tools/spatial"
)

// doorverbs.go is THE THREE WAYS A DOOR CHANGES (rpg-toolkit#1123).
//
// Every one of them does the same four things in the same order, because a door
// changing what it blocks is a world event exactly as a step is: put the new
// state on the door's edges, write the beat, refresh sight, and report what
// happened. The sight refresh is not bookkeeping — it is the point. A door
// opening is how the reference tomb's boss chamber gets REVEALED, and a fight
// can start on it the way one starts on a step, which is why every one of these
// can return a [FormedBubble].
//
// # Refusing rather than quietly succeeding
//
// Opening an open door, closing a shut one, unlocking one that is not locked:
// all three are refused (ErrBadDoor), not answered with a cheerful no-op. That
// is [Encounter.Dissolve]'s call — "a Dissolve on a fight the world already
// ended returns ErrNoBubble, which is the honest answer to asking for something
// that has happened" — and it is the shape this composition has spent several
// slices deleting everywhere else.
//
// # And the lock is a gate, which is a deliberate divergence
//
// [Encounter.OpenDoor] REFUSES a locked door. The old stack does not: its
// OpenDoor succeeds on a locked door and leaves the gating to an orchestrator
// (encounter/data.go's DoorData doc comment says so outright). #1123 says to
// port that stack's open/locked/DC MODEL, and the model is what is ported — the
// non-gating is not part of it, and a verb that opens a locked door is a
// silent-success shape.
//
// # And you have to be able to touch it
//
// Kirk, 2026-09-21: "Currently I can open a door not next to it." All three
// verbs REACH for the door and refuse a hand too far from it
// (rpg-toolkit#1856) — [Encounter.refuseOutOfDoorReach], which is
// [Encounter.Hold]'s rule unchanged, asked of the door's own cells.
//
// The rule is Hold's rather than a door rule because there is nothing about
// a door that makes reaching for one different from reaching for a table:
// grid distance, adjacent by default, and standing on the thing is distance
// zero. A second distance rule here would be the second truth this file's
// doors have spent three slices not having.
//
// It is judged AFTER the probe law and BEFORE the lock, which is the whole
// reason the order is written down: a member across the room is told they
// cannot reach the door, never what its lock would cost them.
//
// # NOTHING HERE COMPARES ANYTHING
//
// Kirk, on this file: "I agree on rules leaking in we need to be diligent." The
// first version of [Encounter.Unlock] took a check total and measured it against
// the authored DC — and "a total that MEETS the DC succeeds" is a 5e rule, sat
// inside the module whose whole charter is that it holds none. It is the same
// overreach as naming a void case for a material, caught in the same wave.
//
// So the outcome ARRIVES AS DATA. The caller rolled; the caller knows whether
// the lock was beaten; it says so, and this changes the door's state. Ties,
// advantage, tool proficiency, a natural 1 that fails regardless, a system where
// meeting the DC is not enough: every one of those is a rulebook's business and
// none of them is expressible here — which is the point. The DC itself stays,
// carried and reported, because that is CONTENT ([Lock] says why).
//
// Data on the input rather than a capability this asks, which is the economy
// gate's ruling applied again: a capability is for what the composition must ASK
// mid-flight, and this is something the caller already holds by the time it
// calls.

// OpenDoorInput names the door to open, and who pushes it.
type OpenDoorInput struct {
	// Door is the door's identifier.
	Door DoorID

	// Actor is the member doing it, named on the beat so the story can say
	// WHO opened the way (rpg-project#269). Optional: empty means the change
	// has no author to narrate. Non-empty must name a member (ErrNotMember),
	// and must be able to REACH the door
	// ([Encounter.refuseOutOfDoorReach]) — an empty one reaches from
	// nowhere and is not measured, exactly as it probes nothing. A session
	// always names an actor, so every hand a player drives is measured.
	Actor MemberID
}

// OpenDoorOutput reports the door's new state and what opening it revealed.
type OpenDoorOutput struct {
	// Door is the door's identifier.
	Door DoorID

	// State is what state it is in now — always [DoorOpen], and present so a
	// caller reads the result off the answer rather than off the fact that it
	// called.
	State DoorStateKind

	// IntelDeltas maps member IDs to their updated percepts after the refresh.
	// An opened door is the whole reason this verb refreshes sight.
	IntelDeltas map[MemberID]*IntelDelta

	// Seq is the sequence number of the recorded beat.
	Seq uint64

	// Formed is set when what the open door revealed started a fight. Nil
	// otherwise.
	Formed *FormedBubble
}

// OpenDoor opens a door: its edges stop blocking, and whatever stood behind it
// comes into view.
//
// Refuses a LOCKED door with ErrLocked, naming every route through the lock
// and its price — [Encounter.Unlock] is
// the way through one. Refuses an already-open door with ErrBadDoor, for the
// reason this file's doc comment gives.
//
// Refuses an actor who cannot REACH the door
// ([Encounter.refuseOutOfDoorReach]).
//
// Validation order (R5 atomicity): nil input → closed → no such door → the
// probe law → the actor is a member → reach → locked → already open → the
// change itself.
//
// Errors: ErrNilInput, ErrClosed, ErrNoDoor, ErrNotMember, ErrBadPlacement,
// ErrOutOfRange, ErrLocked, ErrBadDoor.
func (e *Encounter) OpenDoor(in *OpenDoorInput) (*OpenDoorOutput, error) {
	if in == nil {
		return nil, fmt.Errorf("open door: %w", ErrNilInput)
	}
	if e.outcome != nil {
		return nil, fmt.Errorf("open door: %w", ErrClosed)
	}

	door, err := e.doorOf(in.Door)
	if err != nil {
		return nil, fmt.Errorf("open door: %w", err)
	}
	if err := e.probeDoor(door, in.Actor); err != nil {
		return nil, fmt.Errorf("open door: %w", err)
	}
	if err := e.doorActorOf(in.Actor); err != nil {
		return nil, fmt.Errorf("open door %q: %w", door.id, err)
	}
	if err := e.refuseOutOfDoorReach("open door", door, in.Actor); err != nil {
		return nil, err
	}

	if lock, locked := door.state.Lock(); locked {
		return nil, fmt.Errorf("open door %q: locked, %s: %w", door.id, lockLabel(lock), ErrLocked)
	}
	if door.state.Kind() == DoorOpen {
		return nil, fmt.Errorf("open door %q: it is already open: %w", door.id, ErrBadDoor)
	}

	changed, err := e.setDoorState(door, DoorIsOpen(), doorActorExtra(in.Actor))
	if err != nil {
		return nil, fmt.Errorf("open door %q: %w", door.id, err)
	}

	return &OpenDoorOutput{
		Door:        door.id,
		State:       DoorOpen,
		IntelDeltas: changed.deltas,
		Seq:         changed.seq,
		Formed:      changed.formed,
	}, nil
}

// CloseDoorInput names the door to close, and who shuts it.
type CloseDoorInput struct {
	// Door is the door's identifier.
	Door DoorID

	// Actor is the member doing it — [OpenDoorInput.Actor]'s contract.
	Actor MemberID
}

// CloseDoorOutput reports the door's new state and what closing it hid.
type CloseDoorOutput struct {
	// Door is the door's identifier.
	Door DoorID

	// State is what state it is in now — always [DoorClosed].
	State DoorStateKind

	// IntelDeltas maps member IDs to their updated percepts after the refresh.
	// Shutting a door takes things OUT of sight, which is a change a percept
	// has to hear about just as much as one that puts things in.
	IntelDeltas map[MemberID]*IntelDelta

	// Seq is the sequence number of the recorded beat.
	Seq uint64

	// Formed is set when the refresh started a fight. Nil in practice for a
	// closing door and present because the refresh is the same one every other
	// verb runs — a verb that could not report a formation would be the one
	// place a fight went unrecorded.
	Formed *FormedBubble
}

// CloseDoor shuts a door: its edges block movement and sight again.
//
// Closing does not LOCK. A lock is a fact about who may open a door, not a
// stronger way of shutting it, and inventing one here would be this module
// deciding a dungeon has a self-locking gate. A door authored locked and then
// beaten stays unlocked; shutting it gives an ordinary closed door.
//
// Refuses an already-closed door, and a locked one — a locked door is closed
// already, so this would be asking for something that has happened. Refuses
// an actor who cannot REACH the door
// ([Encounter.refuseOutOfDoorReach]).
//
// Validation order (R5 atomicity): nil input → closed → no such door → the
// probe law → the actor is a member → reach → already closed → the change
// itself.
//
// Errors: ErrNilInput, ErrClosed, ErrNoDoor, ErrNotMember, ErrBadPlacement,
// ErrOutOfRange, ErrBadDoor.
func (e *Encounter) CloseDoor(in *CloseDoorInput) (*CloseDoorOutput, error) {
	if in == nil {
		return nil, fmt.Errorf("close door: %w", ErrNilInput)
	}
	if e.outcome != nil {
		return nil, fmt.Errorf("close door: %w", ErrClosed)
	}

	door, err := e.doorOf(in.Door)
	if err != nil {
		return nil, fmt.Errorf("close door: %w", err)
	}
	if err := e.probeDoor(door, in.Actor); err != nil {
		return nil, fmt.Errorf("close door: %w", err)
	}
	if err := e.doorActorOf(in.Actor); err != nil {
		return nil, fmt.Errorf("close door %q: %w", door.id, err)
	}
	if err := e.refuseOutOfDoorReach("close door", door, in.Actor); err != nil {
		return nil, err
	}

	if door.state.Kind() != DoorOpen {
		return nil, fmt.Errorf("close door %q: it is already %s: %w", door.id, door.state.Kind(), ErrBadDoor)
	}

	changed, err := e.setDoorState(door, DoorIsClosed(), doorActorExtra(in.Actor))
	if err != nil {
		return nil, fmt.Errorf("close door %q: %w", door.id, err)
	}

	return &CloseDoorOutput{
		Door:        door.id,
		State:       DoorClosed,
		IntelDeltas: changed.deltas,
		Seq:         changed.seq,
		Formed:      changed.formed,
	}, nil
}

// UnlockInput names the door and says whether the attempt on it succeeded.
type UnlockInput struct {
	// Door is the door's identifier.
	Door DoorID

	// Beaten is whether the attempt beat the lock. The CALLER decides it;
	// nothing here recomputes or second-guesses it.
	//
	// THIS MODULE IS TOLD, AND IT DOES NOT COMPARE. An earlier version took the
	// check's total and measured it against the authored DC, which put "a total
	// that meets the DC succeeds" — a 5e rule — inside a module not allowed to
	// know one. Whether a tie succeeds, whether advantage applied, whether a
	// natural 1 fails regardless: all of that is the rulebook's, and none of it
	// can leak in through a boolean.
	//
	// False is a real answer rather than an absent one: it means somebody tried
	// and failed, which is a thing that happened and gets a beat.
	Beaten bool

	// Actor is the member whose hands tried the lock — [OpenDoorInput.Actor]'s
	// contract.
	Actor MemberID

	// Total is what the caller's check totalled, CARRIED AND NEVER COMPARED —
	// the same law the DC itself lives under ([Lock]): it rides the beat so
	// the story can say "17 vs DC 12" (full data until v1.0,
	// rpg-project#269), and nothing here reads it against anything. The
	// verdict is Beaten, alone.
	Total int

	// Calculation is the full sourced arithmetic behind Total — the d20 pool
	// with every face it threw and the keep record naming any rule that
	// decided which one counted. CARRIED, NEVER COMPARED, the same law as
	// Total.
	//
	// A DoorChanged written by an unlock attempt IS a check beat
	// (rpg-project#462 R4): "any future check beat" includes the one that
	// already exists. Optional; when present it must describe Total and open
	// with a d20 pool, or the attempt is refused rather than recorded wrong.
	Calculation *RollCalculation

	// Applied is the route the attempt actually took — the one the
	// caller's resolver applied, which is the member's best listed
	// approach per the standing ruling (rpg-project#350; the selection
	// mechanism is [CheckResolver], this wave's). REQUIRED, and it must be
	// one of the lock's listed approaches (ErrBadDoor otherwise): an
	// attempt resolves through exactly one authored route, and the beat
	// names that route's DC — never the whole lock's, which prices each
	// route separately. CARRIED, never compared, like everything else on
	// this input.
	Applied CheckApproach
}

// UnlockOutput reports whether the lock was beaten, and what that revealed.
type UnlockOutput struct {
	// Door is the door's identifier.
	Door DoorID

	// Beaten echoes what the caller said, so a caller reads the result off the
	// answer rather than off the fact that it called — [DissolveOutput.Cause]'s
	// reasoning.
	//
	// A FAILED ATTEMPT IS NOT AN ERROR. It is an outcome — the door is still
	// locked, still there, and still worth another try — and reporting it as an
	// error would make "she did not pick it" indistinguishable from "there is
	// no such door".
	Beaten bool

	// Approaches are the lock's authored ways through, echoed either way so
	// a caller narrating a near miss does not have to go looking for them.
	// CARRIED, never compared — see [Lock].
	Approaches []CheckApproach

	// Applied echoes which route the attempt took — [UnlockInput.Applied],
	// so a caller reads the faced DC off the answer rather than off what
	// it passed in.
	Applied CheckApproach

	// State is what state the door is in now: [DoorOpen] when beaten,
	// [DoorLocked] when not.
	State DoorStateKind

	// IntelDeltas maps member IDs to their updated percepts. Empty of changes
	// on a failed attempt, because nothing moved.
	IntelDeltas map[MemberID]*IntelDelta

	// Seq is the sequence number of the recorded beat. A failed attempt gets
	// one too: somebody tried, and the story is what happened rather than what
	// worked.
	Seq uint64

	// Formed is set when what the opened door revealed started a fight.
	Formed *FormedBubble
}

// Unlock reports an attempt on a locked door, and OPENS it when the caller says
// the lock was beaten.
//
// IT COMPARES NOTHING. What counts as beating a lock is 5e, and 5e lives on the
// other side of this seam — see this file's doc comment for why that matters
// more than the one line of arithmetic it saves. On success the door ends OPEN
// and unlocked, not merely unlocked, which is the old stack's behaviour and the
// right one: a party that just picked a lock is going through, and a separate
// OpenDoor call afterwards would be ceremony with a window in the middle where
// the door is a state nobody authored.
//
// On failure the door is UNCHANGED and remains recoverable: still locked, still
// at the same DC, ready for another attempt. That is the loop the old stack's
// locked_connector_test drives, and it is the one this reproduces.
//
// Refuses a door that is not locked (ErrBadDoor) — there is nothing to beat,
// and answering "beaten" for a door with no lock would be inventing a success.
// Refuses an actor who cannot REACH the door
// ([Encounter.refuseOutOfDoorReach]): hands pick locks, and a lock across
// the room is not one this attempt ever touched.
//
// Validation order (R5 atomicity): nil input → closed → no such door → the
// probe law → the actor is a member → reach → not locked → the applied route
// → the change itself.
//
// Errors: ErrNilInput, ErrClosed, ErrNoDoor, ErrNotMember, ErrBadPlacement,
// ErrOutOfRange, ErrBadDoor.
func (e *Encounter) Unlock(in *UnlockInput) (*UnlockOutput, error) {
	if in == nil {
		return nil, fmt.Errorf("unlock: %w", ErrNilInput)
	}
	if e.outcome != nil {
		return nil, fmt.Errorf("unlock: %w", ErrClosed)
	}

	door, err := e.doorOf(in.Door)
	if err != nil {
		return nil, fmt.Errorf("unlock: %w", err)
	}
	if err := e.probeDoor(door, in.Actor); err != nil {
		return nil, fmt.Errorf("unlock: %w", err)
	}

	if err := e.doorActorOf(in.Actor); err != nil {
		return nil, fmt.Errorf("unlock %q: %w", door.id, err)
	}
	if err := e.refuseOutOfDoorReach("unlock", door, in.Actor); err != nil {
		return nil, err
	}

	lock, locked := door.state.Lock()
	if !locked {
		return nil, fmt.Errorf("unlock %q: it is %s, not locked: %w", door.id, door.state.Kind(), ErrBadDoor)
	}
	if !slices.Contains(lock.Approaches, in.Applied) {
		return nil, fmt.Errorf("unlock %q: the applied route (DC %d) is not one the lock lists: %w",
			door.id, in.Applied.DC, ErrBadDoor)
	}

	// The state to land in, and it is the same call either way: a failed
	// attempt re-states the door as exactly what it already was, so there is
	// one path through setDoorState rather than a beat written in two places.
	next := door.state
	if in.Beaten {
		next = DoorIsOpen()
	}

	extra := doorActorExtra(in.Actor)
	if extra == nil {
		extra = map[string]interface{}{}
	}
	extra["approaches"] = approachesDataFrom(lock.Approaches)
	extra["applied"] = CheckApproachData(in.Applied)
	extra["dc"] = in.Applied.DC
	extra["beaten"] = in.Beaten
	extra["total"] = in.Total
	if in.Calculation != nil {
		if err := validateRecordedTotal(in.Calculation, in.Total); err != nil {
			return nil, fmt.Errorf("unlock %q: calculation: %w", door.id, err)
		}
		extra["calculation"] = in.Calculation
	}

	changed, err := e.setDoorState(door, next, extra)
	if err != nil {
		return nil, fmt.Errorf("unlock %q: %w", door.id, err)
	}

	// THE WORLD'S PRICE FOR AN ACTION (design §5, worldtime.go): one round on
	// the world clock for whoever tried the lock, paid after the door has
	// landed in the state it is in. Nothing at all for a member inside a
	// fight, where the round is what prices time — and nothing when nobody's
	// hands are named, because there is no actor to charge.
	if in.Actor != "" {
		if err := e.spendWorldAction(in.Actor); err != nil {
			return nil, fmt.Errorf("unlock %q: %w", door.id, err)
		}
	}

	return &UnlockOutput{
		Door:        door.id,
		Beaten:      in.Beaten,
		Approaches:  copyApproaches(lock.Approaches),
		Applied:     in.Applied,
		State:       door.state.Kind(),
		IntelDeltas: changed.deltas,
		Seq:         changed.seq,
		Formed:      changed.formed,
	}, nil
}

// doorChange is what a state change produced: the beat it wrote and the sight
// it refreshed.
type doorChange struct {
	seq    uint64
	deltas map[MemberID]*IntelDelta
	formed *FormedBubble
}

// setDoorState is the ONE place a door's state changes, and the one place the
// canvas learns about it.
//
// Three verbs share it so that "what happens when a door changes" is a single
// answer: the edges are re-registered, the beat is written, and sight is
// refreshed — in that order, because a verb's own beat precedes any beat its
// consequences append (the law is stated at [Encounter.refreshSight]).
//
// The state goes onto the record BEFORE the edges are re-registered, so that
// registerDoor reads the state the door is now in rather than being told the
// same thing twice. If the registration fails the encounter is left with a
// record and a canvas that disagree, and the caller's obligation is doc.go's:
// drop the encounter unsaved. That is the same window every other verb has and
// the same remedy — R5 buys atomicity against VALIDATION, not against a spatial
// primitive failing halfway, and a door's edges were validated at construction.
//
// Sight is refreshed even when nothing about the state actually changed (a
// failed unlock re-states the door as itself), which is deliberate: one path
// through here is worth more than the microseconds a "did it really change"
// branch would save, and a refresh over an unchanged world produces no new
// percepts by construction.
func (e *Encounter) setDoorState(door *doorRecord, next DoorState, extra map[string]interface{}) (doorChange, error) {
	door.state = next

	if err := registerDoor(e.canvas.BasicRoom, door); err != nil {
		return doorChange{}, err
	}

	// The beat's audience and the refresh's scope are two different
	// questions now (rpg-toolkit#1371). The BEAT goes to every member who
	// KNOWS the door: for a never-concealed door that is the whole roster
	// (full data until v1.0, unchanged), and for a concealed one it is
	// exactly the members it has been revealed to — computed BEFORE the
	// refresh below, whose sweep may mint new knowers; a member learning of
	// the door on this very change gets DOOR_REVEALED there, never this
	// beat. The REFRESH stays roster-wide regardless: a door changing what
	// it blocks changes what everyone can see, knower or not.
	audience := e.doorBeatAudience(door)

	seq, err := e.appendDoorBeat(door, audience, extra)
	if err != nil {
		return doorChange{}, err
	}

	deltas, formed, err := e.refreshSight(e.rosterIDs())
	if err != nil {
		return doorChange{}, err
	}

	return doorChange{seq: seq, deltas: deltas, formed: formed}, nil
}

// doorBeatAudience is who hears a door's own state-change beat: everyone,
// for a door that was never concealed (full data until v1.0), and exactly
// the current members who KNOW the door for a concealed one — as far as
// concealed structure requires and no further (rpg-project#350;
// rpg-toolkit#1020's shelf coming due for doors). Sorted, like every
// audience this module computes (C8).
func (e *Encounter) doorBeatAudience(door *doorRecord) []MemberID {
	if e.world == nil || door.concealed == nil {
		return e.audienceFor(subjectBeat)
	}
	knowers := make([]MemberID, 0, len(e.members))
	for _, id := range e.rosterIDs() {
		if e.world.knowsDoor(id, door.id) {
			knowers = append(knowers, id)
		}
	}
	return knowers
}

// probeDoor is THE PROBE LAW (ruled on rpg-project#350): everywhere a door
// id is spoken, a concealed door the acting member has not found answers
// with the same not-found error a door that does not exist answers with —
// byte-identical, which is why this returns doorOf's own error shape. A
// DC-naming or state-naming refusal would confirm existence to a guessed
// id, so this runs BEFORE any check that reads the door's state, and before
// actor validation too: a stranger probing a secret learns exactly what a
// stranger probing a typo does.
//
// An EMPTY actor is the host's own hand — an authored, unattributed change
// from the side of the seam that composed the dungeon and holds its whole
// truth — so it probes nothing.
func (e *Encounter) probeDoor(door *doorRecord, actor MemberID) error {
	if e.world == nil || door.concealed == nil || actor == "" {
		return nil
	}
	if e.world.knowsDoor(actor, door.id) {
		return nil
	}
	return fmt.Errorf("door %q: %w", door.id, ErrNoDoor)
}

// appendDoorBeat records what a door did, to the members who know it.
//
// The audience arrives computed ([Encounter.doorBeatAudience]): the whole
// roster for a never-concealed door — whether a member can SEE it move is
// still #1020's asymmetric perception, not this — and the door's knowers
// for a concealed one, which is as far as concealed structure requires.
func (e *Encounter) appendDoorBeat(door *doorRecord, audience []MemberID, extra map[string]interface{}) (uint64, error) {
	payload := map[string]interface{}{
		"beat":  "door",
		"door":  door.id,
		"state": string(door.state.Kind()),
	}
	for k, v := range extra {
		payload[k] = v
	}

	beatBytes, err := json.Marshal(payload)
	if err != nil {
		return 0, fmt.Errorf("marshal door beat: %w", err)
	}

	out, err := e.appendBeat(&record.AppendInput{
		Audience: audience,
		Tags:     map[string]string{"tag": "door"},
		Payload:  beatBytes,
		At:       uint64(e.clock.ToData().HighWater),
	})
	if err != nil {
		return 0, fmt.Errorf("append door beat: %w", err)
	}

	return out.Seq, nil
}

// doorActorOf validates a door verb's optional actor: empty is fine (a
// change with no author to narrate), non-empty must name a member of this
// encounter — a beat crediting a stranger would be the story lying.
func (e *Encounter) doorActorOf(actor MemberID) error {
	if actor == "" {
		return nil
	}
	if _, ok := e.members[actor]; !ok {
		return fmt.Errorf("actor %q: %w", actor, ErrNotMember)
	}
	return nil
}

// doorCells is WHERE A DOOR IS, in cells — the one answer the reach rule
// asks of a door, whichever of the two geometries it was authored in
// ([DoorInput]: exactly one, never both and never neither).
//
//   - An EDGE door stands in its CROSSINGS, and a crossing is two cells:
//     both endpoints of every edge the door holds. A four-hex gate is one
//     door standing in eight cells, which is [DoorEdge]'s "one state, many
//     edges" read for position instead of for blocking, and it is why a
//     wide gate is reachable anywhere along its width.
//   - A FOOTPRINT door stands where [field.placedCells] says its rectangle
//     stands — the SAME derivation a placed prop's reach asks
//     (rpg-toolkit#1854), including its centre-cell clause, so "where is
//     this thing" has one answer for props and doors alike. A door smaller
//     than a hex is reachable from the hex it sits in rather than from
//     nowhere.
//
// Deduplicated, so a gate whose edges share an endpoint does not measure it
// twice. Returned in first-mention edge order, or the field's own cell order
// for a footprint (C8): no caller reads the order — every one asks "is any
// of these in reach" — and it is pinned anyway so no refusal can depend on
// map iteration.
func (e *Encounter) doorCells(door *doorRecord) []spatial.Position {
	if door.placement != nil {
		return e.field.placedCells(*door.placement)
	}

	out := make([]spatial.Position, 0, len(door.edges)*2)
	seen := make(map[spatial.Position]bool, len(door.edges)*2)
	for _, edge := range door.edges {
		for _, cell := range [2]spatial.Position{edge.From, edge.To} {
			if seen[cell] {
				continue
			}
			seen[cell] = true
			out = append(out, cell)
		}
	}

	return out
}

// refuseOutOfDoorReach is A DOOR OPENS ONLY FROM BESIDE IT
// (rpg-toolkit#1856), shared by all three verbs so they cannot come to
// disagree about how far a hand reaches.
//
// [Encounter.holdPlaced]'s rule verbatim, asked of [Encounter.doorCells]
// rather than of a placement: [Encounter.refuseOutOfReachCell] — grid
// distance, reach zero meaning adjacent — against every cell the door stands
// on, IN REACH OF ANY OF THEM BEING IN REACH. Standing in a door's cell is
// distance zero; beside one is distance one. There is no door distance, no
// new flag, and no new sentence (rpg-project#488 R2): what a far-away hand
// is told is Hold's refusal with this verb's name in front of it.
//
// AN EMPTY ACTOR REACHES FROM NOWHERE AND IS NOT MEASURED. That is the
// existing contract ([OpenDoorInput.Actor]: empty means the change has no
// author to narrate) and the same hand [Encounter.probeDoor] already lets
// past — the host's own, changing a dungeon it composed and holds the whole
// truth of. Every hand a session drives names an actor, so every hand a
// player drives is measured.
//
// A named actor who is not on the canvas is ErrBadPlacement, not a silent
// pass: [Encounter.Hold] answers the same wiring fault the same way, and
// measuring from a position nobody has would be inventing one.
func (e *Encounter) refuseOutOfDoorReach(verb string, door *doorRecord, actor MemberID) error {
	if actor == "" {
		return nil
	}
	from, placed := e.canvas.GetEntityPosition(string(actor))
	if !placed {
		return fmt.Errorf("%s: member %q: %w", verb, actor, ErrBadPlacement)
	}
	for _, cell := range e.doorCells(door) {
		if e.refuseOutOfReachCell(verb, from, cell, 0, string(door.id)) == nil {
			return nil
		}
	}

	return fmt.Errorf("%s: %s: %w", verb, door.id, ErrOutOfRange)
}

// doorActorExtra is the actor's ride onto the beat — nil when there is
// nobody to name, so an authored or unattributed change carries no empty
// "actor" key.
func doorActorExtra(actor MemberID) map[string]interface{} {
	if actor == "" {
		return nil
	}
	return map[string]interface{}{"actor": string(actor)}
}
