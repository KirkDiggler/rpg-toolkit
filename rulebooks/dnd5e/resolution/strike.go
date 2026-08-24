// Copyright (C) 2026 Kirk Diggler
// SPDX-License-Identifier: GPL-3.0-or-later

package resolution

import (
	"context"
	"fmt"

	"github.com/KirkDiggler/rpg-toolkit/core"
	"github.com/KirkDiggler/rpg-toolkit/dice"
	"github.com/KirkDiggler/rpg-toolkit/events"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/abilities"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/combat"
	combatActions "github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/combat/actions"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/damage"
	dnd5eEvents "github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/events"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/refs"
)

// criticalThreshold is the roll at or above which an attack crits. A flat 20
// here rather than a field: nothing in slice 1 lowers it, and the attack chain
// carries the number so an effect that does will be read from the folded event
// rather than from this constant.
const criticalThreshold = 20

// StrikeInput describes one attack over an inert shared definition.
type StrikeInput struct {
	AttackerID string
	TargetID   string
	Definition combatActions.Definition
	Roller     dice.Roller
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

	// DamageInstances are the typed amounts that landed after the damage fold
	// and multiplier arithmetic. Empty on a miss.
	DamageInstances []damage.Instance

	// DamageComponents are the folded source-attributed components from which
	// DamageInstances were calculated. Empty on a miss.
	DamageComponents []dnd5eEvents.DamageComponent

	// Conditions records each declared on-hit application in declaration order.
	Conditions []ConditionOutcome
}

// ConditionOutcome records whether one declared on-hit condition landed.
type ConditionOutcome struct {
	Ref     core.Ref        `json:"ref"`
	Contest *ContestOutcome `json:"contest,omitempty"`
	Applied bool            `json:"applied"`
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
	return &strikeMachine{
		in: in,
		applyDamage: func(
			ctx context.Context, target combat.Combatant, input *combat.ApplyDamageInput,
		) *combat.ApplyDamageResult {
			return target.ApplyDamage(ctx, input)
		},
	}
}

type strikeMachine struct {
	in *StrikeInput

	// applyDamage is the one target-application boundary. Keeping it as a
	// dependency lets tests count calls while delegating to the real combatant;
	// NewStrike always installs the direct production behavior.
	applyDamage func(context.Context, combat.Combatant, *combat.ApplyDamageInput) *combat.ApplyDamageResult

	// cast is the sheets this interaction attached, kept because a phase after
	// the first needs them and a step's closure is handed only a bus.
	cast *Participants

	attack           *combatActions.AttackProfile
	sourceRef        *core.Ref
	ability          abilities.Ability
	abilityModifier  int
	twoHanded        bool
	offHandWeaponRef *core.Ref
	prepared         []preparedCondition

	// outcome accumulates across phases. It is the machine's whole state, and
	// the reason a suspension between any two phases would need nothing else.
	outcome StrikeOutcome
}

// Start validates the profile and produces the first post-payment resolution step.
func (m *strikeMachine) Start(ctx context.Context, cast *Participants) (Step, error) {
	if m.in == nil {
		return nil, ErrNilInput
	}
	if m.in.AttackerID == "" || m.in.TargetID == "" {
		return nil, fmt.Errorf("%w: a strike needs an attacker and a target", ErrNilInput)
	}
	if err := m.in.Definition.Validate(); err != nil {
		return nil, fmt.Errorf("%w: %w", ErrBadAction, err)
	}

	m.cast = cast
	m.attack = m.in.Definition.Attack
	m.sourceRef = &m.in.Definition.Ref
	if m.attack.Weapon != nil {
		m.twoHanded = m.attack.Weapon.TwoHanded
		m.offHandWeaponRef = m.attack.Weapon.OffHandWeaponRef
		if m.attack.Weapon.Ref != nil {
			m.sourceRef = m.attack.Weapon.Ref
		}
	}
	if m.attack.Ability != nil {
		m.ability = m.attack.Ability.Ability
		m.abilityModifier = m.attack.Ability.Modifier
	}

	if _, err := combatantFor(cast, m.in.AttackerID); err != nil {
		return nil, err
	}

	target, err := combatantFor(cast, m.in.TargetID)
	if err != nil {
		return nil, err
	}

	longRange, err := deliveryRangeState(ctx, m.in.AttackerID, m.in.TargetID, m.attack.Delivery)
	if err != nil {
		return nil, err
	}
	m.prepared = make([]preparedCondition, len(m.attack.OnHit))
	for index, application := range m.attack.OnHit {
		if application.Save != nil {
			if gateErr := validateConditionGate(application.Save); gateErr != nil {
				return nil, fmt.Errorf("validate on-hit condition %s: %w", application.Ref.String(), gateErr)
			}
		}
		prepared, prepareErr := prepareCondition(application, m.in.TargetID, m.in.Definition.Ref.String())
		if prepareErr != nil {
			return nil, prepareErr
		}
		m.prepared[index] = prepared
	}

	return m.effectiveACStep(target, longRange), nil
}

