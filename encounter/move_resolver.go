package encounter

// Wave 2.11e — MovementResolver SDK interface.
//
// MovementResolver bridges Encounter.Move to a rulebook implementation of
// per-step movement mechanics (MovementChain execution, OA triggering).
// Optional — when not supplied, Encounter.Move uses the legacy single-jump
// behavior that mutates position to path[-1] without running any chain.
//
// Wave 2.11e ships NPC-OA-only scope per director signoff on #658 (Q1=b):
// per-step iteration + trigger buffer drain. NPC OA triggers are resolved
// inline by the resolver impl (rpg-api wraps combat.MoveEntity which calls
// triggerOpportunityAttack → ResolveAttack end-to-end). Player-pause
// branch (Sentinel-shape / spell reactions) deferred to #665.
//
// Pattern parallel: this is the second instance of the resolver-per-verb
// pattern that PhasedCombatResolver established. ADR-0027 names it as the
// canonical seam for any future SDK verbs that need rulebook-aware chain
// execution.

import (
	encountercore "github.com/KirkDiggler/rpg-toolkit/encounter/core"
)

// MovementResolver bridges the encounter SDK to a rulebook implementation
// of per-step movement mechanics. The orchestrator (rpg-api) implements
// this interface against a specific rulebook (today: dnd5e wrapping
// combat.MoveEntity); the encounter SDK never imports rulebook packages.
//
// Optional — when no resolver is wired, Encounter.Move falls back to the
// legacy single-jump path that mutates position directly without per-step
// chain execution. Non-combat encounters (free-roam, social) don't need a
// resolver and don't pay the per-step iteration cost.
type MovementResolver interface {
	// ResolveStep runs the rulebook's MovementChain for a single step
	// (FromHex → ToHex). Chain subscribers (Disengage marker, OA condition)
	// may mutate prevention sources and publish ReactionTriggerEvents on
	// the encounter bus.
	//
	// The encounter SDK installs a buffered subscriber on
	// ReactionTriggerTopic before this call, so triggers published by chain
	// subscribers are observed via the bus. Triggers flow via the buffered
	// bus subscription only — the resolver does not return them in the
	// step result. The bus path is canonical for OA/reaction handoff.
	//
	// NPC OAs are resolved inline by the resolver impl: combat.MoveEntity
	// → triggerOpportunityAttack → ResolveAttack runs end-to-end, applying
	// damage and publishing AttackResolved events on the bus before
	// ResolveStep returns. The encounter SDK does not need to act on NPC
	// triggers — they were already resolved.
	//
	// The resolver MUST NOT mutate encounter SDK state directly. The SDK
	// accumulates a traveled path across successful step calls and applies
	// the final position + publishes events once after the loop (see
	// applyAndPublishMove). The resolver only signals chain outcomes per
	// step; the SDK owns when state is committed.
	ResolveStep(input MovementStepInput) (*MovementStepResult, error)
}

// MovementStepInput is the per-step input shape for MovementResolver.
type MovementStepInput struct {
	// EntityID is the moving entity (player or monster). The encounter SDK
	// passes the player's EntityID (not PlayerID) so the resolver can look
	// up rulebook state by the same key the rest of the bus uses.
	EntityID encountercore.EntityID

	// FromHex is the entity's position before this step.
	FromHex encountercore.Hex

	// ToHex is the entity's destination after this step.
	ToHex encountercore.Hex
}

// MovementStepResult is the per-step output shape for MovementResolver.
//
// Triggers flow via the buffered bus subscription only — the SDK installs
// a buffered subscriber on ReactionTriggerTopic per step and drains it
// after ResolveStep returns. Chain subscribers (Disengage marker, OA
// condition) publish ReactionTriggerEvents on the encounter bus; the SDK
// observes them through the buffer. There is intentionally no
// resolver-returned trigger slot: a second channel would invite
// implementers to silently drop bus triggers, and the bus path is
// canonical for OA/reaction handoff (see PhasedCombatResolver for the
// same shape applied to attack reactions).
type MovementStepResult struct {
	// Prevented is true when chain subscribers (Disengage, etc.) blocked
	// the step. The encounter SDK stops the move and does NOT advance to
	// ToHex. The MoveEvent then carries the traveled segments — the ToHexes
	// of all successfully-completed steps before this one (which equals
	// the prevented step's FromHex for any non-first step). If prevention
	// fires on the very first step, no MoveEvent is published and position
	// is unchanged.
	Prevented bool

	// PreventReason is a human-readable explanation when Prevented is true.
	// Optional; the SDK does not interpret it.
	PreventReason string
}
