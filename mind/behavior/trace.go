// Copyright (C) 2026 Kirk Diggler
// SPDX-License-Identifier: GPL-3.0-or-later

package behavior

import "github.com/KirkDiggler/rpg-toolkit/mind/machine"

const (
	// TraceSleeping means timed input was not delivered before the role's wake.
	TraceSleeping = "sleeping"
	// TraceNotAddressed means this observation targeted a different instance.
	TraceNotAddressed = "not_addressed"
)

const (
	traceNotSkipped uint8 = iota
	traceSleeping
	traceNotAddressed
)

// InstanceOutcome explains one instance's participation in a successful call,
// in authored precedence order. Definition is its existing configuration digest.
// Before and After are the actual states before and after the composer call.
// Step may describe a PROPOSED transition whose resulting blob was not committed.
//
// Requested is the table the preview asked for. Selected means its table owned
// the choice, including a no-candidate hold; Committed says the candidate blob
// was retained. A data-only update can commit without selection, and consuming a
// due wake can commit without firing any transition. Skipped names nondelivery,
// never a false guard. Step is nil only for such a skipped instance.
type InstanceOutcome struct {
	ID         string
	Definition string
	Before     string
	After      string
	Requested  AnswerKey
	Selected   bool
	Committed  bool
	Skipped    string
	Step       *machine.TraceStepOutput
}

// instanceOutcomes reads the decisions already made at preview/selection/commit
// sites. It neither repeats those decisions nor invokes any vocabulary reader.
func (c *Composer) instanceOutcomes(proposals []compositionProposal) []InstanceOutcome {
	if len(proposals) == 0 {
		return nil
	}
	out := make([]InstanceOutcome, len(proposals))
	for i, p := range proposals {
		instance := &c.instances[i]
		before := instance.blob.State
		if p.traced != nil {
			before = p.traced.Trace.Before
		}
		skipped := ""
		switch p.skipped {
		case traceSleeping:
			skipped = TraceSleeping
		case traceNotAddressed:
			skipped = TraceNotAddressed
		}
		out[i] = InstanceOutcome{
			ID: instance.spec.ID, Definition: instance.definition,
			Before: before, After: instance.blob.State, Requested: p.pick,
			Selected: p.selected, Committed: p.committed, Skipped: skipped, Step: p.traced,
		}
	}
	return out
}
