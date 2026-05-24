package encounter

// Wave 2.11d — discrete two-phase combat verbs + reaction trigger drain.
//
// The encounter SDK becomes the explicit orchestrator for two-phase attacks:
//   - Encounter.TakeAction installs a buffered subscriber on the dnd5e
//     ReactionTriggerTopic, calls the resolver's ResolveAttackHit, then
//     either resolves NPC triggers inline + runs phase 2 + publishes events
//     (Resolved=true), or returns the player triggers + the in-flight phase-1
//     context to the caller for prompt-driven completion (Reactions=...).
//   - Encounter.CompleteTakeAction runs phase 2 with the reactor responses
//     baked in and publishes the same outcome events as the inline path.
//
// Per Director B1: the wrapper lives at the SDK layer; the resolver only
// exposes phased verbs. Per Director B3: a small inline buffered subscriber
// is used here rather than a generalized helper.

import (
	"context"
	"fmt"
	"sync"

	"github.com/KirkDiggler/rpg-toolkit/encounter/core"
	"github.com/KirkDiggler/rpg-toolkit/encounter/events"
	dnd5eEvents "github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/events"
)

// TakeActionPhased dispatches a player attack via the discrete two-phase
// resolver path. Use this when the orchestrator needs to surface reaction
// prompts to players; for callers that only need the legacy single-phase
// path, TakeAction wraps this and returns just the error.
//
// On success, the returned outcome is exactly one of:
//   - Resolved=true: phase 1 + phase 2 ran inline; the encounter HP/events
//     are already published.
//   - Reactions populated + AttackContext set: phase 1 published triggers for
//     one or more PLAYER reactors. The orchestrator must persist the encounter
//     snapshot (carrying the in-flight AttackContext via PendingPrompts), push
//     InputRequired{reaction_prompt} to each reactor, and call
//     CompleteTakeAction once all reactors respond.
//
// NPC-only triggers (auto-resolve) collapse into Resolved=true — the wrapper
// resolves them inline before publishing events.
//
//nolint:gocyclo // Two-phase orchestration requires sequencing multiple gates
func (e *Encounter) TakeActionPhased(
	playerID core.PlayerID, ref ActionRef, target ActionTarget,
) (*TakeActionOutcome, error) {
	if e.data.Mode == core.ModeEnded {
		return nil, ErrEncounterEnded
	}
	if e.data.Mode != core.ModeTurnBased {
		return nil, ErrNotTurnBased
	}
	if len(e.data.Initiative) == 0 {
		return nil, ErrNoCombatants
	}
	player, ok := e.data.Players[playerID]
	if !ok {
		return nil, fmt.Errorf("player %q not in encounter", playerID)
	}
	if active := e.ActiveActor(); active != player.EntityID {
		return nil, fmt.Errorf("%w: active=%q got=%q", ErrNotYourTurn, active, player.EntityID)
	}
	if ref.ID != actionIDAttack {
		return nil, fmt.Errorf("%w: %q", ErrUnsupportedAction, ref.ID)
	}
	if !isPlayerCombatant(player) {
		return nil, fmt.Errorf("%w: player %q missing HP/AC/DamageDice", ErrNonCombatant, playerID)
	}
	if target.EntityID == "" {
		return nil, fmt.Errorf("%w: empty target", ErrUnknownTarget)
	}
	monster, ok := e.data.Monsters[target.EntityID]
	if !ok {
		return nil, fmt.Errorf("%w: %q", ErrUnknownTarget, target.EntityID)
	}
	if e.combatResolver == nil {
		return nil, ErrNoCombatResolver
	}

	input := AttackInput{
		AttackerID:          player.EntityID,
		TargetID:            target.EntityID,
		ActionRef:           toolkitRef(ref),
		AttackerAttackBonus: player.AttackBonus,
		AttackerDamageDice:  player.DamageDice,
		AttackerDamageType:  player.DamageType,
		TargetAC:            monster.AC,
		EventBus:            e.bus,
	}

	// Type-assert for the optional phased interface. Resolvers that only
	// implement CombatResolver fall back to the legacy single-phase flow.
	phased, ok := e.combatResolver.(PhasedCombatResolver)
	if !ok {
		outcome, err := e.combatResolver.ResolveAttack(input)
		if err != nil {
			return nil, fmt.Errorf("combat resolver: %w", err)
		}
		if outcome == nil {
			return nil, fmt.Errorf("combat resolver: nil outcome with nil error")
		}
		if err := e.applyAndPublishOutcome(player, monster, outcome); err != nil {
			return nil, err
		}
		return &TakeActionOutcome{Resolved: true}, nil
	}

	// Phased path: install buffered ReactionTriggerTopic subscriber, run
	// phase 1, partition triggers by reactor type.
	triggers, drainCleanup, err := e.installTriggerBuffer()
	if err != nil {
		return nil, fmt.Errorf("install trigger buffer: %w", err)
	}
	defer drainCleanup()

	attackCtx, resolverTriggers, err := phased.ResolveAttackHit(input)
	if err != nil {
		return nil, fmt.Errorf("resolve attack hit: %w", err)
	}

	// Some resolvers may pre-collect triggers themselves; merge with the
	// buffered subscriber's view (the subscriber sees everything published
	// on the encounter bus during the call window).
	allTriggers := append([]ReactionTrigger{}, resolverTriggers...)
	allTriggers = append(allTriggers, *triggers...)

	npcTriggers, playerTriggers := e.partitionTriggers(allTriggers)

	if len(playerTriggers) == 0 {
		// All triggers (if any) come from NPC reactors. Resolve them inline
		// by translating each into a ReactionModifier and running phase 2.
		modifiers := e.npcModifiers(npcTriggers)
		outcome, err := phased.ApplyAttackOutcome(attackCtx, modifiers)
		if err != nil {
			return nil, fmt.Errorf("apply attack outcome: %w", err)
		}
		if outcome == nil {
			return nil, fmt.Errorf("combat resolver: nil outcome with nil error")
		}
		if err := e.applyAndPublishOutcome(player, monster, outcome); err != nil {
			return nil, err
		}
		return &TakeActionOutcome{Resolved: true}, nil
	}

	// Player triggers present — orchestrator must prompt and resume via
	// CompleteTakeAction. NO outcome events are published yet; the encounter
	// snapshot must persist attackCtx (and the pending triggers) so the
	// resume can run phase 2 with the reactors' choices.
	return &TakeActionOutcome{
		Reactions:     playerTriggers,
		AttackContext: attackCtx,
	}, nil
}

