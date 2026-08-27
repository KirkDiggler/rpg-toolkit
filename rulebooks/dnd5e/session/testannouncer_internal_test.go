// Copyright (C) 2026 Kirk Diggler
// SPDX-License-Identifier: GPL-3.0-or-later

package session

import (
	"context"

	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/encounter"
)

// encQuietAnnouncer is the internal-package twin of session_test's own — see
// its doc. A separate type because the internal test package cannot import the
// external one.
type encQuietAnnouncer struct{}

func (encQuietAnnouncer) Announce(context.Context, *encounter.Encounter, []encounter.Boundary) error {
	return nil
}
