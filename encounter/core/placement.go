// Copyright (C) 2024 Kirk Diggler
// SPDX-License-Identifier: GPL-3.0-or-later

package core

// PlacementOffset is an optional authored [x,y,z] translation in canonical
// game-world axes relative to an entity placement's canonical origin. It is
// presentation transport only; encounter mechanics never interpret it.
type PlacementOffset [3]float64
