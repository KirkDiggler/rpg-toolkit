// Copyright (C) 2026 Kirk Diggler
// SPDX-License-Identifier: GPL-3.0-or-later

// Package monster provides monster/enemy entity types for D&D 5e combat.
//
// # Templates: authored, derived once
//
// A [Template] is the authored half of a stat block — scores, hit dice,
// armour, proficiency, trained skills, weapons and experience — over a named
// rulebook base. [FromTemplate] is the one function that turns it into a
// [Monster] (rpg-project#555 R6), and it derives every total: hit points from
// the hit dice average and CON, armour class through armor.ArmorClass,
// attacks through the same weapon assembly a hand-built constructor uses, and
// passive Perception from WIS and training. Nothing derived is ever stored on
// the template (R3), because a stored total silently stops following the
// score it came from. A rulebook base such as the human block is itself a
// template, assembled by the same function.
//
// The derived sheet carries the TEMPLATE's ref (dnd5e:monsters:guard), passed
// to [FromTemplate] explicitly and never inherited from the base (R2), so
// every authored creature keeps its own id. What it does inherit from the base
// is its creature type.
package monster
