// Copyright (C) 2026 Kirk Diggler
// SPDX-License-Identifier: GPL-3.0-or-later

package dungeonspec

// regions_helper_test.go gives the tests one way to write a floor in Go
// (rpg-project#725): a cell set becomes one region's canonical rows. Exported
// so the external test package reaches it too; it is test code and ships in
// no build.

// OneRegion is a region holding exactly cells, lit at 1, its rows canonical
// whatever order cells come in.
func OneRegion(id, name string, cells []RoomCell) RoomRegion {
	return RoomRegion{ID: id, Name: name, Lighting: RoomLighting{Intensity: 1}, Cells: CellRows(cells)}
}

// SetFloor replaces a room's regions with ONE region, the implicit one under
// its existing id and name (the first region's name, if any), holding cells:
// the v5 spelling of what assigning the walkable list used to be.
func SetFloor(gp *RoomGameplaySource, cells []RoomCell) {
	name := ""
	if len(gp.Regions) > 0 {
		name = gp.Regions[0].Name
	}
	gp.Regions = []RoomRegion{OneRegion(gp.ImplicitRegionID, name, cells)}
}

// Floor is the union of a room's regions' cells, in authored order.
func Floor(gp *RoomGameplaySource) []RoomCell { return gp.floorCells() }
