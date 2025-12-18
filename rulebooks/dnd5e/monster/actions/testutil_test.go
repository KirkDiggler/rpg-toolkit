// Copyright (C) 2024 Kirk Diggler
// SPDX-License-Identifier: GPL-3.0-or-later

package actions

import "github.com/KirkDiggler/rpg-toolkit/tools/spatial"

// hexAt creates a CubeCoordinate from X (z defaults to 0), deriving Y = -X
func hexAt(x int) spatial.CubeCoordinate {
	return spatial.CubeCoordinate{X: x, Y: -x, Z: 0}
}
