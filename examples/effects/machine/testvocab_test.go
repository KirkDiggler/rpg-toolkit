// Copyright (C) 2026 Kirk Diggler
// SPDX-License-Identifier: GPL-3.0-or-later

package machine_test

import (
	"context"
	"encoding/json/jsontext"

	"github.com/KirkDiggler/rpg-toolkit/core"
	"github.com/KirkDiggler/rpg-toolkit/core/chain"
	"github.com/KirkDiggler/rpg-toolkit/events"
	"github.com/KirkDiggler/rpg-toolkit/examples/effects/machine"
)

// The Rage-shaped test vocabulary (ruling 4): exactly the facts, beats and
// selectors Rage reads, named as Rage names them, over test events.

type turnEnd struct {
	SubjectID string
	Round     int
}

type damageReceived struct{ TargetID string }

type postAttackRoll struct{ AttackerID string }

type droppedToZero struct{ MemberID string }

type rest struct{ CharacterID string }

type combatEnd struct{ SubjectID string }

var (
	turnEndTopic        = events.DefineTypedTopic[turnEnd]("test.turn_end")
	damageReceivedTopic = events.DefineTypedTopic[damageReceived]("test.damage_received")
	postAttackRollChain = events.DefineChainedTopic[postAttackRoll]("test.post_attack_roll")
	droppedToZeroTopic  = events.DefineTypedTopic[droppedToZero]("test.dropped_to_zero")
	restTopic           = events.DefineTypedTopic[rest]("test.rest")
	combatEndTopic      = events.DefineTypedTopic[combatEnd]("test.combat_end")
)

const (
	beatTurnEnd        machine.Beat = "turn_end"
	beatDamageReceived machine.Beat = "damage_received"
	beatPostAttackRoll machine.Beat = "post_attack_roll"
	beatDroppedToZero  machine.Beat = "dropped_to_zero"
	beatRest           machine.Beat = "rest"
	beatCombatEnd      machine.Beat = "combat_end"

	selSubject machine.Selector = "holder_is_subject"
	selActor   machine.Selector = "holder_is_actor"
	selTarget  machine.Selector = "holder_is_target"

	keyWasHit    = "was_hit_this_turn"
	keyDidAttack = "did_attack_this_turn"

	stFresh  = "fresh"
	stRaging = "raging"
	stEnded  = "ended"

	factMelee      machine.FactName = "action.melee"
	factWeaponPool machine.FactName = "action.weapon_pool"
	factAbility    machine.FactName = "action.ability"
)

// rageFrame is the test frame: a nil pointer or empty string is an unknown
// fact.
type rageFrame struct {
	Melee      *bool
	WeaponPool *bool
	Ability    string
}

func readBool(get func(rageFrame) *bool) func(any) (any, bool) {
	return func(frame any) (any, bool) {
		f, ok := frame.(rageFrame)
		if !ok {
			return nil, false
		}
		v := get(f)
		if v == nil {
			return nil, false
		}
		return *v, true
	}
}

// typedBeat plugs a typed topic in as a beat.
func typedBeat[T any](
	def *events.TypedTopicDef[T], sels []machine.Selector, id func(T) string, round func(T) (int, bool),
) machine.BeatSpec {
	spec := machine.BeatSpec{
		Selectors: sels,
		Subscribe: func(ctx context.Context, bus events.EventBus, h func(context.Context, any) error) (string, error) {
			return def.On(bus).Subscribe(ctx, func(ctx context.Context, e T) error { return h(ctx, e) })
		},
		About: func(ev any, _ machine.Selector) (string, bool) {
			e, ok := ev.(T)
			if !ok {
				return "", false
			}
			return id(e), true
		},
	}
	if round != nil {
		spec.Round = func(ev any) (int, bool) {
			e, ok := ev.(T)
			if !ok {
				return 0, false
			}
			return round(e)
		}
	}
	return spec
}

// chainedBeat plugs a chained topic in as a beat; the handler returns the
// chain unmodified.
func chainedBeat[T any](def *events.ChainedTopicDef[T], sels []machine.Selector, id func(T) string) machine.BeatSpec {
	return machine.BeatSpec{
		Selectors: sels,
		Subscribe: func(ctx context.Context, bus events.EventBus, h func(context.Context, any) error) (string, error) {
			return def.On(bus).SubscribeWithChain(ctx, func(ctx context.Context, e T, c chain.Chain[T]) (chain.Chain[T], error) {
				return c, h(ctx, e)
			})
		},
		About: func(ev any, _ machine.Selector) (string, bool) {
			e, ok := ev.(T)
			if !ok {
				return "", false
			}
			return id(e), true
		},
	}
}

