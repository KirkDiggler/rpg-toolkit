// Copyright (C) 2026 Kirk Diggler
// SPDX-License-Identifier: GPL-3.0-or-later

package conditions

import (
	"github.com/KirkDiggler/rpg-toolkit/dice"
)

// RollerBinder is the optional contract a persisted condition implements to
// receive the dice roller its rules roll with, at attach time.
//
// It exists because a condition's roller has two lives. A condition built in
// process is constructed with one; a condition restored by [LoadJSON] is not —
// its persisted bytes carry no roller, and the loader has none to give. Before
// this contract the loaded condition silently fell back to a process-global
// default at roll time, so a session that owns an interaction-scoped roller
// — one whose every face is meant to be recorded in the roll trace — had no
// way to hand it to a persisted sheet.
//
// It is an optional interface rather than a loader parameter on purpose, the
// same way [dnd5eEvents.EffectScoper] is: putting a roller on every load path
// would fork each one into roller and roller-less variants for a concern only
// an attach site has. Attach is the moment a condition goes on a bus — the
// same moment its subscriptions go live — so it is also the moment the runtime
// roller it will roll with can be handed over. A condition that does not
// implement it is untouched, and a host that supplies no roller changes
// nothing.
type RollerBinder interface {
	// BindRoller binds the roller this condition's rules roll with.
	//
	// A nil roller must not clear a roller the condition already holds: nil
	// means "leave the current roller as it is", never "erase".
	BindRoller(roller dice.Roller)
}
