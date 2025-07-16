package spatial

import (
	"context"
	"fmt"

	"github.com/KirkDiggler/rpg-toolkit/core"
	"github.com/KirkDiggler/rpg-toolkit/events"
)

// QueryUtils provides convenient methods for performing spatial queries
type QueryUtils struct {
	eventBus events.EventBus
}

// NewQueryUtils creates a new query utilities instance
func NewQueryUtils(eventBus events.EventBus) *QueryUtils {
	return &QueryUtils{
		eventBus: eventBus,
	}
}

// QueryPositionsInRange performs a positions-in-range query through the event system
func (q *QueryUtils) QueryPositionsInRange(
	ctx context.Context, center Position, radius float64, roomID string,
) ([]Position, error) {
	event := events.NewGameEvent(EventQueryPositionsInRange, nil, nil)
	event.Context().Set("center", center)
	event.Context().Set("radius", radius)
	event.Context().Set("room_id", roomID)

	err := q.eventBus.Publish(ctx, event)
	if err != nil {
		return nil, fmt.Errorf("failed to publish positions query: %w", err)
	}

	// Check for errors in the event context
	if eventErr, exists := event.Context().Get("error"); exists {
		return nil, eventErr.(error)
	}

	// Get results from event context
	results, exists := event.Context().Get("results")
	if !exists {
		return nil, fmt.Errorf("no results found in event context")
	}

	positions, ok := results.([]Position)
	if !ok {
		return nil, fmt.Errorf("invalid result type: expected []Position, got %T", results)
	}

	return positions, nil
}

// QueryEntitiesInRange performs an entities-in-range query through the event system
func (q *QueryUtils) QueryEntitiesInRange(
	ctx context.Context, center Position, radius float64, roomID string, filter EntityFilter,
) ([]core.Entity, error) {
	event := events.NewGameEvent(EventQueryEntitiesInRange, nil, nil)
	event.Context().Set("center", center)
	event.Context().Set("radius", radius)
	event.Context().Set("room_id", roomID)

	if filter != nil {
		event.Context().Set("filter", filter)
	}

	err := q.eventBus.Publish(ctx, event)
	if err != nil {
		return nil, fmt.Errorf("failed to publish entities query: %w", err)
	}

	// Check for errors in the event context
	if eventErr, exists := event.Context().Get("error"); exists {
		return nil, eventErr.(error)
	}

	// Get results from event context
	results, exists := event.Context().Get("results")
	if !exists {
		return nil, fmt.Errorf("no results found in event context")
	}

	entities, ok := results.([]core.Entity)
	if !ok {
		return nil, fmt.Errorf("invalid result type: expected []core.Entity, got %T", results)
	}

	return entities, nil
}

// QueryLineOfSight performs a line-of-sight query through the event system
func (q *QueryUtils) QueryLineOfSight(ctx context.Context, from, to Position, roomID string) ([]Position, bool, error) {
	event := events.NewGameEvent(EventQueryLineOfSight, nil, nil)
	event.Context().Set("from", from)
	event.Context().Set("to", to)
	event.Context().Set("room_id", roomID)

	err := q.eventBus.Publish(ctx, event)
	if err != nil {
		return nil, false, fmt.Errorf("failed to publish line of sight query: %w", err)
	}

	// Check for errors in the event context
	if eventErr, exists := event.Context().Get("error"); exists {
		return nil, false, eventErr.(error)
	}

	// Get results from event context
	results, exists := event.Context().Get("results")
	if !exists {
		return nil, false, fmt.Errorf("no results found in event context")
	}

	positions, ok := results.([]Position)
	if !ok {
		return nil, false, fmt.Errorf("invalid result type: expected []Position, got %T", results)
	}

	blocked, exists := event.Context().Get("blocked")
	if !exists {
		return nil, false, fmt.Errorf("no blocked status found in event context")
	}

	blockedBool, ok := blocked.(bool)
	if !ok {
		return nil, false, fmt.Errorf("invalid blocked type: expected bool, got %T", blocked)
	}

	return positions, blockedBool, nil
}

