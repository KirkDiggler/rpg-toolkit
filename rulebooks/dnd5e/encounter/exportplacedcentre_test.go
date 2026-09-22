// Copyright (C) 2026 Kirk Diggler
// SPDX-License-Identifier: GPL-3.0-or-later

package encounter

import "github.com/KirkDiggler/rpg-toolkit/tools/spatial"

// ExportedPlacedCentre is [placedCentre] for the external test package, which
// is where the claim about it lives: the point this module calls a
// placement's centre is inside the rectangle spatial itself traces.
//
// A TEST-ONLY DOOR, in a _test.go file, so nothing outside this package can
// reach it at build time. The alternative is either an exported API nobody
// needs or an internal test that cannot use the suite's own fixtures.
func ExportedPlacedCentre(p spatial.FootprintPlacement) spatial.Point { return placedCentre(p) }
