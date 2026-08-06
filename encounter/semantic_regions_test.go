package encounter

import (
	"testing"

	"github.com/KirkDiggler/rpg-toolkit/encounter/core"
	"github.com/KirkDiggler/rpg-toolkit/encounter/perception"
	"github.com/KirkDiggler/rpg-toolkit/tools/spatial"
	"github.com/stretchr/testify/require"
)

const (
	semanticViewerID = "viewer"
	semanticInnerID  = "inner"
)

func TestAuthorizedZonesDisclosesOnlyKnownScopeAncestors(t *testing.T) {
	a := core.HexFromPosition(spatial.Position{X: 0, Y: 0})
	b := core.HexFromPosition(spatial.Position{X: 1, Y: 0})
	hidden := core.HexFromPosition(spatial.Position{X: 2, Y: 0})
	chamber := ArchetypeChamber
	view := perception.NewView(semanticViewerID, a, 0)
	view.Observe(perception.HexObservation{
		Position: a, State: perception.KnowledgeStateVisible, ZoneID: semanticInnerID,
	})
	view.Observe(perception.HexObservation{
		Position: b, State: perception.KnowledgeStateRemembered,
	})
	enc := &Encounter{data: &Data{
		Players: map[core.PlayerID]*PlayerData{semanticViewerID: {ID: semanticViewerID, View: view}},
		Space: &SpaceData{SemanticRegions: []SemanticRegionData{
			{ID: "outer", Archetype: &chamber, Cells: core.HexSet{a: {}, b: {}}},
			{ID: semanticInnerID, Cells: core.HexSet{a: {}}},
			{ID: "hidden", Cells: core.HexSet{hidden: {}}},
		}},
	}}

	zones := enc.AuthorizedZones(semanticViewerID)
	require.Equal(t, []string{semanticInnerID, "outer"}, []string{zones[0].ID, zones[1].ID})
	require.Equal(t, "outer", *zones[0].ParentID)
	require.Equal(t, chamber, *zones[0].Archetype, "inner inherits its property")
	require.Nil(t, zones[1].ParentID)
	for _, zone := range zones {
		require.NotEqual(t, "hidden", zone.ID)
	}
}

func TestAuthorizedZonesRemembersRootWithoutDisclosingLaterPaint(t *testing.T) {
	painted := core.HexFromPosition(spatial.Position{X: 0, Y: 0})
	view := perception.NewView(semanticViewerID, painted, 0)
	view.Observe(perception.HexObservation{
		Position: painted, State: perception.KnowledgeStateRemembered,
		// Empty ZoneID is the persisted observation that this was root.
	})
	enc := &Encounter{data: &Data{
		Players: map[core.PlayerID]*PlayerData{semanticViewerID: {ID: semanticViewerID, View: view}},
		Space: &SpaceData{SemanticRegions: []SemanticRegionData{{
			ID: "painted-later", Cells: core.HexSet{painted: {}},
		}}},
	}}

	require.Empty(t, enc.AuthorizedZones(semanticViewerID))
}

func TestLoadFromDataRejectsDanglingObservedZoneID(t *testing.T) {
	known := core.HexFromPosition(spatial.Position{X: 0, Y: 0})
	view := perception.NewView(semanticViewerID, known, 0)
	view.Observe(perception.HexObservation{
		Position: known, State: perception.KnowledgeStateRemembered, ZoneID: "missing-zone",
	})
	data := &Data{
		Players: map[core.PlayerID]*PlayerData{semanticViewerID: {ID: semanticViewerID, View: view}},
		Space: &SpaceData{
			FloorSource: FloorSourceCanvas,
			Width:       1,
			Height:      1,
		},
	}
	broker := NewBroker(NewInMemoryTransport())

	_, err := LoadFromData(t.Context(), data, broker)
	require.ErrorContains(t, err, "references missing semantic zone \"missing-zone\"")
}
