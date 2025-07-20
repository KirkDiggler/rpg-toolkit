// Package behavior provides infrastructure for entity decision-making and
// action execution without implementing any specific behaviors.
//
// Purpose:
// This package establishes contracts for AI behavior systems, supporting
// multiple paradigms (state machines, behavior trees, utility AI) while
// remaining agnostic to specific game rules or creature behaviors.
//
// Scope:
//   - Behavior interfaces and context
//   - State machine infrastructure
//   - Behavior tree node types
//   - Perception system interfaces
//   - Action types and constants
//   - Memory management for behaviors
//   - Decision event publishing
//
// Non-Goals:
//   - Specific creature behaviors: Implement in game rulebooks
//   - Combat tactics: Game-specific AI belongs in games
//   - Pathfinding algorithms: May add A* infrastructure later
//   - Behavior implementations: Only infrastructure here
//   - Game state access: Behaviors receive filtered context
//
// Integration:
// This package integrates with:
//   - spatial: For position queries and movement validation
//   - events: For publishing decision events
//   - selectables: For weighted action selection
//
// The behavior package is designed to be extended by game implementations
// that provide concrete behaviors for their specific creatures and NPCs.
//
// Example:
//
//	// Game implements concrete behavior
//	type AggressiveBehavior struct {
//	    targetPriority []behavior.TargetPriority
//	}
//
//	func (b *AggressiveBehavior) Execute(ctx BehaviorContext) (Action, error) {
//	    perception := ctx.GetPerception()
//	    nearest := findNearest(perception.VisibleEntities)
//
//	    if distanceTo(nearest) <= meleeRange {
//	        return Action{
//	            Type: ActionTypeAttack,
//	            Target: nearest.ID(),
//	        }, nil
//	    }
//
//	    return Action{
//	        Type: ActionTypeMove,
//	        Target: nearest.Position(),
//	    }, nil
//	}
//
//	func (b *AggressiveBehavior) Priority() BehaviorPriority {
//	    return BehaviorPriorityCombat
//	}
package behavior
