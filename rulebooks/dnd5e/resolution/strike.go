// Copyright (C) 2026 Kirk Diggler
// SPDX-License-Identifier: GPL-3.0-or-later

package resolution

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/KirkDiggler/rpg-toolkit/dice"
	"github.com/KirkDiggler/rpg-toolkit/events"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/combat"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/damage"
	dnd5eEvents "github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/events"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/monster"
	monsterActions "github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/monster/actions"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/refs"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/saves"
)

// criticalThreshold is the roll at or above which an attack crits. A flat 20
// here rather than a field: nothing in slice 1 lowers it, and the attack chain
// carries the number so an effect that does will be read from the folded event
// rather than from this constant.
const criticalThreshold = 20

// StrikeInput describes one attack.
//
// The attack itself arrives as content — the action's persisted data, exactly
// as a monster's stat block carries it — rather than as a runtime object with
// its numbers pulled out. That is the same seam every other input here uses,
// and it is what lets the headline test drive the catalog wolf's own bite
// instead of a hand-built approximation of it.
type StrikeInput struct {
	// AttackerID names the participant swinging.
	AttackerID string

	// TargetID names the participant being swung at.
	TargetID string

	// Action is the attack, as the content declares it. Slice 1 understands
	// the bite; another ref is refused by name rather than guessed at.
	Action monster.ActionData

	// Roller rolls the attack and its damage. Nil takes the default roller.
	Roller dice.Roller
}

// StrikeOutcome is what an attack produced, in enough detail to explain it.
//
// A bare "hit for 6" cannot answer the question a player actually asks, which
// is why: what made it hit, what the target's AC was, which effects granted
// advantage, and — when the blow carried a rider — whether the save that gated
// it succeeded. All of that is here, and the miss case carries the same
// breakdown rather than an empty struct.
type StrikeOutcome struct {
	// AttackerID and TargetID name the two sides.
	AttackerID string
	TargetID   string

	// Roll is the d20 as rolled, after advantage or disadvantage was applied.
	Roll int

	// Total is Roll plus the attack bonus the chain settled on.
	Total int

	// TargetAC is what the total had to reach.
	TargetAC int

	// Hit is whether it landed. A natural 1 never does and a natural 20
	// always does, whatever the arithmetic says.
	Hit bool

	// Critical is whether the roll was a natural 20.
	Critical bool

	// Folded is the attack chain after every subscriber had its say — the
	// record of which effects granted advantage or imposed disadvantage.
	Folded dnd5eEvents.AttackChainEvent

	// Damage is what was dealt. Zero on a miss.
	Damage int

	// Contest is the interaction that gated the blow's rider, when the action
	// declared one and the blow landed. Nil when the action carries no gate,
	// and nil on a miss — a strike that misses rolls no save.
	Contest *ContestOutcome
}

func (StrikeOutcome) isOutcome() {}

// NewStrike returns the machine for one attack: fold the attack chain, roll it
// against AC, and on a hit deal damage and contest whatever rider the action
// declared.
//
// Linear, and re-enterable by construction rather than by intent. Every phase
// boundary is a yielded step, so the machine's own fields are the only state
// there is and nothing accumulates on the Go stack between phases — which is
// what the reaction windows of ADR-0027 will need, and why they can be added
// without rebuilding this. They are not here: reactions are wave 5.
func NewStrike(in *StrikeInput) Machine {
	return &strikeMachine{in: in}
}

type strikeMachine struct {
	in *StrikeInput

	// cast is the sheets this interaction attached, kept because a phase after
	// the first needs them and a step's closure is handed only a bus.
	cast *Participants

	// attack is the content's numbers, decoded once.
	attack strikeProfile

	// outcome accumulates across phases. It is the machine's whole state, and
	// the reason a suspension between any two phases would need nothing else.
	outcome StrikeOutcome
}

