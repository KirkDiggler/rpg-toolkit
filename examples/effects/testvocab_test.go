// Copyright (C) 2026 Kirk Diggler
// SPDX-License-Identifier: GPL-3.0-or-later

package effects_test

import (
	"context"
	"encoding/json/jsontext"
	"encoding/json/v2"
	"errors"
	"fmt"
	"slices"

	"github.com/KirkDiggler/rpg-toolkit/core"
	"github.com/KirkDiggler/rpg-toolkit/core/chain"
	"github.com/KirkDiggler/rpg-toolkit/events"
	"github.com/KirkDiggler/rpg-toolkit/examples/effects"
	"github.com/KirkDiggler/rpg-toolkit/examples/effects/machine"
)

// The composer's Rage-shaped test vocabulary (ruling 4). The machine's side
// repeats the machine package's test vocabulary, since test files do not
// cross packages; the composer's side adds the three kinds Rage reads
// (damage_bonus, resist, advantage) over test chains, and the one amount
// (by_class_level).

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

	factMelee      machine.FactName = "action.melee"
	factWeaponPool machine.FactName = "action.weapon_pool"
	factAbility    machine.FactName = "action.ability"

	kindDamageBonus effects.KindName = "damage_bonus"
	kindResist      effects.KindName = "resist"
	kindAdvantage   effects.KindName = "advantage"

	keyWasHit    = "was_hit_this_turn"
	keyDidAttack = "did_attack_this_turn"

	stFresh  = "fresh"
	stRaging = "raging"
	stEnded  = "ended"

	holder      = "barbarian-1"
	otherHolder = "goblin-1"
	barbarian   = "barbarian"
	abilitySTR  = "str"
)

// rageFrame is the test frame: a nil pointer, empty string or zero level is
// an unknown fact.
type rageFrame struct {
	Melee      *bool
	WeaponPool *bool
	Ability    string
	Levels     map[string]int
}

func boolPtr(b bool) *bool { return &b }

// strMelee is a Strength melee weapon attack by a barbarian of level.
func strMelee(level int) rageFrame {
	return rageFrame{
		Melee: boolPtr(true), WeaponPool: boolPtr(true), Ability: abilitySTR,
		Levels: map[string]int{barbarian: level},
	}
}

func readBool(get func(rageFrame) *bool) func(any) (any, bool) {
	return func(frame any) (any, bool) {
		f, ok := frame.(rageFrame)
		if !ok || get(f) == nil {
			return nil, false
		}
		return *get(f), true
	}
}