// effectiveACStep performs the first event-backed work only after the door has
// accepted payment, then builds the attack event from that folded AC.
func (m *strikeMachine) effectiveACStep(target combat.Combatant, longRange bool) Gather {
	return Gather{
		name: "effective AC",
		run: func(ctx context.Context, _ events.EventBus) (Step, error) {
			// The AC the target actually has — armor worn and every effect folded onto
			// the AC chain — rather than the flat number the sheet happens to carry.
			//
			// combat.GetEffectiveAC is bus-free at its signature and dispatches on the
			// sheet: a character folds combat.ACChain, a monster has no such method and
			// falls through to its stat block's AC, which is correct for a stat block.
			//
			// The character's fold rides the bus parked on the sheet, and under Resolve
			// that bus IS this interaction's participant view — Attach was handed it —
			// so the fold runs among the same subscribers everything else in this strike
			// folds among. Running that read inside this first Gather is load-bearing:
			// Start is pure preflight, and an action the door refuses publishes nothing.
			//
			// THE CHAIN EVENT'S COPY IS THE ONE THAT MATTERS. afterAttackChain compares
			// against the FOLDED event and then assigns m.outcome.TargetAC from it, so
			// the event's number both decides the hit and replaces whatever the outcome
			// was built with. Setting only the outcome would report the effective AC
			// and roll against the flat one — a strike that tells the truth and does
			// something else. Pinned by TestTheFoldedACDecidesTheHitNotJustTheReport.
			effectiveAC := combat.GetEffectiveAC(ctx, target)

			m.outcome = StrikeOutcome{
				AttackerID: m.in.AttackerID,
				TargetID:   m.in.TargetID,
				TargetAC:   effectiveAC,
			}

			event := dnd5eEvents.AttackChainEvent{
				AttackerID:        m.in.AttackerID,
				TargetID:          m.in.TargetID,
				WeaponRef:         m.sourceRef,
				IsMelee:           m.attack.Delivery.IsMelee(),
				AttackBonus:       m.attack.AttackBonus,
				TargetAC:          effectiveAC,
				CriticalThreshold: criticalThreshold,
			}
			if longRange {
				event.DisadvantageSources = append(event.DisadvantageSources, dnd5eEvents.AttackModifierSource{
					SourceRef: &m.in.Definition.Ref,
					SourceID:  m.in.AttackerID,
					Reason:    "target is beyond normal range",
				})
			}

			return gatherAttack(event, m.afterAttackChain), nil
		},
	}
}

// afterAttackChain rolls the die the fold decided the shape of, and decides
// whether the blow lands.
//
// The advantage rules are 5e's and are mirrored from combat.ResolveAttackHit
// rather than invented: advantage and disadvantage cancel exactly, a natural 1
// always misses, a natural 20 always hits. Rolling here rather than calling
// combat is not a preference — every exported attack entry point in that
// package requires an event bus, and there is no bus-free roll to call.
//
// Slice 2 left it that way on purpose. Exporting a bus-free attack roll would
// be a second divestment with its own evidence to produce, and the old attack
// path is the one that would have to give it up; it retires with #966, which
// is when this stops being a mirror and starts being the only copy.
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

	// PostAttackRollChain is the reaction boundary for both hits and misses.
	// Shield and rage-attempt subscribers need the actual d20 result before
	// phase two can decide damage, so publish it before the miss short-circuit.
	postRoll := &dnd5eEvents.PostAttackRollEvent{
		AttackerID:          m.outcome.AttackerID,
		TargetID:            m.outcome.TargetID,
		OriginalAC:          folded.TargetAC,
		AttackRoll:          roll,
		AttackBonus:         folded.AttackBonus,
		TotalAttack:         m.outcome.Total,
		WouldHit:            m.outcome.Hit,
		IsNaturalTwenty:     roll == 20,
		IsNaturalOne:        roll == 1,
		HasAdvantage:        hasAdvantage,
		HasDisadvantage:     hasDisadvantage,
		AdvantageSources:    attackModifierRefs(folded.AdvantageSources),
		DisadvantageSources: attackModifierRefs(folded.DisadvantageSources),
	}

	return publishPostAttackRoll(postRoll, func(nextCtx context.Context) (Step, error) {
		if !m.outcome.Hit {
			// A miss ends the strike here: no damage, and no save. The rider the
			// action declares is gated on the blow landing, so a bite that misses
			// rolls no save (rpg-toolkit#962's residual).
			return Done{Outcome: m.outcome}, nil
		}

		return m.rollDamage(nextCtx, roller)
	}), nil
}