// SetPendingReactionPrompt records that a reactor has an outstanding
// reaction prompt. Called by the orchestrator after TakeActionPhased
// returns Reactions != nil.
//
// reactorPlayerID is the player controlling the reactor entity.
// The prompt's AttackContextJSON is the resolver's serialized
// PhasedAttackContext (opaque to the SDK; the orchestrator marshals it via
// its own type before calling).
func (e *Encounter) SetPendingReactionPrompt(
	reactorPlayerID core.PlayerID,
	prompt *PendingReactionPrompt,
) {
	if e.data.PendingReactionPrompts == nil {
		e.data.PendingReactionPrompts = make(map[core.PlayerID]*PendingReactionPrompt)
	}
	e.data.PendingReactionPrompts[reactorPlayerID] = prompt
}

// PublishInputRequiredDelivered emits an InputRequiredDeliveredEvent on the
// broker, audience-of-one targeted at the reactor. The orchestrator calls
// this after SetPendingReactionPrompt to notify the reactor's stream that a
// prompt is now pending in encounter Data.
//
// promptKind is a discriminator (e.g. events.PromptKindReaction); the
// wire-side translator reads PendingReactionPrompts[reactorPlayerID] for
// the prompt content (rulebook-opaque to the SDK).
func (e *Encounter) PublishInputRequiredDelivered(
	reactorPlayerID core.PlayerID,
	promptKind string,
) error {
	evt := events.NewInputRequiredDeliveredEvent(
		e.data.ID, e.nextSeq(), reactorPlayerID, promptKind,
	)
	if err := e.broker.Publish(evt); err != nil {
		return fmt.Errorf("publish input-required-delivered: %w", err)
	}
	return nil
}

