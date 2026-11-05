// Copyright (C) 2026 Kirk Diggler
// SPDX-License-Identifier: GPL-3.0-or-later

package encounter

import (
	"encoding/json"
	"errors"
	"fmt"

	"github.com/KirkDiggler/rpg-toolkit/play/record"
	"github.com/KirkDiggler/rpg-toolkit/tools/spatial"
)

// barred.go is THE BUMP IS A BEAT THE BODY HEARS (rpg-project#842, S2 of the
// guard holds the gate; design R2 and R7): a player's attempt stopped by a
// body writes [BeatBarred], and the body in the way hears it at once.
//
// # Who writes one
//
// [Encounter.Step], when a body refuses the step: a member on the cell, or
// a crossing a post holds (the barrier is then the nearest holder the mover
// sees, else the nearest holder). And the door verbs, when a post holds the
// door: the attempt is the bump (R7). Never [Encounter.stepTo], so a driven
// walk routes round and writes none; never a wall, a door, a field or a
// masked refusal, which keep their own sentences and write nothing.
//
// # Heard at once, by him alone
//
// The beat is targeted on the barrier and matched through
// [Encounter.matchOne] in the verb that bumped. It is NEVER LANDED on
// [Encounter.landed], so nothing else that has landed is matched early:
// wakebeats.go's law ("landed beats wait for the raise that matches them")
// holds for every other beat. A bump is a touch on one creature now, not
// time passing for the world. His reaction runs as any reaction does
// ([Encounter.react]) and spends nothing; whatever it makes (his voice, a
// fact he teaches) lands as it always does, so bystanders hear his voice
// when time next passes.
//
// # A stop, not a refusal
//
// The verb answers with output, not an error: the beat, the intel his
// answer changed, a fight it formed and an ending it reached. Nothing is
// spent: no pace, no round.

// DoorRefusal is a door verb refused because a post holds the door against
// the actor (rpg-project#835 R7, #842): the verb's output carries it instead
// of an error, so what the holder's answer to the attempt made (a fight, an
// intel change) is never lost.
type DoorRefusal struct {
	// Holder is the nearest holder the actor sees, else the nearest holder:
	// the one member the [BeatBarred] beat landed on.
	Holder MemberID

	// Reason is the public sentence: "<name> bars the door", or "someone bars
	// the door" when the actor sees no holder.
	Reason string
}

// bumpInput is one bump: who was stopped, by whom, at which cell, and the
// door and the verb ([doorVerbOpen], ...) for a door verb.
type bumpInput struct {
	mover   MemberID
	barrier MemberID
	at      spatial.Position
	door    DoorID
	verb    string
}

// The door verbs a bump names ([BeatBarred]'s `verb`).
const (
	doorVerbOpen   = "open"
	doorVerbClose  = "close"
	doorVerbUnlock = "unlock"
)

// bumpOutput is what a bump wrote and what the barrier's answer made.
type bumpOutput struct {
	seq     uint64
	deltas  map[MemberID]*IntelDelta
	formed  *FormedBubble
	outcome *Outcome
}

// bump writes [BeatBarred] and matches it on the barrier alone, at once
// (this file's doc). The sight refresh runs only when his answer did
// something a look can see (a pick, a body that stopped or started
// blocking): a bump nobody answers writes the one beat.
//
// Errors: the beat's append, his machine or pick, the refresh.
func (e *Encounter) bump(in bumpInput) (bumpOutput, error) {
	at := uint64(e.clock.ToData().HighWater)
	// A STEP NAMES THE CELL IT WAS REFUSED INTO, a door verb its door
	// (rpg-api-protos#418's BarredAct): the cell a door bump is measured
	// from is the composition's own, for the barrier's scope, never told.
	payload := map[string]interface{}{
		"beat":    BeatBarred,
		"mover":   string(in.mover),
		"barrier": string(in.barrier),
	}
	if in.door == "" {
		payload["cell"] = in.at
	} else {
		payload["door"] = string(in.door)
		payload["verb"] = in.verb
	}
	raw, err := json.Marshal(payload)
	if err != nil {
		return bumpOutput{}, fmt.Errorf("barred beat: %w", err)
	}
	appended, err := e.appendBeat(&record.AppendInput{
		At:       at,
		Audience: e.audienceFor(subjectBeat, in.mover, in.barrier),
		Tags:     map[string]string{"tag": BeatBarred},
		Payload:  raw,
	})
	if err != nil {
		return bumpOutput{}, fmt.Errorf("barred beat: %w", err)
	}
	out := bumpOutput{seq: appended.Seq}

	// THE PASS FLAGS ARE THIS BUMP'S. A bump is never inside a raise (the
	// verbs that bump are a host's), and every raise resets them, so they say
	// here what they say there: whether the reaction did something a look
	// can see.
	e.reactionPicked, e.blockingTurned, e.stanceFormed = false, false, nil
	cell := in.at
	// HIS ANSWER IS IN THE BUMP'S ACT (act.go, Law 2): a reaction reopens
	// its trigger's act, so his pick, his line and his voice carry the id of
	// the verb that bumped, and a reader ties his answer to the bump.
	prevAct := e.reactAct
	e.reactAct = e.CurrentAct()
	defer func() { e.reactAct = prevAct }()
	if err := e.matchOne(wakeBeat{Beat: beatBarred, Cell: &cell, Subject: in.mover, Member: in.barrier, At: at}); err != nil {
		return bumpOutput{}, fmt.Errorf("barred: %w", err)
	}
	out.formed, e.stanceFormed = e.stanceFormed, nil
	if e.outcome != nil {
		out.outcome = e.outcome
		return out, nil
	}
	if !e.reactionPicked && !e.blockingTurned {
		return out, nil
	}
	deltas, formed, err := e.refreshCostingReach(e.cellsOf(in.mover, in.barrier), in.mover)
	if err != nil {
		return bumpOutput{}, fmt.Errorf("barred refresh sight: %w", err)
	}
	out.deltas = deltas
	if formed != nil {
		out.formed = formed
	}
	out.outcome = e.outcome

	return out, nil
}

// heldDoorBumped is a door verb's bump: what it wrote and the refusal the
// verb's output carries.
type heldDoorBumped struct {
	out     bumpOutput
	refusal *DoorRefusal
}

// heldDoorBump is the hold read every door verb makes (R7), answered: false
// when no post holds the door against the actor; the bump otherwise, at the
// door's cell nearest the actor, heard by the holder the refusal names.
//
// Errors: the hold read's participation read; the bump's.
func (e *Encounter) heldDoorBump(d *doorRecord, actor MemberID, verb string) (heldDoorBumped, bool, error) {
	refusal := e.refuseHeldDoor(d, actor)
	if refusal == nil {
		return heldDoorBumped{}, false, nil
	}
	var held *DoorHeldError
	if !errors.As(refusal, &held) {
		return heldDoorBumped{}, false, refusal
	}
	from, _ := e.canvas.GetEntityPosition(string(actor))
	var cell spatial.Position
	for i, c := range e.doorCells(d) {
		if i == 0 || e.Distance(from, c) < e.Distance(from, cell) {
			cell = c
		}
	}
	out, err := e.bump(bumpInput{mover: actor, barrier: held.Holder, at: cell, door: d.id, verb: verb})
	if err != nil {
		return heldDoorBumped{}, false, err
	}

	return heldDoorBumped{out: out, refusal: &DoorRefusal{Holder: held.Holder, Reason: held.PublicReason()}}, true, nil
}
