// Copyright (C) 2026 Kirk Diggler
// SPDX-License-Identifier: GPL-3.0-or-later

package session

// doors.go carries the door verbs across the seam (rpg-project#268, design
// rpg-project#269): Doors reads every door's live state, OpenDoor pushes a
// shut one open, and Unlock is where the lock's verdict lives — the session
// loads the sheet, rolls the check, and TELLS the composition Beaten. The
// composition compares nothing (encounter.UnlockInput's law), and this seam
// is the layer allowed to know that a total meeting the DC succeeds.

import (
	"context"
	"fmt"

	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/abilities"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/checks"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/encounter"
)

// DoorsInput asks for a session's doors.
type DoorsInput struct {
	// Session is the session to read.
	Session string
}

// DoorsOutput carries every authored door's live state.
type DoorsOutput struct {
	// Doors, in the composition's own stable order.
	Doors []Door `json:"doors"`
}

// Doors reports every door's identity and live state — the dynamic half of
// the Atlas's doorways. The Atlas says where a door's edges are and never
// changes; this says what each door is doing now. A host reads it once and
// keeps it fresh from EventDoor beats.
func (m *Manager) Doors(ctx context.Context, in *DoorsInput) (*DoorsOutput, error) {
	if in == nil {
		return nil, fmt.Errorf("doors: %w", ErrNilInput)
	}
	enc, err := m.open(ctx, in.Session)
	if err != nil {
		return nil, fmt.Errorf("doors: %w", err)
	}

	doors := enc.Doors()
	out := &DoorsOutput{Doors: make([]Door, 0, len(doors))}
	for _, d := range doors {
		out.Doors = append(out.Doors, projectDoor(d))
	}
	return out, nil
}

// OpenDoorInput names the door and who pushes it.
type OpenDoorInput struct {
	// Session is the session to act in.
	Session string

	// Member is who pushes it open — the beat's actor.
	Member string

	// Door is the door's identifier, as the Atlas's doorways name it.
	Door string
}

// OpenDoorOutput reports the door as it now stands and what opening it
// revealed.
type OpenDoorOutput struct {
	// Door is the door, open now.
	Door Door `json:"door"`

	// Discovered is what the refresh brought into each observer's view —
	// an opened door is the whole reason the verb refreshes sight.
	Discovered map[string]Discovery `json:"discovered,omitempty"`

	// Formed is present when what the door revealed started a fight.
	Formed *Formed `json:"formed,omitempty"`

	// Seq is the door beat's sequence number.
	Seq uint64 `json:"seq"`

	Saved    SaveReport     `json:"saved"`
	Delivery DeliveryReport `json:"delivery"`
}

// OpenDoor pushes a shut door open as Member.
//
// A locked door refuses with ErrLocked, naming the DC — Unlock is the way
// through one. An already-open door refuses with ErrNoConnection, the
// composition's own refusal translated.
func (m *Manager) OpenDoor(ctx context.Context, in *OpenDoorInput) (*OpenDoorOutput, error) {
	if in == nil {
		return nil, fmt.Errorf("opendoor: %w", ErrNilInput)
	}
	if in.Member == "" {
		return nil, fmt.Errorf("opendoor: %w", ErrNoMemberID)
	}
	if in.Door == "" {
		return nil, fmt.Errorf("opendoor: %w", ErrNoConnection)
	}

	scope, err := m.openForWrite(ctx, in.Session)
	if err != nil {
		return nil, fmt.Errorf("opendoor: %w", err)
	}

	opened, err := scope.enc.OpenDoor(&encounter.OpenDoorInput{
		Door:  in.Door,
		Actor: encounter.MemberID(in.Member),
	})
	if err != nil {
		return nil, fmt.Errorf("opendoor: %w", translate(err))
	}

	down, err := discoveryStanding(scope)
	if err != nil {
		return nil, fmt.Errorf("opendoor: %w", err)
	}

	report, delivery, err := m.commit(ctx, scope)
	if err != nil {
		return nil, fmt.Errorf("opendoor: %w", err)
	}

	return &OpenDoorOutput{
		Door:       Door{ID: opened.Door, State: string(opened.State)},
		Discovered: projectDiscoveries(opened.IntelDeltas, down),
		Formed:     projectFormed(opened.Formed),
		Seq:        opened.Seq,
		Saved:      report,
		Delivery:   delivery,
	}, nil
}

// UnlockInput names the lock and whose hands try it.
type UnlockInput struct {
	// Session is the session to act in.
	Session string

	// Member is whose hands — and whose ability modifier — try the lock.
	Member string

	// Door is the door's identifier.
	Door string
}

