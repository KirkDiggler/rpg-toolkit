// Copyright (C) 2026 Kirk Diggler
// SPDX-License-Identifier: GPL-3.0-or-later

package dungeonspec

import (
	"fmt"

	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/encounter"
	"github.com/KirkDiggler/rpg-toolkit/tools/spatial"
)

// singleRoomRegionLayout is construction output, not another knowledge store.
// Region identities and cells enter the existing FieldInput/save format; only
// their derivation belongs to the builder dialect.
type singleRoomRegionLayout struct {
	regions []encounter.RegionInput
	owner   map[spatial.Position]string
}

// singleRoomRegions gives geometry-separated floor to ordinary room discovery.
// The editing document is not a declaration that all of its floor is one room.
// Encounter owns partitioning through its canonical canvas; this adapter names
// the results and preserves the source region's fixed metadata.
func singleRoomRegions(field encounter.FieldInput) (singleRoomRegionLayout, error) {
	base := field.Regions[0]
	partition, err := encounter.PartitionRegion(encounter.PartitionRegionInput{Field: field, Region: base.ID})
	if err != nil {
		return singleRoomRegionLayout{}, err
	}
	out := singleRoomRegionLayout{owner: make(map[spatial.Position]string, len(base.Cells))}
	for i, cells := range partition.Components {
		region := base
		lighting := *base.Lighting
		region.Lighting = &lighting
		if i > 0 {
			region.ID = fmt.Sprintf("%s/space/%.0f,%.0f", base.ID, cells[0].X, cells[0].Y)
		}
		region.Cells = cells
		for _, cell := range cells {
			out.owner[cell] = region.ID
		}
		out.regions = append(out.regions, region)
	}
	return out, nil
}
