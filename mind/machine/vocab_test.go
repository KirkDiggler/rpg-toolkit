// Copyright (C) 2026 Kirk Diggler
// SPDX-License-Identifier: GPL-3.0-or-later

package machine_test

import (
	"github.com/KirkDiggler/rpg-toolkit/core"
	"github.com/KirkDiggler/rpg-toolkit/mind/machine"
)

// The beats and facts a host would supply. Nothing here is a rule: a tick
// carries its round, a sound carries how far it is from the listener, and
// the environment is a map of whatever the host knows.
const (
	beatTime  machine.Beat = "time"
	beatHeard machine.Beat = "heard"
	beatBell  machine.Beat = "bell"
)

// tick is a time beat's event: the round the clock reached.
type tick struct{ Round uint64 }

// sound is a heard beat's event: the distance to the listener in hexes and
// in regions.
type sound struct{ Hexes, Regions int }

// env is the host's facts about the creature asking.
type env map[machine.FactName]any

func testVocabulary() machine.Vocabulary {
	read := func(name machine.FactName) func(any) (any, bool) {
		return func(e any) (any, bool) {
			m, ok := e.(env)
			if !ok {
				return nil, false
			}
			v, ok := m[name]
			return v, ok
		}
	}
	return machine.Vocabulary{
		Facts: map[machine.FactName]machine.FactSpec{
			"enemy":  {Kind: machine.KindString, Read: read("enemy")},
			"wounds": {Kind: machine.KindInt, Read: read("wounds")},
			"lit":    {Kind: machine.KindBool, Read: read("lit")},
		},
		Beats: map[machine.Beat]machine.BeatSpec{
			beatTime: {
				Timed: true,
				Round: func(ev any) (uint64, bool) {
					t, ok := ev.(tick)
					return t.Round, ok
				},
			},
			beatHeard: {
				InScope: func(ev any, s machine.ScopeLeaf) bool {
					snd, ok := ev.(sound)
					if !ok {
						return false
					}
					if s.Within > 0 && snd.Hexes > s.Within {
						return false
					}
					if s.Regions > 0 && snd.Regions > s.Regions {
						return false
					}
					return true
				},
			},
			beatBell: {},
		},
	}
}

func testRef(id core.ID) core.Ref {
	return core.Ref{Module: "test", Type: "machines", ID: id}
}

// guardRecipe is the doc's recipe: a posted guard turns alert on a near
// sound, falls back to its post three rounds later, and an asleep one wakes
// only to a sound at its ear.
func guardRecipe() machine.Recipe {
	return machine.Recipe{
		Ref:     testRef("guard"),
		Initial: "posted",
		Bag:     machine.Bag{"alarmed": false, "seen": ""},
		States: map[string]machine.State{
			"posted": {Transitions: []machine.Transition{
				{On: beatTime, Do: []machine.Act{{Pick: "time"}}},
				{
					On:     beatHeard,
					When:   &machine.Guard{Scope: &machine.ScopeLeaf{Within: 6}},
					To:     "alert",
					Do:     []machine.Act{{Anchor: true}, {Set: &machine.SetAct{Key: "alarmed", Value: true}}},
					Reason: "heard something",
				},
			}},
			"alert": {Transitions: []machine.Transition{
				{
					On:     beatTime,
					When:   &machine.Guard{Rounds: &machine.RoundsLeaf{Op: machine.OpGte, N: 3}},
					To:     "posted",
					Do:     []machine.Act{{Clear: []string{"alarmed"}}},
					Reason: "nothing came",
				},
				{On: beatTime, Do: []machine.Act{{Pick: "alert"}}},
			}},
			"asleep": {Transitions: []machine.Transition{
				{
					On:     beatHeard,
					When:   &machine.Guard{Scope: &machine.ScopeLeaf{Within: 2}},
					To:     "posted",
					Reason: "woken",
				},
			}},
		},
	}
}

func mustCompile(r machine.Recipe) *machine.Program {
	p, err := r.Compile(testVocabulary())
	if err != nil {
		panic(err)
	}
	return p
}
