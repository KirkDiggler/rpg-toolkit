// Copyright (C) 2026 Kirk Diggler
// SPDX-License-Identifier: GPL-3.0-or-later

package effects

import (
	"context"

	"github.com/KirkDiggler/rpg-toolkit/core/chain"
	"github.com/KirkDiggler/rpg-toolkit/events"
	generic "github.com/KirkDiggler/rpg-toolkit/mechanics/effects"
	"github.com/KirkDiggler/rpg-toolkit/mechanics/effects/machine"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/contributions"
	dndevents "github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/events"
)

const (
	// FactMelee reads whether the action reaches as a melee attack.
	FactMelee machine.FactName = "action.melee"
	// FactWeaponPool reads whether the action has a weapon damage pool.
	FactWeaponPool machine.FactName = "action.weapon_pool"
	// FactAbility reads the action/save/check's actual governing ability.
	FactAbility machine.FactName = "action.ability"
	// Holder selects the entity the occurrence is about.
	Holder machine.Selector = "holder"
	// BeatTurnEnd names an entity's turn end and current round.
	BeatTurnEnd machine.Beat = "turn_end"
	// BeatDamageReceived observes positive, already-applied DamageTaken facts.
	// It never subscribes to the legacy instruction-like DamageReceived topic.
	BeatDamageReceived machine.Beat = "damage_received"
	// BeatAttackAttempt observes attack rolls, including misses.
	BeatAttackAttempt machine.Beat = "attack_attempt"
	// BeatDroppedToZero observes the damage-taken zero-HP fact.
	BeatDroppedToZero machine.Beat = "dropped_to_zero"
	// BeatRest observes a recipient's rest.
	BeatRest machine.Beat = "rest"
	// BeatCombatEnd observes a recipient's combat ending.
	BeatCombatEnd machine.Beat = "combat_end"
)

// Bind supplies a fresh closed vocabulary bound to immutable display/origin
// values. It refuses absent or invalid provenance/display; it creates no runtime,
// pays nothing, attaches nothing and knows no named condition or item package.
func Bind(in *BindInput) (*BindOutput, error) {
	if err := validateBinding(in); err != nil {
		return nil, err
	}
	binding := *in
	kinds := make(map[generic.KindName]generic.KindSpec, len(kindTable))
	for name, definition := range kindTable {
		if definition.meaning == nil {
			continue // lifecycle-only parts produce recipes, not an output kind
		}
		spec := generic.KindSpec{
			Check:     definition.check,
			Execution: definition.execution,
			Assess: func(frame any, out *generic.Bound) (generic.Row, error) {
				q, err := queryFrom(frame)
				if err != nil {
					return generic.Row{}, err
				}
				answer, err := assess(assessmentInput(out, binding, q))
				if err != nil {
					return generic.Row{}, err
				}
				row := generic.Row{Kind: out.Kind(), Reason: answer.effect.Reason, Amount: answer.value}
				switch answer.effect.State {
				case contributions.StateApplies:
					row.Verdict = machine.Applies
				case contributions.StateDoesNotApply:
					row.Verdict = machine.DoesNotApply
				default:
					row.Verdict = machine.Depends
				}
				return row, nil
			},
		}
		if definition.execution != generic.KindQuery {
			spec.Attach = func(ctx context.Context, bus events.EventBus, out *generic.Bound) (generic.Detach, error) {
				return definition.attach(ctx, bus, out, binding)
			}
		}
		kinds[name] = spec
	}
	return &BindOutput{Machine: machineVocabulary(), Outputs: generic.OutputVocabulary{Kinds: kinds, Amounts: map[generic.AmountName]generic.AmountFn{
		generic.AmountFixed: fixedAmount, generic.AmountByClassLevel: classAmount, generic.AmountDice: diceAmount,
	}}}, nil
}

func typedBeat[T any](topic *events.TypedTopicDef[T], subject func(T) (string, bool), round func(T) (int, bool)) machine.BeatSpec {
	spec := machine.BeatSpec{Selectors: []machine.Selector{Holder}, Subscribe: func(ctx context.Context, bus events.EventBus, handler func(context.Context, any) error) (string, error) {
		return topic.On(bus).Subscribe(ctx, func(ctx context.Context, event T) error { return handler(ctx, event) })
	}, About: func(event any, _ machine.Selector) (string, bool) {
		typed, ok := event.(T)
		if !ok {
			return "", false
		}
		return subject(typed)
	}}
	if round != nil {
		spec.Round = func(event any) (int, bool) {
			typed, ok := event.(T)
			if !ok {
				return 0, false
			}
			return round(typed)
		}
	}
	return spec
}

