// Copyright (C) 2026 Kirk Diggler
// SPDX-License-Identifier: GPL-3.0-or-later

// Package stage is where an intent meets the world.
//
// A mind speaks in names. The stage is the only code that holds both a
// situation and the truth surface, so it is the only place a name can be
// resolved against what is really there — and it is deliberately a separate
// package, so nothing that decides can import it.
//
// There will be two resolutions here, and they read different things. Aim
// binds a name to the truth surface, for a swing: an illusion binds to nothing.
// Recall will bind a name to the actor's own remembered locus, for a walk: a
// walk toward a ghost never consults truth. Recall arrives with the fixture
// that pays for it.
package stage

import (
	"github.com/KirkDiggler/rpg-toolkit/examples/behavior"
	"github.com/KirkDiggler/rpg-toolkit/examples/perception/projection"
)

// Aim resolves what an actor meant to hit against what is really there.
//
// It finds the contact the actor holds under the intent's name and asks the
// truth surface what, if anything, is behind the track that BEARS the name.
// Only that track: a contact is the actor's claim that several tracks are one
// thing, and the claim can be wrong. The swing goes at what was named. The
// first fixture found this the hard way — resolving through any track in the
// bundle sent a swing at a chant, and the chant was coming from someone else.
//
// The answer is the ledger's own handle for the thing, or "" — and "" is not
// an error. It is what acting on an illusion looks like: the belief was real,
// the swing was real, and there was nothing there.
func Aim(in projection.Input, s behavior.Situation, intent behavior.Intent) string {
	bound := projection.Bind(in)

	for _, c := range s.Contacts {
		if !c.Named || c.Name != intent.Target {
			continue
		}

		return bound[c.Bearer]
	}

	return ""
}
