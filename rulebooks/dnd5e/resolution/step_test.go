// Copyright (C) 2026 Kirk Diggler
// SPDX-License-Identifier: GPL-3.0-or-later

package resolution

import (
	"context"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/KirkDiggler/rpg-toolkit/events"
)

// pointerStepMachine returns *Done — a legal implementer of Step (isStep has a
// value receiver, so pointer forms carry it too) that the driver deliberately
// refuses: the vocabulary is the value forms, one spelling per case.
type pointerStepMachine struct{}

func (pointerStepMachine) Start(_ context.Context, _ *Participants) (Step, error) {
	return &Done{Outcome: captureOutcome{}}, nil
}

// The refusal is diagnosable: the error names the concrete type, so a machine
// author who wrote &Done{} instead of Done{} reads the answer off the message.
func TestDriveNamesAForeignStepForm(t *testing.T) {
	_, err := drive(context.Background(), events.NewEventBus(), pointerStepMachine{}, &Participants{})

	require.ErrorIs(t, err, ErrBadStep)
	require.Contains(t, err.Error(), "*resolution.Done")
}
