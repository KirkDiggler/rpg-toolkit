// Copyright (C) 2026 Kirk Diggler
// SPDX-License-Identifier: GPL-3.0-or-later

package session

// drains_internal_test.go pins #2220's session sentinels that a bare call can
// reach: the news these seams carry never crosses the API, so an external
// test cannot see it. errNoTurn (equipTurn) and errNoWindow (openWindow)
// reach their raise sites only through a live write scope, so their
// consumption is pinned where it lives — by the world-clock equip tests and
// the cast resume's posed path.

import (
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/KirkDiggler/rpg-toolkit/core"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/resolution"
)

func TestNoPreviewAnswersErrNoPreview(t *testing.T) {
	preview, err := previewCastAim(nil, "", nil, nil)
	require.Nil(t, preview)
	require.ErrorIs(t, err, errNoPreview)
}

func TestNoSaveAnswersErrNoSave(t *testing.T) {
	save, err := castSave(resolution.CastTargetOutcome{}, core.Ref{})
	require.Nil(t, save)
	require.ErrorIs(t, err, errNoSave)
}

func TestNoRetaliationAnswersErrNoRetaliation(t *testing.T) {
	unit, err := retaliationUnit(nil, concentration{})
	require.Nil(t, unit)
	require.ErrorIs(t, err, errNoRetaliation)
}
