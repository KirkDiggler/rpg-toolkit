// Copyright (C) 2026 Kirk Diggler
// SPDX-License-Identifier: GPL-3.0-or-later

// Package banditcamp is UC-1: one camp, many ways in, many endings, wired to
// real D&D 5e.
//
// It is the only package here allowed to import a rulebook, and it does so at
// exactly one seam. Read it as three files and a claim.
//
// # Three touchpoints
//
//   - Declare. [Declaration] is the camp as data — who exists, how they stand
//     to each other, which role can change hands, and the folds that turn facts
//     into the present. [Verbs] is what anyone may try. [Contract] is the job.
//     No route into the camp appears in any of them.
//
//   - Inject. [CheckResolver] is D&D 5e: a real character sheet's ability
//     modifier, proficiency bonus and expertise, run through
//     checks.MakeAbilityCheck on an injected [dice.Roller]. Give it a
//     [ScriptedRoller] and the whole camp is reproducible.
//
//   - Subscribe. quest.Instance.Observe hands back a report and its events. A
//     host decides that finishing this contract is worth three hundred
//     experience points; nothing in world learns that it did.
//
// # The claim
//
// Five ways through this camp are asserted in the tests, and no code anywhere
// distinguishes them. Kick the door in and the camp is alerted because it
// witnessed an assault. Come over the wall and it is surprised because it
// witnessed nothing. Kill the chief quietly and wear his face and the camp
// follows you, because allegiance follows whoever holds the leads slot and the
// camp never heard about the body. Talk instead and enough regard converts
// hostility to alliance. Blow the disguise in front of one lieutenant and that
// lieutenant folds a different present from the camp he stands in.
//
// Not one of those is a branch. They are compositions of nine verbs over one
// declaration, and the differences between them are audiences.
//
// # Where the seam actually is
//
// [Executor] is the piece worth arguing about. It reads no world state and
// imports no rulebook — journal and nothing else — which is why an actor's
// class, level, and proficiencies cannot gate an attempt: there is nowhere in
// it to look them up. Whether it belongs in this example or in the kernel is
// the open question the spike hands back.
package banditcamp
