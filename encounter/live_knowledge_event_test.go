package encounter

import (
	"context"
	"testing"
	"time"

	"github.com/KirkDiggler/rpg-toolkit/encounter/core"
	"github.com/KirkDiggler/rpg-toolkit/encounter/events"
	"github.com/KirkDiggler/rpg-toolkit/encounter/perception"
	"github.com/KirkDiggler/rpg-toolkit/tools/spatial"
	"github.com/stretchr/testify/require"
)

const (
	liveKnowledgeAliceID       = "alice"
	liveKnowledgeBobID         = "bob"
	liveKnowledgeAliceEntityID = "alice-entity"
)

func TestHexRevealedEventSnapshotsViewerObservations(t *testing.T) {
	innerHex := core.HexFromPosition(spatial.Position{X: 0, Y: 0})
	rootHex := core.HexFromPosition(spatial.Position{X: 1, Y: 0})
	inner := ArchetypeChamber
	facing := uint32(1)
	alice := perception.NewView(liveKnowledgeAliceID, innerHex, 0)
	alice.Observe(perception.HexObservation{
		Position: innerHex, State: perception.KnowledgeStateVisible, ZoneID: "inner",
		Edges:    []perception.Edge{{From: innerHex, To: rootHex, BlocksMovement: true}},
		Contents: []perception.Placement{{EntityID: "prop", Facing: &facing}},
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
	audience := map[core.PlayerID]struct{}{liveKnowledgeAliceID: {}}
	appeared := events.NewEntityAppearedEvent(
		"enc", 2, "prop", innerHex, audience, e.eventObservationsForAudience(innerHex, audience),
	)
	disappeared := events.NewEntityDisappearedEvent(
		"enc", 3, "prop", map[core.PlayerID]core.Hex{liveKnowledgeAliceID: innerHex},
		e.eventObservationsForPositions(map[core.PlayerID]core.Hex{liveKnowledgeAliceID: innerHex}),
	)
	aliceFacts := knownHexesByPosition(event.PerPlayer[liveKnowledgeAliceID].Observations)
	bobFacts := knownHexesByPosition(event.PerPlayer[liveKnowledgeBobID].Observations)
	require.Equal(t, "inner", aliceFacts[innerHex].ZoneID)
	require.Len(t, aliceFacts[innerHex].Edges, 1)
	require.True(t, aliceFacts[innerHex].Edges[0].BlocksMovement)
	require.Empty(t, aliceFacts[rootHex].ZoneID)
	require.Empty(t, bobFacts[rootHex].ZoneID)
	require.NotContains(t, bobFacts, innerHex, "each viewer receives only their own observations")

	// The event payload is detached from later memory/space changes.
	alice.Memory[innerHex] = perception.HexObservation{Position: innerHex, ZoneID: "changed"}
	e.data.Space.SemanticRegions = nil
	facing = 5
	require.Equal(t, "inner", aliceFacts[innerHex].ZoneID)
	require.Equal(t, uint32(1), *aliceFacts[innerHex].Contents[0].Facing)
	require.Equal(t, uint32(1), *appeared.Observations[liveKnowledgeAliceID].Contents[0].Facing)
	require.Equal(t, uint32(1), *disappeared.Observations[liveKnowledgeAliceID].Contents[0].Facing)
}

func TestMovePassThroughAppearanceSnapshotsTransitionPlacement(t *testing.T) {
	transport := NewInMemoryTransport()
	broker := NewBroker(transport)
	defer func() { _ = broker.Close(); _ = transport.Close() }()
	e := New(context.Background(), "pass-through", broker)
	start := core.Hex{Q: 0, R: 0, S: 0}
	appearedAt := core.Hex{Q: 2, R: 0, S: -2}
	end := core.Hex{Q: 4, R: 0, S: -4}
	require.NoError(t, e.AddPlayer(PlayerInput{
		PlayerID: liveKnowledgeAliceID, EntityID: liveKnowledgeAliceEntityID, Position: start, SightRange: 0,
	}))
	require.NoError(t, e.AddPlayer(PlayerInput{
		PlayerID: liveKnowledgeBobID, EntityID: "bob-entity", Position: appearedAt, SightRange: 0,
	}))
	subscription, err := broker.Subscribe("pass-through", liveKnowledgeBobID)
	require.NoError(t, err)
	defer func() { _ = subscription.Close() }()

	require.NoError(t, e.Move(liveKnowledgeAliceID, []core.Hex{
		{Q: 1, R: 0, S: -1}, appearedAt, {Q: 3, R: 0, S: -3}, end,
	}))
	var appeared *events.EntityAppearedEvent
	var disappeared *events.EntityDisappearedEvent
	deadline := time.After(time.Second)
	for appeared == nil || disappeared == nil {
		select {
		case event := <-subscription.Events():
			switch typed := event.(type) {
			case *events.EntityAppearedEvent:
				if typed.Entity == liveKnowledgeAliceEntityID {
					appeared = typed
				}
			case *events.EntityDisappearedEvent:
				if typed.Entity == liveKnowledgeAliceEntityID {
					disappeared = typed
				}
			}
		case <-deadline:
			t.Fatal("timed out waiting for pass-through transition events")
		}
	}
	appearance := appeared.Observations[liveKnowledgeBobID]
	require.Equal(t, appearedAt, appearance.Position)
	require.Empty(t, appearance.ZoneID)
	require.Contains(t, knownHexEntityIDs(appearance), core.EntityID(liveKnowledgeAliceEntityID))
	disappearance := disappeared.Observations[liveKnowledgeBobID]
	require.Equal(t, appearedAt, disappearance.Position)
	require.NotContains(t, knownHexEntityIDs(disappearance), core.EntityID(liveKnowledgeAliceEntityID),
		"disappearance observations represent post-disappearance memory")
}

func knownHexEntityIDs(observation events.KnownHex) []core.EntityID {
	ids := make([]core.EntityID, 0, len(observation.Contents))
	for _, content := range observation.Contents {
		ids = append(ids, content.EntityID)
	}
	return ids
}

func knownHexesByPosition(observations []events.KnownHex) map[core.Hex]events.KnownHex {
	out := make(map[core.Hex]events.KnownHex, len(observations))
	for _, observation := range observations {
		out[observation.Position] = observation
	}
	return out
}