// PendingReactionPrompt returns the reactor's outstanding prompt (if any),
// nil otherwise. Called by the orchestrator on SubmitCheck{take_reaction}
// to look up the in-flight phase-1 state to feed into CompleteTakeAction.
func (e *Encounter) PendingReactionPrompt(reactorPlayerID core.PlayerID) *PendingReactionPrompt {
	return e.data.PendingReactionPrompts[reactorPlayerID]
}

// ClearPendingReactionPrompt removes the reactor's outstanding prompt.
// Called by the orchestrator after CompleteTakeAction completes (success
// or fail) to free the reactor for the next attack window.
func (e *Encounter) ClearPendingReactionPrompt(reactorPlayerID core.PlayerID) {
	delete(e.data.PendingReactionPrompts, reactorPlayerID)
}

// CompleteTakeAction runs phase 2 of a previously-paused two-phase attack
// with the reactor responses baked in. The orchestrator passes the persisted
// PhasedAttackContext (from a prior TakeActionPhased call) and the list of
// ReactionModifiers built from take_reaction=true responses.
//
// Publishes the AttackResolvedEvent + (on hit) DamageDealtEvent and runs the
// same death/removal chain as the inline path.
//
// Wave 2.11e: accepts either PvE attack direction — player→monster (the
// original 2.11d shape, dispatched via TakeActionPhased) and monster→player
// (the Shield resume direction, dispatched via NPCAct that paused for a
// player reaction). The SDK resolves direction from AttackerID lookup
// against the Players + Monsters maps; the orchestrator passes the
// PhasedAttackContext through unchanged for either direction. Player→
// player attacks return ErrUnsupportedAttackDirection.
func (e *Encounter) CompleteTakeAction(attackCtx *PhasedAttackContext, modifiers []ReactionModifier) error {
	if attackCtx == nil {
		return fmt.Errorf("nil attack context")
	}
	if e.data.Mode == core.ModeEnded {
		return ErrEncounterEnded
	}
	phased, ok := e.combatResolver.(PhasedCombatResolver)
	if !ok {
		return fmt.Errorf("combat resolver does not support phased completion")
	}

	// Resolve attacker against both maps; CompleteTakeAction is the only
	// resume verb so the direction must be inferred from the persisted
	// context (the orchestrator does not pass a direction hint).
	//
	// Cross-map uniqueness of entity IDs is not enforced at AddPlayer /
	// AddMonster time (PlayerData is keyed by PlayerID with EntityID as a
	// separate field; MonsterData is keyed by its EntityID directly), so
	// a player and a monster CAN share an EntityID. If that happens, the
	// dispatch below is ambiguous — reject the resume call rather than
	// silently routing to the wrong publish helper.
	attackerPlayer := e.findPlayerByEntityID(attackCtx.AttackerID)
	attackerMonster := e.data.Monsters[attackCtx.AttackerID]
	if attackerPlayer == nil && attackerMonster == nil {
		return fmt.Errorf("attacker %q not in encounter", attackCtx.AttackerID)
	}
	if attackerPlayer != nil && attackerMonster != nil {
		return fmt.Errorf("ambiguous attacker %q: matches both a player and a monster",
			attackCtx.AttackerID)
	}

	// Resolve target against both maps for the symmetric reason.
	targetMonster := e.data.Monsters[attackCtx.TargetID]
	targetPlayer := e.findPlayerByEntityID(attackCtx.TargetID)
	if targetMonster == nil && targetPlayer == nil {
		return fmt.Errorf("%w: %q", ErrUnknownTarget, attackCtx.TargetID)
	}
	if targetMonster != nil && targetPlayer != nil {
		return fmt.Errorf("ambiguous target %q: matches both a player and a monster",
			attackCtx.TargetID)
	}

	outcome, err := phased.ApplyAttackOutcome(attackCtx, modifiers)
	if err != nil {
		return fmt.Errorf("apply attack outcome: %w", err)
	}
	if outcome == nil {
		return fmt.Errorf("combat resolver: nil outcome with nil error")
	}

	// Dispatch by the resolved direction. PvE only — player→player and
	// monster→monster shapes are out of scope until a wave adds the
	// corresponding verb.
	switch {
	case attackerPlayer != nil && targetMonster != nil:
		return e.applyAndPublishOutcome(attackerPlayer, targetMonster, outcome)
	case attackerMonster != nil && targetPlayer != nil:
		return e.applyAndPublishNPCOutcome(attackerMonster, targetPlayer, outcome)
	default:
		return fmt.Errorf("%w: %q→%q",
			ErrUnsupportedAttackDirection, attackCtx.AttackerID, attackCtx.TargetID)
	}
}