// strikeProfile is what this machine needs to know about an attack, decoded
// from whatever action data it was handed.
type strikeProfile struct {
	attackBonus int
	damageDice  string
	damageType  string
	gate        *saves.SaveGate
}

// Start decodes the action, reads the target's AC, and folds the attack chain.
func (m *strikeMachine) Start(_ context.Context, cast *Participants) (Step, error) {
	if m.in == nil {
		return nil, ErrNilInput
	}
	if m.in.AttackerID == "" || m.in.TargetID == "" {
		return nil, fmt.Errorf("%w: a strike needs an attacker and a target", ErrNilInput)
	}

	m.cast = cast

	profile, err := decodeStrikeProfile(m.in.Action)
	if err != nil {
		return nil, err
	}
	m.attack = profile

	if _, err := combatantFor(cast, m.in.AttackerID); err != nil {
		return nil, err
	}

	target, err := combatantFor(cast, m.in.TargetID)
	if err != nil {
		return nil, err
	}

	// Plain AC, deliberately. Folding the AC chain is GetEffectiveAC's job and
	// it folds on the character's own parked bus — the legacy shape this
	// migration is retiring — so slice 1 uses the number on the sheet and
	// names the gap rather than papering it. Divestment debt — #965 slice 2.
	m.outcome = StrikeOutcome{
		AttackerID: m.in.AttackerID,
		TargetID:   m.in.TargetID,
		TargetAC:   target.AC(),
	}

	event := dnd5eEvents.AttackChainEvent{
		AttackerID:        m.in.AttackerID,
		TargetID:          m.in.TargetID,
		WeaponRef:         &m.in.Action.Ref,
		IsMelee:           true,
		AttackBonus:       profile.attackBonus,
		TargetAC:          target.AC(),
		CriticalThreshold: criticalThreshold,
	}

	return gatherAttack(event, m.afterAttackChain), nil
}

// afterAttackChain rolls the die the fold decided the shape of, and decides
// whether the blow lands.
//
// The advantage rules are 5e's and are mirrored from combat.ResolveAttackHit
// rather than invented: advantage and disadvantage cancel exactly, a natural 1
// always misses, a natural 20 always hits. Rolling here rather than calling
// combat is not a preference — every exported attack entry point in that
// package requires an event bus, and there is no bus-free roll to call.
// Divestment debt — #965 slice 2.
func (m *strikeMachine) afterAttackChain(ctx context.Context, folded dnd5eEvents.AttackChainEvent) (Step, error) {
	roller := m.in.Roller
	if roller == nil {
		roller = dice.NewRoller()
	}

	hasAdvantage := len(folded.AdvantageSources) > 0
	hasDisadvantage := len(folded.DisadvantageSources) > 0

	var roll int
	var err error
	switch {
	case hasAdvantage == hasDisadvantage:
		// Both or neither: one die either way.
		roll, err = roller.Roll(ctx, 20)
	case hasAdvantage:
		roll, err = rollTwice(ctx, roller, takeHigher)
	default:
		roll, err = rollTwice(ctx, roller, takeLower)
	}
	if err != nil {
		return nil, fmt.Errorf("roll attack: %w", err)
	}

	m.outcome.Roll = roll
	m.outcome.Total = roll + folded.AttackBonus
	m.outcome.TargetAC = folded.TargetAC
	m.outcome.Folded = folded

	// A natural 20 is the only automatic hit and a natural 1 the only
	// automatic miss; everything between is arithmetic. The crit range is
	// not a hit range: an effect that widens CriticalThreshold to 19–20
	// widens which HITS crit, never which rolls hit — a 19 that cannot
	// reach the AC is still a miss.
	switch roll {
	case 20:
		m.outcome.Hit = true
	case 1:
		m.outcome.Hit = false
	default:
		m.outcome.Hit = m.outcome.Total >= folded.TargetAC
	}
	m.outcome.Critical = m.outcome.Hit && roll >= folded.CriticalThreshold

	if !m.outcome.Hit {
		// A miss ends the strike here: no damage, and no save. The rider the
		// action declares is gated on the blow landing, so a bite that misses
		// rolls no save (rpg-toolkit#962's residual).
		return Done{Outcome: m.outcome}, nil
	}

	return m.rollDamage(ctx, roller)
}

