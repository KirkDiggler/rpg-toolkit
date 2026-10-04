// Copyright (C) 2026 Kirk Diggler
// SPDX-License-Identifier: GPL-3.0-or-later

// Package assessment supplies detached operation contracts and composition tools
// for rule-owned assessments. It knows no concrete effect identities. Geometry
// and observations arrive as facts; this package neither queries a world nor
// loads a target sheet to fill unknown facts. Rules retain eligibility and
// explanation ownership, and execution retains rolling and effect lifecycle.
//
// A read's incomplete observed universe cannot establish that no qualifying
// participant exists. An execution caller may supply authoritative facts under
// the same contracts without turning an earlier informational answer into
// permission or a frozen offer.
package assessment
