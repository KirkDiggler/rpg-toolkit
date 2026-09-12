// Copyright (C) 2026 Kirk Diggler
// SPDX-License-Identifier: GPL-3.0-or-later

// Package stage is the world wiring: the parts that know what a verb and a
// journal are, so nothing beneath them has to.
//
// It holds the two things world asks for and perception can answer — a
// [Watcher] that says who witnessed an act, and [Aim], which resolves what
// somebody MEANT to do against what is really there.
//
// The split from package act is deliberate. A decider says what an actor wants
// in the vocabulary of its own beliefs and imports no world at all; this package
// turns that into something the world can record. A behaviour author writes the
// first kind and never has to learn the second.
package stage

import (
	"context"
	"errors"
	"fmt"
	"slices"

	"github.com/KirkDiggler/rpg-toolkit/examples/perception/act"
	"github.com/KirkDiggler/rpg-toolkit/examples/perception/projection"
	"github.com/KirkDiggler/rpg-toolkit/examples/perception/testimony"
	"github.com/KirkDiggler/rpg-toolkit/world"
	"github.com/KirkDiggler/rpg-toolkit/world/journal"
)

// ErrActorNotPresent reports an act by somebody the truth surface does not have
// anywhere. It is a wiring fault and it fails loudly: guessing an empty audience
// would silently make every act unwitnessed, which is indistinguishable from a
// perfect sneak.
var ErrActorNotPresent = errors.New("stage: the actor is not on the truth surface")

// Watcher answers world's witnessing question from the same senses the
// perception pass runs on.
//
// That sameness is the point. Who heard about an act and who could perceive the
// place it happened are one decision, so they are made once — a Watcher and a
// projection pass built from the same [projection.Input] cannot disagree about
// who was there.
//
// It reads the truth surface, never anybody's beliefs: who saw you do something
// does not depend on what you thought you were doing.
type Watcher struct {
	In projection.Input
}

// Bystanders answers who, besides the actor and target, perceived this act.
func (w Watcher) Bystanders(
	_ context.Context, actor, _ journal.EntityID, _ world.Witnessing,
) ([]journal.EntityID, error) {
	where, found := w.whereIs(string(actor))
	if !found {
		return nil, fmt.Errorf("%w: %s", ErrActorNotPresent, actor)
	}

	var out []journal.EntityID

	for _, sense := range w.In.Senses {
		if testimony.Observer(actor) == sense.Observer {
			continue
		}

		if !slices.Contains(sense.Reach, where) {
			continue
		}

		id := journal.EntityID(sense.Observer)
		if !slices.Contains(out, id) {
			out = append(out, id)
		}
	}

	return out, nil
}

func (w Watcher) whereIs(source string) (string, bool) {
	for _, p := range w.In.Presences {
		if p.Source == source {
			return p.Where, true
		}
	}

	return "", false
}

// Aim resolves an intent against what is really there.
//
// The actor named something they believe in. This finds the tracks they hold
// under that name and asks the truth surface what, if anything, is behind them.
//
// An empty answer is not an error. It is what acting on an illusion looks like:
// the belief was real, the name was real, the swing was real, and there was
// nothing there. Resolution happens against the world, which is why the world is
// the only thing consulted here.
func Aim(in projection.Input, s act.Situation, intent act.Intent) journal.EntityID {
	bound := projection.Bind(in)

	for _, held := range s.Holds {
		if !held.Named || held.Name != intent.Target {
			continue
		}

		if source, behindIt := bound[held.ID]; behindIt {
			return journal.EntityID(source)
		}
	}

	return ""
}

// Scripted is a resolver with no dice in it, for a spike that must contain no
// randomness.
type Scripted struct {
	Succeeded bool
	Margin    int
}

// Resolve returns the scripted outcome for any attempt.
func (s Scripted) Resolve(_ context.Context, a world.Attempt) (journal.Outcome, error) {
	return journal.Outcome{
		Contested: true,
		Succeeded: s.Succeeded,
		Margin:    s.Margin,
		Detail:    string(a.Approach) + ": scripted",
	}, nil
}