// rollDamage rolls the action's damage dice and yields the fold that lets
// effects modify it (ADR-0026's Resolve).
func (m *strikeMachine) rollDamage(ctx context.Context, roller dice.Roller) (Step, error) {
	pool, err := dice.ParseNotation(m.attack.damageDice)
	if err != nil {
		return nil, fmt.Errorf("%w: damage dice %q: %w", ErrBadAttack, m.attack.damageDice, err)
	}

	result := pool.RollContext(ctx, roller)
	if result.Error() != nil {
		return nil, fmt.Errorf("roll damage: %w", result.Error())
	}

	// Per-die, not summed: OriginalDiceRolls/FinalDiceRolls are a per-die
	// contract — downstream rerolls address dice by index — and the
	// notation's static modifier is not a die, so it rides FlatBonus and is
	// never doubled.
	rolls := flattenDice(result.Rolls())

	// A critical hit doubles the weapon pool's DICE and nothing else
	// (ADR-0036: only the weapon pool doubles, and a flat modifier is not a
	// die). Combat's fold records IsCritical but rolls nothing, so the
	// doubling happens here, where the dice are.
	if m.outcome.Critical {
		again := pool.RollContext(ctx, roller)
		if again.Error() != nil {
			return nil, fmt.Errorf("roll critical damage: %w", again.Error())
		}
		rolls = append(rolls, flattenDice(again.Rolls())...)
	}

	component := dnd5eEvents.DamageComponent{
		Source:            dnd5eEvents.DamageSourceWeapon,
		SourceRef:         &m.in.Action.Ref,
		OriginalDiceRolls: rolls,
		FinalDiceRolls:    append([]int(nil), rolls...),
		FlatBonus:         result.Modifier(),
		DamageType:        damage.Type(m.attack.damageType),
		IsCritical:        m.outcome.Critical,
	}

	return foldDamage(&combat.ResolveDamageInput{
		AttackerID:   m.in.AttackerID,
		TargetID:     m.in.TargetID,
		Components:   []dnd5eEvents.DamageComponent{component},
		IsCritical:   m.outcome.Critical,
		HasAdvantage: len(m.outcome.Folded.AdvantageSources) > 0,
		WeaponDamage: m.attack.damageDice,
		IsMelee:      true,
		WeaponRef:    &m.in.Action.Ref,
	}, m.afterDamageChain), nil
}

// afterDamageChain applies what the fold settled on — bus-free, straight onto
// the sheet (ADR-0026's Apply). Notify is deliberately absent: publishing
// DamageReceivedEvent would apply the damage a second time to a monster
// target, whose sheet-keeper treats that topic as an instruction — the
// one-topic-two-meanings finding #965 slice 2 owes a classification for.
// Pinned by TestAMonsterTargetTakesItsDamageOnce.
func (m *strikeMachine) afterDamageChain(
	ctx context.Context, resolved *combat.ResolveDamageOutput,
) (Step, error) {
	target, err := combatantFor(m.cast, m.in.TargetID)
	if err != nil {
		return nil, err
	}

	instances := make([]combat.DamageInstance, 0, len(resolved.FinalInstances))
	for _, instance := range resolved.FinalInstances {
		instances = append(instances, combat.DamageInstance{
			Amount: instance.Amount,
			Type:   string(instance.Type),
		})
	}

	// Bus-free, and the only phase that is: applying damage is the sheet's own
	// business and takes no bus on either a character or a monster.
	applied := target.ApplyDamage(ctx, &combat.ApplyDamageInput{
		Instances:  instances,
		IsCritical: m.outcome.Critical,
	})
	m.outcome.Damage = applied.TotalDamage

	return m.afterDamage(ctx)
}

