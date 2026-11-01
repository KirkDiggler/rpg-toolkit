// Copyright (C) 2026 Kirk Diggler
// SPDX-License-Identifier: GPL-3.0-or-later

package machine_test

import (
	"testing"

	"github.com/stretchr/testify/assert"

	"github.com/KirkDiggler/rpg-toolkit/mind/machine"
)

func TestDue_PopOrderAndRearm(t *testing.T) {
	var d machine.Due[string]
	assert.Empty(t, d.Pop(100), "the zero heap is empty and usable")

	d.Arm("orc", 3)
	d.Arm("bat", 5)
	d.Arm("cat", 3)
	d.Arm("ant", 3)
	d.Arm("elk", 9)
	assert.Equal(t, 5, d.Len())

	d.Arm("bat", 2) // re-arm replaces, earlier
	d.Arm("elk", 4) // re-arm replaces, earlier
	d.Arm("ant", 6) // re-arm replaces, later
	assert.Equal(t, 5, d.Len(), "arming a key twice keeps one entry")

	assert.Empty(t, d.Pop(1))
	assert.Equal(t, []string{"bat", "cat", "orc"}, d.Pop(3), "by round, then by key")
	assert.Equal(t, 2, d.Len(), "popped keys are removed")

	d.Disarm("elk")
	d.Disarm("nobody")
	assert.Equal(t, []string{"ant"}, d.Pop(10))
	assert.Zero(t, d.Len())

	// A popped key re-armed comes back.
	d.Arm("orc", 11)
	assert.Equal(t, []string{"orc"}, d.Pop(11))
}

func TestDue_ManyKeysPopSorted(t *testing.T) {
	var d machine.Due[int]
	for k := 99; k >= 0; k-- {
		d.Arm(k, uint64(k%7))
	}
	got := d.Pop(6)
	assert.Len(t, got, 100)
	for i := 1; i < len(got); i++ {
		a, b := got[i-1], got[i]
		assert.True(t, a%7 < b%7 || (a%7 == b%7 && a < b), "out of order at %d: %d then %d", i, a, b)
	}
}