func typedBeat[T any](
	def *events.TypedTopicDef[T], sel machine.Selector, id func(T) string, round func(T) (int, bool),
) machine.BeatSpec {
	spec := machine.BeatSpec{
		Selectors: []machine.Selector{sel},
		Subscribe: func(ctx context.Context, bus events.EventBus, h func(context.Context, any) error) (string, error) {
			return def.On(bus).Subscribe(ctx, func(ctx context.Context, e T) error { return h(ctx, e) })
		},
		About: func(ev any, _ machine.Selector) (string, bool) {
			e, ok := ev.(T)
			return id(e), ok
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

func rageVocabulary() machine.Vocabulary {
	return machine.Vocabulary{
		Facts: map[machine.FactName]machine.FactSpec{
			factMelee:      {Kind: machine.KindBool, Read: readBool(func(f rageFrame) *bool { return f.Melee })},
			factWeaponPool: {Kind: machine.KindBool, Read: readBool(func(f rageFrame) *bool { return f.WeaponPool })},
			factAbility: {Kind: machine.KindString, Read: func(frame any) (any, bool) {
				f, ok := frame.(rageFrame)
				return f.Ability, ok && f.Ability != ""
			}},
		},
		Beats: map[machine.Beat]machine.BeatSpec{
			beatTurnEnd: typedBeat(turnEndTopic, selSubject,
				func(e turnEnd) string { return e.SubjectID },
				func(e turnEnd) (int, bool) { return e.Round, e.Round > 0 }),
			beatDamageReceived: typedBeat(damageReceivedTopic, selTarget,
				func(e damageReceived) string { return e.TargetID }, nil),
			beatPostAttackRoll: {
				Selectors: []machine.Selector{selActor},
				Subscribe: func(ctx context.Context, bus events.EventBus, h func(context.Context, any) error) (string, error) {
					return postAttackRollChain.On(bus).SubscribeWithChain(ctx,
						func(ctx context.Context, e postAttackRoll, c chain.Chain[postAttackRoll]) (chain.Chain[postAttackRoll], error) {
							return c, h(ctx, e)
						})
				},
				About: func(ev any, _ machine.Selector) (string, bool) {
					e, ok := ev.(postAttackRoll)
					return e.AttackerID, ok
				},
			},
			beatDroppedToZero: typedBeat(droppedToZeroTopic, selSubject,
				func(e droppedToZero) string { return e.MemberID }, nil),
			beatRest:      typedBeat(restTopic, selSubject, func(e rest) string { return e.CharacterID }, nil),
			beatCombatEnd: typedBeat(combatEndTopic, selSubject, func(e combatEnd) string { return e.SubjectID }, nil),
		},
	}
}

// The test chains the kinds attach to.

type testDamage struct {
	AttackerID string
	TargetID   string
	Frame      rageFrame
	Type       string
	Total      int
}

type testRoll struct {
	RollerID  string
	On        string
	Frame     rageFrame
	Advantage bool
}

var (
	testDamageChain = events.DefineChainedTopic[*testDamage]("test.damage_chain")
	testRollChain   = events.DefineChainedTopic[*testRoll]("test.roll_chain")
)

var damageStages = []chain.Stage{"features", "final"}

// damage publishes one attack through the damage chain and returns the total.
func damage(ctx context.Context, bus events.EventBus, e testDamage) (int, error) {
	c, err := testDamageChain.On(bus).PublishWithChain(ctx, &e, events.NewStagedChain[*testDamage](damageStages))
	if err != nil {
		return 0, err
	}
	out, err := c.Execute(ctx, &e)
	if err != nil {
		return 0, err
	}
	return out.Total, nil
}

// roll publishes one roll through the roll chain and reports advantage.
func roll(ctx context.Context, bus events.EventBus, e testRoll) (bool, error) {
	stages := []chain.Stage{"conditions"}
	c, err := testRollChain.On(bus).PublishWithChain(ctx, &e, events.NewStagedChain[*testRoll](stages))
	if err != nil {
		return false, err
	}
	out, err := c.Execute(ctx, &e)
	if err != nil {
		return false, err
	}
	return out.Advantage, nil
}

var errCannotAnswer = errors.New("rule cannot answer")

// applies asks the output's guard; Depends fails the fold, as dnd5e's
// executeRule does.
func applies(o *effects.Bound, frame any) (bool, error) {
	switch v, reason := o.Answer(frame); v {
	case machine.Applies:
		return true, nil
	case machine.Depends:
		return false, fmt.Errorf("%w: %s", errCannotAnswer, reason)
	}
	return false, nil
}

type damageBonusParams struct {
	Stage chain.Stage `json:"stage"`
}

type resistParams struct {
	Types []string `json:"types"`
}

type advantageParams struct {
	On string `json:"on"`
}

func decode[T any](raw jsontext.Value) (T, error) {
	var p T
	err := json.Unmarshal(raw, &p, json.RejectUnknownMembers(true))
	return p, err
}

func unsubscribe(bus events.EventBus, id string) effects.Detach {
	return func(ctx context.Context) error { return bus.Unsubscribe(ctx, id) }
}

func row(frame any, o *effects.Bound) (effects.Row, error) {
	v, reason := o.Answer(frame)
	r := effects.Row{Kind: o.Kind(), Verdict: v, Reason: reason}
	if v == machine.Applies && o.HasAmount() {
		n, err := o.Amount(frame)
		if err != nil {
			return r, err
		}
		r.Amount = n
	}
	return r, nil
}

var damageBonusKind = effects.KindSpec{
	Check: func(raw jsontext.Value) error {
		_, err := decode[damageBonusParams](raw)
		return err
	},
	Attach: func(ctx context.Context, bus events.EventBus, o *effects.Bound) (effects.Detach, error) {
		p, err := decode[damageBonusParams](o.Params())
		if err != nil {
			return nil, err
		}
		id, err := testDamageChain.On(bus).SubscribeWithChain(ctx, func(
			_ context.Context, e *testDamage, c chain.Chain[*testDamage],
		) (chain.Chain[*testDamage], error) {
			if e.AttackerID != o.Host().Holder {
				return c, nil
			}
			ok, err := applies(o, e.Frame)
			if err != nil || !ok {
				return c, err
			}
			n, err := o.Amount(e.Frame)
			if err != nil {
				return c, err
			}
			return c, c.Add(p.Stage, o.ID(), func(_ context.Context, e *testDamage) (*testDamage, error) {
				e.Total += n
				return e, nil
			})
		})
		if err != nil {
			return nil, err
		}
		return unsubscribe(bus, id), nil
	},
	Assess: row,
}

var resistKind = effects.KindSpec{
	Check: func(raw jsontext.Value) error {
		_, err := decode[resistParams](raw)
		return err
	},
	Attach: func(ctx context.Context, bus events.EventBus, o *effects.Bound) (effects.Detach, error) {
		p, err := decode[resistParams](o.Params())
		if err != nil {
			return nil, err
		}
		id, err := testDamageChain.On(bus).SubscribeWithChain(ctx, func(
			_ context.Context, e *testDamage, c chain.Chain[*testDamage],
		) (chain.Chain[*testDamage], error) {
			if e.TargetID != o.Host().Holder || !slices.Contains(p.Types, e.Type) {
				return c, nil
			}
			return c, c.Add("final", o.ID(), func(_ context.Context, e *testDamage) (*testDamage, error) {
				e.Total /= 2
				return e, nil
			})
		})
		if err != nil {
			return nil, err
		}
		return unsubscribe(bus, id), nil
	},
	Assess: row,
}

var advantageKind = effects.KindSpec{
	Check: func(raw jsontext.Value) error {
		_, err := decode[advantageParams](raw)
		return err
	},
	Attach: func(ctx context.Context, bus events.EventBus, o *effects.Bound) (effects.Detach, error) {
		p, err := decode[advantageParams](o.Params())
		if err != nil {
			return nil, err
		}
		id, err := testRollChain.On(bus).SubscribeWithChain(ctx, func(
			_ context.Context, e *testRoll, c chain.Chain[*testRoll],
		) (chain.Chain[*testRoll], error) {
			if e.RollerID != o.Host().Holder || e.On != p.On {
				return c, nil
			}
			ok, err := applies(o, e.Frame)
			if err != nil || !ok {
				return c, err
			}
			return c, c.Add("conditions", o.ID(), func(_ context.Context, e *testRoll) (*testRoll, error) {
				e.Advantage = true
				return e, nil
			})
		})
		if err != nil {
			return nil, err
		}
		return unsubscribe(bus, id), nil
	},
	Assess: row,
}

// byClassLevel answers the highest step at or below the frame's level in
// the class.
func byClassLevel(frame any, a effects.Amount) (int, error) {
	f, _ := frame.(rageFrame)
	level := f.Levels[a.ByClassLevel.Class]
	if level == 0 {
		return 0, fmt.Errorf("%w: no %s levels on the frame", errCannotAnswer, a.ByClassLevel.Class)
	}
	best, at := 0, 0
	for step, n := range a.ByClassLevel.Steps {
		if step <= level && step > at {
			best, at = n, step
		}
	}
	return best, nil
}

// removedTopic is the test removal topic the End hook publishes on.
type removed struct {
	Ref    string
	Holder string
	Reason string
}

var removedTopic = events.DefineTypedTopic[removed]("test.condition_removed")

func outputVocabulary() effects.OutputVocabulary {
	return effects.OutputVocabulary{
		Kinds: map[effects.KindName]effects.KindSpec{
			kindDamageBonus: damageBonusKind,
			kindResist:      resistKind,
			kindAdvantage:   advantageKind,
		},
		Amounts: map[effects.AmountName]effects.AmountFn{effects.AmountByClassLevel: byClassLevel},
		End: func(ctx context.Context, bus events.EventBus, h effects.Host, reason string) error {
			return removedTopic.On(bus).Publish(ctx, removed{Ref: h.Ref.String(), Holder: h.Holder, Reason: reason})
		},
	}
}

func output(o effects.Output) jsontext.Value {
	data, err := json.Marshal(o)
	if err != nil {
		panic(err)
	}
	return data
}

func strGuard() *machine.Guard {
	return &machine.Guard{Fact: &machine.FactLeaf{Name: factAbility, Op: machine.OpEq, Value: abilitySTR}}
}

// rageOutputs are the design's four Rage outputs as composer data.
func rageOutputs() []jsontext.Value {
	return []jsontext.Value{
		output(effects.Output{
			Kind: kindDamageBonus,
			When: &machine.Guard{All: []machine.Guard{
				{Fact: &machine.FactLeaf{Name: factMelee, Op: machine.OpEq, Value: true}},
				{Fact: &machine.FactLeaf{Name: factWeaponPool, Op: machine.OpEq, Value: true}},
				{Fact: &machine.FactLeaf{Name: factAbility, Op: machine.OpEq, Value: abilitySTR}},
			}},
			Amount: &effects.Amount{ByClassLevel: &effects.ClassSteps{Class: barbarian, Steps: map[int]int{1: 2, 9: 3, 16: 4}}},
			Params: jsontext.Value(`{"stage":"features"}`),
		}),
		output(effects.Output{Kind: kindResist, Params: jsontext.Value(`{"types":["bludgeoning","piercing","slashing"]}`)}),
		output(effects.Output{Kind: kindAdvantage, When: strGuard(), Params: jsontext.Value(`{"on":"saving_throws"}`)}),
		output(effects.Output{Kind: kindAdvantage, When: strGuard(), Params: jsontext.Value(`{"on":"ability_checks"}`)}),
	}
}

var ragingRef = core.Ref{Module: "dnd5e", Type: "conditions", ID: "raging"}

// rageRecipe is the design's raging recipe, field for field.
func rageRecipe() machine.Recipe {
	upkeep := func(clearFlags machine.Act) []machine.Act {
		return []machine.Act{{AnchorRound: true}, {DeriveTurnsActive: true}, clearFlags}
	}
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
			stFresh: {Outputs: rageOutputs(), Transitions: append([]machine.Transition{
				{On: beatTurnEnd, About: selSubject, To: stRaging, Do: upkeep(clearFlags)},
			}, rageEndings()...)},
			stRaging: {Outputs: rageOutputs(), Transitions: append([]machine.Transition{
				{
					On: beatTurnEnd, About: selSubject,
					When: &machine.Guard{Not: &machine.Guard{Any: []machine.Guard{{Flag: keyWasHit}, {Flag: keyDidAttack}}}},
					To:   stEnded, Reason: "no_combat_activity",
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
				{On: beatTurnEnd, About: selSubject, Do: upkeep(clearFlags)},
			}, rageEndings()...)},
			stEnded: {Terminal: true},
		},
	}
}
