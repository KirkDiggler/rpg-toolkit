// Copyright (C) 2026 Kirk Diggler
// SPDX-License-Identifier: GPL-3.0-or-later

package refs_test

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/refs"
)

// The untrained rule's ref is a singleton, so a roll source built here and one
// built in resolution compare equal by pointer (the namespace's own contract).
func TestUntrainedRefIsASingleton(t *testing.T) {
	require.NotNil(t, refs.Rules.Untrained())
	assert.Same(t, refs.Rules.Untrained(), refs.Rules.Untrained())
}

// It reads as a rule, not as content somebody carries: nothing equips an
// "untrained", and the type segment is what says so on the wire and in a log.
func TestUntrainedRefIsCanonical(t *testing.T) {
	assert.Equal(t, "dnd5e:rules:untrained", refs.Rules.Untrained().String())
}
