// Copyright (C) 2026 Kirk Diggler
// SPDX-License-Identifier: GPL-3.0-or-later

package conditions

import (
	"encoding/json"
	"fmt"
	"strings"
	"sync"

	"github.com/KirkDiggler/rpg-toolkit/core"
	generic "github.com/KirkDiggler/rpg-toolkit/mechanics/effects"
	"github.com/KirkDiggler/rpg-toolkit/rpgerr"
	dndfx "github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/effects"
	dnd5eEvents "github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/events"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/shared"
)

// rulesData is the rules library: the effects every creature has by rule,
// as data in the same part shape an author's effect compiles to. Each entry
// is a source a player reads and one part. See the package doc for why these
// are never stored.
const rulesData = `[
 {"source":"Dexterity","part":{"key":"dexterity-initiative","kind":"initiative_bonus","on_terminal":"part",
  "amount":{"ability_modifier":{"ability":"dex"}}}}
]`

// ruleEntry is one rule as written.
type ruleEntry struct {
	Source string                `json:"source"`
	Part   dndfx.PartDeclaration `json:"part"`
}

// rule is one rule compiled and composed, unattached: a bound output nothing
// subscribes and no sheet holds.
type rule struct {
	source string
	output *generic.Bound
}

// rulesRef is the origin every rule is composed under. It names no condition
// in the registry: a rule is never a condition on a sheet.
var rulesRef = core.Ref{Module: "dnd5e", Type: "rules", ID: "every-creature"}

// rulesLibrary compiles rulesData once, the first time a rule is read.
var rulesLibrary = sync.OnceValues(func() ([]rule, error) { return compileRules(json.RawMessage(rulesData)) })

// compileRules decodes, validates and composes a rules library. It refuses
// unknown members, a blank source, a repeated key, and any part the effect
// vocabulary refuses (an unknown ability among them).
func compileRules(data json.RawMessage) ([]rule, error) {
	var entries []ruleEntry
	if err := strictJSON(data, &entries); err != nil {
		return nil, fmt.Errorf("rules library: %w", err)
	}
	seen := map[string]bool{}
	out := make([]rule, 0, len(entries))
	for i, entry := range entries {
		if strings.TrimSpace(entry.Source) == "" {
			return nil, fmt.Errorf("rules library [%d]: a rule needs a source", i)
		}
		if seen[entry.Part.Key] {
			return nil, fmt.Errorf("rules library [%d]: repeated key %q", i, entry.Part.Key)
		}
		seen[entry.Part.Key] = true
		if err := dndfx.ValidatePart(&dndfx.ValidatePartInput{Declaration: entry.Part}); err != nil {
			return nil, fmt.Errorf("rules library %q: %w", entry.Source, err)
		}
		ref := core.Ref{Module: rulesRef.Module, Type: rulesRef.Type, ID: entry.Part.Key}
		bound, err := dndfx.BindPart(&dndfx.BindPartInput{Declaration: entry.Part, Ref: ref, HolderID: rulesRef.ID,
			ApplicationID: rulesRef.ID, DisplayName: entry.Source, Origin: dnd5eEvents.ConditionOrigin{Ref: rulesRef, EntityID: rulesRef.ID}})
		if err != nil {
			return nil, fmt.Errorf("rules library %q: %w", entry.Source, err)
		}
		composed, err := generic.Compose(bound.Part.Compose)
		if err != nil {
			return nil, fmt.Errorf("rules library %q: %w", entry.Source, err)
		}
		outputs := composed.CurrentOutputs()
		if len(outputs) != 1 {
			return nil, fmt.Errorf("rules library %q: a rule is one output, got %d", entry.Source, len(outputs))
		}
		out = append(out, rule{source: entry.Source, output: outputs[0]})
	}
	return out, nil
}

// InitiativeInput is one creature: who it is, its ability scores, and the
// conditions it holds.
type InitiativeInput struct {
	MemberID   string
	Scores     shared.AbilityScores
	Conditions []dnd5eEvents.ConditionBehavior
}

// InitiativeContribution is one named thing a creature adds to initiative.
type InitiativeContribution struct {
	// Source is what a player reads: "Dexterity", or a held effect's name.
	Source string
	// Amount is what it adds; negative subtracts.
	Amount int
}

// InitiativeOutput is a creature's initiative modifier and where it comes
// from.
type InitiativeOutput struct {
	// Total is the sum of Contributions.
	Total int
	// Contributions are the rules first, in library order, then each held
	// effect that adds to initiative, in the order the sheet holds them.
	Contributions []InitiativeContribution
}

// Initiative is the ONE question a fight asks of a creature for its
// initiative: every initiative effect it has, by rule and held, each with its
// source, and their total.
//
// A rule whose amount reads a score the creature's sheet does not state (no
// ability scores at all, or none for that ability) contributes nothing rather
// than reading the missing score as 0.
//
// Errors: a nil input or a blank member (CodeInvalidArgument), a rules
// library that does not compile, and a held condition the readers refuse.
func Initiative(in *InitiativeInput) (*InitiativeOutput, error) {
	if in == nil || strings.TrimSpace(in.MemberID) == "" {
		return nil, rpgerr.New(rpgerr.CodeInvalidArgument, "initiative requires a member")
	}
	rules, err := rulesLibrary()
	if err != nil {
		return nil, err
	}
	holder := &dndfx.HolderQuery{MemberID: in.MemberID, Scores: in.Scores}
	out := &InitiativeOutput{}
	add := func(source string, n int) {
		out.Contributions = append(out.Contributions, InitiativeContribution{Source: source, Amount: n})
		out.Total += n
	}
	for _, r := range rules {
		answer, err := dndfx.InitiativeBonusOf(&dndfx.HeldBonusInput{Output: r.output, Holder: holder})
		if err != nil {
			return nil, fmt.Errorf("rule %q: %w", r.source, err)
		}
		if answer.Answered {
			add(r.source, answer.Amount)
		}
	}
	for _, held := range in.Conditions {
		if held == nil || held.Ref() == nil {
			return nil, rpgerr.New(rpgerr.CodeInvalidArgument, "initiative requires conditions with refs")
		}
		name, outputs := heldOutputs(held)
		total, answered := 0, false
		for _, output := range outputs {
			answer, err := dndfx.InitiativeBonusOf(&dndfx.HeldBonusInput{Output: output, Holder: holder})
			if err != nil {
				return nil, fmt.Errorf("held %s: %w", held.Ref(), err)
			}
			if answer.Answered {
				total += answer.Amount
				answered = true
			}
		}
		if answered {
			add(name, total)
		}
	}
	return out, nil
}

// heldOutputs is a held condition's display name and current outputs, for the
// two condition types whose effects are data.
func heldOutputs(held dnd5eEvents.ConditionBehavior) (string, []*generic.Bound) {
	switch c := held.(type) {
	case *LinkedCondition:
		outputs := make([]*generic.Bound, 0)
		for _, part := range c.assembly.CurrentOutputs() {
			outputs = append(outputs, part.Output)
		}
		return c.bindings.Name, outputs
	case *ComposedCondition:
		return c.display.Name, c.runtime.CurrentOutputs()
	default:
		return "", nil
	}
}
