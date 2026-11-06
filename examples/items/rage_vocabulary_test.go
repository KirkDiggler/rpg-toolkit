// Copyright (C) 2026 Kirk Diggler
// SPDX-License-Identifier: GPL-3.0-or-later

package items_test

import (
	"context"
	"fmt"

	"github.com/KirkDiggler/rpg-toolkit/core/chain"
	"github.com/KirkDiggler/rpg-toolkit/events"
	"github.com/KirkDiggler/rpg-toolkit/examples/effects"
	"github.com/KirkDiggler/rpg-toolkit/examples/effects/machine"
	"github.com/KirkDiggler/rpg-toolkit/examples/items"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/combat"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/contributions"
	dndevents "github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/events"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/skills"
)

// This is a rulebook consumer plug, not a second state machine or a wrapper
// around the handwritten RagingCondition. The generic providers stay untouched.
func typedRageBeat[T any](topic *events.TypedTopicDef[T], subject func(T) (string, bool), round func(T) (int, bool)) machine.BeatSpec {
	spec := machine.BeatSpec{
		Selectors: []machine.Selector{rageSubject},
		Subscribe: func(ctx context.Context, bus events.EventBus, handler func(context.Context, any) error) (string, error) {
			return topic.On(bus).Subscribe(ctx, func(ctx context.Context, event T) error { return handler(ctx, event) })
		},
		About: func(event any, _ machine.Selector) (string, bool) {
			typed, ok := event.(T)
			if !ok {
				return "", false
			}
			return subject(typed)
		},
	}
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

func itemRageVocabulary() machine.Vocabulary {
	return machine.Vocabulary{
		Facts: map[machine.FactName]machine.FactSpec{
			rageMelee: {Kind: machine.KindBool, Read: func(frame any) (any, bool) {
				f, ok := frame.(contributions.Frame)
				if !ok {
					return nil, false
				}
				return f.Action.Melee.Get()
			}},
			rageWeapon: {Kind: machine.KindBool, Read: func(frame any) (any, bool) {
				f, ok := frame.(contributions.Frame)
				if !ok {
					return nil, false
				}
				return f.Action.WeaponPool.Get()
			}},
			rageAbility: {Kind: machine.KindString, Read: func(frame any) (any, bool) {
				switch f := frame.(type) {
				case contributions.Frame:
					ability, known := f.Action.Ability.Get()
					return string(ability), known
				case *dndevents.SavingThrowChainEvent:
					if f != nil {
						return string(f.Ability), true
					}
				case *dndevents.AbilityCheckChainEvent:
					if f != nil {
						return string(skills.Ability(f.Skill)), true
					}
				}
				return nil, false
			}},
		},
		Beats: map[machine.Beat]machine.BeatSpec{
			rageTurn: typedRageBeat(dndevents.TurnEndTopic,
				func(e dndevents.TurnEndEvent) (string, bool) { return e.SubjectID, true },
				func(e dndevents.TurnEndEvent) (int, bool) { return e.Round, true }),
			rageHit: typedRageBeat(dndevents.DamageReceivedTopic,
				func(e dndevents.DamageReceivedEvent) (string, bool) { return e.TargetID, true }, nil),
			rageRest: typedRageBeat(dndevents.RestTopic,
				func(e dndevents.RestEvent) (string, bool) { return e.CharacterID, true }, nil),
			rageCombatEnd: typedRageBeat(dndevents.CombatEndTopic,
				func(e dndevents.CombatEndEvent) (string, bool) { return e.SubjectID, true }, nil),
			rageZero: typedRageBeat(dndevents.DamageTakenTopic,
				func(e *dndevents.DamageTakenEvent) (string, bool) {
					if e == nil || !e.DroppedToZero {
						return "", false
					}
					return e.MemberID, true
				}, nil),
			rageAttack: {
				Selectors: []machine.Selector{rageSubject},
				Subscribe: func(ctx context.Context, bus events.EventBus, handler func(context.Context, any) error) (string, error) {
					return dndevents.PostAttackRollChain.On(bus).SubscribeWithChain(ctx,
						func(ctx context.Context, event *dndevents.PostAttackRollEvent, c chain.Chain[*dndevents.PostAttackRollEvent]) (chain.Chain[*dndevents.PostAttackRollEvent], error) {
							return c, handler(ctx, event)
						})
				},
				About: func(event any, _ machine.Selector) (string, bool) {
					e, ok := event.(*dndevents.PostAttackRollEvent)
					if !ok || e == nil {
						return "", false
					}
					return e.AttackerID, true
				},
			},
		},
	}
}

func rageApplies(out *effects.Bound, frame any) (bool, error) {
	verdict, reason := out.Answer(frame)
	if verdict == machine.Depends {
		return false, fmt.Errorf("%w: %s", contributions.ErrRuleCannotAnswer, reason)
	}
	return verdict == machine.Applies, nil
}

func rageSource(source items.Source) dndevents.RollSource {
	ref := rageConditionRef
	return dndevents.RollSource{Ref: &ref, Name: "Raging", SourceID: source.ItemID}
}

func itemRageOutputs(source items.Source) effects.OutputVocabulary {
	assess := func(frame any, out *effects.Bound) (effects.Row, error) {
		verdict, reason := out.Answer(frame)
		row := effects.Row{Kind: out.Kind(), Verdict: verdict, Reason: reason}
		if out.HasAmount() && verdict == machine.Applies {
			n, err := out.Amount(frame)
			if err != nil {
				return row, err
			}
			row.Amount = n
		}
		return row, nil
	}
	unsubscribe := func(bus events.EventBus, id string) effects.Detach {
		return func(ctx context.Context) error { return bus.Unsubscribe(ctx, id) }
	}
	return effects.OutputVocabulary{
		Amounts: map[effects.AmountName]effects.AmountFn{
			effects.AmountFixed: func(_ any, amount effects.Amount) (int, error) { return *amount.Fixed, nil },
		},
		End: func(ctx context.Context, bus events.EventBus, host effects.Host, reason string) error {
			return dndevents.ConditionRemovedTopic.On(bus).Publish(ctx, dndevents.ConditionRemovedEvent{
				MemberID: host.Holder, ConditionRef: host.Ref.String(), Reason: reason,
			})
		},
		Kinds: map[effects.KindName]effects.KindSpec{
			rageDamageKind: {
				Assess: assess,
				Attach: func(ctx context.Context, bus events.EventBus, out *effects.Bound) (effects.Detach, error) {
					id, err := dndevents.DamageChain.On(bus).SubscribeWithChain(ctx,
						func(_ context.Context, event *dndevents.DamageChainEvent, c chain.Chain[*dndevents.DamageChainEvent]) (chain.Chain[*dndevents.DamageChainEvent], error) {
							if event.AttackerID != out.Host().Holder {
								return c, nil
							}
							if err := event.Frame.Validate(); err != nil {
								return c, err
							}
							if event.Frame.Actor != event.AttackerID {
								return c, fmt.Errorf("damage actor and frame disagree")
							}
							applies, err := rageApplies(out, event.Frame)
							if err != nil || !applies {
								return c, err
							}
							n, err := out.Amount(event.Frame)
							if err != nil {
								return c, err
							}
							return c, c.Add(combat.StageFeatures, out.ID(), func(_ context.Context, e *dndevents.DamageChainEvent) (*dndevents.DamageChainEvent, error) {
								e.Components = append(e.Components, dndevents.DamageComponent{
									Source: dndevents.DamageSourceCondition, DamageType: e.WeaponDamageType,
									Roll: dndevents.RollComponent{Source: rageSource(source), Modifier: &n},
								})
								return e, nil
							})
						})
					if err != nil {
						return nil, err
					}
					return unsubscribe(bus, id), nil
				},
			},
			rageResistKind: {
				Assess: assess,
				Attach: func(ctx context.Context, bus events.EventBus, out *effects.Bound) (effects.Detach, error) {
					id, err := dndevents.IncomingDamageChain.On(bus).SubscribeWithChain(ctx,
						func(_ context.Context, event *dndevents.IncomingDamageEvent, c chain.Chain[*dndevents.IncomingDamageEvent]) (chain.Chain[*dndevents.IncomingDamageEvent], error) {
							if event.TargetID() != out.Host().Holder {
								return c, nil
							}
							return c, c.Add(combat.StageFinal, out.ID(), func(_ context.Context, e *dndevents.IncomingDamageEvent) (*dndevents.IncomingDamageEvent, error) {
								for _, kind := range e.DealtTypes() {
									if kind.IsPhysical() {
										e.Multipliers = append(e.Multipliers, dndevents.DamageMultiplier{
											Category: dndevents.DamageSourceCondition, Source: rageSource(source),
											DamageType: kind, Factor: dndevents.DamageFactorResistance,
										})
									}
								}
								return e, nil
							})
						})
					if err != nil {
						return nil, err
					}
					return unsubscribe(bus, id), nil
				},
			},
			rageSaveKind: {
				Assess: assess,
				Attach: func(ctx context.Context, bus events.EventBus, out *effects.Bound) (effects.Detach, error) {
					id, err := dndevents.SavingThrowChain.On(bus).SubscribeWithChain(ctx,
						func(_ context.Context, event *dndevents.SavingThrowChainEvent, c chain.Chain[*dndevents.SavingThrowChainEvent]) (chain.Chain[*dndevents.SavingThrowChainEvent], error) {
							if event.SaverID != out.Host().Holder {
								return c, nil
							}
							applies, err := rageApplies(out, event)
							if err != nil || !applies {
								return c, err
							}
							return c, c.Add(combat.StageConditions, out.ID(), func(_ context.Context, e *dndevents.SavingThrowChainEvent) (*dndevents.SavingThrowChainEvent, error) {
								ref := rageConditionRef
								e.AdvantageSources = append(e.AdvantageSources, dndevents.SaveModifierSource{
									Name: "Raging", SourceType: "condition", SourceRef: &ref, EntityID: source.ItemID,
								})
								return e, nil
							})
						})
					if err != nil {
						return nil, err
					}
					return unsubscribe(bus, id), nil
				},
			},
			rageCheckKind: {
				Assess: assess,
				Attach: func(ctx context.Context, bus events.EventBus, out *effects.Bound) (effects.Detach, error) {
					id, err := dndevents.AbilityCheckChain.On(bus).SubscribeWithChain(ctx,
						func(_ context.Context, event *dndevents.AbilityCheckChainEvent, c chain.Chain[*dndevents.AbilityCheckChainEvent]) (chain.Chain[*dndevents.AbilityCheckChainEvent], error) {
							if event.CheckerID != out.Host().Holder {
								return c, nil
							}
							applies, err := rageApplies(out, event)
							if err != nil || !applies {
								return c, err
							}
							return c, c.Add(combat.StageConditions, out.ID(), func(_ context.Context, e *dndevents.AbilityCheckChainEvent) (*dndevents.AbilityCheckChainEvent, error) {
								ref := rageConditionRef
								e.AdvantageSources = append(e.AdvantageSources, dndevents.CheckModifierSource{
									Name: "Raging", SourceType: "condition", SourceRef: &ref, EntityID: source.ItemID,
								})
								return e, nil
							})
						})
					if err != nil {
						return nil, err
					}
					return unsubscribe(bus, id), nil
				},
			},
		},
	}
}
