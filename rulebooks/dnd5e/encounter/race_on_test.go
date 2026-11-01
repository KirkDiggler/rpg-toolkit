// Copyright (C) 2026 Kirk Diggler
// SPDX-License-Identifier: GPL-3.0-or-later

//go:build race

package encounter_test

// raceDetector says this binary was built with -race, for the one test too
// slow to run under it (TestObserversOf_EqualsFullRefresh's keep twin).
const raceDetector = true
