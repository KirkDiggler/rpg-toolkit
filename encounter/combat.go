package encounter

import (
	"context"
	"errors"
	"fmt"
	"sort"
	"time"

	"github.com/KirkDiggler/rpg-toolkit/dice"
	"github.com/KirkDiggler/rpg-toolkit/encounter/core"
	"github.com/KirkDiggler/rpg-toolkit/encounter/events"
	"github.com/KirkDiggler/rpg-toolkit/encounter/perception"
	dnd5eCharacter "github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/character"
	dnd5eEvents "github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/events"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/refs"
)

// Combat-verb sentinel errors. Wrap with fmt.Errorf for context; the
// orchestrator inspects via errors.Is and maps to gRPC status codes.
var (
	// ErrNotTurnBased is returned by combat verbs when the encounter mode
	// is not ModeTurnBased. Maps to gRPC FailedPrecondition.
	ErrNotTurnBased = errors.New("encounter is not turn-based")
	// ErrNotYourTurn is returned when a verb is called for an actor that
	// is not the active actor. Maps to gRPC FailedPrecondition.
	ErrNotYourTurn = errors.New("not the active actor's turn")
	// ErrUnsupportedAction is returned when TakeAction is called with an
	// action ref the encounter doesn't dispatch. Maps to gRPC Unimplemented.
	ErrUnsupportedAction = errors.New("unsupported action ref")
	// ErrActionDeferred is returned when TakeAction is called with a ref this
	// build surfaces in the menu but defers resolving to a later beat (e.g.
	// move — movement lands in Beat 2). The menu marks such refs
	// available=false; the verb rejects them to stay consistent. Maps to gRPC
	// Unimplemented.
	ErrActionDeferred = errors.New("action deferred to a later beat")
	// ErrActionUnaffordable is returned when a hydrated character takes an
	// action whose action-economy cost it cannot pay this turn (e.g. an attack
	// with no standard action left). The toolkit owns the economy and gates the
	// verb here so an unaffordable action never resolves; the menu's
	// available=false pre-empts it at the UI (Inv 11/12), and this is the
	// structural backstop. Maps to gRPC FailedPrecondition.
	ErrActionUnaffordable = errors.New("action economy cannot afford this action")
	// ErrUnknownTarget is returned when an action targets an entity that
	// is not in the encounter. Maps to gRPC FailedPrecondition.
	ErrUnknownTarget = errors.New("unknown target entity")
	// ErrNoCombatants is returned when a turn-based verb is invoked on an
	// encounter whose initiative roster is empty. Maps to gRPC
	// FailedPrecondition.
	ErrNoCombatants = errors.New("encounter has no combatants in initiative")
	// ErrNonCombatant is returned when TakeAction is invoked for a player
	// whose combat snapshot (HP, AC, AttackBonus, DamageDice) is not
	// fully populated. Maps to gRPC FailedPrecondition.
	ErrNonCombatant = errors.New("actor is not a combatant")
	// ErrUnsupportedAttackDirection is returned by CompleteTakeAction when
	// the (attacker, target) pair encoded in PhasedAttackContext is not a
	// shipped PvE direction (player→monster or monster→player). Player→
	// player and monster→monster are out of scope until a wave adds the
	// corresponding verb. Maps to gRPC Unimplemented.
	ErrUnsupportedAttackDirection = errors.New("unsupported attack direction")
)

// damageTypeUntyped is the fallback damage type emitted when an attacker's
// configured damage type is empty. The downstream translator (rpg-api)
// maps this to the proto's UNSPECIFIED damage type.
const damageTypeUntyped = "untyped"

// actionIDAttack is the canonical action ID for a standard melee/ranged
// attack. Used as AttackInput.ActionRef.ID for both the player path
// (TakeAction) and the NPC path (NPCAct / npcActScripted).
const actionIDAttack = "attack"

