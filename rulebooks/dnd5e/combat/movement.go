// Copyright (C) 2024 Kirk Diggler
// SPDX-License-Identifier: GPL-3.0-or-later

package combat

import (
	"context"

	"github.com/KirkDiggler/rpg-toolkit/dice"
	"github.com/KirkDiggler/rpg-toolkit/events"
	"github.com/KirkDiggler/rpg-toolkit/rpgerr"
	dnd5eEvents "github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/events"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/weapons"
	"github.com/KirkDiggler/rpg-toolkit/tools/spatial"
)

// roomContextKey is the key type for storing spatial.Room in context.
type roomContextKey struct{}

// WithRoom wraps a context.Context with the provided spatial.Room.
// This enables MoveEntity and related functions to access spatial data
// for threat detection and opportunity attack processing.
func WithRoom(ctx context.Context, room spatial.Room) context.Context {
	return context.WithValue(ctx, roomContextKey{}, room)
}

// getRoomFromContext retrieves the spatial.Room from context.
// Returns nil and an error if no Room is present.
func getRoomFromContext(ctx context.Context) (spatial.Room, error) {
	room, ok := ctx.Value(roomContextKey{}).(spatial.Room)
	if !ok || room == nil {
		return nil, rpgerr.New(rpgerr.CodeNotFound, "no Room found in context")
	}
	return room, nil
}

// DefaultMeleeReach is the default melee reach for most combatants in grid units.
// In D&D 5e with 5ft squares, this is 1 unit (5 feet).
// Reach weapons extend this to 2 units (10 feet).
const DefaultMeleeReach = 1.0

// FeetPerGridUnit is the conversion factor between feet and grid units.
// In D&D 5e, each grid square is 5 feet.
const FeetPerGridUnit = 5.0

// MoveEntityInput contains parameters for moving an entity through the combat space.
// Movement is processed step by step, checking for opportunity attacks at each step.
type MoveEntityInput struct {
	// EntityID is the ID of the entity to move.
	EntityID string

	// EntityType indicates the type of moving entity ("character" or "monster").
	EntityType string

	// Path is the sequence of positions the entity will move through.
	// Each position represents a single step (typically 5 feet).
	Path []spatial.Position

	// EventBus is required for publishing movement chain events.
	EventBus events.EventBus

	// Roller is the dice roller for opportunity attack rolls.
	// If nil, a default roller is used.
	Roller dice.Roller
}

// Validate validates the input fields.
func (i *MoveEntityInput) Validate() error {
	if i == nil {
		return rpgerr.New(rpgerr.CodeInvalidArgument, "MoveEntityInput is nil")
	}
	if i.EntityID == "" {
		return rpgerr.New(rpgerr.CodeInvalidArgument, "EntityID is required")
	}
	if i.EntityType == "" {
		return rpgerr.New(rpgerr.CodeInvalidArgument, "EntityType is required")
	}
	if len(i.Path) == 0 {
		return rpgerr.New(rpgerr.CodeInvalidArgument, "Path is required")
	}
	if i.EventBus == nil {
		return rpgerr.New(rpgerr.CodeInvalidArgument, "EventBus is required")
	}
	return nil
}

// OpportunityAttackResult tracks the result of an opportunity attack that was triggered.
type OpportunityAttackResult struct {
	// AttackerID is the ID of the entity that made the opportunity attack.
	AttackerID string

	// Hit indicates whether the opportunity attack hit.
	Hit bool

	// Damage is the total damage dealt by the opportunity attack.
	Damage int

	// Critical indicates whether the attack was a critical hit.
	Critical bool
}

// MoveEntityResult contains the result of a movement operation.
type MoveEntityResult struct {
	// FinalPosition is where the entity ended up after movement.
	FinalPosition spatial.Position

	// StepsCompleted is the number of steps successfully completed.
	StepsCompleted int

	// OAsTriggered contains all opportunity attacks that were triggered during movement.
	OAsTriggered []OpportunityAttackResult

	// OAErrors contains any errors that occurred while processing opportunity attacks.
	// These are non-fatal errors that didn't stop movement but should be logged for debugging.
	OAErrors []string

	// MovementStopped indicates whether movement was stopped before reaching the destination.
	MovementStopped bool

	// StopReason explains why movement was stopped, if applicable.
	StopReason string
}

