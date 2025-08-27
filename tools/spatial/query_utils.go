package spatial

import (
	"context"
	"fmt"

	"github.com/KirkDiggler/rpg-toolkit/core"
)

// QueryUtils provides convenient methods for performing spatial queries
type QueryUtils struct {
	queryHandler *SpatialQueryHandler
}

// NewQueryUtils creates a new query utilities instance
func NewQueryUtils(queryHandler *SpatialQueryHandler) *QueryUtils {
	return &QueryUtils{
		queryHandler: queryHandler,
	}
}

// QueryPositionsInRange performs a positions-in-range query directly through the query handler
func (q *QueryUtils) QueryPositionsInRange(
	ctx context.Context, center Position, radius float64, roomID string,
) ([]Position, error) {
	data := &QueryPositionsInRangeData{
		Center: center,
		Radius: radius,
		RoomID: roomID,
	}

	result, err := q.queryHandler.handlePositionsInRange(ctx, data)
	if err != nil {
		return nil, fmt.Errorf("failed to execute positions query: %w", err)
	}

	if result.Error != nil {
		return nil, result.Error
	}

	return result.Results, nil
}

// QueryEntitiesInRange performs an entities-in-range query directly through the query handler
func (q *QueryUtils) QueryEntitiesInRange(
	ctx context.Context, center Position, radius float64, roomID string, filter EntityFilter,
) ([]core.Entity, error) {
	data := &QueryEntitiesInRangeData{
		Center: center,
		Radius: radius,
		RoomID: roomID,
		Filter: filter,
	}

	result, err := q.queryHandler.handleEntitiesInRange(ctx, data)
	if err != nil {
		return nil, fmt.Errorf("failed to execute entities query: %w", err)
	}

	if result.Error != nil {
		return nil, result.Error
	}

	return result.Results, nil
}

// QueryLineOfSight performs a line-of-sight query directly through the query handler
func (q *QueryUtils) QueryLineOfSight(ctx context.Context, from, to Position, roomID string) ([]Position, bool, error) {
	data := &QueryLineOfSightData{
		From:   from,
		To:     to,
		RoomID: roomID,
	}

	result, err := q.queryHandler.handleLineOfSight(ctx, data)
	if err != nil {
		return nil, false, fmt.Errorf("failed to execute line of sight query: %w", err)
	}

	if result.Error != nil {
		return nil, false, result.Error
	}

	return result.Results, result.Blocked, nil
}

// QueryMovement performs a movement query directly through the query handler
func (q *QueryUtils) QueryMovement(
	ctx context.Context, entity core.Entity, from, to Position, roomID string,
) (bool, []Position, float64, error) {
	data := &QueryMovementData{
		Entity: entity,
		From:   from,
		To:     to,
		RoomID: roomID,
	}

	result, err := q.queryHandler.handleMovement(ctx, data)
	if err != nil {
		return false, nil, 0, fmt.Errorf("failed to execute movement query: %w", err)
	}

	if result.Error != nil {
		return false, nil, 0, result.Error
	}

	return result.Valid, result.Path, result.Distance, nil
}

// QueryPlacement performs a placement query directly through the query handler
func (q *QueryUtils) QueryPlacement(
	ctx context.Context, entity core.Entity, position Position, roomID string,
) (bool, error) {
	data := &QueryPlacementData{
		Entity:   entity,
		Position: position,
		RoomID:   roomID,
	}

	result, err := q.queryHandler.handlePlacement(ctx, data)
	if err != nil {
		return false, fmt.Errorf("failed to execute placement query: %w", err)
	}

	if result.Error != nil {
		return false, result.Error
	}

	return result.Valid, nil
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
