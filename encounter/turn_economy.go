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
//
// rpg-project#75 (Beat 2, #699): seedActorTurn also revives the rulebook
// turn-start boundary signal (dnd5eEvents.TurnStartTopic) on e.bus, mirroring
// EndTurn's existing dnd5eEvents.TurnEndTopic publish (combat.go) — same bus,
// same not-best-effort error handling. This wakes THREE dormant subscribers,
// not just Dodging: DodgingCondition's self-removal (dodging.go), Barbarian
// RecklessAttackCondition's self-removal (reckless_attack.go — this wave does
// not touch Reckless Attack's code, but its lifecycle now actually fires), and
// UnconsciousCondition's auto-death-save roll (unconscious.go) — the mechanism
// behind the wave's own player-gets-hit/death-gate playtest beat (rpg-api#612).
// The publish is unconditional (unlike the economy-seed below it), because
// conditions key off event.CharacterID == their own owner id regardless of
// whether that actor has a hydrated *character.Character.

import (
	"context"
	"fmt"

	"github.com/KirkDiggler/rpg-toolkit/encounter/core"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/character"
	dnd5eEvents "github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/events"
)

// seedActorTurn publishes the rulebook turn-start boundary for the actor whose
// turn is beginning, then — when that actor is a player with a hydrated
// character — initializes their action economy. It calls character.StartTurn
// on the held instance, which sets one action, one bonus action, one reaction,
// and movement from the character's speed, and clears granted capacity from
// the prior turn.
//
// NPCs and stat-snapshot seats (no hydrated character) still receive the
// turn-start boundary publish (see package doc above) but skip economy
// seeding — they carry no character economy to seed.
//
// Called from the two turn-start sites: SetMode's flip to ModeTurnBased (first
// actor) and EndTurn (the next actor). Errors are returned so the caller fails
// the turn-boundary rather than advancing with an unseeded economy or a
// silently-dropped turn-start signal.
func (e *Encounter) seedActorTurn(ctx context.Context, actorID core.EntityID) error {
	// NOT best-effort: the same failure semantics as EndTurn's TurnEndTopic
	// publish. If this fails, Dodging/RecklessAttack never self-remove and
	// Unconscious never auto-rolls death saves for this turn — a real error,
	// not something to swallow.
	if pubErr := dnd5eEvents.TurnStartTopic.On(e.bus).Publish(ctx, dnd5eEvents.TurnStartEvent{
		CharacterID: string(actorID),
		Round:       e.data.Round,
	}); pubErr != nil {
		return fmt.Errorf("publish turn boundary (turn start) for %q: %w", actorID, pubErr)
	}

	char := e.heldCharacter(actorID)
	if char == nil {
		return nil // NPC or stat-snapshot seat — no character economy to seed.
	}

	// #733: an unconscious/downed player (HP<=0) gets no seeded action economy
	// — the TurnStartTopic publish above still drives their auto-death-save
	// roll, but they cannot act. char.EndTurn is a pure local mutation
	// (zeroes actions/bonus/reaction/movement remaining, resets Granted, no
	// event side effects — see action_economy.go) reused here instead of
	// inventing new zeroing logic. Design intent: "ActionEconomy for an
	// unconscious actor should not offer actions" (rpg-project#75 Beat 2
	// mechanical-effects design doc, "Turn-start is dead code").
	//
	// Gated on char.GetHitPoints() — the held character's LIVE HP — not the
	// encounter's PlayerData.HP snapshot. The TurnStartTopic publish above is
	// synchronous and can itself change the held character's HP before this
	// check runs: UnconsciousCondition's nat-20 death-save path publishes
	// HealingReceivedEvent, which character.onHealingReceived applies to
	// c.hitPoints directly. PlayerData.HP is only refreshed from the held
	// character at ToData() time, so reading the snapshot here would still see
	// the pre-revival value and zero the economy of an actor who just came
	// back to 1 HP (Copilot review, PR #739).
	if char.GetHitPoints() <= 0 {
		if _, err := char.EndTurn(ctx, &character.EndTurnInput{}); err != nil {
			return fmt.Errorf("zero turn economy for downed actor %q: %w", actorID, err)
		}
		return nil
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

// seedActiveActorIfUnseeded is LoadFromData's catch-up for a turn seed that
// was structurally impossible at the moment the turn started (rpg-toolkit#757).
//
// The gap: SetMode's flip to ModeTurnBased can now fire on an UN-hydrated
// encounter — checkCombatEntry runs inline at AddMonster, and the production
// creation flow is New()+AddPlayer+AddMonster, where nothing is hydrated (New
// never hydrates; only a LoadFromData round-trip does). seedActorTurn at that
// moment finds no held character for the first active player and correctly
// skips — but SetMode and EndTurn are the only two turn-start seeding sites,
// so without this catch-up that player's economy would never be seeded for
// their first turn: every TakeAction fails "not in combat". The same latent
// gap predates #757 for any host that called SetMode on a fresh encounter
// (devseed's --inject-combat shape); combat entry just made it the default
// production path.
//
// Detection is the held character's own persisted combat state: InCombat()
// (== actionEconomy != nil) survives ToData/LoadFromData round-trips, so a
// mid-turn reload (economy seeded, partially spent) sees InCombat() == true
// and skips — this catch-up cannot re-seed (and thereby refill) a live
// turn's economy. It fires exactly once: the first load after the missed
// seed, after which the seeded economy persists.
//
// The refreshed turn state is push-published after seeding (Invariant 12):
// the TurnStateChangedEvent the client got at the SetMode flip snapshotted an
// un-hydrated (empty) menu/economy; without a fresh push the real menu would
// be silently stale until the actor's next action. No TurnStartedEvent —
// the turn already started and was announced at the flip; this completes its
// seeding, it doesn't restart it.
func (e *Encounter) seedActiveActorIfUnseeded(ctx context.Context) error {
	if e.data.Mode != core.ModeTurnBased || len(e.data.Initiative) == 0 {
		return nil
	}
	if e.data.ActiveIdx < 0 || e.data.ActiveIdx >= len(e.data.Initiative) {
		return nil
	}
	actorID := e.data.Initiative[e.data.ActiveIdx]
	char := e.heldCharacter(actorID)
	if char == nil || char.InCombat() {
		return nil // NPC / stat-snapshot seat, or already seeded — nothing missed.
	}
	if err := e.seedActorTurn(ctx, actorID); err != nil {
		return err
	}
	if err := e.publishTurnStateChanged(actorID, ""); err != nil {
		return fmt.Errorf("publish caught-up turn state: %w", err)
	}
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
