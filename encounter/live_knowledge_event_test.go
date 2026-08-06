package encounter

import (
	"testing"

	"github.com/KirkDiggler/rpg-toolkit/encounter/core"
	"github.com/KirkDiggler/rpg-toolkit/encounter/events"
	"github.com/KirkDiggler/rpg-toolkit/encounter/perception"
	"github.com/KirkDiggler/rpg-toolkit/tools/spatial"
	"github.com/stretchr/testify/require"
)

const (
	liveKnowledgeAliceID = "alice"
	liveKnowledgeBobID   = "bob"
)

func TestHexRevealedEventSnapshotsViewerObservations(t *testing.T) {
	innerHex := core.HexFromPosition(spatial.Position{X: 0, Y: 0})
	rootHex := core.HexFromPosition(spatial.Position{X: 1, Y: 0})
	inner := ArchetypeChamber
	alice := perception.NewView(liveKnowledgeAliceID, innerHex, 0)
	alice.Observe(perception.HexObservation{
		Position: innerHex, State: perception.KnowledgeStateVisible, ZoneID: "inner",
	})
	alice.Observe(perception.HexObservation{Position: rootHex, State: perception.KnowledgeStateVisible})
	bob := perception.NewView(liveKnowledgeBobID, rootHex, 0)
	bob.Observe(perception.HexObservation{Position: rootHex, State: perception.KnowledgeStateVisible})
	e := &Encounter{data: &Data{
		Players: map[core.PlayerID]*PlayerData{
			liveKnowledgeAliceID: {ID: liveKnowledgeAliceID, View: alice},
			liveKnowledgeBobID:   {ID: liveKnowledgeBobID, View: bob},
		},
		Space: &SpaceData{SemanticRegions: []SemanticRegionData{{
			ID: "inner", Archetype: &inner, Cells: core.HexSet{innerHex: {}},
		}}},
	}}
	reveals := map[core.PlayerID]events.HexRevealedSlice{
		liveKnowledgeAliceID: {Hexes: core.NewHexSet(innerHex, rootHex)},
		liveKnowledgeBobID:   {Hexes: core.NewHexSet(rootHex)},
	}

	e.attachRevealObservations(reveals)
	event := events.NewHexRevealedEvent("enc", 1, reveals)
	aliceFacts := knownHexesByPosition(event.PerPlayer[liveKnowledgeAliceID].Observations)
	bobFacts := knownHexesByPosition(event.PerPlayer[liveKnowledgeBobID].Observations)
	require.Equal(t, "inner", aliceFacts[innerHex].ZoneID)
	require.Empty(t, aliceFacts[rootHex].ZoneID)
	require.Empty(t, bobFacts[rootHex].ZoneID)
	require.NotContains(t, bobFacts, innerHex, "each viewer receives only their own observations")

	// The event payload is detached from later memory/space changes.
	alice.Memory[innerHex] = perception.HexObservation{Position: innerHex, ZoneID: "changed"}
	e.data.Space.SemanticRegions = nil
	require.Equal(t, "inner", aliceFacts[innerHex].ZoneID)
}

func knownHexesByPosition(observations []events.KnownHex) map[core.Hex]events.KnownHex {
	out := make(map[core.Hex]events.KnownHex, len(observations))
	for _, observation := range observations {
		out[observation.Position] = observation
	}
	return out
}
