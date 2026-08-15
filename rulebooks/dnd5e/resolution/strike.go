// Copyright (C) 2026 Kirk Diggler
// SPDX-License-Identifier: GPL-3.0-or-later

package resolution

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/KirkDiggler/rpg-toolkit/core"
	"github.com/KirkDiggler/rpg-toolkit/dice"
	"github.com/KirkDiggler/rpg-toolkit/events"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/abilities"
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
// The attack arrives as an [AttackProfile] — attacker-kind-neutral, compiled
// at the seam from whatever persisted form the attacker has. The headline
// tests still drive the catalog wolf's own bite: content in, one compilation
// step, machine unchanged.
type StrikeInput struct {
	// AttackerID names the participant swinging.
	AttackerID string

	// TargetID names the participant being swung at.
	TargetID string

	// Attack is the attack's numbers, attacker-kind-neutral. The machine
	// never learns who compiled them: a monster action converts through
	// AttackFromMonsterAction, and a character's weapon-plus-sheet compiles
	// through a sibling constructor when it arrives (rpg-toolkit#1003). The
	// phases are the same swing either way — only the compilation differs.
	Attack AttackProfile

	// Roller rolls the attack and its damage. Nil takes the default roller.
	Roller dice.Roller
}

// AttackProfile is what a strike needs to know about an attack, whoever is
// making it. It is derived at strike time, never persisted: the persisted
// forms are the monster's action data and the character's sheet, and each
// compiles into this shape at the seam.
type AttackProfile struct {
	// Ref names the attack for attribution — the weapon or action behind
	// the swing.
	Ref *core.Ref

	// AttackBonus is added to the d20.
	AttackBonus int

	// DamageDice is the weapon pool's notation ("2d4+2").
	DamageDice string

	// DamageType is what kind of harm lands.
	DamageType damage.Type

	// AbilityUsed is the ability the compiler swung with, when it chose one.
	//
	// It carries no arithmetic — the modifier is already inside DamageDice —
	// and exists because effects predicate on it. Rage's damage bonus applies
	// only to melee attacks made with Strength, so it reads this off the
	// folded damage event; with the field empty its predicate never matches
	// and a raging character silently loses the bonus. Measured, not assumed:
	// a raging hero's longsword dealt 8 instead of 10 before this was
	// plumbed (TestARagingHerosBonusArrivesViaTheChainNotTheCompiler).
	//
	// The empty value is meaningful and correct for a stat block: a monster
	// action's numbers are pre-computed and name no ability, so
	// AttackFromMonsterAction leaves it unset rather than guessing STR.
	//
	// This is the field the attack-profile seam named as additive when a
	// predicate finally needed it (docs/ideas/session-sdk/attack-profile-seam.md).
	AbilityUsed abilities.Ability

	// Gate is the rider's contest, if the attack declares one (ADR-0039).
	// Nil means the attack just hits.
	Gate *saves.SaveGate

	// Imposes is what the rider does to a target who fails the gate's save.
	//
	// Gate and Imposes are the two halves of one rider and travel together:
	// the gate is the contest — which abilities, what DC, what a success buys —
	// and this is the consequence the contest is about. ADR-0039 deliberately
	// kept the two apart in the gate's own shape, which left the consequence
	// with nowhere to live: the machine hardcoded prone, so every gated attack
	// knocked its target down whatever the attack was (rpg-toolkit#1013).
	//
	// Nil whenever Gate is nil, and required whenever Gate is not — a contest
	// that gates nothing is not a rule, it is a roll with no meaning.
	//
	// The compiler names it, because translating an action's semantics is the
	// compiler's job: the wolf's KnockdownDC becomes both the gate and prone in
	// the same place, rather than the DC here and the meaning in the machine.
	Imposes Consequence
}