// afterDamage would be where ADR-0026's Notify goes, and deliberately is not.
//
// Publishing DamageReceivedEvent here applies the damage a SECOND time to a
// monster target: monster.SheetKeeper subscribes to that topic and its handler
// calls TakeDamage, so the event is an instruction to that listener rather than
// a notification. Measured, not assumed — a 4-damage bite took a wolf from 11
// to 3 with the publish in place. A character has no such handler, so the same
// event is inert for half the roster: the topic means two different things
// depending on who is listening.
//
// That is the rules-versus-notification classification #965 slice 2 exists to
// make, arriving as evidence rather than as opinion, so slice 1 applies damage
// once and announces nothing. What it costs is real and worth naming: Undead
// Fortitude listens to this topic legitimately, and does not fire here. It
// converts with its gate (#977), by which point the double-apply is gone.
//
// Divestment debt — #965 slice 2.
func (m *strikeMachine) afterDamage(ctx context.Context) (Step, error) {
	return m.afterNotify(ctx)
}

// afterNotify contests the blow's rider, if it declared one.
func (m *strikeMachine) afterNotify(_ context.Context) (Step, error) {
	if m.attack.gate == nil {
		return Done{Outcome: m.outcome}, nil
	}

	return requestContest(&ContestInput{
		Gate:        m.attack.gate,
		SaverID:     m.in.TargetID,
		Consequence: ImposeCondition(refs.Conditions.Prone(), dnd5eEvents.ConditionProne),
		DamageTaken: m.outcome.Damage,
		Roller:      m.in.Roller,
	}, func(_ context.Context, contest ContestOutcome) (Step, error) {
		m.outcome.Contest = &contest

		return Done{Outcome: m.outcome}, nil
	}), nil
}

// decodeStrikeProfile reads an action's numbers out of the data content
// declares it with.
//
// One ref for slice 1. An action this build cannot read is refused by name
// rather than treated as a generic swing, because guessing an attack bonus is
// how a stat block starts lying again.
func decodeStrikeProfile(action monster.ActionData) (strikeProfile, error) {
	if action.Ref.ID != refs.MonsterActions.Bite().ID {
		return strikeProfile{}, fmt.Errorf("%w: %q (slice 1 understands the bite)", ErrBadAttack, action.Ref.ID)
	}

	var config monsterActions.BiteConfig
	if len(action.Config) > 0 {
		if err := json.Unmarshal(action.Config, &config); err != nil {
			return strikeProfile{}, fmt.Errorf("%w: %w", ErrBadAttack, err)
		}
	}

	if config.DamageDice == "" {
		return strikeProfile{}, fmt.Errorf("%w: the action declares no damage dice", ErrBadAttack)
	}

	profile := strikeProfile{
		attackBonus: config.AttackBonus,
		damageDice:  config.DamageDice,
		damageType:  string(config.DamageType),
		gate:        config.SaveGate,
	}

	// A bite persisted by an older build carries a bare knockdown DC rather
	// than a gate; NewBiteAction is what translates it, so the profile comes
	// through the action rather than the config when the gate is absent.
	if profile.gate == nil {
		if bite, ok := mustLoadBite(action); ok {
			profile.gate = bite.SaveGate()
		}
	}

	return profile, nil
}

// mustLoadBite reconstitutes the action so its own loader can apply whatever
// translation the content needs. Failure is not fatal: the numbers are already
// decoded, and only the gate would be missing.
func mustLoadBite(action monster.ActionData) (*monsterActions.BiteAction, bool) {
	loaded, err := monsterActions.LoadAction(action)
	if err != nil {
		return nil, false
	}

	bite, ok := loaded.(*monsterActions.BiteAction)

	return bite, ok
}

// combatantFor finds a participant's sheet as something that can be hit.
func combatantFor(cast *Participants, id string) (combat.Combatant, error) {
	if character, ok := cast.Character(id); ok {
		return character, nil
	}
	if monster, ok := cast.Monster(id); ok {
		return monster, nil
	}

	return nil, fmt.Errorf("%w: %q", ErrNoCombatant, id)
}

