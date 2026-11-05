// Copyright (C) 2026 Kirk Diggler
// SPDX-License-Identifier: GPL-3.0-or-later

// Package items hosts a test-only consumer probe for item composition.
//
// The tests consume the real effects composer, resource tracker and rulebook
// healing operation. Consumer-owned output kinds connect a grant and recovery
// policy to an item-instance pool. All candidate wiring stays in test files:
// this package deliberately exposes no public item API. See README.md for the
// fixture assumptions and unproven production seams.
package items
