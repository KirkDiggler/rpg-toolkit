// Copyright (C) 2026 Kirk Diggler
// SPDX-License-Identifier: GPL-3.0-or-later

package resolution

import (
	"fmt"

	"github.com/KirkDiggler/rpg-toolkit/core"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/encounter"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/refs"
)

// nativeMaintenanceSpell reads the identity a record names for what is being
// kept: spells and authored statuses; anything else is refused. Generic root
// maintenance may name any content, but this adapter must refuse unsupported
// presentation rather than write an item or feature into a SpellIdentity field.
func nativeMaintenanceSpell(ref core.Ref, name string) (encounter.SpellIdentity, error) {
	status := ref.Module == statusSource.Module && ref.Type == statusSource.Type
	if !status && (ref.Module != refs.Module || ref.Type != refs.TypeSpells) {
		return encounter.SpellIdentity{}, fmt.Errorf("%w: unsupported non-spell native maintenance source %q", ErrBadAction, ref.String())
	}
	if err := ref.IsValid(); err != nil {
		return encounter.SpellIdentity{}, fmt.Errorf("%w: invalid maintenance source: %w", ErrBadAction, err)
	}
	if status && name == "" {
		return encounter.SpellIdentity{}, fmt.Errorf("%w: status maintenance source %q has no name", ErrBadAction, ref.String())
	}
	return encounter.SpellIdentity{Ref: ref.String(), Name: name}, nil
}

// StatusSourceRef is the ref a status an author wrote names as its source; the
// session gives every status with it.
func StatusSourceRef(id string) core.Ref {
	return core.Ref{Module: statusSource.Module, Type: statusSource.Type, ID: id}
}

// statusSource is the module and type every authored status names.
var statusSource = core.Ref{Module: "site", Type: "statuses"}
