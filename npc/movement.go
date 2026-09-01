// Copyright (C) 2026 Kirk Diggler
// SPDX-License-Identifier: GPL-3.0-or-later

package npc

import "fmt"

// BlocksMovement maps this movement policy to today's binary spatial
// occupancy seam.
//
// This is an adapter helper, not the full NPC movement model. If a future
// policy needs mover-vs-occupant context, this method should return an error
// until the caller uses a richer adapter.
func (p MovementPolicy) BlocksMovement() (bool, error) {
	switch p {
	case "":
		return false, ErrNoMovementPolicy
	case MovementPolicyBlocking:
		return true, nil
	case MovementPolicyPassable:
		return false, nil
	default:
		return false, fmt.Errorf("%w: %q", ErrUnknownMovementPolicy, p)
	}
}
