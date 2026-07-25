// Copyright (C) 2024 Kirk Diggler
// SPDX-License-Identifier: GPL-3.0-or-later

package dungeonspec_test

import (
	"testing"

	"github.com/KirkDiggler/rpg-toolkit/encounter/dungeonspec"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestDecode_RoundTripsTheReferenceSpec(t *testing.T) {
	spec, err := dungeonspec.Decode([]byte(referenceYAML))
	require.NoError(t, err)
	assert.Equal(t, 1, spec.Version)
	assert.Equal(t, "sunken-crypt", spec.Key)
	assert.Equal(t, 8, spec.Height)
	require.Len(t, spec.Rooms, 4)
	assert.Equal(t, "entrance", spec.Rooms[0].ID)
	require.Len(t, spec.Connectors, 3)
	require.NotNil(t, spec.Connectors[2].Locked)
	assert.Equal(t, 12, spec.Connectors[2].Locked.DC)
}

func TestDecode_UnknownFieldFailsLoudly(t *testing.T) {
	// Deliberate typos below — the test is that this exact typo class dies
	// at decode, so misspell must not flag (or "fix") either string.
	const (
		typoYAML  = "version: 1\nmosnters: []\n" //nolint:misspell
		typoField = "mosnter"                    //nolint:misspell
	)
	_, err := dungeonspec.Decode([]byte(typoYAML))
	require.Error(t, err) // the typo class the design promises dies at load
	assert.Contains(t, err.Error(), typoField)
	assert.Contains(t, err.Error(), "line 2") // the field+line promise: names both
}

func TestDecode_MultiDocumentYAMLRejected(t *testing.T) {
	// A stray `---` second document after a valid first one. yaml.v3
	// otherwise decodes the first document and silently discards the rest —
	// the same silent-drop class KnownFields already closes for unknown
	// fields, closed here deliberately for multi-document input.
	multiDoc := referenceYAML + "\n---\nversion: 1\n"
	_, err := dungeonspec.Decode([]byte(multiDoc))
	require.Error(t, err)
	assert.Contains(t, err.Error(), "multi-document YAML not supported")
}

func TestDecode_EmptyInputReturnsFriendlyError(t *testing.T) {
	_, err := dungeonspec.Decode([]byte(""))
	require.Error(t, err)
	assert.Contains(t, err.Error(), "empty dungeon spec")
}

func TestDecode_PlaceBlockRoundTrips(t *testing.T) {
	spec, err := dungeonspec.Decode([]byte(placedTombYAML))
	require.NoError(t, err)
	tomb := spec.Rooms[1]
	require.Len(t, tomb.Place, 6) // coffin, altar, statue-reaper, brazier x2, skeleton
	assert.Equal(t, "dnd5e:props:coffin", tomb.Place[0].Ref)
	assert.Equal(t, [2]int{6, 3}, tomb.Place[0].At)
	require.NotNil(t, tomb.Place[0].BlocksLoS)
	assert.False(t, *tomb.Place[0].BlocksLoS)
	assert.Equal(t, "dnd5e:monsters:skeleton", tomb.Place[5].Ref)
	require.NotNil(t, tomb.Boss.At)
	assert.Equal(t, [2]int{7, 5}, *tomb.Boss.At)
}
