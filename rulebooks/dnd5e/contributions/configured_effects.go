// Copyright (C) 2026 Kirk Diggler
// SPDX-License-Identifier: GPL-3.0-or-later

package contributions

import (
	"encoding/json"
	"fmt"
	"maps"
	"slices"
	"strings"

	"github.com/KirkDiggler/rpg-toolkit/core"
	generic "github.com/KirkDiggler/rpg-toolkit/mechanics/effects"
	"github.com/KirkDiggler/rpg-toolkit/mechanics/effects/machine"
)

// ConfiguredOutput is a captured output definition, not a live runtime handle.
// Its ID is the provider's opaque output identity, including its generation.
// Meaning and applicability are interpreted by the owning rulebook vocabulary.
type ConfiguredOutput struct {
	ID         string
	Definition generic.Output
}

// HeldEffectDetail describes the configured target-side outputs of one holding.
// Absent detail is unknown; an explicit empty Outputs slice is known-none.
// Origin describes the source, not the member, address or application identity.
type HeldEffectDetail struct {
	MemberID      string
	Held          HeldCondition
	ApplicationID string
	DisplayName   string
	OriginRef     core.Ref
	OriginID      string
	Outputs       []ConfiguredOutput
}

type heldDetailKey struct {
	member string
	held   HeldCondition
}

func (f Frame) validateHeldEffects() error {
	details := make(map[heldDetailKey]bool, len(f.HeldEffects))
	outputs := map[string]bool{}
	for _, detail := range f.HeldEffects {
		key := heldDetailKey{member: detail.MemberID, held: detail.Held}
		if strings.TrimSpace(detail.MemberID) == "" || strings.TrimSpace(detail.Held.SourceID) == "" || details[key] {
			return fmt.Errorf("invalid or duplicate configured holding for %q", detail.MemberID)
		}
		details[key] = true
		held, known := f.HeldBy(detail.MemberID)
		if !known || !slices.Contains(held, detail.Held) {
			return fmt.Errorf("configured detail lacks its matching holding for %q", detail.MemberID)
		}
		if strings.TrimSpace(detail.ApplicationID) == "" || strings.TrimSpace(detail.DisplayName) == "" || strings.TrimSpace(detail.OriginID) == "" {
			return fmt.Errorf("configured detail requires generation, display and origin")
		}
		if err := detail.OriginRef.IsValid(); err != nil {
			return fmt.Errorf("configured origin: %w", err)
		}
		if detail.Outputs == nil {
			return fmt.Errorf("configured detail requires an explicit output list")
		}
		for _, out := range detail.Outputs {
			if strings.TrimSpace(out.ID) == "" || outputs[out.ID] || strings.TrimSpace(string(out.Definition.Kind)) == "" {
				return fmt.Errorf("invalid or duplicate configured output %q", out.ID)
			}
			outputs[out.ID] = true
			if len(out.Definition.Params) > 0 && !json.Valid(out.Definition.Params) {
				return fmt.Errorf("configured output %q has invalid params JSON", out.ID)
			}
		}
	}
	return nil
}

// Clone detaches an output's parameter bytes, guard tree and amount data.
// It performs no semantic interpretation or validation; callers validate the
// frame and the owning vocabulary validates the meaning before assessment.
func (o ConfiguredOutput) Clone() ConfiguredOutput {
	out := o
	out.Definition.Params = slices.Clone(o.Definition.Params)
	out.Definition.When = cloneGuard(o.Definition.When)
	if o.Definition.Amount != nil {
		amount := *o.Definition.Amount
		if amount.Fixed != nil {
			n := *amount.Fixed
			amount.Fixed = &n
		}
		if amount.ByClassLevel != nil {
			steps := *amount.ByClassLevel
			steps.Steps = maps.Clone(steps.Steps)
			amount.ByClassLevel = &steps
		}
		out.Definition.Amount = &amount
	}
	return out
}

func cloneGuard(in *machine.Guard) *machine.Guard {
	if in == nil {
		return nil
	}
	out := *in
	out.All = cloneGuards(in.All)
	out.Any = cloneGuards(in.Any)
	out.Not = cloneGuard(in.Not)
	if in.Fact != nil {
		v := *in.Fact
		v.Value = cloneLiteral(v.Value)
		out.Fact = &v
	}
	if in.Bag != nil {
		v := *in.Bag
		v.Value = cloneLiteral(v.Value)
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

func cloneGuards(in []machine.Guard) []machine.Guard {
	if in == nil {
		return nil
	}
	out := make([]machine.Guard, len(in))
	for i := range in {
		out[i] = *cloneGuard(&in[i])
	}
	return out
}

// Literal validation belongs to the vocabulary. Preserve ownership even for
// JSON-shaped invalid literals so a later refusal cannot mutate captured data.
func cloneLiteral(in any) any {
	switch v := in.(type) {
	case []any:
		out := make([]any, len(v))
		for i := range v {
			out[i] = cloneLiteral(v[i])
		}
		return out
	case map[string]any:
		out := make(map[string]any, len(v))
		for key, value := range v {
			out[key] = cloneLiteral(value)
		}
		return out
	default:
		return in
	}
}

func cloneHeldEffects(in []HeldEffectDetail) []HeldEffectDetail {
	if in == nil {
		return nil
	}
	out := make([]HeldEffectDetail, len(in))
	for i, detail := range in {
		out[i] = detail
		if detail.Outputs != nil {
			out[i].Outputs = make([]ConfiguredOutput, len(detail.Outputs))
			for j, output := range detail.Outputs {
				out[i].Outputs[j] = output.Clone()
			}
		}
	}
	return out
}
