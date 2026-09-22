package encounter

import (
	"github.com/KirkDiggler/rpg-toolkit/tools/spatial"
	"testing"
)

func TestSightAreaSegmentCrossing(t *testing.T) {
	grid := spatial.NewHexGrid(spatial.HexGridConfig{Width: 100, Height: 100})
	a := SightArea{Center: spatial.Position{X: 0, Y: 0}, RadiusFeet: 20}
	if !areaCrosses(a, spatial.Position{X: -30, Y: 0}, spatial.Position{X: 30, Y: 0}, grid) {
		t.Fatal("line through area should be blocked")
	}
	if areaCrosses(a, spatial.Position{X: -30, Y: 30}, spatial.Position{X: 30, Y: 30}, grid) {
		t.Fatal("same-side line should remain clear")
	}
}

func TestSightAreaSourceRemovalAndLoadValidation(t *testing.T) {
	areas := map[string]SightArea{"a": {ID: "a", SourceID: "spell", Center: spatial.Position{}, RadiusFeet: 20}, "b": {ID: "b", SourceID: "spell", Center: spatial.Position{}, RadiusFeet: 20}}
	e := &Encounter{sightAreas: areas}
	if !e.RemoveSightArea("spell") || len(e.sightAreas) != 0 {
		t.Fatal("removal should clear all source areas")
	}
	if err := validateSightAreasData([]SightAreaData{{ID: "a", SourceID: "x", RadiusFeet: 20}, {ID: "a", SourceID: "y", RadiusFeet: 20}}); err == nil {
		t.Fatal("duplicate persisted IDs must be rejected")
	}
}
