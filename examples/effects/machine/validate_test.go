// Copyright (C) 2026 Kirk Diggler
// SPDX-License-Identifier: GPL-3.0-or-later

package machine_test

import (
	"testing"

	"github.com/stretchr/testify/suite"

	"github.com/KirkDiggler/rpg-toolkit/examples/effects/machine"
)

type validateSuite struct {
	suite.Suite
}

// edit applies f to one state of r in place.
func edit(r *machine.Recipe, state string, f func(st *machine.State)) {
	st := r.States[state]
	f(&st)
	r.States[state] = st
}

func (s *validateSuite) TestRefusals() {
	cases := []struct {
		name  string
		spoil func(r *machine.Recipe)
		want  string
	}{
		{"a fact not in the vocabulary", func(r *machine.Recipe) {
			edit(r, stRaging, func(st *machine.State) {
				finesse := &machine.FactLeaf{Name: "action.finesse", Op: machine.OpEq, Value: true}
				st.Transitions[3].When = &machine.Guard{Fact: finesse}
			})
		}, `fact "action.finesse" is not in the vocabulary`},
		{"a flag not declared in the bag", func(r *machine.Recipe) {
			edit(r, stRaging, func(st *machine.State) { st.Transitions[3].When = &machine.Guard{Flag: "was_seen"} })
		}, `flag "was_seen" is not declared in the bag`},
		{"a To naming no state", func(r *machine.Recipe) {
			edit(r, stRaging, func(st *machine.State) { st.Transitions[0].To = "gone" })
		}, `to "gone" names no state`},
		{"an initial state that is terminal", func(r *machine.Recipe) { r.Initial = stEnded },
			`initial state "ended" is terminal`},
		{"a state no transition reaches", func(r *machine.Recipe) {
			r.States["sulking"] = machine.State{Transitions: []machine.Transition{
				{On: beatRest, About: selSubject, To: stEnded, Reason: "rest"},
			}}
		}, `state "sulking" is reached by no transition`},
		{"a transition into a terminal state without a reason", func(r *machine.Recipe) {
			edit(r, stFresh, func(st *machine.State) { st.Transitions[1].Reason = "" })
		}, `into terminal state "ended" has no reason`},
		{"a guard node with zero fields", func(r *machine.Recipe) {
			edit(r, stRaging, func(st *machine.State) { st.Transitions[3].When = &machine.Guard{} })
		}, "guard node sets 0 fields, want exactly 1"},
		{"a guard node with two fields", func(r *machine.Recipe) {
			edit(r, stRaging, func(st *machine.State) {
				st.Transitions[3].When = &machine.Guard{Flag: keyWasHit, Not: &machine.Guard{Flag: keyDidAttack}}
			})
		}, "guard node sets 2 fields, want exactly 1"},
		{"an act with zero fields", func(r *machine.Recipe) {
			edit(r, stRaging, func(st *machine.State) { st.Transitions[3].Do = []machine.Act{{}} })
		}, "act sets 0 fields, want exactly 1"},
		{"an act with two fields", func(r *machine.Recipe) {
			edit(r, stRaging, func(st *machine.State) {
				st.Transitions[3].Do = []machine.Act{{DeriveTurnsActive: true, ClearFlags: []string{keyWasHit}}}
			})
		}, "act sets 2 fields, want exactly 1"},
		{"a bag key nothing reads", func(r *machine.Recipe) { r.Bag["spare"] = 0 },
			`bag key "spare" is read by no guard, act or tracker`},
		{"by_source without a source_id key", func(r *machine.Recipe) { r.Stacks = machine.StacksBySource },
			`stacks "by_source" needs a "source_id" bag key`},
		{"a beat not in the vocabulary", func(r *machine.Recipe) {
			edit(r, stRaging, func(st *machine.State) { st.Transitions[4].On = "offer_taken" })
		}, `beat "offer_taken" is not in the vocabulary`},
		{"a selector the beat does not support", func(r *machine.Recipe) {
			edit(r, stRaging, func(st *machine.State) { st.Transitions[0].About = selActor })
		}, `beat "turn_end" does not support selector "holder_is_actor"`},
		{"a tracker setting an undeclared key", func(r *machine.Recipe) { r.Trackers[0].Set = "did_swing" },
			`flag "did_swing" is not declared in the bag`},
		{"a fact value of the wrong kind", func(r *machine.Recipe) {
			edit(r, stRaging, func(st *machine.State) {
				st.Transitions[3].When = &machine.Guard{Fact: &machine.FactLeaf{Name: factMelee, Op: machine.OpEq, Value: "yes"}}
			})
		}, `fact "action.melee" wants a bool value, got string`},
		{"an op the fact's kind does not support", func(r *machine.Recipe) {
			edit(r, stRaging, func(st *machine.State) {
				gte := &machine.FactLeaf{Name: factAbility, Op: machine.OpGte, Value: abilityStr}
				st.Transitions[3].When = &machine.Guard{Fact: gte}
			})
		}, `op "gte" does not apply to a string fact`},
		{"a rounds leaf on a beat that carries no round", func(r *machine.Recipe) {
			edit(r, stRaging, func(st *machine.State) {
				st.Transitions[4].When = &machine.Guard{Rounds: &machine.RoundsLeaf{WentBackwards: true}}
			})
		}, `beat "dropped_to_zero" carries no round`},
		{"a bag key that collides with the blob header", func(r *machine.Recipe) {
			r.Bag["state"] = ""
		}, `bag key "state" collides with the blob header`},
	}
	for _, tc := range cases {
		s.Run(tc.name, func() {
			r := rageRecipe()
			tc.spoil(&r)
			err := machine.Validate(r, rageVocabulary())
			s.Require().Error(err)
			s.ErrorIs(err, machine.ErrInvalidRecipe)
			s.Contains(err.Error(), tc.want)
		})
	}
}

