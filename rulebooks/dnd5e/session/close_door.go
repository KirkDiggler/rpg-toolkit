// Copyright (C) 2026 Kirk Diggler
// SPDX-License-Identifier: GPL-3.0-or-later

package session

// close_door.go exposes the composition's already-authored CloseDoor verb
// through the seam (rpg-project#169, O2). It is DELIBERATELY THIN: the
// encounter owns every closing rule — it refuses a locked or already-closed
// door, probes concealment, checks the actor is a member, measures reach, flips
// the state, refreshes sight and records the existing door beat — and this file
// only carries that verb across the boundary in [Manager.OpenDoor]'s own
// guarded load/act/save/deliver shape.
//
// WHAT IT DOES NOT DO is the point. It does not look the door's state up first,
// does not compute reach or visibility, does not infer concealment, and does
// not invent a lock. A caller cannot tell a hidden door from a missing one:
// both come back as the composition's probe refusal, translated. Closing never
// locks — the composition's own ruling, carried unchanged.

import (
	"context"
	"fmt"

	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/encounter"
)

// CloseDoorInput names the door and who shuts it.
type CloseDoorInput struct {
	// Session is the session to act in.
	Session string

	// Member is who shuts it — the beat's actor. THE HOST MUST BIND Member TO
	// THE AUTHENTICATED CALLER, exactly as it does for [OpenDoorInput].
	Member string

	// Door is the door's identifier, as the Atlas's doorways name it.
	Door string
}

// CloseDoorOutput reports the door as it now stands and what closing it hid.
//
// The shape mirrors [OpenDoorOutput] exactly, because the lifecycle is the
// same one: shutting a door refreshes sight (things leave it as much as enter
// it), can in principle form a fight, and records one beat.
type CloseDoorOutput struct {
	// Door is the door, closed now.
	Door Door `json:"door"`

	// Discovered is what the refresh changed in each observer's view. A
	// closing door takes things OUT of sight, which is a change a percept
	// hears about just as much as one that puts things in.
	Discovered map[string]Discovery `json:"discovered,omitempty"`

	// Formed is present when what the closed door changed started a fight —
	// the same refresh every other verb runs, so a verb that could not report
	// one would be the place a formation went unrecorded.
	Formed *Formed `json:"formed,omitempty"`

	// Seq is the door beat's sequence in the ACTOR's own delivered numbering
	// (stream.go) — the same number the actor's event for this beat carries.
	Seq uint64 `json:"seq"`

	Saved    SaveReport     `json:"saved"`
	Delivery DeliveryReport `json:"delivery"`
}

// CloseDoor shuts an open door as Member.
//
// A locked or already-closed door refuses with ErrNoConnection — the
// composition's own ErrBadDoor translated, the same refusal an already-open door gives
// [Manager.OpenDoor]. A door that does not exist FOR THIS MEMBER answers
// identically: a concealed door the member has not found is refuted by the
// composition's probe law before anything else, so nothing here may look the
// door up first and answer differently.
//
// A MEMBER OUT OF REACH refuses with ErrOutOfRange, measured by the
// composition against every cell the door stands on (reach zero meaning
// adjacent), after the probe law and before the state check — so a hand across
// the room is told it cannot reach rather than that the door is shut.
//
// CLOSING DOES NOT LOCK. A beaten lock stays beaten; shutting a door gives an
// ordinary closed door. That ruling is the composition's and this seam adds
// nothing to it.
//
// Errors: ErrNilInput, ErrNoMemberID, ErrNoConnection (empty door, unknown,
// already closed or locked), ErrOutOfRange, or the ordinary read/save
// translations.
func (m *Manager) CloseDoor(ctx context.Context, in *CloseDoorInput) (*CloseDoorOutput, error) {
	if in == nil {
		return nil, fmt.Errorf("closedoor: %w", ErrNilInput)
	}
	release, lockErr := m.acquireSession(ctx, in.Session)
	if lockErr != nil {
		return nil, lockErr
	}
	defer release()
	if in.Member == "" {
		return nil, fmt.Errorf("closedoor: %w", ErrNoMemberID)
	}
	if in.Door == "" {
		return nil, fmt.Errorf("closedoor: %w", ErrNoConnection)
	}

	scope, err := m.openForChange(ctx, in.Session)
	if err != nil {
		return nil, fmt.Errorf("closedoor: %w", err)
	}

	closed, err := scope.enc.CloseDoor(&encounter.CloseDoorInput{
		Door:  in.Door,
		Actor: encounter.MemberID(in.Member),
	})
	if err != nil {
		return nil, fmt.Errorf("closedoor: %w", translate(err))
	}

	report, delivery, err := m.commit(ctx, scope)
	if err != nil {
		return nil, fmt.Errorf("closedoor: %w", err)
	}

	return &CloseDoorOutput{
		Door:       Door{ID: closed.Door, State: string(closed.State)},
		Discovered: projectDiscoveries(closed.IntelDeltas),
		Formed:     projectFormedFor(scope, in.Member, closed.Formed),
		Seq:        scope.deliveredSeq(in.Member, closed.Seq),
		Saved:      report,
		Delivery:   delivery,
	}, nil
}
