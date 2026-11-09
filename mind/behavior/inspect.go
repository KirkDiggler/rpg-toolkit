// Copyright (C) 2026 Kirk Diggler
// SPDX-License-Identifier: GPL-3.0-or-later

package behavior

import (
	"errors"
	"fmt"
	"math"
	"slices"

	"github.com/KirkDiggler/rpg-toolkit/mind/machine"
)

// ErrBadWakeDelay reports a wake shift that would overflow the round counter.
// No instance changes when any wake cannot be shifted.
var ErrBadWakeDelay = errors.New("behavior: invalid wake delay")

// InspectComposerInput supplies the caller's current round. Inspect does not
// advance that round, consume a wake or deliver an input.
type InspectComposerInput struct {
	Now uint64
}

// InstanceStatus is detached, derived state for one attached instance. Listens
// retains that instance's non-timed scopes; Timed names its distinct current-state
// timed beats in authored transition order. DueAt follows Program.DueAt and does
// not choose which timed opportunity the caller should offer. WakeAt is the
// stored wake, even when the instance is terminal and therefore not due.
type InstanceStatus struct {
	ID         string
	State      string
	Definition string
	Listens    []machine.Listen
	Timed      []machine.Beat
	DueAt      *uint64
	WakeAt     *uint64
}

// InspectComposerOutput ranges over all attached instances, in authored order.
// It is a snapshot, not another live progress store or a global scheduling index.
type InspectComposerOutput struct {
	Instances []InstanceStatus
}

// Inspect returns current state, definition identity and derived scheduling
// information without stepping, rolling or invoking vocabulary callbacks. Every
// returned slice and pointer is detached. Nil input returns ErrNilInput.
func (c *Composer) Inspect(in *InspectComposerInput) (*InspectComposerOutput, error) {
	if in == nil {
		return nil, ErrNilInput
	}
	out := &InspectComposerOutput{Instances: make([]InstanceStatus, 0, len(c.instances))}
	for i := range c.instances {
		instance := &c.instances[i]
		status := InstanceStatus{
			ID: instance.spec.ID, State: instance.blob.State, Definition: instance.definition,
			Listens: instance.program.Listens(&instance.blob), WakeAt: copyRound(instance.blob.WakeAt),
		}
		if at, due := instance.program.DueAt(&instance.blob, in.Now); due {
			status.DueAt = &at
		}
		state := instance.spec.Recipe.States[instance.blob.State]
		for _, transition := range state.Transitions {
			if c.beats[transition.On].Timed && !slices.Contains(status.Timed, transition.On) {
				status.Timed = append(status.Timed, transition.On)
			}
		}
		out.Instances = append(out.Instances, status)
	}
	return out, nil
}

// InstanceStateInput names one attachment whose state the caller needs.
type InstanceStateInput struct {
	ID string
}

// InstanceStateOutput returns immutable state/definition names without exposing
// a blob, bag, program or mutable configuration.
type InstanceStateOutput struct {
	State      string
	Definition string
}

// StateOf reads one instance in constant time, without deriving all scheduling
// information. Nil input returns ErrNilInput; an unknown ID wraps ErrUnknownInstance.
func (c *Composer) StateOf(in *InstanceStateInput) (*InstanceStateOutput, error) {
	if in == nil {
		return nil, ErrNilInput
	}
	index, exists := c.byID[in.ID]
	if !exists {
		return nil, fmt.Errorf("%w: %q", ErrUnknownInstance, in.ID)
	}
	instance := &c.instances[index]
	return &InstanceStateOutput{State: instance.blob.State, Definition: instance.definition}, nil
}

// DelayWakesInput shifts existing wakes by a caller-chosen span. It expresses
// the caller's time-jump policy, not a clock owned or advanced by the composer.
type DelayWakesInput struct {
	Span uint64
}

// DelayWakesOutput counts wake values actually changed. Zero span changes none.
type DelayWakesOutput struct {
	Shifted int
}

// DelayWakes moves every non-nil wake, leaving states, bags and anchors alone.
// Overflow is checked for all instances before any change. Nil input returns
// ErrNilInput; overflow wraps ErrBadWakeDelay and names the refusing instance.
func (c *Composer) DelayWakes(in *DelayWakesInput) (*DelayWakesOutput, error) {
	if in == nil {
		return nil, ErrNilInput
	}
	out := &DelayWakesOutput{}
	if in.Span == 0 {
		return out, nil
	}
	for i := range c.instances {
		instance := &c.instances[i]
		if wake := instance.blob.WakeAt; wake != nil && *wake > math.MaxUint64-in.Span {
			return nil, fmt.Errorf("%w: instance %q wake would overflow", ErrBadWakeDelay, instance.spec.ID)
		}
	}
	for i := range c.instances {
		blob := &c.instances[i].blob
		if blob.WakeAt != nil {
			at := *blob.WakeAt + in.Span
			blob.WakeAt = &at
			out.Shifted++
		}
	}
	return out, nil
}

func copyRound(value *uint64) *uint64 {
	if value == nil {
		return nil
	}
	out := *value
	return &out
}
