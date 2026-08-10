// Copyright (C) 2026 Kirk Diggler
// SPDX-License-Identifier: GPL-3.0-or-later

// Package encounter implements the free-roam encounter composition.
//
// An encounter is a composition with an outcome (Setup → play → Outcome).
// This module is the courier between play/clock, play/intel, play/record,
// and tools/spatial: it surveils percepts into intel, lets deciders act on
// their own intel, and appends the story to record.
// Members exit, encounters close; player activity pumps the clock, the world
// thinks on the tick.
//
// Design contract: docs/ideas/encounter/design.md (composition laws C1–C8).
// This is not a play/ leaf: the module composes published pieces and exposes
// one aggregate persistence pair at the host seam.
package encounter
