// Copyright (C) 2026 Kirk Diggler
// SPDX-License-Identifier: GPL-3.0-or-later

package session

import (
	"context"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/encounter"
)

// withSiteWalls points world at a site that carries these walls as well. The
// field is site content the checkpoint names by hash (rpg-project#879 S5), so
// a test that wants a different field compiles a different site through the
// process default every load here resolves from.
func withSiteWalls(t *testing.T, world *encounter.EncounterData, walls ...encounter.BoundaryData) {
	t.Helper()
	editSite(t, world, func(field *encounter.FieldData) {
		field.Walls = append(field.Walls, walls...)
	})
}

// editSite points world at the site its own site becomes under edit.
func editSite(t *testing.T, world *encounter.EncounterData, edit func(*encounter.FieldData)) {
	t.Helper()
	ctx := context.Background()
	held, err := encounter.DefaultSites().Site(ctx, &encounter.SiteInput{Hash: world.SiteHash})
	require.NoError(t, err)
	data := held.Site.Data()
	edit(&data.Field)
	edited, err := encounter.DefaultSites().Site(ctx, &encounter.SiteInput{Data: &data})
	require.NoError(t, err)
	world.SiteHash = edited.Site.Hash()
}

// siteField is the field world's site was compiled from, read back.
func siteField(t *testing.T, world encounter.EncounterData) encounter.FieldData {
	t.Helper()
	held, err := encounter.DefaultSites().Site(context.Background(), &encounter.SiteInput{Hash: world.SiteHash})
	require.NoError(t, err)
	return held.Site.Data().Field
}
