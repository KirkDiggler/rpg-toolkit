// Copyright (C) 2026 Kirk Diggler
// SPDX-License-Identifier: GPL-3.0-or-later

package effects

import (
	"encoding/json/jsontext"
	"fmt"
	"slices"
	"strings"

	generic "github.com/KirkDiggler/rpg-toolkit/mechanics/effects"
)

// KindCannot takes away one or more things the holder may do, while the part
// is held. Its params name what is taken, as a closed list:
// {"what":["move"]}. It takes no amount and no guard.
//
// It is registered in [PartClassBenefit], which means "what is in force on
// the recipient while the effect is held" — not "good for the holder". This
// kind TAKES SOMETHING AWAY: a hold is in force on its holder exactly as a
// raise is, and is read the same way.
//
// Like the held bonuses it is query-only: nothing is attached to a bus. The
// sheet that holds the condition asks what is taken each time the question is
// asked, so a removed condition is simply no longer asked.
const KindCannot generic.KindName = "cannot"

// Taken is one thing a [KindCannot] part takes away from its holder.
type Taken string

const (
	// TakenMove takes away moving by the holder's own will. A move made on
	// the holder (a push, a carry) is not the holder's own and is not taken.
	TakenMove Taken = "move"
)

// takenWords is the closed list of what can be taken, each with the words its
// information row uses.
var takenWords = map[Taken]string{
	TakenMove: "move",
}

const domainCannot domain = "cannot"

type cannotParams struct {
	What []Taken `json:"what"`
}

// parseCannot refuses an empty list, an unknown value and a repeated value.
func parseCannot(raw jsontext.Value) ([]Taken, error) {
	params, err := decodeParams[cannotParams](raw)
	if err != nil {
		return nil, fmt.Errorf("cannot requires params naming what is taken: %w", err)
	}
	if len(params.What) == 0 {
		return nil, fmt.Errorf("cannot requires at least one thing taken away")
	}
	seen := make([]Taken, 0, len(params.What))
	for _, what := range params.What {
		if _, ok := takenWords[what]; !ok {
			return nil, fmt.Errorf("cannot does not know %q; it can take away: %s", what, takenList())
		}
		if slices.Contains(seen, what) {
			return nil, fmt.Errorf("cannot lists %q twice", what)
		}
		seen = append(seen, what)
	}
	return seen, nil
}

func takenList() string {
	words := make([]string, 0, len(takenWords))
	for what := range takenWords {
		words = append(words, string(what))
	}
	slices.Sort(words)
	return strings.Join(words, ", ")
}

func cannotMeaning(raw jsontext.Value) (meaning, error) {
	taken, err := parseCannot(raw)
	if err != nil {
		return meaning{}, err
	}
	words := make([]string, 0, len(taken))
	for _, what := range taken {
		words = append(words, takenWords[what])
	}
	return meaning{domain: domainCannot, description: "Cannot " + strings.Join(words, ", ") + "."}, nil
}

// checkCannot refuses anything but a valid what list: no amount and no guard.
func checkCannot(out generic.Output) error {
	if _, err := parseCannot(jsontext.Value(out.Params)); err != nil {
		return err
	}
	if out.When != nil {
		return fmt.Errorf("cannot does not take a guard: it is in force while held")
	}
	if out.Amount != nil {
		return fmt.Errorf("cannot does not take an amount")
	}
	return nil
}

func cannotPayload(decl PartDeclaration) (generic.Output, error) {
	if decl.OnTerminal != generic.EndPart {
		return generic.Output{}, fmt.Errorf("cannot payload requires local part scope")
	}
	out := generic.Output{Kind: KindCannot, Params: jsontext.Value(decl.Params), When: decl.When, Amount: decl.Amount}
	if err := checkCannot(out); err != nil {
		return generic.Output{}, err
	}
	return out, nil
}

func validateCannotPart(decl PartDeclaration) error {
	_, err := cannotPayload(decl)
	return err
}

func bindCannotPart(in *BindPartInput) (boundPart, error) {
	out, err := cannotPayload(in.Declaration)
	if err != nil {
		return boundPart{}, err
	}
	return payloadRecipe(in, out)
}

func inspectCannotPart(in *InspectPartInput) (*PartInfo, error) {
	if err := validateCannotPart(in.Declaration); err != nil {
		return nil, err
	}
	return inspectPayloadState(in.Snapshot)
}

func cannotDefinition() kindDefinition {
	return kindDefinition{class: PartClassBenefit, payload: cannotPayload, validatePart: validateCannotPart, part: bindCannotPart,
		inspectPart: inspectCannotPart, execution: generic.KindQuery, check: checkCannot, meaning: cannotMeaning}
}

// CannotInput asks one bound output what it takes away from its holder.
type CannotInput struct {
	Output *generic.Bound
}

// CannotOutput is what the output takes away, in the order the author listed
// it. Empty when the output is of another kind.
type CannotOutput struct {
	Taken []Taken
}

// CannotOf answers what one bound output takes away from its holder. Errors:
// a nil input or output, or a cannot output whose list is empty, unknown or
// repeated.
func CannotOf(in *CannotInput) (*CannotOutput, error) {
	if in == nil || in.Output == nil {
		return nil, fmt.Errorf("cannot read requires a bound output")
	}
	if in.Output.Kind() != KindCannot {
		return &CannotOutput{}, nil
	}
	definition := in.Output.Definition()
	if err := checkCannot(definition); err != nil {
		return nil, err
	}
	taken, err := parseCannot(jsontext.Value(definition.Params))
	if err != nil {
		return nil, err
	}
	return &CannotOutput{Taken: taken}, nil
}
