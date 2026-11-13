// Copyright (C) 2026 Kirk Diggler
// SPDX-License-Identifier: GPL-3.0-or-later

package effects

import (
	"fmt"
	"strings"
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
	return validatePartDefinition(in.Declaration, definition)
}

func validateDurationPart(declaration PartDeclaration) error {
	_, err := parseDuration(declaration)
	return err
}

func validateEndOnPart(declaration PartDeclaration) error {
	_, err := parseEndOn(declaration)
	return err
}

func validateAddToRollPart(declaration PartDeclaration) error {
	_, err := addToRollPartOutput(declaration)
	return err
}

func validateResistancePart(declaration PartDeclaration) error {
	_, err := resistancePartOutput(declaration)
	return err
}