// validate refuses a profile that cannot drive a strike, naming what is
// missing. Constructors produce valid profiles; this catches the hand-built.
func (p *AttackProfile) validate() error {
	if p.Ref == nil {
		return fmt.Errorf("%w: the attack names no ref", ErrBadAttack)
	}
	if p.DamageDice == "" {
		return fmt.Errorf("%w: the attack declares no damage dice", ErrBadAttack)
	}

	// A gate with nothing riding on it is refused rather than run. The
	// alternative — contest it and impose nothing — is a save the target rolls,
	// can fail, and suffers nothing for, which reads as working while meaning
	// nothing. Same principle the consequence's own validate states: failing at
	// construction is a bug report, failing soft is a bug that ships.
	if p.Gate != nil && p.Imposes == nil {
		return fmt.Errorf("%w: the attack declares a gate but names no consequence", ErrBadAttack)
	}

	// And the other direction, because the pair is a pair. A consequence with
	// no gate is never imposed — afterNotify returns early on a nil gate — so
	// accepting it would silently discard a rule its author believed they had
	// written. Refusing both directions is what makes "Gate and Imposes travel
	// together" a contract rather than a comment.
	if p.Gate == nil && p.Imposes != nil {
		return fmt.Errorf("%w: the attack names a consequence but declares no gate to contest it", ErrBadAttack)
	}

	return nil
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

	// outcome accumulates across phases. It is the machine's whole state, and
	// the reason a suspension between any two phases would need nothing else.
	outcome StrikeOutcome
}

