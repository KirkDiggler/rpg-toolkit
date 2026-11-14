// Copyright (C) 2026 Kirk Diggler
// SPDX-License-Identifier: GPL-3.0-or-later

package refs

import "github.com/KirkDiggler/rpg-toolkit/core"

// TypeDefinitions identifies reusable authored definitions, not running conditions.
const TypeDefinitions = "definitions"

// Definitions exposes discoverable identities for built-in authored content.
var Definitions = definitionsNS{}

type definitionsNS struct{}

// Raging returns the complete authored Rage definition identity as an owned ref.
// Borrowing its parts does not grant the active Raging condition identity.
func (definitionsNS) Raging() *core.Ref {
	return &core.Ref{Module: Module, Type: TypeDefinitions, ID: "raging"}
}
