// Copyright (C) 2026 Kirk Diggler
// SPDX-License-Identifier: GPL-3.0-or-later

// Package scenarios is WHAT A DUNGEON IS FOR (rpg-project#368, design §3.2
// and §6; the form ruled 2026-09-01).
//
// A dungeon is geometry: rooms, walls, doors, things standing on the floor,
// and ways out. It says nothing about why a party is in it. A SCENARIO is
// that why, and it is exactly three things:
//
//   - a DESCRIPTOR — the form the builder renders: field key, label, type,
//     and the guidance, which is the constructor's own refusal text;
//   - a CONFIG — what the author filled in, ids naming things in the file;
//   - a DECLARATION — the endings the encounter runs, and the bound ids the
//     host needs, produced by New(cfg, compiled) or refused in words the
//     form-filler can act on.
//
// # Why this is a package per scenario and not a table
//
// A scenario's refusals are its design. "This scenario needs an artifact —
// which placed thing is the party here to recover" is a sentence somebody
// wrote for a person filling in a form, and it is the same sentence the
// builder shows as guidance and the API returns when the form is wrong. A
// table of field names could not carry it, and a validator generated from
// types could not either. So each scenario is a small package that owns its
// words.
//
// # What is deliberately not here
//
// NO CONTENT RESOLUTION. This package never learns what
// "dnd5e:props:reliquary" is; it works in placement ids and asks
// [dungeonspec.Compiled] whether the thing that id names is holdable. Design
// law C1, one layer out.
//
// NO ENDING MECHANISM. The encounter owns "the run is over" and always has.
// A scenario DECLARES [encounter.EndingInput]s and the composition runs
// them; there is no goal engine and this package holds no state.
//
// # The registry
//
// [All] is what an authoring service serves as ListScenarios. It exists so
// the pinning test can run over every scenario rather than over the one
// somebody remembered — the second scenario cannot skip it.
package scenarios
