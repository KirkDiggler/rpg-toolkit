package encounter

// Beat-1 of the TakeAction wave (rpg-project #54 / #697 chunk 2).
//
// Turn-start economy seeding. The encounter is the turn-boundary authority, so
// it is the encounter — not the host (rpg-api) — that seeds each player's
// action economy when their turn begins. Before this, the encounter emitted
// TurnStartedEvent but never seeded the held character's economy, which forced
// rpg-api to inject ActionEconomyData{1,1,1,Movement:30} itself (a North-Star
// Invariant 2 violation: the host authoring rules state). This moves the
// seeding into the engine, where StartTurn already owns the rule.
//
// The seeding runs on the held *character.Character — the same instance the
// LoadFromData cascade hydrated onto e.bus (#689) — never a re-load. Seats with
// no hydrated character (a flat stat-snapshot seat, or an NPC) are skipped:
// their turn structure is driven by the snapshot path, not the character
// economy.

import (
	"context"
	"fmt"

	"github.com/KirkDiggler/rpg-toolkit/encounter/core"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/character"
)

// seedActorTurn initializes the action economy for the actor whose turn is
// beginning, when that actor is a player with a hydrated character. It calls
// character.StartTurn on the held instance, which sets one action, one bonus
// action, one reaction, and movement from the character's speed, and clears
// granted capacity from the prior turn.
//
// NPCs and stat-snapshot seats (no hydrated character) are skipped — they carry
// no character economy to seed. A nil or non-player actor id is a no-op.
//
// Called from the two turn-start sites: SetMode's flip to ModeTurnBased (first
// actor) and EndTurn (the next actor). Errors from StartTurn are returned so the
// caller fails the turn-boundary rather than advancing with an unseeded economy.
func (e *Encounter) seedActorTurn(ctx context.Context, actorID core.EntityID) error {
	char := e.heldCharacter(actorID)
	if char == nil {
		return nil // NPC or stat-snapshot seat — no character economy to seed.
	}

	if _, err := char.StartTurn(ctx, &character.StartTurnInput{
		Speed:      char.GetSpeed(),
		TurnNumber: e.data.Round,
	}); err != nil {
		return fmt.Errorf("seed turn economy for %q: %w", actorID, err)
	}
	// The TurnStateChangedEvent push happens at the call site AFTER the
	// TurnStartedEvent, so the client sees the cause (turn started) before the
	// menu/economy refresh (Invariant 12) — seedActorTurn only mutates.
	return nil
}

// heldCharacter returns the hydrated *character.Character for the given entity
// id, or nil if the id is not a player seat or carries no hydrated character
// (no DataJSON at load time). It reads the LoadFromData-cascade-held combatant
// (#689) and type-asserts it — it never re-loads from JSON, preserving the
// single-subscribe discipline (re-loading would re-Apply the character's
// conditions to e.bus, the #684 double-subscribe class).
func (e *Encounter) heldCharacter(id core.EntityID) *character.Character {
	c, ok := e.combatants[id].(*character.Character)
	if !ok {
		return nil
	}
	return c
}