// Start validates the profile, reads the target's AC, and folds the attack chain.
func (m *strikeMachine) Start(_ context.Context, cast *Participants) (Step, error) {
	if m.in == nil {
		return nil, ErrNilInput
	}
	if m.in.AttackerID == "" || m.in.TargetID == "" {
		return nil, fmt.Errorf("%w: a strike needs an attacker and a target", ErrNilInput)
	}
	if err := m.in.Attack.validate(); err != nil {
		return nil, err
	}

	m.cast = cast

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
		WeaponRef:         m.in.Attack.Ref,
		IsMelee:           true,
		AttackBonus:       m.in.Attack.AttackBonus,
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
	pool, err := dice.ParseNotation(m.in.Attack.DamageDice)
	if err != nil {
		return nil, fmt.Errorf("%w: damage dice %q: %w", ErrBadAttack, m.in.Attack.DamageDice, err)
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
		SourceRef:         m.in.Attack.Ref,
		OriginalDiceRolls: rolls,
		FinalDiceRolls:    append([]int(nil), rolls...),
		FlatBonus:         result.Modifier(),
		DamageType:        m.in.Attack.DamageType,
		IsCritical:        m.outcome.Critical,
	}

	return foldDamage(&combat.ResolveDamageInput{
		AttackerID:   m.in.AttackerID,
		TargetID:     m.in.TargetID,
		Components:   []dnd5eEvents.DamageComponent{component},
		IsCritical:   m.outcome.Critical,
		HasAdvantage: len(m.outcome.Folded.AdvantageSources) > 0,
		WeaponDamage: m.in.Attack.DamageDice,
		IsMelee:      true,
		// Which ability swung, for the effects that predicate on it — Rage
		// only pays out on a melee Strength attack. Empty when the compiler
		// named none, which is a stat block's honest answer.
		AbilityUsed: m.in.Attack.AbilityUsed,
		WeaponRef:   m.in.Attack.Ref,
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
	if m.in.Attack.Gate == nil {
		return Done{Outcome: m.outcome}, nil
	}

	return requestContest(&ContestInput{
		Gate:    m.in.Attack.Gate,
		SaverID: m.in.TargetID,
		// What the profile named, not what this machine assumes. Until
		// rpg-toolkit#1013 this was a hardcoded prone, so a ghoul's paralysis
		// would have knocked its target over instead — the machine deciding a
		// rule that belongs to the attack.
		Consequence: m.in.Attack.Imposes,
		DamageTaken: m.outcome.Damage,
		Roller:      m.in.Roller,
	}, func(_ context.Context, contest ContestOutcome) (Step, error) {
		m.outcome.Contest = &contest

		return Done{Outcome: m.outcome}, nil
	}), nil
}

// AttackFromMonsterAction compiles a monster action's persisted data into the
// neutral profile a strike consumes. This is the monster half of the seam
// StrikeInput.Attack names; a character's weapon-plus-sheet compiler is its
// sibling (rpg-toolkit#1003) and the machine cannot tell them apart.
//
// Two refs for slice 1: the bite, and the generic melee action every
// stat-block weapon is authored as (a skeleton's scimitar is a MeleeAction
// named "scimitar"). An action this build cannot read is refused by name
// rather than treated as a generic swing, because guessing an attack bonus is
// how a stat block starts lying again — the ranged action waits on range
// semantics the strike does not have, and multiattack is turn economy, not a
// single swing.
//
// Reach is not enforced, for melee weapons exactly as for the bite: the
// strike does not yet check adjacency at all. That is one shared, named gap
// (#965 slice 2's list), not one this case adds.
func AttackFromMonsterAction(action monster.ActionData) (AttackProfile, error) {
	switch action.Ref.ID {
	case refs.MonsterActions.Bite().ID:
		return attackFromBite(action)
	case refs.MonsterActions.Melee().ID:
		return attackFromMelee(action)
	default:
		return AttackProfile{}, fmt.Errorf(
			"%w: %q (the strike compiles the bite and the generic melee action)", ErrBadAttack, action.Ref.ID)
	}
}

// attackFromMelee compiles a stat-block weapon — MeleeConfig is the profile's
// shape already, and a plain weapon declares no gate: the blow just lands.
func attackFromMelee(action monster.ActionData) (AttackProfile, error) {
	var config monsterActions.MeleeConfig
	if len(action.Config) > 0 {
		if err := json.Unmarshal(action.Config, &config); err != nil {
			return AttackProfile{}, fmt.Errorf("%w: %w", ErrBadAttack, err)
		}
	}

	if config.DamageDice == "" {
		return AttackProfile{}, fmt.Errorf("%w: the action declares no damage dice", ErrBadAttack)
	}

	ref := action.Ref

	return AttackProfile{
		Ref:         &ref,
		AttackBonus: config.AttackBonus,
		DamageDice:  config.DamageDice,
		DamageType:  config.DamageType,
	}, nil
}

func attackFromBite(action monster.ActionData) (AttackProfile, error) {
	var config monsterActions.BiteConfig
	if len(action.Config) > 0 {
		if err := json.Unmarshal(action.Config, &config); err != nil {
			return AttackProfile{}, fmt.Errorf("%w: %w", ErrBadAttack, err)
		}
	}

	if config.DamageDice == "" {
		return AttackProfile{}, fmt.Errorf("%w: the action declares no damage dice", ErrBadAttack)
	}

	ref := action.Ref
	profile := AttackProfile{
		Ref:         &ref,
		AttackBonus: config.AttackBonus,
		DamageDice:  config.DamageDice,
		DamageType:  config.DamageType,
		Gate:        config.SaveGate,
	}

	// A bite persisted by an older build carries a bare knockdown DC rather
	// than a gate; NewBiteAction is what translates it, so the profile comes
	// through the action rather than the config when the gate is absent.
	if profile.Gate == nil {
		if bite, ok := mustLoadBite(action); ok {
			profile.Gate = bite.SaveGate()
		}
	}

	// A bite's gate is a KNOCKDOWN — that is what the stat block's
	// KnockdownDC means, and translating it is this function's job in both
	// directions: the DC becomes the contest, and knocked-down becomes prone.
	// Naming it here rather than in the machine is the whole of #1013: the
	// machine imposed prone on every gated attack because the consequence had
	// nowhere else to be said.
	//
	// Conditional on the gate, so a bite authored without one stays a plain
	// bite rather than carrying a consequence nothing can trigger.
	if profile.Gate != nil {
		profile.Imposes = ImposeCondition(refs.Conditions.Prone(), dnd5eEvents.ConditionProne)
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
