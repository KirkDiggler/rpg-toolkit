package encounter

import (
	"context"
	"errors"
	"fmt"
	"sort"

	"github.com/KirkDiggler/rpg-toolkit/dice"
	"github.com/KirkDiggler/rpg-toolkit/encounter/core"
	"github.com/KirkDiggler/rpg-toolkit/encounter/events"
	"github.com/KirkDiggler/rpg-toolkit/encounter/perception"
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
)

// damageTypeUntyped is the fallback damage type emitted when an attacker's
// configured damage type is empty. The downstream translator (rpg-api)
// maps this to the proto's UNSPECIFIED damage type.
const damageTypeUntyped = "untyped"

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
		if err := e.broker.Publish(events.NewTurnStartedEvent(
			e.data.ID, e.nextSeq(), e.data.Initiative[0], e.data.Round,
			e.allViewersTurnStarted(),
		)); err != nil {
			return fmt.Errorf("publish first turn started: %w", err)
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
func (e *Encounter) EndTurn(actorID core.EntityID) (newActiveID core.EntityID, isNPC bool, err error) {
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

	e.data.ActiveIdx++
	if e.data.ActiveIdx >= len(e.data.Initiative) {
		e.data.ActiveIdx = 0
		e.data.Round++
	}
	newActive := e.data.Initiative[e.data.ActiveIdx]

	if pubErr := e.broker.Publish(events.NewTurnStartedEvent(
		e.data.ID, e.nextSeq(), newActive, e.data.Round, e.allViewersTurnStarted(),
	)); pubErr != nil {
		return "", false, fmt.Errorf("publish turn started: %w", pubErr)
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
// Returns ErrEncounterEnded when the encounter is in the terminal state.
// Returns ErrNonCombatant if the player's combat snapshot
// (HP / MaxHP / AC / DamageDice) is not populated — i.e. the seat was
// added without combat fields. PlayerInput documents that a zero combat
// snapshot opts the player out of combat verbs.
func (e *Encounter) TakeAction(playerID core.PlayerID, ref ActionRef, target ActionTarget) error {
	if e.data.Mode == core.ModeEnded {
		return ErrEncounterEnded
	}
	if e.data.Mode != core.ModeTurnBased {
		return ErrNotTurnBased
	}
	if len(e.data.Initiative) == 0 {
		return ErrNoCombatants
	}
	player, ok := e.data.Players[playerID]
	if !ok {
		return fmt.Errorf("player %q not in encounter", playerID)
	}
	if active := e.ActiveActor(); active != player.EntityID {
		return fmt.Errorf("%w: active=%q got=%q", ErrNotYourTurn, active, player.EntityID)
	}
	if ref.ID != "attack" {
		return fmt.Errorf("%w: %q", ErrUnsupportedAction, ref.ID)
	}
	if !isPlayerCombatant(player) {
		return fmt.Errorf("%w: player %q missing HP/AC/DamageDice", ErrNonCombatant, playerID)
	}
	if target.EntityID == "" {
		return fmt.Errorf("%w: empty target", ErrUnknownTarget)
	}
	monster, ok := e.data.Monsters[target.EntityID]
	if !ok {
		return fmt.Errorf("%w: %q", ErrUnknownTarget, target.EntityID)
	}
	if e.combatResolver == nil {
		return ErrNoCombatResolver
	}

	hpBefore := monster.HP
	outcome, err := e.combatResolver.ResolveAttack(AttackInput{
		AttackerID:          player.EntityID,
		TargetID:            target.EntityID,
		ActionRef:           toolkitRef(ref),
		AttackerAttackBonus: player.AttackBonus,
		AttackerDamageDice:  player.DamageDice,
		AttackerDamageType:  player.DamageType,
		TargetAC:            monster.AC,
	})
	if err != nil {
		return fmt.Errorf("combat resolver: %w", err)
	}
	if outcome == nil {
		// Defensive: a well-behaved CombatResolver returns either a non-nil
		// outcome or a non-nil error per the interface contract. Guarding
		// here keeps a misbehaving implementation from panicking the verb.
		return fmt.Errorf("combat resolver: nil outcome with nil error")
	}
	res := attackResolution{
		hit:         outcome.Hit,
		critical:    outcome.Critical,
		attackRoll:  outcome.AttackRoll,
		attackBonus: outcome.AttackBonus,
		targetAC:    outcome.TargetAC,
		damage:      outcome.Damage,
	}
	if res.hit {
		monster.HP -= res.damage
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
		player.EntityID, target.EntityID, res,
		monster.HP, monster.MaxHP, damageType,
		player.View.Position, monster.Position,
	); err != nil {
		return err
	}
	// Fire the death + removal + encounter-end chain only on the
	// HP transition (>0 → 0). Re-attacking an already-dead monster
	// (which can't happen today since dead monsters are spliced
	// out, but the gate is cheap insurance for future paths) does
	// NOT re-fire death events. killEntity also runs the
	// encounter-end predicate which may transition mode to ModeEnded.
	if res.hit && hpBefore > 0 && monster.HP == 0 {
		if err := e.killEntity(target.EntityID, player.EntityID); err != nil {
			return err
		}
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

// attackResolution captures one resolved attack — used by both TakeAction
// (player attacks monster) and NPCAct (monster attacks player). Stand-in
// for the proper combat.ResolveAttack chain (followup).
type attackResolution struct {
	hit         bool
	critical    bool
	attackRoll  int
	attackBonus int
	targetAC    int
	damage      int
}

// resolveAttack rolls a d20+attackBonus attack against targetAC and, on hit,
// rolls damage from the supplied dice notation. Nat-20 is a critical (double
// damage dice). Nat-1 misses regardless of bonus. Empty notation defaults to
// "1d4".
func (e *Encounter) resolveAttack(attackBonus, targetAC int, damageNotation string) attackResolution {
	roll := rollD20(e.roller)
	res := attackResolution{
		attackRoll:  roll,
		attackBonus: attackBonus,
		targetAC:    targetAC,
		critical:    roll == 20,
	}
	switch roll {
	case 1:
		res.hit = false
	case 20:
		res.hit = true
	default:
		res.hit = roll+attackBonus >= targetAC
	}
	if !res.hit {
		return res
	}
	notation := damageNotation
	if notation == "" {
		notation = "1d4"
	}
	pool, err := dice.ParseNotation(notation)
	if err != nil {
		// Indeterminate damage — fall back to 1 to keep the verb non-blocking.
		res.damage = 1
		return res
	}
	rolled := pool.RollContext(context.Background(), e.roller)
	if rolled.Error() != nil {
		res.damage = 1
		return res
	}
	dmg := rolled.Total()
	if res.critical {
		// Crit doubles dice (additive of the dice portion). The simple
		// approximation here is total*2 - modifier; close enough for the
		// stand-in. Followup: full crit-damage chain.
		dmg += rolled.Total() - rolled.Modifier()
	}
	if dmg < 0 {
		dmg = 0
	}
	res.damage = dmg
	return res
}

// publishAttackOutcome emits the AttackResolvedEvent (always) plus the
// DamageDealtEvent (only on hit) with per-viewer projection determined by
// LoS to attacker OR target.
//
// Audience routing matches Move / OpenDoor: viewers who cannot perceive
// the attacker or the target are omitted from PerPlayer entirely (and so
// are excluded from Audience()). The broker delivers only to listed
// viewers, which prevents fog-of-war leakage of out-of-LoS combat.
func (e *Encounter) publishAttackOutcome(
	attackerID, targetID core.EntityID,
	res attackResolution,
	targetHPAfter, targetMaxHP int,
	damageType string,
	attackerPos, targetPos core.Hex,
) error {
	attackPerPlayer := make(map[core.PlayerID]events.AttackResolvedSlice)
	damagePerPlayer := make(map[core.PlayerID]events.DamageDealtSlice)
	for viewerID, viewer := range e.data.Players {
		if !perception.CanSeeAt(viewer.View, attackerPos) &&
			!perception.CanSeeAt(viewer.View, targetPos) {
			continue
		}
		attackPerPlayer[viewerID] = events.AttackResolvedSlice{Visible: true}
		if res.hit {
			damagePerPlayer[viewerID] = events.DamageDealtSlice{Visible: true}
		}
	}
	if err := e.broker.Publish(events.NewAttackResolvedEvent(
		e.data.ID, e.nextSeq(),
		attackerID, targetID,
		res.hit, res.critical, res.attackRoll, res.attackBonus, res.targetAC,
		attackPerPlayer,
	)); err != nil {
		return fmt.Errorf("publish attack resolved: %w", err)
	}
	if res.hit {
		if err := e.broker.Publish(events.NewDamageDealtEvent(
			e.data.ID, e.nextSeq(),
			targetID, attackerID,
			res.damage, damageType,
			targetHPAfter, targetMaxHP,
			damagePerPlayer,
		)); err != nil {
			return fmt.Errorf("publish damage dealt: %w", err)
		}
	}
	return nil
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