func actionBool(frame any, weapon bool) (any, bool) {
	q, err := queryFrom(frame)
	if err != nil || q.Action == nil {
		return nil, false
	}
	if weapon {
		return q.Action.Action.WeaponPool.Get()
	}
	return q.Action.Action.Melee.Get()
}

func machineVocabulary() machine.Vocabulary {
	return machine.Vocabulary{Facts: map[machine.FactName]machine.FactSpec{
		factApplication:   membershipFact(machine.KindString, func(e dndevents.ApplicationMembersChangedEvent) any { return e.ApplicationID }),
		factOwnerRef:      membershipFact(machine.KindString, func(e dndevents.ApplicationMembersChangedEvent) any { return e.Owner.ConditionRef }),
		factOwnerSource:   membershipFact(machine.KindString, func(e dndevents.ApplicationMembersChangedEvent) any { return e.Owner.SourceID }),
		factMembersBefore: membershipFact(machine.KindInt, func(e dndevents.ApplicationMembersChangedEvent) any { return e.Before }),
		factMembersAfter:  membershipFact(machine.KindInt, func(e dndevents.ApplicationMembersChangedEvent) any { return e.After }),
		FactMelee:         {Kind: machine.KindBool, Read: func(frame any) (any, bool) { return actionBool(frame, false) }},
		FactWeaponPool:    {Kind: machine.KindBool, Read: func(frame any) (any, bool) { return actionBool(frame, true) }},
		FactAbility: {Kind: machine.KindString, Read: func(frame any) (any, bool) {
			q, err := queryFrom(frame)
			if err != nil {
				return nil, false
			}
			ability, known := q.ability()
			return string(ability), known
		}},
	}, Beats: map[machine.Beat]machine.BeatSpec{
		beatMembersChanged: typedBeat(dndevents.ApplicationMembersChangedTopic, func(e dndevents.ApplicationMembersChangedEvent) (string, bool) {
			return e.Owner.MemberID, e.Owner.MemberID != ""
		}, nil),
		BeatTurnEnd: typedBeat(dndevents.TurnEndTopic, func(e dndevents.TurnEndEvent) (string, bool) { return e.SubjectID, e.SubjectID != "" }, func(e dndevents.TurnEndEvent) (int, bool) { return e.Round, e.Round > 0 }),
		BeatDamageReceived: typedBeat(dndevents.DamageTakenTopic, func(e *dndevents.DamageTakenEvent) (string, bool) {
			if e == nil || e.Amount <= 0 {
				return "", false
			}
			return e.MemberID, e.MemberID != ""
		}, nil),
		BeatRest:      typedBeat(dndevents.RestTopic, func(e dndevents.RestEvent) (string, bool) { return e.CharacterID, e.CharacterID != "" }, nil),
		BeatCombatEnd: typedBeat(dndevents.CombatEndTopic, func(e dndevents.CombatEndEvent) (string, bool) { return e.SubjectID, e.SubjectID != "" }, nil),
		BeatDroppedToZero: typedBeat(dndevents.DamageTakenTopic, func(e *dndevents.DamageTakenEvent) (string, bool) {
			if e == nil || !e.DroppedToZero {
				return "", false
			}
			return e.MemberID, e.MemberID != ""
		}, nil),
		BeatAttackAttempt: {Selectors: []machine.Selector{Holder}, Subscribe: func(ctx context.Context, bus events.EventBus, handler func(context.Context, any) error) (string, error) {
			return dndevents.PostAttackRollChain.On(bus).SubscribeWithChain(ctx, func(ctx context.Context, event *dndevents.PostAttackRollEvent, c chain.Chain[*dndevents.PostAttackRollEvent]) (chain.Chain[*dndevents.PostAttackRollEvent], error) {
				return c, handler(ctx, event)
			})
		}, About: func(event any, _ machine.Selector) (string, bool) {
			e, ok := event.(*dndevents.PostAttackRollEvent)
			if !ok || e == nil {
				return "", false
			}
			return e.AttackerID, e.AttackerID != ""
		}},
	}}
}