// QueryMovement performs a movement query through the event system
func (q *QueryUtils) QueryMovement(
	ctx context.Context, entity core.Entity, from, to Position, roomID string,
) (bool, []Position, float64, error) {
	event := events.NewGameEvent(EventQueryMovement, nil, nil)
	event.Context().Set("entity", entity)
	event.Context().Set("from", from)
	event.Context().Set("to", to)
	event.Context().Set("room_id", roomID)

	err := q.eventBus.Publish(ctx, event)
	if err != nil {
		return false, nil, 0, fmt.Errorf("failed to publish movement query: %w", err)
	}

	// Check for errors in the event context
	if eventErr, exists := event.Context().Get("error"); exists {
		return false, nil, 0, eventErr.(error)
	}

	// Get valid status
	valid, exists := event.Context().Get("valid")
	if !exists {
		return false, nil, 0, fmt.Errorf("no valid status found in event context")
	}

	validBool, ok := valid.(bool)
	if !ok {
		return false, nil, 0, fmt.Errorf("invalid valid type: expected bool, got %T", valid)
	}

	// Get path
	path, exists := event.Context().Get("path")
	if !exists {
		return false, nil, 0, fmt.Errorf("no path found in event context")
	}

	pathPositions, ok := path.([]Position)
	if !ok {
		return false, nil, 0, fmt.Errorf("invalid path type: expected []Position, got %T", path)
	}

	// Get distance
	distance, exists := event.Context().Get("distance")
	if !exists {
		return false, nil, 0, fmt.Errorf("no distance found in event context")
	}

	distanceFloat, ok := distance.(float64)
	if !ok {
		return false, nil, 0, fmt.Errorf("invalid distance type: expected float64, got %T", distance)
	}

	return validBool, pathPositions, distanceFloat, nil
}

// QueryPlacement performs a placement query through the event system
func (q *QueryUtils) QueryPlacement(
	ctx context.Context, entity core.Entity, position Position, roomID string,
) (bool, error) {
	event := events.NewGameEvent(EventQueryPlacement, nil, nil)
	event.Context().Set("entity", entity)
	event.Context().Set("position", position)
	event.Context().Set("room_id", roomID)

	err := q.eventBus.Publish(ctx, event)
	if err != nil {
		return false, fmt.Errorf("failed to publish placement query: %w", err)
	}

	// Check for errors in the event context
	if eventErr, exists := event.Context().Get("error"); exists {
		return false, eventErr.(error)
	}

	// Get valid status
	valid, exists := event.Context().Get("valid")
	if !exists {
		return false, fmt.Errorf("no valid status found in event context")
	}

	validBool, ok := valid.(bool)
	if !ok {
		return false, fmt.Errorf("invalid valid type: expected bool, got %T", valid)
	}

	return validBool, nil
}

// Convenience methods for common entity filters

// CreateCharacterFilter creates a filter for character entities
func CreateCharacterFilter() EntityFilter {
	return NewSimpleEntityFilter().WithEntityTypes("character")
}

// CreateMonsterFilter creates a filter for monster entities
func CreateMonsterFilter() EntityFilter {
	return NewSimpleEntityFilter().WithEntityTypes("monster")
}

// CreateItemFilter creates a filter for item entities
func CreateItemFilter() EntityFilter {
	return NewSimpleEntityFilter().WithEntityTypes("item")
}

// CreateCombatantFilter creates a filter for combatant entities (characters + monsters)
func CreateCombatantFilter() EntityFilter {
	return NewSimpleEntityFilter().WithEntityTypes("character", "monster")
}

// CreateExcludeFilter creates a filter that excludes specific entity IDs
func CreateExcludeFilter(excludeIDs ...string) EntityFilter {
	return NewSimpleEntityFilter().WithExcludeIDs(excludeIDs...)
}

// CreateIncludeFilter creates a filter that includes only specific entity IDs
func CreateIncludeFilter(includeIDs ...string) EntityFilter {
	return NewSimpleEntityFilter().WithEntityIDs(includeIDs...)
}