// attackModifierRefs projects the richer attack-chain source records onto the
// post-roll snapshot's established reference-only narration contract. The
// strike fold owns source reasons/IDs; the older post-roll event intentionally
// exposes only refs, so no source metadata is duplicated across that boundary.
func attackModifierRefs(sources []dnd5eEvents.AttackModifierSource) []*core.Ref {
	refs := make([]*core.Ref, 0, len(sources))
	for _, source := range sources {
		refs = append(refs, source.SourceRef)
	}
	return refs
}

// rollDamage rolls the action's damage dice and yields the fold that lets
// effects modify it (ADR-0026's Resolve).
func (m *strikeMachine) rollDamage(ctx context.Context, roller dice.Roller) (Step, error) {
	if len(m.attack.Damage) == 0 {
		return m.afterDamage(ctx)
	}
	components := make([]dnd5eEvents.DamageComponent, 0, len(m.attack.Damage)+1)
	var primary *damage.Damage
	for i := range m.attack.Damage {
		declared := &m.attack.Damage[i]
		component, err := m.rollDamageComponent(ctx, roller, *declared)
		if err != nil {
			return nil, err
		}
		components = append(components, component)

		if declared.HasProperty(damage.AddsAttackAbilityModifier) {
			primary = declared
		}
	}

	if primary != nil {
		components = append(components, dnd5eEvents.DamageComponent{
			Source:     dnd5eEvents.DamageSourceAbility,
			SourceRef:  attackAbilityRef(m.ability),
			FlatBonus:  m.abilityModifier,
			DamageType: primary.Type,
			IsCritical: false,
		})
	}

	effectiveAdvantage := len(m.outcome.Folded.AdvantageSources) > 0 &&
		len(m.outcome.Folded.DisadvantageSources) == 0
	var weaponDamageDice string
	var weaponDamageType damage.Type
	if primary != nil {
		weaponDamageDice = primary.Dice
		weaponDamageType = primary.Type
	}

	return foldDamage(dnd5eEvents.NewDamageChainEvent(dnd5eEvents.DamageChainInput{
		AttackerID:       m.in.AttackerID,
		TargetID:         m.in.TargetID,
		Components:       components,
		IsCritical:       m.outcome.Critical,
		HasAdvantage:     effectiveAdvantage,
		WeaponDamageDice: weaponDamageDice,
		WeaponDamageType: weaponDamageType,
		IsMelee:          m.attack.Delivery.IsMelee(),
		// Which ability swung, for the effects that predicate on it — Rage
		// only pays out on a melee Strength attack. Empty when the compiler
		// named none, which is a stat block's honest answer.
		AbilityUsed:     m.ability,
		AbilityModifier: m.abilityModifier,
		WeaponRef:       m.sourceRef,
		// Static equipment facts the compiler already knew (rpg-toolkit#1178)
		// — Dueling's predicate decides eligibility from these rather than a
		// live gamectx lookup, the same way Rage decides from AbilityUsed.
		TwoHanded:        m.twoHanded,
		OffHandWeaponRef: m.offHandWeaponRef,
	}), m.afterDamageChain), nil
}

