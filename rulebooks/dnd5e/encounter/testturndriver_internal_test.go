// Copyright (C) 2026 Kirk Diggler
// SPDX-License-Identifier: GPL-3.0-or-later

package encounter

// passDriver is the internal-package twin of encounter_test's own — see its
// doc. A separate type because the internal test package cannot import the
// external one (and does not need to: this file's whole job is being small).
type passDriver struct{}

func (passDriver) Act(MemberID) (TurnOutcome, error) {
	return Pass{}, nil
}
