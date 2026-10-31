// Copyright (C) 2026 Kirk Diggler
// SPDX-License-Identifier: GPL-3.0-or-later

package keepgen

import (
	"fmt"

	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/encounter"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/encounter/dungeonspec"
	"github.com/KirkDiggler/rpg-toolkit/tools/spatial"
)

// minSeats is the party the start must seat (design Proof before write).
const minSeats = 4

// Prove is the refusal (design Proof before write; README: keepgen.Prove):
// it loads a site through the dialect's own loader and judges what the
// loader compiled, never what the generator meant. It refuses, naming the
// first failing rule:
//
//   - a site [dungeonspec.Load] refuses;
//   - a party start that seats fewer than four: len(compiled.PartyStart),
//     the loader's own seats, with no second derivation (plan G4);
//   - a region with floor that cannot be reached from the party start with
//     every door open (plan G5). The flood is the engine's own partition of
//     the compiled floor ([encounter.PartitionFloor]) with the doors lifted
//     out of the field, so an opening is open and every wall, wall span and
//     permanent sight blocker stands exactly as play compiles it. Opaque
//     footing — floor no partition owns — is scenery, not a room to reach.
//
// It returns the compiled site so a caller need not load it twice.
func Prove(doc []byte) (dungeonspec.Compiled, error) {
	compiled, err := dungeonspec.Load(doc)
	if err != nil {
		return dungeonspec.Compiled{}, fmt.Errorf("keepgen: refused: the site does not load: %w", err)
	}
	if n := len(compiled.PartyStart); n < minSeats {
		return dungeonspec.Compiled{}, fmt.Errorf("keepgen: refused: the party start seats %d, fewer than %d", n, minSeats)
	}
	open := withDoorsLifted(compiled.Field)
	parts, err := encounter.PartitionFloor(encounter.PartitionFloorInput{Field: open})
	if err != nil {
		return dungeonspec.Compiled{}, fmt.Errorf("keepgen: refused: the floor does not partition with the doors open: %w", err)
	}
	start := compiled.PartyStart[0].At
	reached := map[spatial.Position]bool{}
	for _, component := range parts.Components {
		holds := false
		for _, c := range component {
			if c == start {
				holds = true
				break
			}
		}
		if holds {
			for _, c := range component {
				reached[c] = true
			}
			break
		}
	}
	footing := map[spatial.Position]bool{}
	for _, c := range parts.Footing {
		footing[c] = true
	}
	for _, r := range compiled.Field.Regions {
		for _, c := range r.Cells {
			if !reached[c] && !footing[c] {
				return dungeonspec.Compiled{}, fmt.Errorf("keepgen: refused: region %q is unreachable from the party start through the doors", r.ID)
			}
		}
	}
	return compiled, nil
}

// withDoorsLifted is the field with every door taken out of its openings:
// the doors gone and each structural opening bound to no door, so an
// opening is a gap. The walls, their spans and every prop are the field's
// own. The input is not modified.
func withDoorsLifted(f encounter.FieldInput) encounter.FieldInput {
	f.Doors = nil
	walls := make([]encounter.StructuralWallInput, len(f.StructuralWalls))
	for i, w := range f.StructuralWalls {
		openings := make([]encounter.StructuralOpeningInput, len(w.Openings))
		for j, op := range w.Openings {
			op.Door = nil
			openings[j] = op
		}
		w.Openings = openings
		walls[i] = w
	}
	f.StructuralWalls = walls
	return f
}