func TestValidateRefuses(t *testing.T) {
	suite.Run(t, new(validateSuite))
}

type validateAcceptsSuite struct {
	suite.Suite
}

func (s *validateAcceptsSuite) TestRageShapedRecipeIsValid() {
	s.Require().NoError(machine.Validate(rageRecipe(), rageVocabulary()))
}

func (s *validateAcceptsSuite) TestCompileBindsTheProgram() {
	p, err := rageRecipe().Compile(rageVocabulary())
	s.Require().NoError(err)
	s.Equal(ragingRef, p.Ref())
	s.True(p.IsTerminal(stEnded))
	s.False(p.IsTerminal(stRaging))
	s.Len(p.Outputs(stRaging), 4)
}

// A key read only by the composer's outputs is accepted when the composer
// names it; the machine cannot see inside outputs.
func (s *validateAcceptsSuite) TestKeysReadElsewhereAreNamedByTheCaller() {
	r := rageRecipe()
	r.Bag["bonus_override"] = 0
	s.Require().Error(machine.Validate(r, rageVocabulary()))
	s.Require().NoError(machine.Validate(r, rageVocabulary(), "bonus_override"))
}

func (s *validateAcceptsSuite) TestBySourceWithASourceKey() {
	r := rageRecipe()
	r.Stacks = machine.StacksBySource
	r.Bag[machine.KeySourceID] = ""
	s.Require().NoError(machine.Validate(r, rageVocabulary()))
}

func (s *validateAcceptsSuite) TestParametersLandOnlyOnDeclaredKeys() {
	p, err := rageRecipe().Compile(rageVocabulary())
	s.Require().NoError(err)

	bag, err := p.NewBag(map[string]any{machine.KeyRoundActivated: float64(3)})
	s.Require().NoError(err)
	s.Equal(3, bag[machine.KeyRoundActivated], "a JSON number lands as the declared int")
	s.Equal(false, bag[keyWasHit], "undeclared parameters keep the zero value")

	_, err = p.NewBag(map[string]any{"counterpart_id": "goblin-1"})
	s.Require().Error(err)
	s.Contains(err.Error(), `parameter "counterpart_id" names no bag key`)

	_, err = p.NewBag(map[string]any{machine.KeyTurnsActive: "three"})
	s.Require().Error(err)
	s.Contains(err.Error(), `bag key "turns_active" wants a int value, got string`)
}

func TestValidateAcceptsRageShaped(t *testing.T) {
	suite.Run(t, new(validateAcceptsSuite))
}
