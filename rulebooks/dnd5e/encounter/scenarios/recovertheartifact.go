// Copyright (C) 2026 Kirk Diggler
// SPDX-License-Identifier: GPL-3.0-or-later

package scenarios

import (
	"fmt"

	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/encounter"
)

// recovertheartifact.go is the first scenario (rpg-project#368, design §3.2
// and R6): get in, get the thing, get out.
//
// TWO BINDINGS AND ONE ENDING. The form asks which placed thing the party is
// there to recover and which exit counts as escaping with it; the run ends
// when somebody declares Exit at that exit holding that thing. Both ways to
// win — search for the door, or loot the way in off the captain — end here
// and are indistinguishable at the ending, which is correct: the journal's
// silence about a fight is the record of the skipped fight.
//
// # The refusals are the guidance
//
// Each sentence below is BOTH the form's help text and the error the author
// sees when they get that field wrong. They are one string for that reason —
// two would drift, and the one that drifts is always the one nobody reads.

// RecoverTheArtifactID is this scenario's key in a dungeon's `scenarios:`
// map, and the key of the ending it declares.
const RecoverTheArtifactID = "recover-the-artifact"

// The two field keys, exported because a host translating a descriptor to
// the wire should name them rather than spell them.
const (
	// FieldArtifact names the placement the party is there to recover.
	FieldArtifact = "artifact"

	// FieldExitKey names the exit that counts as escaping with it.
	FieldExitKey = "exit"
)

// The guidance, verbatim from design §3.2's table — and the refusal text.
const (
	artifactGuidance = "this scenario needs an artifact — which placed thing is the party here to recover"
	exitGuidance     = "this scenario needs a way out — which exit counts as escaping with the artifact"
)

// recoverTheArtifact is the scenario itself. Stateless: it holds the form
// and the rules for reading one, and nothing about any particular dungeon.
type recoverTheArtifact struct{}

func init() { register(recoverTheArtifact{}) }

// ID is this scenario's key in a dungeon's `scenarios:` map.
func (recoverTheArtifact) ID() string { return RecoverTheArtifactID }

// Name is what the builder shows in the picker.
func (recoverTheArtifact) Name() string { return "Recover the artifact" }

// Fields is the form, in the order it is rendered.
func (recoverTheArtifact) Fields() []Field {
	return []Field{
		{
			Key: FieldArtifact, Label: "Artifact",
			Type: FieldEntityRef, Kind: "prop", Guidance: artifactGuidance,
		},
		{
			Key: FieldExitKey, Label: "Way out",
			Type: FieldEntityRef, Kind: "exit", Guidance: exitGuidance,
		},
	}
}

// New reads one filled-in form against the dungeon it is bound to.
//
// Refusals, each in the words the form itself uses:
//
//   - no artifact named, or one that names nothing this dungeon places;
//   - an artifact that is not takeable — a thing nobody can pick up can
//     never be carried out, so an ending waiting for it is an ending that
//     can never fire;
//   - no exit named, or one this dungeon does not declare.
//
// NOTHING IS DEFAULTED. There is no "the first takeable prop" and no "the
// party's start" — a scenario the author did not finish filling in is a
// scenario that does not run (rpg-toolkit#1033).
func (recoverTheArtifact) New(cfg map[string]string, facts *DungeonFacts) (Declared, error) {
	if facts == nil {
		return Declared{}, fmt.Errorf("%s: no dungeon to bind to", RecoverTheArtifactID)
	}

	artifact := cfg[FieldArtifact]
	if artifact == "" {
		return Declared{}, fmt.Errorf("%s: %s", FieldArtifact, artifactGuidance)
	}
	takeable, placed := facts.Props[artifact]
	if !placed {
		return Declared{}, fmt.Errorf("%s: %q is not a thing this dungeon places — %s",
			FieldArtifact, artifact, artifactGuidance)
	}
	if !takeable {
		return Declared{}, fmt.Errorf(
			"%s: %q is scenery — nobody can pick it up, so nobody can carry it out. Mark it takeable, or %s",
			FieldArtifact, artifact, artifactGuidance)
	}

	exit := cfg[FieldExitKey]
	if exit == "" {
		return Declared{}, fmt.Errorf("%s: %s", FieldExitKey, exitGuidance)
	}
	if !facts.Exits[exit] {
		return Declared{}, fmt.Errorf("%s: %q is not a way out this dungeon declares — %s",
			FieldExitKey, exit, exitGuidance)
	}

	return Declared{
		Endings: []encounter.EndingInput{{
			Key:     RecoverTheArtifactID,
			Trigger: encounter.TriggerExitedHolding{Exit: exit, Item: artifact},
		}},
		Artifact: artifact,
		Exit:     exit,
	}, nil
}