func (m *strikeMachine) rollDamageComponent(
	ctx context.Context, roller dice.Roller, declared damage.Damage,
) (dnd5eEvents.DamageComponent, error) {
	pool, err := dice.ParseNotation(declared.Dice)
	if err != nil {
		return dnd5eEvents.DamageComponent{}, fmt.Errorf(
			"%w: damage dice %q: %w", ErrBadAttack, declared.Dice, err)
	}

	result := pool.RollContext(ctx, roller)
	if result.Error() != nil {
		return dnd5eEvents.DamageComponent{}, fmt.Errorf("roll damage: %w", result.Error())
	}

	// Per-die, not summed: downstream rerolls address dice by index. Intrinsic
	// arithmetic stays in FlatBonus and is therefore never part of a critical
	// hit's second roll.
	rolls := flattenDice(result.Rolls())
	isCritical := m.outcome.Critical && !declared.HasProperty(damage.DoesNotCrit)
	if isCritical {
		again := pool.RollContext(ctx, roller)
		if again.Error() != nil {
			return dnd5eEvents.DamageComponent{}, fmt.Errorf("roll critical damage: %w", again.Error())
		}
		rolls = append(rolls, flattenDice(again.Rolls())...)
	}

	return dnd5eEvents.DamageComponent{
		Source:            dnd5eEvents.DamageSourceWeapon,
		SourceRef:         m.sourceRef,
		Dice:              declared.Dice,
		OriginalDiceRolls: rolls,
		FinalDiceRolls:    append([]int(nil), rolls...),
		FlatBonus:         declared.FlatBonus,
		DamageType:        declared.Type,
		Properties:        append([]damage.Property(nil), declared.Properties...),
		IsCritical:        isCritical,
	}, nil
}

func attackAbilityRef(ability abilities.Ability) *core.Ref {
	switch ability {
	case abilities.STR:
		return refs.Abilities.Strength()
	case abilities.DEX:
		return refs.Abilities.Dexterity()
	case abilities.CON:
		return refs.Abilities.Constitution()
	case abilities.INT:
		return refs.Abilities.Intelligence()
	case abilities.WIS:
		return refs.Abilities.Wisdom()
	case abilities.CHA:
		return refs.Abilities.Charisma()
	default:
		return nil
	}
}

