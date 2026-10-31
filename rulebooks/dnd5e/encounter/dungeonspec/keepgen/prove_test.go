// Copyright (C) 2026 Kirk Diggler
// SPDX-License-Identifier: GPL-3.0-or-later

package keepgen

import (
	"strings"
	"testing"

	"github.com/stretchr/testify/require"
)

// TestProve_RefusesUnreachable: a plan whose doors leave the hidden stash
// with none is refused, naming the room, and no bytes come back.
func TestProve_RefusesUnreachable(t *testing.T) {
	p, err := LoadPlan(strings.NewReader(planWith(t, "  - [treasury, hidden-stash]\n", "")))
	require.NoError(t, err)
	doc, _, err := Generate(p, DefaultOptions())
	require.ErrorContains(t, err, `region "hidden-stash" is unreachable from the party start through the doors`)
	require.Nil(t, doc)
}

// seatsSite is a loadable site whose start region is one cell, walled off
// from a second region: the loader seats one.
const seatsSite = `version: 5
key: one-seat
play: { void: transparent, lighting: bright, standing: centre-covered }
room:
  version: 3
  id: room-1
  name: One Seat
  coordinateFrame: { horizontalPlane: world-xz, verticalAxis: world-y-up, distanceUnit: world-scene-unit, hexRadius: 1, footprintFrame: owner-local-xz }
  scene: { version: 3, id: scene-1, name: One Seat, items: [], groups: [], mapLabels: [] }
  room:
    implicitRegionId: hall
    walls:
      - id: seam
        label: seam
        line: { start: { x: 1.299, z: -0.75 }, end: { x: 1.299, z: 0.75 } }
        appearance: { assetRef: 'dnd5e:env:test:wall', height: 3, thickness: 0.3, elevation: 0 }
        blocker: { blocksMovement: true, blocksLineOfSight: true, footprint: { width: 1.5, depth: 0.2, offsetX: 0, offsetZ: 0 } }
        openings: []
    regions:
      - id: closet
        name: Closet
        lighting: { intensity: 1 }
        cells:
          - { r: 0, q: [[0, 0]] }
      - id: hall
        name: Hall
        lighting: { intensity: 1 }
        cells:
          - { r: 0, q: [[1, 6]] }
    propDeclarations: {}
    arrangementDeclarations: {}
    monsterDeclarations: []
    partyStart: { q: 0, r: 0 }
    doorBindings: {}
`

// TestProve_RefusesSeats: a start that seats fewer than four, counted from
// the loader's own compiled.PartyStart.
func TestProve_RefusesSeats(t *testing.T) {
	_, err := Prove([]byte(seatsSite))
	require.ErrorContains(t, err, "the party start seats 1, fewer than 4")
}

// TestProve_RefusesALoadFailure: bytes the loader refuses.
func TestProve_RefusesALoadFailure(t *testing.T) {
	_, err := Prove([]byte("version: 5\nkey: nothing\n"))
	require.ErrorContains(t, err, "the site does not load")
}