// attackActionRef is the canonical ref string the ActionResolvedEvent carries
// for a standard attack. Mirrors the {Module:"dnd5e", Type:"action",
// ID:"attack"} shape the SDK threads into AttackInput.ActionRef (npc.go), in
// the toolkit-canonical "module:type:id" string form so the encounter SDK
// stays free of rulebook ref types. The menu/economy unification PR will
// generalize the attack publish path to carry the actor's submitted ref so
// non-attack actions (Dodge / Dash / the Monk bonus strike) report their own.
const attackActionRef = "dnd5e:action:attack"

// attackEconomyConsumed reports the turn-economy cost the event-faithfulness
// wave attributes to a standard attack: one standard action. This is the
// honest cost the attack path knows today; the menu/economy unification PR
// sources richer consumption (the two-level "Attack ability grants attacks,
// each strike consumes one" model) from the character action economy.
func attackEconomyConsumed() events.EconomyConsumed {
	return events.EconomyConsumed{Actions: 1}
}

// spendAttackEconomy validates and deducts a player's attack off the held
// character's two-level economy, returning what it consumed. It runs BEFORE the
// combat resolver so the economy gates the attack: a character with no action
// left is rejected here and no damage is resolved (#697 Beat-1: economy enforced
// server-side, end-to-end — the menu's available=false pre-empts the case, and
// this is the structural backstop so the verb never resolves an unaffordable
// attack). The attack verb is thus a citizen of the same economy every other
// action uses:
//
//   - ActivateAbility(attack) spends the standard action and grants one attack
//     (capacity), plus any Extra Attack the character has.
//   - ExecuteAction(strike) consumes one of those attacks and runs the
//     character's post-strike grants — for a Monk with a bonus action in hand,
//     this grants the Martial Arts bonus unarmed strike, which then surfaces in
//     the action menu as a bonus-action option (the bonus-action axis of the
//     Beat-1 done-bar).
//
// The real pre/post economy diff is returned so the resolved-action event
// reports the true cost. When no character is hydrated (a flat stat-snapshot
// seat) there is no two-level economy to drive: the honest one-action cost is
// returned and no economy gate applies (those seats opt out of the menu).
//
// Damage/hit resolution is unchanged — it still runs through the combat resolver
// after this returns. This only owns the economy validate+deduct the snapshot
// path lacked.
func (e *Encounter) spendAttackEconomy(player *PlayerData) (events.EconomyConsumed, error) {
	char := e.heldCharacter(player.EntityID)
	if char == nil {
		return attackEconomyConsumed(), nil
	}

	pre := snapshotEconomy(char.GetActionEconomy())

	ctx := context.Background()
	// Activate the Attack ability: spends the action, grants attack capacity.
	// A failure here means the economy can't afford the attack (no action) —
	// reject so the resolver never runs (structural enforcement of Inv 11/12).
	out, err := char.ActivateAbility(ctx, &dnd5eCharacter.ActivateAbilityInput{
		AbilityRef: refs.CombatAbilities.Attack(),
	})
	if err != nil {
		return events.EconomyConsumed{}, fmt.Errorf("activate attack ability: %w", err)
	}
	if !out.Success {
		return events.EconomyConsumed{}, fmt.Errorf("%w: attack: %s", ErrActionUnaffordable, out.Error)
	}
	// Execute one strike: consumes an attack, fires post-strike grants
	// (Monk Martial Arts bonus when a bonus action remains).
	if _, err := char.ExecuteAction(ctx, &dnd5eCharacter.ExecuteActionInput{
		ActionRef: refs.Actions.Strike(),
	}); err != nil {
		// Strike bookkeeping failed after the action was spent — report the
		// action spend captured so far (the action is gone either way).
		return economyConsumedDiff(pre, char.GetActionEconomy()), nil
	}

	return economyConsumedDiff(pre, char.GetActionEconomy()), nil
}

// isPlayerCombatant reports whether a player seat carries the minimum
// combat snapshot required for TakeAction. PlayerInput documents that a
// zero combat snapshot opts a seat out of combat verbs; this helper is
// the gate that contract enforces. AttackBonus may legitimately be 0
// (no proficiency bonus), so it is NOT required.
func isPlayerCombatant(p *PlayerData) bool {
	if p == nil {
		return false
	}
	return p.MaxHP > 0 && p.AC > 0 && p.DamageDice != ""
}

