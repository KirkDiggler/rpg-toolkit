// Copyright (C) 2026 Kirk Diggler
// SPDX-License-Identifier: GPL-3.0-or-later

package dungeonspec

import (
	"fmt"

	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/encounter"
	"github.com/KirkDiggler/rpg-toolkit/tools/spatial"
)

// singleRoomRegionLayout is construction output, not another knowledge store.
// The authored regions enter the existing FieldInput/save format with the
// wall footing taken out of them; the partitions are read here for the seats
// and then left behind — the encounter derives its own at construction and
// at load (rpg-project#725, R8).
type singleRoomRegionLayout struct {
	regions []encounter.RegionInput
	footing []spatial.Position
	// owner is each floor cell's AUTHORED region: what a monster's and a
	// seat's region is reported as.
	owner map[spatial.Position]string
	// partition is each floor cell's wall partition, by index: what the
	// party's seats stay inside.
	partition map[spatial.Position]int
}

// authoredRegion is one region of the source with the index it was authored
// at, so a refusal can name it.
type authoredRegion struct {
	index int
	input encounter.RegionInput
}

// singleRoomRegions divides the authored floor by its walls (rpg-project#725).
// The editing document's regions say how the floor is NAMED and LIT; whether
// two cells are in the same room — for hearing, for the party's seats, for an
// arrival — is the walls' answer, over the whole floor and across every
// region boundary, and the encounter owns that partitioning through its
// canonical canvas ([encounter.PartitionFloor]).
//
// Permanent opaque footing no single partition can claim is taken out of the
// region that painted it and carried as scenery, as before. A region whose
// every painted cell is footing is refused at its cells: paint the author
// must see is not floor, so a place made of nothing but wall is a mistake to
// be shown rather than a region that silently compiles to nothing. (A region
// listed with NO cells is different — declared and not yet painted — and is
// never handed here.)
func singleRoomRegions(field encounter.FieldInput, authored []authoredRegion) (singleRoomRegionLayout, error) {
	partition, err := encounter.PartitionFloor(encounter.PartitionFloorInput{Field: field})
	if err != nil {
		return singleRoomRegionLayout{}, singleRoomCompileError("room.room", err.Error())
	}
	footing := make(map[spatial.Position]bool, len(partition.Footing))
	for _, cell := range partition.Footing {
		footing[cell] = true
	}
	out := singleRoomRegionLayout{
		footing:   partition.Footing,
		owner:     map[spatial.Position]string{},
		partition: map[spatial.Position]int{},
	}
	for i, cells := range partition.Components {
		for _, cell := range cells {
			out.partition[cell] = i
		}
	}
	for _, a := range authored {
		region := a.input
		kept := make([]spatial.Position, 0, len(region.Cells))
		for _, cell := range region.Cells {
			if !footing[cell] {
				kept = append(kept, cell)
				out.owner[cell] = region.ID
			}
		}
		if len(kept) == 0 {
			return singleRoomRegionLayout{}, singleRoomCompileError(
				fmt.Sprintf("room.room.regions[%d].cells", a.index), "every cell is wall footing")
		}
		region.Cells = kept
		out.regions = append(out.regions, region)
	}
	return out, nil
}

// authoredRegionInputs lowers every region that owns a cell to the
// encounter's region input, in authored order: its id and name, its cells
// through the pointy-top offset, the one archetype this dialect has, and its
// own lighting. A region listed with no cells contributes nothing — it is a
// place declared and not yet painted.
func authoredRegionInputs(gp *RoomGameplaySource, o spatial.HexOrientation) []authoredRegion {
	expanded := regionCells(gp.Regions, func(string, string) {})
	out := make([]authoredRegion, 0, len(gp.Regions))
	for i, region := range gp.Regions {
		if len(expanded[i]) == 0 {
			continue
		}
		cells := make([]spatial.Position, 0, len(expanded[i]))
		for _, c := range expanded[i] {
			cells = append(cells, axialOffset(c, o))
		}
		out = append(out, authoredRegion{index: i, input: encounter.RegionInput{
			ID: region.ID, Name: region.Name, Cells: cells, Archetype: "crypt",
			Lighting: &encounter.Lighting{Intensity: region.Lighting.Intensity},
		}})
	}
	return out
}
