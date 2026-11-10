// Copyright (C) 2026 Kirk Diggler
// SPDX-License-Identifier: GPL-3.0-or-later

package resolution

import (
	"context"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/encounter"
)

// withWall is world on a site that carries one more wall. The field is site
// content the checkpoint names by hash (rpg-project#879 S5), so a test that
// wants a different field compiles a different site — through the process
// default the load resolves from — and points the world at it.
func withWall(t *testing.T, world *encounter.EncounterData, wall encounter.BoundaryData) {
	t.Helper()
	ctx := context.Background()
	held, err := encounter.DefaultSites().Site(ctx, &encounter.SiteInput{Hash: world.SiteHash})
	require.NoError(t, err)
	data := held.Site.Data()
	data.Field.Walls = append(data.Field.Walls, wall)
	walled, err := encounter.DefaultSites().Site(ctx, &encounter.SiteInput{Data: &data})
	require.NoError(t, err)
	require.NotEqual(t, world.SiteHash, walled.Site.Hash(), "a wall is different content")
	world.SiteHash = walled.Site.Hash()
}