// applyAndPublishNPCOutcome mutates the target player's HP, publishes the
// attack/damage events with per-viewer projection, and fires the partial
// player-death event on the >0 → 0 transition. The NPC-attacker mirror of
// applyAndPublishOutcome.
//
// Wave 2.10 partial player-death semantics apply: a player whose HP hits
// 0 fires EntityDiedEvent but is NOT removed from initiative and does NOT
// terminate the encounter — dying-state and TPK are Wave 2.11+ territory.
//
// Shared between two call sites:
//   - applyCapturedAttacks (inline NPC turn that did NOT pause for a
//     player reaction) — publishes the outcome immediately after the
//     resolver returns.
//   - CompleteTakeAction (NPC→player Shield-resume direction) — publishes
//     the outcome after the player's SubmitCheck{take_reaction} reaches
//     the SDK via the orchestrator.
//
// Wave 2.11e: extracted from the per-attack body of applyCapturedAttacks
// (encounter/npc.go) so the Shield resume path emits the same shape as
// the inline path. Without this, NPC-attacker resume would diverge from
// NPC-attacker inline on death-event emission and damage-type fallback.
func (e *Encounter) applyAndPublishNPCOutcome(monster *MonsterData, player *PlayerData, outcome *AttackOutcome) error {
	hpBefore := player.HP
	if outcome.Hit {
		player.HP -= outcome.Damage
		if player.HP < 0 {
			player.HP = 0
		}
	}

	damageType := outcome.DamageType
	if damageType == "" {
		damageType = monster.DamageType
	}
	if damageType == "" {
		damageType = damageTypeUntyped
	}
	if err := e.publishAttackOutcome(
		monster.ID, player.EntityID, outcome,
		player.HP, player.MaxHP, damageType,
		monster.Position, player.View.Position,
	); err != nil {
		return err
	}
	if outcome.Hit && hpBefore > 0 && player.HP == 0 {
		if err := e.publishPlayerDied(player.EntityID, monster.ID); err != nil {
			return err
		}
	}
	return nil
}

// applyAndPublishOutcome mutates target HP, publishes the attack/damage
// events with per-viewer projection, and fires the death + encounter-end
// chain on the >0 → 0 transition. Shared between the legacy single-phase
// path and the phased CompleteTakeAction path (player→monster direction).
// applyAndPublishNPCOutcome is the monster→player mirror.
func (e *Encounter) applyAndPublishOutcome(player *PlayerData, monster *MonsterData, outcome *AttackOutcome) error {
	hpBefore := monster.HP
	if outcome.Hit {
		monster.HP -= outcome.Damage
		if monster.HP < 0 {
			monster.HP = 0
		}
	}

	damageType := outcome.DamageType
	if damageType == "" {
		damageType = player.DamageType
	}
	if damageType == "" {
		damageType = damageTypeUntyped
	}
	if err := e.publishAttackOutcome(
		player.EntityID, monster.ID, outcome,
		monster.HP, monster.MaxHP, damageType,
		player.View.Position, monster.Position,
	); err != nil {
		return err
	}
	if outcome.Hit && hpBefore > 0 && monster.HP == 0 {
		if err := e.killEntity(monster.ID, player.EntityID); err != nil {
			return err
		}
	}
	return nil
}

