// Copyright (C) 2026 Kirk Diggler
// SPDX-License-Identifier: GPL-3.0-or-later
package resolution

import (
	dndEvents "github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/events"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/refs"
	"github.com/stretchr/testify/require"
	"testing"
)

type sightFixture map[string]bool

func (s sightFixture) SeesWithin(a, b string, _ int) (bool, bool) { v, ok := s[a+"/"+b]; return v, ok }
func TestSightAttackModifiers(t *testing.T) {
	for _, tc := range []struct {
		name     string
		sight    sightFixture
		adv, dis int
	}{
		{"clear", sightFixture{"a/b": true, "b/a": true}, 0, 0},
		{"unseen target", sightFixture{"a/b": false, "b/a": true}, 0, 1},
		{"unseen attacker", sightFixture{"a/b": true, "b/a": false}, 1, 0},
		{"both obscured cancel", sightFixture{"a/b": false, "b/a": false}, 1, 1},
		{"unknown is not blind", sightFixture{}, 0, 0},
	} {
		t.Run(tc.name, func(t *testing.T) {
			e := dndEvents.AttackChainEvent{}
			addSightAttackModifiers(tc.sight, &e, "a", "b", 100, refs.Spells.GuidingBolt())
			require.Len(t, e.AdvantageSources, tc.adv)
			require.Len(t, e.DisadvantageSources, tc.dis)
		})
	}
}
