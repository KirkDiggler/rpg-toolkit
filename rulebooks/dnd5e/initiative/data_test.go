package initiative_test

import (
	"encoding/json"
	"testing"

	"github.com/KirkDiggler/rpg-toolkit/core"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/initiative"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestTrackerDataPersistence(t *testing.T) {
	// Create a tracker
	order := []core.Entity{
		initiative.NewParticipant("ranger-123", "character"),
		initiative.NewParticipant("goblin-001", "monster"),
		initiative.NewParticipant("wizard-456", "character"),
	}
	tracker := initiative.New(order)

	// Advance a couple turns
	tracker.Next() // Now goblin's turn
	tracker.Next() // Now wizard's turn

	// Convert to data
	data := tracker.ToData()

	// Verify data
	assert.Equal(t, 3, len(data.Order))
	assert.Equal(t, 2, data.Current) // wizard is at index 2
	assert.Equal(t, 1, data.Round)

	// Marshal to JSON (simulating save to database)
	jsonData, err := json.Marshal(data)
	require.NoError(t, err)

	// Unmarshal from JSON (simulating load from database)
	var loadedData initiative.TrackerData
	err = json.Unmarshal(jsonData, &loadedData)
	require.NoError(t, err)

	// Recreate tracker from data
	newTracker := initiative.LoadFromData(loadedData)

	// Verify state was preserved
	current := newTracker.Current()
	assert.Equal(t, "wizard-456", current.GetID())
	assert.Equal(t, "character", current.GetType())
	assert.Equal(t, 1, newTracker.Round())

	// Next turn should wrap to round 2
	next := newTracker.Next()
	assert.Equal(t, "ranger-123", next.GetID())
	assert.Equal(t, 2, newTracker.Round())
}

func TestTrackerDataWithRemovals(t *testing.T) {
	// Create tracker
	order := []core.Entity{
		initiative.NewParticipant("fighter", "character"),
		initiative.NewParticipant("orc", "monster"),
		initiative.NewParticipant("cleric", "character"),
	}
	tracker := initiative.New(order)

	// Fighter's turn, then advance
	tracker.Next() // Orc's turn

	// Remove the orc
	err := tracker.Remove("orc")
	require.NoError(t, err)

	// Save state
	data := tracker.ToData()

	// Only 2 entities should remain
	assert.Equal(t, 2, len(data.Order))
	assert.Equal(t, "fighter", data.Order[0].ID)
	assert.Equal(t, "cleric", data.Order[1].ID)

	// Load from data
	newTracker := initiative.LoadFromData(data)

	// Should still be cleric's turn (orc was removed)
	current := newTracker.Current()
	assert.Equal(t, "cleric", current.GetID())
}
