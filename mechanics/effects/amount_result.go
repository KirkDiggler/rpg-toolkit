// Copyright (C) 2026 Kirk Diggler
// SPDX-License-Identifier: GPL-3.0-or-later

package effects

import (
	"errors"
	"fmt"
	"strings"
)

// ErrInvalidAmountResult marks a missing or ambiguous amount description.
var ErrInvalidAmountResult = errors.New("invalid amount result")

// ErrUnresolvedAmount marks an attempt to read dice as an already settled scalar.
var ErrUnresolvedAmount = errors.New("amount is unresolved dice")

// AmountResult describes either a present scalar (including zero) or unresolved
// dice. It never rolls. Dice syntax and its interpretation belong to the consumer
// vocabulary; the generic boundary only requires a nonblank description.
type AmountResult struct {
	Scalar *int   `json:"scalar,omitempty"`
	Dice   string `json:"dice,omitempty"`
}

// Validate refuses neither/both alternatives and blank dice descriptions.
func (r AmountResult) Validate() error {
	if r.Scalar != nil && r.Dice == "" {
		return nil
	}
	if r.Scalar == nil && strings.TrimSpace(r.Dice) != "" {
		return nil
	}
	return fmt.Errorf("%w: set exactly one scalar or nonblank dice expression", ErrInvalidAmountResult)
}

// ScalarValue returns a settled scalar, refusing invalid results and unresolved
// dice instead of converting, rolling or inventing zero.
func (r AmountResult) ScalarValue() (int, error) {
	if err := r.Validate(); err != nil {
		return 0, err
	}
	if r.Scalar == nil {
		return 0, ErrUnresolvedAmount
	}
	return *r.Scalar, nil
}

func (r AmountResult) owned() AmountResult {
	if r.Scalar != nil {
		value := *r.Scalar
		r.Scalar = &value
	}
	return r
}
