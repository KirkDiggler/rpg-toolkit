// Copyright (C) 2026 Kirk Diggler
// SPDX-License-Identifier: GPL-3.0-or-later

package resolution

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/KirkDiggler/rpg-toolkit/dice"
	combatActions "github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/combat/actions"
	dnd5eEvents "github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/events"
)

// frozenCastKind and frozenCastVersion discriminate what a stored blob is,
// the same trust boundary [frozenStrikeKind]/[frozenContestKind] keep.
const (
	frozenCastKind    = "cast.post_save_roll"
	frozenCastVersion = 1
)

// frozenCastTargetState is one target's PREFLIGHT verdict, carried rather
// than re-derived on resume for [frozenStrike]'s reason: a target that moved
// out of range during the pause must not change what [castMachine.Start]
// already decided before the door charged the caster.
type frozenCastTargetState struct {
	TargetID string `json:"target_id"`
	Missed   bool   `json:"missed"`
}

// frozenCast is a multi-target cast stopped mid-save on ONE of its targets,
// in enough detail to finish every remaining target and in no more detail
// than that — the cast sibling of [frozenStrike] and [frozenContest].
//
// Definition is stored WHOLE, [frozenStrike.Definition]'s own reason: a
// target beyond the one that posed may still need a fresh
// [newGatedCast]/[newGatelessCast] machine built for it, exactly as the
// original [newCast] built one, and that construction takes the full
// definition.
type frozenCast struct {
	Kind    string `json:"kind"`
	Version int    `json:"version"`

	Definition     combatActions.Definition `json:"definition"`
	CasterID       string                   `json:"caster_id"`
	Option         string                   `json:"option,omitempty"`
	DerivedTargets bool                     `json:"derived_targets,omitempty"`

	// Targets is every target this cast named, in the SAME order
	// [castMachine.targets] held them, so Index below addresses the same
	// entry it did when the pose was frozen.
	Targets []frozenCastTargetState `json:"targets"`

	// Index is which target posed.
	Index int `json:"index"`

	// Outcome is every target's outcome BEFORE Index — already delivered,
	// already recorded, and not touched again on resume.
	Outcome CastOutcome `json:"outcome"`

	// Contest is target Index's own frozen bytes, opaque here exactly as
	// [frozenContest.Save] is opaque to [castMachine].
	Contest json.RawMessage `json:"contest"`
}

// poseCast turns a contest's own pose into the cast's, freezing what
// [castMachine.resolveTarget] needs to finish every remaining target once
// the posed one is answered.
func poseCast(m *castMachine, index int, contest Pose) (Step, error) {
	targets := make([]frozenCastTargetState, len(m.targets))
	for i, t := range m.targets {
		targets[i] = frozenCastTargetState{TargetID: t.targetID, Missed: t.missed}
	}

	profile := m.profile
	frozen, err := json.Marshal(frozenCast{
		Kind: frozenCastKind, Version: frozenCastVersion,
		Definition:     combatActions.Definition{Ref: m.spell, Name: m.spellName, Cast: &profile},
		CasterID:       m.casterID,
		Option:         m.option,
		DerivedTargets: m.derivedTargets,
		Targets:        targets,
		Index:          index,
		Outcome:        m.outcome,
		Contest:        json.RawMessage(contest.Frozen),
	})
	if err != nil {
		return nil, fmt.Errorf("%w: freeze cast: %v", ErrBadFrozen, err)
	}

	return Pose{Ask: contest.Ask, Frozen: frozen}, nil
}

// CastResumeInput continues a cast that posed on one of its targets' saves.
type CastResumeInput struct {
	// Frozen is the bytes [Pose.Frozen] handed out. REQUIRED.
	Frozen []byte

	// Answer is [OfferSpend] or [OfferKeep]. REQUIRED.
	Answer OfferAnswer

	// Roller rolls the offered die, and rolls for every target beyond the
	// one that posed that has not yet made its own save. REQUIRED: the same
	// interaction, the same roller, whether or not any one target suspended
	// it.
	Roller dice.Roller
}

