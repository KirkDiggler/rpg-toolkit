// Copyright (C) 2026 Kirk Diggler
// SPDX-License-Identifier: GPL-3.0-or-later

package resolution

import "github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/encounter"

// Actors are the actors every world a resolution loads carries. Resolve calls
// no encounter verb, so none is ever asked; each refuses by name if one is.
//
// This package exports the value and never installs it: a host puts it in the
// [Input]'s Capabilities.
var Actors = encounter.RefusingActors()
