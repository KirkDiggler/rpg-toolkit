// Copyright (C) 2026 Kirk Diggler
// SPDX-License-Identifier: GPL-3.0-or-later

package encounter

import (
	"testing"

	"github.com/stretchr/testify/require"
)

func TestPerceptionSubjectsOtherThanTheCurrentVersionAreRefused(t *testing.T) {
	for _, version := range []uint32{0, 99} {
		require.ErrorIs(t, requirePerceptionSubjects(version), ErrInvalidData, "version %d", version)
	}
	require.NoError(t, requirePerceptionSubjects(perceptionSubjectVersion))
}
