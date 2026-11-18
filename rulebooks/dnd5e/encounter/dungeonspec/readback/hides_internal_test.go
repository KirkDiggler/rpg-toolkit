// Copyright (C) 2026 Kirk Diggler
// SPDX-License-Identifier: GPL-3.0-or-later

package readback

import (
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/encounter"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/encounter/dungeonspec"
)

// TestASecretConnectsTheDoorsAndPropsItHides: the site dialect cannot yet
// author a hidden door (`concealments.<id>.doors` is refused as a key it
// does not read), but the compiled concealment carries doors, so the
// statement is held to connect them beside its props.
func TestASecretConnectsTheDoorsAndPropsItHides(t *testing.T) {
	b := &builder{c: &dungeonspec.Compiled{Key: "site"}}
	st := b.concealmentStatement(encounter.ConcealmentInput{
		ID:     "site/spyhole",
		Doors:  []encounter.DoorID{"site/gate-door"},
		Props:  []encounter.PropID{"site/grate"},
		Checks: []encounter.CheckApproach{{Ability: "perception", DC: 14}},
	})
	require.Equal(t, []Connection{
		{Kind: "door", ID: "gate-door", Role: "hides"},
		{Kind: "prop", ID: "grate", Role: "hides"},
	}, st.Connects)
}