func rageVocabulary() machine.Vocabulary {
	subject := []machine.Selector{selSubject}
	return machine.Vocabulary{
		Facts: map[machine.FactName]machine.FactSpec{
			factMelee:      {Kind: machine.KindBool, Read: readBool(func(f rageFrame) *bool { return f.Melee })},
			factWeaponPool: {Kind: machine.KindBool, Read: readBool(func(f rageFrame) *bool { return f.WeaponPool })},
			factAbility: {Kind: machine.KindString, Read: func(frame any) (any, bool) {
				f, ok := frame.(rageFrame)
				if !ok || f.Ability == "" {
					return nil, false
				}
				return f.Ability, true
			}},
		},
		Beats: map[machine.Beat]machine.BeatSpec{
			beatTurnEnd: typedBeat(turnEndTopic, subject,
				func(e turnEnd) string { return e.SubjectID },
				func(e turnEnd) (int, bool) { return e.Round, e.Round > 0 }),
			beatDamageReceived: typedBeat(damageReceivedTopic, []machine.Selector{selTarget},
				func(e damageReceived) string { return e.TargetID }, nil),
			beatPostAttackRoll: chainedBeat(postAttackRollChain, []machine.Selector{selActor},
				func(e postAttackRoll) string { return e.AttackerID }),
			beatDroppedToZero: typedBeat(droppedToZeroTopic, subject,
				func(e droppedToZero) string { return e.MemberID }, nil),
			beatRest: typedBeat(restTopic, subject,
				func(e rest) string { return e.CharacterID }, nil),
			beatCombatEnd: typedBeat(combatEndTopic, subject,
				func(e combatEnd) string { return e.SubjectID }, nil),
		},
	}
}

// rageOutputs are opaque to the machine: the composer's JSON, carried
// through a recipe and its round-trip untouched.
var rageOutputs = []jsontext.Value{
	jsontext.Value(`{"kind":"damage_bonus","when":{"all":[` +
		`{"fact":{"name":"action.melee","op":"eq","value":true}},` +
		`{"fact":{"name":"action.weapon_pool","op":"eq","value":true}},` +
		`{"fact":{"name":"action.ability","op":"eq","value":"str"}}]},` +
		`"amount":{"by_class_level":{"class":"barbarian","steps":{"1":2,"9":3,"16":4}}},` +
		`"params":{"stage":"features"}}`),
	jsontext.Value(`{"kind":"resist","params":{"types":["bludgeoning","piercing","slashing"]}}`),
	jsontext.Value(`{"kind":"advantage","when":{"fact":{"name":"action.ability","op":"eq","value":"str"}},` +
		`"params":{"on":"saving_throws"}}`),
	jsontext.Value(`{"kind":"advantage","when":{"fact":{"name":"action.ability","op":"eq","value":"str"}},` +
		`"params":{"on":"ability_checks"}}`),
}

var ragingRef = core.Ref{Module: "dnd5e", Type: "conditions", ID: "raging"}

// rageRecipe is the design's raging recipe, field for field.
func rageRecipe() machine.Recipe {
	clearFlags := machine.Act{ClearFlags: []string{keyWasHit, keyDidAttack}}
	rageEndings := func() []machine.Transition {
		return []machine.Transition{
			{On: beatDroppedToZero, About: selSubject, To: stEnded, Reason: "unconscious"},
			{On: beatRest, About: selSubject, To: stEnded, Reason: "rest"},
			{On: beatCombatEnd, About: selSubject, To: stEnded, Reason: "combat_ended"},
		}
	}
	return machine.Recipe{
		Ref:     ragingRef,
		Stacks:  machine.StacksByRef,
		Initial: stFresh,
		Bag: machine.Bag{
			keyWasHit:                 false,
			keyDidAttack:              false,
			machine.KeySawTurnEnd:     false,
			machine.KeyRoundActivated: 0,
			machine.KeyTurnsActive:    0,
		},
		Trackers: []machine.Tracker{
			{On: beatPostAttackRoll, About: selActor, Set: keyDidAttack},
			{On: beatDamageReceived, About: selTarget, Set: keyWasHit},
		},
		States: map[string]machine.State{
			stFresh: {Outputs: rageOutputs, Transitions: append([]machine.Transition{
				{On: beatTurnEnd, About: selSubject, To: stRaging, Do: []machine.Act{{AnchorRound: true}, clearFlags}},
			}, rageEndings()...)},
			stRaging: {Outputs: rageOutputs, Transitions: append([]machine.Transition{
				{
					On: beatTurnEnd, About: selSubject,
					When: &machine.Guard{Not: &machine.Guard{Any: []machine.Guard{
						{Flag: keyWasHit}, {Flag: keyDidAttack},
					}}},
					To: stEnded, Reason: "no_combat_activity",
				},
				{
					On: beatTurnEnd, About: selSubject,
					When: &machine.Guard{Rounds: &machine.RoundsLeaf{SinceAnchor: &machine.IntOp{Op: machine.OpGte, N: 9}}},
					To:   stEnded, Reason: "duration_expired",
				},
				{
					On: beatTurnEnd, About: selSubject,
					When: &machine.Guard{Rounds: &machine.RoundsLeaf{WentBackwards: true}},
					To:   stEnded, Reason: "clock_reset",
				},
				{On: beatTurnEnd, About: selSubject, Do: []machine.Act{{DeriveTurnsActive: true}, clearFlags}},
			}, rageEndings()...)},
			stEnded: {Terminal: true},
		},
	}
}
