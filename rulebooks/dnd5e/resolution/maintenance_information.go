// Copyright (C) 2026 Kirk Diggler
// SPDX-License-Identifier: GPL-3.0-or-later

package resolution

import (
	"fmt"

	"github.com/KirkDiggler/rpg-toolkit/core"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/encounter"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/refs"
)

// The current native record contract names spells. Generic root maintenance may
// name any content, but this adapter must refuse unsupported presentation rather
// than write an item or feature into a SpellIdentity field.
func nativeMaintenanceSpell(ref core.Ref, name string) (encounter.SpellIdentity, error) {
	if ref.Module != refs.Module || ref.Type != refs.TypeSpells {
		return encounter.SpellIdentity{}, fmt.Errorf("%w: unsupported non-spell native maintenance source %q", ErrBadAction, ref.String())
	}
	if err := ref.IsValid(); err != nil {
		return encounter.SpellIdentity{}, fmt.Errorf("%w: invalid maintenance source: %w", ErrBadAction, err)
	}
	return encounter.SpellIdentity{Ref: ref.String(), Name: name}, nil
}
