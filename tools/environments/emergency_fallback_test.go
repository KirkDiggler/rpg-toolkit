package environments

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/KirkDiggler/rpg-toolkit/events"
	"github.com/KirkDiggler/rpg-toolkit/tools/spatial"
)

func TestEmergencyFallbackEventPublishing(t *testing.T) {
	// Create a simple event bus to capture events
	eventBus := events.NewBus()

	// Set up event capture
	var capturedEvents []events.Event
	eventBus.SubscribeFunc(EventEmergencyFallbackTriggered, 0, events.HandlerFunc(func(_ context.Context, event events.Event) error {
		capturedEvents = append(capturedEvents, event)
		return nil
	}))

	// Create test shape and size
	testShape := &RoomShape{
		Name: "test_rectangle",
		Connections: []ConnectionPoint{
			{Name: "entrance", Position: spatial.Position{X: 0, Y: 5}},
			{Name: "exit", Position: spatial.Position{X: 10, Y: 5}},
		},
	}
	testSize := spatial.Dimensions{Width: 10, Height: 10}

	// Create parameters that will trigger emergency fallback
	params := PatternParams{
		Density:           0.8, // High density
		DestructibleRatio: 0.5,
		RandomSeed:        42,
		Safety: PathSafetyParams{
			MinPathWidth:      2.0,
			MinOpenSpace:      0.99, // Nearly impossible to satisfy
			EntitySize:        1.0,
			EmergencyFallback: true,
		},
		Material:   "stone",
		WallHeight: 3.0,
		EventBus:   eventBus, // Pass the event bus
	}

	// Call RandomPattern - this should trigger the emergency fallback
	walls, err := RandomPattern(context.Background(), testShape, testSize, params)
	require.NoError(t, err)

	// Verify emergency fallback was triggered
	assert.Empty(t, walls, "Emergency fallback should return empty room")
	assert.Len(t, capturedEvents, 1, "Should have captured exactly one emergency fallback event")

	// Verify event details
	if len(capturedEvents) == 1 {
		event := capturedEvents[0]
		assert.Equal(t, EventEmergencyFallbackTriggered, event.Type())

		// Check event context data
		ctx := event.Context()

		patternType, ok := ctx.Get("pattern_type")
		assert.True(t, ok)
		assert.Equal(t, "random", patternType)

		reason, ok := ctx.Get("reason")
		assert.True(t, ok)
		assert.Equal(t, "path_safety_validation_failed", reason)

		density, ok := ctx.Get("original_density")
		assert.True(t, ok)
		assert.Equal(t, 0.8, density)

		fallbackType, ok := ctx.Get("fallback_type")
		assert.True(t, ok)
		assert.Equal(t, "empty_room", fallbackType)

		roomSize, ok := ctx.Get("room_size")
		assert.True(t, ok)
		assert.Equal(t, "10x10", roomSize)

		_, ok = ctx.Get("validation_error")
		assert.True(t, ok)

		_, ok = ctx.Get("original_wall_count")
		assert.True(t, ok)
	}
}

func TestEmergencyFallbackNoEventBus(t *testing.T) {
	// Test that emergency fallback works even without event bus
	testShape := &RoomShape{
		Name: "test_rectangle",
		Connections: []ConnectionPoint{
			{Name: "entrance", Position: spatial.Position{X: 0, Y: 5}},
			{Name: "exit", Position: spatial.Position{X: 10, Y: 5}},
		},
	}
	testSize := spatial.Dimensions{Width: 10, Height: 10}

	// Create parameters that will trigger emergency fallback (no event bus)
	params := PatternParams{
		Density:           0.8,
		DestructibleRatio: 0.5,
		RandomSeed:        42,
		Safety: PathSafetyParams{
			MinPathWidth:      2.0,
			MinOpenSpace:      0.99, // Nearly impossible to satisfy
			EntitySize:        1.0,
			EmergencyFallback: true,
		},
		Material:   "stone",
		WallHeight: 3.0,
		EventBus:   nil, // No event bus
	}

	// Call RandomPattern - this should still work, just without event
	walls, err := RandomPattern(context.Background(), testShape, testSize, params)
	require.NoError(t, err)

	// Verify emergency fallback was triggered
	assert.Empty(t, walls, "Emergency fallback should return empty room even without event bus")
}
