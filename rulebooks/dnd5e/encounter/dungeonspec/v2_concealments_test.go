// Copyright (C) 2026 Kirk Diggler
// SPDX-License-Identifier: GPL-3.0-or-later

package dungeonspec_test

// v2_concealments_test.go is THE v2 ADAPTER'S OWN RULES, as claims
// (rpg-project#490, E5).
//
// [encounter.ConcealmentInput] knows nothing about regions, suites or scenery
// strips. It knows an id, some checks, and three lists of what belongs to it.
// The rules pinned here exist because ONE v2 DOCUMENT can say a thing the one
// noun cannot — a hidden room per flag, several of them behind one secret
// door — and something has to decide what such a document MEANS.
//
// That decision is the adapter's (concealments.go), and these are the claims
// it makes. Pinned on small inline documents rather than on the shipped
// content, because the shipped tomb exercises exactly one of them: a lowering
// nothing authors is a behaviour, and a behaviour nobody wrote down is one
// the next reader has to re-derive from the code.

import (
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/encounter"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/encounter/dungeonspec"
)

// suiteDoc is a visible hall and TWO hidden rooms behind one secret door.
//
//	hall [2,0] | panel | inner [3,0] -- (plain gap) -- deeper [4,0]
//
// The hall|inner crossing carries a wall with the concealed `panel` in it.
// The inner|deeper crossing carries nothing at all — a bare way, wholly
// inside hidden space, which the v2 coherence walk calls "nobody's business".
const suiteDoc = `
version: 2
key: suite
orientation: pointy
void: opaque
regions:
  - id: hall
    archetype: crypt
    lighting: { intensity: 1 }
    cells:
      - [[2,0]]
  - id: inner
    archetype: crypt
    lighting: { intensity: 1 }
    concealed: true
    cells:
      - [[3,0]]
  - id: deeper
    archetype: crypt
    lighting: { intensity: 1 }
    concealed: true
    cells:
      - [[4,0]]
start: [2, 0]
walls:
  - start: { cell: [3,0], offset: [-0.5, 0] }
    end:   { cell: [2,1], offset: [0, 0] }
doors:
  - id: panel
    at: { cell: [3,0], offset: [-0.5, 0] }
    closed: true
    concealed: [{ ability: perception, dc: 15 }]
`

func v2Compiled(t *testing.T, raw string) dungeonspec.Compiled {
	t.Helper()
	compiled, err := dungeonspec.Load([]byte(raw))
	require.NoError(t, err, "the document compiles")

	return compiled
}

func cellCount(c encounter.ConcealmentInput) int { return len(c.Cells) }

// TestASecretSuiteLowersToOneConcealment is the merge rule: concealed regions
// joined by a way through hidden space are ONE secret.
//
// WHY IT IS NOT THE LITERAL READING. Taken room by room, `deeper` would lower
// to a concealment of its own with no concealed door touching it — and so no
// checks, which the composition refuses by name. The document is legal v2
// (the coherence walk says a way wholly inside hidden space is nobody's
// business), so the adapter has to mean something by it, and "a suite of
// hidden rooms behind one secret door is one secret" is what one noun can
// hold.
//
// IT COLLAPSES A KNOWLEDGE MOMENT, which is the primitive's own ruling rather
// than an accident here: finding the panel gives you both rooms. Under the
// two flags each room arrived separately, as its own door was perceived open.
func TestASecretSuiteLowersToOneConcealment(t *testing.T) {
	compiled := v2Compiled(t, suiteDoc)

	require.Len(t, compiled.Concealments, 1, "two hidden rooms joined through hidden space are one secret")
	hidden := compiled.Concealments[0]
	require.Equal(t, encounter.ConcealmentID("suite/inner"), hidden.ID,
		"minted from the FIRST merged room in authored order, so the id does not depend on door order")
	require.Equal(t, 2, cellCount(hidden), "both rooms' floor hides with it")
	require.Equal(t, []encounter.DoorID{"suite/panel"}, hidden.Doors)
	require.Len(t, hidden.Checks, 1, "the one way in, and it is the whole of what finds the suite")
}

