// Copyright (C) 2026 Kirk Diggler
// SPDX-License-Identifier: GPL-3.0-or-later

package session

import (
	"context"
	"fmt"

	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/encounter"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/resolution"
)

// This file is the only reader of a resolution output's World,
// DirtyCharacters, DirtyMonsters, OpenedAreas, ClosedAreas,
// ConcentrationChecks and ConcentrationBreaks. A verb reads Outcome and Posed
// to choose what it records and which window it poses, and hands the rest to
// [Manager.land]. land_only_test.go holds the helpers below to this file.

// landing is what one verb or seam asks of an output's landing: where it
// lands, what it records, which windows it poses, and what runs after. The
// order is [Manager.land]'s, never the caller's.
type landing struct {
	// Live is the encounter a seam was called from; nil for a verb, whose scope
	// adopts the output's world. A Live landing never commits: the encounter
	// that called the seam is mid-verb, and the calling verb commits.
	Live *encounter.Encounter
	// Record tells the outcome's beats on enc, given the interaction's
	// concentration, and owns what it tells. Nil records nothing, and then a
	// landing handed concentration refuses with ErrInvalidWorld: no landing
	// drops concentration.
	Record func(enc *encounter.Encounter, told concentration) error
	// Window poses (and tells) the windows the output leaves. Nil poses none.
	// The window a resume answered was consumed before the resume
	// ([Manager.answerWindow]), so there is nothing to answer here.
	Window func(enc *encounter.Encounter) error
	// Continue is the verb's follow-on after the windows. Nil does nothing.
	Continue func(enc *encounter.Encounter) error
}

// concentration is the interaction's concentration checks and breaks, handed
// to the record step by the landing. No verb reads them off the output.
type concentration struct {
	Checks []encounter.ConcentrationCheck
	Breaks []encounter.ConcentrationBreak
}

// empty reports that there is no concentration to tell.
func (c concentration) empty() bool {
	return len(c.Checks) == 0 && len(c.Breaks) == 0
}

// landed is what a committed landing reports. A Live landing commits nothing
// and reports the zero value.
type landed struct {
	Saved    SaveReport
	Delivery DeliveryReport
}

// land lands out on scope in the one order: adopt the world, write the dirty
// sheets, record the outcome, land the areas, pose windows, run the
// continuation, commit. Every field of out it reads lands exactly once.
//
// A failure in the record, area, window or continuation step happens after the
// sheets are durable, so it passes through [translate] once and is reported
// with what landed ([reportUnrecorded], design ruling R10). A failure to adopt
// or to save a sheet, and a failure to commit, are returned as they are: the
// first two precede any write of this landing's, and commit reports its own.
func (m *Manager) land(ctx context.Context, scope *writeScope, out *resolution.Output, l *landing) (*landed, error) {
	unrecorded := func(err error) error {
		return reportUnrecorded(scope, translate(err))
	}

	// 1. Adopt: the world that came back is the only true one now. A seam's
	// landing adopts nothing, because the encounter that called it is mid-verb.
	enc := l.Live
	if enc == nil {
		if err := m.adopt(ctx, scope, out.World); err != nil {
			return nil, err
		}
		enc = scope.enc
	}

	// 2. Sheets, before any beat: the record consults who is standing, and
	// standingSeam answers out of exactly these stores.
	if err := m.saveDirty(ctx, scope, out); err != nil {
		return nil, err
	}

	// 3. Record, with the interaction's concentration. Concentration with
	// nothing to tell it is refused: no landing drops it.
	told := concentration{Checks: out.ConcentrationChecks, Breaks: out.ConcentrationBreaks}
	if l.Record != nil {
		if err := l.Record(enc, told); err != nil {
			return nil, unrecorded(err)
		}
	} else if !told.empty() {
		return nil, unrecorded(fmt.Errorf(
			"%w: %d concentration checks and %d breaks with nothing to tell them",
			ErrInvalidWorld, len(told.Checks), len(told.Breaks)))
	}

	// 4. Areas, after the beats that caused them. Every area an output
	// changed was caused by a unit it tells or concentration it tells, so
	// they always land now.
	if err := m.landAreas(enc, scope, out); err != nil {
		return nil, unrecorded(err)
	}

	// 5. Windows the output leaves.
	if l.Window != nil {
		if err := l.Window(enc); err != nil {
			return nil, unrecorded(err)
		}
		scope.data.Windows = scope.ledger.ToData()
		scope.touched = true
	}

	// 6. The verb's continuation.
	if l.Continue != nil {
		if err := l.Continue(enc); err != nil {
			return nil, unrecorded(err)
		}
	}

	// 7. A seam's landing commits nothing: the calling verb commits.
	if l.Live != nil {
		return &landed{}, nil
	}

	// 8. Commit.
	report, delivery, err := m.commit(ctx, scope)
	if err != nil {
		return nil, err
	}
	return &landed{Saved: report, Delivery: delivery}, nil
}