// MoveEntity executes movement step by step, checking for opportunity attacks at each step.
// The function fires a MovementChain event before each step to allow conditions like
// Disengaging to prevent opportunity attacks, or features like Sentinel to stop movement.
//
// For each step in the path:
//  1. Determine which entities threaten the current position
//  2. Fire MovementChain event to collect modifiers
//  3. If movement is not prevented:
//     a. For each threatening entity that the mover is LEAVING threat range of:
//     - Trigger opportunity attack (unless OA is prevented)
//     b. Move to next position
//  4. If movement is blocked, stop and return current state
//
//nolint:gocyclo // Movement resolution requires coordinating multiple game systems
func MoveEntity(ctx context.Context, input *MoveEntityInput) (*MoveEntityResult, error) {
	if err := input.Validate(); err != nil {
		return nil, err
	}

	// Get the room from context for spatial queries
	room, err := getRoomFromContext(ctx)
	if err != nil {
		return nil, rpgerr.Wrap(err, "room is required for movement")
	}

	// Get current position of the moving entity
	currentPos, found := room.GetEntityPosition(input.EntityID)
	if !found {
		return nil, rpgerr.Newf(rpgerr.CodeNotFound, "entity %s not found in room", input.EntityID)
	}

	// Use provided roller or default
	roller := input.Roller
	if roller == nil {
		roller = dice.NewRoller()
	}

	result := &MoveEntityResult{
		FinalPosition:  currentPos,
		StepsCompleted: 0,
		OAsTriggered:   make([]OpportunityAttackResult, 0),
		OAErrors:       make([]string, 0),
	}

	// Track actual steps taken (separate from loop index to handle skipped positions)
	actualSteps := 0

	// Process each step in the path
	for _, nextPos := range input.Path {
		// Skip if this is the current position (first position in path might be starting point)
		if currentPos.Equals(nextPos) {
			continue
		}

		// Find entities that threaten the current position
		threateningEntities := findThreateningEntities(ctx, room, input.EntityID, currentPos)

		// Create movement chain event
		movementEvent := &dnd5eEvents.MovementChainEvent{
			EntityID:            input.EntityID,
			EntityType:          input.EntityType,
			FromPosition:        toEventPosition(currentPos),
			ToPosition:          toEventPosition(nextPos),
			ThreateningEntities: threateningEntities,
			OAPreventionSources: make([]dnd5eEvents.MovementModifierSource, 0),
			MovementPrevented:   false,
			PreventionReason:    "",
		}

		// Fire movement chain to collect modifiers
		movementChain := events.NewStagedChain[*dnd5eEvents.MovementChainEvent](ModifierStages)
		movements := dnd5eEvents.MovementChain.On(input.EventBus)

		modifiedChain, err := movements.PublishWithChain(ctx, movementEvent, movementChain)
		if err != nil {
			return nil, rpgerr.Wrap(err, "failed to publish movement chain")
		}

		finalEvent, err := modifiedChain.Execute(ctx, movementEvent)
		if err != nil {
			return nil, rpgerr.Wrap(err, "failed to execute movement chain")
		}

		// Check if movement was prevented by a modifier
		if finalEvent.MovementPrevented {
			result.MovementStopped = true
			result.StopReason = finalEvent.PreventionReason
			return result, nil
		}

		// Process opportunity attacks if not prevented
		if !finalEvent.IsOAPrevented() {
			for _, threatenerID := range threateningEntities {
				// Check if mover is leaving this threatener's threat range
				if isLeavingThreatRange(ctx, room, input.EntityID, threatenerID, currentPos, nextPos) {
					// Trigger opportunity attack
					oaResult, err := triggerOpportunityAttack(ctx, threatenerID, input.EntityID, input.EventBus, roller)
					if err != nil {
						// Record error for debugging but continue - OA failure shouldn't stop movement
						result.OAErrors = append(result.OAErrors, err.Error())
						continue
					}

					if oaResult != nil {
						result.OAsTriggered = append(result.OAsTriggered, *oaResult)

						// Future: Check for Sentinel-style effects that stop movement on hit
						// For now, OAs don't stop movement unless the target is incapacitated
					}
				}
			}
		}

		// Actually move the entity in the spatial room
		if err := room.MoveEntity(input.EntityID, nextPos); err != nil {
			return nil, rpgerr.Wrapf(err, "failed to move entity to position (%v, %v)", nextPos.X, nextPos.Y)
		}

		// Update tracking
		currentPos = nextPos
		actualSteps++
		result.FinalPosition = currentPos
		result.StepsCompleted = actualSteps
	}

	return result, nil
}

// findThreateningEntities returns the IDs of all entities that threaten the given position.
// An entity threatens a position if:
// - It is within melee reach of the position
// - It is not the moving entity itself
// - It can make opportunity attacks (not incapacitated)
//
// NOTE: This function currently does not check hostility. In D&D 5e, only hostile
// creatures can make opportunity attacks. Future implementation should add a hostility
// check once faction/allegiance tracking is available in gamectx.
func findThreateningEntities(
	ctx context.Context,
	room spatial.Room,
	movingEntityID string,
	position spatial.Position,
) []string {
	// Get all entities within melee reach of this position (in grid units)
	entitiesInRange := room.GetEntitiesInRange(position, DefaultMeleeReach)

	threatening := make([]string, 0, len(entitiesInRange))
	for _, entity := range entitiesInRange {
		// Skip the moving entity itself
		if entity.GetID() == movingEntityID {
			continue
		}

		// Check if this entity can make opportunity attacks
		// For now, assume all entities in range can threaten (future: check for incapacitated, etc.)
		if canMakeOpportunityAttack(ctx, entity.GetID()) {
			threatening = append(threatening, entity.GetID())
		}
	}

	return threatening
}