// ActionRef identifies an action via the toolkit's three-part ref shape
// (module / type / id). Wave 2.8 only dispatches on id == "attack".
type ActionRef struct {
	Module string
	Type   string
	ID     string
}

// ActionTarget addresses an entity targeted by an action. Wave 2.8 only
// uses the EntityID variant.
type ActionTarget struct {
	EntityID core.EntityID
}

// SetMode flips the encounter mode. On flip to ModeTurnBased, initiative
// is rolled across all players + monsters using the encounter's roller and
// the round/active-idx are initialized.
//
// Publishes ModeChangedEvent in both directions; on the FreeRoam->TurnBased
// flip also publishes a TurnStartedEvent for the first actor.
func (e *Encounter) SetMode(mode core.EncounterMode) error {
	if mode == core.ModeUnspecified {
		return errors.New("mode unspecified")
	}
	if mode == core.ModeEnded {
		// Terminal state is gameplay-driven (Encounter.checkEncounterEnd
		// fires when the last hostile dies), never set externally. Reject
		// so callers don't accidentally bypass the kill chain.
		return errors.New("ModeEnded is set internally by checkEncounterEnd, not via SetMode")
	}
	if e.data.Mode == core.ModeEnded {
		return ErrEncounterEnded
	}
	from := e.data.Mode
	if from == mode {
		return fmt.Errorf("mode is already %s", mode)
	}

	e.data.Mode = mode
	switch mode {
	case core.ModeTurnBased:
		e.rollInitiative()
		e.data.ActiveIdx = 0
		e.data.Round = 1
	case core.ModeFreeRoam:
		e.data.Initiative = nil
		e.data.ActiveIdx = 0
		e.data.Round = 0
	}

	if err := e.broker.Publish(events.NewModeChangedEvent(
		e.data.ID, e.nextSeq(), from, mode, "", e.allViewersModeChanged(),
	)); err != nil {
		return fmt.Errorf("publish mode change: %w", err)
	}

	if mode == core.ModeTurnBased && len(e.data.Initiative) > 0 {
		// Seed the first actor's economy before announcing their turn, so the
		// menu/economy is ready when the TurnStartedEvent lands (Beat-1: the
		// engine owns turn-start seeding, not the host).
		if err := e.seedActorTurn(context.Background(), e.data.Initiative[0]); err != nil {
			return err
		}
		if err := e.broker.Publish(events.NewTurnStartedEvent(
			e.data.ID, e.nextSeq(), e.data.Initiative[0], e.data.Round,
			e.allViewersTurnStarted(),
		)); err != nil {
			return fmt.Errorf("publish first turn started: %w", err)
		}
		// Push the fresh turn state after announcing the turn (Invariant 12):
		// cause (turn started) before the menu/economy refresh. No correlation
		// id — a turn-start refresh is not caused by an action.
		if err := e.publishTurnStateChanged(e.data.Initiative[0], ""); err != nil {
			return fmt.Errorf("publish first turn state: %w", err)
		}
	}
	return nil
}