// saveDirty writes back every sheet the interaction changed.
//
// Characters go to the host's repository; NPCs live in the session record, so
// they are folded into it and the record is marked touched. This is the first
// verb in the package that writes a character at all — damage has to persist —
// and the no-clobber pin gained a row for it rather than losing its guard.
//
// IT RUNS BEFORE THE OUTCOME IS RECORDED, and that is a correctness ordering
// rather than a convenience: the composition's Record consults who is standing,
// [standingSeam] answers out of exactly these two stores, and a consult run
// against sheets this verb has not written back yet is a consult about a world
// that no longer exists. See [Manager.Attack].
func (m *Manager) saveDirty(ctx context.Context, scope *writeScope, out *resolution.Output) error {
	// The report names what LANDED as well as what did not (S6). A sheet
	// written before the failure is durable, and a caller told only about the
	// failure would retry a write that already succeeded — which is the
	// difference between repair and retry that the report exists to carry.
	//
	// Every entry goes on the SCOPE rather than a local, so it outlives this
	// call: the sheets are durable whether the next write succeeds or fails,
	// and persist opens its report with them either way. Kept in a local, they
	// were reported only when saveDirty itself failed — so a swing whose WORLD
	// save failed named nothing at all, and the host retried a swing whose
	// damage was already on disk (rpg-toolkit#1056).
	for _, data := range out.DirtyCharacters {
		if data == nil {
			continue
		}
		if err := m.sheetsFor(scope).save(ctx, data); err != nil {
			return err
		}
		// The walker's own sheet follows what the interaction did to it, so
		// the NEXT step of the same walk resolves over the damage this one
		// dealt rather than over the sheet the walk started with. See
		// [writeScope.walker].
		if scope.walker != nil && scope.walker.ID == data.ID {
			scope.walker = data
			scope.walkerDirtied = true
		}
	}

	for _, dirty := range out.DirtyMonsters {
		if dirty == nil {
			continue
		}
		scope.replaceMonsterSheet(dirty)
	}
	return nil
}

// landAreas applies the runtime areas an interaction closed and opened to the
// live encounter, through the encounter's own verbs, and refreshes perception
// over the area set they leave (rpg-project#539, "Who is in an area"). enc is
// the encounter the outcome was recorded on: scope.enc for a verb, the calling
// encounter for a seam the composition drives from inside its own verbs.
//
// IT RUNS AFTER THE OUTCOME IS RECORDED. The encounter tells every member
// inside a changed area the moment the change is applied, so the call order is
// the story order: the cast or the broken concentration first, then who
// entered or who was in the area that ended. A path that records nothing of
// its own calls it straight after [Manager.saveDirty].
//
// Closed before opened, as [resolution.Output.ClosedAreas] states: a recast
// ends the old area under the id the new one takes. No area set is copied
// back; the encounter holds the only one.
//
// IT CONSUMES WHAT IT LANDS: both lists are emptied on out once applied, so a
// path that reaches it twice for one output (a resumed walk's movement arm
// and its caller) lands each change once. A second AddSightArea of the same
// area would be refused as already open.
func (m *Manager) landAreas(enc *encounter.Encounter, scope *writeScope, out *resolution.Output) error {
	if len(out.ClosedAreas) == 0 && len(out.OpenedAreas) == 0 {
		return nil
	}
	for _, source := range out.ClosedAreas {
		if _, err := enc.RemoveSightArea(source); err != nil {
			return translate(err)
		}
	}
	for i := range out.OpenedAreas {
		if err := enc.AddSightArea(&out.OpenedAreas[i]); err != nil {
			return translate(err)
		}
	}
	out.ClosedAreas, out.OpenedAreas = nil, nil
	if err := enc.RefreshPerception(); err != nil {
		return translate(err)
	}
	scope.touched = true
	return nil
}
