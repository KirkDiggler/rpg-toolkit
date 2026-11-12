// Copyright (C) 2026 Kirk Diggler
// SPDX-License-Identifier: GPL-3.0-or-later

package effects

import (
	"slices"

	"github.com/KirkDiggler/rpg-toolkit/core"
	"github.com/KirkDiggler/rpg-toolkit/mechanics/effects/machine"
)

// Snapshot is a condition's mutable lifecycle state as owned values. It contains
// no recipe, applying source or rulebook bindings; their owner stores them beside
// this state. Bag values are the machine's validated bool, int or string scalars.
type Snapshot struct {
	State string      `json:"state"`
	Bag   machine.Bag `json:"bag"`
}

// Snapshot returns the current state and a detached bag copy. Reading it neither
// attaches nor mutates the condition; retaining or editing it does not change or
// follow the live state. It also represents terminal state for inspection; the
// consumer's load contract decides which states it accepts for restoration.
func (c *Composed) Snapshot() Snapshot {
	return Snapshot{State: c.m.State(), Bag: c.m.Bag()}
}

// CurrentOutputs returns a defensive slice of the current state's observational
// handles, in recipe order. The handles expose no mutation operation; Params and
// Host return copies. Their guards read the live bag, so fetch current outputs
// again after a state transition and use Snapshot to retain lifecycle state.
// During End-before-detach, the previous state's outputs still answer, just as
// subscribed contributions do. After end reporting, terminal state has no outputs.
// Reading does not attach or publish; Snapshot always reports actual machine state.
func (c *Composed) CurrentOutputs() []*Bound {
	state := c.m.State()
	if c.endState != nil {
		state = *c.endState
	}
	if c.m.Program().IsTerminal(state) {
		return nil
	}
	return slices.Clone(c.outputs[state])
}

func cloneRef(ref *core.Ref) *core.Ref {
	if ref == nil {
		return nil
	}
	copy := *ref
	return &copy
}
