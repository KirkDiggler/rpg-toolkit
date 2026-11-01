// Copyright (C) 2026 Kirk Diggler
// SPDX-License-Identifier: GPL-3.0-or-later

package machine_test

import (
	"testing"

	"github.com/stretchr/testify/assert"

	"github.com/KirkDiggler/rpg-toolkit/mind/machine"
)

func TestListeners_SetDropFor(t *testing.T) {
	var l machine.Listeners[string]
	assert.Empty(t, l.For(beatHeard), "the zero index is empty and usable")

	near := &machine.ScopeLeaf{Within: 2}
	l.Set("orc", []machine.Listen{{Beat: beatHeard, Scope: near}, {Beat: beatBell}})
	l.Set("bat", []machine.Listen{{Beat: beatHeard}})
	l.Set("cat", []machine.Listen{{Beat: beatBell}})

	assert.Equal(t, []machine.Listener[string]{{Key: "bat"}, {Key: "orc", Scope: near}}, l.For(beatHeard), "in key order")
	assert.Equal(t, []machine.Listener[string]{{Key: "cat"}, {Key: "orc"}}, l.For(beatBell))

	// Set replaces what a key listened to.
	l.Set("orc", []machine.Listen{{Beat: beatBell}})
	assert.Equal(t, []machine.Listener[string]{{Key: "bat"}}, l.For(beatHeard))
	assert.Equal(t, []machine.Listener[string]{{Key: "cat"}, {Key: "orc"}}, l.For(beatBell))

	// A slice from For stays as it was while the index changes under it, so
	// a host may step the listeners it holds and re-set them as it goes.
	held := l.For(beatBell)
	l.Drop("cat")
	l.Set("ant", []machine.Listen{{Beat: beatBell}})
	assert.Equal(t, []machine.Listener[string]{{Key: "cat"}, {Key: "orc"}}, held)
	assert.Equal(t, []machine.Listener[string]{{Key: "ant"}, {Key: "orc"}}, l.For(beatBell))

	// Drop, and Set to nothing, both forget the key.
	l.Drop("orc")
	l.Set("bat", nil)
	l.Drop("nobody")
	assert.Empty(t, l.For(beatHeard))
	assert.Equal(t, []machine.Listener[string]{{Key: "ant"}}, l.For(beatBell))

	// A repeated beat in one Set keeps the first.
	l.Set("elk", []machine.Listen{{Beat: beatHeard, Scope: near}, {Beat: beatHeard}})
	assert.Equal(t, []machine.Listener[string]{{Key: "elk", Scope: near}}, l.For(beatHeard))
}
