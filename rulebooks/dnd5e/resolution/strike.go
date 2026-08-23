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

	// Name is the catalog's display name for Ref — "Longsword", "Unarmed
	// Strike" — carried alongside it so a caller reporting what was swung
	// (a beat line, a struck/missed event) never has to look the ref back
	// up in a catalog of its own (rpg-toolkit#866). Populated by
	// [AttackFromCharacter]; empty from [AttackFromMonsterAction] today —
	// monster attackers do not yet reach a caller that reports this
	// (v1 compiles CHARACTER attackers only, session.Attack's own doc).
	Name string

	// AttackBonus is added to the d20.
	AttackBonus int

	// Reach is how far this melee attack reaches, in FEET: 5 (the 5e
	// default, and the unarmed strike's own reach) or 10 for a weapon
	// carrying the Reach property (rpg-toolkit#1010). Feet, not cells
	// (Kirk, rpg-project#254 review) — the same unit monster.ActionData's
	// own Reach field and monster.SpeedData already use; a cell is 5 feet,
	// and the one place that needs cells (the reach gate, the monster
	// turn's movement budget) converts once, at the point of comparison,
	// rather than every producer of this field guessing at the conversion.
	//
	// Populated by both compilers now — [AttackFromCharacter] via
	// [defaultMeleeReach]/[reachPropertyReach], [AttackFromMonsterAction]
	// via the action's own authored Reach (rpg-project#254; previously
	// silently zero for every monster action, since nothing read it).
	// Meaningless for a ranged attack, which never compiles a profile at
	// all (IsRanged refuses before one is built).
	Reach int

	// Damage is the ordered set of canonical damage pools declared by the
	// weapon or monster action. Dice notation remains pure NdM; intrinsic
	// arithmetic stays in each pool's FlatBonus.
	Damage []damage.Damage

	// AbilityUsed is the ability the compiler swung with, when it chose one.
	//
	// Effects predicate on this field. Rage's damage bonus applies
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

	// AbilityModifier is the arithmetic selected by a character compiler. It
	// stays separate from canonical dice and intrinsic monster flat bonuses so
	// resolution can attribute it to the one marked pool. Monster profiles keep
	// this at zero.
	AbilityModifier int

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
	// Nil whenever Gate is nil, and required whenever Gate is not. BOTH
	// DIRECTIONS ARE ENFORCED by validate, because a consequence with no
	// contest is as meaningless as a contest with no consequence: one is a
	// roll that costs nothing, the other is a cost nothing ever rolls for.
	// Enforcing only the first half is what let a stranded Imposes pass
	// validation and then be silently dropped by afterNotify's early exit
	// (Copilot review, #1014).
	//
	// The compiler names it, because translating an action's semantics is the
	// compiler's job: the wolf's KnockdownDC becomes both the gate and prone in
	// the same place, rather than the DC here and the meaning in the machine.
	Imposes Consequence

	// TwoHanded says this swing is being made with both hands on the
	// weapon. A static fact of the grip and the weapon, threaded onto the
	// damage chain event the same way AbilityUsed already is — this is the
	// seam's own precedent (see AbilityUsed's doc above), exercised for
	// rpg-toolkit#1178: Dueling's eligibility used to be decided by a live
	// gamectx.CharacterRegistry lookup the session stack never satisfies;
	// moving the static half of that decision here let Dueling's own
	// predicate read it off the event instead, the same way Rage does.
	TwoHanded bool

	// OffHandWeaponRef names the weapon, if any, occupying the attacker's
	// OTHER hand from the one this profile compiled — nil when that hand
	// is empty or holds something that is not a weapon (a shield, most
	// often). Also additive for rpg-toolkit#1178, and also a static
	// equipment fact rather than something a chain-fold predicate should
	// have to ask a live registry for.
	OffHandWeaponRef *core.Ref
}

