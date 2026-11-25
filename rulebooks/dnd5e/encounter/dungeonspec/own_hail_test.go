// Copyright (C) 2026 Kirk Diggler
// SPDX-License-Identifier: GPL-3.0-or-later

package dungeonspec_test

// own_hail_test.go is A CREATURE HAILS FOR ITSELF, FROM ITS OWN MACHINE
// (rpg-project#1139, slice rpg-project#1140), as an author writes it: a
// creature with no conversation writes `on: approached` with its own `within`.
// The refusal of a step with no distance is not in this slice (reported on
// the issue: it breaks TestTheSeizureSaysTheHailsDistance).

import (
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/encounter/dungeonspec"
)

// ownHailStates is a guard who stops whoever comes near; when is the step's
// own guard, written as an author writes it.
func ownHailStates(when string) string {
	return `        in-hall:
          transitions:
            - on: approached
              when: ` + when + `
              to: stopped
              reason: someone came near
        stopped: {}
`
}

func TestDungeonspec_OwnHail_AStepWithItsOwnWithinCompiles(t *testing.T) {
	compiled := loadSource(t, valueSiteWith(ownHailStates("{ scope: { within: 2 } }"), "", topicBar))
	require.NotNil(t, monsterNamed(t, compiled, "goblin-1").Behaviors, "the creature compiled with its machine")
}

func TestDungeonspec_OwnHail_AStepAmongOthersCompiles(t *testing.T) {
	when := "{ all: [{ scope: { within: 2 } }, { fact: { name: stance, op: eq, value: neutral } }] }"
	loadSource(t, valueSiteWith(ownHailStates(when), "", topicBar))
}

func TestDungeonspec_OwnHail_TheCastleStillCompiles(t *testing.T) {
	_, err := dungeonspec.Load(readFixture(t, "testdata/world-builder-v4-castle.yaml"))
	require.NoError(t, err, "no castle creature is refused: each carries the gate conversation")
}