// installTriggerBuffer subscribes a buffered slice handler to
// ReactionTriggerTopic on the encounter bus. Returns the slice (so the caller
// reads it after the chain completes) and a cleanup func that unsubscribes.
//
// Per Director B3, this is a small inline buffer — not a generalized helper.
// If a second consumer of the pattern appears, refactor to events.BufferedSubscriber.
//
// Concurrency: today's bus implementation is synchronous (handlers run in the
// publisher's goroutine), so a sync.Mutex on the buffer would be redundant.
// We add one anyway for safety parity with the test helpers that use the same
// pattern (opportunity_attack_test.go / shield_spell_test.go), and to future-
// proof against a bus implementation that fans handlers out concurrently.
func (e *Encounter) installTriggerBuffer() (*[]ReactionTrigger, func(), error) {
	var mu sync.Mutex
	collected := &[]ReactionTrigger{}
	topic := dnd5eEvents.ReactionTriggerTopic.On(e.bus)
	handler := func(_ context.Context, evt dnd5eEvents.ReactionTriggerEvent) error {
		mu.Lock()
		*collected = append(*collected, ReactionTrigger{
			ReactorID:    core.EntityID(evt.ReactorID),
			ConditionRef: evt.ConditionRef,
			TriggerKind:  string(evt.TriggerKind),
			SourceEntity: core.EntityID(evt.SourceEntity),
			Payload:      evt.Payload,
		})
		mu.Unlock()
		return nil
	}
	subID, err := topic.Subscribe(context.Background(), handler)
	if err != nil {
		return nil, func() {}, err
	}
	cleanup := func() {
		_ = topic.Unsubscribe(context.Background(), subID)
	}
	return collected, cleanup, nil
}

// partitionTriggers splits the buffered triggers into NPC-reactor (auto-
// resolve inline) and player-reactor (surface to orchestrator) groups.
//
// A reactor is a player iff its EntityID matches a PlayerData.EntityID in
// the encounter; otherwise it is treated as an NPC.
func (e *Encounter) partitionTriggers(triggers []ReactionTrigger) (npc, player []ReactionTrigger) {
	for _, t := range triggers {
		if e.findPlayerByEntityID(t.ReactorID) != nil {
			player = append(player, t)
		} else {
			npc = append(npc, t)
		}
	}
	return npc, player
}

// shieldRef is the canonical core.Ref string for the Shield spell. Used to
// match NPC reaction triggers against the supported spell-cost auto-resolve
// modifier in npcModifiers.
const shieldRef = "dnd5e:spells:shield"

// npcModifiers translates NPC reaction triggers into ReactionModifiers for
// inline phase-2 application. Wave 2.11d default: NPCs always take available
// reactions (no AI strategy yet); the modifier shape per condition ref is
// hard-coded here for the supported reactions.
//
// Future: per-NPC strategy hints, multi-modifier reactions, ref-driven
// modifier table loaded from the rulebook.
func (e *Encounter) npcModifiers(triggers []ReactionTrigger) []ReactionModifier {
	modifiers := make([]ReactionModifier, 0, len(triggers))
	for _, t := range triggers {
		// Wave 2.11d only auto-resolves Shield from NPC reactors. OA does not
		// produce an AC modifier on the attacker — it is a re-entrant attack.
		// Wave 2.11d defers NPC OA execution to a later slice (the inline call
		// from MoveEntity is already today's behavior; the new condition-based
		// path needs an OA-as-reaction resolver verb, out of scope here).
		if t.ConditionRef == shieldRef {
			modifiers = append(modifiers, ReactionModifier{
				ConditionRef: t.ConditionRef,
				ACBonus:      5,
			})
		}
	}
	return modifiers
}
