// Copyright (C) 2026 Kirk Diggler
// SPDX-License-Identifier: GPL-3.0-or-later

package effects

import (
	"encoding/json/jsontext"
	"fmt"
	"strings"

	generic "github.com/KirkDiggler/rpg-toolkit/mechanics/effects"
)

// ValidatePartInput supplies authored parameters without inventing a holder or
// application. Runtime identity validation belongs to BindPart.
type ValidatePartInput struct{ Declaration PartDeclaration }

// ValidatePart refuses invalid authored part data before payment/delivery. It
// uses the same private kind validation as binding; no runtime is constructed.
func ValidatePart(in *ValidatePartInput) error {
	if in == nil || strings.TrimSpace(in.Declaration.Key) == "" {
		return fmt.Errorf("part validation requires a declaration and key")
	}
	definition, ok := kindTable[in.Declaration.Kind]
	if !ok || definition.validatePart == nil {
		return fmt.Errorf("unsupported effect part %q", in.Declaration.Kind)
	}
	return definition.validatePart(in.Declaration)
}

func validateDurationPart(declaration PartDeclaration) error {
	_, err := parseDuration(declaration)
	return err
}

func validateEndOnPart(declaration PartDeclaration) error {
	_, err := parseEndOn(declaration)
	return err
}

func validateRollPart(declaration PartDeclaration) error {
	_, err := rollPartOutput(declaration)
	return err
}

func validateResistancePart(declaration PartDeclaration) error {
	if declaration.OnTerminal != generic.EndPart {
		return fmt.Errorf("resistance payload requires local part scope")
	}
	return checkResistance(generic.Output{Kind: KindResistance, Params: jsontext.Value(declaration.Params)})
}
