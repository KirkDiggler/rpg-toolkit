// Copyright (C) 2026 Kirk Diggler
// SPDX-License-Identifier: GPL-3.0-or-later

package spells

import (
	"encoding/json"
	generic "github.com/KirkDiggler/rpg-toolkit/mechanics/effects"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/effects"
	"strconv"
)

type spellMaintenanceInput struct {
	turnEnds  int
	skipFirst bool
}

// spellMaintenance authors the existing shared spell policies as ordinary parts.
// This is content, not a lifecycle implementation or a named-spell dispatch.
func spellMaintenance(in spellMaintenanceInput) []effects.PartDeclaration {
	return []effects.PartDeclaration{
		{Key: "concentration", Kind: effects.KindConcentration, OnTerminal: generic.EndApplication},
		{Key: "duration", Kind: effects.KindDuration, Params: json.RawMessage(`{"turn_ends":` + strconv.Itoa(in.turnEnds) + `,"skip_first":` + strconv.FormatBool(in.skipFirst) + `}`), OnTerminal: generic.EndApplication},
		{Key: "empty", Kind: effects.KindUntilEmpty, OnTerminal: generic.EndApplication},
		{Key: "rest", Kind: effects.KindEndOn, Params: json.RawMessage(`{"beat":"rest","reason":"rest"}`), OnTerminal: generic.EndApplication},
		{Key: "combat", Kind: effects.KindEndOn, Params: json.RawMessage(`{"beat":"combat_end","reason":"combat_end"}`), OnTerminal: generic.EndApplication},
	}
}