// rollTwice rolls two d20s and picks one, which is what advantage and
// disadvantage each are.
func rollTwice(ctx context.Context, roller dice.Roller, pick func(a, b int) int) (int, error) {
	rolls, err := roller.RollN(ctx, 2, 20)
	if err != nil {
		return 0, err
	}
	if len(rolls) < 2 {
		return 0, fmt.Errorf("%w: roller returned %d dice for a pair", ErrBadAttack, len(rolls))
	}

	return pick(rolls[0], rolls[1]), nil
}

func takeHigher(a, b int) int {
	if a > b {
		return a
	}

	return b
}

func takeLower(a, b int) int {
	if a < b {
		return a
	}

	return b
}

// gatherAttack builds the step that folds the attack chain.
//
// A genuine fold, and the phase where prone's range predicate fires: it reads
// both combatants' positions out of the room this interaction installed, and
// answers advantage from within five feet, disadvantage from beyond.
func gatherAttack(
	event dnd5eEvents.AttackChainEvent,
	next func(context.Context, dnd5eEvents.AttackChainEvent) (Step, error),
) Gather {
	return Gather{
		name: "attack chain",
		run: func(ctx context.Context, bus events.EventBus) (Step, error) {
			chain := events.NewStagedChain[dnd5eEvents.AttackChainEvent](combat.ModifierStages)

			modified, err := dnd5eEvents.AttackChain.On(bus).PublishWithChain(ctx, event, chain)
			if err != nil {
				return nil, fmt.Errorf("publish attack chain: %w", err)
			}

			folded, err := modified.Execute(ctx, event)
			if err != nil {
				return nil, fmt.Errorf("execute attack chain: %w", err)
			}

			return next(ctx, folded)
		},
	}
}

// foldDamage builds the step that folds the damage chain.
//
// **Divestment debt — #965 slice 2.** This hands resolution's own bus to
// combat.ResolveDamage, which folds the chain itself. Functionally it is the
// save precedent — the fold happens on the interaction's bus either way, since
// resolution attached every subscriber to it — and what differs is custody of
// the fold mechanics. There is no bus-free alternative to call: every exported
// attack and damage entry point in combat requires a bus, and the arithmetic
// that applies resistance and vulnerability is unexported, so folding here
// would mean reimplementing damage multipliers. Slice 2 retires this.
func foldDamage(
	in *combat.ResolveDamageInput,
	next func(context.Context, *combat.ResolveDamageOutput) (Step, error),
) Gather {
	return Gather{
		name: "damage chain",
		run: func(ctx context.Context, bus events.EventBus) (Step, error) {
			in.EventBus = bus

			resolved, err := combat.ResolveDamage(ctx, in)
			if err != nil {
				return nil, fmt.Errorf("resolve damage: %w", err)
			}

			return next(ctx, resolved)
		},
	}
}

// flattenDice collapses a pool's grouped rolls into one per-die list, in roll
// order. Per-die rather than summed because OriginalDiceRolls/FinalDiceRolls
// are a per-die contract — downstream rerolls address dice by index.
func flattenDice(groups [][]int) []int {
	var dice []int
	for _, group := range groups {
		dice = append(dice, group...)
	}

	return dice
}

// requestContest builds the Request step for a contested rider, typed so the
// requester resumes with a ContestOutcome.
func requestContest(in *ContestInput, next func(context.Context, ContestOutcome) (Step, error)) Request {
	return Request{
		name:    "contest",
		machine: NewContest(in),
		next: func(ctx context.Context, out Outcome) (Step, error) {
			contest, ok := out.(ContestOutcome)
			if !ok {
				return nil, fmt.Errorf("%w: contest produced %T", ErrBadStep, out)
			}

			return next(ctx, contest)
		},
	}
}
