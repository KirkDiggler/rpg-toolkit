// Copyright (C) 2026 Kirk Diggler
// SPDX-License-Identifier: GPL-3.0-or-later

//go:build race

package replaytest_test

// raceDetector says this binary was built with -race, for the garrison keep
// too slow to replay twice under it (the encounter package's own pattern).
const raceDetector = true
