package encounter

import (
	"testing"

	"github.com/KirkDiggler/rpg-toolkit/encounter/core"
	"github.com/KirkDiggler/rpg-toolkit/encounter/perception"
	"github.com/KirkDiggler/rpg-toolkit/tools/spatial"
	"github.com/stretchr/testify/require"
)

func TestAuthorizedZonesDisclosesOnlyKnownScopeAncestors(t *testing.T) {
	a := core.HexFromPosition(spatial.Position{X: 0, Y: 0})
	b := core.HexFromPosition(spatial.Position{X: 1, Y: 0})
	hidden := core.HexFromPosition(spatial.Position{X: 2, Y: 0})
	chamber := ArchetypeChamber
	view := perception.NewView("viewer", a, 0)
	view.Observe(perception.HexObservation{Position: a, State: perception.KnowledgeStateVisible})
	view.Observe(perception.HexObservation{Position: b, State: perception.KnowledgeStateRemembered})
	enc := &Encounter{data: &Data{
		Players: map[core.PlayerID]*PlayerData{"viewer": {ID: "viewer", View: view}},
		Space: &SpaceData{SemanticRegions: []SemanticRegionData{
			{ID: "outer", Archetype: &chamber, Cells: core.HexSet{a: {}, b: {}}},
			{ID: "inner", Cells: core.HexSet{a: {}}},
			{ID: "hidden", Cells: core.HexSet{hidden: {}}},
		}},
	}}

	zones := enc.AuthorizedZones("viewer")
	require.Equal(t, []string{"inner", "outer"}, []string{zones[0].ID, zones[1].ID})
	require.Equal(t, "outer", *zones[0].ParentID)
	require.Equal(t, chamber, *zones[0].Archetype, "inner inherits its property")
	require.Nil(t, zones[1].ParentID)
	for _, zone := range zones {
		require.NotEqual(t, "hidden", zone.ID)
	}
}