// isLeavingThreatRange checks if moving from fromPos to toPos leaves the threatener's threat range.
// An entity leaves threat range when:
// - They were within threat range at fromPos
// - They will be outside threat range at toPos
func isLeavingThreatRange(
	ctx context.Context,
	room spatial.Room,
	_ string,
	threatenerID string,
	fromPos, toPos spatial.Position,
) bool {
	threatenerPos, found := room.GetEntityPosition(threatenerID)
	if !found {
		return false
	}

	grid := room.GetGrid()
	distanceFrom := grid.Distance(threatenerPos, fromPos)
	distanceTo := grid.Distance(threatenerPos, toPos)

	// Get the threatener's reach (default 5ft for now)
	reach := getEntityReach(ctx, threatenerID)

	// Leaving threat range means: was in range, will be out of range
	return distanceFrom <= reach && distanceTo > reach
}

// getEntityReach returns the melee threat reach for an entity in grid units.
// Most entities have 1 unit reach (5ft), but reach weapons extend this to 2 units (10ft).
// Future: Check equipped weapons for reach property.
func getEntityReach(_ context.Context, _ string) float64 {
	// For now, assume all entities have standard 1 unit (5ft) reach
	// Future: Look up equipped weapon and check for reach property
	return DefaultMeleeReach
}

// canMakeOpportunityAttack checks if an entity is capable of making opportunity attacks.
// An entity cannot make OAs if they are incapacitated, have no reaction available, etc.
// Future: Check conditions and reaction availability.
func canMakeOpportunityAttack(_ context.Context, _ string) bool {
	// For now, assume all entities can make opportunity attacks
	// Future: Check for incapacitated condition, reaction availability, etc.
	return true
}

// triggerOpportunityAttack resolves an opportunity attack from attacker against target.
// Returns nil if the attacker cannot make an attack (no weapon, etc.).
func triggerOpportunityAttack(
	ctx context.Context,
	attackerID, targetID string,
	bus events.EventBus,
	roller dice.Roller,
) (*OpportunityAttackResult, error) {
	// Get the attacker from context
	attacker, err := GetCombatantFromContext(ctx, attackerID)
	if err != nil {
		return nil, rpgerr.Wrapf(err, "failed to get attacker %s for opportunity attack", attackerID)
	}

	// Get the attacker's melee weapon
	// For now, use unarmed strike as fallback
	weapon := getAttackerMeleeWeapon(ctx, attackerID)
	if weapon == nil {
		// No melee weapon available - cannot make opportunity attack
		return nil, nil
	}

	// TODO: Use attacker to check and consume reaction availability via ActionEconomy.
	// Opportunity attacks require a reaction, which should be checked and consumed here.
	// Until reaction checks are implemented, attacker is retrieved but not otherwise used.
	_ = attacker

	// Resolve the opportunity attack
	attackResult, err := ResolveAttack(ctx, &AttackInput{
		AttackerID: attackerID,
		TargetID:   targetID,
		Weapon:     weapon,
		EventBus:   bus,
		Roller:     roller,
		AttackType: dnd5eEvents.AttackTypeOpportunity,
	})
	if err != nil {
		return nil, rpgerr.Wrap(err, "failed to resolve opportunity attack")
	}

	// Future: Consume the attacker's reaction through ActionEconomy
	// This would happen regardless of hit/miss since the reaction is spent on the attempt

	return &OpportunityAttackResult{
		AttackerID: attackerID,
		Hit:        attackResult.Hit,
		Damage:     attackResult.TotalDamage,
		Critical:   attackResult.Critical,
	}, nil
}

// getAttackerMeleeWeapon returns the melee weapon the attacker would use for an opportunity attack.
// Returns nil if the attacker has no melee weapon available.
func getAttackerMeleeWeapon(_ context.Context, _ string) *weapons.Weapon {
	// For now, return a basic unarmed strike
	// Future: Look up equipped weapon from character/monster state
	return defaultUnarmedStrike()
}

// defaultUnarmedStrike returns a basic unarmed strike weapon for opportunity attacks.
// In D&D 5e, unarmed strikes deal 1 + STR modifier bludgeoning damage.
// Note: We use "1d1" instead of "1" because the dice parser requires dice notation.
func defaultUnarmedStrike() *weapons.Weapon {
	return &weapons.Weapon{
		ID:         "unarmed-strike",
		Name:       "Unarmed Strike",
		Category:   weapons.CategorySimpleMelee,
		Damage:     "1d1", // Always rolls 1, plus STR modifier
		DamageType: "bludgeoning",
		Weight:     0,
		Properties: nil,
	}
}

// toEventPosition converts a spatial.Position to an events.Position.
func toEventPosition(pos spatial.Position) dnd5eEvents.Position {
	return dnd5eEvents.Position{
		X: pos.X,
		Y: pos.Y,
	}
}
