// Copyright (C) 2026 Kirk Diggler
// SPDX-License-Identifier: GPL-3.0-or-later

package session

import (
	"strconv"
	"strings"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestDeathSaveRecordSequenceMismatchRedactsGlobalOrdering(t *testing.T) {
	const recorded, pending uint64 = 4_815_162_342, 8_151_623_421

	err := validateDeathSaveRecordSequence(recorded, pending)
	require.ErrorIs(t, err, ErrInvalidWorld)
	require.EqualError(t, err, "death save record sequence mismatch: invalid encounter data")

	message := strings.ToLower(err.Error())
	for _, private := range []string{
		strconv.FormatUint(recorded, 10),
		strconv.FormatUint(pending, 10),
		"global",
		"watermark",
		"pending",
		"expected",
	} {
		require.NotContains(t, message, private)
	}

	require.NoError(t, validateDeathSaveRecordSequence(pending, pending))
}