// TestTwoSecretsEitherSideOfAPublicHallStayTwo is the other side of the same
// rule: THE FLOOD ONLY EVER ENTERS HIDDEN SPACE.
//
// Two hidden rooms, each behind its own secret door off one visible hall, and
// not touching each other. Nothing joins them but floor anybody can walk, so
// they are two secrets and each is found by its own check. A flood that
// stepped onto visible floor would merge them — and would go on merging until
// one find check had the run of the whole dungeon.
func TestTwoSecretsEitherSideOfAPublicHallStayTwo(t *testing.T) {
	compiled := v2Compiled(t, `
version: 2
key: apart
orientation: pointy
void: opaque
regions:
  - id: hall
    archetype: crypt
    lighting: { intensity: 1 }
    cells:
      - [[2,0]]
  - id: west-vault
    archetype: crypt
    lighting: { intensity: 1 }
    concealed: true
    cells:
      - [[1,0]]
  - id: east-vault
    archetype: crypt
    lighting: { intensity: 1 }
    concealed: true
    cells:
      - [[3,0]]
start: [2, 0]
walls:
  - start: { cell: [3,0], offset: [-0.5, 0] }
    end:   { cell: [2,1], offset: [0, 0] }
  - start: { cell: [2,0], offset: [-0.5, 0] }
    end:   { cell: [1,1], offset: [0, 0] }
doors:
  - id: east-panel
    at: { cell: [3,0], offset: [-0.5, 0] }
    closed: true
    concealed: [{ ability: perception, dc: 15 }]
  - id: west-panel
    at: { cell: [2,0], offset: [-0.5, 0] }
    closed: true
    concealed: [{ ability: investigation, dc: 17 }]
`)

	require.Len(t, compiled.Concealments, 2, "a hall anybody can walk is not a way between two secrets")
	require.Equal(t, encounter.ConcealmentID("apart/west-vault"), compiled.Concealments[0].ID,
		"in authored region order")
	require.Equal(t, encounter.ConcealmentID("apart/east-vault"), compiled.Concealments[1].ID)
	require.Equal(t, []encounter.DoorID{"apart/west-panel"}, compiled.Concealments[0].Doors)
	require.Equal(t, []encounter.DoorID{"apart/east-panel"}, compiled.Concealments[1].Doors,
		"each room is found by its own door's check, and finding one says nothing about the other")
	for _, c := range compiled.Concealments {
		require.Equal(t, 1, cellCount(c))
		require.Len(t, c.Checks, 1)
	}
}

// TestADoorAcrossSceneryGuardsTheRoomAtTheFarEnd is the through-scenery rule.
//
// A way in v2 may run THROUGH scenery — floor that belongs to nobody — so the
// door that hides a room need not stand on that room's own edge. Reading only
// the door's two endpoint cells would leave the room with no check anywhere
// and hand the composition a secret nobody could ever find, which is the
// thing "a concealment lists no way to find it" refuses one layer down.
func TestADoorAcrossSceneryGuardsTheRoomAtTheFarEnd(t *testing.T) {
	compiled := v2Compiled(t, `
version: 2
key: strip
orientation: pointy
void: opaque
regions:
  - id: hall
    archetype: crypt
    lighting: { intensity: 1 }
    cells:
      - [[2,0]]
  - id: vault
    archetype: crypt
    lighting: { intensity: 1 }
    concealed: true
    cells:
      - [[4,0]]
scenery:
  - [[3,0]]
start: [2, 0]
walls:
  - start: { cell: [3,0], offset: [-0.5, 0] }
    end:   { cell: [2,1], offset: [0, 0] }
doors:
  - id: panel
    at: { cell: [3,0], offset: [-0.5, 0] }
    closed: true
    concealed: [{ ability: perception, dc: 15 }]
`)

	require.Len(t, compiled.Concealments, 1)
	hidden := compiled.Concealments[0]
	require.Equal(t, encounter.ConcealmentID("strip/vault"), hidden.ID,
		"the secret is the ROOM, not the crossing — the door only guards it")
	require.Equal(t, []encounter.DoorID{"strip/panel"}, hidden.Doors,
		"a door standing on the strip, one cell short of the room, is still the room's way in")
	require.Len(t, hidden.Checks, 1, "so the room has a check, which is what makes it findable at all")
	require.Equal(t, 1, cellCount(hidden))
}

// The fourth rule — a concealed door guarding no hidden room lowers to a
// cell-less concealment under the door's own compiled id — is pinned on the
// shipped tomb instead, where the shortcut door stands INSIDE one visible
// region and a second inline document would only be a smaller copy of it:
// see TestAConcealedDoorLowersToAConcealment in compile_test.go.