// NewCastResumed returns the machine that finishes a cast somebody answered
// the offer on — the cast sibling of [NewStrikeResumed]. It is handed to
// [Resolve] exactly like a fresh cast's machine: nothing downstream of
// [driveStep] needs to know a machine is resuming rather than starting.
//
// # It fails closed on a frozen blob it cannot trust
//
// Kind and version are checked before the world is loaded and before
// anything is charged — repairing either would resolve a cast nobody paid
// for a second time.
func NewCastResumed(in *CastResumeInput) (Machine, error) {
	if in == nil {
		return nil, ErrNilInput
	}
	if len(in.Frozen) == 0 {
		return nil, fmt.Errorf("%w: no frozen cast to resume", ErrBadFrozen)
	}
	switch in.Answer {
	case OfferSpend, OfferKeep:
	default:
		return nil, fmt.Errorf("%w: %q is not an answer this machine posed", ErrNotOffered, in.Answer)
	}

	var frozen frozenCast
	if err := json.Unmarshal(in.Frozen, &frozen); err != nil {
		return nil, fmt.Errorf("%w: %v", ErrBadFrozen, err)
	}
	if frozen.Kind != frozenCastKind || frozen.Version != frozenCastVersion {
		return nil, fmt.Errorf("%w: kind %q version %d is not one this build froze",
			ErrBadFrozen, frozen.Kind, frozen.Version)
	}
	if frozen.Definition.Cast == nil {
		return nil, fmt.Errorf("%w: frozen cast has no cast profile", ErrBadFrozen)
	}
	if frozen.Index < 0 || frozen.Index >= len(frozen.Targets) {
		return nil, fmt.Errorf("%w: frozen cast index %d is out of range for %d targets",
			ErrBadFrozen, frozen.Index, len(frozen.Targets))
	}

	var resumedInner Machine
	var err error
	if frozen.Definition.Cast.Attack != nil {
		resumedInner, err = NewStrikeResumed(&StrikeResumeInput{Frozen: frozen.Contest, Answer: in.Answer, Roller: in.Roller})
	} else {
		resumedInner, err = newContestResumed(frozen.Contest, in.Answer, in.Roller)
	}
	if err != nil {
		return nil, err
	}

	targets := make([]castTargetMachine, len(frozen.Targets))
	for i, t := range frozen.Targets {
		targets[i] = castTargetMachine{targetID: t.TargetID, missed: t.Missed}
	}

	profile := *frozen.Definition.Cast
	m := &castMachine{
		spell: frozen.Definition.Ref, spellName: frozen.Definition.Name, casterID: frozen.CasterID,
		option: frozen.Option, profile: profile, concentration: profile.Concentration,
		targets: targets, derivedTargets: frozen.DerivedTargets, outcome: frozen.Outcome,
	}

	return &castResumeMachine{
		cast: m, index: frozen.Index, resumedInner: resumedInner,
		definition: frozen.Definition, option: frozen.Option, roller: in.Roller,
	}, nil
}

// castResumeMachine rebuilds every target [castMachine.resolveTarget] still
// needs to run, in one Start, then hands off to the ordinary resolveTarget
// chain from the posed index onward.
//
// A distinct type from [castMachine] rather than a resume field on it,
// [contestResumeMachine]'s own reason: a fresh cast's Start preflights every
// target through its OWN Start; a resumed cast preflights nothing before
// Index (already delivered) and substitutes the frozen contest for Index
// itself, which is a different enough shape to deserve its own Start rather
// than a branch inside the one that already does something else.
type castResumeMachine struct {
	cast         *castMachine
	index        int
	resumedInner Machine
	definition   combatActions.Definition
	option       string
	roller       dice.Roller
}

func (m *castResumeMachine) Start(ctx context.Context, cast *Participants) (Step, error) {
	m.cast.cast = cast

	cause := dnd5eEvents.SaveCause{
		Trigger: dnd5eEvents.SaveTriggerSpell, EffectRef: &m.definition.Ref, InstigatorID: m.cast.casterID,
	}
	gated := m.definition.Cast.Save != nil

	for i := range m.cast.targets {
		target := &m.cast.targets[i]
		switch {
		case i < m.index || target.missed:
			// Already delivered ([CastResumeInput] carries it in Outcome), or
			// never reaches its inner machine at all — [castMachine.resolveTarget]
			// short-circuits a missed target before touching target.inner.
			continue

		case i == m.index:
			first, err := m.resumedInner.Start(ctx, cast)
			if err != nil {
				return nil, fmt.Errorf("target %d %q: %w", i, target.targetID, err)
			}
			target.inner, target.first = m.resumedInner, first

		default:
			// A target beyond the one that posed, not yet touched. Rebuilt
			// exactly as [newCast] built it the first time — same
			// definition, same option, same roller — because eligibility for
			// every target was already decided once, before the door
			// charged the caster, and is not re-decided here.
			var inner Machine
			var err error
			if m.definition.Cast.Attack != nil {
				inner, err = newAttackCast(m.definition, m.cast.casterID, target.targetID, m.roller)
			} else if gated {
				inner, err = newGatedCast(m.definition, m.cast.casterID, target.targetID, m.option, cause, m.roller)
			} else {
				inner, err = newGatelessCast(m.definition, m.cast.casterID, target.targetID, m.option, m.roller)
			}
			if err != nil {
				return nil, fmt.Errorf("target %d %q: %w", i, target.targetID, err)
			}
			first, startErr := inner.Start(ctx, cast)
			if startErr != nil {
				return nil, fmt.Errorf("target %d %q: %w", i, target.targetID, startErr)
			}
			target.inner, target.first = inner, first
		}
	}

	return m.cast.resolveTarget(m.index), nil
}
