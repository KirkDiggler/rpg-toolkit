// Copyright (C) 2024 Kirk Diggler
// SPDX-License-Identifier: GPL-3.0-or-later

package encounter

import (
	"testing"

	"github.com/KirkDiggler/rpg-toolkit/encounter/core"
	"github.com/stretchr/testify/require"
)

// TestValidateAuthoredEdgeOverlay_RejectsConnectorDoorCollision exercises the
// defense-in-depth branch for a future generator that exposes a connector
// physical edge whose endpoints both happen to be semantic floor cells. The
// current v1 connector column is rejected earlier as non-floor; this production
// helper is still the authoritative no-overlay rule if that geometry evolves.
func TestValidateAuthoredEdgeOverlay_RejectsConnectorDoorCollision(t *testing.T) {
	from := core.Hex{Q: 0, R: 0, S: 0}
	to := core.Hex{Q: 1, R: -1, S: 0}
	err := validateAuthoredEdgeOverlay([]generatedEdgeRecord{{
		edge: GeneratedEdge{From: from, To: to, Kind: GeneratedEdgeKindDoor, DoorID: "connector-door"},
	}}, []AuthoredEdge{{From: from, To: to, Kind: GeneratedEdgeKindSolid}})
	require.Error(t, err)
	require.ErrorContains(t, err, "connector-derived")
}