// EndTurn ends the active actor's turn and advances initiative. Returns
// the new active actor's id and whether it is an NPC, so the orchestrator
// can decide whether to call NPCAct next.
//
// Publishes TurnEndedEvent for the actor whose turn ended, then
// TurnStartedEvent for the new active actor (with Round incremented when
// the active index wraps).
//
// Returns ErrEncounterEnded when the encounter is in the terminal state
// (ModeEnded). Returns ErrNoCombatants if Initiative is empty (can happen
// if SetMode flipped to TURN_BASED with no players or monsters).
//
// #689: EndTurn now emits the rulebook turn-boundary signal
// (dnd5eEvents.TurnEndTopic) directly on e.bus for the actor whose turn ended,
// so held conditions (e.g. SneakAttack) reset their per-turn state in place
// with NO re-load. This replaces the host's re-loading
// publishTurnEndAndPersistReset + its defer Cleanup #684 patch. ctx flows to
// the publish. The SDK already imports dnd5eEvents; emitting directly here
// (rather than via a pluggable signaler) matches the existing publish pattern
// in this dnd5e-coupled package.
func (e *Encounter) EndTurn(
	ctx context.Context, actorID core.EntityID,
) (newActiveID core.EntityID, isNPC bool, err error) {
	if e.data.Mode == core.ModeEnded {
		return "", false, ErrEncounterEnded
	}
	if e.data.Mode != core.ModeTurnBased {
		return "", false, ErrNotTurnBased
	}
	if len(e.data.Initiative) == 0 {
		return "", false, ErrNoCombatants
	}
	if active := e.ActiveActor(); active != actorID {
		return "", false, fmt.Errorf("%w: active=%q got=%q", ErrNotYourTurn, active, actorID)
	}

	if pubErr := e.broker.Publish(events.NewTurnEndedEvent(
		e.data.ID, e.nextSeq(), actorID, e.allViewersTurnEnded(),
	)); pubErr != nil {
		return "", false, fmt.Errorf("publish turn ended: %w", pubErr)
	}

	// Emit the rulebook turn-boundary on the encounter bus so held conditions
	// reset per-turn state (SneakAttack.UsedThisTurn → false) with no re-load.
	// This is NOT best-effort: it is the ONLY thing that resets per-turn state.
	// If it fails, UsedThisTurn (and similar) would persist into the next turn —
	// nothing else flips the flag, so "resets on the next rehydration" is false.
	// We fail the turn-end before advancing initiative (mirroring the broker
	// publish above) so the host sees the boundary did not fully apply rather
	// than silently carrying stale per-turn state forward. The bus is in-process
	// and synchronous, so this realistically never fails — but a failure is a
	// real error, not something to swallow.
	if pubErr := dnd5eEvents.TurnEndTopic.On(e.bus).Publish(ctx, dnd5eEvents.TurnEndEvent{
		CharacterID: string(actorID),
	}); pubErr != nil {
		return "", false, fmt.Errorf("publish turn boundary (per-turn reset): %w", pubErr)
	}

	e.data.ActiveIdx++
	if e.data.ActiveIdx >= len(e.data.Initiative) {
		e.data.ActiveIdx = 0
		e.data.Round++
	}
	newActive := e.data.Initiative[e.data.ActiveIdx]

	// Seed the new actor's economy before announcing their turn (Beat-1: the
	// engine owns turn-start seeding). No-op for NPCs and stat-snapshot seats.
	if seedErr := e.seedActorTurn(ctx, newActive); seedErr != nil {
		return "", false, seedErr
	}

	if pubErr := e.broker.Publish(events.NewTurnStartedEvent(
		e.data.ID, e.nextSeq(), newActive, e.data.Round, e.allViewersTurnStarted(),
	)); pubErr != nil {
		return "", false, fmt.Errorf("publish turn started: %w", pubErr)
	}
	// Push the new actor's fresh turn state after announcing their turn
	// (Invariant 12). No-op for NPCs / stat-snapshot seats. No correlation id.
	if tsErr := e.publishTurnStateChanged(newActive, ""); tsErr != nil {
		return "", false, fmt.Errorf("publish turn state: %w", tsErr)
	}

	return newActive, e.IsNPC(newActive), nil
}

