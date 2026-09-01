// Copyright (C) 2026 Kirk Diggler
// SPDX-License-Identifier: GPL-3.0-or-later

// Package npc defines generic non-player-character content.
//
// An NPC is a reusable content record: identity, display name, broad policy,
// and opaque capability labels. It is not a placed encounter member, a shop,
// a dialogue tree, or a living-world actor by itself.
//
// Rulebooks own concrete behavior. For example, a D&D merchant can compose with
// an NPC carrying CapabilityVendor, while the D&D rulebook still owns item refs,
// stock rules, prices, quote flow, purchase flow, and inventory mutation.
//
// Runtime systems own placement and current state. Encounter/session/world
// adapters may project an NPC into their own records, but this package does not
// import those systems and does not decide teams, hostility, location knowledge,
// sight, sound, smell, or movement at runtime.
package npc