// afterDamageChain applies what the fold settled on — bus-free, straight onto
// the sheet (ADR-0026's Apply). Notify is deliberately absent: publishing
// DamageReceivedEvent would apply the damage a second time to a monster
// target, whose sheet-keeper treats that topic as an instruction — the
// one-topic-two-meanings finding #965 slice 2 owes a classification for.
// Pinned by TestAMonsterTargetTakesItsDamageOnce.
func (m *strikeMachine) afterDamageChain(
	ctx context.Context, folded *dnd5eEvents.DamageChainEvent,
) (Step, error) {
	target, err := combatantFor(m.cast, m.in.TargetID)
	if err != nil {
		return nil, err
	}

	// The multipliers are combat's arithmetic, called bus-free now that it is
	// exported (#965 slice 2 PR-A). Resistance, vulnerability, and immunity
	// stacking stays one implementation shared with the legacy stack rather
	// than a copy that can drift.
	final, _ := combat.FinalDamage(folded.Components)

	m.outcome.DamageInstances = make([]damage.Instance, 0, len(final))
	instances := make([]combat.DamageInstance, 0, len(final))
	for _, instance := range final {
		m.outcome.DamageInstances = append(m.outcome.DamageInstances, damage.Instance{
			Amount: instance.Amount,
			Type:   instance.Type,
		})
		instances = append(instances, combat.DamageInstance{
			Amount: instance.Amount,
			Type:   string(instance.Type),
		})
	}
	m.outcome.DamageComponents = append([]dnd5eEvents.DamageComponent(nil), folded.Components...)

	// Bus-free, and the only phase that is: applying damage is the sheet's own
	// business and takes no bus on either a character or a monster.
	applied := m.applyDamage(ctx, target, &combat.ApplyDamageInput{
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
// Slice 2's census classified it, and the answer was sharper than the question:
// DamageReceivedTopic has FIVE subscribers across THREE meanings. One applies
// damage (monster.onDamageReceived calls TakeDamage — the instruction). Three
// are genuine rules that only observe: Undead Fortitude's survival save,
// Unconscious's death-save failures, and Rage's was-I-hit upkeep. One is the
// encounter layer, which captures the events and DRAINS them precisely to stop
// the damage landing twice. A character subscribes for none of it, so the topic
// is inert for half the roster.
//
// The topic MEANS "damage has landed" — a notification. The subscriber that
// treats it as an instruction is the defect, not the topic, and it retires with
// #977 when Undead Fortitude converts to a gate. Publishing here before then
// would double-apply to every monster; publishing after costs nothing and buys
// the three rules back. So this waits on that conversion, not on a question.
//
// What it costs meanwhile is named rather than hidden: those three rules do not
// fire for resolution-driven damage yet.
func (m *strikeMachine) afterDamage(ctx context.Context) (Step, error) {
	return m.afterNotify(ctx)
}

// afterNotify processes declared on-hit conditions in authoring order.
func (m *strikeMachine) afterNotify(_ context.Context) (Step, error) {
	return m.nextCondition(0)
}

func (m *strikeMachine) nextCondition(index int) (Step, error) {
	if index >= len(m.prepared) {
		return Done{Outcome: m.outcome}, nil
	}
	prepared := m.prepared[index]
	application := prepared.declaration
	if application.Save == nil {
		return publishPreparedCondition(prepared, m.cast, m.in.TargetID, func() (Step, error) {
			m.outcome.Conditions = append(m.outcome.Conditions, ConditionOutcome{
				Ref: application.Ref, Applied: true,
			})
			return m.nextCondition(index + 1)
		}), nil
	}

	return requestContest(&ContestInput{
		Gate:        application.Save,
		SaverID:     m.in.TargetID,
		Application: application,
		DamageTaken: m.outcome.Damage,
		Roller:      m.in.Roller,
		prepared:    &prepared,
	}, func(_ context.Context, contest ContestOutcome) (Step, error) {
		m.outcome.Conditions = append(m.outcome.Conditions, ConditionOutcome{
			Ref: application.Ref, Contest: &contest, Applied: !contest.Succeeded,
		})
		return m.nextCondition(index + 1)
	}), nil
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

// publishPostAttackRoll runs the post-roll reaction chain on the interaction's
// bus. It deliberately executes for misses as well as hits: subscribers such
// as Rage record an attack attempt, while Shield simply ignores misses.
func publishPostAttackRoll(
	event *dnd5eEvents.PostAttackRollEvent,
	next func(context.Context) (Step, error),
) Gather {
	return Gather{
		name: "post attack roll",
		run: func(ctx context.Context, bus events.EventBus) (Step, error) {
			chain := events.NewStagedChain[*dnd5eEvents.PostAttackRollEvent](combat.ModifierStages)
			modified, err := dnd5eEvents.PostAttackRollChain.On(bus).PublishWithChain(ctx, event, chain)
			if err != nil {
				return nil, fmt.Errorf("publish post attack roll: %w", err)
			}
			if _, err := modified.Execute(ctx, event); err != nil {
				return nil, fmt.Errorf("execute post attack roll: %w", err)
			}
			return next(ctx)
		},
	}
}

// foldDamage builds the step that folds the damage chain on this
// interaction's own bus.
//
// Resolution publishes and executes the chain itself, the same way it already
// does for the attack and the saving throw, and then calls the exported
// [combat.FinalDamage] for the multiplier arithmetic. Slice 1 handed its bus
// to combat.ResolveDamage instead — the fold happened on the right bus either
// way, since resolution attached every subscriber to it, but custody of the
// fold sat in the other module and there was no bus-free arithmetic to call.
// PR-A exported that arithmetic; this is the half that uses it.
//
// What is deliberately NOT reimplemented: the stacking rules. FinalDamage is
// shared with the legacy stack, so resistance and immunity cannot drift
// between the two.
func foldDamage(
	event *dnd5eEvents.DamageChainEvent,
	next func(context.Context, *dnd5eEvents.DamageChainEvent) (Step, error),
) Gather {
	return Gather{
		name: "damage chain",
		run: func(ctx context.Context, bus events.EventBus) (Step, error) {
			chain := events.NewStagedChain[*dnd5eEvents.DamageChainEvent](combat.ModifierStages)

			modified, err := dnd5eEvents.DamageChain.On(bus).PublishWithChain(ctx, event, chain)
			if err != nil {
				return nil, fmt.Errorf("publish damage chain: %w", err)
			}

			folded, err := modified.Execute(ctx, event)
			if err != nil {
				return nil, fmt.Errorf("execute damage chain: %w", err)
			}

			return next(ctx, folded)
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