// UnlockOutput is the attempt, in the open: the roll is public down to the
// number (full data until v1.0), and the beat every member hears carries the
// same facts.
type UnlockOutput struct {
	// Beaten is whether the check beat the DC. The session rolled it; the
	// composition was told this verdict and nothing else.
	Beaten bool `json:"beaten"`

	// Total is what the check totalled. No omitempty: zero is an answer
	// (TestFalseIsAnAnswerOnTheWire's law).
	Total int `json:"total"`

	// DC is what it had to reach.
	DC int `json:"dc"`

	// Door is the door as it now stands: open when beaten — a beaten lock
	// OPENS the door, it does not merely unlock it — unchanged and
	// retryable when not.
	Door Door `json:"door"`

	// Discovered is what a beaten lock brought into view.
	Discovered map[string]Discovery `json:"discovered,omitempty"`

	// Formed is present when what the opened door revealed started a fight.
	Formed *Formed `json:"formed,omitempty"`

	// Seq is the door beat's sequence number.
	Seq uint64 `json:"seq"`

	Saved    SaveReport     `json:"saved"`
	Delivery DeliveryReport `json:"delivery"`
}

// Unlock tries a locked door as Member: an ability check against the lock's
// authored DC, rolled HERE.
//
// The modifier is the member's ability modifier for the lock's authored
// ability (the reference tomb's is "dex") — tool proficiency is shelved with
// the tomb's authoring, which names no tool (rpg-project#269 §6.4). A failed
// attempt is an outcome, not an error: the door is unchanged, the attempt is
// narrated, and the caller may try again.
func (m *Manager) Unlock(ctx context.Context, in *UnlockInput) (*UnlockOutput, error) {
	if in == nil {
		return nil, fmt.Errorf("unlock: %w", ErrNilInput)
	}
	if in.Member == "" {
		return nil, fmt.Errorf("unlock: %w", ErrNoMemberID)
	}
	if in.Door == "" {
		return nil, fmt.Errorf("unlock: %w", ErrNoConnection)
	}

	scope, err := m.openForWrite(ctx, in.Session)
	if err != nil {
		return nil, fmt.Errorf("unlock: %w", err)
	}

	// The lock is read before anything rolls: the DC and the ability are
	// the composition's facts, and a door that is not locked is refused by
	// the composition itself below, in its own words.
	var lock encounter.Lock
	var locked bool
	for _, d := range scope.enc.Doors() {
		if d.ID == in.Door {
			lock, locked = d.State.Lock()
			break
		}
	}

	beaten, total := false, 0
	if locked {
		sheet, err := m.loadAttackSheet(ctx, in.Member)
		if err != nil {
			return nil, fmt.Errorf("unlock: %w", err)
		}

		check, err := checks.MakeAbilityCheck(ctx, &checks.AbilityCheckInput{
			Roller:   &diceSeam{roller: m.dice},
			DC:       lock.DC,
			Modifier: sheet.GetAbilityModifier(abilities.Ability(lock.Ability)),
		})
		if err != nil {
			// A foreign error (rpgerr): carried as text, never wrapped into
			// our chain (translate's own law).
			return nil, fmt.Errorf("unlock %q: check failed: %v", in.Door, err)
		}
		beaten, total = check.Success, check.Total
	}

	unlocked, err := scope.enc.Unlock(&encounter.UnlockInput{
		Door:   in.Door,
		Beaten: beaten,
		Actor:  encounter.MemberID(in.Member),
		Total:  total,
	})
	if err != nil {
		return nil, fmt.Errorf("unlock: %w", translate(err))
	}

	down, err := discoveryStanding(scope)
	if err != nil {
		return nil, fmt.Errorf("unlock: %w", err)
	}

	report, delivery, err := m.commit(ctx, scope)
	if err != nil {
		return nil, fmt.Errorf("unlock: %w", err)
	}

	return &UnlockOutput{
		Beaten:     unlocked.Beaten,
		Total:      total,
		DC:         unlocked.DC,
		Door:       Door{ID: unlocked.Door, State: string(unlocked.State)},
		Discovered: projectDiscoveries(unlocked.IntelDeltas, down),
		Formed:     projectFormed(unlocked.Formed),
		Seq:        unlocked.Seq,
		Saved:      report,
		Delivery:   delivery,
	}, nil
}

// projectDoor is the seam's one projection of a door: the sealed state
// interface becomes a string kind, and the lock rides only while it is real.
func projectDoor(d encounter.Door) Door {
	out := Door{ID: d.ID, State: string(d.State.Kind())}
	if lock, locked := d.State.Lock(); locked {
		out.Lock = &DoorLock{DC: lock.DC, Ability: lock.Ability, Tool: lock.Tool}
	}
	return out
}
