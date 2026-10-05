// Copyright (C) 2026 Kirk Diggler
// SPDX-License-Identifier: GPL-3.0-or-later

package contributions

import (
	"fmt"
	"strings"
)

// Applicability is an owning rule's answer. Unsupported evaluation and failed
// validation are not eligibility answers; neither may become DoesNotApply.
type Applicability string

const (
	// Applies means the supplied facts establish this rule's eligibility.
	Applies Applicability = "applies"
	// DoesNotApply means known facts establish ineligibility.
	DoesNotApply Applicability = "does_not_apply"
	// Depends means the frame's facts cannot yet settle eligibility. At
	// execution it is an error, never a silent non-application.
	Depends Applicability = "depends"
)

// Decision is the explanation shared by execution and information consumers.
// It does not grant permission to act or decide a stacking winner.
type Decision struct {
	Applicability Applicability
	Reason        string
}

// Validate requires one of the three answers and an owner-authored reason.
// The zero decision is invalid: an empty answer is a producer defect.
func (d Decision) Validate() error {
	if strings.TrimSpace(d.Reason) == "" {
		return fmt.Errorf("rule decision requires a reason")
	}
	switch d.Applicability {
	case Applies, DoesNotApply, Depends:
		return nil
	default:
		return fmt.Errorf("unknown applicability %q", d.Applicability)
	}
}

// Participation says whether an effect joins the action as it is taken or is
// a choice its holder makes later, such as a die offered after the roll.
type Participation string

const (
	// ContributesNow means the effect joins the action's own calculation.
	ContributesNow Participation = "contributes_now"
	// LaterChoice means the effect is offered later and is never shown as
	// already added.
	LaterChoice Participation = "later_choice"
)
