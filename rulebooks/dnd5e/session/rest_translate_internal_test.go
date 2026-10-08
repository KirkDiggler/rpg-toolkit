// Copyright (C) 2026 Kirk Diggler
// SPDX-License-Identifier: GPL-3.0-or-later

package session

import (
	"fmt"
	"testing"

	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/resolution"
	"github.com/stretchr/testify/require"
)

// TestARestThatTheRunBrokeIsAnInvalidSession: a world with no die, and a hold
// whose effect sits on a member the run does not hold, are the run's fault —
// neither the request nor any one sheet is bad.
func TestARestThatTheRunBrokeIsAnInvalidSession(t *testing.T) {
	for name, cause := range map[string]error{
		"no die":               resolution.ErrNoRoller,
		"a hold on a stranger": resolution.ErrBadParticipant,
	} {
		t.Run(name, func(t *testing.T) {
			err := translateRest("bob", fmt.Errorf("resolution: short rest: %w", cause))
			require.ErrorIs(t, err, ErrInvalidSession)
			require.NotErrorIs(t, err, ErrBadCharacter)
			require.NotErrorIs(t, err, ErrBadRest)
		})
	}
}
