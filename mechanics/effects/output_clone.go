// Copyright (C) 2026 Kirk Diggler
// SPDX-License-Identifier: GPL-3.0-or-later

package effects

import (
	"maps"
	"slices"

	"github.com/KirkDiggler/rpg-toolkit/mechanics/effects/machine"
)

// Clone returns detached authored output data: params, guard tree and amount
// strategy. It does not validate meaning, evaluate a guard/amount or capture a
// live bag. Guard literals are scalar values or JSON-shaped containers.
func (o Output) Clone() Output {
	out := o
	out.Params = slices.Clone(o.Params)
	out.When = cloneOutputGuard(o.When)
	if o.Amount != nil {
		amount := *o.Amount
		if amount.Fixed != nil {
			n := *amount.Fixed
			amount.Fixed = &n
		}
		if amount.ByClassLevel != nil {
			steps := *amount.ByClassLevel
			steps.Steps = maps.Clone(steps.Steps)
			amount.ByClassLevel = &steps
		}
		out.Amount = &amount
	}
	return out
}
func cloneOutputGuard(in *machine.Guard) *machine.Guard {
	if in == nil {
		return nil
	}
	out := *in
	out.All = cloneOutputGuards(in.All)
	out.Any = cloneOutputGuards(in.Any)
	out.Not = cloneOutputGuard(in.Not)
	if in.Fact != nil {
		v := *in.Fact
		v.Value = cloneOutputLiteral(v.Value)
		out.Fact = &v
	}
	if in.Bag != nil {
		v := *in.Bag
		v.Value = cloneOutputLiteral(v.Value)
		out.Bag = &v
	}
	if in.Rounds != nil {
		v := *in.Rounds
		if v.SinceAnchor != nil {
			n := *v.SinceAnchor
			v.SinceAnchor = &n
		}
		out.Rounds = &v
	}
	if in.ClassLevel != nil {
		v := *in.ClassLevel
		out.ClassLevel = &v
	}
	return &out
}
func cloneOutputGuards(in []machine.Guard) []machine.Guard {
	if in == nil {
		return nil
	}
	out := make([]machine.Guard, len(in))
	for i := range in {
		out[i] = *cloneOutputGuard(&in[i])
	}
	return out
}
func cloneOutputLiteral(in any) any {
	switch v := in.(type) {
	case []any:
		out := make([]any, len(v))
		for i := range v {
			out[i] = cloneOutputLiteral(v[i])
		}
		return out
	case map[string]any:
		out := make(map[string]any, len(v))
		for k, value := range v {
			out[k] = cloneOutputLiteral(value)
		}
		return out
	default:
		return in
	}
}