// validate refuses a profile that cannot drive a strike, naming what is
// missing. Constructors produce valid profiles; this catches the hand-built.
func (p *AttackProfile) validate() error {
	if p.Ref == nil {
		return fmt.Errorf("%w: the attack names no ref", ErrBadAttack)
	}
	if err := damage.Validate(p.Damage); err != nil {
		return fmt.Errorf("%w: invalid attack damage: %w", ErrBadAttack, err)
	}

	abilityMarkers := 0
	for _, pool := range p.Damage {
		for _, property := range pool.Properties {
			if property == damage.AddsAttackAbilityModifier {
				abilityMarkers++
			}
		}
	}

	if p.AbilityUsed != "" {
		if abilityMarkers != 1 {
			return fmt.Errorf(
				"%w: ability %q requires exactly one %q damage pool",
				ErrBadAttack, p.AbilityUsed, damage.AddsAttackAbilityModifier)
		}
	} else {
		if p.AbilityModifier != 0 {
			return fmt.Errorf("%w: an attack with no ability cannot carry ability modifier %+d", ErrBadAttack, p.AbilityModifier)
		}
		if abilityMarkers != 0 {
			return fmt.Errorf(
				"%w: an attack with no ability cannot mark a %q damage pool",
				ErrBadAttack, damage.AddsAttackAbilityModifier)
		}
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

	// DamageInstances are the typed amounts that landed after the damage fold
	// and multiplier arithmetic. Empty on a miss.
	DamageInstances []damage.Instance

	// DamageComponents are the folded source-attributed components from which
	// DamageInstances were calculated. Empty on a miss.
	DamageComponents []dnd5eEvents.DamageComponent

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

	// outcome accumulates across phases. It is the machine's whole state, and
	// the reason a suspension between any two phases would need nothing else.
	outcome StrikeOutcome
}

// Start validates the profile, reads the target's AC, and folds the attack chain.
func (m *strikeMachine) Start(ctx context.Context, cast *Participants) (Step, error) {
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
	// folds among. That is the legacy shape and it is load-bearing here: a
	// resolution-owned step (an AC Gather before the attack event is built) is
	// the eventual form if AC folding ever needs to be inspectable in the step
	// log or suspendable at a reaction window. Documented rather than built —
	// nothing asks for it yet, and the fold reaches the right subscribers
	// either way.
	//
	// THE CHAIN EVENT'S COPY IS THE ONE THAT MATTERS. afterAttackChain compares
	// against the FOLDED event and then assigns m.outcome.TargetAC from it, so
	// the event's number both decides the hit and replaces whatever the outcome
	// was built with. Setting only the outcome would report the effective AC
	// and roll against the flat one — a strike that tells the truth and does
	// something else. Pinned by TestTheFoldedACDecidesTheHitNotJustTheReport.
	//
	// The outcome is seeded with it anyway, and that seed is dead on every
	// path that exists today: nothing returns a StrikeOutcome before the fold
	// runs. It stays because a future phase that can end the strike earlier —
	// a reaction window declining the attack, say — would otherwise hand back
	// an outcome with a zero AC, and that is a worse failure than a redundant
	// assignment.
	effectiveAC := combat.GetEffectiveAC(ctx, target)

	m.outcome = StrikeOutcome{
		AttackerID: m.in.AttackerID,
		TargetID:   m.in.TargetID,
		TargetAC:   effectiveAC,
	}

	event := dnd5eEvents.AttackChainEvent{
		AttackerID:        m.in.AttackerID,
		TargetID:          m.in.TargetID,
		WeaponRef:         m.in.Attack.Ref,
		IsMelee:           true,
		AttackBonus:       m.in.Attack.AttackBonus,
		TargetAC:          effectiveAC,
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
	components := make([]dnd5eEvents.DamageComponent, 0, len(m.in.Attack.Damage)+1)
	var primary *damage.Damage
	for i := range m.in.Attack.Damage {
		declared := &m.in.Attack.Damage[i]
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
			SourceRef:  attackAbilityRef(m.in.Attack.AbilityUsed),
			FlatBonus:  m.in.Attack.AbilityModifier,
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
		IsMelee:          true,
		// Which ability swung, for the effects that predicate on it — Rage
		// only pays out on a melee Strength attack. Empty when the compiler
		// named none, which is a stat block's honest answer.
		AbilityUsed:     m.in.Attack.AbilityUsed,
		AbilityModifier: m.in.Attack.AbilityModifier,
		WeaponRef:       m.in.Attack.Ref,
		// Static equipment facts the compiler already knew (rpg-toolkit#1178)
		// — Dueling's predicate decides eligibility from these rather than a
		// live gamectx lookup, the same way Rage decides from AbilityUsed.
		TwoHanded:        m.in.Attack.TwoHanded,
		OffHandWeaponRef: m.in.Attack.OffHandWeaponRef,
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
		SourceRef:         m.in.Attack.Ref,
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
// strike does not check adjacency at all. Deferred explicitly rather than
// carried silently — rpg-toolkit#1010 owns the rule inventory (five-foot
// reach, the reach property, ranged brackets) and it lands with the movement
// wave, where the MovementChain custody it belongs beside already sits.
// Positions exist and the room is built, so the check is cheap whenever
// someone owns the rule.
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
	if _, err := monsterActions.LoadAction(action); err != nil {
		return AttackProfile{}, fmt.Errorf("%w: invalid melee action: %w", ErrBadAttack, err)
	}

	var config monsterActions.MeleeConfig
	if err := json.Unmarshal(action.Config, &config); err != nil {
		return AttackProfile{}, fmt.Errorf("%w: %w", ErrBadAttack, err)
	}

	ref := action.Ref
	profile := AttackProfile{
		Ref: &ref,
		// The stat block's own authored display name — "shortsword",
		// "scimitar" — carried through exactly as AttackFromCharacter
		// already carries a weapon's (rpg-toolkit#866). Previously dropped
		// (AttackProfile.Name stayed "" for every monster melee action),
		// harmless only while nothing that compiled a monster's attack
		// ever reported it to a caller — session's Striker now does
		// (rpg-project#254).
		Name:        config.Name,
		AttackBonus: config.AttackBonus,
		Damage:      copyDamagePools(config.Damage),
		// The stat block's own authored reach, in cells — carried through
		// faithfully rather than clamped or defaulted here: this compiler
		// trusts the content the same way it already trusts AttackBonus and
		// Damage. Previously dropped entirely (AttackProfile.Reach stayed
		// zero for every monster melee action), which was harmless only
		// because nothing gated a monster's attack by reach yet.
		Reach: config.Reach,
	}
	if err := profile.validate(); err != nil {
		return AttackProfile{}, err
	}

	return profile, nil
}

func attackFromBite(action monster.ActionData) (AttackProfile, error) {
	if _, err := monsterActions.LoadAction(action); err != nil {
		return AttackProfile{}, fmt.Errorf("%w: invalid bite action: %w", ErrBadAttack, err)
	}

	var config monsterActions.BiteConfig
	if err := json.Unmarshal(action.Config, &config); err != nil {
		return AttackProfile{}, fmt.Errorf("%w: %w", ErrBadAttack, err)
	}

	ref := action.Ref
	profile := AttackProfile{
		Ref: &ref,
		// BiteConfig carries no authored display name — every bite is just
		// "the bite" — so this is a fixed label rather than read off
		// content, the same way Reach below is a fixed value rather than a
		// configured one. Kept non-empty for the same reason melee's Name
		// is carried at all: a caller reporting what was swung (a beat
		// line, a struck/missed event) reads this rather than a catalog.
		Name:        "Bite",
		AttackBonus: config.AttackBonus,
		Damage:      copyDamagePools(config.Damage),
		Gate:        config.SaveGate,
		// BiteConfig carries no configurable reach — a bite is always the
		// fixed melee reach, the same default a plain weapon or an unarmed
		// strike compiles to (character_attack.go's defaultMeleeReach).
		Reach: defaultMeleeReach,
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
	if err := profile.validate(); err != nil {
		return AttackProfile{}, err
	}

	return profile, nil
}

func copyDamagePools(pools []damage.Damage) []damage.Damage {
	copy := make([]damage.Damage, len(pools))
	for i, pool := range pools {
		copy[i] = pool
		copy[i].Properties = append([]damage.Property(nil), pool.Properties...)
	}

	return copy
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