// TakeAction dispatches a player's action. Wave 2.8 only wires the
// "attack" action ref; other refs return ErrUnsupportedAction.
//
// On hit, mutates target HP and publishes AttackResolvedEvent +
// DamageDealtEvent (per-viewer projection driven by LoS to attacker OR
// target). On miss, publishes only AttackResolvedEvent.
//
// Wave 2.10: when an attack drops the monster's HP to zero, also publishes
// EntityDiedEvent + EntityRemovedEvent for the monster, and (if it was the
// last hostile) flips the encounter to ModeEnded and publishes
// EncounterEndedEvent.
//
// Wave 2.11d: TakeAction is now a thin wrapper over TakeActionPhased that
// returns just the error for callers that don't need reaction-prompt support.
// A returned outcome with Reactions populated is treated as an internal-state
// programming error here — callers that wire a PhasedCombatResolver should
// use TakeActionPhased directly to handle reaction prompts.
//
// Returns ErrEncounterEnded when the encounter is in the terminal state.
// Returns ErrNonCombatant if the player's combat snapshot
// (HP / MaxHP / AC / DamageDice) is not populated — i.e. the seat was
// added without combat fields. PlayerInput documents that a zero combat
// snapshot opts the player out of combat verbs.
func (e *Encounter) TakeAction(playerID core.PlayerID, ref ActionRef, target ActionTarget) error {
	outcome, err := e.TakeActionPhased(playerID, ref, target)
	if err != nil {
		return err
	}
	if outcome != nil && len(outcome.Reactions) > 0 {
		return fmt.Errorf("TakeAction: reactions pending but caller did not use TakeActionPhased; " +
			"use TakeActionPhased to handle reaction prompts")
	}
	return nil
}

// rollInitiative seeds Initiative with all combatants in d20-roll-desc
// order. Ties broken by entity id for determinism.
func (e *Encounter) rollInitiative() {
	type seed struct {
		id   core.EntityID
		roll int
	}
	seeds := make([]seed, 0, len(e.data.Players)+len(e.data.Monsters))
	for _, p := range e.data.Players {
		seeds = append(seeds, seed{id: p.EntityID, roll: rollD20(e.roller)})
	}
	for _, m := range e.data.Monsters {
		seeds = append(seeds, seed{id: m.ID, roll: rollD20(e.roller)})
	}
	sort.Slice(seeds, func(i, j int) bool {
		if seeds[i].roll != seeds[j].roll {
			return seeds[i].roll > seeds[j].roll
		}
		return seeds[i].id < seeds[j].id
	})
	out := make([]core.EntityID, len(seeds))
	for i, s := range seeds {
		out[i] = s.id
	}
	e.data.Initiative = out
}

// rollD20 wraps a roller call so failures fall back to 10 (the d20
// average) — initiative shouldn't fail an entire SetMode call on a roll
// hiccup.
func rollD20(r dice.Roller) int {
	v, err := r.Roll(context.Background(), 20)
	if err != nil || v <= 0 {
		return 10
	}
	return v
}

