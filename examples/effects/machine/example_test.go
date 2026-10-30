// Copyright (C) 2026 Kirk Diggler
// SPDX-License-Identifier: GPL-3.0-or-later

package machine_test

import (
	"context"
	"fmt"

	"github.com/KirkDiggler/rpg-toolkit/core"
	"github.com/KirkDiggler/rpg-toolkit/events"
	"github.com/KirkDiggler/rpg-toolkit/examples/effects/machine"
)

type tick struct{ SubjectID string }
type tend struct{ TargetID string }

var tickTopic, tendTopic = events.DefineTypedTopic[tick]("example.tick"), events.DefineTypedTopic[tend]("example.tend")

// exampleBeat plugs a topic in as a beat: the vocabulary is the rulebook's.
func exampleBeat[T any](def *events.TypedTopicDef[T], sel machine.Selector, id func(T) string) machine.BeatSpec {
	return machine.BeatSpec{
		Selectors: []machine.Selector{sel},
		Subscribe: func(ctx context.Context, bus events.EventBus, h func(context.Context, any) error) (string, error) {
			return def.On(bus).Subscribe(ctx, func(ctx context.Context, e T) error { return h(ctx, e) })
		},
		About: func(ev any, _ machine.Selector) (string, bool) { e, ok := ev.(T); return id(e), ok },
	}
}

// A candle is lit by its first tick and goes out on any later tick where
// nobody tended it since the one before: three states, one tracker, and two
// transitions out of "lit" that run in recipe order.
func Example() {
	const candle, tended = "candle-1", "tended"
	const onTick, onTend machine.Beat = "tick", "tend"
	const subject, target machine.Selector = "holder_is_subject", "holder_is_target"
	vocab := machine.Vocabulary{Beats: map[machine.Beat]machine.BeatSpec{
		onTick: exampleBeat(tickTopic, subject, func(e tick) string { return e.SubjectID }),
		onTend: exampleBeat(tendTopic, target, func(e tend) string { return e.TargetID }),
	}}
	recipe := machine.Recipe{
		Ref:      core.Ref{Module: "example", Type: "lights", ID: "candle"},
		Initial:  "unlit",
		Bag:      machine.Bag{tended: false},
		Trackers: []machine.Tracker{{On: onTend, About: target, Set: tended}},
		States: map[string]machine.State{
			"unlit": {Transitions: []machine.Transition{{On: onTick, About: subject, To: "lit"}}},
			"lit": {Transitions: []machine.Transition{
				{On: onTick, About: subject, When: &machine.Guard{Not: &machine.Guard{Flag: tended}},
					To: "out", Reason: "untended"},
				{On: onTick, About: subject, Do: []machine.Act{{ClearFlags: []string{tended}}}}, // To empty: stay
			}},
			"out": {Terminal: true},
		},
	}
	program, err := recipe.Compile(vocab) // Validate runs here and refuses a bad recipe by name
	if err != nil {
		fmt.Println(err)
		return
	}
	ctx, bus := context.Background(), events.NewEventBus()
	m := machine.New(program, candle, nil, nil, "")
	m.OnTransition(func(_ context.Context, t machine.Transitioned) error {
		fmt.Printf("%s -> %s %q\n", t.From, t.To, t.Reason)
		return nil
	})
	_ = m.Apply(ctx, bus)
	ticks, tends := tickTopic.On(bus), tendTopic.On(bus)
	_ = ticks.Publish(ctx, tick{SubjectID: candle}) // unlit -> lit
	_ = tends.Publish(ctx, tend{TargetID: candle})  // the tracker sets tended
	_ = ticks.Publish(ctx, tick{SubjectID: candle}) // tended: stay, and clear it
	_ = ticks.Publish(ctx, tick{SubjectID: candle}) // untended: lit -> out
	fmt.Println(m.State(), m.Bag(), m.IsApplied())  // the terminal state detached the machine
	// Output:
	// unlit -> lit ""
	// lit -> out "untended"
	// out map[tended:false] false
}
