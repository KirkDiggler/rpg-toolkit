// Copyright (C) 2026 Kirk Diggler
// SPDX-License-Identifier: GPL-3.0-or-later

package encounter

import (
	"encoding/json"
	"fmt"

	"github.com/KirkDiggler/rpg-toolkit/play/intel"
	"github.com/KirkDiggler/rpg-toolkit/play/record"
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
	// has no author to narrate. Non-empty must name a member (ErrNotMember).
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
	IntelDeltas map[MemberID]*intel.SurveilOutput

	// Seq is the sequence number of the recorded beat.
	Seq uint64

	// Formed is set when what the open door revealed started a fight. Nil
	// otherwise.
	Formed *FormedBubble
}

// OpenDoor opens a door: its edges stop blocking, and whatever stood behind it
// comes into view.
//
// Refuses a LOCKED door with ErrLocked, naming the DC — [Encounter.Unlock] is
// the way through one. Refuses an already-open door with ErrBadDoor, for the
// reason this file's doc comment gives.
//
// Validation order (R5 atomicity): nil input → closed → no such door → locked →
// already open → the change itself.
//
// Errors: ErrNilInput, ErrClosed, ErrNoDoor, ErrLocked, ErrBadDoor.
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
	if err := e.doorActorOf(in.Actor); err != nil {
		return nil, fmt.Errorf("open door %q: %w", door.id, err)
	}

	if lock, locked := door.state.Lock(); locked {
		return nil, fmt.Errorf("open door %q: locked, DC %d: %w", door.id, lock.DC, ErrLocked)
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
	IntelDeltas map[MemberID]*intel.SurveilOutput

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
// already, so this would be asking for something that has happened.
//
// Errors: ErrNilInput, ErrClosed, ErrNoDoor, ErrBadDoor.
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
	if err := e.doorActorOf(in.Actor); err != nil {
		return nil, fmt.Errorf("close door %q: %w", door.id, err)
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

	// DC is the lock's authored difficulty, echoed either way so a caller
	// narrating a near miss does not have to go looking for it. CARRIED, never
	// compared — see [Lock].
	DC int

	// State is what state the door is in now: [DoorOpen] when beaten,
	// [DoorLocked] when not.
	State DoorStateKind

	// IntelDeltas maps member IDs to their updated percepts. Empty of changes
	// on a failed attempt, because nothing moved.
	IntelDeltas map[MemberID]*intel.SurveilOutput

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
//
// Errors: ErrNilInput, ErrClosed, ErrNoDoor, ErrBadDoor.
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

	if err := e.doorActorOf(in.Actor); err != nil {
		return nil, fmt.Errorf("unlock %q: %w", door.id, err)
	}

	lock, locked := door.state.Lock()
	if !locked {
		return nil, fmt.Errorf("unlock %q: it is %s, not locked: %w", door.id, door.state.Kind(), ErrBadDoor)
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
	extra["dc"] = lock.DC
	extra["beaten"] = in.Beaten
	extra["total"] = in.Total

	changed, err := e.setDoorState(door, next, extra)
	if err != nil {
		return nil, fmt.Errorf("unlock %q: %w", door.id, err)
	}

	return &UnlockOutput{
		Door:        door.id,
		Beaten:      in.Beaten,
		DC:          lock.DC,
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
	deltas map[MemberID]*intel.SurveilOutput
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

	// subjectBeat: not a whole-table fact, and no one member is really "the
	// subject" of a door — #940's eventual perception scoping is far more
	// likely to key off who can currently see the door than off any one
	// member, so there's no subject to pass today. v1 still sends everyone
	// regardless (audienceFor's doc); this is the passthrough
	// appendDoorBeat's own doc calls "the one early adopter."
	audience := e.audienceFor(subjectBeat)

	seq, err := e.appendDoorBeat(door, audience, extra)
	if err != nil {
		return doorChange{}, err
	}

	deltas, formed, err := e.refreshSight(audience)
	if err != nil {
		return doorChange{}, err
	}

	return doorChange{seq: seq, deltas: deltas, formed: formed}, nil
}

// appendDoorBeat records what a door did, to everybody.
//
// THE WHOLE ROSTER HEARS IT, which is a simplification worth naming rather than
// hiding: whether a member can SEE a door move is a perception question this
// module does not ask yet, and #1020's asymmetric perception is where it
// belongs. Audience routing for door beats rides in with it. Until then the
// beat is honest about what happened and the percepts beside it are honest
// about what each member can see.
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

// doorActorExtra is the actor's ride onto the beat — nil when there is
// nobody to name, so an authored or unattributed change carries no empty
// "actor" key.
func doorActorExtra(actor MemberID) map[string]interface{} {
	if actor == "" {
		return nil
	}
	return map[string]interface{}{"actor": string(actor)}
}