// publishAttackOutcome emits the first-class ActionResolvedEvent (the cause
// beat, Invariant 9), the AttackResolvedEvent (attack roll detail, always),
// and the DamageDealtEvent (only on hit) with per-viewer projection determined
// by LoS to attacker OR target.
//
// All three events share one correlation id (Invariant 8), derived from the
// ActionResolvedEvent's own (encounter id + sequence) identity so the
// toolkit-owned combat log can group "this damage came from that action"
// without relying on adjacent sequence numbers. The correlation id is set on
// each event before publish; the broker stamps game-event time at the publish
// moment (Invariant 5).
//
// actionRef names the action taken (e.g. "dnd5e:actions:strike") and consumed
// reports what it spent off the turn economy. Wave: the inline attack path
// supplies the known cost; the menu/economy unification PR will source richer
// consumption.
//
// Audience routing matches Move / OpenDoor: viewers who cannot perceive
// the attacker or the target are omitted from PerPlayer entirely (and so
// are excluded from Audience()). The broker delivers only to listed
// viewers, which prevents fog-of-war leakage of out-of-LoS combat.
func (e *Encounter) publishAttackOutcome(
	attackerID, targetID core.EntityID,
	outcome *AttackOutcome,
	targetHPAfter, targetMaxHP int,
	damageType string,
	attackerPos, targetPos core.Hex,
	actionRef string,
	consumed events.EconomyConsumed,
) (core.CorrelationID, error) {
	actionPerPlayer := make(map[core.PlayerID]events.ActionResolvedSlice)
	attackPerPlayer := make(map[core.PlayerID]events.AttackResolvedSlice)
	damagePerPlayer := make(map[core.PlayerID]events.DamageDealtSlice)
	for viewerID, viewer := range e.data.Players {
		if !perception.CanSeeAt(viewer.View, attackerPos) &&
			!perception.CanSeeAt(viewer.View, targetPos) {
			continue
		}
		actionPerPlayer[viewerID] = events.ActionResolvedSlice{Visible: true}
		attackPerPlayer[viewerID] = events.AttackResolvedSlice{Visible: true}
		if outcome.Hit {
			damagePerPlayer[viewerID] = events.DamageDealtSlice{Visible: true}
		}
	}

	// The resolved-action event is the cause beat; its identity seeds the
	// correlation id every effect of this action carries.
	actionSeq := e.nextSeq()
	corrID := e.correlationFor(actionSeq)
	actionEvt := events.NewActionResolvedEvent(
		e.data.ID, actionSeq,
		attackerID, actionRef, targetID,
		consumed, actionPerPlayer,
	)
	if err := e.publishCorrelated(actionEvt, corrID); err != nil {
		return corrID, fmt.Errorf("publish action resolved: %w", err)
	}

	if err := e.publishCorrelated(events.NewAttackResolvedEvent(
		e.data.ID, e.nextSeq(),
		attackerID, targetID,
		outcome.Hit, outcome.Critical, outcome.AttackRoll, outcome.AttackBonus, outcome.TargetAC,
		attackPerPlayer,
	), corrID); err != nil {
		return corrID, fmt.Errorf("publish attack resolved: %w", err)
	}
	if outcome.Hit {
		evt := events.NewDamageDealtEvent(
			e.data.ID, e.nextSeq(),
			targetID, attackerID,
			outcome.Damage, damageType,
			targetHPAfter, targetMaxHP,
			damagePerPlayer,
		)
		evt.Components = outcome.Components
		if err := e.publishCorrelated(evt, corrID); err != nil {
			return corrID, fmt.Errorf("publish damage dealt: %w", err)
		}
	}
	return corrID, nil
}

// correlationFor derives the correlation id for an action-resolution group
// from the resolved-action event's own (encounter, sequence) identity. Riding
// the existing monotonic sequence keeps it deterministic (no new dependency,
// trivially assertable in tests) and unique per action within an encounter.
func (e *Encounter) correlationFor(actionSeq uint64) core.CorrelationID {
	return core.CorrelationID(fmt.Sprintf("corr-%s-%d", e.data.ID, actionSeq))
}

// publishCorrelated sets the correlation id on an event (Invariant 8) and
// publishes it through the broker, which stamps game-event time (Invariant 5).
// The OccurredAt arg to Stamp is zero here — the broker overwrites it at the
// literal publish moment, preserving the correlation id we set.
func (e *Encounter) publishCorrelated(evt events.EncounterEvent, corrID core.CorrelationID) error {
	evt.Stamp(time.Time{}, corrID)
	return e.broker.Publish(evt)
}

// allViewersModeChanged builds a per-player slice marking every player as
// a viewer of the mode change.
func (e *Encounter) allViewersModeChanged() map[core.PlayerID]events.ModeChangedSlice {
	out := make(map[core.PlayerID]events.ModeChangedSlice, len(e.data.Players))
	for id := range e.data.Players {
		out[id] = events.ModeChangedSlice{Visible: true}
	}
	return out
}

// allViewersTurnStarted builds a per-player slice for turn-start events.
func (e *Encounter) allViewersTurnStarted() map[core.PlayerID]events.TurnStartedSlice {
	out := make(map[core.PlayerID]events.TurnStartedSlice, len(e.data.Players))
	for id := range e.data.Players {
		out[id] = events.TurnStartedSlice{Visible: true}
	}
	return out
}

// allViewersTurnEnded builds a per-player slice for turn-end events.
func (e *Encounter) allViewersTurnEnded() map[core.PlayerID]events.TurnEndedSlice {
	out := make(map[core.PlayerID]events.TurnEndedSlice, len(e.data.Players))
	for id := range e.data.Players {
		out[id] = events.TurnEndedSlice{Visible: true}
	}
	return out
}
